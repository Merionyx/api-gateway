package usecase

import (
	"fmt"
	"log/slog"
	"time"

	syncmetrics "merionyx/api-gateway/internal/contract-syncer/metrics"
	sharedgit "merionyx/api-gateway/internal/shared/git"
)

type SyncUseCase struct {
	gitManager     *sharedgit.RepositoryManager
	metricsEnabled bool
}

func NewSyncUseCase(gitManager *sharedgit.RepositoryManager, metricsEnabled bool) *SyncUseCase {
	return &SyncUseCase{
		gitManager:     gitManager,
		metricsEnabled: metricsEnabled,
	}
}

func (u *SyncUseCase) Sync(repository, ref, path string) ([]sharedgit.ContractSnapshot, error) {
	slog.Info("Syncing repository", "repository", repository, "ref", ref, "path", path)

	start := time.Now()
	snapshots, err := u.gitManager.GetRepositorySnapshots(repository, ref, path)
	if err != nil {
		syncmetrics.RecordGitSyncDuration(u.metricsEnabled, syncmetrics.GitResultError, time.Since(start))
		return nil, fmt.Errorf("failed to get repository snapshots: %w", err)
	}

	syncmetrics.RecordGitSyncDuration(u.metricsEnabled, syncmetrics.GitResultOK, time.Since(start))
	syncmetrics.RecordSnapshotsProduced(u.metricsEnabled, len(snapshots))

	slog.Info("Successfully synced repository", "repository", repository, "snapshots_count", len(snapshots))

	return snapshots, nil
}
