package authz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

type tokenGrantRepoStub struct {
	rec kvvalue.TokenGrantValue
	err error
}

func (s *tokenGrantRepoStub) Get(context.Context, string) (kvvalue.TokenGrantValue, int64, error) {
	if s.err != nil {
		return kvvalue.TokenGrantValue{}, 0, s.err
	}
	return s.rec, 1, nil
}

func TestPermissionEvaluatorRequireAnyHTTPPermission_viaRole(t *testing.T) {
	t.Parallel()
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	e := NewPermissionEvaluator(cat, nil)
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

func TestPermissionEvaluatorRequireAnyHTTPPermission_viaTokenGrant(t *testing.T) {
	t.Parallel()
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	repo := &tokenGrantRepoStub{
		rec: kvvalue.TokenGrantValue{
			Permissions: []string{permissions.ContractsExport},
			ExpiresAt:   time.Now().Add(time.Minute),
		},
	}
	e := NewPermissionEvaluator(cat, repo)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(middleware.CtxKeyAPIJWTClaims, jwt.MapClaims{
			"jti":   "550e8400-e29b-41d4-a716-446655440000",
			"roles": []string{roles.APIRoleViewer},
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

func TestPermissionEvaluatorRequireAnyHTTPPermission_tokenGrantReadError(t *testing.T) {
	t.Parallel()
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	e := NewPermissionEvaluator(cat, &tokenGrantRepoStub{err: errors.New("boom")})
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(middleware.CtxKeyAPIJWTClaims, jwt.MapClaims{
			"jti": "550e8400-e29b-41d4-a716-446655440000",
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
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestPermissionEvaluatorRequireDelegatedPermissions(t *testing.T) {
	t.Parallel()
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	e := NewPermissionEvaluator(cat, &tokenGrantRepoStub{err: apierrors.ErrNotFound})
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
