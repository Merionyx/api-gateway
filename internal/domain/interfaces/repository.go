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
}
type EnvironmentRepository interface {
	// Save environment configuration
	SaveEnvironment(ctx context.Context, env *models.Environment) error

	// Get environment
	GetEnvironment(ctx context.Context, name string) (*models.Environment, error)

	// Get all environments
	ListEnvironments(ctx context.Context) (map[string]*models.Environment, error)

	// Watch changes
	WatchEnvironments(ctx context.Context) clientv3.WatchChan
}

// import (
// 	"context"

// 	"merionyx/api-gateway/control-plane/internal/domain/models"

// 	"github.com/google/uuid"
// )

// // TenantRepository interface for working with tenants
// type TenantRepository interface {
// 	// Create creates a new tenant
// 	Create(ctx context.Context, tenant *models.Tenant) error

// 	// GetByID gets a tenant by ID
// 	GetByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error)

// 	// GetByName gets a tenant by name
// 	GetByName(ctx context.Context, name string) (*models.Tenant, error)

// 	// GetAll gets all tenants
// 	GetAll(ctx context.Context) ([]*models.Tenant, error)

// 	// Update updates a tenant
// 	Update(ctx context.Context, tenant *models.Tenant) error

// 	// Delete deletes a tenant
// 	Delete(ctx context.Context, id uuid.UUID) error
// }

// // EnvironmentRepository interface for working with environments
// type EnvironmentRepository interface {
// 	// Create creates a new environment
// 	Create(ctx context.Context, environment *models.Environment) error

// 	// GetByID gets an environment by ID
// 	GetByID(ctx context.Context, id uuid.UUID) (*models.Environment, error)

// 	// GetByName gets an environment by name
// 	GetByName(ctx context.Context, name string) (*models.Environment, error)

// 	// GetAll gets all environments
// 	GetAll(ctx context.Context) ([]*models.Environment, error)

// 	// GetByTenantID gets environments by tenant ID
// 	GetByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*models.Environment, error)

// 	// Update updates an environment
// 	Update(ctx context.Context, environment *models.Environment) error

// 	// Delete deletes an environment
// 	Delete(ctx context.Context, id uuid.UUID) error

// 	// MapToTenant maps an environment to a tenant
// 	MapToTenant(ctx context.Context, environmentID, tenantID uuid.UUID) error

// 	// UnmapFromTenant unmaps an environment from a tenant
// 	UnmapFromTenant(ctx context.Context, environmentID, tenantID uuid.UUID) error
// }

// // ListenerRepository interface for working with listeners
// type ListenerRepository interface {
// 	// Create creates a new listener
// 	Create(ctx context.Context, listener *models.Listener) error

// 	// GetByID gets a listener by ID
// 	GetByID(ctx context.Context, id uuid.UUID) (*models.Listener, error)

// 	// GetByName gets a listener by name
// 	GetByName(ctx context.Context, name string) (*models.Listener, error)

// 	// GetAll gets all listeners
// 	GetAll(ctx context.Context) ([]*models.Listener, error)

// 	// GetByEnvironmentID gets listeners by environment ID
// 	GetByEnvironmentID(ctx context.Context, environmentID uuid.UUID) ([]*models.Listener, error)

// 	// Update updates a listener
// 	Update(ctx context.Context, listener *models.Listener) error

// 	// Delete deletes a listener
// 	Delete(ctx context.Context, id uuid.UUID) error

// 	// MapToEnvironment maps a listener to an environment
// 	MapToEnvironment(ctx context.Context, listenerID, environmentID uuid.UUID) error

// 	// UnmapFromEnvironment unmaps a listener from an environment
// 	UnmapFromEnvironment(ctx context.Context, listenerID, environmentID uuid.UUID) error
// }
