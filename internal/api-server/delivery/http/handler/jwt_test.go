package handler

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"

	"github.com/gofiber/fiber/v3"
)

func TestJWTHandler_GenerateToken_ValidationAppID(t *testing.T) {
	uc, err := auth.NewJWTUseCase(t.TempDir(), "iss")
	if err != nil {
		t.Fatal(err)
	}
	h := NewJWTHandler(uc, false)
	app := fiber.New()
	app.Post("/tokens", h.GenerateToken)

	req := httptest.NewRequest(fiber.MethodPost, "/tokens", strings.NewReader(`{"app_id":"","environments":["e"],"expires_at":"2099-01-01T00:00:00Z"}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestJWTHandler_GenerateToken_Created(t *testing.T) {
	uc, err := auth.NewJWTUseCase(t.TempDir(), "iss")
	if err != nil {
		t.Fatal(err)
	}
	h := NewJWTHandler(uc, false)
	app := fiber.New()
	app.Post("/tokens", h.GenerateToken)

	body := `{"app_id":"a1","environments":["dev"],"expires_at":"` + time.Now().Add(time.Hour).Format(time.RFC3339Nano) + `"}`
	req := httptest.NewRequest(fiber.MethodPost, "/tokens", strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
}
