package command

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
	apiserverclient "github.com/merionyx/api-gateway/internal/cli/apiserver/client"
	"github.com/merionyx/api-gateway/internal/cli/style"
	"golang.org/x/term"
)

func providerDisplayName(p apiserverclient.OidcProviderDescriptor) string {
	return strings.TrimSpace(p.Name)
}

func providerSelectLabel(p apiserverclient.OidcProviderDescriptor) string {
	name := providerDisplayName(p)
	id := strings.TrimSpace(p.Id)
	return fmt.Sprintf("%s  %s", name, style.S(true, style.Dim, fmt.Sprintf("(%s)", id)))
}

// resolveAuthLoginProviderID returns explicit non-empty id, or fetches GET /api/v1/auth/oidc-providers
// and picks one (single provider auto-selected; multiple + TTY → arrow-key prompt; multiple + non-TTY → error).
func resolveAuthLoginProviderID(ctx context.Context, server string, httpClient *http.Client, explicit string, out io.Writer) (string, error) {
	if s := strings.TrimSpace(explicit); s != "" {
		return s, nil
	}
	cw, err := apiserverclient.NewClientWithResponses(server, apiserverclient.WithHTTPClient(httpClient))
	if err != nil {
		return "", err
	}
	resp, err := cw.ListOidcProvidersWithResponse(ctx)
	if err != nil {
		return "", fmt.Errorf("list OIDC providers: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("list OIDC providers: HTTP %d", resp.StatusCode())
	}
	if resp.JSON200 == nil {
		return "", fmt.Errorf("list OIDC providers: unexpected response body")
	}
	providers := *resp.JSON200
	if len(providers) == 0 {
		return "", fmt.Errorf("API Server has no configured OIDC providers (auth.oidc_providers is empty)")
	}
	if len(providers) == 1 {
		id := strings.TrimSpace(providers[0].Id)
		_, _ = fmt.Fprintf(out, "Using the only configured provider %q [%s] (%s).\n", providerDisplayName(providers[0]), id, providers[0].Kind)
		return id, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return "", fmt.Errorf("multiple OIDC providers configured; choose one with --provider-id (stdin/stdout is not a TTY)")
	}
	labels := make([]string, len(providers))
	for i, p := range providers {
		labels[i] = providerSelectLabel(p)
	}
	fmt.Fprintln(out, "")
	prompt := promptui.Select{
		Label: "Select login provider",
		Items: labels,
		Size:  12,
	}
	idx, _, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(providers[idx].Id), nil
}
