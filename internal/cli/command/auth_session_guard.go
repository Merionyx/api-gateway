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

const tokenExpiryLeeway = 10 * time.Second

var (
	nowUTC = func() time.Time { return time.Now().UTC() }

	sessionRefresh = func(ctx context.Context, out io.Writer, server string, baseHTTP *http.Client, contextName string) error {
		return runAuthRefresh(ctx, out, server, baseHTTP, contextName, httpapi.RequestedTokenTTLs{}, false, false)
	}

	sessionLogin = func(ctx context.Context, out io.Writer, server string, baseHTTP *http.Client, contextName, providerID string) error {
		return runAuthLogin(
			ctx,
			out,
			server,
			baseHTTP,
			contextName,
			providerID,
			defaultCallbackHost,
			defaultCallbackPort,
			false,
			"",
			"",
		)
	}
)

func authorizedHTTPClientFromCmd(cmd *cobra.Command, server string) (*http.Client, error) {
	baseHTTP, err := httpClientFromCmd(cmd)
	if err != nil {
		return nil, err
	}
	ctxName, err := effectiveContextName(cmd)
	if err != nil {
		return nil, err
	}

	execCtx := cmd.Context()
	if execCtx == nil {
		execCtx = context.Background()
	}
	saved, err := ensureAuthorizedSession(execCtx, cmd.OutOrStdout(), server, baseHTTP, ctxName)
	if err != nil {
		return nil, err
	}
	return withAuthorizationHeader(baseHTTP, saved.TokenType, saved.AccessToken), nil
}

func ensureAuthorizedSession(ctx context.Context, out io.Writer, server string, baseHTTP *http.Client, contextName string) (credentials.Entry, error) {
	saved, err := credentials.GetContext(contextName)
	if err != nil {
		return credentials.Entry{}, err
	}

	now := nowUTC()
	accessExp, err := parseSavedExpiry("access_expires_at", saved.AccessExpiresAt)
	if err != nil {
		return credentials.Entry{}, err
	}
	refreshExp, err := parseSavedExpiry("refresh_expires_at", saved.RefreshExpiresAt)
	if err != nil {
		return credentials.Entry{}, err
	}

	if isExpired(refreshExp, now) {
		if err := sessionLogin(ctx, out, server, baseHTTP, contextName, strings.TrimSpace(saved.ProviderID)); err != nil {
			return credentials.Entry{}, fmt.Errorf("auto login: %w", err)
		}
	} else if isExpired(accessExp, now) {
		if err := sessionRefresh(ctx, out, server, baseHTTP, contextName); err != nil {
			if isRefreshExpiredError(err) {
				if lerr := sessionLogin(ctx, out, server, baseHTTP, contextName, strings.TrimSpace(saved.ProviderID)); lerr != nil {
					return credentials.Entry{}, fmt.Errorf("auto refresh failed (%v) and auto login failed: %w", err, lerr)
				}
			} else {
				return credentials.Entry{}, fmt.Errorf("auto refresh: %w", err)
			}
		}
	}

	fresh, err := credentials.GetContext(contextName)
	if err != nil {
		return credentials.Entry{}, err
	}
	if strings.TrimSpace(fresh.AccessToken) == "" {
		return credentials.Entry{}, fmt.Errorf("credentials: empty access_token for context %q; run `agwctl auth login`", contextName)
	}
	return fresh, nil
}

func isRefreshExpiredError(err error) bool {
	if err == nil {
		return false
	}
	// API problem code from POST /api/v1/auth/refresh.
	return strings.Contains(err.Error(), "SESSION_REFRESH_EXPIRED")
}

func parseSavedExpiry(field, value string) (time.Time, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return time.Time{}, nil
	}
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("credentials: invalid %s %q: %w", field, s, err)
	}
	return tm.UTC(), nil
}

func isExpired(exp, now time.Time) bool {
	return !exp.After(now.Add(tokenExpiryLeeway))
}

func withAuthorizationHeader(baseHTTP *http.Client, tokenType, accessToken string) *http.Client {
	token := strings.TrimSpace(accessToken)
	tt := strings.TrimSpace(tokenType)
	if tt == "" {
		tt = "Bearer"
	}
	baseTransport := http.DefaultTransport
	if baseHTTP != nil && baseHTTP.Transport != nil {
		baseTransport = baseHTTP.Transport
	}

	cloned := &http.Client{
		Transport: &authorizationHeaderTransport{
			base:      baseTransport,
			tokenType: tt,
			token:     token,
		},
	}
	if baseHTTP != nil {
		cloned.Timeout = baseHTTP.Timeout
		cloned.Jar = baseHTTP.Jar
	}
	if baseHTTP != nil && baseHTTP.CheckRedirect != nil {
		cloned.CheckRedirect = baseHTTP.CheckRedirect
	}
	return cloned
}

type authorizationHeaderTransport struct {
	base      http.RoundTripper
	tokenType string
	token     string
}

func (t *authorizationHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Header.Set("Authorization", t.tokenType+" "+t.token)
	return base.RoundTrip(clone)
}
