package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"merionyx/api-gateway/internal/api-server/domain/interfaces"
	"merionyx/api-gateway/internal/api-server/domain/models"
	sharedgit "merionyx/api-gateway/internal/shared/git"
	pb "merionyx/api-gateway/pkg/api/contract_syncer/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type BundleSyncUseCase struct {
	snapshotRepo       interfaces.SnapshotRepository
	controllerRepo     interfaces.ControllerRepository
	contractSyncerAddr string
	registryUseCase    *ControllerRegistryUseCase
}

func NewBundleSyncUseCase(
	snapshotRepo interfaces.SnapshotRepository,
	controllerRepo interfaces.ControllerRepository,
	contractSyncerAddr string,
) *BundleSyncUseCase {
	return &BundleSyncUseCase{
		snapshotRepo:       snapshotRepo,
		controllerRepo:     controllerRepo,
		contractSyncerAddr: contractSyncerAddr,
	}
}

func (uc *BundleSyncUseCase) SetRegistryUseCase(registryUseCase *ControllerRegistryUseCase) {
	uc.registryUseCase = registryUseCase
}

func (uc *BundleSyncUseCase) SyncBundle(ctx context.Context, bundle models.BundleInfo) ([]sharedgit.ContractSnapshot, error) {
	slog.Info("Syncing bundle", "repository", bundle.Repository, "ref", bundle.Ref, "path", bundle.Path)

	conn, err := grpc.NewClient(uc.contractSyncerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to contract syncer: %w", err)
	}
	defer conn.Close()

	client := pb.NewContractSyncerServiceClient(conn)

	resp, err := client.Sync(ctx, &pb.SyncRequest{
		Repository: bundle.Repository,
		Ref:        bundle.Ref,
		Path:       bundle.Path,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sync bundle: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("contract syncer error: %s", resp.Error)
	}

	var snapshots []sharedgit.ContractSnapshot
	for _, pbSnapshot := range resp.Snapshots {
		var apps []sharedgit.App
		for _, pbApp := range pbSnapshot.Access.Apps {
			apps = append(apps, sharedgit.App{
				AppID:        pbApp.AppId,
				Environments: pbApp.Environments,
			})
		}

		snapshots = append(snapshots, sharedgit.ContractSnapshot{
			Name:   pbSnapshot.Name,
			Prefix: pbSnapshot.Prefix,
			Upstream: sharedgit.ContractUpstream{
				Name: pbSnapshot.Upstream.Name,
			},
			AllowUndefinedMethods: pbSnapshot.AllowUndefinedMethods,
			Access: sharedgit.Access{
				Secure: pbSnapshot.Access.Secure,
				Apps:   apps,
			},
		})
	}

	safeRef := strings.ReplaceAll(bundle.Ref, "/", "%2F")
	safePath := ""
	if bundle.Path == "" {
		safePath = "."
	} else {
		safePath = strings.ReplaceAll(bundle.Path, "/", "%2F")
	}
	bundleKey := fmt.Sprintf("%s/%s/%s", bundle.Repository, safeRef, safePath)
	if err := uc.snapshotRepo.SaveSnapshots(ctx, bundleKey, snapshots); err != nil {
		return nil, fmt.Errorf("failed to save snapshots: %w", err)
	}

	if uc.registryUseCase != nil {
		if err := uc.registryUseCase.NotifySnapshotUpdate(ctx, bundleKey, snapshots); err != nil {
			slog.Error("Failed to notify snapshot update", "error", err)
		}
	}

	return snapshots, nil
}

func (uc *BundleSyncUseCase) StartBundleWatcher(ctx context.Context) {
	slog.Info("Starting bundle watcher")

	// First run through a few seconds to allow the Gateway Controller to register
	// (otherwise /api-gateway/api-server/snapshots/ is empty until the first 30s tick).
	firstSync := time.NewTimer(5 * time.Second)
	defer firstSync.Stop()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Bundle watcher stopped")
			return
		case <-firstSync.C:
			slog.Info("Running initial bundle sync to api-server etcd")
			uc.syncAllBundles(ctx)
		case <-ticker.C:
			uc.syncAllBundles(ctx)
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
				bundleKey := fmt.Sprintf("%s/%s/%s", bundle.Repository, bundle.Ref, bundle.Path)
				bundlesMap[bundleKey] = bundle
			}
		}
	}

	for _, bundle := range bundlesMap {
		if _, err := uc.SyncBundle(ctx, bundle); err != nil {
			slog.Error("Failed to sync bundle", "bundle", bundle, "error", err)
		}
	}
}
