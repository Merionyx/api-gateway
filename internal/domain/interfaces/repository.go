package interfaces

import (
	"context"

	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/repository/git"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type SchemaRepository interface {
	// Save contract snapshot
	SaveContractSnapshot(ctx context.Context, repo, ref, contract string, snapshot *git.ContractSnapshot) error

	// Get contract snapshot
	GetContractSnapshot(ctx context.Context, repo, ref, contract string) (*git.ContractSnapshot, error)

	// Get all snapshots for environment
	GetEnvironmentSnapshots(ctx context.Context, envName string) ([]git.ContractSnapshot, error)

	// List contract snapshots
	ListContractSnapshots(ctx context.Context, repository, ref string) ([]git.ContractSnapshot, error)
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
