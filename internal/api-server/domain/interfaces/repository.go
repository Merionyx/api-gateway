package interfaces

import (
	"context"
	"merionyx/api-gateway/internal/api-server/domain/models"
	sharedgit "merionyx/api-gateway/internal/shared/git"
)

type SnapshotRepository interface {
	SaveSnapshots(ctx context.Context, bundleKey string, snapshots []sharedgit.ContractSnapshot) error
	GetSnapshots(ctx context.Context, bundleKey string) ([]sharedgit.ContractSnapshot, error)
	ListBundleKeys(ctx context.Context) ([]string, error)
}

type ControllerRepository interface {
	RegisterController(ctx context.Context, info models.ControllerInfo) error
	GetController(ctx context.Context, controllerID string) (*models.ControllerInfo, error)
	ListControllers(ctx context.Context) ([]models.ControllerInfo, error)
	UpdateControllerHeartbeat(ctx context.Context, controllerID string, environments []models.EnvironmentInfo) error
}
