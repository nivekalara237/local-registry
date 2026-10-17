package cli

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// version is the release of this binary, given to Execute by cmd (set at build time with
// go build -ldflags "-X Execute.version=v1.2.3" -o local-registry ./cmd).
var version = "dev"

// Execute runs the command line argv as the binary of release v and returns the process exit
// status: 0 on success, 1 when the server fails, 2 for a usage or configuration error.
func Execute(v string, argv []string, stdout, stderr io.Writer) int {
	version = v
	return run(argv, stdout, stderr, serveRegistry)
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
	fmt.Fprintf(&s, "local-registry %s (%s, %s/%s", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
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
  public-url: https://registry.example.com
  upload-token: change-me        # keep this file chmod 600

The upload token is read from ` + envPrefix + `_UPLOAD_TOKEN or upload-token in the file, never
from a flag. Without it, upload is disabled (403).`
