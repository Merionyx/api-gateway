package usecase

import (
	"context"
	"fmt"
	"time"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"

	"github.com/google/uuid"
)

type tenantUseCase struct {
	tenantRepo interfaces.TenantRepository
}

// NewTenantUseCase creates a new instance of TenantUseCase
func NewTenantUseCase(tenantRepo interfaces.TenantRepository) interfaces.TenantUseCase {
	return &tenantUseCase{
		tenantRepo: tenantRepo,
	}
}

func (uc *tenantUseCase) CreateTenant(ctx context.Context, req *models.CreateTenantRequest) (*models.Tenant, error) {
	// Check if the tenant with the same name does not exist
	existingTenant, err := uc.tenantRepo.GetByName(ctx, req.Name)
	if err == nil && existingTenant != nil {
		return nil, fmt.Errorf("tenant with name '%s' already exists", req.Name)
	}

	tenant := &models.Tenant{
		ID:        uuid.New(),
		Name:      req.Name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := uc.tenantRepo.Create(ctx, tenant); err != nil {
		return nil, fmt.Errorf("error creating tenant: %w", err)
	}

	return tenant, nil
}

func (uc *tenantUseCase) GetTenantByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error) {
	tenant, err := uc.tenantRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("error getting tenant by ID: %w", err)
	}

	return tenant, nil
}

func (uc *tenantUseCase) GetTenantByName(ctx context.Context, name string) (*models.Tenant, error) {
	tenant, err := uc.tenantRepo.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("error getting tenant by name: %w", err)
	}

	return tenant, nil
}

func (uc *tenantUseCase) GetAllTenants(ctx context.Context) ([]*models.Tenant, error) {
	tenants, err := uc.tenantRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting list of tenants: %w", err)
	}

	return tenants, nil
}

func (uc *tenantUseCase) UpdateTenant(ctx context.Context, id uuid.UUID, req *models.UpdateTenantRequest) (*models.Tenant, error) {
	// Check if the tenant exists
	tenant, err := uc.tenantRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}

	// Check if the tenant with the new name does not exist (if the name changed)
	if tenant.Name != req.Name {
		existingTenant, err := uc.tenantRepo.GetByName(ctx, req.Name)
		if err == nil && existingTenant != nil {
			return nil, fmt.Errorf("tenant with name '%s' already exists", req.Name)
		}
	}

	tenant.Name = req.Name
	tenant.UpdatedAt = time.Now()

	if err := uc.tenantRepo.Update(ctx, tenant); err != nil {
		return nil, fmt.Errorf("error updating tenant: %w", err)
	}

	return tenant, nil
}

func (uc *tenantUseCase) DeleteTenant(ctx context.Context, id uuid.UUID) error {
	// Check if the tenant exists
	_, err := uc.tenantRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("tenant not found: %w", err)
	}

	if err := uc.tenantRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("error deleting tenant: %w", err)
	}

	return nil
}
