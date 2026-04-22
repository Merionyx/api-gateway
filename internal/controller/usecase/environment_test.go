package usecase

import (
	"context"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/repository/memory"
	"github.com/merionyx/api-gateway/internal/controller/xds/builder"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type fakeEnvRepo struct {
	get     *models.Environment
	getErr  error
	saved   []*models.Environment
	list    map[string]*models.Environment
	delName string
}

func (f *fakeEnvRepo) SaveEnvironment(_ context.Context, env *models.Environment) error {
	f.saved = append(f.saved, env)
	return nil
}
func (f *fakeEnvRepo) GetEnvironment(context.Context, string) (*models.Environment, error) {
	return f.get, f.getErr
}
func (f *fakeEnvRepo) ListEnvironments(context.Context) (map[string]*models.Environment, error) {
	return f.list, nil
}
func (f *fakeEnvRepo) DeleteEnvironment(_ context.Context, name string) error {
	f.delName = name
	return nil
}
func (f *fakeEnvRepo) WatchEnvironments(context.Context) clientv3.WatchChan { return nil }

func TestEnvironmentsUseCase_CreateEnvironment(t *testing.T) {
	svcRepo := memory.NewServiceRepository()
	if err := svcRepo.Initialize(&config.Config{
		Services: config.ServicesConfig{
			Static: []config.StaticServiceConfig{{Name: "s1", Upstream: "http://127.0.0.1:1"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	_ = builder.NewXDSBuilder(svcRepo)
	su := NewSchemasUseCase().(*schemasUseCase)
	su.SetDependencies(&fakeSchemaRepo{}, nil)

	uc := NewEnvironmentsUseCase().(*environmentsUseCase)
	repo := &fakeEnvRepo{}
	uc.SetDependencies(repo, nil, su, nil)

	env, err := uc.CreateEnvironment(context.Background(), &models.CreateEnvironmentRequest{
		Name: "test-env",
		Bundles: &models.EnvironmentBundleConfig{
			Static: []models.StaticContractBundleConfig{{Name: "b1", Repository: "r", Ref: "main", Path: "p"}},
		},
		Services: &models.EnvironmentServiceConfig{
			Static: []models.StaticServiceConfig{{Name: "s1", Upstream: "http://127.0.0.1:1"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if env.Name != "test-env" || len(repo.saved) != 1 {
		t.Fatalf("env=%+v saved=%d", env, len(repo.saved))
	}
}

func TestEnvironmentsUseCase_CreateEnvironment_AlreadyExists(t *testing.T) {
	svcRepo := memory.NewServiceRepository()
	_ = svcRepo.Initialize(&config.Config{Services: config.ServicesConfig{Static: []config.StaticServiceConfig{{Name: "s1", Upstream: "http://x"}}}})
	_ = builder.NewXDSBuilder(svcRepo)
	su := NewSchemasUseCase().(*schemasUseCase)
	su.SetDependencies(&fakeSchemaRepo{}, nil)
	uc := NewEnvironmentsUseCase().(*environmentsUseCase)
	uc.SetDependencies(&fakeEnvRepo{get: &models.Environment{Name: "e1"}}, nil, su, nil)

	_, err := uc.CreateEnvironment(context.Background(), &models.CreateEnvironmentRequest{Name: "e1"})
	if err == nil {
		t.Fatal("expected error")
	}
}
