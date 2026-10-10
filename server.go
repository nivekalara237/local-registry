package local_registry

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	// defaultWriteTimeout bounds each response; a route that needs longer (upload)
	// moves its own deadlines with http.ResponseController.
	defaultWriteTimeout = 20 * time.Second
	shutdownTimeout     = 10 * time.Second
)

type LocalRegistryServer struct {
	port              int
	address           string
	serverMux         *http.ServeMux
	absoluteAssetPath string

	// uploadMu serialises the moves into staging and the publish step, so a file
	// can never land in a version while (or after) it is published.
	uploadMu sync.Mutex

	// tlsConfig, when set by SetTLS, makes the server speak HTTPS only.
	tlsConfig *tls.Config
	// uploadCfg is set by AddUploadRoutes; ServeContext purges stale uploads with it.
	uploadCfg *UploadConfig
	// auditLog, when set by SetAuditLog, receives the upload routes' audit events.
	auditLog *AuditLog
}

// SetTLS makes the server speak HTTPS only, with the PEM certificate (chain) and key in the
// two files. The pair is loaded now, so a bad file fails here rather than at the first client.
func (s *LocalRegistryServer) SetTLS(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("loading TLS certificate: %w", err)
	}
	s.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	return nil
}

// TLSEnabled reports whether SetTLS succeeded.
func (s *LocalRegistryServer) TLSEnabled() bool {
	return s.tlsConfig != nil
}

func NewLocalRegistryServer(port int, addr, absoluteAssetPath string) *LocalRegistryServer {
	return &LocalRegistryServer{
		port:              port,
		address:           addr,
		serverMux:         http.NewServeMux(),
		absoluteAssetPath: absoluteAssetPath,
	}
}

// Handler returns the routes registered so far, without logging or listening.
func (s *LocalRegistryServer) Handler() http.Handler {
	return s.serverMux
}

// statusRecorder captures the status and size of a response for the request log.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying connection.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		_ = http.NewResponseController(rec).SetWriteDeadline(start.Add(defaultWriteTimeout))

		next.ServeHTTP(rec, r)

		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		client := map[string]any{}
		clientFields(r, client)
		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"bytes", rec.bytes,
			"duration", time.Since(start),
			"remote", client["remote"],
		}
		if fwd, ok := client["forwarded_for"]; ok {
			attrs = append(attrs, "forwarded_for", fwd)
		}
		slog.Info("request", attrs...)
	})
}

// Serve listens until SIGINT or SIGTERM, then shuts down gracefully.
// It exits the process with status 1 if the address cannot be bound or the server fails.
func (s *LocalRegistryServer) Serve() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := s.ServeContext(ctx); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// ServeContext binds the address first, so a busy port fails immediately, then
// serves until ctx is done and drains in-flight requests for up to 10 seconds.
func (s *LocalRegistryServer) ServeContext(ctx context.Context) error {
	ln, err := net.Listen("tcp", net.JoinHostPort(s.address, strconv.Itoa(s.port)))
	if err != nil {
		return err
	}

	server := &http.Server{
		Handler:           logMiddleware(s.serverMux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		TLSConfig:         s.tlsConfig,
	}
	scheme := "http"
	if s.tlsConfig != nil {
		scheme = "https"
	}

	fmt.Println("##################################################")
	fmt.Println("##          Local Registry Server               ##")
	fmt.Println("##################################################")
	fmt.Println("")
	fmt.Printf("Listen      : %s://%s\n", scheme, ln.Addr())
	fmt.Println("Start At    : " + time.Now().String())
	fmt.Println("Assets Path : " + s.absoluteAssetPath)
	fmt.Println()

	if s.uploadCfg != nil && s.uploadCfg.StagingTTL > 0 {
		purgeCtx, stopPurge := context.WithCancel(ctx)
		defer stopPurge()
		go s.runPurger(purgeCtx, s.uploadCfg.StagingTTL)
	}

	errc := make(chan error, 1)
	go func() {
		if s.tlsConfig != nil {
			errc <- server.ServeTLS(ln, "", "") // certificates come from TLSConfig
			return
		}
		errc <- server.Serve(ln)
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
