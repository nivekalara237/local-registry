package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	localregistry "github.com/nivekalara237/local-registry"
)

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

func newServer() (*localregistry.LocalRegistryServer, error) {
	server := localregistry.NewLocalRegistryServer(args.port, args.addr, args.absAsset)
	if args.tlsCert != "" {
		if err := server.SetTLS(args.tlsCert, args.tlsKey); err != nil {
			return nil, err
		}
	}
	if args.auditLog != "" {
		a, err := localregistry.OpenAuditLog(args.auditLog)
		if err != nil {
			return nil, err
		}
		server.SetAuditLog(a)
	}
	server.AddUploadRoutes(localregistry.UploadConfig{
		Token:           args.uploadToken,
		MaxFileSize:     args.maxUploadMB << 20,
		MaxVersionSize:  limit(args.maxVersionMB << 20),
		MaxVersionFiles: limit(args.maxVersionFiles),
		MaxStagingSize:  limit(args.maxStagingMB << 20),
		MinFreeSpace:    limit(args.minFreeMB << 20),
		StagingTTL:      limit(args.stagingTTL),
		PublicURL:       args.publicURL,
	})

	// curl --request GET localhost:8077/assets/binary/vcs-gitlab-latest -o  bitlab.bin
	server.AddFileRoute("/assets/binary/{filePath}", "GET", "filePath")
	server.AddFileRoute("/assets/checksum/{filename}", "GET", "filename")
	server.AddReleaseRoutes(args.publicURL)
	server.AddProductRoutes()
	// Liveness probe (Docker HEALTHCHECK, load balancers): answers as soon as the server serves.
	server.AddRoute("/healthz", "GET", func(...string) any { return map[string]string{"status": "ok"} })
	server.AddRouteE("/{product}/artifacts", "GET", fnGetArtifacts)
	server.AddRouteE("/{product}/artifacts/{version}", "GET", fnGetArtifacts)
	server.AddRouteE("/artifacts/all", "GET", fnGetAllArtifacts)
	return server, nil
}

func fnGetAllArtifacts(vps ...string) (any, error) {
	asset := &localregistry.Asset{}

	return asset.FindAssetContents(localregistry.NewLayout(args.absAsset).AssetsDir())
}

func fnGetArtifacts(pvs ...string) (any, error) {
	var version string
	var product string
	if len(pvs) > 0 {
		product = pvs[0]
	}
	if len(pvs) > 1 {
		version = pvs[1]
	}
	artifactsFile, err := localregistry.NewLayout(args.absAsset).ArtifactsFile(product, version)
	if err != nil {
		return nil, err
	}
	file, err := os.ReadFile(artifactsFile)
	if err != nil {
		return nil, err
	}
	var res any
	if err := json.Unmarshal(file, &res); err != nil {
		return nil, err
	}
	return res, nil
}
