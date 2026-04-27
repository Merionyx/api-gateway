package handler

import (
	"encoding/base64"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/authz"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/bundle"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

type ContractsExportHandler struct {
	exportUC       *bundle.ContractExportUseCase
	permissionEval *authz.PermissionEvaluator
}

func NewContractsExportHandler(exportUC *bundle.ContractExportUseCase, permissionEval *authz.PermissionEvaluator) *ContractsExportHandler {
	return &ContractsExportHandler{
		exportUC:       exportUC,
		permissionEval: permissionEval,
	}
}

type contractsExportRequest struct {
	Repository   string `json:"repository"`
	Ref          string `json:"ref"`
	Path         string `json:"path"`
	ContractName string `json:"contract_name"`
}

type contractsExportFileJSON struct {
	ContractName  string `json:"contract_name"`
	SourcePath    string `json:"source_path"`
	ContentBase64 string `json:"content_base64"`
}

type contractsExportResponse struct {
	Files []contractsExportFileJSON `json:"files"`
}

// Export POST /v1/contracts/export — forwards to Contract Syncer (no etcd).
func (h *ContractsExportHandler) Export(c fiber.Ctx) error {
	span := beginHandlerSpan(c, "Export")
	defer span.End()
	if h.permissionEval != nil {
		if denied, werr := h.permissionEval.RequireAnyHTTPPermission(c, permissions.ContractsExport); denied {
			return werr
		}
	} else if denied, werr := authz.RequireAnyHTTPRole(c, roles.APIContractsExport); denied {
		return werr
	}
	var req contractsExportRequest
	if err := c.Bind().Body(&req); err != nil {
		telemetry.MarkError(span, err)
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeInvalidJSONBody, "", problem.DetailInvalidJSONBody))
	}
	if req.Repository == "" || req.Ref == "" {
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeExportRepositoryRefRequired, "", problem.DetailExportRepositoryRefRequired))
	}

	files, err := h.exportUC.Export(c.Context(), req.Repository, req.Ref, req.Path, req.ContractName)
	if err != nil {
		telemetry.MarkError(span, err)
		return problem.WriteContractSync(c, err)
	}

	resp := contractsExportResponse{Files: make([]contractsExportFileJSON, 0, len(files))}
	for i := range files {
		f := files[i]
		resp.Files = append(resp.Files, contractsExportFileJSON{
			ContractName:  f.ContractName,
			SourcePath:    f.SourcePath,
			ContentBase64: base64.StdEncoding.EncodeToString(f.Content),
		})
	}
	return c.JSON(resp)
}
