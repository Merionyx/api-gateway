package openapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/container"
	httpauthz "github.com/merionyx/api-gateway/internal/api-server/delivery/http/authz"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
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
		Config:              &config.Config{},
		JWTUseCase:          uc,
		RoleCatalog:         cat,
		PermissionEvaluator: httpauthz.NewPermissionEvaluator(cat),
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

func TestStrictInspectTokenPermissions_SubjectClaimPriority(t *testing.T) {
	t.Parallel()

	uc := testJWTUseCase(t)
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	cnt := &container.Container{
		Config:              &config.Config{},
		JWTUseCase:          uc,
		RoleCatalog:         cat,
		PermissionEvaluator: httpauthz.NewPermissionEvaluator(cat),
	}
	app := testStrictApp(cnt, nil)

	cases := []struct {
		name            string
		subject         string
		snapshot        string
		expectedSubject string
	}{
		{
			name:            "prefers_email_over_sub",
			subject:         "sub-123",
			snapshot:        `{"email":"fallback@example.com"}`,
			expectedSubject: "fallback@example.com",
		},
		{
			name:            "prefers_preferred_username_over_sub_when_email_missing",
			subject:         "sub-123",
			snapshot:        `{"preferred_username":"pref-user"}`,
			expectedSubject: "pref-user",
		},
		{
			name:            "uses_sub_when_only_sub_is_present",
			subject:         "sub-only",
			snapshot:        `{}`,
			expectedSubject: "sub-only",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tok, _, _, err := uc.MintInteractiveAPIAccessJWTFromSnapshot(t.Context(), tc.subject, []byte(tc.snapshot), time.Minute)
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
			if out.Data.Subject != tc.expectedSubject {
				t.Fatalf("subject %q", out.Data.Subject)
			}
		})
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
		Config:              &config.Config{},
		JWTUseCase:          uc,
		RoleCatalog:         cat,
		PermissionEvaluator: httpauthz.NewPermissionEvaluator(cat),
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

func TestStrictTokenOidc_GrantTypeIsReadOnlyFromFormBody(t *testing.T) {
	t.Parallel()

	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	cnt := &container.Container{
		Config:              &config.Config{},
		RoleCatalog:         cat,
		PermissionEvaluator: httpauthz.NewPermissionEvaluator(cat),
		OAuthTokenUseCase:   auth.NewOAuthTokenUseCase(nil, nil, nil, nil, auth.TokenTTLPolicy{}),
	}
	app := testStrictApp(cnt, nil)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/token?grant_type=authorization_code",
		strings.NewReader("code=code-1&redirect_uri=https%3A%2F%2Fclient.example%2Fcb&client_id=client-1&code_verifier=verifier-1"),
	)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}

	var out apiserver.OAuthTokenError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != "invalid_request" {
		t.Fatalf("error %q", out.Error)
	}
	if out.ErrorDescription == nil || !strings.Contains(*out.ErrorDescription, "unsupported grant_type") {
		t.Fatalf("error_description %#v", out.ErrorDescription)
	}
}

func TestStrictTokenOidc_RejectsBasicAuthorizationClientAuthentication(t *testing.T) {
	t.Parallel()

	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	cnt := &container.Container{
		Config:              &config.Config{},
		RoleCatalog:         cat,
		PermissionEvaluator: httpauthz.NewPermissionEvaluator(cat),
		OAuthTokenUseCase:   auth.NewOAuthTokenUseCase(nil, nil, nil, nil, auth.TokenTTLPolicy{}),
	}
	app := testStrictApp(cnt, nil)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/token",
		strings.NewReader("grant_type=authorization_code&code=code-1&redirect_uri=https%3A%2F%2Fclient.example%2Fcb&client_id=client-1&code_verifier=verifier-1"),
	)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	req.Header.Set(fiber.HeaderAuthorization, "Basic Y2xpZW50LTE6c2VjcmV0")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}

	var out apiserver.OAuthTokenError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != "invalid_request" {
		t.Fatalf("error %q", out.Error)
	}
	if out.ErrorDescription == nil || !strings.Contains(*out.ErrorDescription, "Authorization: Basic is not supported") {
		t.Fatalf("error_description %#v", out.ErrorDescription)
	}
}

func TestStrictTokenOidc_UsesTypedRequestBodyWithoutFiberFormFallback(t *testing.T) {
	t.Parallel()

	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	cnt := &container.Container{
		Config:              &config.Config{},
		RoleCatalog:         cat,
		PermissionEvaluator: httpauthz.NewPermissionEvaluator(cat),
		OAuthTokenUseCase:   auth.NewOAuthTokenUseCase(nil, nil, nil, nil, auth.TokenTTLPolicy{}),
	}
	srv := NewStrictOpenAPIServer(cnt)

	refreshGrant := apiserver.OAuthTokenRequestFormGrantType("refresh_token")
	refreshToken := "refresh-token-1"
	resp, err := srv.TokenOidc(context.Background(), apiserver.TokenOidcRequestObject{
		Body: &apiserver.TokenOidcFormdataRequestBody{
			GrantType:    refreshGrant,
			RefreshToken: &refreshToken,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, ok := resp.(apiserver.TokenOidc400JSONResponse)
	if !ok {
		t.Fatalf("response type %T", resp)
	}
	if out.Error != "invalid_request" {
		t.Fatalf("error %q", out.Error)
	}
	if out.ErrorDescription == nil || !strings.Contains(*out.ErrorDescription, "refresh use case not configured") {
		t.Fatalf("error_description %#v", out.ErrorDescription)
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
		Config:              &config.Config{},
		JWTUseCase:          uc,
		RoleCatalog:         cat,
		PermissionEvaluator: httpauthz.NewPermissionEvaluator(cat),
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

func TestStrictIssueApiAccessToken_SubjectClaimPriority(t *testing.T) {
	t.Parallel()

	uc := testJWTUseCase(t)
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	cnt := &container.Container{
		Config:              &config.Config{},
		JWTUseCase:          uc,
		RoleCatalog:         cat,
		PermissionEvaluator: httpauthz.NewPermissionEvaluator(cat),
	}

	callerClaims := jwt.MapClaims{
		"sub":                "sub-123",
		"email":              "fallback@example.com",
		"preferred_username": "pref-user",
		"roles":              []any{roles.APIAccessTokensIssue},
		"exp":                float64(time.Now().Add(10 * time.Minute).Unix()),
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
	if got, _ := issuedClaims["sub"].(string); got != "fallback@example.com" {
		t.Fatalf("issued sub %q", got)
	}
}

func TestStrictIssueEdgeToken_ReturnsForbiddenForInsufficientPermissions(t *testing.T) {
	t.Parallel()

	uc := testJWTUseCase(t)
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	cnt := &container.Container{
		Config:              &config.Config{},
		JWTUseCase:          uc,
		RoleCatalog:         cat,
		PermissionEvaluator: httpauthz.NewPermissionEvaluator(cat),
	}

	app := testStrictApp(cnt, func(c fiber.Ctx) error {
		c.Locals(middleware.CtxKeyAPIJWTClaims, jwt.MapClaims{
			"sub": "user-1",
			"exp": float64(time.Now().Add(10 * time.Minute).Unix()),
		})
		return c.Next()
	})

	reqBody := `{"data":{"app_id":"app-1","environments":["dev"],"expires_at":"` + time.Now().Add(10*time.Minute).UTC().Format(time.RFC3339) + `"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/edge", strings.NewReader(reqBody))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}

	var p apiserver.Problem
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.Code == nil || *p.Code != problem.CodeInsufficientPermissions {
		t.Fatalf("unexpected problem code %#v", p.Code)
	}
}
