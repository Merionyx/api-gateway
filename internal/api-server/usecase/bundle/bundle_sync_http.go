package bundle

import (
	"context"

	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
)

// BundleHTTPSyncUseCase implements HTTP POST /api/v1/bundles/sync semantics (from_cache when etcd already has snapshots and force is false).
type BundleHTTPSyncUseCase struct {
	snapshots interfaces.SnapshotRepository
	syncer    interfaces.BundleSyncUseCase
}

func NewBundleHTTPSyncUseCase(
	snapshots interfaces.SnapshotRepository,
	syncer interfaces.BundleSyncUseCase,
) *BundleHTTPSyncUseCase {
	return &BundleHTTPSyncUseCase{snapshots: snapshots, syncer: syncer}
}

// Sync returns materialized snapshots and whether they were read from etcd without calling the Contract Syncer.
func (u *BundleHTTPSyncUseCase) Sync(ctx context.Context, repository, ref, bundleName string, force bool) (fromCache bool, snapshots []sharedgit.ContractSnapshot, err error) {
	bundle := models.BundleInfo{
		Name:       bundleName,
		Repository: repository,
		Ref:        ref,
		Path:       "",
	}
	bk := bundlekey.Build(bundle.Repository, bundle.Ref, bundle.Path)

	if !force {
		cached, err := u.snapshots.GetSnapshots(ctx, bk)
		if err != nil {
			return false, nil, err
		}
		if len(cached) > 0 {
			return true, cached, nil
		}
	}

	snapshots, err = u.syncer.SyncBundle(ctx, bundle)
	if err != nil {
		return false, nil, err
	}
	return false, snapshots, nil
}
