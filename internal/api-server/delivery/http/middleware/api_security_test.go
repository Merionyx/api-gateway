package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"

	"github.com/gofiber/fiber/v3"
)

func Test_requiresAPISecurity(t *testing.T) {
	t.Parallel()
	if requiresAPISecurity(http.MethodGet, "/health") {
		t.Fatal("health public")
	}
	if !requiresAPISecurity(http.MethodGet, "/api/v1/status") {
		t.Fatal("status protected")
	}
	if requiresAPISecurity(http.MethodPost, "/api/v1/auth/refresh") {
		t.Fatal("refresh public")
	}
	if !requiresAPISecurity(http.MethodGet, "/api/v1/auth/refresh") {
		t.Fatal("non-post refresh protected")
	}
}

func TestAPISecurity_allowsHealthWithoutAuth(t *testing.T) {
	t.Parallel()
	uc := testJWTUC(t)
	app := fiber.New()
	app.Use(APISecurity(uc, nil))
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
	app.Use(APISecurity(uc, nil))
	app.Get("/api/v1/status", func(c fiber.Ctx) error { return c.SendString("no") })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
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
	app.Use(APISecurity(uc, nil))
	app.Get("/api/v1/status", func(c fiber.Ctx) error { return c.SendString("yes") })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
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
