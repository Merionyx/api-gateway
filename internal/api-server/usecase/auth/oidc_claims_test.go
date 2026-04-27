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
				"slug":         "platform",
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
	if len(roles) != 1 || roles[0] != "api:role:viewer" {
		t.Fatalf("%v", roles)
	}
}

func TestGoogleExtraRoles_hdDenied(t *testing.T) {
	t.Parallel()
	mc := jwt.MapClaims{"hd": "other.com", "email": "u@other.com"}
	_, err := googleExtraRoles(&config.GoogleOIDCProviderConfig{
		AllowedHostedDomains: []string{"example.com"},
	}, mc)
	if !errors.Is(err, apierrors.ErrGoogleLoginDenied) {
		t.Fatalf("got %v", err)
	}
}

func TestGoogleExtraRoles_hdBindings(t *testing.T) {
	t.Parallel()
	mc := jwt.MapClaims{"hd": "example.com", "email": "u@example.com"}
	roles, err := googleExtraRoles(&config.GoogleOIDCProviderConfig{
		HostedDomainRoleBindings: []config.GoogleHostedDomainRoleBinding{{
			HD:    "example.com",
			Roles: []string{"api:role:admin"},
		}},
	}, mc)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0] != "api:role:admin" {
		t.Fatalf("%v", roles)
	}
}

func TestGoogleExtraRoles_emailDomainGate(t *testing.T) {
	t.Parallel()
	mc := jwt.MapClaims{"email": "u@example.com"}
	_, err := googleExtraRoles(&config.GoogleOIDCProviderConfig{
		AllowedEmailDomains: []string{"other.org"},
	}, mc)
	if !errors.Is(err, apierrors.ErrGoogleLoginDenied) {
		t.Fatalf("got %v", err)
	}
	roles, err := googleExtraRoles(&config.GoogleOIDCProviderConfig{
		AllowedEmailDomains: []string{"example.com"},
		EmailDomainRoleBindings: []config.GoogleEmailDomainRoleBinding{{
			Domain: "example.com",
			Roles:  []string{"api:contracts:export"},
		}},
	}, mc)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 {
		t.Fatalf("%v", roles)
	}
}

func TestOktaExtraRoles_gateDenied(t *testing.T) {
	t.Parallel()
	mc := jwt.MapClaims{"groups": []any{"Other"}}
	_, err := oktaExtraRoles(&config.OktaOIDCProviderConfig{
		AllowedIDTokenGroups: []string{"API-Admins"},
	}, mc)
	if !errors.Is(err, apierrors.ErrOktaLoginDenied) {
		t.Fatalf("got %v", err)
	}
}

func TestOktaExtraRoles_noGroupsClaimWhenRequired(t *testing.T) {
	t.Parallel()
	mc := jwt.MapClaims{"sub": "u1"}
	_, err := oktaExtraRoles(&config.OktaOIDCProviderConfig{
		AllowedIDTokenGroups: []string{"API-Admins"},
	}, mc)
	if !errors.Is(err, apierrors.ErrOktaLoginDenied) {
		t.Fatalf("got %v", err)
	}
}

func TestOktaExtraRoles_bindings(t *testing.T) {
	t.Parallel()
	mc := jwt.MapClaims{"groups": []any{"API-Admins", "Everyone"}}
	roles, err := oktaExtraRoles(&config.OktaOIDCProviderConfig{
		GroupRoleBindings: []config.OktaGroupRoleBinding{{
			GroupName: "API-Admins",
			Roles:     []string{"api:role:admin"},
		}},
	}, mc)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0] != "api:role:admin" {
		t.Fatalf("%v", roles)
	}
}

func TestIdTokenStringArrayClaim_string(t *testing.T) {
	t.Parallel()
	got := idTokenStringArrayClaim(jwt.MapClaims{"groups": " Solo "}, "groups")
	if len(got) != 1 || got[0] != "Solo" {
		t.Fatalf("%v", got)
	}
}

func TestEntraExtraRoles_tenantDenied(t *testing.T) {
	t.Parallel()
	mc := jwt.MapClaims{"tid": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}
	_, err := entraExtraRoles(&config.EntraOIDCProviderConfig{
		AllowedTenantIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
	}, mc)
	if !errors.Is(err, apierrors.ErrEntraLoginDenied) {
		t.Fatalf("got %v", err)
	}
}

func TestEntraExtraRoles_tenantCaseInsensitive(t *testing.T) {
	t.Parallel()
	mc := jwt.MapClaims{"tid": "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA"}
	_, err := entraExtraRoles(&config.EntraOIDCProviderConfig{
		AllowedTenantIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
	}, mc)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEntraExtraRoles_groupsGate(t *testing.T) {
	t.Parallel()
	mc := jwt.MapClaims{"tid": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "groups": []any{"Other"}}
	_, err := entraExtraRoles(&config.EntraOIDCProviderConfig{
		AllowedTenantIDs:     []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
		AllowedIDTokenGroups: []string{"API-Admins"},
	}, mc)
	if !errors.Is(err, apierrors.ErrEntraLoginDenied) {
		t.Fatalf("got %v", err)
	}
}

func TestEntraExtraRoles_groupBindings(t *testing.T) {
	t.Parallel()
	mc := jwt.MapClaims{
		"tid":    "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"groups": []any{"everyone-uuid", "admins-uuid"},
	}
	roles, err := entraExtraRoles(&config.EntraOIDCProviderConfig{
		GroupRoleBindings: []config.EntraGroupRoleBinding{{
			Group: "admins-uuid",
			Roles: []string{"api:role:admin"},
		}},
	}, mc)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0] != "api:role:admin" {
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

func TestGitlabExtraRoles_groupDenied(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/groups" {
			_ = json.NewEncoder(w).Encode([]map[string]string{{"full_path": "other/top"}})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	_, err := gitlabExtraRoles(t.Context(), srv.Client(), &config.GitLabOIDCProviderConfig{
		APIV4Base:         srv.URL + "/api/v4",
		AllowedGroupPaths: []string{"acme"},
	}, "https://gitlab.com", "oauth-token")
	if !errors.Is(err, apierrors.ErrGitLabLoginDenied) {
		t.Fatalf("got %v", err)
	}
}

func TestGitlabExtraRoles_groupBindings(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/groups" {
			_ = json.NewEncoder(w).Encode([]map[string]string{{"full_path": "acme/platform"}})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	roles, err := gitlabExtraRoles(t.Context(), srv.Client(), &config.GitLabOIDCProviderConfig{
		APIV4Base: srv.URL + "/api/v4",
		GroupRoleBindings: []config.GitLabGroupRoleBinding{{
			GroupFullPath: "acme/platform",
			Roles:         []string{"api:role:admin"},
		}},
	}, "https://gitlab.com", "oauth-token")
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0] != "api:role:admin" {
		t.Fatalf("%v", roles)
	}
}
