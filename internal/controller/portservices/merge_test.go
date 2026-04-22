package portservices

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/repository/memory"
)

func TestMergeEnvStaticWithRootPoolUpstreams_envWins(t *testing.T) {
	p := memory.NewServiceRepository()
	_ = p.Initialize(&config.Config{
		Services: config.ServicesConfig{Static: []config.StaticServiceConfig{
			{Name: "a", Upstream: "http://file:1"},
		}},
	})
	p.SetKubernetesGlobalServices([]models.StaticServiceConfig{
		{Name: "b", Upstream: "http://kube:2"},
		{Name: "a", Upstream: "http://bad"},
	})
	env := &models.Environment{Services: &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{
		{Name: "a", Upstream: "http://env:3"},
	}}}
	m := MergeEnvStaticWithRootPoolUpstreams(env, p)
	if m["a"] != "http://env:3" || m["b"] != "http://kube:2" {
		t.Fatalf("got %v", m)
	}
}

func TestMergeEnvStaticWithRootPoolUpstreams_emptyEnv(t *testing.T) {
	p := memory.NewServiceRepository()
	_ = p.Initialize(&config.Config{
		Services: config.ServicesConfig{Static: []config.StaticServiceConfig{{Name: "g", Upstream: "http://g"}}},
	})
	m := MergeEnvStaticWithRootPoolUpstreams(nil, p)
	if m["g"] != "http://g" {
		t.Fatalf("%v", m)
	}
}

func TestRootPoolDeduplicatedExcludingNames(t *testing.T) {
	p := memory.NewServiceRepository()
	_ = p.Initialize(&config.Config{
		Services: config.ServicesConfig{Static: []config.StaticServiceConfig{{Name: "a", Upstream: "1"}}},
	})
	p.SetKubernetesGlobalServices([]models.StaticServiceConfig{{Name: "b", Upstream: "2"}})
	exclude := map[string]struct{}{"a": {}}
	f, k := RootPoolDeduplicatedExcludingNames(p, exclude)
	if len(f) != 0 || len(k) != 1 || k[0].Name != "b" {
		t.Fatalf("f=%v k=%v", f, k)
	}
}
