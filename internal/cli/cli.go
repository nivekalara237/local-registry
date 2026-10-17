package cli

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cast"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	localregistry "github.com/nivekalara237/local-registry"
)

// envPrefix prefixes every environment variable: --max-upload-mb is LOCAL_REGISTRY_MAX_UPLOAD_MB.
const envPrefix = "LOCAL_REGISTRY"

// tokenKey is the only setting with no flag: a flag would show the secret in ps.
const tokenKey = "upload-token"

// minStagingTTL keeps the purge away from uploads still streaming (they may take 30 min).
const minStagingTTL = time.Hour

type arguments struct {
	port        int
	addr        string
	absAsset    string
	maxUploadMB int64
	uploadToken string
	tlsCert     string
	tlsKey      string

	maxVersionMB    int64
	maxVersionFiles int
	maxStagingMB    int64
	minFreeMB       int64
	stagingTTL      time.Duration
	auditLog        string
	publicURL       string

	generateToken bool   // --disable-generate-token=false
	tokenSource   string // where a configured token comes from, for the warning
}

// limit turns a command-line value where 0 means "no limit" into an UploadConfig value,
// where 0 means "default" and a negative value means "no limit".
func limit[T int | int64 | time.Duration](v T) T {
	if v == 0 {
		return -1
	}
	return v
}

// args is the configuration the route handlers read.
var args arguments

// usageError is a bad command line or configuration: the process exits with status 2.
type usageError struct{ error }

func (e usageError) Unwrap() error { return e.error }

func usagef(format string, a ...any) error {
	return usageError{fmt.Errorf(format, a...)}
}

// addFlags declares every setting. Each usage line ends with its environment variable.
func addFlags(fs *pflag.FlagSet) {
	fs.String("config", "", "YAML configuration file")
	fs.String("asset-path", "", "directory holding bin.local/assets (required)")
	fs.String("address", "localhost", "address to listen on")
	fs.Int("port", 8077, "port to listen on")
	fs.String("tls-cert", "", "PEM certificate (chain) file: serve HTTPS only (requires --tls-key)")
	fs.String("tls-key", "", "PEM private key file (requires --tls-cert)")
	fs.Int64("max-upload-mb", localregistry.DefaultMaxFileSize>>20, "largest file accepted by upload, in MiB")
	fs.Int64("max-version-mb", localregistry.DefaultMaxVersionSize>>20, "bytes staged per version, in MiB (0: no limit)")
	fs.Int("max-version-files", localregistry.DefaultMaxVersionFiles, "files staged per version (0: no limit)")
	fs.Int64("max-staging-mb", localregistry.DefaultMaxStagingSize>>20, "bytes staged for all versions, in MiB (0: no limit)")
	fs.Int64("min-free-mb", localregistry.DefaultMinFreeSpace>>20, "disk space an upload must leave free, in MiB (0: no check)")
	fs.Duration("staging-ttl", localregistry.DefaultStagingTTL, "purge staged versions untouched for this long, at least 1h (0: never)")
	fs.String("audit-log", "", "audit log file (default <asset-path>/bin.local/audit.log)")
	fs.String("public-url", "", "address clients reach the server at, e.g. https://registry.example.com: published artifacts.json paths become absolute URLs under it (default: relative paths)")
	fs.Bool("disable-generate-token", true, "with =false, generate a random upload token at startup and print it on stdout only, unless a token is configured")
	fs.SortFlags = false
	fs.VisitAll(func(f *pflag.Flag) {
		f.Usage += " [env " + envName(f.Name) + "]"
	})
}

func envName(key string) string {
	return envPrefix + "_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
}

// fileKeys are the settings a configuration file may hold: every flag but --config, and the token.
func fileKeys(fs *pflag.FlagSet) []string {
	keys := []string{tokenKey}
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Name != "config" && f.Name != "help" {
			keys = append(keys, f.Name)
		}
	})
	return keys
}

// loadConfig resolves and validates the settings: flags, then LOCAL_REGISTRY_* variables,
// then the --config file, then defaults. Any problem is a usageError.
func loadConfig(v *viper.Viper, fs *pflag.FlagSet, stderr io.Writer) (arguments, error) {
	var a arguments
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
	if err := v.BindPFlags(fs); err != nil {
		return a, err
	}

	file, err := expandPath(v.GetString("config"))
	if err != nil {
		return a, usagef("config: %v", err)
	}
	if file != "" {
		v.SetConfigFile(file)
		v.SetConfigType("yaml")
		if err := v.ReadInConfig(); err != nil {
			return a, usagef("config file %s: %v", file, err)
		}
		known := fileKeys(fs)
		for _, k := range v.AllKeys() {
			if v.InConfig(k) && !slices.Contains(known, k) {
				return a, usagef("config file %s: unknown setting %q", file, k)
			}
		}
		if v.InConfig(tokenKey) {
			if info, err := os.Stat(file); err == nil && info.Mode().Perm()&0o077 != 0 {
				fmt.Fprintf(stderr, "warning: %s holds the upload token but is readable by other users (mode %v): chmod 600 %s\n",
					file, info.Mode().Perm(), file)
			}
		}
	}

	// Typed reads that fail loudly: viper's plain getters turn "abc" into 0 without a word.
	var errs []error
	str := func(key string) string {
		s, err := cast.ToStringE(v.Get(key))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", key, err))
		}
		return s
	}
	i64 := func(key string) int64 {
		n, err := cast.ToInt64E(v.Get(key))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %q is not a whole number", key, v.Get(key)))
		}
		return n
	}
	// path also expands $VAR, ${VAR} and a leading ~: a value from the config file, a systemd
	// unit or a single-quoted argument never went through a shell.
	path := func(key string) string {
		p, err := expandPath(str(key))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", key, err))
		}
		return p
	}
	a.absAsset = path("asset-path")
	a.addr = str("address")
	a.port = int(i64("port"))
	a.tlsCert = path("tls-cert")
	a.tlsKey = path("tls-key")
	a.maxUploadMB = i64("max-upload-mb")
	a.maxVersionMB = i64("max-version-mb")
	a.maxVersionFiles = int(i64("max-version-files"))
	a.maxStagingMB = i64("max-staging-mb")
	a.minFreeMB = i64("min-free-mb")
	a.auditLog = path("audit-log")
	a.publicURL = str("public-url")
	a.uploadToken = str(tokenKey)
	if a.uploadToken != "" {
		a.tokenSource = source(v, fs, tokenKey)
	}
	disableGen, err := cast.ToBoolE(v.Get("disable-generate-token"))
	if err != nil {
		errs = append(errs, fmt.Errorf("disable-generate-token: %q is not true or false", v.Get("disable-generate-token")))
	}
	a.generateToken = !disableGen
	ttl, err := cast.ToDurationE(v.Get("staging-ttl"))
	if err != nil {
		errs = append(errs, fmt.Errorf("staging-ttl: %q is not a duration such as 90m or 24h", v.Get("staging-ttl")))
	}
	a.stagingTTL = ttl
	if len(errs) > 0 {
		return a, usageError{errors.Join(errs...)}
	}
	return a, validate(&a)
}

// expandPath replaces $VAR and ${VAR} by their value, and a leading ~ by the home directory.
// An unset variable is an error, not an empty string: "$HOMEE/x" must not become "/x".
func expandPath(p string) (string, error) {
	var missing []string
	p = os.Expand(p, func(name string) string {
		v, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, "$"+name)
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("environment variable %s is not set", strings.Join(missing, ", "))
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %v", err)
		}
		p = home + p[1:]
	} else if strings.HasPrefix(p, "~") {
		return "", fmt.Errorf("%s: only ~ and ~/ are expanded, not ~user", p)
	}
	return p, nil
}

// validate checks the settings and completes the derived ones (absolute paths, audit log).
// --asset-path is required: defaulting to the working directory would publish whatever it holds.
func validate(a *arguments) error {
	if a.absAsset == "" {
		return usagef("asset-path is required (--asset-path, %s or asset-path in the config file)", envName("asset-path"))
	}
	abs, err := filepath.Abs(a.absAsset)
	if err != nil {
		return usagef("asset-path: %v", err)
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return usagef("asset-path %s is not an existing directory", abs)
	}
	a.absAsset = abs

	if a.port < 0 || a.port > 65535 {
		return usagef("port %d is out of range 0-65535", a.port)
	}
	if (a.tlsCert == "") != (a.tlsKey == "") {
		return usagef("tls-cert and tls-key go together")
	}
	for _, l := range []struct {
		name string
		v    int64
	}{{"max-version-mb", a.maxVersionMB}, {"max-version-files", int64(a.maxVersionFiles)}, {"max-staging-mb", a.maxStagingMB}, {"min-free-mb", a.minFreeMB}} {
		if l.v < 0 {
			return usagef("%s cannot be negative", l.name)
		}
	}
	if a.stagingTTL != 0 && a.stagingTTL < minStagingTTL {
		return usagef("staging-ttl must be 0 or at least %s", minStagingTTL)
	}

	if a.publicURL != "" {
		u, err := url.Parse(a.publicURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(a.publicURL, "?") {
			return usagef("public-url %q must be an http or https URL with no user, query or fragment, such as https://registry.example.com", a.publicURL)
		}
		a.publicURL = strings.TrimRight(a.publicURL, "/")
	}

	if a.auditLog == "" {
		a.auditLog = filepath.Join(a.absAsset, "bin.local", "audit.log")
	}
	if a.auditLog, err = filepath.Abs(a.auditLog); err != nil {
		return usagef("audit-log: %v", err)
	}
	// The file routes serve some trees under the asset path: the audit log must not be in one.
	if rel, err := filepath.Rel(a.absAsset, a.auditLog); err == nil && localregistry.IsPublicFile(filepath.ToSlash(rel)) {
		return usagef("audit-log %s would be downloadable: put it outside bin.local/assets and assets", a.auditLog)
	}
	return nil
}

// source tells where the effective value of key comes from.
func source(v *viper.Viper, fs *pflag.FlagSet, key string) string {
	if f := fs.Lookup(key); f != nil && f.Changed {
		return "flag"
	}
	if _, ok := os.LookupEnv(envName(key)); ok {
		return "env " + envName(key)
	}
	if v.InConfig(key) {
		return "config file"
	}
	return "default"
}
