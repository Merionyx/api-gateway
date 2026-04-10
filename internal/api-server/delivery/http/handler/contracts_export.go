package handler

import (
	"encoding/base64"
	"errors"

	"github.com/merionyx/api-gateway/internal/api-server/usecase"

	"github.com/gofiber/fiber/v3"
)

type ContractsExportHandler struct {
	exportUC *usecase.ContractExportUseCase
}

func NewContractsExportHandler(exportUC *usecase.ContractExportUseCase) *ContractsExportHandler {
	return &ContractsExportHandler{exportUC: exportUC}
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

// Export POST /api/v1/contracts/export — forwards to Contract Syncer (no etcd).
func (h *ContractsExportHandler) Export(c fiber.Ctx) error {
	var req contractsExportRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Repository == "" || req.Ref == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "repository and ref are required"})
	}

	files, err := h.exportUC.Export(c.Context(), req.Repository, req.Ref, req.Path, req.ContractName)
	if err != nil {
		if errors.Is(err, usecase.ErrContractSyncerRejected) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
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
