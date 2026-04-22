package usecase

import (
	"context"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/repository/memory"
	"github.com/merionyx/api-gateway/internal/controller/xds/builder"
	xdscache "github.com/merionyx/api-gateway/internal/controller/xds/cache"
)

type snapEnvFake struct {
	list map[string]*models.Environment
}

func (snapEnvFake) SetDependencies(interfaces.EnvironmentRepository, interfaces.InMemoryEnvironmentsRepository, interfaces.SchemasUseCase, interfaces.EffectiveReconciler) {
}

func (f snapEnvFake) CreateEnvironment(context.Context, *models.CreateEnvironmentRequest) (*models.Environment, error) {
	return nil, nil
}
func (f snapEnvFake) GetEnvironment(context.Context, string) (*models.Environment, error) {
	return nil, nil
}
func (f snapEnvFake) ListEnvironments(context.Context) (map[string]*models.Environment, error) {
	return f.list, nil
}
func (snapEnvFake) UpdateEnvironment(context.Context, *models.UpdateEnvironmentRequest) (*models.Environment, error) {
	return nil, nil
}
func (snapEnvFake) DeleteEnvironment(context.Context, string) error { return nil }

var _ interfaces.EnvironmentsUseCase = snapEnvFake{}

func TestSnapshotsUseCase_UpdateSnapshot_AllEnvironments(t *testing.T) {
	memSvc := memory.NewServiceRepository()
	if err := memSvc.Initialize(&config.Config{
		Services: config.ServicesConfig{
			Static: []config.StaticServiceConfig{{Name: "be", Upstream: "http://127.0.0.1:1"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	xb := builder.NewXDSBuilder(memSvc)
	sm := xdscache.NewSnapshotManager(false)

	env := &models.Environment{
		Name: "e1",
		Services: &models.EnvironmentServiceConfig{
			Static: []models.StaticServiceConfig{{Name: "be", Upstream: "http://127.0.0.1:1"}},
		},
		Snapshots: []models.ContractSnapshot{
			{Name: "c1", Prefix: "/p/", Upstream: models.ContractUpstream{Name: "be"}},
		},
	}
	fake := snapEnvFake{list: map[string]*models.Environment{"e1": env}}

	uc := NewSnapshotsUseCase().(*snapshotsUseCase)
	uc.SetDependencies(fake, sm, xb)

	resp, err := uc.UpdateSnapshot(context.Background(), &models.UpdateSnapshotRequest{Environment: ""})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success || len(resp.UpdatedEnvironments) != 1 {
		t.Fatalf("%+v", resp)
	}
}

func TestSnapshotsUseCase_GetSnapshotStatus(t *testing.T) {
	memSvc := memory.NewServiceRepository()
	_ = memSvc.Initialize(&config.Config{
		Services: config.ServicesConfig{
			Static: []config.StaticServiceConfig{{Name: "be", Upstream: "http://127.0.0.1:1"}},
		},
	})
	xb := builder.NewXDSBuilder(memSvc)
	sm := xdscache.NewSnapshotManager(false)

	env := &models.Environment{
		Name: "e2",
		Services: &models.EnvironmentServiceConfig{
			Static: []models.StaticServiceConfig{{Name: "be", Upstream: "http://127.0.0.1:1"}},
		},
		Snapshots: []models.ContractSnapshot{{Name: "x", Prefix: "/", Upstream: models.ContractUpstream{Name: "be"}}},
	}
	fake := snapFakeGet{env: env}
	uc := NewSnapshotsUseCase().(*snapshotsUseCase)
	uc.SetDependencies(fake, sm, xb)

	if _, err := uc.UpdateSnapshot(context.Background(), &models.UpdateSnapshotRequest{Environment: "e2"}); err != nil {
		t.Fatal(err)
	}
	st, err := uc.GetSnapshotStatus(context.Background(), &models.GetSnapshotStatusRequest{Environment: "e2"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Environment != "e2" || st.ContractsCount != 1 {
		t.Fatalf("%+v", st)
	}
}

type snapFakeGet struct {
	env *models.Environment
}

func (snapFakeGet) SetDependencies(interfaces.EnvironmentRepository, interfaces.InMemoryEnvironmentsRepository, interfaces.SchemasUseCase, interfaces.EffectiveReconciler) {
}
func (f snapFakeGet) GetEnvironment(context.Context, string) (*models.Environment, error) {
	return f.env, nil
}
func (snapFakeGet) CreateEnvironment(context.Context, *models.CreateEnvironmentRequest) (*models.Environment, error) {
	return nil, nil
}
func (snapFakeGet) ListEnvironments(context.Context) (map[string]*models.Environment, error) {
	return nil, nil
}
func (snapFakeGet) UpdateEnvironment(context.Context, *models.UpdateEnvironmentRequest) (*models.Environment, error) {
	return nil, nil
}
func (snapFakeGet) DeleteEnvironment(context.Context, string) error { return nil }

var _ interfaces.EnvironmentsUseCase = snapFakeGet{}
