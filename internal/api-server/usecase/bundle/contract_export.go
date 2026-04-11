package bundle

import (
	"context"

	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

// ContractExportUseCase proxies contract file export to Contract Syncer (no etcd).
type ContractExportUseCase struct {
	remote interfaces.ContractExportRemote
}

func NewContractExportUseCase(remote interfaces.ContractExportRemote) *ContractExportUseCase {
	return &ContractExportUseCase{remote: remote}
}

func (u *ContractExportUseCase) Export(ctx context.Context, repository, ref, path, contractName string) ([]sharedgit.ExportedContractFile, error) {
	return u.remote.ExportContractFiles(ctx, repository, ref, path, contractName)
}
