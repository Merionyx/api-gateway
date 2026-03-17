package usecase

import (
	"context"
	"fmt"
	"log"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/repository/git"
	xdscache "merionyx/api-gateway/control-plane/internal/xds/cache"
	"merionyx/api-gateway/control-plane/internal/xds/snapshot"
)

type environmentsUseCase struct {
	environmentRepo    interfaces.EnvironmentRepository
	xdsSnapshotManager *xdscache.SnapshotManager
}

func NewEnvironmentsUseCase(
	environmentRepo interfaces.EnvironmentRepository,
	xdsSnapshotManager *xdscache.SnapshotManager,
) interfaces.EnvironmentsUseCase {
	return &environmentsUseCase{
		environmentRepo:    environmentRepo,
		xdsSnapshotManager: xdsSnapshotManager,
	}
}

func (uc *environmentsUseCase) CreateEnvironment(ctx context.Context, req *models.CreateEnvironmentRequest) (*models.Environment, error) {
	// Check if environment does not exist
	existing, _ := uc.environmentRepo.GetEnvironment(ctx, req.Name)
	if existing != nil {
		return nil, fmt.Errorf("environment %s already exists", req.Name)
	}

	env := &models.Environment{
		Name:      req.Name,
		Bundles:   req.Bundles,
		Services:  req.Services,
		Snapshots: make([]git.ContractSnapshot, 0),
	}

	// Save to etcd
	if err := uc.environmentRepo.SaveEnvironment(ctx, env); err != nil {
		return nil, fmt.Errorf("failed to save environment: %w", err)
	}

	log.Printf("Environment %s created", req.Name)

	// Create initial xDS snapshot
	if err := uc.rebuildXDSSnapshot(ctx, env); err != nil {
		log.Printf("Failed to build initial xDS snapshot for %s: %v", req.Name, err)
	}

	return env, nil
}

func (uc *environmentsUseCase) GetEnvironment(ctx context.Context, name string) (*models.Environment, error) {
	return uc.environmentRepo.GetEnvironment(ctx, name)
}

func (uc *environmentsUseCase) ListEnvironments(ctx context.Context) (map[string]*models.Environment, error) {
	return uc.environmentRepo.ListEnvironments(ctx)
}

func (uc *environmentsUseCase) UpdateEnvironment(ctx context.Context, req *models.UpdateEnvironmentRequest) (*models.Environment, error) {
	// Check if environment exists
	existing, err := uc.environmentRepo.GetEnvironment(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("environment %s not found: %w", req.Name, err)
	}

	// Update fields
	existing.Bundles = req.Bundles
	existing.Services = req.Services

	// Save to etcd
	if err := uc.environmentRepo.SaveEnvironment(ctx, existing); err != nil {
		return nil, fmt.Errorf("failed to update environment: %w", err)
	}

	log.Printf("Environment %s updated", req.Name)

	// Rebuild xDS snapshot
	if err := uc.rebuildXDSSnapshot(ctx, existing); err != nil {
		log.Printf("Failed to rebuild xDS snapshot for %s: %v", req.Name, err)
	}

	return existing, nil
}

func (uc *environmentsUseCase) DeleteEnvironment(ctx context.Context, name string) error {
	// Check if environment exists
	_, err := uc.environmentRepo.GetEnvironment(ctx, name)
	if err != nil {
		return fmt.Errorf("environment %s not found: %w", name, err)
	}

	// Delete from etcd
	if err := uc.environmentRepo.DeleteEnvironment(ctx, name); err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}

	log.Printf("Environment %s deleted", name)

	// Delete xDS snapshot (optional, depends on your implementation)
	nodeID := fmt.Sprintf("envoy-%s", name)
	uc.xdsSnapshotManager.DeleteSnapshot(nodeID)

	return nil
}

func (uc *environmentsUseCase) rebuildXDSSnapshot(ctx context.Context, env *models.Environment) error {
	xdsSnapshot := snapshot.BuildEnvoySnapshot(env)
	nodeID := fmt.Sprintf("envoy-%s", env.Name)

	if err := uc.xdsSnapshotManager.UpdateSnapshot(nodeID, xdsSnapshot); err != nil {
		return fmt.Errorf("failed to update xDS snapshot: %w", err)
	}

	log.Printf("xDS snapshot rebuilt for environment: %s", env.Name)
	return nil
}
