package command

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/merionyx/api-gateway/internal/cli/apiserver/httpapi"
	"github.com/merionyx/api-gateway/internal/cli/config"
	"github.com/merionyx/api-gateway/internal/cli/credentials"

	"github.com/spf13/cobra"
)

func TestAuthRefreshCommand_Success(t *testing.T) {
	t.Setenv("AGWCTL_CREDENTIALS", filepath.Join(t.TempDir(), "credentials.yaml"))
	if err := credentials.PutContext("dev", credentials.Entry{
		ProviderID:   "google",
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		TokenType:    "Bearer",
	}); err != nil {
		t.Fatal(err)
	}

	var gotRefreshToken string
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s", r.Method)
			}
			if r.URL.String() != "https://api.example.test/api/v1/auth/token" {
				t.Fatalf("url = %s", r.URL.String())
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			form, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("parse body: %v", err)
			}
			if form.Get("grant_type") != "refresh_token" {
				t.Fatalf("body = %s", string(body))
			}
			if form.Get("refresh_token") != "old-refresh" {
				t.Fatalf("body = %s", string(body))
			}
			gotRefreshToken = "old-refresh"
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access","access_expires_at":"2026-05-01T00:00:00Z","refresh_token":"new-refresh","refresh_expires_at":"2026-05-31T00:00:00Z","token_type":"Bearer"}`)),
			}, nil
		}),
	}

	var out bytes.Buffer
	if err := runAuthRefresh(context.Background(), &out, "https://api.example.test", httpClient, "dev", httpapi.RequestedTokenTTLs{}, false, false); err != nil {
		t.Fatal(err)
	}

	if gotRefreshToken != "old-refresh" {
		t.Fatalf("refresh token = %q", gotRefreshToken)
	}
	got, err := credentials.GetContext("dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "new-access" || got.RefreshToken != "new-refresh" {
		t.Fatalf("saved credentials = %+v", got)
	}
	if !strings.Contains(out.String(), `Refreshed tokens for context "dev"`) {
		t.Fatalf("output = %q", out.String())
	}
}

func TestAuthRefreshCommand_NoSavedCredentials(t *testing.T) {
	t.Setenv("AGWCTL_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("AGWCTL_CREDENTIALS", filepath.Join(t.TempDir(), "credentials.yaml"))

	if err := config.Save(&config.File{
		CurrentContext: "dev",
		Contexts: map[string]config.ContextConfig{
			"dev": {Server: "https://example.invalid"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	root, _ := newAuthRefreshTestRoot("https://example.invalid")
	root.SetContext(context.Background())
	root.SetArgs([]string{"auth", "refresh"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `no tokens saved for context "dev"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestAuthRefreshCommand_UsesSavedRequestedTTLsByDefault(t *testing.T) {
	t.Setenv("AGWCTL_CREDENTIALS", filepath.Join(t.TempDir(), "credentials.yaml"))
	if err := credentials.PutContext("dev", credentials.Entry{
		ProviderID:               "google",
		AccessToken:              "old-access",
		RefreshToken:             "old-refresh",
		TokenType:                "Bearer",
		RequestedAccessTokenTTL:  "168h",
		RequestedRefreshTokenTTL: "720h",
	}); err != nil {
		t.Fatal(err)
	}

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			form, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("parse body: %v", err)
			}
			if form.Get("grant_type") != "refresh_token" {
				t.Fatalf("body = %s", string(body))
			}
			if form.Get("refresh_token") != "old-refresh" {
				t.Fatalf("body = %s", string(body))
			}
			if form.Get("requested_access_token_ttl_seconds") != "604800" {
				t.Fatalf("body = %s", string(body))
			}
			if form.Get("requested_refresh_token_ttl_seconds") != "2592000" {
				t.Fatalf("body = %s", string(body))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access","access_expires_at":"2026-05-01T00:00:00Z","refresh_token":"new-refresh","refresh_expires_at":"2026-05-31T00:00:00Z","token_type":"Bearer"}`)),
			}, nil
		}),
	}

	if err := runAuthRefresh(context.Background(), io.Discard, "https://api.example.test", httpClient, "dev", httpapi.RequestedTokenTTLs{}, false, false); err != nil {
		t.Fatal(err)
	}
	got, err := credentials.GetContext("dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.RequestedAccessTokenTTL != "168h" || got.RequestedRefreshTokenTTL != "720h" {
		t.Fatalf("saved credentials = %+v", got)
	}
}

func TestAuthRefreshCommand_UsesBuiltInRequestedTTLsWhenSavedAreMissing(t *testing.T) {
	t.Setenv("AGWCTL_CREDENTIALS", filepath.Join(t.TempDir(), "credentials.yaml"))
	if err := credentials.PutContext("dev", credentials.Entry{
		ProviderID:   "google",
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		TokenType:    "Bearer",
	}); err != nil {
		t.Fatal(err)
	}

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			form, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("parse body: %v", err)
			}
			if form.Get("grant_type") != "refresh_token" {
				t.Fatalf("body = %s", string(body))
			}
			if form.Get("refresh_token") != "old-refresh" {
				t.Fatalf("body = %s", string(body))
			}
			if form.Get("requested_access_token_ttl_seconds") != "604800" {
				t.Fatalf("body = %s", string(body))
			}
			if form.Get("requested_refresh_token_ttl_seconds") != "2592000" {
				t.Fatalf("body = %s", string(body))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access","access_expires_at":"2026-05-01T00:00:00Z","refresh_token":"new-refresh","refresh_expires_at":"2026-05-31T00:00:00Z","token_type":"Bearer"}`)),
			}, nil
		}),
	}

	if err := runAuthRefresh(context.Background(), io.Discard, "https://api.example.test", httpClient, "dev", httpapi.RequestedTokenTTLs{}, false, false); err != nil {
		t.Fatal(err)
	}
	got, err := credentials.GetContext("dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.RequestedAccessTokenTTL != "168h" || got.RequestedRefreshTokenTTL != "720h" {
		t.Fatalf("saved credentials = %+v", got)
	}
}

func TestNewAuthCommand_IncludesRefreshSubcommand(t *testing.T) {
	cmd := NewAuthCommand(func() (string, error) { return "https://api.example.test", nil })
	found, _, err := cmd.Find([]string{"refresh"})
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.Use != "refresh" {
		t.Fatalf("found = %#v", found)
	}
}

func newAuthRefreshTestRoot(server string) (*cobra.Command, *bytes.Buffer) {
	var out bytes.Buffer
	root := &cobra.Command{Use: "agwctl", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("context", "", "")
	root.PersistentFlags().Bool("insecure", false, "")
	root.PersistentFlags().String("ca-cert", "", "")
	root.SetOut(&out)
	root.SetErr(&out)
	root.AddCommand(NewAuthCommand(func() (string, error) { return server, nil }))
	return root, &out
}
