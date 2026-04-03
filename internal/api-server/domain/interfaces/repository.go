package interfaces

import (
	"context"
	"merionyx/api-gateway/internal/api-server/domain/models"
	sharedgit "merionyx/api-gateway/internal/shared/git"
)

type SnapshotRepository interface {
	// SaveSnapshots returns true if at least one etcd key was written (revision changed).
	SaveSnapshots(ctx context.Context, bundleKey string, snapshots []sharedgit.ContractSnapshot) (written bool, err error)
	GetSnapshots(ctx context.Context, bundleKey string) ([]sharedgit.ContractSnapshot, error)
	ListBundleKeys(ctx context.Context) ([]string, error)
}

type ControllerRepository interface {
	RegisterController(ctx context.Context, info models.ControllerInfo) error
	GetController(ctx context.Context, controllerID string) (*models.ControllerInfo, error)
	ListControllers(ctx context.Context) ([]models.ControllerInfo, error)
	// UpdateControllerHeartbeat returns true if the main controller record in etcd was rewritten
	// (environments or other fields changed). Heartbeat subkey is always updated.
	UpdateControllerHeartbeat(ctx context.Context, controllerID string, environments []models.EnvironmentInfo) (mainKeyUpdated bool, err error)
}
