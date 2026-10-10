package local_registry

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
)

// ErrNotFound makes an AddRouteE handler answer 404. Errors wrapping
// fs.ErrNotExist do the same; an *HTTPError answers its own status; any other error answers 500.
var ErrNotFound = errors.New("not found")

// HTTPError is an error whose status and message are sent to the client as is.
type HTTPError struct {
	Status int
	Msg    string
}

func (e *HTTPError) Error() string { return e.Msg }

func httpError(status int, format string, a ...any) error {
	return &HTTPError{Status: status, Msg: fmt.Sprintf(format, a...)}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// errorStatus is the status and client-safe message for err.
func errorStatus(err error) (int, string) {
	var he *HTTPError
	switch {
	case errors.As(err, &he):
		return he.Status, he.Msg
	case errors.Is(err, ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound, "not found"
	}
	return http.StatusInternalServerError, "internal error"
}

// writeError answers err as {"error": "..."}. Only HTTPError messages reach the client;
// other errors may hold paths and are only logged.
func writeError(w http.ResponseWriter, req *http.Request, err error) {
	status, msg := errorStatus(err)
	level := slog.LevelWarn
	if status >= 500 {
		level = slog.LevelError
	}
	slog.Log(req.Context(), level, "request failed", "method", req.Method, "path", req.URL.Path, "status", status, "error", err)
	writeJSON(w, status, map[string]string{"error": msg})
}

var pathVarRe = regexp.MustCompile(`\{([^}]+)\}`)

func pattern(uri, method string) string {
	return fmt.Sprintf("%s /%s", strings.ToUpper(method), strings.TrimLeft(uri, "/"))
}

// AddRoute registers a JSON route; f receives the path values in the order they appear in uri.
func (s *LocalRegistryServer) AddRoute(uri, method string, f func(...string) any) {
	s.AddRouteE(uri, method, func(pvs ...string) (any, error) {
		return f(pvs...), nil
	})
}

// AddRouteE is AddRoute for handlers that can fail: the error becomes a JSON
// {"error": "..."} body: 404 for ErrNotFound and fs.ErrNotExist, an *HTTPError's own status, else 500.
func (s *LocalRegistryServer) AddRouteE(uri, method string, f func(...string) (any, error)) {
	var varNames []string
	for _, match := range pathVarRe.FindAllStringSubmatch(uri, -1) {
		varNames = append(varNames, match[1])
	}

	s.serverMux.HandleFunc(pattern(uri, method), func(w http.ResponseWriter, req *http.Request) {
		values := make([]string, len(varNames))
		for i, name := range varNames {
			values[i] = req.PathValue(name)
		}

		res, err := f(values...)
		if err != nil {
			writeError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	})
}

// AddFileRoute serves the file named by the path value filename, relative to the asset path.
// Only public files are served (see IsPublicFile); anything else, directories included, answers 404.
// The file is opened through an os.Root, so a symbolic link cannot lead outside the asset path.
func (s *LocalRegistryServer) AddFileRoute(uri, method, filename string) {
	s.serverMux.HandleFunc(pattern(uri, method), func(writer http.ResponseWriter, req *http.Request) {
		root := s.absoluteAssetPath
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
		serveFile(writer, req, root, name)
	})
}

// serveFile sends name, a slash-separated path relative to root, as an attachment.
// It is opened through an os.Root: a symbolic link cannot lead outside root.
// A missing file, a directory or any other non-regular file answers 404.
func serveFile(w http.ResponseWriter, req *http.Request, root, name string) {
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
