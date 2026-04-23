package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

func TestGithubExtraRoles_orgDenied(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/orgs":
			_ = json.NewEncoder(w).Encode([]map[string]string{{"login": "other-org"}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	_, err := githubExtraRoles(t.Context(), srv.Client(), &config.GitHubOIDCProviderConfig{
		AllowedOrgLogins: []string{"acme"},
	}, "oauth-token", srv.URL)
	if !errors.Is(err, apierrors.ErrGitHubLoginDenied) {
		t.Fatalf("got %v", err)
	}
}

func TestGithubExtraRoles_teamBindings(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/teams":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"slug": "platform",
				"organization": map[string]string{"login": "acme"},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	roles, err := githubExtraRoles(t.Context(), srv.Client(), &config.GitHubOIDCProviderConfig{
		TeamRoleBindings: []config.GitHubTeamRoleBinding{{
			Org: "acme", TeamSlug: "platform", Roles: []string{"api:contracts:export", "api:access_tokens:issue"},
		}},
	}, "oauth-token", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 2 {
		t.Fatalf("roles=%v", roles)
	}
}

func TestClaimsSnapshotFromProvider_generic(t *testing.T) {
	t.Parallel()
	mc := jwt.MapClaims{"sub": "u1", "email": "a@b.c"}
	raw, err := claimsSnapshotFromProvider(context.Background(), config.OIDCProviderConfig{}, mc, "", http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	roles, _ := m["roles"].([]any)
	if len(roles) != 1 || roles[0] != "api:member" {
		t.Fatalf("%v", roles)
	}
}

func TestClaimsSnapshotFromProvider_githubNoExtraCalls(t *testing.T) {
	t.Parallel()
	mc := jwt.MapClaims{"sub": "1", "exp": float64(time.Now().Add(time.Hour).Unix())}
	_, err := claimsSnapshotFromProvider(context.Background(), config.OIDCProviderConfig{Kind: "github"}, mc, "", http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
}
