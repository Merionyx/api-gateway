package interfaces

import (
	sharedgit "merionyx/api-gateway/internal/shared/git"
)

type SyncUseCase interface {
	Sync(repository, ref, path string) ([]sharedgit.ContractSnapshot, error)
}
