package usecase

import (
	"context"
	"fmt"
	"time"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"

	"github.com/google/uuid"
)

type environmentUseCase struct {
	environmentRepo interfaces.EnvironmentRepository
	tenantRepo      interfaces.TenantRepository
}

// NewEnvironmentUseCase creates a new instance of EnvironmentUseCase
func NewEnvironmentUseCase(
	environmentRepo interfaces.EnvironmentRepository,
	tenantRepo interfaces.TenantRepository,
) interfaces.EnvironmentUseCase {
	return &environmentUseCase{
		environmentRepo: environmentRepo,
		tenantRepo:      tenantRepo,
	}
}

func (uc *environmentUseCase) CreateEnvironment(ctx context.Context, req *models.CreateEnvironmentRequest) (*models.Environment, error) {
	// Check if the tenant exists
	_, err := uc.tenantRepo.GetByID(ctx, req.TenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	// Check if the environment with the same name does not exist
	existingEnvironment, err := uc.environmentRepo.GetByName(ctx, req.Name)
	if err == nil && existingEnvironment != nil {
		return nil, fmt.Errorf("environment with name '%s' already exists", req.Name)
	}

	environment := &models.Environment{
		ID:        uuid.New(),
		Name:      req.Name,
		Config:    req.Config,
		TenantID:  req.TenantID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := uc.environmentRepo.Create(ctx, environment); err != nil {
		return nil, fmt.Errorf("error creating environment: %w", err)
	}

	// Map the environment to the tenant
	if err := uc.environmentRepo.MapToTenant(ctx, environment.ID, req.TenantID); err != nil {
		return nil, fmt.Errorf("error mapping environment to tenant: %w", err)
	}

	return environment, nil
}

func (uc *environmentUseCase) GetEnvironmentByID(ctx context.Context, id uuid.UUID) (*models.Environment, error) {
	environment, err := uc.environmentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("error getting environment by ID: %w", err)
	}

	return environment, nil
}

func (uc *environmentUseCase) GetEnvironmentByName(ctx context.Context, name string) (*models.Environment, error) {
	environment, err := uc.environmentRepo.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("error getting environment by name: %w", err)
	}

	return environment, nil
}

func (uc *environmentUseCase) GetAllEnvironments(ctx context.Context) ([]*models.Environment, error) {
	environments, err := uc.environmentRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting list of environments: %w", err)
	}

	return environments, nil
}

func (uc *environmentUseCase) GetEnvironmentsByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*models.Environment, error) {
	// Check if the tenant exists
	_, err := uc.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	environments, err := uc.environmentRepo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("error getting environments by tenant ID: %w", err)
	}

	return environments, nil
}

func (uc *environmentUseCase) UpdateEnvironment(ctx context.Context, id uuid.UUID, req *models.UpdateEnvironmentRequest) (*models.Environment, error) {
	// Check if the environment exists
	environment, err := uc.environmentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("environment not found: %w", err)
	}

	// Check if the environment with the new name does not exist (if the name changed)
	if environment.Name != req.Name {
		existingEnvironment, err := uc.environmentRepo.GetByName(ctx, req.Name)
		if err == nil && existingEnvironment != nil {
			return nil, fmt.Errorf("environment with name '%s' already exists", req.Name)
		}
	}

	environment.Name = req.Name
	environment.Config = req.Config
	environment.UpdatedAt = time.Now()

	if err := uc.environmentRepo.Update(ctx, environment); err != nil {
		return nil, fmt.Errorf("error updating environment: %w", err)
	}

	return environment, nil
}

func (uc *environmentUseCase) DeleteEnvironment(ctx context.Context, id uuid.UUID) error {
	// Check if the environment exists
	_, err := uc.environmentRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("environment not found: %w", err)
	}

	if err := uc.environmentRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("error deleting environment: %w", err)
	}

	return nil
}

func (uc *environmentUseCase) MapEnvironmentToTenant(ctx context.Context, environmentID, tenantID uuid.UUID) error {
	// Check if the environment and tenant exist
	_, err := uc.environmentRepo.GetByID(ctx, environmentID)
	if err != nil {
		return fmt.Errorf("environment not found: %w", err)
	}

	_, err = uc.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("tenant not found: %w", err)
	}

	if err := uc.environmentRepo.MapToTenant(ctx, environmentID, tenantID); err != nil {
		return fmt.Errorf("error mapping environment to tenant: %w", err)
	}

	return nil
}

func (uc *environmentUseCase) UnmapEnvironmentFromTenant(ctx context.Context, environmentID, tenantID uuid.UUID) error {
	// Check if the environment and tenant exist
	_, err := uc.environmentRepo.GetByID(ctx, environmentID)
	if err != nil {
		return fmt.Errorf("environment not found: %w", err)
	}

	_, err = uc.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("tenant not found: %w", err)
	}

	if err := uc.environmentRepo.UnmapFromTenant(ctx, environmentID, tenantID); err != nil {
		return fmt.Errorf("error unmapping environment from tenant: %w", err)
	}

	return nil
}
