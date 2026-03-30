package usecase

import (
	"context"
	"fmt"
	"log"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/repository/git"
	"merionyx/api-gateway/control-plane/internal/utils"
	xdscache "merionyx/api-gateway/control-plane/internal/xds/cache"
	"merionyx/api-gateway/control-plane/internal/xds/snapshot"
)

type environmentsUseCase struct {
	environmentRepo    interfaces.EnvironmentRepository
	schamasUseCase     interfaces.SchemasUseCase
	xdsSnapshotManager *xdscache.SnapshotManager
}

func NewEnvironmentsUseCase() interfaces.EnvironmentsUseCase {
	return &environmentsUseCase{}
}

func (uc *environmentsUseCase) SetDependencies(environmentRepo interfaces.EnvironmentRepository, schamasUseCase interfaces.SchemasUseCase, xdsSnapshotManager *xdscache.SnapshotManager) {
	uc.environmentRepo = environmentRepo
	uc.schamasUseCase = schamasUseCase
	uc.xdsSnapshotManager = xdsSnapshotManager
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
	env, err := uc.environmentRepo.GetEnvironment(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	for _, bundle := range env.Bundles.Static {
		snapshots, err := uc.schamasUseCase.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref)
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
			snapshots, err := uc.schamasUseCase.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref)
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
	xdsSnapshot := snapshot.BuildEnvoySnapshot(env)
	nodeID := fmt.Sprintf("envoy-%s", env.Name)

	if err := uc.xdsSnapshotManager.UpdateSnapshot(nodeID, xdsSnapshot); err != nil {
		return fmt.Errorf("failed to update xDS snapshot: %w", err)
	}

	log.Printf("xDS snapshot rebuilt for environment: %s", env.Name)
	return nil
}

func (uc *environmentsUseCase) WatchSnapshotsUpdates(ctx context.Context) error {
	watchChan := uc.schamasUseCase.WatchContractBundlesSnapshots(ctx)
	for watchResp := range watchChan {
		for _, event := range watchResp.Events {
			log.Printf("Event: %s, Key: %s, Value: %s\n", event.Type, event.Kv.Key, event.Kv.Value)
			repo, ref, contract := utils.ExtractRepoRefContractFromKey(event.Kv.Key)
			log.Printf("Repo: %s, Ref: %s, Contract: %s\n", repo, ref, contract)

			envs, err := uc.ListEnvironments(ctx)
			if err != nil {
				return fmt.Errorf("failed to list environments: %w", err)
			}
			for _, env := range envs {
				for _, bundle := range env.Bundles.Static {
					if bundle.Repository == repo && bundle.Ref == ref {
						log.Printf("Auto-rebuilding xDS snapshot for environment: %s\n", env.Name)
						if err := uc.rebuildXDSSnapshot(ctx, env); err != nil {
							log.Printf("Failed to rebuild xDS snapshot for environment: %s: %v", env.Name, err)
						}
					}
				}
			}
		}
	}
	return nil
}
