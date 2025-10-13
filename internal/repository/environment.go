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

type environmentRepository struct {
	db      *pgxpool.Pool
	queries *pg_queries.Queries
}

// NewEnvironmentRepository creates a new instance of EnvironmentRepository
func NewEnvironmentRepository(db *pgxpool.Pool) interfaces.EnvironmentRepository {
	return &environmentRepository{
		db:      db,
		queries: pg_queries.New(db),
	}
}

func (r *environmentRepository) Create(ctx context.Context, environment *models.Environment) error {
	err := r.queries.CreateEnvironment(ctx, pg_queries.CreateEnvironmentParams{
		Uuid:   environment.ID,
		Name:   environment.Name,
		Config: environment.Config,
	})
	if err != nil {
		return fmt.Errorf("error creating environment in DB: %w", err)
	}
	return nil
}

func (r *environmentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Environment, error) {
	dbEnvironment, err := r.queries.GetEnvironmentByUUID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("environment with ID %s not found", id)
		}
		return nil, fmt.Errorf("error getting environment from DB: %w", err)
	}

	return &models.Environment{
		ID:     dbEnvironment.Uuid,
		Name:   dbEnvironment.Name,
		Config: dbEnvironment.Config,
	}, nil
}

func (r *environmentRepository) GetByName(ctx context.Context, name string) (*models.Environment, error) {
	dbEnvironment, err := r.queries.GetEnvironmentByName(ctx, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("environment with name %s not found", name)
		}
		return nil, fmt.Errorf("error getting environment from DB: %w", err)
	}

	return &models.Environment{
		ID:     dbEnvironment.Uuid,
		Name:   dbEnvironment.Name,
		Config: dbEnvironment.Config,
	}, nil
}

func (r *environmentRepository) GetAll(ctx context.Context) ([]*models.Environment, error) {
	dbEnvironments, err := r.queries.GetEnvironments(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting list of environments from DB: %w", err)
	}

	environments := make([]*models.Environment, len(dbEnvironments))
	for i, dbEnvironment := range dbEnvironments {
		environments[i] = &models.Environment{
			ID:     dbEnvironment.Uuid,
			Name:   dbEnvironment.Name,
			Config: dbEnvironment.Config,
		}
	}

	return environments, nil
}

func (r *environmentRepository) GetByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*models.Environment, error) {
	dbEnvironments, err := r.queries.GetEnvironmentsByTenantUUID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("error getting environments by tenant ID from DB: %w", err)
	}

	environments := make([]*models.Environment, len(dbEnvironments))
	for i, dbEnvironment := range dbEnvironments {
		environments[i] = &models.Environment{
			ID:       dbEnvironment.Uuid,
			Name:     dbEnvironment.Name,
			Config:   dbEnvironment.Config,
			TenantID: tenantID,
		}
	}

	return environments, nil
}

func (r *environmentRepository) Update(ctx context.Context, environment *models.Environment) error {
	err := r.queries.UpdateEnvironment(ctx, pg_queries.UpdateEnvironmentParams{
		Uuid:   environment.ID,
		Name:   environment.Name,
		Config: environment.Config,
	})
	if err != nil {
		return fmt.Errorf("error updating environment in DB: %w", err)
	}
	return nil
}

func (r *environmentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// First delete the relationships with tenants
	err := r.queries.DeleteEnvironmentTenantMappings(ctx, id)
	if err != nil {
		return fmt.Errorf("error deleting environment tenant relationships: %w", err)
	}

	// Then delete the relationships with listeners
	err = r.queries.DeleteEnvironmentListenerMappings(ctx, id)
	if err != nil {
		return fmt.Errorf("error deleting environment listener relationships: %w", err)
	}

	// Then delete the environment itself
	err = r.queries.DeleteEnvironment(ctx, id)
	if err != nil {
		return fmt.Errorf("error deleting environment from DB: %w", err)
	}
	return nil
}

func (r *environmentRepository) MapToTenant(ctx context.Context, environmentID, tenantID uuid.UUID) error {
	err := r.queries.MapTenantToEnvironment(ctx, pg_queries.MapTenantToEnvironmentParams{
		TenantUuid:      tenantID,
		EnvironmentUuid: environmentID,
	})
	if err != nil {
		return fmt.Errorf("error mapping environment to tenant in DB: %w", err)
	}
	return nil
}

func (r *environmentRepository) UnmapFromTenant(ctx context.Context, environmentID, tenantID uuid.UUID) error {
	err := r.queries.UnmapTenantFromEnvironment(ctx, pg_queries.UnmapTenantFromEnvironmentParams{
		TenantUuid:      tenantID,
		EnvironmentUuid: environmentID,
	})
	if err != nil {
		return fmt.Errorf("error unmapping environment from tenant in DB: %w", err)
	}
	return nil
}
