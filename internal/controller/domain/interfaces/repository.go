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

type InMemoryEnvironmentsRepository interface {
	// SetDependencies must run before Initialize so merged config/Kubernetes updates can rebuild xDS.
	SetDependencies(xdsSnapshotManager *xdscache.SnapshotManager, xdsBuilder XDSBuilder, schemaRepo SchemaRepository)
	Initialize(config *config.Config) error
	GetEnvironment(ctx context.Context, name string) (*models.Environment, error)
	ListEnvironments(ctx context.Context) (map[string]*models.Environment, error)
	ApplyKubernetesEnvironments(ctx context.Context, envs map[string]*models.Environment) error
}
