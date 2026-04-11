package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestContractsExportHandler_invalidJSON(t *testing.T) {
	t.Parallel()
	h := NewContractsExportHandler(bundle.NewContractExportUseCase(&exportRemoteStub{}))
	app := fiber.New()
	app.Post("/", h.Export)
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
	h := NewContractsExportHandler(bundle.NewContractExportUseCase(&exportRemoteStub{}))
	app := fiber.New()
	app.Post("/", h.Export)
	req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader(`{"repository":"","ref":""}`))
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
	h := NewContractsExportHandler(bundle.NewContractExportUseCase(remote))
	app := fiber.New()
	app.Post("/", h.Export)
	body := `{"repository":"r","ref":"main","path":"p","contract_name":"api"}`
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
	var out contractsExportResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Files) != 1 || out.Files[0].ContractName != "api" || out.Files[0].ContentBase64 == "" {
		t.Fatalf("got %#v", out.Files)
	}
}

var _ interfaces.ContractExportRemote = (*exportRemoteStub)(nil)
