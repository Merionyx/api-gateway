package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

func TestAuthIntrospectionHandler_ListPermissions_includesConfiguredPermissions(t *testing.T) {
	t.Parallel()

	cat, err := roles.NewCatalog([]roles.ConfiguredRole{{
		ID:          "api:role:custom",
		Permissions: []string{"custom.permission"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	h := NewAuthIntrospectionHandler(nil, cat)

	app := fiber.New()
	app.Get("/v1/auth/permissions", h.ListPermissions)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/permissions", nil)
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
		Data []apiserver.PermissionDescriptor `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Data) == 0 {
		t.Fatal("empty permission list")
	}

	foundBuiltIn := false
	foundCustom := false
	for i := range out.Data {
		if out.Data[i].Id == permissions.Wildcard {
			foundBuiltIn = true
		}
		if out.Data[i].Id == "custom.permission" {
			foundCustom = true
			if strings.TrimSpace(out.Data[i].Description) == "" {
				t.Fatal("custom permission description is empty")
			}
		}
	}
	if !foundBuiltIn {
		t.Fatalf("built-in permission %q not found", permissions.Wildcard)
	}
	if !foundCustom {
		t.Fatal("configured permission not found")
	}
}

func TestAuthIntrospectionHandler_ListRoles_returnsDescriptors(t *testing.T) {
	t.Parallel()

	cat, err := roles.NewCatalog([]roles.ConfiguredRole{{
		ID:          "api:role:custom",
		Permissions: []string{"custom.permission"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	h := NewAuthIntrospectionHandler(nil, cat)

	app := fiber.New()
	app.Get("/v1/auth/roles", h.ListRoles)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/roles", nil)
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
		Data []apiserver.RolePermissions `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Data) == 0 {
		t.Fatal("empty role list")
	}

	var custom *apiserver.RolePermissions
	for i := range out.Data {
		if out.Data[i].Role == "api:role:custom" {
			custom = &out.Data[i]
			break
		}
	}
	if custom == nil {
		t.Fatal("custom role not found")
	}
	if len(custom.Permissions) != 1 || custom.Permissions[0].Id != "custom.permission" {
		t.Fatalf("unexpected custom role permissions %#v", custom.Permissions)
	}
	if strings.TrimSpace(custom.Permissions[0].Description) == "" {
		t.Fatal("custom role permission description is empty")
	}
}

func TestAuthIntrospectionHandler_InspectTokenPermissions_success(t *testing.T) {
	t.Parallel()

	uc := jwtHandlerTestUC(t)
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	h := NewAuthIntrospectionHandler(uc, cat)

	tok, _, _, err := uc.MintInteractiveAPIAccessJWTFromSnapshot(
		t.Context(),
		"user-1",
		[]byte(`{"roles":["api:role:viewer"],"permissions":["api.contracts.export","custom.claim.permission"]}`),
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Post("/v1/auth/token-permissions", h.InspectTokenPermissions)

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

	ids := make([]string, 0, len(out.Data.Permissions))
	for i := range out.Data.Permissions {
		ids = append(ids, out.Data.Permissions[i].Id)
		if strings.TrimSpace(out.Data.Permissions[i].Description) == "" {
			t.Fatalf("permission %q has empty description", out.Data.Permissions[i].Id)
		}
	}
	if !slices.Contains(ids, permissions.BundleRead) {
		t.Fatalf("missing viewer permission %q: %v", permissions.BundleRead, ids)
	}
	if !slices.Contains(ids, permissions.ContractsExport) {
		t.Fatalf("missing direct claim permission %q: %v", permissions.ContractsExport, ids)
	}
	if !slices.Contains(ids, "custom.claim.permission") {
		t.Fatalf("missing custom claim permission: %v", ids)
	}
}

func TestAuthIntrospectionHandler_InspectTokenPermissions_validation(t *testing.T) {
	t.Parallel()

	uc := jwtHandlerTestUC(t)
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	h := NewAuthIntrospectionHandler(uc, cat)

	app := fiber.New()
	app.Post("/v1/auth/token-permissions", h.InspectTokenPermissions)

	t.Run("missing_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/token-permissions", strings.NewReader(`{"data":{"access_token":""}}`))
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
	})

	t.Run("invalid_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/token-permissions", strings.NewReader(`{"data":{"access_token":"bad-token"}}`))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusUnauthorized {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("status %d body %s", resp.StatusCode, b)
		}
	})
}
