package local_registry

import (
	"bufio"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultMaxFileSize caps one uploaded file when UploadConfig.MaxFileSize is not set.
	DefaultMaxFileSize int64 = 200 << 20
	// uploadTimeout replaces the default 20 s deadlines on upload routes.
	uploadTimeout = 30 * time.Minute
)

// UploadConfig enables the upload routes. For every limit, 0 means its Default* value and,
// except for MaxFileSize, a negative value disables it.
type UploadConfig struct {
	// Token is the shared secret expected as "Authorization: Bearer <token>".
	// Empty disables uploads: the routes answer 403.
	Token string
	// MaxFileSize caps one file in bytes; 0 or less means DefaultMaxFileSize.
	MaxFileSize int64
	// MaxVersionSize caps the bytes staged for one version (413 beyond).
	MaxVersionSize int64
	// MaxVersionFiles caps the files staged for one version (422 beyond).
	MaxVersionFiles int
	// MaxStagingSize caps the bytes staged for all versions together (507 beyond).
	MaxStagingSize int64
	// MinFreeSpace is the disk space, in bytes, an upload must leave free (507 otherwise).
	MinFreeSpace int64
	// StagingTTL is how long a staged version may sit untouched before it is purged.
	// The purge runs while ServeContext serves. It should exceed the 30 min upload timeout.
	StagingTTL time.Duration
}

// withDefaults replaces zero limits by their defaults.
func (cfg UploadConfig) withDefaults() UploadConfig {
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = DefaultMaxFileSize
	}
	if cfg.MaxVersionSize == 0 {
		cfg.MaxVersionSize = DefaultMaxVersionSize
	}
	if cfg.MaxVersionFiles == 0 {
		cfg.MaxVersionFiles = DefaultMaxVersionFiles
	}
	if cfg.MaxStagingSize == 0 {
		cfg.MaxStagingSize = DefaultMaxStagingSize
	}
	if cfg.MinFreeSpace == 0 {
		cfg.MinFreeSpace = DefaultMinFreeSpace
	}
	if cfg.StagingTTL == 0 {
		cfg.StagingTTL = DefaultStagingTTL
	}
	return cfg
}

// AddUploadRoutes registers the two-step upload of a release:
//
//	PUT  /{vcs}/artifacts/{version}/files/{name}   stream one file into staging
//	POST /{vcs}/artifacts/{version}/publish        check the staged files and publish them at once
//
// A published version is immutable: both routes answer 409 once it exists.
// Every accepted or refused request is recorded in the audit log (see SetAuditLog).
func (s *LocalRegistryServer) AddUploadRoutes(cfg UploadConfig) {
	cfg = cfg.withDefaults()
	s.uploadCfg = &cfg
	tid := ""
	if cfg.Token != "" {
		tid = TokenID(cfg.Token)
	}
	// refused audits and answers a request that failed after authentication.
	refused := func(w http.ResponseWriter, r *http.Request, event string, err error) {
		status, msg := errorStatus(err)
		if status >= 500 {
			msg = err.Error() // the audit log is private: keep the cause
		}
		s.audit(r, event, map[string]any{
			"vcs": r.PathValue("vcs"), "version": r.PathValue("version"), "name": r.PathValue("name"),
			"status": status, "error": msg, "token_id": tid,
		})
		writeError(w, r, err)
	}

	s.serverMux.HandleFunc("PUT /{vcs}/artifacts/{version}/files/{name}", func(w http.ResponseWriter, r *http.Request) {

		fmt.Println("**************************************")
		fmt.Println(r.Header)
		fmt.Println("**************************************")

		if err := authorize(cfg, r); err != nil {
			s.auditAuthFailure(r, err)
			writeAuthError(w, r, err)
			return
		}
		res, err := s.putFile(cfg, w, r)
		if err != nil {
			refused(w, r, "upload_refused", err)
			return
		}
		s.audit(r, "upload", map[string]any{
			"vcs": r.PathValue("vcs"), "version": r.PathValue("version"), "name": res.Name,
			"size": res.Size, "sha256": res.Sha256, "replaced": res.replaced, "token_id": tid,
		})
		writeJSON(w, http.StatusCreated, res)
	})
	s.serverMux.HandleFunc("POST /{vcs}/artifacts/{version}/publish", func(w http.ResponseWriter, r *http.Request) {
		if err := authorize(cfg, r); err != nil {
			s.auditAuthFailure(r, err)
			writeAuthError(w, r, err)
			return
		}
		res, err := s.publish(w, r)
		if err != nil {
			refused(w, r, "publish_refused", err)
			return
		}
		s.audit(r, "publish", map[string]any{
			"vcs": res.VCS, "version": res.Version, "latest": res.Latest, "files": res.records, "token_id": tid,
		})
		writeJSON(w, http.StatusOK, res)
	})
}

// auditAuthFailure records a refused authentication; the token sent is never recorded.
func (s *LocalRegistryServer) auditAuthFailure(r *http.Request, err error) {
	status, msg := errorStatus(err)
	s.audit(r, "auth_failed", map[string]any{
		"method": r.Method, "path": r.URL.Path, "status": status, "error": msg,
		"authorization_sent": r.Header.Get("Authorization") != "",
	})
}

func authorize(cfg UploadConfig, r *http.Request) error {
	if cfg.Token == "" {
		return httpError(http.StatusForbidden, "upload is disabled: no token configured")
	}
	got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(cfg.Token)) != 1 {
		return httpError(http.StatusUnauthorized, "invalid or missing bearer token")
	}
	return nil
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	var he *HTTPError
	if errors.As(err, &he) && he.Status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	writeError(w, r, err)
}

// extendDeadlines lets a long upload outlive the default per-request write deadline.
func extendDeadlines(w http.ResponseWriter) {
	rc := http.NewResponseController(w)
	deadline := time.Now().Add(uploadTimeout)
	_ = rc.SetReadDeadline(deadline)
	_ = rc.SetWriteDeadline(deadline)
}

// target validates vcs and version and returns the published and staging directories.
func (s *LocalRegistryServer) target(r *http.Request) (vcs, version, final, staging string, err error) {
	vcs, version = r.PathValue("vcs"), r.PathValue("version")
	if !ValidName(vcs) || !ValidName(version) || version == "latest" {
		return "", "", "", "", httpError(http.StatusBadRequest, "invalid vcs or version name")
	}
	l := NewLayout(s.absoluteAssetPath)
	if final, err = l.VersionDir(vcs, version); err != nil {
		return "", "", "", "", err
	}
	if staging, err = l.StagingDir(vcs, version); err != nil {
		return "", "", "", "", err
	}
	return vcs, version, final, staging, nil
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func errPublished(vcs, version string) error {
	return httpError(http.StatusConflict, "version %s/%s is already published and cannot be changed", vcs, version)
}

type putResult struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Sha256 string `json:"sha256"`

	replaced bool // a staged file of the same name was overwritten
}

func (s *LocalRegistryServer) putFile(cfg UploadConfig, w http.ResponseWriter, r *http.Request) (*putResult, error) {
	vcs, version, final, staging, err := s.target(r)
	if err != nil {
		return nil, err
	}
	name := r.PathValue("name")
	if !ValidName(name) {
		return nil, httpError(http.StatusBadRequest, "invalid file name: use [A-Za-z0-9._-] only")
	}
	if published, err := exists(final); err != nil || published {
		if err != nil {
			return nil, err
		}
		return nil, errPublished(vcs, version)
	}
	b, err := s.uploadBudget(cfg, vcs, version, staging, name)
	if err != nil {
		return nil, err
	}
	if r.ContentLength > b.limit {
		return nil, b.tooLarge
	}
	tmpDir := NewLayout(s.absoluteAssetPath).UploadTmpDir()
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, err
	}
	need := r.ContentLength
	if need < 0 { // chunked: assume the worst the budget allows
		need = b.limit
	}
	if err := checkFreeSpace(cfg, tmpDir, need); err != nil {
		return nil, err
	}
	extendDeadlines(w)

	tmp, err := os.CreateTemp(tmpDir, "upload-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed into staging
	defer tmp.Close()

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, h), http.MaxBytesReader(w, r.Body, b.limit))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, b.tooLarge
		}
		return nil, fmt.Errorf("reading upload body: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if want := r.Header.Get("X-Checksum-Sha256"); want != "" && !strings.EqualFold(want, sum) {
		return nil, httpError(http.StatusUnprocessableEntity, "sha256 mismatch: got %s, X-Checksum-Sha256 says %s", sum, want)
	}

	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	if published, err := exists(final); err != nil || published {
		if err != nil {
			return nil, err
		}
		return nil, errPublished(vcs, version)
	}
	// Other uploads may have landed while this one streamed: check the quotas again, for good.
	if b, err = s.uploadBudget(cfg, vcs, version, staging, name); err != nil {
		return nil, err
	}
	if size > b.limit {
		return nil, b.tooLarge
	}
	dst := filepath.Join(staging, name)
	replaced, err := exists(dst)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return nil, err
	}
	return &putResult{Name: name, Size: size, Sha256: sum, replaced: replaced}, nil
}

type publishResult struct {
	VCS     string   `json:"vcs"`
	Version string   `json:"version"`
	Files   []string `json:"files"`
	Latest  bool     `json:"latest"`

	records []fileRecord // for the audit log
}

// fileRecord is one published file as the audit log keeps it.
type fileRecord struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Sha256 string `json:"sha256"`
}

func (s *LocalRegistryServer) publish(w http.ResponseWriter, r *http.Request) (*publishResult, error) {
	vcs, version, final, staging, err := s.target(r)
	if err != nil {
		return nil, err
	}
	latest := false
	if v := r.URL.Query().Get("latest"); v != "" {
		if latest, err = strconv.ParseBool(v); err != nil {
			return nil, httpError(http.StatusBadRequest, "latest must be true or false")
		}
	}
	extendDeadlines(w) // hashing every staged file can take a while

	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()

	if published, err := exists(final); err != nil || published {
		if err != nil {
			return nil, err
		}
		return nil, errPublished(vcs, version)
	}
	entries, err := os.ReadDir(staging)
	if errors.Is(err, os.ErrNotExist) {
		return nil, httpError(http.StatusNotFound, "nothing uploaded for %s/%s", vcs, version)
	}
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.Type().IsRegular() {
			files = append(files, e.Name())
		}
	}
	sums, err := checkRelease(staging, version, files)
	if err != nil {
		return nil, err
	}
	records := make([]fileRecord, 0, len(files))
	for _, f := range files {
		info, err := os.Stat(filepath.Join(staging, f))
		if err != nil {
			return nil, err
		}
		records = append(records, fileRecord{Name: f, Size: info.Size(), Sha256: sums[f]})
	}

	vcsDir := filepath.Dir(final)
	if err := os.MkdirAll(vcsDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.Rename(staging, final); err != nil {
		return nil, err
	}
	if latest {
		if err := copyAtomic(filepath.Join(final, "artifacts.json"), filepath.Join(vcsDir, "artifacts.json")); err != nil {
			return nil, fmt.Errorf("version published but latest not updated: %w", err)
		}
	}
	return &publishResult{VCS: vcs, Version: version, Files: files, Latest: latest, records: records}, nil
}

// unchecked files carry no line in the checksums file: the metadata and signatures of the checksums.
func unchecked(name string) bool {
	return name == "artifacts.json" || name == "metadata.json" ||
		strings.HasSuffix(name, ".sig") || strings.HasSuffix(name, ".pem")
}

// checkRelease refuses (422) a staged release that goreleaser would not have produced whole.
// It returns the sha256 of every file.
func checkRelease(dir, version string, files []string) (map[string]string, error) {
	invalid := func(format string, a ...any) error {
		return httpError(http.StatusUnprocessableEntity, format, a...)
	}

	var checksumsFile string
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), "checksums.txt") {
			if checksumsFile != "" {
				return nil, invalid("several checksums files: %s and %s", checksumsFile, f)
			}
			checksumsFile = f
		}
	}
	if checksumsFile == "" {
		return nil, invalid("no checksums file (*checksums.txt) uploaded")
	}
	for _, required := range []string{"artifacts.json", "metadata.json"} {
		if !slices.Contains(files, required) {
			return nil, invalid("%s not uploaded", required)
		}
	}

	listed, err := readChecksums(filepath.Join(dir, checksumsFile))
	if err != nil {
		return nil, invalid("%s: %v", checksumsFile, err)
	}
	sums := make(map[string]string, len(files))
	for _, f := range files {
		got, err := fileSha256(filepath.Join(dir, f))
		if err != nil {
			return nil, err
		}
		sums[f] = got
		if f == checksumsFile || unchecked(f) {
			continue
		}
		want, ok := listed[f]
		if !ok {
			return nil, invalid("%s is not listed in %s", f, checksumsFile)
		}
		if !strings.EqualFold(got, want) {
			return nil, invalid("%s: sha256 is %s but %s says %s", f, got, checksumsFile, want)
		}
	}

	artifacts, err := os.ReadFile(filepath.Join(dir, "artifacts.json"))
	if err != nil {
		return nil, err
	}
	if !json.Valid(artifacts) {
		return nil, invalid("artifacts.json is not valid JSON")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return nil, err
	}
	var meta struct {
		Tag     string `json:"tag"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, invalid("metadata.json is not valid JSON")
	}
	if meta.Tag == "" && meta.Version == "" {
		return nil, invalid("metadata.json has neither tag nor version")
	}
	if version != meta.Tag && version != meta.Version && version != "v"+meta.Version {
		return nil, invalid("metadata.json describes %s (version %s), not %s", meta.Tag, meta.Version, version)
	}
	return sums, nil
}

// readChecksums parses "<sha256>  <name>" lines, as written by goreleaser and sha256sum.
func readChecksums(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sums := map[string]string{}
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[0]) != sha256.Size*2 {
			return nil, fmt.Errorf("line %d is not \"<sha256>  <name>\"", n)
		}
		sums[strings.TrimPrefix(fields[1], "*")] = fields[0]
	}
	return sums, sc.Err()
}

func fileSha256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyAtomic copies src over dst through a temp file and a rename, so readers never see half a file.
func copyAtomic(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}
