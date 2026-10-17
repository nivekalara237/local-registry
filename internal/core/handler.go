package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/nivekalara237/local-registry/internal/audit"
)

// ErrNotFound makes an AddRouteE handler answer 404. Errors wrapping
// fs.ErrNotExist do the same; an *HTTPError answers its own status; any other error answers 500.
var ErrNotFound = errors.New("not found")

// Registry is what every route shares: the router, the asset tree and the audit log.
type Registry struct {
	Mux    *http.ServeMux
	Layout Layout
	// Audit receives the upload routes' audit events; nil sends them to slog (see audit.AuditLog.Record).
	// It is read at each event, so it may be set after the routes are registered.
	Audit *audit.AuditLog
	// UploadMu serialises the moves into staging, the publish step and the staging purge,
	// so a file can never land in a version while (or after) it is published or purged.
	UploadMu sync.Mutex
}

// NewRegistry returns a Registry with no route, serving the asset tree under assetPath.
func NewRegistry(assetPath string) *Registry {
	return &Registry{Mux: http.NewServeMux(), Layout: NewLayout(assetPath)}
}

// HTTPError is an error whose status and message are sent to the client as is.
type HTTPError struct {
	Status int
	Msg    string
}

func (e *HTTPError) Error() string { return e.Msg }

// Errorf returns an *HTTPError: its status and message are sent to the client as is.
func Errorf(status int, format string, a ...any) error {
	return &HTTPError{Status: status, Msg: fmt.Sprintf(format, a...)}
}

// WriteJSON answers v as JSON with status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ErrorStatus is the status and client-safe message for err.
func ErrorStatus(err error) (int, string) {
	var he *HTTPError
	switch {
	case errors.As(err, &he):
		return he.Status, he.Msg
	case errors.Is(err, ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound, "not found"
	}
	return http.StatusInternalServerError, "internal error"
}

// WriteError answers err as {"error": "..."}. Only HTTPError messages reach the client;
// other errors may hold paths and are only logged.
func WriteError(w http.ResponseWriter, req *http.Request, err error) {
	status, msg := ErrorStatus(err)
	level := slog.LevelWarn
	if status >= 500 {
		level = slog.LevelError
	}
	slog.Log(req.Context(), level, "request failed", "method", req.Method, "path", req.URL.Path, "status", status, "error", err)
	WriteJSON(w, status, map[string]string{"error": msg})
}

var pathVarRe = regexp.MustCompile(`\{([^}]+)\}`)

func pattern(uri, method string) string {
	return fmt.Sprintf("%s /%s", strings.ToUpper(method), strings.TrimLeft(uri, "/"))
}

// AddRoute registers a JSON route; f receives the path values in the order they appear in uri.
func (s *Registry) AddRoute(uri, method string, f func(...string) any) {
	s.AddRouteE(uri, method, func(pvs ...string) (any, error) {
		return f(pvs...), nil
	})
}

// AddRouteE is AddRoute for handlers that can fail: the error becomes a JSON
// {"error": "..."} body: 404 for ErrNotFound and fs.ErrNotExist, an *HTTPError's own status, else 500.
func (s *Registry) AddRouteE(uri, method string, f func(...string) (any, error)) {
	var varNames []string
	for _, match := range pathVarRe.FindAllStringSubmatch(uri, -1) {
		varNames = append(varNames, match[1])
	}

	s.Mux.HandleFunc(pattern(uri, method), func(w http.ResponseWriter, req *http.Request) {
		values := make([]string, len(varNames))
		for i, name := range varNames {
			values[i] = req.PathValue(name)
		}

		res, err := f(values...)
		if err != nil {
			WriteError(w, req, err)
			return
		}
		WriteJSON(w, http.StatusOK, res)
	})
}

// AddFileRoute serves the file named by the path value filename, relative to the asset path.
// Only public files are served (see IsPublicFile); anything else, directories included, answers 404.
// The file is opened through an os.Root, so a symbolic link cannot lead outside the asset path.
func (s *Registry) AddFileRoute(uri, method, filename string) {
	s.Mux.HandleFunc(pattern(uri, method), func(writer http.ResponseWriter, req *http.Request) {
		root := s.Layout.Root()
		if uri == "" {
			pwd, err := os.Getwd()
			if err != nil {
				http.Error(writer, "internal error", http.StatusInternalServerError)
				return
			}
			root = pwd
		}

		// PathValue has already decoded the name once (%2F → /); it is not decoded again,
		// so "+" and "%" in a name mean themselves and a double-encoded path stays inert.
		name := strings.TrimLeft(req.PathValue(filename), "/")
		if !IsPublicFile(name) {
			http.NotFound(writer, req)
			return
		}
		ServeFile(writer, req, root, name)
	})
}

// ServeFile sends name, a slash-separated path relative to root, as an attachment.
// It is opened through an os.Root: a symbolic link cannot lead outside root.
// A missing file, a directory or any other non-regular file answers 404.
func ServeFile(w http.ResponseWriter, req *http.Request, root, name string) {
	r, err := os.OpenRoot(root)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	defer r.Close()
	fsys := r.FS()
	info, err := fs.Stat(fsys, name)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, req)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(name)}))
	http.ServeFileFS(w, req, fsys, name)
}
