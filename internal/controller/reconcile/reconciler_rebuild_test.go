package reconcile

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/xds/cache"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestUnionEnvNames(t *testing.T) {
	t.Parallel()
	m1 := map[string]*models.Environment{"b": {Name: "b"}, "a": {Name: "a"}}
	m2 := map[string]*models.Environment{"c": {Name: "c"}}
	got := unionEnvNames(m1, m2)
	want := []string{"a", "b", "c"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if g := unionEnvNames(nil, nil); len(g) != 0 {
		t.Fatalf("empty union: %v", g)
	}
}

// testEtcdListError — ListEnvironments fails (exercises [Reconciler.listEtcdEnvironments] error path).
type testEtcdListError struct{}

func (testEtcdListError) GetEnvironment(context.Context, string) (*models.Environment, error) {
	return nil, errTestNotFound
}
func (testEtcdListError) ListEnvironments(context.Context) (map[string]*models.Environment, error) {
	return nil, errors.New("etcd list down")
}
func (testEtcdListError) SaveEnvironment(context.Context, *models.Environment) error { return nil }
func (testEtcdListError) DeleteEnvironment(context.Context, string) error            { return nil }
func (testEtcdListError) WatchEnvironments(context.Context) clientv3.WatchChan       { return nil }

var _ interfaces.EnvironmentRepository = testEtcdListError{}

func TestReconciler_listEtcdEnvironments_listError(t *testing.T) {
	t.Parallel()
	r := &Reconciler{etcd: testEtcdListError{}}
	if m := r.listEtcdEnvironments(context.Background()); m != nil {
		t.Fatalf("want nil on list error, got %v", m)
	}
}

func TestReconciler_listEtcdEnvironments_nilEtcd(t *testing.T) {
	t.Parallel()
	r := &Reconciler{etcd: nil}
	if m := r.listEtcdEnvironments(context.Background()); m != nil {
		t.Fatalf("want nil, got %v", m)
	}
}

// TestRebuildAllFromMemory_unionsInMemAndEtcd: one name only in memory, one only in etcd — both get xDS.
func TestRebuildAllFromMemory_unionsInMemAndEtcd(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	xm := cache.NewSnapshotManager(false)
	mem := map[string]*models.Environment{
		"only-mem": {Name: "only-mem", Type: "static"},
	}
	etcdData := &testEtcd{byName: map[string]*models.Environment{
		"only-etcd": {Name: "only-etcd", Type: "kubernetes"},
	}}
	rec := New(ReconcilerDeps{
		Etcd:         etcdData,
		InMemory:     nil,
		Schema:       nil,
		XDSM:         xm,
		XDSB:         stubXDSBuilder{},
		Materialized: nil,
		Leader:       staticLeader{ok: false},
	})
	rec.RebuildAllFromMemory(ctx, mem)
	for _, name := range []string{"only-etcd", "only-mem"} {
		if _, err := xm.GetSnapshot(nodeIDForEnv(name)); err != nil {
			t.Fatalf("missing snapshot for %q: %v", name, err)
		}
	}
}
