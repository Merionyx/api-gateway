package bundle

import (
	"context"

	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// ContractExportUseCase proxies contract file export to Contract Syncer (no etcd).
type ContractExportUseCase struct {
	remote interfaces.ContractExportRemote
}

func NewContractExportUseCase(remote interfaces.ContractExportRemote) *ContractExportUseCase {
	return &ContractExportUseCase{remote: remote}
}

func (u *ContractExportUseCase) Export(ctx context.Context, repository, ref, path, contractName string) ([]sharedgit.ExportedContractFile, error) {
	ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseBundlePkg, "Export"))
	defer span.End()
	files, err := u.remote.ExportContractFiles(ctx, repository, ref, path, contractName)
	if err != nil {
		telemetry.MarkError(span, err)
	}
	return files, err
}
