package memory

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/envmodel"
	"github.com/merionyx/api-gateway/internal/controller/index/bundleenv"
	ctrlmetrics "github.com/merionyx/api-gateway/internal/controller/metrics"
	xdscache "github.com/merionyx/api-gateway/internal/controller/xds/cache"
)

type EnvironmentsRepository struct {
	mu sync.RWMutex

	fromFile map[string]*models.Environment
	fromK8s  map[string]*models.Environment

	xdsSnapshotManager *xdscache.SnapshotManager
	bundleEnvIndex     *bundleenv.Index
	bundleEnvMetrics   bool

	// Effective (ADR 0001): optional; wired from container after xDS + schema + reconciler.
	eff interfaces.EffectiveReconciler
}

func NewEnvironmentsRepository() interfaces.InMemoryEnvironmentsRepository {
	return &EnvironmentsRepository{
		fromFile: make(map[string]*models.Environment),
		fromK8s:  make(map[string]*models.Environment),
	}
}

func (r *EnvironmentsRepository) SetDependencies(xdsSnapshotManager *xdscache.SnapshotManager, _ interfaces.XDSBuilder, _ interfaces.SchemaRepository) {
	r.xdsSnapshotManager = xdsSnapshotManager
}

// SetEnvironmentReconciler wires 3-way merge (memory ∪ etcd) xDS and optional materialized. Call
// after SetDependencies and before Initialize when etcd and leader are available. May be nil (tests).
func (r *EnvironmentsRepository) SetEnvironmentReconciler(eff interfaces.EffectiveReconciler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eff = eff
}

// SetBundleEnvIndex wires the bundle→environment index refreshed after Kubernetes or static config changes.
func (r *EnvironmentsRepository) SetBundleEnvIndex(idx *bundleenv.Index, metricsEnabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bundleEnvIndex = idx
	r.bundleEnvMetrics = metricsEnabled
}

func (r *EnvironmentsRepository) scheduleBundleEnvIndexRebuildLocked() {
	idx := r.bundleEnvIndex
	metrics := r.bundleEnvMetrics
	if idx == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		idx.Rebuild(ctx)
		ctrlmetrics.RecordBundleEnvIndexRebuild(metrics)
	}()
}

func (r *EnvironmentsRepository) Initialize(config *config.Config) error {
	r.mu.Lock()

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

	r.mu.Unlock()
	r.rebuildMergedXDSSnapshots(context.Background())
	r.mu.Lock()
	r.scheduleBundleEnvIndexRebuildLocked()
	r.mu.Unlock()
	return nil
}

// UnionStaticContractBundles returns the union of two bundle lists keyed by repository, ref, path, and name.
func UnionStaticContractBundles(a, b []models.StaticContractBundleConfig) []models.StaticContractBundleConfig {
	return envmodel.UnionStaticBundles(a, b)
}

// UnionStaticServices returns the union of two service lists by name (later entries override same name).
func UnionStaticServices(a, b []models.StaticServiceConfig) []models.StaticServiceConfig {
	return envmodel.UnionStaticServices(a, b)
}

// mergedLocked returns the in-memory (file ∪ K8s) view for each name. The repository only stores
// fromFile and fromK8s; this is never a cached "effective" layer, but a fresh envmodel result per call.
// Single-source entries are ToAPIServerSkeleton copies so the returned map never aliases storage.
func (r *EnvironmentsRepository) mergedLocked() map[string]*models.Environment {
	keys := make(map[string]struct{})
	for k := range r.fromFile {
		keys[k] = struct{}{}
	}
	for k := range r.fromK8s {
		keys[k] = struct{}{}
	}
	out := make(map[string]*models.Environment, len(keys))
	for k := range keys {
		var f, k8s *models.Environment
		if e, ok := r.fromFile[k]; ok {
			f = e
		}
		if e, ok := r.fromK8s[k]; ok {
			k8s = e
		}
		out[k] = envmodel.InMemoryEffective(f, k8s)
	}
	return out
}

// rebuildMergedXDSSnapshots runs without holding r.mu. Delegates to the effective reconciler
// (file ∪ k8s ∪ controller etcd) → xDS + optional materialized.
func (r *EnvironmentsRepository) rebuildMergedXDSSnapshots(ctx context.Context) {
	if r.eff == nil {
		return
	}
	if r.xdsSnapshotManager == nil {
		return
	}
	r.mu.RLock()
	merged := r.mergedLocked()
	r.mu.RUnlock()
	r.eff.RebuildAllFromMemory(ctx, merged)
}

// ApplyKubernetesEnvironments replaces the Kubernetes-sourced environment map and rebuilds xDS.
func (r *EnvironmentsRepository) ApplyKubernetesEnvironments(ctx context.Context, envs map[string]*models.Environment) error {
	r.mu.Lock()
	r.fromK8s = envs
	r.mu.Unlock()
	r.rebuildMergedXDSSnapshots(ctx)
	r.mu.Lock()
	r.scheduleBundleEnvIndexRebuildLocked()
	r.mu.Unlock()
	return nil
}

// GetEnvironment returns the in-memory (file ∪ K8s) view. Storage holds only the two partials;
// this result is envmodel-computed, with single-source values as skeleton copies (no pointer alias into fromFile/fromK8s).
func (r *EnvironmentsRepository) GetEnvironment(ctx context.Context, name string) (*models.Environment, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	fileEnv, fromFile := r.fromFile[name]
	k8sEnv, fromK8s := r.fromK8s[name]
	if !fromFile && !fromK8s {
		return nil, fmt.Errorf("environment %s not found in config", name)
	}
	return envmodel.InMemoryEffective(fileEnv, k8sEnv), nil
}

// ListEnvironments returns the file ∪ K8s effective map (computed; not a third stored snapshot).
func (r *EnvironmentsRepository) ListEnvironments(ctx context.Context) (map[string]*models.Environment, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mergedLocked(), nil
}

// FileAndK8sStaticBundles returns unmerged static bundle lists for provenance (ADR 0001, phase 3).
func (r *EnvironmentsRepository) FileAndK8sStaticBundles(_ context.Context, name string) (file, k8s []models.StaticContractBundleConfig) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if e, ok := r.fromFile[name]; ok && e != nil && e.Bundles != nil {
		file = e.Bundles.Static
	}
	if e, ok := r.fromK8s[name]; ok && e != nil && e.Bundles != nil {
		k8s = e.Bundles.Static
	}
	return file, k8s
}
