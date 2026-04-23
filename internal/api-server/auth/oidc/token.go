package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// TokenResponse is the subset of OAuth2 token response used by OIDC authorization code flow.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token"`
	Scope        string `json:"scope,omitempty"`
}

// ExchangeAuthorizationCode calls the token endpoint (grant_type=authorization_code, client_secret in body).
// codeVerifier is the PKCE code_verifier when using S256 (RFC 7636); pass empty if PKCE is not used.
func ExchangeAuthorizationCode(ctx context.Context, hc *http.Client, tokenEndpoint, clientID, clientSecret, code, redirectURI, codeVerifier string) (*TokenResponse, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	if strings.TrimSpace(codeVerifier) != "" {
		form.Set("code_verifier", codeVerifier)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTokenExchange, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: read: %w", ErrTokenExchange, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d body=%s", ErrTokenExchange, resp.StatusCode, truncateForErr(body, 512))
	}
	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("%w: json: %w", ErrTokenExchange, err)
	}
	if tr.IDToken == "" {
		return nil, fmt.Errorf("%w: missing id_token", ErrTokenExchange)
	}
	return &tr, nil
}

func truncateForErr(b []byte, max int) string {
	s := string(b)
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
