package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Discovery is a subset of OpenID Provider Metadata (RFC 8414 / OIDC Discovery).
type Discovery struct {
	Issuer                            string `json:"issuer"`
	AuthorizationEndpoint             string `json:"authorization_endpoint"`
	TokenEndpoint                     string `json:"token_endpoint"`
	JWKSURI                           string `json:"jwks_uri"`
	UserinfoEndpoint                  string `json:"userinfo_endpoint,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
}

// FetchDiscovery loads /.well-known/openid-configuration for issuerBase (scheme+host, no trailing slash).
func FetchDiscovery(ctx context.Context, hc *http.Client, issuerBase string) (*Discovery, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	base := strings.TrimSuffix(strings.TrimSpace(issuerBase), "/")
	if base == "" {
		return nil, fmt.Errorf("%w: empty issuer", ErrDiscovery)
	}
	u := base + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDiscovery, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: read: %w", ErrDiscovery, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrDiscovery, resp.StatusCode)
	}
	var d Discovery
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, fmt.Errorf("%w: json: %w", ErrDiscovery, err)
	}
	patchWellKnownIncomplete(&d)
	if d.Issuer == "" || d.TokenEndpoint == "" || d.JWKSURI == "" {
		return nil, fmt.Errorf("%w: missing required fields", ErrDiscovery)
	}
	return &d, nil
}

// patchWellKnownIncomplete fills endpoints some providers omit from openid-configuration.
// GitHub documents issuer "https://github.com" but serves metadata under /login/oauth and
// does not include authorization_endpoint / token_endpoint in that JSON.
func patchWellKnownIncomplete(d *Discovery) {
	if d == nil {
		return
	}
	if d.Issuer != "https://github.com" || d.JWKSURI == "" {
		return
	}
	if d.AuthorizationEndpoint == "" {
		d.AuthorizationEndpoint = "https://github.com/login/oauth/authorize"
	}
	if d.TokenEndpoint == "" {
		d.TokenEndpoint = "https://github.com/login/oauth/access_token"
	}
}
