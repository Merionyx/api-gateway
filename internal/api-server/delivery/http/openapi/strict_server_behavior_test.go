package openapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/container"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
)

func testJWTUseCase(t *testing.T) *auth.JWTUseCase {
	t.Helper()
	root := t.TempDir()
	uc, err := auth.NewJWTUseCase(&config.JWTConfig{
		KeysDir:      root,
		EdgeKeysDir:  filepath.Join(root, "edge"),
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

func testStrictApp(cnt *container.Container, injectLocals fiber.Handler) *fiber.App {
	app := fiber.New()
	app.Use(BindFiberContextForStrictHandlers())
	if injectLocals != nil {
		app.Use(injectLocals)
	}
	apiserver.RegisterHandlers(app, apiserver.NewStrictHandler(NewStrictOpenAPIServer(cnt), nil))
	return app
}

func TestStrictInspectTokenPermissions_UsesDataWrapper(t *testing.T) {
	t.Parallel()

	uc := testJWTUseCase(t)
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	cnt := &container.Container{
		Config:      &config.Config{},
		JWTUseCase:  uc,
		RoleCatalog: cat,
	}
	app := testStrictApp(cnt, nil)

	tok, _, _, err := uc.MintInteractiveAPIAccessJWTFromSnapshot(
		t.Context(),
		"user-1",
		[]byte(`{"roles":["api:role:viewer"],"permissions":["custom.claim.permission"]}`),
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/token-permissions", strings.NewReader(`{"data":{"access_token":"`+tok+`"}}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}

	var out struct {
		Data apiserver.TokenPermissionsResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Data.Subject != "user-1" {
		t.Fatalf("subject %q", out.Data.Subject)
	}
	if len(out.Data.Roles) != 1 || out.Data.Roles[0] != roles.APIRoleViewer {
		t.Fatalf("roles %#v", out.Data.Roles)
	}
	if len(out.Data.Permissions) == 0 {
		t.Fatal("permissions must not be empty")
	}
}

func TestStrictInspectTokenPermissions_RejectsFlatBodyWithoutData(t *testing.T) {
	t.Parallel()

	uc := testJWTUseCase(t)
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	cnt := &container.Container{
		Config:      &config.Config{},
		JWTUseCase:  uc,
		RoleCatalog: cat,
	}
	app := testStrictApp(cnt, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/token-permissions", strings.NewReader(`{"access_token":"bad"}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}

	var p apiserver.Problem
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.Code == nil || *p.Code != "ACCESS_TOKEN_REQUIRED" {
		t.Fatalf("unexpected problem code %#v", p.Code)
	}
}

func TestStrictIssueApiAccessToken_UsesDefaultTTLAndOmitsRoles(t *testing.T) {
	t.Parallel()

	uc := testJWTUseCase(t)
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	cnt := &container.Container{
		Config:      &config.Config{},
		JWTUseCase:  uc,
		RoleCatalog: cat,
	}

	callerToken, _, _, err := uc.MintInteractiveAPIAccessJWTFromSnapshot(
		t.Context(),
		"user@example.com",
		[]byte(`{"roles":["`+roles.APIAccessTokensIssue+`"]}`),
		10*time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	callerClaims, err := uc.ParseAndValidateAPIProfileBearerToken(callerToken)
	if err != nil {
		t.Fatal(err)
	}

	app := testStrictApp(cnt, func(c fiber.Ctx) error {
		c.Locals(middleware.CtxKeyAPIJWTClaims, callerClaims)
		return c.Next()
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(`{"data":{}}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}

	var out struct {
		Data apiserver.ApiAccessTokenIssued `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	issuedClaims, err := uc.ParseAndValidateAPIProfileBearerToken(out.Data.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := issuedClaims["roles"]; ok {
		t.Fatalf("issued token must omit roles, got %#v", issuedClaims["roles"])
	}
}
