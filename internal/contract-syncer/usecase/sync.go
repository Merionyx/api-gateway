package usecase

import (
	"fmt"
	"log/slog"
	"time"

	syncmetrics "github.com/merionyx/api-gateway/internal/contract-syncer/metrics"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
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

func (u *SyncUseCase) ExportContracts(repository, ref, path, contractName string) ([]sharedgit.ExportedContractFile, error) {
	slog.Info("Exporting contracts", "repository", repository, "ref", ref, "path", path, "contract", contractName)
	start := time.Now()
	files, err := u.gitManager.ExportContractFiles(repository, ref, path, contractName)
	if err != nil {
		syncmetrics.RecordGitSyncDuration(u.metricsEnabled, syncmetrics.GitResultError, time.Since(start))
		return nil, err
	}
	syncmetrics.RecordGitSyncDuration(u.metricsEnabled, syncmetrics.GitResultOK, time.Since(start))
	return files, nil
}
