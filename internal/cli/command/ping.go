package command

import (
	"context"
	"fmt"

	"github.com/merionyx/api-gateway/internal/cli/apiclient"

	"github.com/spf13/cobra"
)

// NewPingCommand builds `agwctl ping` (GET /health — liveness — on the API Server).
func NewPingCommand(resolveServer func() (string, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Check connectivity to the API Server (HTTP GET /health, liveness)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, err := resolveServer()
			if err != nil {
				return err
			}
			httpClient, err := httpClientFromCmd(cmd)
			if err != nil {
				return err
			}
			if err := apiclient.PingDefaultTimeout(context.Background(), httpClient, server); err != nil {
				return fmt.Errorf("ping %s: %w", server, err)
			}
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintln(out)
			_, _ = fmt.Fprintf(out, "ok %s\n", server)
			return nil
		},
	}
}
