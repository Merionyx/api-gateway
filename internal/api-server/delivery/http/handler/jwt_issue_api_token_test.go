package handler

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

	"github.com/merionyx/api-gateway/internal/api-server/adapter/etcd"
	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/authz"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"

	"github.com/gofiber/fiber/v3"
)

func TestJWTHandler_IssueApiAccessToken_viaAPIKey(t *testing.T) {
	t.Parallel()
	uc := jwtHandlerTestUC(t)
	h := NewJWTHandler(uc, false, 5*time.Minute, nil)
	secret := "issue-api-test-secret"
	d := etcd.SHA256DigestHexFromSecret(secret)
	repo := &stubAPIKeyRepoForIssue{
		wantDigest: d,
		rec: kvvalue.APIKeyValue{
			SchemaVersion: kvvalue.APIKeySchemaV2,
			Algorithm:     "sha256",
			Roles:         []string{roles.APIAccessTokensIssue},
			RecordFormat:  kvvalue.DefaultAPIKeyRecordFormat,
		},
	}
	app := fiber.New()
	app.Use(middleware.APISecurity(uc, repo))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(`{}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-API-Key", secret)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
	var out apiserver.ApiAccessTokenIssued
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.AccessToken == "" || out.ExpiresAt.IsZero() {
		t.Fatalf("response %+v", out)
	}
	mc, err := uc.ParseAndValidateAPIProfileBearerToken(out.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if mc["sub"] != "m2m:"+d {
		t.Fatalf("sub %v", mc["sub"])
	}
}

func TestJWTHandler_IssueApiAccessToken_forbiddenWrongAPIKeyRole(t *testing.T) {
	t.Parallel()
	uc := jwtHandlerTestUC(t)
	h := NewJWTHandler(uc, false, 5*time.Minute, nil)
	secret := "issue-api-wrong-role"
	d := etcd.SHA256DigestHexFromSecret(secret)
	repo := &stubAPIKeyRepoForIssue{
		wantDigest: d,
		rec: kvvalue.APIKeyValue{
			SchemaVersion: kvvalue.APIKeySchemaV2,
			Algorithm:     "sha256",
			Roles:         []string{roles.APIRoleViewer},
			RecordFormat:  kvvalue.DefaultAPIKeyRecordFormat,
		},
	}
	app := fiber.New()
	app.Use(middleware.APISecurity(uc, repo))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(`{}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-API-Key", secret)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
}

func TestJWTHandler_IssueApiAccessToken_forbiddenBearerBaselineMember(t *testing.T) {
	t.Parallel()
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
	tok, _, _, err := uc.MintInteractiveAPIAccessJWT(t.Context(), "user@example.com", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	h := NewJWTHandler(uc, false, 2*time.Minute, nil)
	app := fiber.New()
	app.Use(middleware.APISecurity(uc, nil))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(`{}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+tok)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
}

func TestJWTHandler_IssueApiAccessToken_viaAPIKeyAdminBypass(t *testing.T) {
	t.Parallel()
	uc := jwtHandlerTestUC(t)
	h := NewJWTHandler(uc, false, 5*time.Minute, nil)
	secret := "issue-api-admin-secret"
	d := etcd.SHA256DigestHexFromSecret(secret)
	repo := &stubAPIKeyRepoForIssue{
		wantDigest: d,
		rec: kvvalue.APIKeyValue{
			SchemaVersion: kvvalue.APIKeySchemaV2,
			Algorithm:     "sha256",
			Roles:         []string{roles.APIRoleAdmin},
			RecordFormat:  kvvalue.DefaultAPIKeyRecordFormat,
		},
	}
	app := fiber.New()
	app.Use(middleware.APISecurity(uc, repo))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(`{}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-API-Key", secret)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
}

type stubAPIKeyRepoForIssue struct {
	wantDigest string
	rec        kvvalue.APIKeyValue
}

func (s *stubAPIKeyRepoForIssue) Get(_ context.Context, digestHex string) (kvvalue.APIKeyValue, int64, error) {
	if digestHex != s.wantDigest {
		return kvvalue.APIKeyValue{}, 0, apierrors.ErrNotFound
	}
	return s.rec, 1, nil
}

func TestJWTHandler_IssueApiAccessToken_viaBearer(t *testing.T) {
	t.Parallel()
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
	snap, err := json.Marshal(map[string]any{"roles": []string{roles.APIAccessTokensIssue}})
	if err != nil {
		t.Fatal(err)
	}
	tok, _, _, err := uc.MintInteractiveAPIAccessJWTFromSnapshot(t.Context(), "user@example.com", snap, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	h := NewJWTHandler(uc, false, 2*time.Minute, nil)
	app := fiber.New()
	app.Use(middleware.APISecurity(uc, nil))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+tok)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
}

func TestJWTHandler_IssueApiAccessToken_embedsRequestedPermissionsInTokenClaims(t *testing.T) {
	t.Parallel()
	uc := jwtHandlerTestUC(t)
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	eval := authz.NewPermissionEvaluator(cat)
	h := NewJWTHandler(uc, false, 2*time.Minute, eval)
	secret := "issue-api-admin-grant"
	d := etcd.SHA256DigestHexFromSecret(secret)
	repo := &stubAPIKeyRepoForIssue{
		wantDigest: d,
		rec: kvvalue.APIKeyValue{
			SchemaVersion: kvvalue.APIKeySchemaV2,
			Algorithm:     "sha256",
			Roles:         []string{roles.APIRoleAdmin},
			RecordFormat:  kvvalue.DefaultAPIKeyRecordFormat,
		},
	}
	app := fiber.New()
	app.Use(middleware.APISecurity(uc, repo))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(`{"permissions":["api.token.edge.issue"]}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-API-Key", secret)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
	var out apiserver.ApiAccessTokenIssued
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	mc, err := uc.ParseAndValidateAPIProfileBearerToken(out.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	perms, _ := mc["permissions"].([]any)
	if len(perms) != 1 || perms[0] != permissions.EdgeTokenIssue {
		t.Fatalf("permissions claim %#v", mc["permissions"])
	}
	if _, ok := mc["roles"]; ok {
		t.Fatalf("roles claim must be omitted, got %#v", mc["roles"])
	}
}

func TestJWTHandler_IssueApiAccessToken_rejectsRequestedPermissionsOutsideCallerPermissions(t *testing.T) {
	t.Parallel()
	uc := jwtHandlerTestUC(t)
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	eval := authz.NewPermissionEvaluator(cat)
	h := NewJWTHandler(uc, false, 2*time.Minute, eval)
	secret := "issue-api-non-admin-grant"
	d := etcd.SHA256DigestHexFromSecret(secret)
	repo := &stubAPIKeyRepoForIssue{
		wantDigest: d,
		rec: kvvalue.APIKeyValue{
			SchemaVersion: kvvalue.APIKeySchemaV2,
			Algorithm:     "sha256",
			Roles:         []string{roles.APIAccessTokensIssue},
			RecordFormat:  kvvalue.DefaultAPIKeyRecordFormat,
		},
	}
	app := fiber.New()
	app.Use(middleware.APISecurity(uc, repo))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(`{"permissions":["api.token.edge.issue"]}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-API-Key", secret)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
}
