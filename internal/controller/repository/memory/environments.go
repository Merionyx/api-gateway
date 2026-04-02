package memory

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/domain/interfaces"
	"merionyx/api-gateway/internal/controller/domain/models"
	xdscache "merionyx/api-gateway/internal/controller/xds/cache"
	"merionyx/api-gateway/internal/controller/xds/snapshot"
)

type EnvironmentsRepository struct {
	mu sync.RWMutex

	fromFile map[string]*models.Environment
	fromK8s  map[string]*models.Environment

	xdsSnapshotManager *xdscache.SnapshotManager
	xdsBuilder         interfaces.XDSBuilder
}

func NewEnvironmentsRepository() interfaces.InMemoryEnvironmentsRepository {
	return &EnvironmentsRepository{
		fromFile: make(map[string]*models.Environment),
		fromK8s:  make(map[string]*models.Environment),
	}
}

func (r *EnvironmentsRepository) SetDependencies(xdsSnapshotManager *xdscache.SnapshotManager, xdsBuilder interfaces.XDSBuilder) {
	r.xdsSnapshotManager = xdsSnapshotManager
	r.xdsBuilder = xdsBuilder
}

func (r *EnvironmentsRepository) Initialize(config *config.Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.fromFile = make(map[string]*models.Environment)
	r.fromK8s = make(map[string]*models.Environment)

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
			Snapshots: make([]models.ContractSnapshot, 0),
		}
		r.fromFile[configEnv.Name] = env
	}

	for _, environment := range config.Environments {
		for _, bundle := range environment.Bundles.Static {
			r.fromFile[environment.Name].Bundles.Static = append(r.fromFile[environment.Name].Bundles.Static, models.StaticContractBundleConfig{
				Name:       bundle.Name,
				Repository: bundle.Repository,
				Ref:        bundle.Ref,
				Path:       bundle.Path,
			})
		}
	}

	for _, environment := range config.Environments {
		for _, service := range environment.Services.Static {
			r.fromFile[environment.Name].Services.Static = append(r.fromFile[environment.Name].Services.Static, models.StaticServiceConfig{
				Name:     service.Name,
				Upstream: service.Upstream,
			})
		}
	}

	for _, environment := range r.mergedLocked() {
		slog.Info("environment from config", "name", environment.Name, "bundles", len(environment.Bundles.Static), "services", len(environment.Services.Static))
	}

	return r.rebuildMergedXDSSnapshotsLocked()
}

func (r *EnvironmentsRepository) mergedLocked() map[string]*models.Environment {
	out := make(map[string]*models.Environment)
	for k, v := range r.fromFile {
		out[k] = v
	}
	for k, v := range r.fromK8s {
		out[k] = v
	}
	return out
}

func (r *EnvironmentsRepository) rebuildMergedXDSSnapshotsLocked() error {
	if r.xdsSnapshotManager == nil || r.xdsBuilder == nil {
		return nil
	}
	for envName, env := range r.mergedLocked() {
		snap := snapshot.BuildEnvoySnapshot(r.xdsBuilder, env)
		nodeID := fmt.Sprintf("envoy-%s", envName)
		if err := r.xdsSnapshotManager.UpdateSnapshot(nodeID, snap); err != nil {
			slog.Error("xDS snapshot update failed", "node_id", nodeID, "error", err)
			return err
		}
		slog.Info("updated xDS snapshot", "environment", envName, "node_id", nodeID)
	}
	return nil
}

// ApplyKubernetesEnvironments replaces the Kubernetes-sourced environment map and rebuilds xDS.
func (r *EnvironmentsRepository) ApplyKubernetesEnvironments(_ context.Context, envs map[string]*models.Environment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fromK8s = envs
	return r.rebuildMergedXDSSnapshotsLocked()
}

// GetEnvironment returns kubernetes overlay first, then static config.
func (r *EnvironmentsRepository) GetEnvironment(ctx context.Context, name string) (*models.Environment, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	if env, ok := r.fromK8s[name]; ok {
		return env, nil
	}
	if env, ok := r.fromFile[name]; ok {
		return env, nil
	}
	return nil, fmt.Errorf("environment %s not found in config", name)
}

// ListEnvironments merges static config with Kubernetes sources (Kubernetes wins on name conflict).
func (r *EnvironmentsRepository) ListEnvironments(ctx context.Context) (map[string]*models.Environment, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mergedLocked(), nil
}
