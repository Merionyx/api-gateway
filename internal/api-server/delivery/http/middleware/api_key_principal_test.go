package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/adapter/etcd"
	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

type stubAPIKeyRepo struct {
	wantDigest string
	rec        kvvalue.APIKeyValue
	err        error
}

func (s *stubAPIKeyRepo) Get(_ context.Context, digestHex string) (kvvalue.APIKeyValue, int64, error) {
	if s.err != nil {
		return kvvalue.APIKeyValue{}, 0, s.err
	}
	if digestHex != s.wantDigest {
		return kvvalue.APIKeyValue{}, 0, apierrors.ErrNotFound
	}
	return s.rec, 3, nil
}

type countingAPIKeyRepo struct {
	calls int
}

func (c *countingAPIKeyRepo) Get(context.Context, string) (kvvalue.APIKeyValue, int64, error) {
	c.calls++
	return kvvalue.APIKeyValue{}, 0, apierrors.ErrNotFound
}

func Test_tryAPIKeyPrincipal_rejectsNewlinesInSecret(t *testing.T) {
	t.Parallel()
	var repo countingAPIKeyRepo
	_, err := tryAPIKeyPrincipal(context.Background(), &repo, "ab\ncd")
	if err != nil {
		t.Fatal(err)
	}
	if repo.calls != 0 {
		t.Fatalf("unexpected etcd calls %d", repo.calls)
	}
}

func Test_tryAPIKeyPrincipal_ok(t *testing.T) {
	t.Parallel()
	secret := "unit-test-secret-32bytes!!!!"
	d := etcd.SHA256DigestHexFromSecret(secret)
	repo := &stubAPIKeyRepo{
		wantDigest: d,
		rec: kvvalue.APIKeyValue{
			SchemaVersion: kvvalue.APIKeySchemaV2,
			Algorithm:     "sha256",
			Roles:         []string{"ops"},
			Scopes:        []string{"registry:read"},
			RecordFormat:  kvvalue.DefaultAPIKeyRecordFormat,
		},
	}
	p, err := tryAPIKeyPrincipal(context.Background(), repo, "  "+secret+"  ")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.DigestHex != d || len(p.Roles) != 1 || p.Roles[0] != "ops" {
		t.Fatalf("principal %+v", p)
	}
}

func TestAPISecurity_apiKeySetsLocals(t *testing.T) {
	t.Parallel()
	uc := testJWTUC(t)
	secret := "k2"
	d := etcd.SHA256DigestHexFromSecret(secret)
	repo := &stubAPIKeyRepo{
		wantDigest: d,
		rec: kvvalue.APIKeyValue{
			SchemaVersion: kvvalue.APIKeySchemaV2,
			Algorithm:     "sha256",
			Roles:         []string{"r1"},
			RecordFormat:  kvvalue.DefaultAPIKeyRecordFormat,
		},
	}
	app := fiber.New()
	app.Use(APISecurity(uc, repo, testAPISecurityContract(t)))
	app.Get("/v1/status", func(c fiber.Ctx) error {
		p, ok := APIKeyPrincipalFromCtx(c)
		if !ok || p == nil || p.Roles[0] != "r1" {
			return c.SendStatus(fiber.StatusTeapot)
		}
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	req.Header.Set("X-API-Key", secret)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
