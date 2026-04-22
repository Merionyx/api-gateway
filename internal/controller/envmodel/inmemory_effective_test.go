package envmodel

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

func TestInMemoryEffective_nil(t *testing.T) {
	if InMemoryEffective(nil, nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestInMemoryEffective_merged_not_alias(t *testing.T) {
	f := &models.Environment{Name: "e", Type: "static", Bundles: &models.EnvironmentBundleConfig{
		Static: []models.StaticContractBundleConfig{{Name: "a", Repository: "r", Ref: "1", Path: "p"}},
	}, Services: &models.EnvironmentServiceConfig{Static: nil}, Snapshots: nil}
	k := &models.Environment{Name: "e", Type: "kubernetes", Bundles: &models.EnvironmentBundleConfig{
		Static: []models.StaticContractBundleConfig{{Name: "b", Repository: "r2", Ref: "2", Path: "p2"}},
	}, Services: &models.EnvironmentServiceConfig{Static: nil}, Snapshots: nil}
	m := InMemoryEffective(f, k)
	if m == f || m == k {
		t.Fatalf("merge must not alias file or k8s pointers: %p %p %p", f, k, m)
	}
}

func TestInMemoryEffective_fileOnly_copies(t *testing.T) {
	f := &models.Environment{Name: "e", Type: "static", Bundles: &models.EnvironmentBundleConfig{
		Static: []models.StaticContractBundleConfig{{Name: "a", Repository: "r", Ref: "1", Path: "p"}},
	}, Services: &models.EnvironmentServiceConfig{Static: nil}, Snapshots: make([]models.ContractSnapshot, 0)}
	m := InMemoryEffective(f, nil)
	if m == f {
		t.Fatal("file-only result must not alias fromFile")
	}
	if m.Bundles == f.Bundles {
		t.Fatal("bundles config must be a distinct copy")
	}
}

func TestInMemoryEffective_k8sOnly_copies(t *testing.T) {
	k := &models.Environment{Name: "e", Type: "kubernetes", Bundles: &models.EnvironmentBundleConfig{
		Static: []models.StaticContractBundleConfig{{Name: "a", Repository: "r", Ref: "1", Path: "p"}},
	}, Services: &models.EnvironmentServiceConfig{Static: nil}, Snapshots: nil}
	m := InMemoryEffective(nil, k)
	if m == k {
		t.Fatal("k8s-only result must not alias fromK8s")
	}
}
