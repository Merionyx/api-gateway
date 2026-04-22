package effective

import (
	"errors"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/envmodel"
)

func TestMergeMemoryAndControllerEtcd_matchesEnvmodel(t *testing.T) {
	t.Parallel()
	got, err := MergeMemoryAndControllerEtcd(nil, nil)
	if err == nil || got != nil {
		t.Fatalf("nil,nil: want err, got %v, %v", got, err)
	}
	if !errors.Is(err, ErrNotFound) || !errors.Is(err, envmodel.ErrBuildEffectiveNotFound) {
		t.Fatalf("errors: %v", err)
	}
	want, err2 := envmodel.BuildOptionalEffectiveEnvironment(nil, nil)
	if !errors.Is(err, err2) || want != nil {
		t.Fatalf("envmodel mismatch: other=%v, %v", want, err2)
	}
}

func TestMergeMemoryAndControllerEtcd_etcdOnly(t *testing.T) {
	t.Parallel()
	etcd := &models.Environment{Name: "x", Type: "kubernetes", Bundles: &models.EnvironmentBundleConfig{Static: nil}}
	a, err := MergeMemoryAndControllerEtcd(nil, etcd)
	if err != nil {
		t.Fatal(err)
	}
	b, err2 := envmodel.BuildOptionalEffectiveEnvironment(nil, etcd)
	if err2 != nil {
		t.Fatal(err2)
	}
	if a.Name != b.Name || a.Type != b.Type {
		t.Fatalf("diff: %#v vs %#v", a, b)
	}
}
