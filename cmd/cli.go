package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	localregistry "sonarbridge-go/local-registry"

	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// version is the release of this binary, set at build time:
//
//	go build -ldflags "-X main.version=v1.2.3" -o local-registry ./cmd
var version = "dev"

// envPrefix prefixes every environment variable: --max-upload-mb is LOCAL_REGISTRY_MAX_UPLOAD_MB.
const envPrefix = "LOCAL_REGISTRY"

// tokenKey is the only setting with no flag: a flag would show the secret in ps.
const tokenKey = "upload-token"

// minStagingTTL keeps the purge away from uploads still streaming (they may take 30 min).
const minStagingTTL = time.Hour

// usageError is a bad command line or configuration: the process exits with status 2.
type usageError struct{ error }

func (e usageError) Unwrap() error { return e.error }

func usagef(format string, a ...any) error {
	return usageError{fmt.Errorf(format, a...)}
}

// serveFunc runs the server with a validated configuration; tests replace it.
type serveFunc func(a arguments, stderr io.Writer) error

// run executes the command line and returns the process exit status:
// 0 on success, 1 when the server fails, 2 for a usage or configuration error.
func run(argv []string, stdout, stderr io.Writer, serve serveFunc) int {
	root := newRootCmd(serve)
	root.SetArgs(argv)
	root.SetOut(stdout)
	root.SetErr(stderr)
	cmd, err := root.ExecuteC()
	if err == nil {
		return 0
	}
	fmt.Fprintln(stderr, "Error:", err)
	var ue usageError
	if errors.As(err, &ue) {
		fmt.Fprintf(stderr, "Run '%s --help' for usage.\n", cmd.CommandPath())
		return 2
	}
	return 1
}

func versionString() string {
	var s strings.Builder
	s.WriteString(fmt.Sprintf("local-registry %s (%s, %s/%s", version, runtime.Version(), runtime.GOOS, runtime.GOARCH))
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, kv := range info.Settings {
			if kv.Key == "vcs.revision" && len(kv.Value) >= 12 {
				s.WriteString(", commit " + kv.Value[:12])
			}
		}
	}
	return s.String() + ")"
}

func newRootCmd(serve serveFunc) *cobra.Command {
	root := &cobra.Command{
		Use:   "local-registry",
		Short: "Local HTTP registry for goreleaser releases",
		Long: `local-registry serves a catalogue of goreleaser releases, their binaries and checksums,
and accepts new releases through a two-step upload (send the files, then publish atomically).

Every setting can come, from highest to lowest priority, from a command-line flag, an
environment variable (` + envPrefix + `_<FLAG_NAME>), a YAML configuration file (--config) or
its default. Run 'local-registry config' to see the effective value of each setting.`,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usagef("unknown command %q for %q", args[0], cmd.CommandPath())
			}
			return cmd.Help()
		},
	}
	root.SetVersionTemplate(versionString() + "\n")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err} })

	root.AddCommand(newServeCmd(serve), newConfigCmd(), &cobra.Command{
		Use:   "version",
		Short: "Print the version of local-registry",
		Args:  noArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), versionString())
		},
	})
	return root
}

func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return usagef("unexpected argument %q for %q", args[0], cmd.CommandPath())
	}
	return nil
}

const configHelp = `
Configuration file (--config, YAML): the same names as the flags, plus upload-token.
Use absolute paths: relative ones are resolved from the working directory.

  asset-path: /srv/registry
  address: 0.0.0.0
  port: 8443
  tls-cert: /etc/local-registry/fullchain.pem
  tls-key: /etc/local-registry/privkey.pem
  max-version-mb: 4096
  staging-ttl: 48h
  upload-token: change-me        # keep this file chmod 600

The upload token is read from ` + envPrefix + `_UPLOAD_TOKEN or upload-token in the file, never
from a flag. Without it, upload is disabled (403).`

func newServeCmd(serve serveFunc) *cobra.Command {
	v := viper.New()
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the registry server",
		Long: `Run the registry server until SIGINT or SIGTERM, then drain in-flight requests for up to 10 s.

Exit status: 0 after a clean stop, 1 if the server fails (port in use, bad TLS
certificate, unreadable audit log), 2 for a usage or configuration error.
` + configHelp,
		Example: `  # Read-only, local
  local-registry serve --asset-path /srv/registry

  # Network, HTTPS, upload enabled
  LOCAL_REGISTRY_UPLOAD_TOKEN=change-me local-registry serve --asset-path /srv/registry \
    --address 0.0.0.0 --tls-cert fullchain.pem --tls-key privkey.pem

  # Everything from a configuration file
  local-registry serve --config /etc/local-registry/config.yaml`,
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := loadConfig(v, cmd.Flags(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if err := generateToken(&a, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				return err
			}
			return serve(a, cmd.ErrOrStderr())
		},
	}
	addFlags(cmd.Flags())
	return cmd
}

func newConfigCmd() *cobra.Command {
	v := viper.New()
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Print the effective configuration and where each value comes from",
		Long: `Resolve the configuration exactly as 'serve' would (flags, environment, configuration
file, defaults), validate it, and print each setting with its source. The upload token is
never printed: only whether it is set and its token_id, as written in the audit log.
` + configHelp,
		Example: `  local-registry config --config /etc/local-registry/config.yaml
  LOCAL_REGISTRY_PORT=9000 local-registry config --asset-path /srv/registry`,
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := loadConfig(v, cmd.Flags(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			printConfig(cmd.OutOrStdout(), v, cmd.Flags(), a)
			return nil
		},
	}
	addFlags(cmd.Flags())
	return cmd
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

// configPath is the configuration file in use, expanded as loadConfig expanded it.
func configPath(v *viper.Viper) string {
	p, _ := expandPath(v.GetString("config"))
	return p
}

func printConfig(w io.Writer, v *viper.Viper, fs *pflag.FlagSet, a arguments) {
	token := "not set: upload disabled"
	switch {
	case a.uploadToken != "" && a.generateToken:
		token = "set (token_id " + localregistry.TokenID(a.uploadToken) + "); generation skipped"
	case a.uploadToken != "":
		token = "set (token_id " + localregistry.TokenID(a.uploadToken) + ")"
	case a.generateToken:
		token = "generated at startup, printed on stdout only"
	}
	rows := [][2]string{
		{"config", configPath(v)},
		{"asset-path", a.absAsset},
		{"address", a.addr},
		{"port", fmt.Sprint(a.port)},
		{"tls-cert", a.tlsCert},
		{"tls-key", a.tlsKey},
		{"max-upload-mb", fmt.Sprint(a.maxUploadMB)},
		{"max-version-mb", fmt.Sprint(a.maxVersionMB)},
		{"max-version-files", fmt.Sprint(a.maxVersionFiles)},
		{"max-staging-mb", fmt.Sprint(a.maxStagingMB)},
		{"min-free-mb", fmt.Sprint(a.minFreeMB)},
		{"staging-ttl", a.stagingTTL.String()},
		{"audit-log", a.auditLog},
		{"disable-generate-token", fmt.Sprint(!a.generateToken)},
		{tokenKey, token},
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SETTING\tVALUE\tSOURCE")
	for _, r := range rows {
		val := r[1]
		if val == "" {
			val = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r[0], val, source(v, fs, r[0]))
	}
	tw.Flush()
}

// generatedTokenBytes is the entropy of a generated token: 32 bytes, 64 hex digits.
const generatedTokenBytes = 32

// generateToken applies --disable-generate-token=false. With no token configured, it draws a
// random one and prints it on stdout only: never through the logs or the audit log, which may
// end up in files. With a token configured, that token stays in use and a warning says so.
func generateToken(a *arguments, stdout, stderr io.Writer) error {
	if !a.generateToken {
		return nil
	}
	if a.uploadToken != "" {
		log.New(stderr, "", log.LstdFlags).Printf(
			"WARN upload token generation skipped: a token is already configured (%s) and stays in use; "+
				"unset %s and upload-token to get a generated one, or drop --disable-generate-token=false",
			a.tokenSource, envName(tokenKey))
		return nil
	}
	b := make([]byte, generatedTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("generating the upload token: %w", err)
	}
	a.uploadToken = hex.EncodeToString(b)
	fmt.Fprintf(stdout, "Upload token: %s\n", a.uploadToken)
	fmt.Fprintf(stdout, "  generated for this run only and saved nowhere: it changes at each start (token_id %s)\n",
		localregistry.TokenID(a.uploadToken))
	return nil
}

// serveRegistry runs the server until SIGINT or SIGTERM.
func serveRegistry(a arguments, stderr io.Writer) error {
	args = a
	if args.uploadToken == "" {
		fmt.Fprintf(stderr, "%s is not set: upload is disabled\n", envName(tokenKey))
	}
	server, err := newServer()
	if err != nil {
		return err
	}
	if args.uploadToken != "" && !server.TLSEnabled() && !isLoopback(args.addr) {
		fmt.Fprintf(stderr, "warning: upload is enabled on %s without TLS: the token travels in clear text (use --tls-cert/--tls-key or a TLS reverse proxy)\n", args.addr)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return server.ServeContext(ctx)
}

func isLoopback(addr string) bool {
	if addr == "localhost" {
		return true
	}
	ip := net.ParseIP(addr)
	return ip != nil && ip.IsLoopback()
}
