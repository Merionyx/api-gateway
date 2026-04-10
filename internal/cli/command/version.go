package command

import (
	"fmt"

	"github.com/merionyx/api-gateway/internal/cli/version"

	"github.com/spf13/cobra"
)

// NewVersionCommand builds `agwctl version`.
func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print agwctl version and build metadata",
		Run: func(cmd *cobra.Command, _ []string) {
			out := cmd.OutOrStdout()
			fmt.Fprintln(out)
			fmt.Fprintln(out, version.Details())
		},
	}
}
