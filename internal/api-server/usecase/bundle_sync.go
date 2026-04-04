package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"merionyx/api-gateway/internal/api-server/domain/interfaces"
	"merionyx/api-gateway/internal/api-server/domain/models"
	"merionyx/api-gateway/internal/shared/bundlekey"
	"merionyx/api-gateway/internal/shared/election"
	sharedgit "merionyx/api-gateway/internal/shared/git"
	"merionyx/api-gateway/internal/shared/grpcobs"
	"merionyx/api-gateway/internal/shared/grpcutil"
	pb "merionyx/api-gateway/pkg/api/contract_syncer/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// errContractSyncerRejected marks a non-transient failure from the syncer (invalid bundle, etc.); do not gRPC-retry.
var errContractSyncerRejected = errors.New("contract syncer rejected sync")

type BundleSyncUseCase struct {
	snapshotRepo       interfaces.SnapshotRepository
	controllerRepo     interfaces.ControllerRepository
	contractSyncerAddr string
	contractSyncerTLS  grpcobs.ClientTLSConfig
	leader             election.LeaderGate
}

func NewBundleSyncUseCase(
	snapshotRepo interfaces.SnapshotRepository,
	controllerRepo interfaces.ControllerRepository,
	contractSyncerAddr string,
	contractSyncerTLS grpcobs.ClientTLSConfig,
	leader election.LeaderGate,
) *BundleSyncUseCase {
	if leader == nil {
		leader = election.NoopGate{}
	}
	return &BundleSyncUseCase{
		snapshotRepo:       snapshotRepo,
		controllerRepo:     controllerRepo,
		contractSyncerAddr: contractSyncerAddr,
		contractSyncerTLS:  contractSyncerTLS,
		leader:             leader,
	}
}

func (uc *BundleSyncUseCase) grpcDialOptions() ([]grpc.DialOption, error) {
	tlsOpts, err := grpcobs.DialOptions(uc.contractSyncerTLS)
	if err != nil {
		return nil, err
	}
	return append(tlsOpts,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                20 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	), nil
}

// SyncBundle pulls schemas from Contract Syncer (with transient gRPC retries), writes API Server etcd, notifies controllers.
func (uc *BundleSyncUseCase) SyncBundle(ctx context.Context, bundle models.BundleInfo) ([]sharedgit.ContractSnapshot, error) {
	slog.Info("Syncing bundle", "repository", bundle.Repository, "ref", bundle.Ref, "path", bundle.Path)

	const maxAttempts = 5
	backoff := time.Duration(0)
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if backoff > 0 {
			if err := grpcutil.SleepOrDone(ctx, backoff); err != nil {
				return nil, err
			}
		}

		snapshots, err := uc.syncBundleOnce(ctx, bundle)
		if err == nil {
			return snapshots, nil
		}
		if errors.Is(err, errContractSyncerRejected) {
			return nil, err
		}
		lastErr = err
		slog.Warn("Contract Syncer sync attempt failed", "attempt", attempt, "max", maxAttempts, "error", err)
		backoff = grpcutil.NextReconnectBackoff(backoff, 400*time.Millisecond, 10*time.Second)
	}

	return nil, fmt.Errorf("contract syncer after %d attempts: %w", maxAttempts, lastErr)
}

func (uc *BundleSyncUseCase) syncBundleOnce(ctx context.Context, bundle models.BundleInfo) ([]sharedgit.ContractSnapshot, error) {
	dialOpts, err := uc.grpcDialOptions()
	if err != nil {
		return nil, fmt.Errorf("contract syncer dial options: %w", err)
	}
	conn, err := grpc.NewClient(uc.contractSyncerAddr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("dial contract syncer: %w", err)
	}
	defer conn.Close()

	client := pb.NewContractSyncerServiceClient(conn)
	resp, err := client.Sync(ctx, &pb.SyncRequest{
		Repository: bundle.Repository,
		Ref:        bundle.Ref,
		Path:       bundle.Path,
	})
	if err != nil {
		return nil, fmt.Errorf("sync rpc: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("%w: %s", errContractSyncerRejected, resp.Error)
	}

	var snapshots []sharedgit.ContractSnapshot
	for _, pbSnapshot := range resp.Snapshots {
		var apps []sharedgit.App
		if acc := pbSnapshot.GetAccess(); acc != nil {
			for _, pbApp := range acc.GetApps() {
				apps = append(apps, sharedgit.App{
					AppID:        pbApp.GetAppId(),
					Environments: pbApp.GetEnvironments(),
				})
			}
		}
		upstreamName := ""
		if u := pbSnapshot.GetUpstream(); u != nil {
			upstreamName = u.GetName()
		}
		secure := false
		if acc := pbSnapshot.GetAccess(); acc != nil {
			secure = acc.GetSecure()
		}
		snapshots = append(snapshots, sharedgit.ContractSnapshot{
			Name:                  pbSnapshot.GetName(),
			Prefix:                pbSnapshot.GetPrefix(),
			Upstream:              sharedgit.ContractUpstream{Name: upstreamName},
			AllowUndefinedMethods: pbSnapshot.GetAllowUndefinedMethods(),
			Access: sharedgit.Access{
				Secure: secure,
				Apps:   apps,
			},
		})
	}

	bundleKey := bundlekey.Build(bundle.Repository, bundle.Ref, bundle.Path)
	written, err := uc.snapshotRepo.SaveSnapshots(ctx, bundleKey, snapshots)
	if err != nil {
		return nil, fmt.Errorf("save snapshots: %w", err)
	}
	if !written {
		slog.Debug("API Server: bundle sync finished, no etcd snapshot keys changed", "bundle_key", bundleKey)
	}

	// Controllers are notified via etcd watch only when a snapshot key revision changes.

	return snapshots, nil
}

func (uc *BundleSyncUseCase) StartBundleWatcher(ctx context.Context) {
	slog.Info("Starting bundle watcher")

	// Initial sync runs once this replica becomes leader (avoids missing the first tick when election is slow).
	go uc.runInitialSyncWhenLeader(ctx)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Bundle watcher stopped")
			return
		case <-ticker.C:
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

	bundlesMap := make(map[string]models.BundleInfo)
	for _, controller := range controllers {
		for _, env := range controller.Environments {
			for _, bundle := range env.Bundles {
				key := bundlekey.Build(bundle.Repository, bundle.Ref, bundle.Path)
				bundlesMap[key] = bundle
			}
		}
	}

	for _, bundle := range bundlesMap {
		if _, err := uc.SyncBundle(ctx, bundle); err != nil {
			slog.Error("Failed to sync bundle", "bundle", bundle, "error", err)
		}
	}
}
