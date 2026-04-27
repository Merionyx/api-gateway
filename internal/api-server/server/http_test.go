package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/gofiber/fiber/v3"
	oapimw "github.com/oapi-codegen/fiber-middleware/v2"

	"github.com/merionyx/api-gateway/internal/api-server/config"
	httpxmw "github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
)

func TestOpenAPISecuritySatisfied(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		c.Locals(httpxmw.CtxKeyAPIJWTClaims, map[string]any{"sub": "user@example.com"})
		c.Locals(httpxmw.CtxKeyAPIKeyPrincipal, &httpxmw.APIKeyPrincipal{DigestHex: "abc"})

		if err := openAPISecuritySatisfied(c, &openapi3filter.AuthenticationInput{SecuritySchemeName: "bearerAuth"}); err != nil {
			t.Fatalf("bearer auth rejected: %v", err)
		}
		if err := openAPISecuritySatisfied(c, &openapi3filter.AuthenticationInput{SecuritySchemeName: "apiKey"}); err != nil {
			t.Fatalf("api key auth rejected: %v", err)
		}
		if err := openAPISecuritySatisfied(c, &openapi3filter.AuthenticationInput{SecuritySchemeName: "unknown"}); err == nil {
			t.Fatal("unknown scheme should fail")
		}

		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestProtectedRouteBearerPassesOpenAPIValidation(t *testing.T) {
	t.Parallel()

	swagger, err := apiserver.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	swagger.Servers = nil

	uc := testJWTUC(t)
	tok, _, _, err := uc.MintInteractiveAPIAccessJWT(t.Context(), "a@b.c", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Use(httpxmw.APISecurity(uc, nil))
	app.Use(oapimw.OapiRequestValidatorWithOptions(swagger, &oapimw.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: AuthenticationFunc,
		},
	}))
	app.Get("/v1/controllers", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/controllers?limit=50", nil)
	req.Header.Set(fiber.HeaderAccept, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+tok)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func testJWTUC(t *testing.T) *auth.JWTUseCase {
	t.Helper()
	dir := t.TempDir()
	uc, err := auth.NewJWTUseCase(&config.JWTConfig{
		KeysDir:      dir,
		EdgeKeysDir:  filepath.Join(dir, "edge"),
		Issuer:       "iss",
		APIAudience:  "api-aud",
		EdgeIssuer:   "edge-iss",
		EdgeAudience: "edge-aud",
	})
	if err != nil {
		t.Fatal(err)
	}
	return uc
}
