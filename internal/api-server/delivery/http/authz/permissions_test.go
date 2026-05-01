package authz

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
)

func TestPermissionEvaluatorRequireAnyHTTPPermission_viaRole(t *testing.T) {
	t.Parallel()
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	e := NewPermissionEvaluator(cat)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(middleware.CtxKeyAPIKeyPrincipal, &middleware.APIKeyPrincipal{
			Roles: []string{roles.APIAccessTokensIssue},
		})
		return c.Next()
	})
	app.Get("/", func(c fiber.Ctx) error {
		denied, err := e.RequireAnyHTTPPermission(c, permissions.APIAccessTokenIssue)
		if denied {
			return err
		}
		return c.SendStatus(http.StatusNoContent)
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestClaimStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		claim any
		want  []string
	}{
		{
			name:  "slice strings normalized",
			claim: []string{" contracts:export ", "", "contracts:export", "edge:token:issue"},
			want:  []string{"contracts:export", "edge:token:issue"},
		},
		{
			name:  "slice any normalized",
			claim: []any{" contracts:export ", "edge:token:issue", "contracts:export", 42},
			want:  []string{"contracts:export", "edge:token:issue"},
		},
		{
			name:  "single string normalized",
			claim: " contracts:export ",
			want:  []string{"contracts:export"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ClaimStrings(jwt.MapClaims{"permissions": tt.claim}, "permissions")
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ClaimStrings() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestHasPermission(t *testing.T) {
	t.Parallel()

	have := map[string]struct{}{
		permissions.ContractsExport: {},
	}
	if !HasPermission(have, " "+permissions.ContractsExport+" ") {
		t.Fatalf("HasPermission must match trimmed required permission")
	}

	if HasPermission(have, " ") {
		t.Fatalf("HasPermission must reject empty required permission")
	}

	if HasPermission(have, permissions.EdgeTokenIssue) {
		t.Fatalf("HasPermission must reject missing permission")
	}

	if !HasPermission(map[string]struct{}{permissions.Wildcard: {}}, permissions.EdgeTokenIssue) {
		t.Fatalf("HasPermission must allow wildcard permission")
	}
}

func TestPermissionEvaluatorRequireAnyHTTPPermission_viaTokenPermissionsClaim(t *testing.T) {
	t.Parallel()
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	e := NewPermissionEvaluator(cat)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(middleware.CtxKeyAPIJWTClaims, jwt.MapClaims{
			"roles":       []string{roles.APIRoleViewer},
			"permissions": []string{permissions.ContractsExport},
		})
		return c.Next()
	})
	app.Get("/", func(c fiber.Ctx) error {
		denied, err := e.RequireAnyHTTPPermission(c, permissions.ContractsExport)
		if denied {
			return err
		}
		return c.SendStatus(http.StatusNoContent)
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestPermissionEvaluatorRequireAnyHTTPPermission_IgnoresLegacyScopesClaim(t *testing.T) {
	t.Parallel()
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	e := NewPermissionEvaluator(cat)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(middleware.CtxKeyAPIJWTClaims, jwt.MapClaims{
			"roles":  []string{roles.APIRoleViewer},
			"scopes": []string{permissions.ContractsExport},
		})
		return c.Next()
	})
	app.Get("/", func(c fiber.Ctx) error {
		denied, err := e.RequireAnyHTTPPermission(c, permissions.ContractsExport)
		if denied {
			return err
		}
		return c.SendStatus(http.StatusNoContent)
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestPermissionEvaluatorRequireDelegatedPermissions(t *testing.T) {
	t.Parallel()
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	e := NewPermissionEvaluator(cat)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(middleware.CtxKeyAPIKeyPrincipal, &middleware.APIKeyPrincipal{
			Scopes: []string{permissions.ContractsExport},
		})
		return c.Next()
	})
	app.Get("/", func(c fiber.Ctx) error {
		denied, err := e.RequireDelegatedPermissions(c, []string{permissions.ContractsExport})
		if denied {
			return err
		}
		return c.SendStatus(http.StatusNoContent)
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
