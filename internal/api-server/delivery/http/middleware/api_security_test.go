package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"

	"github.com/gofiber/fiber/v3"
)

func TestAPISecurityContract_MatchesOpenAPIPublicOperations(t *testing.T) {
	t.Parallel()

	swagger, err := apiserver.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}

	contract, err := NewAPISecurityContract(swagger)
	if err != nil {
		t.Fatal(err)
	}

	for path, pathItem := range swagger.Paths.Map() {
		if pathItem == nil {
			continue
		}
		for method, op := range pathItem.Operations() {
			if op == nil {
				continue
			}
			wantPublic := op.Security != nil && len(*op.Security) == 0
			if gotPublic := contract.IsPublic(method, path); gotPublic != wantPublic {
				t.Fatalf("%s %s public=%v want=%v", strings.ToUpper(method), path, gotPublic, wantPublic)
			}
		}
	}
}

func TestAPISecurityContract_isMethodAware(t *testing.T) {
	t.Parallel()

	swagger, err := apiserver.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	contract, err := NewAPISecurityContract(swagger)
	if err != nil {
		t.Fatal(err)
	}

	if contract.RequiresAPISecurity(http.MethodPost, "/v1/auth/token") {
		t.Fatal("POST /v1/auth/token should be public by OpenAPI contract")
	}
	if !contract.RequiresAPISecurity(http.MethodGet, "/v1/auth/token") {
		t.Fatal("GET /v1/auth/token should stay protected because no public GET operation is declared")
	}
}

func TestAPISecurity_allowsHealthWithoutAuth(t *testing.T) {
	t.Parallel()
	uc := testJWTUC(t)
	app := fiber.New()
	app.Use(APISecurity(uc, nil, testAPISecurityContract(t)))
	app.Get("/health", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestAPISecurity_blocksProtectedWithoutAuth(t *testing.T) {
	t.Parallel()
	uc := testJWTUC(t)
	app := fiber.New()
	app.Use(APISecurity(uc, nil, testAPISecurityContract(t)))
	app.Get("/v1/status", func(c fiber.Ctx) error { return c.SendString("no") })

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
}

func TestAPISecurity_legacyAuthPathFallsThroughTo404(t *testing.T) {
	t.Parallel()
	uc := testJWTUC(t)
	app := fiber.New()
	app.Use(APISecurity(uc, nil, testAPISecurityContract(t)))
	app.Get("/health", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/callback", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
}

func TestAPISecurity_rejectsEdgeProfileBearer(t *testing.T) {
	t.Parallel()
	uc := testJWTUC(t)
	edgeResp, err := uc.GenerateToken(t.Context(), &models.GenerateTokenRequest{
		AppID:        "edge-app",
		Environments: []string{"dev"},
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uc.ParseAndValidateAPIProfileBearerToken(edgeResp.Token); err == nil {
		t.Fatal("edge token must not validate as API profile")
	}

	app := fiber.New()
	app.Use(APISecurity(uc, nil, testAPISecurityContract(t)))
	app.Get("/v1/status", func(c fiber.Ctx) error { return c.SendString("no") })

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+edgeResp.Token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
}

func TestAPISecurity_allowsBearerAPIJWT(t *testing.T) {
	t.Parallel()
	uc := testJWTUC(t)
	tok, _, _, err := uc.MintInteractiveAPIAccessJWT(t.Context(), "a@b.c", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(APISecurity(uc, nil, testAPISecurityContract(t)))
	app.Get("/v1/status", func(c fiber.Ctx) error { return c.SendString("yes") })

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+tok)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
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

func Test_parseBearer(t *testing.T) {
	t.Parallel()
	if g := parseBearer("Bearer abc"); g != "abc" {
		t.Fatalf("got %q", g)
	}
	if g := parseBearer("bearer  x "); g != "x" {
		t.Fatalf("got %q", g)
	}
	if parseBearer("Basic x") != "" {
		t.Fatal("want empty")
	}
}

func testAPISecurityContract(t *testing.T) *APISecurityContract {
	t.Helper()
	swagger, err := apiserver.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	contract, err := NewAPISecurityContract(swagger)
	if err != nil {
		t.Fatal(err)
	}
	return contract
}
