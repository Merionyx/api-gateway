package memory

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/envmodel"
	"github.com/merionyx/api-gateway/internal/controller/index/bundleenv"
	ctrlmetrics "github.com/merionyx/api-gateway/internal/controller/metrics"
	xdscache "github.com/merionyx/api-gateway/internal/controller/xds/cache"
	"github.com/merionyx/api-gateway/internal/controller/xds/snapshot"
	"github.com/merionyx/api-gateway/internal/shared/election"
)

type EnvironmentsRepository struct {
	mu sync.RWMutex

	fromFile map[string]*models.Environment
	fromK8s  map[string]*models.Environment

	xdsSnapshotManager *xdscache.SnapshotManager
	xdsBuilder         interfaces.XDSBuilder
	schemaRepo         interfaces.SchemaRepository
	bundleEnvIndex     *bundleenv.Index
	bundleEnvMetrics   bool

	// Effective reconcile (ADR 0001, phase 2); optional, wired from container.
	etcdEnv                  interfaces.EnvironmentRepository
	leader                   election.LeaderGate
	materialized             interfaces.MaterializedEffectiveStore
	materializedWriteEnabled bool
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

// SetBundleEnvIndex wires the bundle→environment index refreshed after Kubernetes or static config changes.
func (r *EnvironmentsRepository) SetBundleEnvIndex(idx *bundleenv.Index, metricsEnabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bundleEnvIndex = idx
	r.bundleEnvMetrics = metricsEnabled
}

// SetEffectiveReconcile wires 3-way merge (memory ∪ etcd) for xDS, optional materialized writes, and leader gate.
// Call after SetDependencies, when etcd EnvironmentRepository is available. Nil etcdEnv disables etcd merge in xDS.
func (r *EnvironmentsRepository) SetEffectiveReconcile(
	etcdEnv interfaces.EnvironmentRepository,
	leader election.LeaderGate,
	materialized interfaces.MaterializedEffectiveStore,
	writeEnabled bool,
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.etcdEnv = etcdEnv
	r.leader = leader
	r.materialized = materialized
	r.materializedWriteEnabled = writeEnabled
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

func (r *EnvironmentsRepository) listEtcdEnvironments(ctx context.Context) map[string]*models.Environment {
	if r.etcdEnv == nil {
		return nil
	}
	m, err := r.etcdEnv.ListEnvironments(ctx)
	if err != nil {
		slog.Error("xDS rebuild: list etcd environments", "error", err)
		return nil
	}
	return m
}

// unionEnvNames returns sorted names from memory-merged and optional etcd map.
func unionEnvNames(merged map[string]*models.Environment, etcd map[string]*models.Environment) []string {
	seen := make(map[string]struct{})
	for n := range merged {
		seen[n] = struct{}{}
	}
	for n := range etcd {
		seen[n] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
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

// rebuildMergedXDSSnapshots runs without holding r.mu. It merges the in-memory view
// (file ∪ k8s) with etcd gRPC environment config, same as API Server / sync use case.
func (r *EnvironmentsRepository) rebuildMergedXDSSnapshots(ctx context.Context) {
	if r.xdsSnapshotManager == nil || r.xdsBuilder == nil {
		return
	}
	r.mu.RLock()
	merged := r.mergedLocked()
	r.mu.RUnlock()

	etcdByName := r.listEtcdEnvironments(ctx)
	for _, envName := range unionEnvNames(merged, etcdByName) {
		var mem, etcdEnv *models.Environment
		if merged != nil {
			mem = merged[envName]
		}
		if etcdByName != nil {
			etcdEnv = etcdByName[envName]
		}
		eff, err := envmodel.BuildOptionalEffectiveEnvironment(mem, etcdEnv)
		if err != nil {
			continue
		}
		buildEnv, err := r.envWithSnapshotsFromEtcd(ctx, eff)
		if err != nil {
			slog.Error("xDS rebuild from memory: enrich snapshots from etcd", "environment", envName, "error", err)
			continue
		}
		envoySnap, err := snapshot.BuildEnvoySnapshot(r.xdsBuilder, buildEnv)
		if err != nil {
			slog.Error("xDS rebuild from memory: build envoy snapshot", "environment", envName, "error", err)
			continue
		}
		nodeID := fmt.Sprintf("envoy-%s", envName)
		if err := r.xdsSnapshotManager.UpdateSnapshot(nodeID, envoySnap); err != nil {
			slog.Error("xDS snapshot update failed", "node_id", nodeID, "error", err)
			continue
		}
		slog.Info("updated xDS snapshot", "environment", envName, "node_id", nodeID, "reconcile", "memory_etcd_merged")
		if r.shouldWriteMaterialized() {
			if err := r.materialized.ReconcileIfChanged(ctx, eff); err != nil {
				slog.Error("materialized effective write failed", "environment", envName, "error", err)
			} else {
				slog.Info("materialized effective reconciled", "environment", envName)
			}
		}
	}
}

func (r *EnvironmentsRepository) shouldWriteMaterialized() bool {
	if !r.materializedWriteEnabled || r.materialized == nil {
		return false
	}
	if r.leader == nil {
		return false
	}
	return r.leader.IsLeader()
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
