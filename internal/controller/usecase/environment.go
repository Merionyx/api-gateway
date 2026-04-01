package usecase

import (
	"context"
	"fmt"
	"log"

	"merionyx/api-gateway/internal/controller/domain/interfaces"
	"merionyx/api-gateway/internal/controller/domain/models"
	xdscache "merionyx/api-gateway/internal/controller/xds/cache"
	"merionyx/api-gateway/internal/controller/xds/snapshot"
)

type environmentsUseCase struct {
	environmentRepo    interfaces.EnvironmentRepository
	schamasUseCase     interfaces.SchemasUseCase
	xdsSnapshotManager *xdscache.SnapshotManager
	xdsBuilder         interfaces.XDSBuilder
}

func NewEnvironmentsUseCase() interfaces.EnvironmentsUseCase {
	return &environmentsUseCase{}
}

func (uc *environmentsUseCase) SetDependencies(environmentRepo interfaces.EnvironmentRepository, schamasUseCase interfaces.SchemasUseCase, xdsSnapshotManager *xdscache.SnapshotManager, xdsBuilder interfaces.XDSBuilder) {
	uc.environmentRepo = environmentRepo
	uc.schamasUseCase = schamasUseCase
	uc.xdsSnapshotManager = xdsSnapshotManager
	uc.xdsBuilder = xdsBuilder
}

func (uc *environmentsUseCase) CreateEnvironment(ctx context.Context, req *models.CreateEnvironmentRequest) (*models.Environment, error) {
	// Check if environment does not exist
	existing, _ := uc.environmentRepo.GetEnvironment(ctx, req.Name)
	if existing != nil {
		return nil, fmt.Errorf("environment %s already exists", req.Name)
	}

	env := &models.Environment{
		Name:      req.Name,
		Type:      req.Type,
		Bundles:   req.Bundles,
		Services:  req.Services,
		Snapshots: make([]models.ContractSnapshot, 0),
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
	env, err := uc.environmentRepo.GetEnvironment(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	for _, bundle := range env.Bundles.Static {
		snapshots, err := uc.schamasUseCase.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref, bundle.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to get environment snapshots: %w", err)
		}
		env.Snapshots = append(env.Snapshots, snapshots...)
	}

	return env, nil
}

func (uc *environmentsUseCase) ListEnvironments(ctx context.Context) (map[string]*models.Environment, error) {
	environments, err := uc.environmentRepo.ListEnvironments(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}
	for _, environment := range environments {
		for _, bundle := range environment.Bundles.Static {
			snapshots, err := uc.schamasUseCase.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref, bundle.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to get environment snapshots: %w", err)
			}
			environment.Snapshots = append(environment.Snapshots, snapshots...)
		}
		environments[environment.Name] = environment
	}
	return environments, nil
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
	xdsSnapshot := snapshot.BuildEnvoySnapshot(uc.xdsBuilder, env)
	nodeID := fmt.Sprintf("envoy-%s", env.Name)

	if err := uc.xdsSnapshotManager.UpdateSnapshot(nodeID, xdsSnapshot); err != nil {
		return fmt.Errorf("failed to update xDS snapshot: %w", err)
	}

	log.Printf("xDS snapshot rebuilt for environment: %s", env.Name)
	return nil
}
