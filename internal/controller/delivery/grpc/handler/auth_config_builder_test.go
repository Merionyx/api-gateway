package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type testEnvRepo struct {
	get func(ctx context.Context, name string) (*models.Environment, error)
}

func (t *testEnvRepo) GetEnvironment(ctx context.Context, name string) (*models.Environment, error) {
	if t.get != nil {
		return t.get(ctx, name)
	}
	return nil, errors.New("not found")
}
func (*testEnvRepo) SaveEnvironment(context.Context, *models.Environment) error   { return nil }
func (*testEnvRepo) ListEnvironments(context.Context) (map[string]*models.Environment, error) { return nil, nil }
func (*testEnvRepo) DeleteEnvironment(context.Context, string) error { return nil }
func (*testEnvRepo) WatchEnvironments(context.Context) clientv3.WatchChan { return nil }

var _ interfaces.EnvironmentRepository = (*testEnvRepo)(nil)

type testSchemaList struct {
	list func(ctx context.Context, repository, ref, bundlePath string) ([]models.ContractSnapshot, error)
}

func (s *testSchemaList) ListContractSnapshots(ctx context.Context, repository, ref, bundlePath string) ([]models.ContractSnapshot, error) {
	if s.list != nil {
		return s.list(ctx, repository, ref, bundlePath)
	}
	return nil, nil
}
func (*testSchemaList) SaveContractSnapshot(context.Context, string, string, string, string, *models.ContractSnapshot) error {
	return nil
}
func (*testSchemaList) GetContractSnapshot(context.Context, string, string, string, string) (*models.ContractSnapshot, error) {
	return nil, nil
}
func (*testSchemaList) GetEnvironmentSnapshots(context.Context, string) ([]models.ContractSnapshot, error) {
	return nil, nil
}
func (*testSchemaList) WatchContractBundlesSnapshots(context.Context) clientv3.WatchChan { return nil }

var _ interfaces.SchemaRepository = (*testSchemaList)(nil)

func TestAppAllowedForEnvironment(t *testing.T) {
	t.Parallel()
	env := "dev"
	appAll := models.App{AppID: "a1", Environments: nil}
	if !appAllowedForEnvironment(env, appAll, appMatchSnapshotFromMemory) {
		t.Fatal("empty app envs => all")
	}
	if !appAllowedForEnvironment(env, models.App{AppID: "a2", Environments: []string{"dev"}}, appMatchSnapshotFromMemory) {
		t.Fatal("exact")
	}
	if appAllowedForEnvironment(env, models.App{AppID: "a3", Environments: []string{"prod"}}, appMatchSnapshotFromMemory) {
		t.Fatal("no match")
	}
	// pattern path: same string "dev" as pattern is valid literal match for MatchesEnvironmentPattern
	if !appAllowedForEnvironment(env, models.App{AppID: "a4", Environments: []string{"dev"}}, appMatchSchemaBundle) {
		t.Fatal("pattern path literal")
	}
}

func TestAuthConfigBuilder_inMemorySnapshotsWhenEtcdMissing(t *testing.T) {
	t.Parallel()
	etcd := &testEnvRepo{get: func(ctx context.Context, name string) (*models.Environment, error) {
		return nil, errors.New("no etcd")
	}}
	mem := &stubMem{
		envs: map[string]*models.Environment{
			"dev": {
				Name: "dev",
				Snapshots: []models.ContractSnapshot{
					{
						Name:   "c1",
						Prefix: "/p",
						Access: models.Access{
							Apps: []models.App{{AppID: "app1", Environments: []string{"dev"}}},
						},
					},
				},
			},
		},
	}
	b := NewAuthConfigBuilder(etcd, mem, &testSchemaList{})
	out, err := b.BuildAccessConfig(context.Background(), "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Contracts) != 1 || out.Contracts[0].ContractName != "c1" || len(out.Contracts[0].Apps) != 1 {
		t.Fatalf("got %+v", out.Contracts)
	}
}

func TestAuthConfigBuilder_etcdBundlesListSchema(t *testing.T) {
	t.Parallel()
	etcd := &testEnvRepo{get: func(ctx context.Context, name string) (*models.Environment, error) {
		return &models.Environment{
			Name: name,
			Bundles: &models.EnvironmentBundleConfig{
				Static: []models.StaticContractBundleConfig{
					{Name: "b", Repository: "r", Ref: "main", Path: "p"},
				},
			},
		}, nil
	}}
	schema := &testSchemaList{
		list: func(ctx context.Context, repository, ref, bundlePath string) ([]models.ContractSnapshot, error) {
			if repository != "r" || ref != "main" || bundlePath != "p" {
				t.Fatalf("unexpected %s %s %s", repository, ref, bundlePath)
			}
			return []models.ContractSnapshot{
				{
					Name:   "api",
					Prefix: "/api",
					Access: models.Access{
						Apps: []models.App{{AppID: "x", Environments: []string{`^dev$`}}}, // regex
					},
				},
			}, nil
		},
	}
	b := NewAuthConfigBuilder(etcd, &stubMem{envs: map[string]*models.Environment{}}, schema)
	out, err := b.BuildAccessConfig(context.Background(), "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Contracts) != 1 || out.Contracts[0].ContractName != "api" {
		t.Fatalf("got %+v", out.Contracts)
	}
}

func TestAuthConfigBuilder_notFound(t *testing.T) {
	t.Parallel()
	etcd := &testEnvRepo{get: func(ctx context.Context, name string) (*models.Environment, error) {
		return nil, errors.New("nope")
	}}
	mem := &stubMem{envs: map[string]*models.Environment{}}
	b := NewAuthConfigBuilder(etcd, mem, &testSchemaList{})
	_, err := b.BuildAccessConfig(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
}
