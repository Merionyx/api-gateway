package interfaces

import (
	"context"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"

	xdscache "github.com/merionyx/api-gateway/internal/controller/xds/cache"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// SnapshotsUseCase is xDS snapshot orchestration. SetDependencies breaks construction cycles:
// the container calls New* then wires peers in a fixed order before serving traffic.
type SnapshotsUseCase interface {
	SetDependencies(environmentUseCase EnvironmentsUseCase, xdsSnapshotManager *xdscache.SnapshotManager, xdsBuilder XDSBuilder)

	UpdateSnapshot(ctx context.Context, req *models.UpdateSnapshotRequest) (*models.UpdateSnapshotResponse, error)
	GetSnapshotStatus(ctx context.Context, req *models.GetSnapshotStatusRequest) (*models.GetSnapshotStatusResponse, error)
}

// EnvironmentsUseCase manages environments in etcd and xDS. SetDependencies is two-phase wiring (see container).
type EnvironmentsUseCase interface {
	SetDependencies(environmentRepo EnvironmentRepository, inMemory InMemoryEnvironmentsRepository, schemasUseCase SchemasUseCase, eff EffectiveReconciler)

	CreateEnvironment(ctx context.Context, req *models.CreateEnvironmentRequest) (*models.Environment, error)
	GetEnvironment(ctx context.Context, name string) (*models.Environment, error)
	ListEnvironments(ctx context.Context) (map[string]*models.Environment, error)
	UpdateEnvironment(ctx context.Context, req *models.UpdateEnvironmentRequest) (*models.Environment, error)
	DeleteEnvironment(ctx context.Context, name string) error
}

// SchemasUseCase loads contract snapshots. SetDependencies is two-phase wiring (see container).
type SchemasUseCase interface {
	SetDependencies(schemaRepo SchemaRepository, environmentRepo EnvironmentRepository)

	SyncContractBundle(ctx context.Context, req *models.SyncContractBundleRequest) (*models.SyncContractBundleResponse, error)
	GetContractSnapshot(ctx context.Context, repository, ref, bundlePath, contract string) (*models.ContractSnapshot, error)
	ListContractSnapshots(ctx context.Context, repository, ref, bundlePath string) ([]models.ContractSnapshot, error)
	SyncAllContracts(ctx context.Context, req *models.SyncAllContractsRequest) (*models.SyncAllContractsResponse, error)

	WatchContractBundlesSnapshots(ctx context.Context) clientv3.WatchChan
}
