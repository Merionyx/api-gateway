package usecase

import (
	"fmt"
	"log/slog"

	sharedgit "merionyx/api-gateway/internal/shared/git"
)

type SyncUseCase struct {
	gitManager *sharedgit.RepositoryManager
}

func NewSyncUseCase(gitManager *sharedgit.RepositoryManager) *SyncUseCase {
	return &SyncUseCase{
		gitManager: gitManager,
	}
}

func (u *SyncUseCase) Sync(repository, ref, path string) ([]sharedgit.ContractSnapshot, error) {
	slog.Info("Syncing repository", "repository", repository, "ref", ref, "path", path)

	snapshots, err := u.gitManager.GetRepositorySnapshots(repository, ref, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository snapshots: %w", err)
	}

	slog.Info("Successfully synced repository", "repository", repository, "snapshots_count", len(snapshots))

	return snapshots, nil
}
