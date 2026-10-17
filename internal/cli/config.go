package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	localregistry "github.com/nivekalara237/local-registry"
)

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
		{"public-url", a.publicURL},
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
