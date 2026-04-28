package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/adapter/etcd"
	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/authz"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
)

func TestJWTHandler_IssueApiAccessToken_forbiddenViaAPIKey(t *testing.T) {
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
	if resp.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
}

func TestJWTHandler_IssueApiAccessToken_forbiddenWhenBearerHasNoRoles(t *testing.T) {
	t.Parallel()
	uc := jwtHandlerTestUC(t)
	h := NewJWTHandler(uc, false, 5*time.Minute, nil)

	caller := mintAPITokenFromSnapshot(t, uc, "user@example.com", map[string]any{
		"omit_roles":  true,
		"permissions": []string{permissions.APIAccessTokenIssue},
	}, 2*time.Minute)

	app := fiber.New()
	app.Use(middleware.APISecurity(uc, nil))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(`{}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+caller)
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

func TestJWTHandler_IssueApiAccessToken_viaBearerHuman(t *testing.T) {
	t.Parallel()
	uc := jwtHandlerTestUC(t)
	h := NewJWTHandler(uc, false, 2*time.Minute, nil)

	caller := mintAPITokenFromSnapshot(t, uc, "user@example.com", map[string]any{
		"roles": []string{roles.APIAccessTokensIssue},
	}, 10*time.Minute)

	app := fiber.New()
	app.Use(middleware.APISecurity(uc, nil))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(`{}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+caller)
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
	if _, ok := mc["roles"]; ok {
		t.Fatalf("issued api token must omit roles, got %v", mc["roles"])
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

	caller := mintAPITokenFromSnapshot(t, uc, "user@example.com", map[string]any{
		"roles": []string{roles.APIAccessTokensIssue},
	}, 10*time.Minute)

	app := fiber.New()
	app.Use(middleware.APISecurity(uc, nil))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(`{"permissions":["api.token.edge.issue"]}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+caller)
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

func TestJWTHandler_IssueApiAccessToken_embedsRequestedPermissionsInTokenClaims(t *testing.T) {
	t.Parallel()
	uc := jwtHandlerTestUC(t)
	cat, err := roles.NewCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	eval := authz.NewPermissionEvaluator(cat)
	h := NewJWTHandler(uc, false, 2*time.Minute, eval)

	caller := mintAPITokenFromSnapshot(t, uc, "user@example.com", map[string]any{
		"roles": []string{roles.APIRoleAdmin},
	}, 10*time.Minute)

	app := fiber.New()
	app.Use(middleware.APISecurity(uc, nil))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(`{"permissions":["api.token.edge.issue"]}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+caller)
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
}

func TestJWTHandler_IssueApiAccessToken_expiresAtAcceptedWithinLimits(t *testing.T) {
	t.Parallel()
	uc := jwtHandlerTestUC(t)
	h := NewJWTHandler(uc, false, 5*time.Minute, nil)

	caller := mintAPITokenFromSnapshot(t, uc, "user@example.com", map[string]any{
		"roles": []string{roles.APIAccessTokensIssue},
	}, 8*time.Minute)

	requested := time.Now().UTC().Add(2 * time.Minute).Round(time.Second)
	body := `{"expires_at":"` + requested.Format(time.RFC3339) + `"}`

	app := fiber.New()
	app.Use(middleware.APISecurity(uc, nil))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+caller)
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
	if out.ExpiresAt.After(requested.Add(2 * time.Second)) {
		t.Fatalf("expires_at too late: got %s want <= %s", out.ExpiresAt, requested)
	}
}

func TestJWTHandler_IssueApiAccessToken_expiresAtRejectedAbovePolicyTTL(t *testing.T) {
	t.Parallel()
	uc := jwtHandlerTestUC(t)
	h := NewJWTHandler(uc, false, 2*time.Minute, nil)

	caller := mintAPITokenFromSnapshot(t, uc, "user@example.com", map[string]any{
		"roles": []string{roles.APIAccessTokensIssue},
	}, 10*time.Minute)

	requested := time.Now().UTC().Add(5 * time.Minute).Round(time.Second)
	body := `{"expires_at":"` + requested.Format(time.RFC3339) + `"}`

	app := fiber.New()
	app.Use(middleware.APISecurity(uc, nil))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+caller)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
}

func TestJWTHandler_IssueApiAccessToken_expiresAtRejectedAboveCallerExp(t *testing.T) {
	t.Parallel()
	uc := jwtHandlerTestUC(t)
	h := NewJWTHandler(uc, false, 10*time.Minute, nil)

	caller := mintAPITokenFromSnapshot(t, uc, "user@example.com", map[string]any{
		"roles": []string{roles.APIAccessTokensIssue},
	}, 2*time.Minute)

	requested := time.Now().UTC().Add(5 * time.Minute).Round(time.Second)
	body := `{"expires_at":"` + requested.Format(time.RFC3339) + `"}`

	app := fiber.New()
	app.Use(middleware.APISecurity(uc, nil))
	app.Post("/v1/tokens/api", h.IssueApiAccessToken)

	req := httptest.NewRequest(http.MethodPost, "/v1/tokens/api", strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+caller)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
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

func mintAPITokenFromSnapshot(t *testing.T, uc *auth.JWTUseCase, subject string, snap map[string]any, ttl time.Duration) string {
	t.Helper()
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	tok, _, _, err := uc.MintInteractiveAPIAccessJWTFromSnapshot(t.Context(), subject, raw, ttl)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}
