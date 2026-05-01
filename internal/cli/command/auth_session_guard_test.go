package command

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/cli/credentials"
)

func TestEnsureAuthorizedSession_AccessExpiredRefreshes(t *testing.T) {
	t.Setenv("AGWCTL_CREDENTIALS", filepath.Join(t.TempDir(), "credentials.yaml"))
	now := time.Date(2026, 4, 27, 15, 0, 0, 0, time.UTC)

	if err := credentials.PutContext("dev", credentials.Entry{
		ProviderID:               "github",
		AccessToken:              "old-access",
		RefreshToken:             "refresh",
		TokenType:                "Bearer",
		AccessExpiresAt:          now.Add(-1 * time.Minute).Format(time.RFC3339),
		RefreshExpiresAt:         now.Add(30 * time.Minute).Format(time.RFC3339),
		RequestedAccessTokenTTL:  "168h",
		RequestedRefreshTokenTTL: "720h",
	}); err != nil {
		t.Fatal(err)
	}

	prevNowUTC := nowUTC
	prevRefresh := sessionRefresh
	prevLogin := sessionLogin
	t.Cleanup(func() {
		nowUTC = prevNowUTC
		sessionRefresh = prevRefresh
		sessionLogin = prevLogin
	})

	nowUTC = func() time.Time { return now }
	refreshCalled := false
	sessionRefresh = func(_ context.Context, _ io.Writer, _ string, _ *http.Client, contextName string) error {
		refreshCalled = true
		return credentials.PutContext(contextName, credentials.Entry{
			ProviderID:               "github",
			AccessToken:              "new-access",
			RefreshToken:             "new-refresh",
			TokenType:                "Bearer",
			AccessExpiresAt:          now.Add(15 * time.Minute).Format(time.RFC3339),
			RefreshExpiresAt:         now.Add(30 * time.Minute).Format(time.RFC3339),
			RequestedAccessTokenTTL:  "168h",
			RequestedRefreshTokenTTL: "720h",
		})
	}
	sessionLogin = func(_ context.Context, _ io.Writer, _ string, _ *http.Client, _, _ string) error {
		t.Fatal("sessionLogin must not be called when refresh token is still valid")
		return nil
	}

	got, err := ensureAuthorizedSession(context.Background(), io.Discard, "https://api.example.test", &http.Client{}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !refreshCalled {
		t.Fatal("expected sessionRefresh to be called")
	}
	if got.AccessToken != "new-access" {
		t.Fatalf("access token = %q", got.AccessToken)
	}
}

func TestEnsureAuthorizedSession_RefreshExpiredTriggersLogin(t *testing.T) {
	t.Setenv("AGWCTL_CREDENTIALS", filepath.Join(t.TempDir(), "credentials.yaml"))
	now := time.Date(2026, 4, 27, 15, 0, 0, 0, time.UTC)

	if err := credentials.PutContext("dev", credentials.Entry{
		ProviderID:               "github",
		AccessToken:              "old-access",
		RefreshToken:             "old-refresh",
		TokenType:                "Bearer",
		AccessExpiresAt:          now.Add(-1 * time.Minute).Format(time.RFC3339),
		RefreshExpiresAt:         now.Add(-10 * time.Second).Format(time.RFC3339),
		RequestedAccessTokenTTL:  "168h",
		RequestedRefreshTokenTTL: "720h",
	}); err != nil {
		t.Fatal(err)
	}

	prevNowUTC := nowUTC
	prevRefresh := sessionRefresh
	prevLogin := sessionLogin
	t.Cleanup(func() {
		nowUTC = prevNowUTC
		sessionRefresh = prevRefresh
		sessionLogin = prevLogin
	})

	nowUTC = func() time.Time { return now }
	loginCalled := false
	sessionRefresh = func(_ context.Context, _ io.Writer, _ string, _ *http.Client, _ string) error {
		t.Fatal("sessionRefresh must not be called when refresh token is expired")
		return nil
	}
	sessionLogin = func(_ context.Context, _ io.Writer, _ string, _ *http.Client, contextName, providerID string) error {
		loginCalled = true
		if providerID != "github" {
			t.Fatalf("providerID = %q", providerID)
		}
		return credentials.PutContext(contextName, credentials.Entry{
			ProviderID:               "github",
			AccessToken:              "post-login-access",
			RefreshToken:             "post-login-refresh",
			TokenType:                "Bearer",
			AccessExpiresAt:          now.Add(20 * time.Minute).Format(time.RFC3339),
			RefreshExpiresAt:         now.Add(3 * time.Hour).Format(time.RFC3339),
			RequestedAccessTokenTTL:  "168h",
			RequestedRefreshTokenTTL: "720h",
		})
	}

	got, err := ensureAuthorizedSession(context.Background(), io.Discard, "https://api.example.test", &http.Client{}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !loginCalled {
		t.Fatal("expected sessionLogin to be called")
	}
	if got.AccessToken != "post-login-access" {
		t.Fatalf("access token = %q", got.AccessToken)
	}
}

func TestEnsureAuthorizedSession_RefreshExpiredByServerFallsBackToLogin(t *testing.T) {
	t.Setenv("AGWCTL_CREDENTIALS", filepath.Join(t.TempDir(), "credentials.yaml"))
	now := time.Date(2026, 4, 27, 15, 0, 0, 0, time.UTC)

	if err := credentials.PutContext("dev", credentials.Entry{
		ProviderID:               "github",
		AccessToken:              "old-access",
		RefreshToken:             "old-refresh",
		TokenType:                "Bearer",
		AccessExpiresAt:          now.Add(-1 * time.Minute).Format(time.RFC3339),
		RefreshExpiresAt:         now.Add(1 * time.Hour).Format(time.RFC3339),
		RequestedAccessTokenTTL:  "168h",
		RequestedRefreshTokenTTL: "720h",
	}); err != nil {
		t.Fatal(err)
	}

	prevNowUTC := nowUTC
	prevRefresh := sessionRefresh
	prevLogin := sessionLogin
	t.Cleanup(func() {
		nowUTC = prevNowUTC
		sessionRefresh = prevRefresh
		sessionLogin = prevLogin
	})

	nowUTC = func() time.Time { return now }
	sessionRefresh = func(_ context.Context, _ io.Writer, _ string, _ *http.Client, _ string) error {
		return fmt.Errorf("refresh session: api: SESSION_REFRESH_EXPIRED: expired")
	}
	loginCalled := false
	sessionLogin = func(_ context.Context, _ io.Writer, _ string, _ *http.Client, contextName, _ string) error {
		loginCalled = true
		return credentials.PutContext(contextName, credentials.Entry{
			ProviderID:               "github",
			AccessToken:              "post-login-access",
			RefreshToken:             "post-login-refresh",
			TokenType:                "Bearer",
			AccessExpiresAt:          now.Add(20 * time.Minute).Format(time.RFC3339),
			RefreshExpiresAt:         now.Add(3 * time.Hour).Format(time.RFC3339),
			RequestedAccessTokenTTL:  "168h",
			RequestedRefreshTokenTTL: "720h",
		})
	}

	got, err := ensureAuthorizedSession(context.Background(), io.Discard, "https://api.example.test", &http.Client{}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !loginCalled {
		t.Fatal("expected sessionLogin fallback to be called")
	}
	if got.AccessToken != "post-login-access" {
		t.Fatalf("access token = %q", got.AccessToken)
	}
}

func TestEnsureAuthorizedSession_ValidTokensNoop(t *testing.T) {
	t.Setenv("AGWCTL_CREDENTIALS", filepath.Join(t.TempDir(), "credentials.yaml"))
	now := time.Date(2026, 4, 27, 15, 0, 0, 0, time.UTC)

	if err := credentials.PutContext("dev", credentials.Entry{
		ProviderID:               "github",
		AccessToken:              "access",
		RefreshToken:             "refresh",
		TokenType:                "Bearer",
		AccessExpiresAt:          now.Add(30 * time.Minute).Format(time.RFC3339),
		RefreshExpiresAt:         now.Add(2 * time.Hour).Format(time.RFC3339),
		RequestedAccessTokenTTL:  "168h",
		RequestedRefreshTokenTTL: "720h",
	}); err != nil {
		t.Fatal(err)
	}

	prevNowUTC := nowUTC
	prevRefresh := sessionRefresh
	prevLogin := sessionLogin
	t.Cleanup(func() {
		nowUTC = prevNowUTC
		sessionRefresh = prevRefresh
		sessionLogin = prevLogin
	})
	nowUTC = func() time.Time { return now }
	sessionRefresh = func(_ context.Context, _ io.Writer, _ string, _ *http.Client, _ string) error {
		t.Fatal("sessionRefresh must not be called for valid tokens")
		return nil
	}
	sessionLogin = func(_ context.Context, _ io.Writer, _ string, _ *http.Client, _, _ string) error {
		t.Fatal("sessionLogin must not be called for valid tokens")
		return nil
	}

	got, err := ensureAuthorizedSession(context.Background(), io.Discard, "https://api.example.test", &http.Client{}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "access" {
		t.Fatalf("access token = %q", got.AccessToken)
	}
}

func TestWithAuthorizationHeader_SetsHeader(t *testing.T) {
	base := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
				t.Fatalf("Authorization = %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	client := withAuthorizationHeader(base, "", "token-123")
	req, err := http.NewRequest(http.MethodGet, "https://api.example.test/v1/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}
