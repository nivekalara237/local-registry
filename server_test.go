package local_registry

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/nivekalara237/local-registry/internal/testutil"
)

// freePort returns a port that was free a moment ago.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestServeContextFailsFastOnBusyPort(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	s := NewLocalRegistryServer(ln.Addr().(*net.TCPAddr).Port, "localhost", t.TempDir())

	done := make(chan error, 1)
	go func() { done <- s.ServeContext(context.Background()) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a bind error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ServeContext did not fail on a busy port")
	}
}

func TestServeContextDrainsInFlightRequestsOnShutdown(t *testing.T) {
	port := freePort(t)
	s := NewLocalRegistryServer(port, "localhost", t.TempDir())
	started := make(chan struct{})
	s.AddRoute("/slow", "GET", func(...string) any {
		close(started)
		time.Sleep(300 * time.Millisecond)
		return "done"
	})

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- s.ServeContext(ctx) }()

	url := "http://" + net.JoinHostPort("localhost", strconv.Itoa(port)) + "/slow"
	type result struct {
		body string
		err  error
	}
	resc := make(chan result, 1)
	go func() {
		var resp *http.Response
		var err error
		for range 50 { // the listener may not be up yet
			if resp, err = http.Get(url); err == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if err != nil {
			resc <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		resc <- result{string(b), err}
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request never reached the handler")
	}
	cancel()

	res := <-resc
	if res.err != nil || res.body != "\"done\"\n" {
		t.Fatalf("in-flight request: body = %q, err = %v", res.body, res.err)
	}
	if err := <-served; err != nil {
		t.Fatalf("ServeContext = %v", err)
	}
}

func TestLogMiddlewareRecordsStatus(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	h := logMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/missing", nil))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log = %q: %v", buf.String(), err)
	}
	if entry["msg"] != "request" || entry["method"] != "GET" || entry["path"] != "/missing" || entry["status"] != float64(404) || entry["remote"] != "192.0.2.1" {
		t.Fatalf("log entry = %v", entry)
	}
	if _, ok := entry["duration"]; !ok {
		t.Fatalf("log entry has no duration: %v", entry)
	}
}

// selfSigned writes a certificate for localhost/127.0.0.1 and its key, and returns their paths and the cert.
func selfSigned(t *testing.T) (certFile, keyFile string, cert *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ = x509.ParseCertificate(der)
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	testutil.WriteFile(t, certFile, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
	testutil.WriteFile(t, keyFile, string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})))
	return certFile, keyFile, cert
}

func TestServeContextTLS(t *testing.T) {
	certFile, keyFile, cert := selfSigned(t)
	port := freePort(t)
	s := NewLocalRegistryServer(port, "localhost", t.TempDir())
	if err := s.SetTLS(certFile, keyFile); err != nil {
		t.Fatal(err)
	}
	if !s.TLSEnabled() {
		t.Fatal("TLSEnabled = false after SetTLS")
	}
	s.AddRoute("/ping", "GET", func(...string) any { return "pong" })

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- s.ServeContext(ctx) }()
	defer func() {
		cancel()
		if err := <-served; err != nil {
			t.Errorf("ServeContext = %v", err)
		}
	}()

	pool := x509.NewCertPool()
	pool.AddCert(cert)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}
	base := "localhost:" + strconv.Itoa(port) + "/ping"

	var resp *http.Response
	var err error
	for range 50 { // the listener may not be up yet
		if resp, err = client.Get("https://" + base); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "\"pong\"\n" {
		t.Fatalf("https: status = %d, body = %q", resp.StatusCode, body)
	}

	// Plain HTTP on the TLS port is refused.
	if resp, err := http.Get("http://" + base); err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Fatal("plain HTTP served on a TLS server")
		}
	}
}

func TestSetTLSRejectsBadFiles(t *testing.T) {
	certFile, keyFile, _ := selfSigned(t)
	otherCert, _, _ := selfSigned(t)
	s := NewLocalRegistryServer(0, "localhost", t.TempDir())

	for _, pair := range [][2]string{
		{filepath.Join(t.TempDir(), "missing.pem"), keyFile},
		{certFile, certFile}, // a certificate is not a key
		{otherCert, keyFile}, // key of another certificate
	} {
		if err := s.SetTLS(pair[0], pair[1]); err == nil {
			t.Fatalf("SetTLS(%s, %s): expected an error", pair[0], pair[1])
		}
	}
	if s.TLSEnabled() {
		t.Fatal("TLSEnabled = true after failed SetTLS calls")
	}
}

func TestServeContextRunsPurger(t *testing.T) {
	root := t.TempDir()
	s := NewLocalRegistryServer(freePort(t), "localhost", root)
	s.AddUploadRoutes(UploadConfig{Token: testutil.TestToken, StagingTTL: time.Hour})
	stale, _ := NewLayout(root).StagingDir("gitlab", "v1")
	testutil.WriteFile(t, filepath.Join(stale, "a"), "x")
	testutil.Age(t, filepath.Join(stale, "a"), 2*time.Hour)
	testutil.Age(t, stale, 2*time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.ServeContext(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(stale); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("stale staging was not purged at startup")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
