package interfaces

import (
	"context"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

type ControllerRegistryUseCase interface {
	RegisterController(ctx context.Context, info models.ControllerInfo) error
	StreamSnapshots(ctx context.Context, controllerID string, stream SnapshotStream) error
	Heartbeat(ctx context.Context, controllerID string, environments []models.EnvironmentInfo, registryPayloadVersion int32) error
	// StartEtcdWatch runs until ctx is cancelled; it debounces etcd events and notifies connected controller streams on this process.
	StartEtcdWatch(ctx context.Context)
}

type SnapshotStream interface {
	Send(environment, bundleKey string, snapshots []sharedgit.ContractSnapshot) error
}

type BundleSyncUseCase interface {
	SyncBundle(ctx context.Context, bundle models.BundleInfo) ([]sharedgit.ContractSnapshot, error)
	StartBundleWatcher(ctx context.Context)
}
