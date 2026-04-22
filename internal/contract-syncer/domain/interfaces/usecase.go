package interfaces

import (
	"context"

	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

type SyncUseCase interface {
	Sync(ctx context.Context, repository, ref, path string) ([]sharedgit.ContractSnapshot, error)
	ExportContracts(ctx context.Context, repository, ref, path, contractName string) ([]sharedgit.ExportedContractFile, error)
}
