package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/auth/oidc"
	"github.com/merionyx/api-gateway/internal/api-server/auth/pkce"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

type stubIntentStore struct {
	last         kvvalue.LoginIntentValue
	lastIntentID string
}

func (s *stubIntentStore) Create(_ context.Context, intentID string, v kvvalue.LoginIntentValue, _ time.Duration) error {
	s.lastIntentID = intentID
	s.last = v
	return nil
}

func TestOIDCLoginUseCase_Start_RedirectNotAllowlisted(t *testing.T) {
	t.Parallel()
	stub := &stubIntentStore{}
	uc := NewOIDCLoginUseCase([]config.OIDCProviderConfig{{
		ID:                   "p1",
		Issuer:               "https://issuer.unused.example",
		ClientID:             "c",
		RedirectURIAllowlist: []string{"http://127.0.0.1:8080/cb"},
	}}, time.Minute, stub, http.DefaultClient)
	_, err := uc.Start(t.Context(), "p1", "http://127.0.0.1:9999/wrong", "")
	if !errors.Is(err, apierrors.ErrOIDCRedirectNotAllowlisted) {
		t.Fatalf("got %v", err)
	}
}

func TestOIDCLoginUseCase_Start_HappyPath(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			base := "http://" + r.Host
			_ = json.NewEncoder(w).Encode(oidc.Discovery{
				Issuer:                base,
				AuthorizationEndpoint: base + "/authorize",
				TokenEndpoint:         base + "/token",
				JWKSURI:               base + "/jwks",
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	stub := &stubIntentStore{}
	uc := NewOIDCLoginUseCase([]config.OIDCProviderConfig{{
		ID:                   "p1",
		Issuer:               srv.URL,
		ClientID:             "cid",
		RedirectURIAllowlist: []string{"http://127.0.0.1:8080/cb"},
	}}, time.Minute, stub, srv.Client())

	loc, err := uc.Start(t.Context(), "p1", "http://127.0.0.1:8080/cb", "n1")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if !strings.Contains(u.Path, "authorize") {
		t.Fatalf("path %q", u.Path)
	}
	if q.Get("client_id") != "cid" || q.Get("redirect_uri") != "http://127.0.0.1:8080/cb" {
		t.Fatalf("query %v", q)
	}
	if q.Get("nonce") != "n1" {
		t.Fatalf("nonce %q", q.Get("nonce"))
	}
	wantChal := pkce.ChallengeS256(stub.last.PKCEVerifier)
	if q.Get("code_challenge") != wantChal {
		t.Fatalf("code_challenge got %q want %q", q.Get("code_challenge"), wantChal)
	}
	if q.Get("state") != stub.lastIntentID || stub.lastIntentID != stub.last.OAuthState {
		t.Fatalf("state: query=%q intent_id=%q oauth_state=%q", q.Get("state"), stub.lastIntentID, stub.last.OAuthState)
	}
	if stub.last.OAuthState == "" || stub.last.PKCEVerifier == "" {
		t.Fatalf("intent: %+v", stub.last)
	}
}

func TestApplyProviderAuthorizeParams_googleAddsOfflineRefreshParams(t *testing.T) {
	t.Parallel()

	q := url.Values{}
	applyProviderAuthorizeParams(q, config.OIDCProviderConfig{
		ID:   "google",
		Kind: "google",
	})
	if q.Get("access_type") != "offline" {
		t.Fatalf("access_type=%q", q.Get("access_type"))
	}
	if q.Get("include_granted_scopes") != "true" {
		t.Fatalf("include_granted_scopes=%q", q.Get("include_granted_scopes"))
	}
	if q.Get("prompt") != "consent" {
		t.Fatalf("prompt=%q", q.Get("prompt"))
	}
}
