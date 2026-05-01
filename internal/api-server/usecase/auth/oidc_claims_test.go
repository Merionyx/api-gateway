package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

func TestClaimsSnapshotFromProvider_DefaultViewer(t *testing.T) {
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
		t.Fatalf("roles=%v", roles)
	}
	if m["email"] != "a@b.c" {
		t.Fatalf("email=%v", m["email"])
	}
}

func TestClaimsSnapshotFromProvider_CEL_Deny(t *testing.T) {
	t.Parallel()
	p := config.OIDCProviderConfig{
		ClaimMapping: &config.OIDCClaimMappingConfig{Rules: []config.OIDCClaimMappingRule{{
			Name:        "corp-only",
			When:        "!id_token.email.endsWith('@example.com')",
			Deny:        true,
			DenyCode:    "EMAIL_DOMAIN_DENIED",
			DenyMessage: "Only @example.com users are allowed.",
		}}},
	}
	_, err := claimsSnapshotFromProvider(context.Background(), p, jwt.MapClaims{"email": "u@other.com"}, "", http.DefaultClient)
	if !errors.Is(err, apierrors.ErrOIDCClaimMappingDenied) {
		t.Fatalf("expected deny, got %v", err)
	}
	var deny *oidcClaimMappingDenyError
	if !errors.As(err, &deny) {
		t.Fatalf("expected deny error type, got %T", err)
	}
	if deny.Code != "EMAIL_DOMAIN_DENIED" {
		t.Fatalf("deny code=%q", deny.Code)
	}
}

func TestClaimsSnapshotFromProvider_CEL_AddRolesPermissionsClaims(t *testing.T) {
	t.Parallel()
	p := config.OIDCProviderConfig{
		ClaimMapping: &config.OIDCClaimMappingConfig{Rules: []config.OIDCClaimMappingRule{
			{
				Name:           "groups-to-roles",
				When:           "has(id_token.groups)",
				AddRolesExpr:   "id_token.groups",
				AddPermissions: []string{"api.contracts.export"},
				AddGroupsExpr:  "id_token.groups",
				SetClaims:      map[string]string{"tenant": "id_token.tid"},
			},
		}},
	}
	mc := jwt.MapClaims{
		"sub":    "u1",
		"tid":    "tenant-a",
		"groups": []any{"api:role:admin", "platform"},
	}
	raw, err := claimsSnapshotFromProvider(context.Background(), p, mc, "", http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	roles := asStringSlice(t, out["roles"])
	if len(roles) != 3 {
		t.Fatalf("roles=%v", roles)
	}
	perms := asStringSlice(t, out["permissions"])
	if len(perms) != 1 || perms[0] != "api.contracts.export" {
		t.Fatalf("permissions=%v", perms)
	}
	groups := asStringSlice(t, out["groups"])
	if len(groups) != 2 {
		t.Fatalf("groups=%v", groups)
	}
	if out["tenant"] != "tenant-a" {
		t.Fatalf("tenant=%v", out["tenant"])
	}
}

func TestClaimsSnapshotFromProvider_CEL_GitHubProviderData(t *testing.T) {
	t.Parallel()
	hc := &http.Client{
		Transport: claimMappingRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			status := http.StatusOK
			body := ""
			switch {
			case strings.HasSuffix(r.URL.Path, "/user/orgs"):
				body = `[{"login":"acme"}]`
			case strings.HasSuffix(r.URL.Path, "/user/teams"):
				body = `[{"slug":"platform","organization":{"login":"acme"}}]`
			default:
				status = http.StatusNotFound
				body = `{"message":"not found"}`
			}
			return &http.Response{
				StatusCode: status,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    r,
			}, nil
		}),
	}

	p := config.OIDCProviderConfig{
		Kind: "github",
		GitHub: &config.GitHubOIDCProviderConfig{
			RESTAPIBase: "https://github.local/api/v3",
		},
		ClaimMapping: &config.OIDCClaimMappingConfig{Rules: []config.OIDCClaimMappingRule{
			{
				Name:     "org-check",
				When:     "provider.github.org_logins.exists(o, o == 'acme')",
				AddRoles: []string{"api:role:admin"},
			},
			{
				Name:           "team-check",
				When:           "provider.github.teams.exists(t, t.org_login == 'acme' && t.slug == 'platform')",
				AddPermissions: []string{"api.token.edge.issue"},
			},
		}},
	}
	raw, err := claimsSnapshotFromProvider(context.Background(), p, jwt.MapClaims{"sub": "u1"}, "oauth-token", hc)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	roles := asStringSlice(t, out["roles"])
	if len(roles) != 2 {
		t.Fatalf("roles=%v", roles)
	}
	perms := asStringSlice(t, out["permissions"])
	if len(perms) != 1 || perms[0] != "api.token.edge.issue" {
		t.Fatalf("permissions=%v", perms)
	}
}

type claimMappingRoundTripFunc func(*http.Request) (*http.Response, error)

func (f claimMappingRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestClaimsSnapshotFromProvider_CEL_RuntimeError(t *testing.T) {
	t.Parallel()
	p := config.OIDCProviderConfig{
		ClaimMapping: &config.OIDCClaimMappingConfig{Rules: []config.OIDCClaimMappingRule{{
			When:     "id_token.",
			AddRoles: []string{"api:role:admin"},
		}}},
	}
	_, err := claimsSnapshotFromProvider(context.Background(), p, jwt.MapClaims{"sub": "u1"}, "", http.DefaultClient)
	if !errors.Is(err, apierrors.ErrOIDCClaimMappingRuntime) {
		t.Fatalf("expected runtime mapping error, got %v", err)
	}
}

func asStringSlice(t *testing.T, v any) []string {
	t.Helper()
	raw, ok := v.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", v)
	}
	out := make([]string, 0, len(raw))
	for i := range raw {
		s, ok := raw[i].(string)
		if !ok {
			t.Fatalf("item[%d] type=%T", i, raw[i])
		}
		out = append(out, s)
	}
	return out
}
