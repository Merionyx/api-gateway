package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/index/bundleenv"
	xdscache "github.com/merionyx/api-gateway/internal/controller/xds/cache"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
)

type stubMem struct {
	envs map[string]*models.Environment
}

func (s *stubMem) SetDependencies(*xdscache.SnapshotManager, interfaces.XDSBuilder, interfaces.SchemaRepository) {
}
func (s *stubMem) Initialize(*config.Config) error { return nil }
func (s *stubMem) GetEnvironment(ctx context.Context, name string) (*models.Environment, error) {
	e, ok := s.envs[name]
	if !ok {
		return nil, errors.New("not found")
	}
	return e, nil
}
func (s *stubMem) ListEnvironments(ctx context.Context) (map[string]*models.Environment, error) {
	out := make(map[string]*models.Environment, len(s.envs))
	for k, v := range s.envs {
		out[k] = v
	}
	return out, nil
}
func (s *stubMem) ApplyKubernetesEnvironments(ctx context.Context, envs map[string]*models.Environment) error {
	return nil
}

func envNamed(name string, repo, ref, path string) *models.Environment {
	return &models.Environment{
		Name: name,
		Bundles: &models.EnvironmentBundleConfig{
			Static: []models.StaticContractBundleConfig{
				{Name: "b", Repository: repo, Ref: ref, Path: path},
			},
		},
	}
}

func TestPlanAuthSchemaNotify_nilIndex_notifyAll(t *testing.T) {
	plan := PlanAuthSchemaNotify("any", nil, false)
	if !plan.NotifyAll || len(plan.TargetEnvironments) != 0 {
		t.Fatalf("got %+v", plan)
	}
}

func TestPlanAuthSchemaNotify_mapsEnvironments(t *testing.T) {
	bk := bundlekey.Build("r", "main", "p")
	mem := &stubMem{
		envs: map[string]*models.Environment{
			"dev": envNamed("dev", "r", "main", "p"),
		},
	}
	idx := bundleenv.NewIndex(nil, mem)
	idx.Rebuild(context.Background())

	plan := PlanAuthSchemaNotify(bk, idx, false)
	if plan.NotifyAll || len(plan.TargetEnvironments) != 1 || plan.TargetEnvironments[0] != "dev" {
		t.Fatalf("got %+v", plan)
	}
}

func TestPlanAuthSchemaNotify_emptyAfterRebuild_notifyAll(t *testing.T) {
	mem := &stubMem{envs: map[string]*models.Environment{}}
	idx := bundleenv.NewIndex(nil, mem)
	idx.Rebuild(context.Background())

	plan := PlanAuthSchemaNotify("unknown-bundle-key", idx, false)
	if !plan.NotifyAll {
		t.Fatalf("expected NotifyAll, got %+v", plan)
	}
}
