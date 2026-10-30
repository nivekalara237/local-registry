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
	"syscall"
	"time"

	"github.com/nivekalara237/local-registry/internal/audit"
	"github.com/nivekalara237/local-registry/internal/core"
	"github.com/nivekalara237/local-registry/internal/endpoints"
	regfs "github.com/nivekalara237/local-registry/internal/fs"
)

const (
	// defaultWriteTimeout bounds each response; a route that needs longer (upload)
	// moves its own deadlines with http.ResponseController.
	defaultWriteTimeout = 20 * time.Second
	shutdownTimeout     = 10 * time.Second
)

type LocalRegistryServer struct {
	port    int
	address string
	// reg holds the routes, the asset tree and the audit log every route shares.
	reg *core.Registry

	// tlsConfig, when set by SetTLS, makes the server speak HTTPS only.
	tlsConfig *tls.Config
	// uploadCfg is set by AddUploadRoutes; ServeContext purges stale uploads with it.
	uploadCfg *UploadConfig
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
		port:    port,
		address: addr,
		reg:     core.NewRegistry(absoluteAssetPath),
	}
}

// Handler returns the routes registered so far, without logging or listening.
func (s *LocalRegistryServer) Handler() http.Handler {
	return s.reg.Mux
}

// SetAuditLog sends audit events to a. Without one, they go to the standard log (slog) only.
func (s *LocalRegistryServer) SetAuditLog(a *AuditLog) {
	s.reg.Audit = a
}

// AddRoute registers a JSON route; f receives the path values in the order they appear in uri.
func (s *LocalRegistryServer) AddRoute(uri, method string, f func(...string) any) {
	s.reg.AddRoute(uri, method, f)
}

// AddRouteE is AddRoute for handlers that can fail: the error becomes a JSON
// {"error": "..."} body: 404 for ErrNotFound and fs.ErrNotExist, an *HTTPError's own status, else 500.
func (s *LocalRegistryServer) AddRouteE(uri, method string, f func(...string) (any, error)) {
	s.reg.AddRouteE(uri, method, f)
}

// AddFileRoute serves the file named by the path value filename, relative to the asset path.
// Only public files are served (see IsPublicFile); anything else, directories included, answers 404.
// The file is opened through an os.Root, so a symbolic link cannot lead outside the asset path.
func (s *LocalRegistryServer) AddFileRoute(uri, method, filename string) {
	s.reg.AddFileRoute(uri, method, filename)
}

// AddUploadRoutes registers the two-step upload of a release, and the download of its files:
//
//	PUT  /{product}/artifacts/{version}/files/{name}   stream one file into staging
//	POST /{product}/artifacts/{version}/publish        check the staged files and publish them at once
//	GET  /{product}/artifacts/{version}/files/{name}   download a file of a published version
//
// artifacts.json must be uploaded last: it is refused while a file it lists is missing.
// Publishing rewrites it so that each "path" is the file's GET URL, and keeps the uploaded
// one as artifacts.goreleaser.json.
// A published version is immutable: PUT and POST answer 409 once it exists.
// Every accepted or refused request is recorded in the audit log (see SetAuditLog).
func (s *LocalRegistryServer) AddUploadRoutes(cfg UploadConfig) {
	cfg = endpoints.AddUploadRoutes(s.reg, cfg)
	s.uploadCfg = &cfg
}

// AddReleaseRoutes registers the read-only release routes:
//
//	POST /releases/{product}/{version}            files of a version, with their download URL
//	POST /releases/{version}                  deprecated: the same for product "gitlab"
//	GET  /artifacts/metadatas/{product}?version=  metadata.json of a version (default: latest)
//
// publicURL, when set, makes the download URLs absolute (see UploadConfig.PublicURL).
func (s *LocalRegistryServer) AddReleaseRoutes(publicURL string) {
	endpoints.AddReleaseRoutes(s.reg, publicURL)
}

// AddProductRoutes registers the read-only routes of the local-registry/v1 standard:
//
//	GET /products                        every product, with its latest version
//	GET /{product}/index                 index of a product and its versions
//	GET /{product}/releases/{version}    release.json of a version ("latest" allowed)
func (s *LocalRegistryServer) AddProductRoutes() {
	endpoints.AddProductRoutes(s.reg)
}

func (s *LocalRegistryServer) AddProductSchema() {
	endpoints.AddProductSchemaRoutes(s.reg)
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
		audit.ClientFields(r, client)
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
		Handler:           logMiddleware(s.reg.Mux),
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
	fmt.Println("Assets Path : " + s.reg.Layout.Root())
	fmt.Println()

	if s.uploadCfg != nil && s.uploadCfg.StagingTTL > 0 {
		purgeCtx, stopPurge := context.WithCancel(ctx)
		defer stopPurge()
		go regfs.RunPurger(purgeCtx, s.reg, s.uploadCfg.StagingTTL)
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
