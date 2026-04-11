package metrics

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestMetricsEnabledFromCtx(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error {
		if MetricsEnabledFromCtx(c) {
			t.Fatal("expected false before Locals set")
		}
		c.Locals(LocalsKeyMetricsHTTP, true)
		if !MetricsEnabledFromCtx(c) {
			t.Fatal("expected true")
		}
		c.Locals(LocalsKeyMetricsHTTP, "not-a-bool")
		if MetricsEnabledFromCtx(c) {
			t.Fatal("expected false for wrong type")
		}
		return c.SendStatus(fiber.StatusOK)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/x", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
}

func TestHTTPMiddleware(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(HTTPMiddleware(false))
	app.Get("/ping", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/ping", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("status %d", resp.StatusCode)
	}

	app2 := fiber.New()
	app2.Use(HTTPMiddleware(true))
	app2.Get("/ok", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	req2 := httptest.NewRequest(fiber.MethodGet, "/ok", nil)
	resp2, err := app2.Test(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp2.Body)
	if resp2.StatusCode != fiber.StatusOK {
		t.Fatalf("status %d", resp2.StatusCode)
	}

	app3 := fiber.New()
	app3.Use(HTTPMiddleware(true))
	app3.Get("/bad", func(c fiber.Ctx) error {
		return errorsSentinel()
	})
	req3 := httptest.NewRequest(fiber.MethodGet, "/bad", nil)
	resp3, err := app3.Test(req3)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp3.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp3.Body)
}

func errorsSentinel() error {
	return fiber.ErrBadRequest
}
