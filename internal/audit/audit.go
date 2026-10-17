package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditLog appends one JSON object per line for each security-relevant event of the upload
// routes: accepted and refused uploads, publications with the sha256 of every file, failed
// authentications and purges. Each line is synced to disk before the request is answered.
type AuditLog struct {
	mu sync.Mutex
	f  *os.File
}

// OpenAuditLog opens path for appending, creating it (mode 0600) and its directory if needed.
func OpenAuditLog(path string) (*AuditLog, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("audit log: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("audit log: %w", err)
	}
	return &AuditLog{f: f}, nil
}

func (a *AuditLog) Close() error {
	return a.f.Close()
}

func (a *AuditLog) write(entry map[string]any) error {
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, err := a.f.Write(append(line, '\n')); err != nil {
		return err
	}
	return a.f.Sync()
}

// TokenID identifies a token in the audit log without revealing it: the first 12 hex digits of its sha256.
func TokenID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:6])
}

// ClientFields describes who sent r: the TCP peer, and X-Forwarded-For as sent (not trusted).
func ClientFields(r *http.Request, into map[string]any) {
	remote := r.RemoteAddr
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	into["remote"] = remote
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		into["forwarded_for"] = fwd
	}
}

// Record writes event with fields; r, when not nil, adds the client's address.
// On a nil AuditLog, the event goes to the standard log (slog) only.
// A failure to write the audit log is logged as an error and does not fail the request.
func (a *AuditLog) Record(r *http.Request, event string, fields map[string]any) {
	entry := map[string]any{"time": time.Now().UTC().Format(time.RFC3339Nano), "event": event}
	for k, v := range fields {
		entry[k] = v
	}
	if r != nil {
		ClientFields(r, entry)
	}
	if a == nil {
		attrs := make([]any, 0, 2*len(entry))
		for k, v := range entry {
			if k != "time" {
				attrs = append(attrs, k, v)
			}
		}
		slog.Info("audit", attrs...)
		return
	}
	if err := a.write(entry); err != nil {
		slog.Error("audit log write failed", "event", event, "error", err)
	}
}
