package interfaces

import (
	"context"

	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/domain/models"
	xdscache "merionyx/api-gateway/internal/controller/xds/cache"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type SchemaRepository interface {
	// Save contract snapshot
	SaveContractSnapshot(ctx context.Context, repo, ref, contract string, snapshot *models.ContractSnapshot) error

	// Get contract snapshot
	GetContractSnapshot(ctx context.Context, repo, ref, contract string) (*models.ContractSnapshot, error)

	// Get all snapshots for environment
	GetEnvironmentSnapshots(ctx context.Context, envName string) ([]models.ContractSnapshot, error)

	// List contract snapshots
	ListContractSnapshots(ctx context.Context, repository, ref string) ([]models.ContractSnapshot, error)

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
}

type InMemoryEnvironmentsRepository interface {
	SetDependencies(xdsSnapshotManager *xdscache.SnapshotManager, xdsBuilder XDSBuilder)
	Initialize(config *config.Config) error
	GetEnvironment(ctx context.Context, name string) (*models.Environment, error)
	ListEnvironments(ctx context.Context) (map[string]*models.Environment, error)
}
