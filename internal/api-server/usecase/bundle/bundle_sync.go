package bundle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	apimetrics "github.com/merionyx/api-gateway/internal/api-server/metrics"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
	"github.com/merionyx/api-gateway/internal/shared/election"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
	"github.com/merionyx/api-gateway/internal/shared/grpcutil"
)

// ErrContractSyncerRejected is re-exported for backward compatibility; prefer apierrors.ErrContractSyncerRejected.
var ErrContractSyncerRejected = apierrors.ErrContractSyncerRejected

type BundleSyncUseCase struct {
	snapshotRepo   interfaces.SnapshotRepository
	controllerRepo interfaces.ControllerRepository
	syncRemote     interfaces.ContractSyncRemote
	leader         election.LeaderGate
	metricsEnabled bool
}

func NewBundleSyncUseCase(
	snapshotRepo interfaces.SnapshotRepository,
	controllerRepo interfaces.ControllerRepository,
	syncRemote interfaces.ContractSyncRemote,
	leader election.LeaderGate,
	metricsEnabled bool,
) *BundleSyncUseCase {
	if leader == nil {
		leader = election.NoopGate{}
	}
	return &BundleSyncUseCase{
		snapshotRepo:   snapshotRepo,
		controllerRepo: controllerRepo,
		syncRemote:     syncRemote,
		leader:         leader,
		metricsEnabled: metricsEnabled,
	}
}

// SyncBundle pulls schemas from Contract Syncer (with transient retries), writes API Server etcd, notifies controllers.
func (uc *BundleSyncUseCase) SyncBundle(ctx context.Context, bundle models.BundleInfo) ([]sharedgit.ContractSnapshot, error) {
	slog.Info("Syncing bundle", "repository", bundle.Repository, "ref", bundle.Ref, "path", bundle.Path)

	start := time.Now()
	const maxAttempts = 5
	backoff := time.Duration(0)
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			apimetrics.RecordBundleSyncOutcome(uc.metricsEnabled, apimetrics.BundleOutcomeCtxCanceled)
			return nil, err
		}
		if backoff > 0 {
			if err := grpcutil.SleepOrDone(ctx, backoff); err != nil {
				apimetrics.RecordBundleSyncOutcome(uc.metricsEnabled, apimetrics.BundleOutcomeCtxCanceled)
				return nil, err
			}
		}

		snapshots, err := uc.syncBundleOnce(ctx, bundle)
		if err == nil {
			apimetrics.RecordBundleSyncOutcome(uc.metricsEnabled, apimetrics.BundleOutcomeSuccess)
			apimetrics.RecordBundleSyncDuration(uc.metricsEnabled, time.Since(start))
			return snapshots, nil
		}
		if errors.Is(err, apierrors.ErrContractSyncerRejected) {
			apimetrics.RecordBundleSyncOutcome(uc.metricsEnabled, apimetrics.BundleOutcomeRejected)
			return nil, err
		}
		lastErr = err
		slog.Warn("Contract Syncer sync attempt failed", "attempt", attempt, "max", maxAttempts, "error", err)
		backoff = grpcutil.NextReconnectBackoff(backoff, 400*time.Millisecond, 10*time.Second)
	}

	apimetrics.RecordBundleSyncOutcome(uc.metricsEnabled, apimetrics.BundleOutcomeFailed)
	return nil, apierrors.JoinContractSyncer(fmt.Sprintf("syncBundle after %d attempts", maxAttempts), lastErr)
}

func (uc *BundleSyncUseCase) syncBundleOnce(ctx context.Context, bundle models.BundleInfo) ([]sharedgit.ContractSnapshot, error) {
	snapshots, err := uc.syncRemote.FetchContractSnapshots(ctx, bundle)
	if err != nil {
		if errors.Is(err, apierrors.ErrContractSyncerRejected) {
			apimetrics.RecordBundleSyncAttempt(uc.metricsEnabled, apimetrics.BundleAttemptResponseError)
		} else {
			apimetrics.RecordBundleSyncAttempt(uc.metricsEnabled, apimetrics.BundleAttemptRPCError)
		}
		return nil, err
	}

	bundleKey := bundlekey.Build(bundle.Repository, bundle.Ref, bundle.Path)
	written, err := uc.snapshotRepo.SaveSnapshots(ctx, bundleKey, snapshots)
	if err != nil {
		apimetrics.RecordBundleSyncAttempt(uc.metricsEnabled, apimetrics.BundleAttemptSaveError)
		return nil, apierrors.JoinStore("save snapshots after sync", err)
	}
	if !written {
		slog.Debug("API Server: bundle sync finished, no etcd snapshot keys changed", "bundle_key", bundleKey)
	}

	apimetrics.RecordBundleSyncAttempt(uc.metricsEnabled, apimetrics.BundleAttemptOK)
	apimetrics.RecordBundleEtcdWrite(uc.metricsEnabled, written)
	return snapshots, nil
}

func (uc *BundleSyncUseCase) StartBundleWatcher(ctx context.Context) {
	slog.Info("Starting bundle watcher")

	apimetrics.SetLeader(uc.metricsEnabled, uc.leader.IsLeader())
	if ch := uc.leader.LeaderChanged(); ch != nil {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-ch:
					apimetrics.SetLeader(uc.metricsEnabled, uc.leader.IsLeader())
				}
			}
		}()
	}

	go uc.runInitialSyncWhenLeader(ctx)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Bundle watcher stopped")
			return
		case <-ticker.C:
			apimetrics.SetLeader(uc.metricsEnabled, uc.leader.IsLeader())
			if !uc.leader.IsLeader() {
				slog.Debug("Skipping bundle sync tick: not leader")
				continue
			}
			uc.syncAllBundles(ctx)
		}
	}
}

func (uc *BundleSyncUseCase) runInitialSyncWhenLeader(ctx context.Context) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		if uc.leader.IsLeader() {
			apimetrics.SetLeader(uc.metricsEnabled, true)
			slog.Info("Running initial bundle sync to api-server etcd (leader)")
			uc.syncAllBundles(ctx)
			return
		}
		if err := grpcutil.SleepOrDone(ctx, time.Second); err != nil {
			return
		}
	}
}

func (uc *BundleSyncUseCase) syncAllBundles(ctx context.Context) {
	controllers, err := uc.controllerRepo.ListControllers(ctx)
	if err != nil {
		slog.Error("Failed to list controllers", "error", err)
		return
	}

	bundles := collectUniqueBundles(controllers)
	const maxParallelBundleSync = 8
	runParallelForEachBundle(ctx, bundles, maxParallelBundleSync, func(gctx context.Context, b models.BundleInfo) {
		if _, err := uc.SyncBundle(gctx, b); err != nil {
			slog.Error("Failed to sync bundle", "bundle", b, "error", err)
		}
	})
}
