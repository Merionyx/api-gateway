package repository

import (
	"context"
	"database/sql"
	"fmt"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	pg_queries "merionyx/api-gateway/control-plane/internal/queries"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type tenantRepository struct {
	db      *pgxpool.Pool
	queries *pg_queries.Queries
}

// NewTenantRepository creates a new instance of TenantRepository
func NewTenantRepository(db *pgxpool.Pool) interfaces.TenantRepository {
	return &tenantRepository{
		db:      db,
		queries: pg_queries.New(db),
	}
}

func (r *tenantRepository) Create(ctx context.Context, tenant *models.Tenant) error {
	err := r.queries.CreateTenant(ctx, pg_queries.CreateTenantParams{
		Uuid: tenant.ID,
		Name: tenant.Name,
	})
	if err != nil {
		return fmt.Errorf("error creating tenant in DB: %w", err)
	}
	return nil
}

func (r *tenantRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error) {
	dbTenant, err := r.queries.GetTenantByUUID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tenant with ID %s not found", id)
		}
		return nil, fmt.Errorf("error getting tenant from DB: %w", err)
	}

	return &models.Tenant{
		ID:   dbTenant.Uuid,
		Name: dbTenant.Name,
	}, nil
}

func (r *tenantRepository) GetByName(ctx context.Context, name string) (*models.Tenant, error) {
	dbTenant, err := r.queries.GetTenantByName(ctx, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tenant with name %s not found", name)
		}
		return nil, fmt.Errorf("error getting tenant from DB: %w", err)
	}

	return &models.Tenant{
		ID:   dbTenant.Uuid,
		Name: dbTenant.Name,
	}, nil
}

func (r *tenantRepository) GetAll(ctx context.Context) ([]*models.Tenant, error) {
	dbTenants, err := r.queries.GetTenants(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting list of tenants from DB: %w", err)
	}

	tenants := make([]*models.Tenant, len(dbTenants))
	for i, dbTenant := range dbTenants {
		tenants[i] = &models.Tenant{
			ID:   dbTenant.Uuid,
			Name: dbTenant.Name,
		}
	}

	return tenants, nil
}

func (r *tenantRepository) Update(ctx context.Context, tenant *models.Tenant) error {
	err := r.queries.UpdateTenant(ctx, pg_queries.UpdateTenantParams{
		Uuid: tenant.ID,
		Name: tenant.Name,
	})
	if err != nil {
		return fmt.Errorf("error updating tenant in DB: %w", err)
	}
	return nil
}

func (r *tenantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// First delete the relationships with environments
	err := r.queries.DeleteTenantEnvironmentMappings(ctx, id)
	if err != nil {
		return fmt.Errorf("error deleting tenant environment relationships: %w", err)
	}

	// Then delete the tenant itself
	err = r.queries.DeleteTenant(ctx, id)
	if err != nil {
		return fmt.Errorf("error deleting tenant from DB: %w", err)
	}
	return nil
}
