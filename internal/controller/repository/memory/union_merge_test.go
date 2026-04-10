package memory

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

func TestUnionStaticContractBundles(t *testing.T) {
	a := []models.StaticContractBundleConfig{
		{Name: "b1", Repository: "r1", Ref: "main", Path: "p1"},
	}
	b := []models.StaticContractBundleConfig{
		{Name: "b2", Repository: "r2", Ref: "main", Path: "p2"},
	}
	u := UnionStaticContractBundles(a, b)
	if len(u) != 2 {
		t.Fatalf("%+v", u)
	}
}

func TestUnionStaticServices_LaterOverrides(t *testing.T) {
	a := []models.StaticServiceConfig{{Name: "s", Upstream: "http://old:1"}}
	b := []models.StaticServiceConfig{{Name: "s", Upstream: "http://new:2"}}
	u := UnionStaticServices(a, b)
	if len(u) != 1 || u[0].Upstream != "http://new:2" {
		t.Fatalf("%+v", u)
	}
}

func TestMergeEnvironment(t *testing.T) {
	file := &models.Environment{
		Name: "e1",
		Type: "static",
		Bundles: &models.EnvironmentBundleConfig{
			Static: []models.StaticContractBundleConfig{{Name: "bf", Repository: "rf", Ref: "1", Path: "p"}},
		},
		Services: &models.EnvironmentServiceConfig{
			Static: []models.StaticServiceConfig{{Name: "sf", Upstream: "http://f:1"}},
		},
	}
	k8s := &models.Environment{
		Name: "e1",
		Type: "kubernetes",
		Bundles: &models.EnvironmentBundleConfig{
			Static: []models.StaticContractBundleConfig{{Name: "bk", Repository: "rk", Ref: "2", Path: "p"}},
		},
		Services: &models.EnvironmentServiceConfig{
			Static: []models.StaticServiceConfig{{Name: "sk", Upstream: "http://k:2"}},
		},
	}
	m := mergeEnvironment(file, k8s)
	if m.Name != "e1" || len(m.Bundles.Static) != 2 || len(m.Services.Static) != 2 {
		t.Fatalf("%+v", m)
	}
}
