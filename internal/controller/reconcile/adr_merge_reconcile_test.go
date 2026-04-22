package reconcile

import (
	"context"
	"errors"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/effective"
	"github.com/merionyx/api-gateway/internal/controller/xds/cache"
	"github.com/merionyx/api-gateway/internal/controller/xds/snapshot"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// ADR-0001 сценарии: in-memory + controller etcd → [effective.MergeMemoryAndControllerEtcd] → Reconciler
// (xDS; materialized при leader+флаге). Документация в коде для п.8 / п.1 — см. [effective.doc] в пакете effective.
//
// Сценарии намеренно узкие: фиксируем merge+reconcile контракт, а не полный production wiring.

var errTestNotFound = errors.New("not found")

type testInMem struct {
	byName map[string]*models.Environment
}

func (m *testInMem) SetDependencies(*cache.SnapshotManager, interfaces.XDSBuilder, interfaces.SchemaRepository) {
}
func (*testInMem) Initialize(*config.Config) error { return nil }
func (m *testInMem) GetEnvironment(_ context.Context, name string) (*models.Environment, error) {
	if m == nil || m.byName == nil {
		return nil, errTestNotFound
	}
	e, ok := m.byName[name]
	if !ok {
		return nil, errTestNotFound
	}
	return e, nil
}
func (m *testInMem) ListEnvironments(_ context.Context) (map[string]*models.Environment, error) {
	if m == nil || m.byName == nil {
		return nil, nil
	}
	out := make(map[string]*models.Environment, len(m.byName))
	for k, v := range m.byName {
		out[k] = v
	}
	return out, nil
}
func (*testInMem) ApplyKubernetesEnvironments(context.Context, map[string]*models.Environment) error {
	return nil
}

var _ interfaces.InMemoryEnvironmentsRepository = (*testInMem)(nil)

type testEtcd struct {
	byName map[string]*models.Environment
}

func (e *testEtcd) GetEnvironment(_ context.Context, name string) (*models.Environment, error) {
	if e == nil || e.byName == nil {
		return nil, errTestNotFound
	}
	v, ok := e.byName[name]
	if !ok {
		return nil, errTestNotFound
	}
	return v, nil
}
func (e *testEtcd) ListEnvironments(_ context.Context) (map[string]*models.Environment, error) {
	if e == nil || e.byName == nil {
		return nil, nil
	}
	out := make(map[string]*models.Environment, len(e.byName))
	for k, v := range e.byName {
		out[k] = v
	}
	return out, nil
}
func (*testEtcd) SaveEnvironment(context.Context, *models.Environment) error   { return nil }
func (*testEtcd) DeleteEnvironment(context.Context, string) error              { return nil }
func (*testEtcd) WatchEnvironments(context.Context) clientv3.WatchChan         { return nil }

var _ interfaces.EnvironmentRepository = (*testEtcd)(nil)

// stubXDSBuilder matches [snapshot] tests: empty xDS resources, enough for [snapshot.BuildEnvoySnapshot].
type stubXDSBuilder struct{}

func (stubXDSBuilder) BuildListeners(*models.Environment) ([]*listenerv3.Listener, error) {
	return nil, nil
}
func (stubXDSBuilder) BuildClusters(*models.Environment) ([]*clusterv3.Cluster, error) { return nil, nil }
func (stubXDSBuilder) BuildRoutes(*models.Environment) ([]*routev3.RouteConfiguration, error) {
	return nil, nil
}
func (stubXDSBuilder) BuildEndpoints(*models.Environment) ([]*endpointv3.ClusterLoadAssignment, error) {
	return nil, nil
}

type staticLeader struct{ ok bool }

func (s staticLeader) IsLeader() bool { return s.ok }
func (staticLeader) LeaderChanged() <-chan struct{} {
	ch := make(chan struct{})
	return ch
}

type recordMaterialized struct {
	reconciled []string
	deleted    []string
}

func (r *recordMaterialized) ReconcileIfChanged(_ context.Context, skel *models.Environment) error {
	if skel == nil {
		return nil
	}
	r.reconciled = append(r.reconciled, skel.Name)
	return nil
}
func (r *recordMaterialized) Delete(_ context.Context, name string) error {
	r.deleted = append(r.deleted, name)
	return nil
}

var _ interfaces.MaterializedEffectiveStore = (*recordMaterialized)(nil)

func nodeIDForEnv(name string) string { return "envoy-" + name }

// TestReconcileOne_ADR0001_MergeScenarios: таблица путей mem∪etcd → ожидаемый merge и эффект на xDS.
func TestReconcileOne_ADR0001_MergeScenarios(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cases := []struct {
		name         string
		mem          *testInMem
		etcd         *testEtcd
		envName      string
		wantMergeErr bool
		// wantXDS: после ReconcileOne(follower, writeMaterialized false) снапшот для node присутствует
		wantXDS      bool
		writeMat     bool
		wantMatRecon int // len(reconciled) if leader+mat
		wantMatDel   int
		leader       bool
	}{
		{
			name:    "mem_only_etcd_empty",
			mem:     &testInMem{byName: map[string]*models.Environment{"dev": {Name: "dev", Type: "static"}}},
			etcd:    &testEtcd{byName: map[string]*models.Environment{}},
			envName: "dev",
			wantXDS: true,
		},
		{
			name:    "etcd_only_mem_empty",
			mem:     &testInMem{byName: map[string]*models.Environment{}},
			etcd:    &testEtcd{byName: map[string]*models.Environment{"stg": {Name: "stg", Type: "kubernetes"}}},
			envName: "stg",
			wantXDS: true,
		},
		{
			name: "mem_and_etcd_merged",
			mem: &testInMem{byName: map[string]*models.Environment{
				"prod": {Name: "prod", Type: "static"},
			}},
			etcd: &testEtcd{byName: map[string]*models.Environment{
				"prod": {
					Name: "prod",
					Services: &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{
						{Name: "s1", Upstream: "u1"},
					}},
				},
			}},
			envName: "prod",
			wantXDS: true,
		},
		{
			name:         "neither_remove_xds",
			mem:          &testInMem{byName: map[string]*models.Environment{}},
			etcd:         &testEtcd{byName: map[string]*models.Environment{}},
			envName:      "ghost",
			wantMergeErr: true, // merge(nil,nil) in ReconcileOne path: both get fail -> nil,nil -> not found
			wantXDS:      false,
		},
		{
			name:    "materialized_only_when_leader_writeMat",
			mem:     &testInMem{byName: map[string]*models.Environment{"a": {Name: "a"}}},
			etcd:    &testEtcd{byName: nil},
			envName: "a",
			wantXDS: true, leader: true, writeMat: true, wantMatRecon: 1, wantMatDel: 0,
		},
		{
			name:    "no_materialized_when_not_leader",
			mem:     &testInMem{byName: map[string]*models.Environment{"b": {Name: "b"}}},
			etcd:    &testEtcd{byName: nil},
			envName: "b",
			wantXDS: true, leader: false, writeMat: true, wantMatRecon: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var memEnv, etcdEnv *models.Environment
			if tc.mem != nil {
				memEnv, _ = tc.mem.GetEnvironment(ctx, tc.envName)
			}
			if tc.etcd != nil {
				etcdEnv, _ = tc.etcd.GetEnvironment(ctx, tc.envName)
			}
			merged, merr := effective.MergeMemoryAndControllerEtcd(memEnv, etcdEnv)
			if tc.wantMergeErr {
				if merr == nil || !errors.Is(merr, effective.ErrNotFound) {
					t.Fatalf("merge: want ErrNotFound, got %v, %v", merged, merr)
				}
			} else {
				if merr != nil {
					t.Fatalf("merge: %v", merr)
				}
				if merged == nil || merged.Name != tc.envName {
					t.Fatalf("merge: bad env: %#v", merged)
				}
			}

			xm := cache.NewSnapshotManager(false)
			mat := &recordMaterialized{}
			rec := New(ReconcilerDeps{
				Etcd:                     tc.etcd,
				InMemory:                 tc.mem,
				Schema:                   nil,
				XDSM:                     xm,
				XDSB:                     stubXDSBuilder{},
				Materialized:             mat,
				Leader:                   staticLeader{ok: tc.leader},
				MaterializedWriteEnabled: true,
			})
			rerr := rec.ReconcileOne(ctx, tc.envName, tc.writeMat)
			if rerr != nil {
				t.Fatalf("ReconcileOne: %v", rerr)
			}

			_, gerr := xm.GetSnapshot(nodeIDForEnv(tc.envName))
			if tc.wantXDS {
				if gerr != nil {
					t.Fatalf("expected xDS snapshot, got %v", gerr)
				}
			} else {
				if gerr == nil {
					t.Fatal("expected no xDS snapshot")
				}
			}
			if len(mat.reconciled) != tc.wantMatRecon {
				t.Fatalf("mat reconciled: got %v want %d", mat.reconciled, tc.wantMatRecon)
			}
			if len(mat.deleted) != tc.wantMatDel {
				t.Fatalf("mat deleted: got %v", mat.deleted)
			}
			if tc.name == "mem_and_etcd_merged" {
				if merged == nil || merged.Services == nil || len(merged.Services.Static) != 1 {
					t.Fatalf("expected merged static service from etcd, got %#v", merged)
				}
			}
		})
	}
}

// TestSnapshotBuild_UsesSameBuilderAsReconciler — регресс: пустой stub + env == то, что отдаёт merge.
func TestSnapshotBuild_UsesSameBuilderAsReconciler(t *testing.T) {
	t.Parallel()
	merged, err := effective.MergeMemoryAndControllerEtcd(
		&models.Environment{Name: "e", Type: "static"},
		&models.Environment{Name: "e", Services: &models.EnvironmentServiceConfig{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = snapshot.BuildEnvoySnapshot(stubXDSBuilder{}, merged)
	if err != nil {
		t.Fatal(err)
	}
}
