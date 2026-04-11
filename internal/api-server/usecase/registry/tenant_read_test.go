package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
)

func TestTenantReadUseCase_ListTenants(t *testing.T) {
	t.Parallel()
	u := NewTenantReadUseCase(&stubControllerRepo{
		list: []models.ControllerInfo{
			{Tenant: "b"},
			{Tenant: "a"},
			{Tenant: ""},
			{Tenant: "a"},
		},
	})
	names, _, _, err := u.ListTenants(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Fatalf("got %#v", names)
	}

	u2 := NewTenantReadUseCase(&stubControllerRepo{err: errors.New("fail")})
	_, _, _, err = u2.ListTenants(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTenantReadUseCase_ListEnvironmentsByTenant_merge(t *testing.T) {
	t.Parallel()
	u := NewTenantReadUseCase(&stubControllerRepo{
		list: []models.ControllerInfo{
			{
				Tenant: "acme",
				Environments: []models.EnvironmentInfo{
					{
						Name: "dev",
						Bundles: []models.BundleInfo{
							{Name: "n1", Repository: "r", Ref: "main", Path: "p1"},
						},
					},
				},
			},
			{
				Tenant: "acme",
				Environments: []models.EnvironmentInfo{
					{
						Name: "dev",
						Bundles: []models.BundleInfo{
							{Name: "n2", Repository: "r", Ref: "main", Path: "p2"},
						},
					},
				},
			},
		},
	})
	envs, _, _, err := u.ListEnvironmentsByTenant(context.Background(), "acme", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(envs) != 1 || len(envs[0].Bundles) != 2 {
		t.Fatalf("got %#v", envs)
	}
}

func TestTenantReadUseCase_ListBundlesByTenant(t *testing.T) {
	t.Parallel()
	u := NewTenantReadUseCase(&stubControllerRepo{
		list: []models.ControllerInfo{
			{
				Tenant: "acme",
				Environments: []models.EnvironmentInfo{
					{
						Name: "dev",
						Bundles: []models.BundleInfo{
							{Name: "a", Repository: "r1", Ref: "m", Path: "p"},
							{Name: "b", Repository: "r2", Ref: "m", Path: "p"},
						},
					},
				},
			},
		},
	})
	bundles, _, _, err := u.ListBundlesByTenant(context.Background(), "acme", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundles) != 2 {
		t.Fatalf("got %#v", bundles)
	}
}
