package memory

import (
	"context"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/xds/builder"
	xdscache "github.com/merionyx/api-gateway/internal/controller/xds/cache"
)

func TestEnvironmentsRepository_Initialize_RebuildsXDS(t *testing.T) {
	memSvc := NewServiceRepository()
	if err := memSvc.Initialize(&config.Config{
		Services: config.ServicesConfig{
			Static: []config.StaticServiceConfig{{Name: "be", Upstream: "http://127.0.0.1:8080"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	xb := builder.NewXDSBuilder(memSvc)
	sm := xdscache.NewSnapshotManager(false)

	repo := NewEnvironmentsRepository().(*EnvironmentsRepository)
	repo.SetDependencies(sm, xb, nil)

	cfg := &config.Config{
		Services: config.ServicesConfig{
			Static: []config.StaticServiceConfig{{Name: "be", Upstream: "http://127.0.0.1:8080"}},
		},
		Environments: []config.EnvironmentConfig{
			{
				Name: "dev",
				Bundles: config.BundlesConfig{
					Static: []config.StaticBundleConfig{{Name: "b1", Repository: "repo", Ref: "v1", Path: "openapi"}},
				},
				Services: config.ServicesConfig{
					Static: []config.StaticServiceConfig{{Name: "be", Upstream: "http://127.0.0.1:8080"}},
				},
			},
		},
	}
	if err := repo.Initialize(cfg); err != nil {
		t.Fatal(err)
	}

	envs, err := repo.ListEnvironments(context.Background())
	if err != nil || len(envs) != 1 {
		t.Fatalf("list: %v %d", err, len(envs))
	}
	if _, err := sm.GetSnapshot("envoy-dev"); err != nil {
		t.Fatalf("expected xDS snapshot for dev: %v", err)
	}
}
