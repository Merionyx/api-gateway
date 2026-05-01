package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestFiberCtxFromStrictContext_missing(t *testing.T) {
	t.Parallel()

	if _, err := fiberCtxFromStrictContext(context.Background()); err == nil {
		t.Fatal("expected error for context without fiber ctx")
	}
}

func TestBindFiberContextForStrictHandlers_setsContextValue(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(BindFiberContextForStrictHandlers())
	app.Get("/", func(c fiber.Ctx) error {
		fc, err := fiberCtxFromStrictContext(c.Context())
		if err != nil {
			t.Fatalf("fiber ctx from strict context: %v", err)
		}
		if fc == nil {
			t.Fatal("fiber ctx is nil")
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
