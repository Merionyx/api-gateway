package interfaces

import (
	"context"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

type SnapshotRepository interface {
	// SaveSnapshots returns true if the contract key set changed: at least one Put or Delete under the bundle contracts prefix.
	SaveSnapshots(ctx context.Context, bundleKey string, snapshots []sharedgit.ContractSnapshot) (written bool, err error)
	GetSnapshots(ctx context.Context, bundleKey string) ([]sharedgit.ContractSnapshot, error)
	ListBundleKeys(ctx context.Context) ([]string, error)
}

type ControllerRepository interface {
	RegisterController(ctx context.Context, info models.ControllerInfo) error
	GetController(ctx context.Context, controllerID string) (*models.ControllerInfo, error)
	// GetHeartbeat returns the last stored heartbeat timestamp for the controller.
	// Returns a not-found error when the controller or heartbeat record is missing.
	GetHeartbeat(ctx context.Context, controllerID string) (time.Time, error)
	ListControllers(ctx context.Context) ([]models.ControllerInfo, error)
	// UpdateControllerHeartbeat returns true if the main controller record in etcd was rewritten
	// (environments or other fields changed). Heartbeat subkey is always updated.
	// registryPayloadVersion, when positive, is stored on the controller record.
	UpdateControllerHeartbeat(ctx context.Context, controllerID string, environments []models.EnvironmentInfo, registryPayloadVersion int32) (mainKeyUpdated bool, err error)
}
