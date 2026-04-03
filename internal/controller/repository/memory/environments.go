package memory

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
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
	schemaRepo         interfaces.SchemaRepository
}

func NewEnvironmentsRepository() interfaces.InMemoryEnvironmentsRepository {
	return &EnvironmentsRepository{
		fromFile: make(map[string]*models.Environment),
		fromK8s:  make(map[string]*models.Environment),
	}
}

func (r *EnvironmentsRepository) SetDependencies(xdsSnapshotManager *xdscache.SnapshotManager, xdsBuilder interfaces.XDSBuilder, schemaRepo interfaces.SchemaRepository) {
	r.xdsSnapshotManager = xdsSnapshotManager
	r.xdsBuilder = xdsBuilder
	r.schemaRepo = schemaRepo
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

	r.rebuildMergedXDSSnapshotsLocked(context.Background())
	return nil
}

func bundleDedupeKey(b models.StaticContractBundleConfig) string {
	return b.Repository + "\x00" + b.Ref + "\x00" + b.Path + "\x00" + b.Name
}

func unionStaticBundles(file, k8s []models.StaticContractBundleConfig) []models.StaticContractBundleConfig {
	byKey := make(map[string]models.StaticContractBundleConfig)
	for _, b := range file {
		byKey[bundleDedupeKey(b)] = b
	}
	for _, b := range k8s {
		byKey[bundleDedupeKey(b)] = b
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]models.StaticContractBundleConfig, 0, len(keys))
	for _, k := range keys {
		out = append(out, byKey[k])
	}
	return out
}

func unionStaticServices(file, k8s []models.StaticServiceConfig) []models.StaticServiceConfig {
	byName := make(map[string]models.StaticServiceConfig)
	for _, s := range file {
		byName[s.Name] = s
	}
	for _, s := range k8s {
		byName[s.Name] = s
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]models.StaticServiceConfig, 0, len(names))
	for _, n := range names {
		out = append(out, byName[n])
	}
	return out
}

// mergeEnvironment overlays Kubernetes-discovered bundles/services onto static config for the same logical environment.
func mergeEnvironment(file, k8s *models.Environment) *models.Environment {
	var fBundles, kBundles []models.StaticContractBundleConfig
	if file != nil && file.Bundles != nil {
		fBundles = file.Bundles.Static
	}
	if k8s != nil && k8s.Bundles != nil {
		kBundles = k8s.Bundles.Static
	}
	var fSvc, kSvc []models.StaticServiceConfig
	if file != nil && file.Services != nil {
		fSvc = file.Services.Static
	}
	if k8s != nil && k8s.Services != nil {
		kSvc = k8s.Services.Static
	}
	name := ""
	typ := "kubernetes"
	if k8s != nil {
		name = k8s.Name
		typ = k8s.Type
	} else if file != nil {
		name = file.Name
		typ = file.Type
	}
	return &models.Environment{
		Name: name,
		Type: typ,
		Bundles: &models.EnvironmentBundleConfig{
			Static: unionStaticBundles(fBundles, kBundles),
		},
		Services: &models.EnvironmentServiceConfig{
			Static: unionStaticServices(fSvc, kSvc),
		},
		Snapshots: nil,
	}
}

func (r *EnvironmentsRepository) mergedLocked() map[string]*models.Environment {
	out := make(map[string]*models.Environment)
	for k, v := range r.fromFile {
		out[k] = v
	}
	for k, k8s := range r.fromK8s {
		if fileEnv, ok := r.fromFile[k]; ok {
			out[k] = mergeEnvironment(fileEnv, k8s)
		} else {
			out[k] = k8s
		}
	}
	return out
}

func (r *EnvironmentsRepository) envWithSnapshotsFromEtcd(ctx context.Context, env *models.Environment) (*models.Environment, error) {
	if env == nil {
		return nil, nil
	}
	if r.schemaRepo == nil {
		return env, nil
	}
	out := *env
	out.Snapshots = nil
	if env.Bundles == nil {
		return &out, nil
	}
	for _, bundle := range env.Bundles.Static {
		snaps, err := r.schemaRepo.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref, bundle.Path)
		if err != nil {
			return nil, fmt.Errorf("list contract snapshots for bundle %q (repo %q ref %q path %q): %w", bundle.Name, bundle.Repository, bundle.Ref, bundle.Path, err)
		}
		out.Snapshots = append(out.Snapshots, snaps...)
	}
	return &out, nil
}

func (r *EnvironmentsRepository) rebuildMergedXDSSnapshotsLocked(ctx context.Context) {
	if r.xdsSnapshotManager == nil || r.xdsBuilder == nil {
		return
	}
	for envName, env := range r.mergedLocked() {
		buildEnv, err := r.envWithSnapshotsFromEtcd(ctx, env)
		if err != nil {
			slog.Error("xDS rebuild from memory: enrich snapshots from etcd", "environment", envName, "error", err)
			continue
		}
		snap := snapshot.BuildEnvoySnapshot(r.xdsBuilder, buildEnv)
		nodeID := fmt.Sprintf("envoy-%s", envName)
		if err := r.xdsSnapshotManager.UpdateSnapshot(nodeID, snap); err != nil {
			slog.Error("xDS snapshot update failed", "node_id", nodeID, "error", err)
			continue
		}
		slog.Info("updated xDS snapshot", "environment", envName, "node_id", nodeID)
	}
}

// ApplyKubernetesEnvironments replaces the Kubernetes-sourced environment map and rebuilds xDS.
func (r *EnvironmentsRepository) ApplyKubernetesEnvironments(ctx context.Context, envs map[string]*models.Environment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fromK8s = envs
	r.rebuildMergedXDSSnapshotsLocked(ctx)
	return nil
}

// GetEnvironment returns the merged view when the same name exists in static config and Kubernetes; otherwise the sole source.
func (r *EnvironmentsRepository) GetEnvironment(ctx context.Context, name string) (*models.Environment, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	fileEnv, fromFile := r.fromFile[name]
	k8sEnv, fromK8s := r.fromK8s[name]
	if fromFile && fromK8s {
		return mergeEnvironment(fileEnv, k8sEnv), nil
	}
	if fromK8s {
		return k8sEnv, nil
	}
	if fromFile {
		return fileEnv, nil
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
