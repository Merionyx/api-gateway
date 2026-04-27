package authz

import (
	"net/http"
	"net/http/httptest"
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
