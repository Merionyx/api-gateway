package handler

import (
	"encoding/base64"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/authz"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
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
	var req apiserver.ExportContractsJSONBody
	if err := c.Bind().Body(&req); err != nil {
		telemetry.MarkError(span, err)
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeInvalidJSONBody, "", problem.DetailInvalidJSONBody))
	}
	if req.Data.Repository == "" || req.Data.Ref == "" {
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeExportRepositoryRefRequired, "", problem.DetailExportRepositoryRefRequired))
	}

	path := ""
	if req.Data.Path != nil {
		path = *req.Data.Path
	}
	contractName := ""
	if req.Data.ContractName != nil {
		contractName = *req.Data.ContractName
	}
	files, err := h.exportUC.Export(c.Context(), req.Data.Repository, req.Data.Ref, path, contractName)
	if err != nil {
		telemetry.MarkError(span, err)
		return problem.WriteContractSync(c, err)
	}

	resp := apiserver.ExportContracts200JSONResponse{Data: apiserver.ContractsExportResponse{
		Files: make([]apiserver.ContractsExportFile, 0, len(files)),
	}}
	for i := range files {
		f := files[i]
		resp.Data.Files = append(resp.Data.Files, apiserver.ContractsExportFile{
			ContractName:  f.ContractName,
			SourcePath:    f.SourcePath,
			ContentBase64: base64.StdEncoding.EncodeToString(f.Content),
		})
	}
	return c.JSON(resp)
}
