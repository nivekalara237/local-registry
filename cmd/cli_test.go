package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cleanEnv removes every LOCAL_REGISTRY_* variable of the developer's shell for this test.
func cleanEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, envPrefix+"_") {
			t.Setenv(k, "") // restored after the test
			os.Unsetenv(k)
		}
	}
}

type cliResult struct {
	code           int
	stdout, stderr string
	served         *arguments // what serve received, nil if it was not called
}

func runCLI(t *testing.T, argv ...string) cliResult {
	t.Helper()
	var out, errOut bytes.Buffer
	var res cliResult
	res.code = run(argv, &out, &errOut, func(a arguments, _ io.Writer) error {
		res.served = &a
		return nil
	})
	res.stdout, res.stderr = out.String(), errOut.String()
	return res
}

func TestServeDefaultsAndFlags(t *testing.T) {
	cleanEnv(t)
	dir := t.TempDir()

	r := runCLI(t, "serve", "--asset-path", dir)
	if r.code != 0 || r.served == nil {
		t.Fatalf("code = %d, stderr = %s", r.code, r.stderr)
	}
	a := *r.served
	if a.absAsset != dir || a.addr != "localhost" || a.port != 8077 || a.maxUploadMB != 200 ||
		a.maxVersionMB != 2048 || a.maxVersionFiles != 100 || a.maxStagingMB != 10240 || a.minFreeMB != 1024 ||
		a.stagingTTL != 24*time.Hour || a.auditLog != filepath.Join(dir, "bin.local", "audit.log") || a.uploadToken != "" {
		t.Fatalf("defaults = %+v", a)
	}

	r = runCLI(t, "serve", "--asset-path="+dir, "--port", "9000", "--address", "0.0.0.0", "--max-upload-mb", "5",
		"--max-version-mb", "0", "--staging-ttl", "0", "--audit-log", filepath.Join(dir, "logs", "a.log"))
	if r.code != 0 {
		t.Fatalf("code = %d, stderr = %s", r.code, r.stderr)
	}
	if a := *r.served; a.port != 9000 || a.addr != "0.0.0.0" || a.maxUploadMB != 5 || a.maxVersionMB != 0 || a.stagingTTL != 0 ||
		a.auditLog != filepath.Join(dir, "logs", "a.log") {
		t.Fatalf("flags = %+v", a)
	}

	// A relative asset path becomes absolute.
	t.Chdir(dir)
	if r := runCLI(t, "serve", "--asset-path", "."); r.code != 0 || !filepath.IsAbs(r.served.absAsset) {
		t.Fatalf("relative path: %+v", r)
	}
}

func TestPrecedenceFlagEnvFileDefault(t *testing.T) {
	cleanEnv(t)
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	writeFile(t, cfg, "asset-path: "+dir+"\nport: 7000\naddress: 10.0.0.1\nmax-version-files: 7\nstaging-ttl: 48h\n")

	// File only.
	r := runCLI(t, "serve", "--config", cfg)
	if r.code != 0 {
		t.Fatalf("code = %d, stderr = %s", r.code, r.stderr)
	}
	if a := *r.served; a.port != 7000 || a.addr != "10.0.0.1" || a.maxVersionFiles != 7 || a.stagingTTL != 48*time.Hour || a.maxUploadMB != 200 {
		t.Fatalf("file values = %+v", a)
	}

	// Environment beats the file; the config file itself can come from the environment.
	t.Setenv("LOCAL_REGISTRY_CONFIG", cfg)
	t.Setenv("LOCAL_REGISTRY_PORT", "7100")
	t.Setenv("LOCAL_REGISTRY_MAX_VERSION_FILES", "8")
	r = runCLI(t, "serve")
	if a := *r.served; r.code != 0 || a.port != 7100 || a.maxVersionFiles != 8 || a.addr != "10.0.0.1" {
		t.Fatalf("env over file: code %d, %+v, %s", r.code, a, r.stderr)
	}

	// A flag beats both.
	r = runCLI(t, "serve", "--port", "7200")
	if a := *r.served; r.code != 0 || a.port != 7200 || a.maxVersionFiles != 8 {
		t.Fatalf("flag over env: %+v", a)
	}
}

func TestUploadToken(t *testing.T) {
	cleanEnv(t)
	dir := t.TempDir()

	t.Setenv("LOCAL_REGISTRY_UPLOAD_TOKEN", "from-env")
	if r := runCLI(t, "serve", "--asset-path", dir); r.code != 0 || r.served.uploadToken != "from-env" {
		t.Fatalf("env token: %+v", r)
	}
	os.Unsetenv("LOCAL_REGISTRY_UPLOAD_TOKEN")

	cfg := filepath.Join(dir, "config.yaml")
	writeFile(t, cfg, "asset-path: "+dir+"\nupload-token: from-file\n")
	if err := os.Chmod(cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	r := runCLI(t, "serve", "--config", cfg)
	if r.code != 0 || r.served.uploadToken != "from-file" || strings.Contains(r.stderr, "readable by other users") {
		t.Fatalf("file token: %+v", r)
	}

	// A token in a file others can read is accepted, with a warning.
	if err := os.Chmod(cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	if r := runCLI(t, "serve", "--config", cfg); r.code != 0 || !strings.Contains(r.stderr, "chmod 600") {
		t.Fatalf("readable file: %+v", r)
	}

	// There is no flag for the token: it would show in ps.
	if r := runCLI(t, "serve", "--asset-path", dir, "--upload-token", "x"); r.code != 2 {
		t.Fatalf("--upload-token flag accepted: %+v", r)
	}
}

func TestUsageErrors(t *testing.T) {
	cleanEnv(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	writeFile(t, file, "x")
	badCfg := filepath.Join(dir, "bad.yaml")
	writeFile(t, badCfg, "asset-path: "+dir+"\nmax-verion-mb: 10\n") // typo
	nested := filepath.Join(dir, "nested.yaml")
	writeFile(t, nested, "asset-path: "+dir+"\ntls:\n  cert: x\n")
	broken := filepath.Join(dir, "broken.yaml")
	writeFile(t, broken, "asset-path: [\n")

	tests := []struct {
		name string
		env  map[string]string
		argv []string
		want string
	}{
		{"no asset path", nil, []string{"serve"}, "asset-path is required"},
		{"missing dir", nil, []string{"serve", "--asset-path", filepath.Join(dir, "missing")}, "not an existing directory"},
		{"file as dir", nil, []string{"serve", "--asset-path", file}, "not an existing directory"},
		{"cert without key", nil, []string{"serve", "--asset-path", dir, "--tls-cert", "c.pem"}, "go together"},
		{"short ttl", nil, []string{"serve", "--asset-path", dir, "--staging-ttl", "30m"}, "at least 1h"},
		{"negative quota", nil, []string{"serve", "--asset-path", dir, "--max-version-mb", "-1"}, "cannot be negative"},
		{"negative files", nil, []string{"serve", "--asset-path", dir, "--max-version-files", "-1"}, "cannot be negative"},
		{"port range", nil, []string{"serve", "--asset-path", dir, "--port", "70000"}, "out of range"},
		{"public url without scheme", nil, []string{"serve", "--asset-path", dir, "--public-url", "registry.example.com"}, "public-url"},
		{"public url ftp", nil, []string{"serve", "--asset-path", dir, "--public-url", "ftp://registry.example.com"}, "public-url"},
		{"public url with query", nil, []string{"serve", "--asset-path", dir, "--public-url", "https://r.example.com/?a=1"}, "public-url"},
		{"public url with user", nil, []string{"serve", "--asset-path", dir, "--public-url", "https://u:p@r.example.com"}, "public-url"},
		{"audit in public tree", nil, []string{"serve", "--asset-path", dir, "--audit-log", filepath.Join(dir, "bin.local/assets/a.log")}, "downloadable"},
		{"bad flag value", nil, []string{"serve", "--asset-path", dir, "--port", "abc"}, "invalid argument"},
		{"unknown flag", nil, []string{"serve", "--nope"}, "unknown flag"},
		{"extra argument", nil, []string{"serve", "--asset-path", dir, "extra"}, `unexpected argument "extra"`},
		{"unknown command", nil, []string{"serv"}, `unknown command "serv"`},
		{"bad env number", map[string]string{"LOCAL_REGISTRY_PORT": "abc"}, []string{"serve", "--asset-path", dir}, `port: "abc" is not a whole number`},
		{"bad env duration", map[string]string{"LOCAL_REGISTRY_STAGING_TTL": "tomorrow"}, []string{"serve", "--asset-path", dir}, "not a duration"},
		{"config typo", nil, []string{"serve", "--config", badCfg}, `unknown setting "max-verion-mb"`},
		{"config nested key", nil, []string{"serve", "--config", nested}, `unknown setting "tls.cert"`},
		{"config missing", nil, []string{"serve", "--config", filepath.Join(dir, "none.yaml")}, "config file"},
		{"config broken", nil, []string{"serve", "--config", broken}, "config file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			r := runCLI(t, tt.argv...)
			if r.code != 2 || r.served != nil || !strings.Contains(r.stderr, tt.want) || !strings.Contains(r.stderr, "--help' for usage") {
				t.Fatalf("code = %d, served = %v, stderr = %q; want 2 and %q", r.code, r.served != nil, r.stderr, tt.want)
			}
		})
	}
}

func TestServeFailureExitsOne(t *testing.T) {
	cleanEnv(t)
	var errOut bytes.Buffer
	code := run([]string{"serve", "--asset-path", t.TempDir()}, io.Discard, &errOut, func(arguments, io.Writer) error {
		return os.ErrPermission
	})
	if code != 1 || !strings.Contains(errOut.String(), "permission denied") || strings.Contains(errOut.String(), "for usage") {
		t.Fatalf("code = %d, stderr = %q", code, errOut.String())
	}
}

func TestConfigCommand(t *testing.T) {
	cleanEnv(t)
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	writeFile(t, cfg, "asset-path: "+dir+"\nport: 7000\nupload-token: very-secret\n")
	if err := os.Chmod(cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCAL_REGISTRY_ADDRESS", "0.0.0.0")

	r := runCLI(t, "config", "--config", cfg, "--max-upload-mb", "50")
	if r.code != 0 || r.served != nil {
		t.Fatalf("code = %d, stderr = %s", r.code, r.stderr)
	}
	for _, want := range []string{
		"SETTING", "SOURCE",
		"asset-path", dir, "config file",
		"port", "7000",
		"address", "0.0.0.0", "env LOCAL_REGISTRY_ADDRESS",
		"max-upload-mb", "50", "flag",
		"staging-ttl", "24h0m0s", "default",
		"upload-token", "set (token_id " + "", // token_id value checked below
	} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("config output lacks %q:\n%s", want, r.stdout)
		}
	}
	if strings.Contains(r.stdout, "very-secret") {
		t.Fatal("config printed the token")
	}
	if r := runCLI(t, "config"); r.code != 2 { // validated like serve
		t.Fatalf("config without asset-path: %+v", r)
	}
}

func TestHelpAndVersion(t *testing.T) {
	cleanEnv(t)
	for _, argv := range [][]string{{}, {"--help"}, {"help"}} {
		r := runCLI(t, argv...)
		if r.code != 0 || !strings.Contains(r.stdout, "serve") || !strings.Contains(r.stdout, "config") || !strings.Contains(r.stdout, "version") {
			t.Fatalf("%q: code %d, stdout %q", argv, r.code, r.stdout)
		}
	}
	r := runCLI(t, "serve", "--help")
	for _, want := range []string{"--asset-path", "[env LOCAL_REGISTRY_ASSET_PATH]", "--staging-ttl", "LOCAL_REGISTRY_UPLOAD_TOKEN", "Examples:", "upload-token: change-me"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("serve --help lacks %q", want)
		}
	}
	if r.served != nil {
		t.Fatal("--help started the server")
	}
	for _, argv := range [][]string{{"version"}, {"--version"}} {
		if r := runCLI(t, argv...); r.code != 0 || !strings.HasPrefix(r.stdout, "local-registry dev (go") {
			t.Fatalf("%q: %q", argv, r.stdout)
		}
	}
}

func TestExpandPath(t *testing.T) {
	cleanEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("REG", "/srv/reg")
	t.Setenv("EMPTY", "")

	for in, want := range map[string]string{
		"$HOME/.bridge/registry":   home + "/.bridge/registry",
		"${HOME}/.bridge/registry": home + "/.bridge/registry",
		"~/.bridge/registry":       home + "/.bridge/registry",
		"~":                        home,
		"$REG/v1":                  "/srv/reg/v1",
		"/a/$EMPTY/b":              "/a//b", // set but empty is allowed
		"/plain/path":              "/plain/path",
		"/not~home":                "/not~home",
		"":                         "",
	} {
		if got, err := expandPath(in); err != nil || got != want {
			t.Errorf("expandPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for in, want := range map[string]string{
		"$HOMEE/x":       "$HOMEE is not set",
		"${NOPE}/$NADA":  "$NOPE, $NADA is not set",
		"~root/registry": "not ~user",
	} {
		if _, err := expandPath(in); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("expandPath(%q) err = %v; want %q", in, err, want)
		}
	}
}

func TestPathSettingsAreExpanded(t *testing.T) {
	cleanEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	reg := filepath.Join(home, ".bridge", "registry")
	if err := os.MkdirAll(reg, 0o755); err != nil {
		t.Fatal(err)
	}

	// The reported case: a literal $HOME, as a single-quoted shell argument gives it.
	r := runCLI(t, "serve", "--asset-path", "$HOME/.bridge/registry", "--audit-log", "~/logs/audit.log",
		"--tls-cert", "${HOME}/c.pem", "--tls-key", "~/k.pem")
	if r.code != 0 {
		t.Fatalf("code = %d, stderr = %s", r.code, r.stderr)
	}
	if a := *r.served; a.absAsset != reg || a.auditLog != filepath.Join(home, "logs", "audit.log") ||
		a.tlsCert != filepath.Join(home, "c.pem") || a.tlsKey != filepath.Join(home, "k.pem") {
		t.Fatalf("flags not expanded: %+v", a)
	}

	// From the environment, and from a config file whose own path uses a variable.
	t.Setenv("LOCAL_REGISTRY_ASSET_PATH", "~/.bridge/registry")
	if r := runCLI(t, "serve"); r.code != 0 || r.served.absAsset != reg {
		t.Fatalf("env not expanded: %+v", r)
	}
	os.Unsetenv("LOCAL_REGISTRY_ASSET_PATH")
	writeFile(t, filepath.Join(home, "config.yaml"), "asset-path: $HOME/.bridge/registry\n")
	r = runCLI(t, "serve", "--config", "$HOME/config.yaml")
	if r.code != 0 || r.served.absAsset != reg {
		t.Fatalf("config file not expanded: %+v", r)
	}

	// config shows the expanded values.
	if r := runCLI(t, "config", "--config", "~/config.yaml"); r.code != 0 || !strings.Contains(r.stdout, filepath.Join(home, "config.yaml")) || !strings.Contains(r.stdout, reg) {
		t.Fatalf("config output: %s %s", r.stdout, r.stderr)
	}

	// An unset variable is a usage error naming it, not an empty string.
	for _, argv := range [][]string{
		{"serve", "--asset-path", "$NOPE/registry"},
		{"serve", "--asset-path", reg, "--audit-log", "$NOPE/a.log"},
		{"serve", "--config", "$NOPE/config.yaml"},
	} {
		if r := runCLI(t, argv...); r.code != 2 || !strings.Contains(r.stderr, "$NOPE is not set") {
			t.Fatalf("%q: code %d, stderr %q", argv, r.code, r.stderr)
		}
	}
}

func TestGenerateToken(t *testing.T) {
	dir := t.TempDir()
	hex64 := func(s string) bool {
		if len(s) != 64 {
			return false
		}
		for _, c := range s {
			if !strings.ContainsRune("0123456789abcdef", c) {
				return false
			}
		}
		return true
	}

	t.Run("default: no generation", func(t *testing.T) {
		cleanEnv(t)
		r := runCLI(t, "serve", "--asset-path", dir)
		if r.code != 0 || r.served.uploadToken != "" || strings.Contains(r.stdout, "Upload token") {
			t.Fatalf("%+v", r)
		}
	})

	t.Run("=false without token: generated, on stdout only", func(t *testing.T) {
		cleanEnv(t)
		r := runCLI(t, "serve", "--asset-path", dir, "--disable-generate-token=false")
		tok := r.served.uploadToken
		if r.code != 0 || !hex64(tok) {
			t.Fatalf("token = %q, %+v", tok, r)
		}
		if !strings.Contains(r.stdout, "Upload token: "+tok+"\n") {
			t.Fatalf("stdout = %q", r.stdout)
		}
		if strings.Contains(r.stderr, tok) || strings.Contains(r.stderr, "WARN") {
			t.Fatalf("token or warning on stderr: %q", r.stderr)
		}
		// A new token at each start.
		if r2 := runCLI(t, "serve", "--asset-path", dir, "--disable-generate-token=false"); r2.served.uploadToken == tok {
			t.Fatal("the same token was generated twice")
		}
	})

	t.Run("=false from env and from the config file", func(t *testing.T) {
		cleanEnv(t)
		t.Setenv("LOCAL_REGISTRY_DISABLE_GENERATE_TOKEN", "false")
		if r := runCLI(t, "serve", "--asset-path", dir); r.code != 0 || !hex64(r.served.uploadToken) {
			t.Fatalf("env: %+v", r)
		}
		os.Unsetenv("LOCAL_REGISTRY_DISABLE_GENERATE_TOKEN")
		cfg := filepath.Join(dir, "gen.yaml")
		writeFile(t, cfg, "asset-path: "+dir+"\ndisable-generate-token: false\n")
		if r := runCLI(t, "serve", "--config", cfg); r.code != 0 || !hex64(r.served.uploadToken) {
			t.Fatalf("config file: %+v", r)
		}
		// A flag still wins over the file.
		if r := runCLI(t, "serve", "--config", cfg, "--disable-generate-token=true"); r.code != 0 || r.served.uploadToken != "" {
			t.Fatalf("flag over file: %+v", r)
		}
	})

	t.Run("=false with a configured token: kept, warning", func(t *testing.T) {
		cleanEnv(t)
		t.Setenv("LOCAL_REGISTRY_UPLOAD_TOKEN", "from-env")
		r := runCLI(t, "serve", "--asset-path", dir, "--disable-generate-token=false")
		if r.code != 0 || r.served.uploadToken != "from-env" || strings.Contains(r.stdout, "Upload token") {
			t.Fatalf("%+v", r)
		}
		if !strings.Contains(r.stderr, "WARN upload token generation skipped") || !strings.Contains(r.stderr, "env LOCAL_REGISTRY_UPLOAD_TOKEN") {
			t.Fatalf("stderr = %q", r.stderr)
		}
		if strings.Contains(r.stderr, "from-env") {
			t.Fatal("the configured token was printed")
		}

		os.Unsetenv("LOCAL_REGISTRY_UPLOAD_TOKEN")
		cfg := filepath.Join(dir, "tok.yaml")
		writeFile(t, cfg, "asset-path: "+dir+"\nupload-token: from-file\n")
		if err := os.Chmod(cfg, 0o600); err != nil {
			t.Fatal(err)
		}
		r = runCLI(t, "serve", "--config", cfg, "--disable-generate-token=false")
		if r.served.uploadToken != "from-file" || !strings.Contains(r.stderr, "(config file)") {
			t.Fatalf("config file token: %+v", r)
		}
	})

	t.Run("bad value", func(t *testing.T) {
		cleanEnv(t)
		t.Setenv("LOCAL_REGISTRY_DISABLE_GENERATE_TOKEN", "maybe")
		if r := runCLI(t, "serve", "--asset-path", dir); r.code != 2 || !strings.Contains(r.stderr, "is not true or false") {
			t.Fatalf("%+v", r)
		}
	})

	t.Run("config shows the plan and generates nothing", func(t *testing.T) {
		cleanEnv(t)
		r := runCLI(t, "config", "--asset-path", dir, "--disable-generate-token=false")
		if r.code != 0 || !strings.Contains(r.stdout, "generated at startup, printed on stdout only") ||
			strings.Contains(r.stdout, "Upload token:") {
			t.Fatalf("%+v", r)
		}
	})
}

func TestPublicURL(t *testing.T) {
	cleanEnv(t)
	dir := t.TempDir()
	r := runCLI(t, "serve", "--asset-path", dir)
	if r.code != 0 || r.served.publicURL != "" {
		t.Fatalf("default: code = %d, public-url = %q, stderr = %s", r.code, r.served.publicURL, r.stderr)
	}
	t.Setenv(envName("public-url"), "https://registry.example.com/sub/")
	r = runCLI(t, "serve", "--asset-path", dir)
	if r.code != 0 || r.served.publicURL != "https://registry.example.com/sub" {
		t.Fatalf("env: code = %d, public-url = %q, stderr = %s", r.code, r.served.publicURL, r.stderr)
	}
	r = runCLI(t, "config", "--asset-path", dir)
	if !strings.Contains(r.stdout, "public-url") || !strings.Contains(r.stdout, "https://registry.example.com/sub") {
		t.Fatalf("config output = %s", r.stdout)
	}
}
