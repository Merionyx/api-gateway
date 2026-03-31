package memory

import (
	"context"
	"fmt"
	"log"
	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/domain/interfaces"
	"merionyx/api-gateway/internal/controller/domain/models"
	"merionyx/api-gateway/internal/controller/repository/git"
	xdscache "merionyx/api-gateway/internal/controller/xds/cache"
	"merionyx/api-gateway/internal/controller/xds/snapshot"
)

type EnvironmentsRepository struct {
	environments         map[string]*models.Environment
	xdsSnapshotManager   *xdscache.SnapshotManager
	xdsBuilder           interfaces.XDSBuilder
	gitRepositoryManager *git.RepositoryManager
}

func NewEnvironmentsRepository() interfaces.InMemoryEnvironmentsRepository {
	return &EnvironmentsRepository{
		environments:         make(map[string]*models.Environment),
		xdsSnapshotManager:   nil,
		xdsBuilder:           nil,
		gitRepositoryManager: nil,
	}
}

func (r *EnvironmentsRepository) SetDependencies(xdsSnapshotManager *xdscache.SnapshotManager, xdsBuilder interfaces.XDSBuilder, gitRepositoryManager *git.RepositoryManager) {
	r.xdsSnapshotManager = xdsSnapshotManager
	r.xdsBuilder = xdsBuilder
	r.gitRepositoryManager = gitRepositoryManager
}

func (r *EnvironmentsRepository) Initialize(config *config.Config) error {
	// Initialize environments from config
	for _, configEnv := range config.Environments {
		env := &models.Environment{
			Name: configEnv.Name,
			Type: "static",
			Bundles: &models.EnvironmentBundleConfig{
				Static: make([]models.StaticContractBundleConfig, 0),
			},
			Services: &models.EnvironmentServiceConfig{
				Static: make([]models.StaticServiceConfig, 0),
			},
			Snapshots: make([]git.ContractSnapshot, 0),
		}
		r.environments[configEnv.Name] = env
	}

	// Initialize contracts from config
	for _, environment := range config.Environments {
		for _, bundle := range environment.Bundles.Static {
			r.environments[environment.Name].Bundles.Static = append(r.environments[environment.Name].Bundles.Static, models.StaticContractBundleConfig{
				Name:       bundle.Name,
				Repository: bundle.Repository,
				Ref:        bundle.Ref,
				Path:       bundle.Path,
			})
		}
	}

	// Initialize services from config
	for _, environment := range config.Environments {
		for _, service := range environment.Services.Static {
			r.environments[environment.Name].Services.Static = append(r.environments[environment.Name].Services.Static, models.StaticServiceConfig{
				Name:     service.Name,
				Upstream: service.Upstream,
			})
		}
	}

	for _, environment := range r.environments {
		for _, bundle := range environment.Bundles.Static {
			snapshots, err := r.gitRepositoryManager.GetRepositorySnapshots(bundle.Repository, bundle.Ref, bundle.Path)
			if err != nil {
				log.Fatalf("Failed to get repository snapshots: %v", err)
			}
			for _, snapshot := range snapshots {
				environment.Snapshots = append(environment.Snapshots, snapshot)
			}
		}
	}

	for _, environment := range r.environments {
		log.Println("Environment:", environment.Name)
		log.Println("Bundles:", environment.Bundles.Static)
		log.Println("Services:", environment.Services.Static)
		log.Println("Snapshots:", environment.Snapshots)
	}

	for envName, env := range r.environments {
		snapshot := snapshot.BuildEnvoySnapshot(r.xdsBuilder, env)
		nodeID := fmt.Sprintf("envoy-%s", envName)

		if err := r.xdsSnapshotManager.UpdateSnapshot(nodeID, snapshot); err != nil {
			log.Fatalf("Failed to update snapshot for %s: %v", nodeID, err)
		}

		log.Printf("Created xDS snapshot for environment: %s (nodeID: %s)", envName, nodeID)
	}

	return nil
}

// GetEnvironment gets the environment by name from the in-memory storage
func (r *EnvironmentsRepository) GetEnvironment(ctx context.Context, name string) (*models.Environment, error) {
	env, exists := r.environments[name]
	if !exists {
		return nil, fmt.Errorf("environment %s not found in config", name)
	}
	return env, nil
}

// ListEnvironments returns all environments from the in-memory storage
func (r *EnvironmentsRepository) ListEnvironments(ctx context.Context) (map[string]*models.Environment, error) {
	result := make(map[string]*models.Environment)
	for name, env := range r.environments {
		result[name] = env
	}
	return result, nil
}
