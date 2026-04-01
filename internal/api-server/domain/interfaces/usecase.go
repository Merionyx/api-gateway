package interfaces

import (
	"context"
	"merionyx/api-gateway/internal/api-server/domain/models"
	sharedgit "merionyx/api-gateway/internal/shared/git"
)

type ControllerRegistryUseCase interface {
	RegisterController(ctx context.Context, info models.ControllerInfo) error
	StreamSnapshots(ctx context.Context, controllerID string, stream SnapshotStream) error
	Heartbeat(ctx context.Context, controllerID string, environments []models.EnvironmentInfo) error
}

type SnapshotStream interface {
	Send(environment, bundleKey string, snapshots []sharedgit.ContractSnapshot) error
}

type BundleSyncUseCase interface {
	SyncBundle(ctx context.Context, bundle models.BundleInfo) ([]sharedgit.ContractSnapshot, error)
	StartBundleWatcher(ctx context.Context)
}
