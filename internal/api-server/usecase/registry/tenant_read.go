package registry

import (
	"context"
	"sort"

	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/pagination"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
)

// TenantReadUseCase aggregates tenant-scoped registry views from controller registrations.
type TenantReadUseCase struct {
	controllers interfaces.ControllerRepository
}

func NewTenantReadUseCase(controllers interfaces.ControllerRepository) *TenantReadUseCase {
	return &TenantReadUseCase{controllers: controllers}
}

func (u *TenantReadUseCase) ListTenants(ctx context.Context, limit *int, cursor *string) ([]string, *string, bool, error) {
	all, err := u.controllers.ListControllers(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	seen := make(map[string]struct{}, len(all))
	for i := range all {
		t := all[i].Tenant
		if t == "" {
			continue
		}
		seen[t] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for t := range seen {
		names = append(names, t)
	}
	sort.Strings(names)
	lim := pagination.ResolveLimit(limit)
	return pagination.PageStringSlice(names, lim, cursor)
}

// ListEnvironmentsByTenant merges environments for the tenant across controllers (same name → combined bundles, de-duplicated by bundle key).
func (u *TenantReadUseCase) ListEnvironmentsByTenant(ctx context.Context, tenant string, limit *int, cursor *string) ([]models.EnvironmentInfo, *string, bool, error) {
	all, err := u.controllers.ListControllers(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	byName := make(map[string]models.EnvironmentInfo)
	for i := range all {
		if all[i].Tenant != tenant {
			continue
		}
		for _, env := range all[i].Environments {
			prev, ok := byName[env.Name]
			if !ok {
				byName[env.Name] = models.EnvironmentInfo{
					Name:    env.Name,
					Bundles: dedupeBundles(env.Bundles),
				}
				continue
			}
			prev.Bundles = dedupeBundles(append(append([]models.BundleInfo{}, prev.Bundles...), env.Bundles...))
			byName[env.Name] = prev
		}
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	items := make([]models.EnvironmentInfo, 0, len(names))
	for _, n := range names {
		items = append(items, byName[n])
	}
	lim := pagination.ResolveLimit(limit)
	return pagination.PageSlice(items, lim, cursor)
}

// ListBundlesByTenant returns distinct static bundle descriptors for the tenant (union across environments and controllers).
func (u *TenantReadUseCase) ListBundlesByTenant(ctx context.Context, tenant string, limit *int, cursor *string) ([]models.BundleInfo, *string, bool, error) {
	all, err := u.controllers.ListControllers(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	seen := make(map[string]models.BundleInfo)
	for i := range all {
		if all[i].Tenant != tenant {
			continue
		}
		for _, env := range all[i].Environments {
			for _, b := range env.Bundles {
				k := bundlekey.Build(b.Repository, b.Ref, b.Path)
				if _, ok := seen[k]; ok {
					continue
				}
				seen[k] = b
			}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]models.BundleInfo, 0, len(keys))
	for _, k := range keys {
		out = append(out, seen[k])
	}
	lim := pagination.ResolveLimit(limit)
	return pagination.PageSlice(out, lim, cursor)
}

func dedupeBundles(in []models.BundleInfo) []models.BundleInfo {
	if len(in) <= 1 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]models.BundleInfo, 0, len(in))
	for _, b := range in {
		k := bundlekey.Build(b.Repository, b.Ref, b.Path)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool {
		return bundlekey.Build(out[i].Repository, out[i].Ref, out[i].Path) <
			bundlekey.Build(out[j].Repository, out[j].Ref, out[j].Path)
	})
	return out
}
