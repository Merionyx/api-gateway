package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/bundle"

	"github.com/gofiber/fiber/v3"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

type exportRemoteStub struct {
	out []sharedgit.ExportedContractFile
	err error
}

func (e *exportRemoteStub) ExportContractFiles(context.Context, string, string, string, string) ([]sharedgit.ExportedContractFile, error) {
	return e.out, e.err
}

func contractsExportApp(h *ContractsExportHandler, injectRoles []string) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		if len(injectRoles) > 0 {
			c.Locals(middleware.CtxKeyAPIKeyPrincipal, &middleware.APIKeyPrincipal{Roles: injectRoles})
		}
		return c.Next()
	})
	app.Post("/", h.Export)
	return app
}

func TestContractsExportHandler_invalidJSON(t *testing.T) {
	t.Parallel()
	h := NewContractsExportHandler(bundle.NewContractExportUseCase(&exportRemoteStub{}), nil)
	app := contractsExportApp(h, []string{roles.APIContractsExport})
	req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader(`{`))
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

func TestContractsExportHandler_missingRepoRef(t *testing.T) {
	t.Parallel()
	h := NewContractsExportHandler(bundle.NewContractExportUseCase(&exportRemoteStub{}), nil)
	app := contractsExportApp(h, []string{roles.APIContractsExport})
	req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader(`{"data":{"repository":"","ref":""}}`))
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

func TestContractsExportHandler_success(t *testing.T) {
	t.Parallel()
	remote := &exportRemoteStub{
		out: []sharedgit.ExportedContractFile{
			{ContractName: "api", SourcePath: "x.yaml", Content: []byte("hello")},
		},
	}
	h := NewContractsExportHandler(bundle.NewContractExportUseCase(remote), nil)
	app := contractsExportApp(h, []string{roles.APIContractsExport})
	body := `{"data":{"repository":"r","ref":"main","path":"p","contract_name":"api"}}`
	req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != fiber.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d %s", resp.StatusCode, b)
	}
	var out struct {
		Data struct {
			Files []struct {
				ContractName  string `json:"contract_name"`
				SourcePath    string `json:"source_path"`
				ContentBase64 string `json:"content_base64"`
			} `json:"files"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Data.Files) != 1 || out.Data.Files[0].ContractName != "api" || out.Data.Files[0].ContentBase64 == "" {
		t.Fatalf("got %#v", out.Data.Files)
	}
}

func TestContractsExportHandler_forbiddenWithoutPrincipal(t *testing.T) {
	t.Parallel()
	h := NewContractsExportHandler(bundle.NewContractExportUseCase(&exportRemoteStub{}), nil)
	app := contractsExportApp(h, nil)
	body := `{"data":{"repository":"r","ref":"main"}}`
	req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestContractsExportHandler_forbiddenWrongRole(t *testing.T) {
	t.Parallel()
	h := NewContractsExportHandler(bundle.NewContractExportUseCase(&exportRemoteStub{}), nil)
	app := contractsExportApp(h, []string{roles.APIRoleViewer})
	body := `{"data":{"repository":"r","ref":"main"}}`
	req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

var _ interfaces.ContractExportRemote = (*exportRemoteStub)(nil)
