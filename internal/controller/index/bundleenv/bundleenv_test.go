package bundleenv

import (
	"context"
	"errors"
	"testing"

	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/domain/interfaces"
	"merionyx/api-gateway/internal/controller/domain/models"
	xdscache "merionyx/api-gateway/internal/controller/xds/cache"
	"merionyx/api-gateway/internal/shared/bundlekey"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type stubEtcdEnv struct {
	envs map[string]*models.Environment
}

func (s *stubEtcdEnv) SaveEnvironment(ctx context.Context, env *models.Environment) error {
	return errors.New("not implemented")
}

func (s *stubEtcdEnv) GetEnvironment(ctx context.Context, name string) (*models.Environment, error) {
	e, ok := s.envs[name]
	if !ok {
		return nil, errors.New("not found")
	}
	return e, nil
}

func (s *stubEtcdEnv) ListEnvironments(ctx context.Context) (map[string]*models.Environment, error) {
	out := make(map[string]*models.Environment, len(s.envs))
	for k, v := range s.envs {
		out[k] = v
	}
	return out, nil
}

func (s *stubEtcdEnv) DeleteEnvironment(ctx context.Context, name string) error {
	return errors.New("not implemented")
}

func (s *stubEtcdEnv) WatchEnvironments(ctx context.Context) clientv3.WatchChan {
	return nil
}

type stubMemEnv struct {
	envs map[string]*models.Environment
}

func (s *stubMemEnv) SetDependencies(*xdscache.SnapshotManager, interfaces.XDSBuilder, interfaces.SchemaRepository) {
}

func (s *stubMemEnv) Initialize(*config.Config) error {
	return nil
}

func (s *stubMemEnv) GetEnvironment(ctx context.Context, name string) (*models.Environment, error) {
	e, ok := s.envs[name]
	if !ok {
		return nil, errors.New("not found")
	}
	return e, nil
}

func (s *stubMemEnv) ListEnvironments(ctx context.Context) (map[string]*models.Environment, error) {
	out := make(map[string]*models.Environment, len(s.envs))
	for k, v := range s.envs {
		out[k] = v
	}
	return out, nil
}

func (s *stubMemEnv) ApplyKubernetesEnvironments(ctx context.Context, envs map[string]*models.Environment) error {
	return nil
}

func envWithBundle(name, repo, ref, path string) *models.Environment {
	return &models.Environment{
		Name: name,
		Bundles: &models.EnvironmentBundleConfig{
			Static: []models.StaticContractBundleConfig{
				{Name: "b", Repository: repo, Ref: ref, Path: path},
			},
		},
	}
}

func TestIndex_Rebuild_memOnly_sharedBundle(t *testing.T) {
	bk := bundlekey.Build("r", "main", "pkg")
	mem := &stubMemEnv{
		envs: map[string]*models.Environment{
			"dev":  envWithBundle("dev", "r", "main", "pkg"),
			"prod": envWithBundle("prod", "r", "main", "pkg"),
		},
	}
	idx := NewIndex(nil, mem)
	idx.Rebuild(context.Background())

	got := idx.EnvironmentsForBundle(bk)
	if len(got) != 2 {
		t.Fatalf("want 2 envs, got %v", got)
	}
	if got[0] != "dev" || got[1] != "prod" {
		t.Fatalf("sorted names: %v", got)
	}
}

func TestIndex_Rebuild_etcdPreferredOverMem(t *testing.T) {
	etcd := &stubEtcdEnv{
		envs: map[string]*models.Environment{
			"only-etcd": envWithBundle("only-etcd", "x", "y", "z"),
		},
	}
	mem := &stubMemEnv{envs: map[string]*models.Environment{}}
	idx := NewIndex(etcd, mem)
	idx.Rebuild(context.Background())

	bk := bundlekey.Build("x", "y", "z")
	got := idx.EnvironmentsForBundle(bk)
	if len(got) != 1 || got[0] != "only-etcd" {
		t.Fatalf("got %v", got)
	}
}

func TestIndex_getEnvironment_etcdFirst(t *testing.T) {
	// Same name: etcd succeeds, mem has different bundles — index should use etcd definition.
	etcd := &stubEtcdEnv{
		envs: map[string]*models.Environment{
			"e": envWithBundle("e", "a", "b", "c"),
		},
	}
	mem := &stubMemEnv{
		envs: map[string]*models.Environment{
			"e": envWithBundle("e", "other", "b", "c"),
		},
	}
	idx := NewIndex(etcd, mem)
	idx.Rebuild(context.Background())

	bkEtcd := bundlekey.Build("a", "b", "c")
	if envs := idx.EnvironmentsForBundle(bkEtcd); len(envs) != 1 {
		t.Fatalf("expected env from etcd bundle, got %v", envs)
	}
}
