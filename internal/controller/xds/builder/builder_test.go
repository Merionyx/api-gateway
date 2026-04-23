package builder

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/repository/memory"
)

func TestXDSBuilder_BuildAll_RealisticEnvironment(t *testing.T) {
	svc := memory.NewServiceRepository()
	if err := svc.Initialize(&config.Config{
		Services: config.ServicesConfig{
			Static: []config.StaticServiceConfig{
				{Name: "be-svc", Upstream: "http://backend.example:8080"},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	b := NewXDSBuilder(svc)

	env := &models.Environment{
		Name: "test-env",
		Services: &models.EnvironmentServiceConfig{
			Static: []models.StaticServiceConfig{
				{Name: "be-svc", Upstream: "http://backend.example:8080"},
			},
		},
		Snapshots: []models.ContractSnapshot{
			{
				Name:   "api-v1",
				Prefix: "/api/v1/",
				Upstream: models.ContractUpstream{
					Name: "be-svc",
				},
				Access: models.Access{Secure: true, Apps: []models.App{{AppID: "app1", Environments: []string{"test-env"}}}},
			},
		},
	}

	listeners, err := b.BuildListeners(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(listeners) != 1 || listeners[0].GetName() == "" {
		t.Fatalf("listeners: %+v", listeners)
	}

	clusters, err := b.BuildClusters(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) < 2 {
		t.Fatalf("expected sidecar + service clusters, got %d", len(clusters))
	}

	routes, err := b.BuildRoutes(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || len(routes[0].GetVirtualHosts()) != 1 {
		t.Fatalf("routes: %+v", routes)
	}

	endpoints, err := b.BuildEndpoints(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("endpoints: %+v", endpoints)
	}
}
