package command

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/merionyx/api-gateway/internal/cli/apiserver/httpapi"
	"github.com/merionyx/api-gateway/internal/cli/credentials"

	"github.com/spf13/cobra"
)

func newAuthRefreshCmd(resolveServer func() (string, error)) *cobra.Command {
	var (
		accessTTL  string
		refreshTTL string
	)
	cmd := &cobra.Command{
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
			requestedTTLs, err := requestedTTLsFromFlags(accessTTL, refreshTTL)
			if err != nil {
				return err
			}
			return runAuthRefresh(
				execCtx,
				cmd.OutOrStdout(),
				server,
				httpClient,
				ctxName,
				requestedTTLs,
				strings.TrimSpace(accessTTL) != "",
				strings.TrimSpace(refreshTTL) != "",
			)
		},
	}
	cmd.Flags().StringVar(&accessTTL, "access-ttl", "", "requested access token lifetime (Go duration, e.g. 168h)")
	cmd.Flags().StringVar(&refreshTTL, "refresh-ttl", "", "requested refresh token lifetime (Go duration, e.g. 720h)")
	return cmd
}

func runAuthRefresh(ctx context.Context, out io.Writer, server string, httpClient *http.Client, contextName string, requestedTTLs httpapi.RequestedTokenTTLs, accessExplicit, refreshExplicit bool) error {
	saved, err := credentials.GetContext(contextName)
	if err != nil {
		return err
	}
	savedTTLs, err := requestedTTLsFromCredentials(saved)
	if err != nil {
		return err
	}
	if !accessExplicit {
		requestedTTLs.AccessTTL = savedTTLs.AccessTTL
	}
	if !refreshExplicit {
		requestedTTLs.RefreshTTL = savedTTLs.RefreshTTL
	}

	tok, err := httpapi.RefreshSession(ctx, httpClient, server, saved.RefreshToken, requestedTTLs)
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
	requestedAccessTTL := saved.RequestedAccessTokenTTL
	requestedRefreshTTL := saved.RequestedRefreshTokenTTL
	if accessExplicit && requestedTTLs.AccessTTL > 0 {
		requestedAccessTTL = requestedTTLs.AccessTTL.String()
	}
	if refreshExplicit && requestedTTLs.RefreshTTL > 0 {
		requestedRefreshTTL = requestedTTLs.RefreshTTL.String()
	}

	if err := credentials.PutContext(contextName, credentials.Entry{
		ProviderID:               saved.ProviderID,
		AccessToken:              tok.AccessToken,
		RefreshToken:             tok.RefreshToken,
		TokenType:                tokenType,
		AccessExpiresAt:          tok.AccessExpiresAt.UTC().Format(time.RFC3339),
		RefreshExpiresAt:         tok.RefreshExpiresAt.UTC().Format(time.RFC3339),
		RequestedAccessTokenTTL:  requestedAccessTTL,
		RequestedRefreshTokenTTL: requestedRefreshTTL,
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
