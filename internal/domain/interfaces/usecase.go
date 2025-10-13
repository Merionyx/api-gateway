package interfaces

import (
	"context"

	"merionyx/api-gateway/control-plane/internal/domain/models"

	"github.com/google/uuid"
)

// TenantUseCase interface for tenant business logic
type TenantUseCase interface {
	// CreateTenant creates a new tenant
	CreateTenant(ctx context.Context, req *models.CreateTenantRequest) (*models.Tenant, error)

	// GetTenantByID gets a tenant by ID
	GetTenantByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error)

	// GetTenantByName gets a tenant by name
	GetTenantByName(ctx context.Context, name string) (*models.Tenant, error)

	// GetAllTenants gets all tenants
	GetAllTenants(ctx context.Context) ([]*models.Tenant, error)

	// UpdateTenant updates a tenant
	UpdateTenant(ctx context.Context, id uuid.UUID, req *models.UpdateTenantRequest) (*models.Tenant, error)

	// DeleteTenant deletes a tenant
	DeleteTenant(ctx context.Context, id uuid.UUID) error
}

// EnvironmentUseCase interface for environment business logic
type EnvironmentUseCase interface {
	// CreateEnvironment creates a new environment
	CreateEnvironment(ctx context.Context, req *models.CreateEnvironmentRequest) (*models.Environment, error)

	// GetEnvironmentByID gets an environment by ID
	GetEnvironmentByID(ctx context.Context, id uuid.UUID) (*models.Environment, error)

	// GetEnvironmentByName gets an environment by name
	GetEnvironmentByName(ctx context.Context, name string) (*models.Environment, error)

	// GetAllEnvironments gets all environments
	GetAllEnvironments(ctx context.Context) ([]*models.Environment, error)

	// GetEnvironmentsByTenantID gets environments by tenant ID
	GetEnvironmentsByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*models.Environment, error)

	// UpdateEnvironment updates an environment
	UpdateEnvironment(ctx context.Context, id uuid.UUID, req *models.UpdateEnvironmentRequest) (*models.Environment, error)

	// DeleteEnvironment deletes an environment
	DeleteEnvironment(ctx context.Context, id uuid.UUID) error

	// MapEnvironmentToTenant maps an environment to a tenant
	MapEnvironmentToTenant(ctx context.Context, environmentID, tenantID uuid.UUID) error

	// UnmapEnvironmentFromTenant unmaps an environment from a tenant
	UnmapEnvironmentFromTenant(ctx context.Context, environmentID, tenantID uuid.UUID) error
}

// ListenerUseCase interface for listener business logic
type ListenerUseCase interface {
	// CreateListener creates a new listener
	CreateListener(ctx context.Context, req *models.CreateListenerRequest) (*models.Listener, error)

	// GetListenerByID gets a listener by ID
	GetListenerByID(ctx context.Context, id uuid.UUID) (*models.Listener, error)

	// GetListenerByName gets a listener by name
	GetListenerByName(ctx context.Context, name string) (*models.Listener, error)

	// GetAllListeners gets all listeners
	GetAllListeners(ctx context.Context) ([]*models.Listener, error)

	// GetListenersByEnvironmentID gets listeners by environment ID
	GetListenersByEnvironmentID(ctx context.Context, environmentID uuid.UUID) ([]*models.Listener, error)

	// UpdateListener updates a listener
	UpdateListener(ctx context.Context, id uuid.UUID, req *models.UpdateListenerRequest) (*models.Listener, error)

	// DeleteListener deletes a listener
	DeleteListener(ctx context.Context, id uuid.UUID) error

	// MapListenerToEnvironment maps a listener to an environment
	MapListenerToEnvironment(ctx context.Context, listenerID, environmentID uuid.UUID) error

	// UnmapListenerFromEnvironment unmaps a listener from an environment
	UnmapListenerFromEnvironment(ctx context.Context, listenerID, environmentID uuid.UUID) error
}
