package command

import (
	"github.com/spf13/cobra"
)

// NewAuthCommand builds `agwctl auth ...` (interactive OIDC login; roadmap ш. 32).
func NewAuthCommand(resolveServer func() (string, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Interactive authentication (OIDC browser login)",
	}
	cmd.AddCommand(newAuthLoginCmd(resolveServer))
	return cmd
}
