package command

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/merionyx/api-gateway/internal/cli/apiserver/httpapi"
	"github.com/merionyx/api-gateway/internal/cli/credentials"

	"github.com/spf13/cobra"
)

func newAuthRefreshCmd(resolveServer func() (string, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Refresh saved API tokens for the current agwctl context",
		Long: strings.TrimSpace(`
Calls POST /api/v1/auth/refresh with the refresh token saved in ~/.config/agwctl/credentials.yaml
(or AGWCTL_CREDENTIALS), using the current agwctl context unless --context is set.

On success, agwctl overwrites the saved token pair for that context and keeps the provider id.
`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctxName, err := effectiveContextName(cmd)
			if err != nil {
				return err
			}
			server, err := resolveServer()
			if err != nil {
				return err
			}
			httpClient, err := httpClientFromCmd(cmd)
			if err != nil {
				return err
			}
			execCtx := cmd.Context()
			if execCtx == nil {
				execCtx = context.Background()
			}
			return runAuthRefresh(execCtx, cmd.OutOrStdout(), server, httpClient, ctxName)
		},
	}
}

func runAuthRefresh(ctx context.Context, out io.Writer, server string, httpClient *http.Client, contextName string) error {
	saved, err := credentials.GetContext(contextName)
	if err != nil {
		return err
	}

	tok, err := httpapi.RefreshSession(ctx, httpClient, server, saved.RefreshToken)
	if err != nil {
		return fmt.Errorf("refresh session: %w", err)
	}

	tokenType := strings.TrimSpace(saved.TokenType)
	if tok.TokenType != nil && strings.TrimSpace(*tok.TokenType) != "" {
		tokenType = strings.TrimSpace(*tok.TokenType)
	}
	if tokenType == "" {
		tokenType = "Bearer"
	}

	if err := credentials.PutContext(contextName, credentials.Entry{
		ProviderID:   saved.ProviderID,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tokenType,
	}); err != nil {
		return err
	}
	credPath, perr := credentials.Path()
	if perr != nil {
		credPath = "(credentials path)"
	}
	_, _ = fmt.Fprintf(out, "Refreshed tokens for context %q and saved them to %s.\n", contextName, credPath)
	return nil
}
