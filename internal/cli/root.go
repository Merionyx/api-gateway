// Package cli is the root command for agwctl (Merionyx API Gateway CLI).
package cli

import (
	"fmt"
	"os"

	"github.com/merionyx/api-gateway/internal/cli/command"
	"github.com/merionyx/api-gateway/internal/cli/config"
	"github.com/merionyx/api-gateway/internal/cli/version"

	"github.com/spf13/cobra"
)

var (
	contextName    string
	serverOverride string
	tlsInsecure    bool
	tlsCACert      string
)

// Execute runs the root cobra command (os.Exit on error).
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:           "agwctl",
	Short:         "\nMerionyx API Gateway control CLI",
	SilenceUsage:  true,
	SilenceErrors: true, // print errors only once in Execute (stderr), not again as "Error:" from cobra
}

func init() {
	rootCmd.Version = version.Line()
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	rootCmd.PersistentFlags().StringVar(&contextName, "context", "", "config context name (see ~/.config/agwctl/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&serverOverride, "server", "", "API Server base URL, e.g. http://127.0.0.1:8080 (overrides context)")
	rootCmd.PersistentFlags().BoolVar(&tlsInsecure, "insecure", false, "skip TLS certificate verification for HTTPS (unsafe; only when you must)")
	rootCmd.PersistentFlags().StringVar(&tlsCACert, "ca-cert", "", "path to PEM file with extra CA certificate(s) for HTTPS (e.g. corporate CA); not used with --insecure")
	resolveServer := func() (string, error) {
		return config.ResolveServerURL(contextName, serverOverride)
	}
	rootCmd.AddCommand(command.NewContractCommand(resolveServer))
	rootCmd.AddCommand(command.NewPingCommand(resolveServer))
	rootCmd.AddCommand(command.NewConfigCommand())
	rootCmd.AddCommand(command.NewVersionCommand())
	rootCmd.AddCommand(command.NewValidateCommand())
	rootCmd.AddCommand(command.NewListCommand(resolveServer))
	rootCmd.AddCommand(command.NewDescribeCommand(resolveServer))
}
