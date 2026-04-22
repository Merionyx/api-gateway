package reconcile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/effective"
	xdscache "github.com/merionyx/api-gateway/internal/controller/xds/cache"
	"github.com/merionyx/api-gateway/internal/controller/xds/snapshot"
	"github.com/merionyx/api-gateway/internal/shared/election"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// spanReconcilePkg is the package path for [telemetry.SpanName] in the reconciler.
const spanReconcilePkg = "internal/controller/reconcile"

// ReconcilerDeps are dependencies for the effective reconciler. Nil etcd is allowed (in-memory xDS only).
type ReconcilerDeps struct {
	Etcd                     interfaces.EnvironmentRepository
	InMemory                 interfaces.InMemoryEnvironmentsRepository
	Schema                   interfaces.SchemaRepository
	XDSM                     *xdscache.SnapshotManager
	XDSB                     interfaces.XDSBuilder
	Materialized             interfaces.MaterializedEffectiveStore
	Leader                   election.LeaderGate
	MaterializedWriteEnabled bool
}

// Reconciler implements ADR 0001 effective environment reconciliation.
type Reconciler struct {
	etcd   interfaces.EnvironmentRepository
	inMem  interfaces.InMemoryEnvironmentsRepository
	schema interfaces.SchemaRepository
	xm     *xdscache.SnapshotManager
	xb     interfaces.XDSBuilder
	mat    interfaces.MaterializedEffectiveStore
	// matWrite is the single policy for materialized etcd keys; see [MaterializedWritePolicy].
	matWrite MaterializedWritePolicy
}

// New builds a reconciler. Omitted/ nil leader defaults to "no materialized writes" when checked.
func New(d ReconcilerDeps) *Reconciler {
	return &Reconciler{
		etcd:     d.Etcd,
		inMem:    d.InMemory,
		schema:   d.Schema,
		xm:       d.XDSM,
		xb:       d.XDSB,
		mat:      d.Materialized,
		matWrite: NewMaterializedWritePolicy(d.MaterializedWriteEnabled, d.Materialized, d.Leader),
	}
}

// RebuildAllFromMemory enqueues xDS and optional materialized for the union of names in memory-merged
// and etcd, same policy as the former EnvironmentsRepository.rebuildMergedXDSSnapshots.
func (r *Reconciler) RebuildAllFromMemory(ctx context.Context, memoryMergedByName map[string]*models.Environment) {
	if r.xm == nil || r.xb == nil {
		return
	}
	etcdByName := r.listEtcdEnvironments(ctx)
	for _, envName := range unionEnvNames(memoryMergedByName, etcdByName) {
		var mem, etcdE *models.Environment
		if memoryMergedByName != nil {
			mem = memoryMergedByName[envName]
		}
		if etcdByName != nil {
			etcdE = etcdByName[envName]
		}
		eff, err := effective.MergeMemoryAndControllerEtcd(mem, etcdE)
		if err != nil {
			continue
		}
		if err := r.reconcileOneBuilt(ctx, envName, eff, r.matWrite.Allow()); err != nil {
			slog.Error("effective reconciler: rebuild all", "environment", envName, "error", err)
		}
	}
}

func (r *Reconciler) listEtcdEnvironments(ctx context.Context) map[string]*models.Environment {
	if r.etcd == nil {
		return nil
	}
	m, err := r.etcd.ListEnvironments(ctx)
	if err != nil {
		slog.Error("effective reconciler: list etcd environments", "error", err)
		return nil
	}
	return m
}

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

func (r *Reconciler) envWithSnapshotsFromSchema(ctx context.Context, env *models.Environment) (*models.Environment, error) {
	if env == nil {
		return nil, nil
	}
	if r.schema == nil {
		return env, nil
	}
	out := *env
	out.Snapshots = nil
	if env.Bundles == nil {
		return &out, nil
	}
	for _, b := range env.Bundles.Static {
		snaps, err := r.schema.ListContractSnapshots(ctx, b.Repository, b.Ref, b.Path)
		if err != nil {
			return nil, fmt.Errorf("list contract snapshots for bundle %q: %w", b.Name, err)
		}
		out.Snapshots = append(out.Snapshots, snaps...)
	}
	return &out, nil
}

// ReconcileOne refreshes a single name (same as APIServerSync environmentForXDS, plus materialized if requested).
func (r *Reconciler) ReconcileOne(ctx context.Context, name string, writeMaterialized bool) error {
	if r.xm == nil || r.xb == nil {
		return nil
	}
	var mem *models.Environment
	if r.inMem != nil {
		if m, err := r.inMem.GetEnvironment(ctx, name); err == nil {
			mem = m
		}
	}
	var etcdE *models.Environment
	if r.etcd != nil {
		if e, err := r.etcd.GetEnvironment(ctx, name); err == nil {
			etcdE = e
		}
	}
	eff, err := effective.MergeMemoryAndControllerEtcd(mem, etcdE)
	if err != nil {
		if errors.Is(err, effective.ErrNotFound) {
			return r.removeOne(ctx, name, writeMaterialized)
		}
		return err
	}
	writeMat := writeMaterialized && r.matWrite.Allow()
	if err := r.reconcileOneBuilt(ctx, name, eff, writeMat); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) reconcileOneBuilt(ctx context.Context, name string, eff *models.Environment, writeMat bool) error {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanReconcilePkg, "reconcileOneBuilt"))
	defer span.End()
	buildEnv, err := r.envWithSnapshotsFromSchema(ctx, eff)
	if err != nil {
		telemetry.MarkError(span, err)
		return fmt.Errorf("enrich with snapshots: %w", err)
	}
	if buildEnv == nil {
		return nil
	}
	envoySnap, err := snapshot.BuildEnvoySnapshot(r.xb, buildEnv)
	if err != nil {
		telemetry.MarkError(span, err)
		return fmt.Errorf("build envoy snapshot: %w", err)
	}
	nodeID := fmt.Sprintf("envoy-%s", name)
	if err := r.xm.UpdateSnapshot(nodeID, envoySnap); err != nil {
		telemetry.MarkError(span, err)
		return fmt.Errorf("update xDS snapshot: %w", err)
	}
	slog.Info("updated xDS snapshot", "environment", name, "node_id", nodeID, "reconcile", "memory_etcd_merged")
	if writeMat {
		if err := r.mat.ReconcileIfChanged(ctx, eff); err != nil {
			slog.Error("materialized effective write failed", "environment", name, "error", err)
		} else {
			slog.Info("materialized effective reconciled", "environment", name)
		}
	}
	return nil
}

func (r *Reconciler) removeOne(ctx context.Context, name string, requestMaterialized bool) error {
	nodeID := fmt.Sprintf("envoy-%s", name)
	if err := r.xm.DeleteSnapshot(nodeID); err != nil {
		return fmt.Errorf("delete xDS snapshot: %w", err)
	}
	if !requestMaterialized {
		return nil
	}
	if r.matWrite.Allow() {
		if err := r.mat.Delete(ctx, name); err != nil {
			return fmt.Errorf("delete materialized: %w", err)
		}
		slog.Info("materialized effective deleted", "environment", name)
	}
	return nil
}
