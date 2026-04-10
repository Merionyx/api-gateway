// Package cli is the root command for agwctl (Merionyx API Gateway CLI).
package cli

import (
	"fmt"
	"os"

	"github.com/merionyx/api-gateway/internal/cli/command"
	"github.com/merionyx/api-gateway/internal/cli/config"

	"github.com/spf13/cobra"
)

var (
	contextName    string
	serverOverride string
)

// Execute runs the root cobra command (os.Exit on error).
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "agwctl",
	Short: "Merionyx API Gateway control CLI",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&contextName, "context", "", "config context name (see ~/.config/agwctl/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&serverOverride, "server", "", "API Server base URL, e.g. http://127.0.0.1:8080 (overrides context)")
	rootCmd.AddCommand(command.NewContractCommand(func() (string, error) {
		return config.ResolveServerURL(contextName, serverOverride)
	}))
}
