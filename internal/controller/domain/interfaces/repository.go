package interfaces

import (
	"context"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	xdscache "github.com/merionyx/api-gateway/internal/controller/xds/cache"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type SchemaRepository interface {
	// Save contract snapshot
	SaveContractSnapshot(ctx context.Context, repo, ref, bundlePath, contract string, snapshot *models.ContractSnapshot) error

	// Get contract snapshot
	GetContractSnapshot(ctx context.Context, repo, ref, bundlePath, contract string) (*models.ContractSnapshot, error)

	// Get all snapshots for environment
	GetEnvironmentSnapshots(ctx context.Context, envName string) ([]models.ContractSnapshot, error)

	// List contract snapshots
	ListContractSnapshots(ctx context.Context, repository, ref, bundlePath string) ([]models.ContractSnapshot, error)

	// Watch contract snapshots
	WatchContractBundlesSnapshots(ctx context.Context) clientv3.WatchChan
}
type EnvironmentRepository interface {
	// Save environment configuration
	SaveEnvironment(ctx context.Context, env *models.Environment) error

	// Get environment
	GetEnvironment(ctx context.Context, name string) (*models.Environment, error)

	// Get all environments
	ListEnvironments(ctx context.Context) (map[string]*models.Environment, error)

	// Delete environment
	DeleteEnvironment(ctx context.Context, name string) error

	// Watch changes
	WatchEnvironments(ctx context.Context) clientv3.WatchChan
}

type InMemoryServiceRepository interface {
	Initialize(config *config.Config) error
	GetService(name string) (*models.StaticServiceConfig, error)
	ListServices() ([]models.StaticServiceConfig, error)
	SetKubernetesGlobalServices(services []models.StaticServiceConfig)
}

// MaterializedEffectiveStore persists idempotent materialized effective (static) to etcd (ADR 0001). Optional; may be nil.
type MaterializedEffectiveStore interface {
	ReconcileIfChanged(ctx context.Context, skel *models.Environment) error
	// Delete removes the materialized v1 key for the environment. No-op for nil/invalid name or if the store is nil.
	Delete(ctx context.Context, environmentName string) error
}

// EffectiveReconciler reconciles the effective environment (in-memory file∪K8s ∪ controller etcd) into
// xDS and optionally materialized JSON in etcd (ADR 0001). Used by the memory repository, Environments
// and API Server sync use cases. May be nil in unit tests.
type EffectiveReconciler interface {
	RebuildAllFromMemory(ctx context.Context, memoryMergedByName map[string]*models.Environment)
	// ReconcileOne is one environment by name. writeMaterialized: true for CRUD paths (leader + flag); false for hot-path follower watch.
	ReconcileOne(ctx context.Context, name string, writeMaterialized bool) error
}

type InMemoryEnvironmentsRepository interface {
	// SetDependencies must run before Initialize so merged config/Kubernetes updates can rebuild xDS.
	SetDependencies(xdsSnapshotManager *xdscache.SnapshotManager, xdsBuilder XDSBuilder, schemaRepo SchemaRepository)
	Initialize(config *config.Config) error
	GetEnvironment(ctx context.Context, name string) (*models.Environment, error)
	ListEnvironments(ctx context.Context) (map[string]*models.Environment, error)
	ApplyKubernetesEnvironments(ctx context.Context, envs map[string]*models.Environment) error
}
