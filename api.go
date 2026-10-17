package local_registry

import (
	"github.com/nivekalara237/local-registry/internal/audit"
	"github.com/nivekalara237/local-registry/internal/core"
	"github.com/nivekalara237/local-registry/internal/endpoints"
	regfs "github.com/nivekalara237/local-registry/internal/fs"
)

// The library's public API. The implementation lives in internal/, which other modules cannot
// import: these aliases keep every documented name reachable from this package.

type (
	// HTTPError is an error whose status and message are sent to the client as is.
	HTTPError = core.HTTPError
	// Layout is the asset tree under the asset path (see NewLayout).
	Layout = core.Layout
	// AuditLog is the audit log of the upload routes (see OpenAuditLog).
	AuditLog = audit.AuditLog
	// UploadConfig enables the upload routes (see AddUploadRoutes).
	UploadConfig = endpoints.UploadConfig
	// Release lists the files of one version, as POST /releases/{vcs}/{version} answers it.
	Release = endpoints.Release
	// ReleaseAsset is one file of a Release.
	ReleaseAsset = endpoints.ReleaseAsset
	// Asset scans the catalogue files (GET /artifacts/all).
	Asset         = endpoints.Asset
	AssetPath     = endpoints.AssetPath
	AssetContents = endpoints.AssetContents
)

// ErrNotFound makes an AddRouteE handler answer 404.
var ErrNotFound = core.ErrNotFound

// Defaults applied to the zero values of UploadConfig.
const (
	DefaultMaxFileSize     = endpoints.DefaultMaxFileSize
	DefaultMaxVersionSize  = regfs.DefaultMaxVersionSize
	DefaultMaxVersionFiles = regfs.DefaultMaxVersionFiles
	DefaultMaxStagingSize  = regfs.DefaultMaxStagingSize
	DefaultMinFreeSpace    = regfs.DefaultMinFreeSpace
	DefaultStagingTTL      = regfs.DefaultStagingTTL
)

// NewLayout returns the asset tree under assetPath.
func NewLayout(assetPath string) Layout { return core.NewLayout(assetPath) }

// ValidName reports whether s is a single path segment that stays inside its parent directory.
func ValidName(s string) bool { return core.ValidName(s) }

// IsPublicFile reports whether name, a slash-separated path relative to the asset path,
// may be served by a file route.
func IsPublicFile(name string) bool { return core.IsPublicFile(name) }

// OpenAuditLog opens path for appending, creating it (mode 0600) and its directory if needed.
func OpenAuditLog(path string) (*AuditLog, error) { return audit.OpenAuditLog(path) }

// TokenID identifies a token in the audit log without revealing it: the first 12 hex digits of its sha256.
func TokenID(token string) string { return audit.TokenID(token) }

// LatestVersion is the version that <vcs>/artifacts.json of l designates.
func LatestVersion(l Layout, vcs string) (string, error) { return endpoints.LatestVersion(l, vcs) }

// ListRelease lists the files of <vcs>/<version> ("latest" allowed) in l.
func ListRelease(l Layout, vcs, version, publicURL string) (*Release, error) {
	return endpoints.ListRelease(l, vcs, version, publicURL)
}

// ReadMetadata returns the metadata.json of the published version <vcs>/<version> in l, decoded.
func ReadMetadata(l Layout, vcs, version string) (any, error) {
	return endpoints.ReadMetadata(l, vcs, version)
}
