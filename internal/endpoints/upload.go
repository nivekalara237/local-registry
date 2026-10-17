package endpoints

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

	"github.com/nivekalara237/local-registry/internal/audit"
	"github.com/nivekalara237/local-registry/internal/core"
	regfs "github.com/nivekalara237/local-registry/internal/fs"
)

const (
	// DefaultMaxFileSize caps one uploaded file when UploadConfig.MaxFileSize is not set.
	DefaultMaxFileSize int64 = 200 << 20
	// uploadTimeout replaces the default 20 s deadlines on upload routes.
	uploadTimeout = 30 * time.Minute
)

// UploadConfig enables the upload routes. For every limit, 0 means its Default* value and,
// except for MaxFileSize, a negative value disables it. The Default* values of the staging
// limits are in internal/fs.
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
	// PublicURL is the server's address as clients reach it, such as https://registry.example.com
	// behind a reverse proxy. Publishing writes it before each download path in artifacts.json;
	// empty, the paths stay relative (/{vcs}/artifacts/{version}/files/{name}).
	PublicURL string
}

// WithDefaults replaces zero limits by their defaults.
func (cfg UploadConfig) WithDefaults() UploadConfig {
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = DefaultMaxFileSize
	}
	if cfg.MaxVersionSize == 0 {
		cfg.MaxVersionSize = regfs.DefaultMaxVersionSize
	}
	if cfg.MaxVersionFiles == 0 {
		cfg.MaxVersionFiles = regfs.DefaultMaxVersionFiles
	}
	if cfg.MaxStagingSize == 0 {
		cfg.MaxStagingSize = regfs.DefaultMaxStagingSize
	}
	if cfg.MinFreeSpace == 0 {
		cfg.MinFreeSpace = regfs.DefaultMinFreeSpace
	}
	if cfg.StagingTTL == 0 {
		cfg.StagingTTL = regfs.DefaultStagingTTL
	}
	return cfg
}

// limits are the staging quotas of cfg, as the staging budget applies them.
func (cfg UploadConfig) limits() regfs.Limits {
	return regfs.Limits{
		MaxFileSize:     cfg.MaxFileSize,
		MaxVersionSize:  cfg.MaxVersionSize,
		MaxVersionFiles: cfg.MaxVersionFiles,
		MaxStagingSize:  cfg.MaxStagingSize,
	}
}

// uploader serves the upload routes of one registry with one configuration.
type uploader struct {
	reg *core.Registry
	cfg UploadConfig
}

// AddUploadRoutes registers the two-step upload of a release, and the download of its files:
//
//	PUT  /{vcs}/artifacts/{version}/files/{name}   stream one file into staging
//	POST /{vcs}/artifacts/{version}/publish        check the staged files and publish them at once
//	GET  /{vcs}/artifacts/{version}/files/{name}   download a file of a published version
//
// artifacts.json must be uploaded last: it is refused while a file it lists is missing.
// Publishing rewrites it so that each "path" is the file's GET URL, and keeps the uploaded
// one as artifacts.goreleaser.json.
// A published version is immutable: PUT and POST answer 409 once it exists.
// Every accepted or refused request is recorded in reg.Audit.
// It returns cfg with its defaults applied.
func AddUploadRoutes(reg *core.Registry, cfg UploadConfig) UploadConfig {
	cfg = cfg.WithDefaults()
	s := &uploader{reg: reg, cfg: cfg}
	tid := ""
	if cfg.Token != "" {
		tid = audit.TokenID(cfg.Token)
	}
	// refused audits and answers a request that failed after authentication.
	refused := func(w http.ResponseWriter, r *http.Request, event string, err error) {
		status, msg := core.ErrorStatus(err)
		if status >= 500 {
			msg = err.Error() // the audit log is private: keep the cause
		}
		s.reg.Audit.Record(r, event, map[string]any{
			"vcs": r.PathValue("vcs"), "version": r.PathValue("version"), "name": r.PathValue("name"),
			"status": status, "error": msg, "token_id": tid,
		})
		core.WriteError(w, r, err)
	}

	reg.Mux.HandleFunc("PUT /{vcs}/artifacts/{version}/files/{name}", func(w http.ResponseWriter, r *http.Request) {
		if err := authorize(cfg, r); err != nil {
			s.auditAuthFailure(r, err)
			writeAuthError(w, r, err)
			return
		}
		res, err := s.putFile(w, r)
		if err != nil {
			refused(w, r, "upload_refused", err)
			return
		}
		s.reg.Audit.Record(r, "upload", map[string]any{
			"vcs": r.PathValue("vcs"), "version": r.PathValue("version"), "name": res.Name,
			"size": res.Size, "sha256": res.Sha256, "replaced": res.replaced, "token_id": tid,
		})
		core.WriteJSON(w, http.StatusCreated, res)
	})
	reg.Mux.HandleFunc("POST /{vcs}/artifacts/{version}/publish", func(w http.ResponseWriter, r *http.Request) {
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
		s.reg.Audit.Record(r, "publish", map[string]any{
			"vcs": res.VCS, "version": res.Version, "latest": res.Latest, "files": res.records, "token_id": tid,
		})
		core.WriteJSON(w, http.StatusOK, res)
	})
	reg.Mux.HandleFunc("GET /{vcs}/artifacts/{version}/files/{name}", serveReleaseFile(reg.Layout))
	return cfg
}

// auditAuthFailure records a refused authentication; the token sent is never recorded.
func (s *uploader) auditAuthFailure(r *http.Request, err error) {
	status, msg := core.ErrorStatus(err)
	s.reg.Audit.Record(r, "auth_failed", map[string]any{
		"method": r.Method, "path": r.URL.Path, "status": status, "error": msg,
		"authorization_sent": r.Header.Get("Authorization") != "",
	})
}

func authorize(cfg UploadConfig, r *http.Request) error {
	if cfg.Token == "" {
		return core.Errorf(http.StatusForbidden, "upload is disabled: no token configured")
	}
	got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(cfg.Token)) != 1 {
		return core.Errorf(http.StatusUnauthorized, "invalid or missing bearer token")
	}
	return nil
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	var he *core.HTTPError
	if errors.As(err, &he) && he.Status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	core.WriteError(w, r, err)
}

// extendDeadlines lets a long upload outlive the default per-request write deadline.
func extendDeadlines(w http.ResponseWriter) {
	rc := http.NewResponseController(w)
	deadline := time.Now().Add(uploadTimeout)
	_ = rc.SetReadDeadline(deadline)
	_ = rc.SetWriteDeadline(deadline)
}

// target validates vcs and version and returns the published and staging directories.
func (s *uploader) target(r *http.Request) (vcs, version, final, staging string, err error) {
	vcs, version = r.PathValue("vcs"), r.PathValue("version")
	if !core.ValidName(vcs) || !core.ValidName(version) || version == "latest" {
		return "", "", "", "", core.Errorf(http.StatusBadRequest, "invalid vcs or version name")
	}
	l := s.reg.Layout
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
	return core.Errorf(http.StatusConflict, "version %s/%s is already published and cannot be changed", vcs, version)
}

type putResult struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Sha256 string `json:"sha256"`

	replaced bool // a staged file of the same name was overwritten
}

func (s *uploader) putFile(w http.ResponseWriter, r *http.Request) (*putResult, error) {
	cfg := s.cfg
	vcs, version, final, staging, err := s.target(r)
	if err != nil {
		return nil, err
	}
	name := r.PathValue("name")
	if !core.ValidName(name) {
		return nil, core.Errorf(http.StatusBadRequest, "invalid file name: use [A-Za-z0-9._-] only")
	}
	if name == originalArtifactsFile {
		return nil, core.Errorf(http.StatusBadRequest, "%s is reserved: publishing writes it", name)
	}
	if published, err := exists(final); err != nil || published {
		if err != nil {
			return nil, err
		}
		return nil, errPublished(vcs, version)
	}
	b, err := regfs.UploadBudget(cfg.limits(), s.reg.Layout, vcs, version, staging, name)
	if err != nil {
		return nil, err
	}
	if r.ContentLength > b.Limit {
		return nil, b.TooLarge
	}
	tmpDir := s.reg.Layout.UploadTmpDir()
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, err
	}
	need := r.ContentLength
	if need < 0 { // chunked: assume the worst the budget allows
		need = b.Limit
	}
	if err := regfs.CheckFreeSpace(cfg.MinFreeSpace, tmpDir, need); err != nil {
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
	size, err := io.Copy(io.MultiWriter(tmp, h), http.MaxBytesReader(w, r.Body, b.Limit))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, b.TooLarge
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
		return nil, core.Errorf(http.StatusUnprocessableEntity, "sha256 mismatch: got %s, X-Checksum-Sha256 says %s", sum, want)
	}

	s.reg.UploadMu.Lock()
	defer s.reg.UploadMu.Unlock()
	if published, err := exists(final); err != nil || published {
		if err != nil {
			return nil, err
		}
		return nil, errPublished(vcs, version)
	}
	// Other uploads may have landed while this one streamed: check the quotas again, for good.
	if b, err = regfs.UploadBudget(cfg.limits(), s.reg.Layout, vcs, version, staging, name); err != nil {
		return nil, err
	}
	if size > b.Limit {
		return nil, b.TooLarge
	}
	if name == artifactsFile {
		if err := checkUploadedArtifacts(tmp.Name(), staging, size); err != nil {
			return nil, err
		}
		// Left by a publish that failed after its rewrite: it would shadow this upload.
		if err := os.Remove(filepath.Join(staging, originalArtifactsFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
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

// checkUploadedArtifacts checks an artifacts.json being uploaded to staging: every file it
// publishes must already be there, with its sha256.
func checkUploadedArtifacts(file, staging string, size int64) error {
	if size > maxArtifactsSize {
		return unprocessable("artifacts.json is larger than %d MiB", maxArtifactsSize>>20)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	_, err = checkArtifacts(staging, data, nil)
	return err
}

func (s *uploader) publish(w http.ResponseWriter, r *http.Request) (*publishResult, error) {
	cfg := s.cfg
	vcs, version, final, staging, err := s.target(r)
	if err != nil {
		return nil, err
	}
	latest := false
	if v := r.URL.Query().Get("latest"); v != "" {
		if latest, err = strconv.ParseBool(v); err != nil {
			return nil, core.Errorf(http.StatusBadRequest, "latest must be true or false")
		}
	}
	extendDeadlines(w) // hashing every staged file can take a while

	s.reg.UploadMu.Lock()
	defer s.reg.UploadMu.Unlock()

	if published, err := exists(final); err != nil || published {
		if err != nil {
			return nil, err
		}
		return nil, errPublished(vcs, version)
	}
	entries, err := os.ReadDir(staging)
	if errors.Is(err, os.ErrNotExist) {
		return nil, core.Errorf(http.StatusNotFound, "nothing uploaded for %s/%s", vcs, version)
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
	if files, err = rewriteStagedArtifacts(cfg, staging, vcs, version, files, sums); err != nil {
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
	return name == artifactsFile || name == originalArtifactsFile || name == "metadata.json" ||
		strings.HasSuffix(name, ".sig") || strings.HasSuffix(name, ".pem")
}

// checkRelease refuses (422) a staged release that goreleaser would not have produced whole.
// It returns the sha256 of every file.
func checkRelease(dir, version string, files []string) (map[string]string, error) {
	invalid := func(format string, a ...any) error {
		return core.Errorf(http.StatusUnprocessableEntity, format, a...)
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

// rewriteStagedArtifacts checks the staged artifacts.json against the staged files, then
// replaces it by its rewritten form and keeps the uploaded one as artifacts.goreleaser.json.
// It returns the staged file names and updates sums to match.
func rewriteStagedArtifacts(cfg UploadConfig, staging, vcs, version string, files []string, sums map[string]string) ([]string, error) {
	// An artifacts.goreleaser.json in staging was left by a publish that failed after its
	// rewrite (uploading artifacts.json removes it): it is the uploaded artifacts.json.
	original := filepath.Join(staging, originalArtifactsFile)
	source := filepath.Join(staging, artifactsFile)
	hadOriginal := slices.Contains(files, originalArtifactsFile)
	if hadOriginal {
		source = original
	}
	if info, err := os.Stat(source); err != nil {
		return nil, err
	} else if info.Size() > maxArtifactsSize {
		return nil, unprocessable("artifacts.json is larger than %d MiB", maxArtifactsSize>>20)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, err
	}
	published, err := checkArtifacts(staging, data, sums)
	if err != nil {
		return nil, err
	}
	listed := map[string]bool{artifactsFile: true, originalArtifactsFile: true, "metadata.json": true}
	for _, a := range published {
		listed[a.Name] = true
	}
	for _, f := range files {
		if !listed[f] {
			return nil, unprocessable("%s is uploaded but artifacts.json does not list it", f)
		}
	}

	rewritten, err := rewriteArtifacts(published, releaseFilesURL(cfg.PublicURL, vcs, version))
	if err != nil {
		return nil, err
	}
	if !hadOriginal {
		if err := writeAtomic(original, data); err != nil {
			return nil, err
		}
		files = append(files, originalArtifactsFile)
		slices.Sort(files)
	}
	if err := writeAtomic(filepath.Join(staging, artifactsFile), rewritten); err != nil {
		return nil, err
	}
	sums[originalArtifactsFile] = hexSha256(data)
	sums[artifactsFile] = hexSha256(rewritten)
	return files, nil
}

func hexSha256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// copyAtomic copies src over dst through a temp file and a rename, so readers never see half a file.
func copyAtomic(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeAtomic(dst, data)
}

// writeAtomic writes data to dst through a temp file and a rename, so readers never see half a file.
func writeAtomic(dst string, data []byte) error {
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
