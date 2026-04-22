package registry

import (
	"context"
	"sort"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/pagination"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// TenantReadUseCase aggregates tenant-scoped registry views from controller registrations.
type TenantReadUseCase struct {
	controllers interfaces.ControllerRepository
}

func NewTenantReadUseCase(controllers interfaces.ControllerRepository) *TenantReadUseCase {
	return &TenantReadUseCase{controllers: controllers}
}

func (u *TenantReadUseCase) ListTenants(ctx context.Context, limit *int, cursor *string) ([]string, *string, bool, error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseRegistryPkg, "ListTenants"))
	defer span.End()
	all, err := u.controllers.ListControllers(ctx)
	if err != nil {
		telemetry.MarkError(span, err)
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
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseRegistryPkg, "ListEnvironmentsByTenant"))
	defer span.End()
	all, err := u.controllers.ListControllers(ctx)
	if err != nil {
		telemetry.MarkError(span, err)
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
					Name:     env.Name,
					Bundles:  dedupeBundles(env.Bundles),
					Services: dedupeServices(env.Services),
					Meta:     cloneEnvironmentMetaForMerge(env.Meta),
				}
				continue
			}
			prev.Bundles = dedupeBundles(append(append([]models.BundleInfo{}, prev.Bundles...), env.Bundles...))
			prev.Services = dedupeServices(append(append([]models.ServiceInfo{}, prev.Services...), env.Services...))
			prev.Meta = mergeEnvironmentMetasForTenant(prev.Meta, env.Meta)
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
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseRegistryPkg, "ListBundlesByTenant"))
	defer span.End()
	all, err := u.controllers.ListControllers(ctx)
	if err != nil {
		telemetry.MarkError(span, err)
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

func environmentMetaIsEmpty(m *models.EnvironmentMeta) bool {
	if m == nil {
		return true
	}
	return m.Provenance == nil && m.EffectiveGeneration == nil && m.SourcesFingerprint == "" &&
		m.EnvironmentType == "" && m.MaterializedUpdatedAt == "" && m.MaterializedSchemaVersion == nil && m.MaterializedMismatch == nil
}

func cloneEnvironmentMetaForMerge(m *models.EnvironmentMeta) *models.EnvironmentMeta {
	if m == nil {
		return nil
	}
	out := &models.EnvironmentMeta{SourcesFingerprint: m.SourcesFingerprint, EnvironmentType: m.EnvironmentType, MaterializedUpdatedAt: m.MaterializedUpdatedAt}
	if p := m.Provenance; p != nil {
		cp := *p
		out.Provenance = &cp
	}
	if m.EffectiveGeneration != nil {
		g := *m.EffectiveGeneration
		out.EffectiveGeneration = &g
	}
	if m.MaterializedSchemaVersion != nil {
		sv := *m.MaterializedSchemaVersion
		out.MaterializedSchemaVersion = &sv
	}
	if m.MaterializedMismatch != nil {
		mm := *m.MaterializedMismatch
		out.MaterializedMismatch = &mm
	}
	if environmentMetaIsEmpty(out) {
		return nil
	}
	return out
}

func mergeEnvironmentMetasForTenant(a, b *models.EnvironmentMeta) *models.EnvironmentMeta {
	if a == nil && b == nil {
		return nil
	}
	if a == nil {
		return cloneEnvironmentMetaForMerge(b)
	}
	if b == nil {
		return cloneEnvironmentMetaForMerge(a)
	}
	out := cloneEnvironmentMetaForMerge(a)
	if b.EffectiveGeneration != nil {
		if out.EffectiveGeneration == nil || *b.EffectiveGeneration > *out.EffectiveGeneration {
			g := *b.EffectiveGeneration
			out.EffectiveGeneration = &g
			if b.SourcesFingerprint != "" {
				out.SourcesFingerprint = b.SourcesFingerprint
			}
			// Newer materialization wins on observability row.
			out.MaterializedUpdatedAt = b.MaterializedUpdatedAt
			if b.MaterializedSchemaVersion != nil {
				sv := *b.MaterializedSchemaVersion
				out.MaterializedSchemaVersion = &sv
			}
		}
	} else if out.SourcesFingerprint == "" && b.SourcesFingerprint != "" {
		out.SourcesFingerprint = b.SourcesFingerprint
	}
	// Favor the newer `materialized_updated_at` (RFC3339) when both set.
	if b.MaterializedUpdatedAt != "" && a.MaterializedUpdatedAt != "" {
		if t2, e2 := time.Parse(time.RFC3339Nano, b.MaterializedUpdatedAt); e2 == nil {
			if t1, e1 := time.Parse(time.RFC3339Nano, a.MaterializedUpdatedAt); e1 == nil {
				if t2.After(t1) {
					out.MaterializedUpdatedAt = b.MaterializedUpdatedAt
					if b.MaterializedSchemaVersion != nil {
						sv := *b.MaterializedSchemaVersion
						out.MaterializedSchemaVersion = &sv
					}
				}
			}
		}
	}
	// Mismatch: OR across controllers for the same env.
	if a.MaterializedMismatch != nil && *a.MaterializedMismatch {
		m := true
		out.MaterializedMismatch = &m
	} else if b.MaterializedMismatch != nil && *b.MaterializedMismatch {
		m := true
		out.MaterializedMismatch = &m
	}
	if out.EnvironmentType == "" && b.EnvironmentType != "" {
		out.EnvironmentType = b.EnvironmentType
	} else if out.EnvironmentType == "" {
		out.EnvironmentType = a.EnvironmentType
	}
	ra := configSourceRank(models.EnvironmentMetaConfigSource(a))
	rb := configSourceRank(models.EnvironmentMetaConfigSource(b))
	stronger := a
	if rb > ra {
		stronger = b
	}
	if stronger != nil && stronger.Provenance != nil {
		p := *stronger.Provenance
		out.Provenance = &p
	} else {
		src := strongerConfigSource(models.EnvironmentMetaConfigSource(a), models.EnvironmentMetaConfigSource(b))
		if src != "" {
			if out.Provenance == nil {
				out.Provenance = &models.Provenance{ConfigSource: src}
			} else {
				cp := *out.Provenance
				cp.ConfigSource = src
				out.Provenance = &cp
			}
		}
	}
	if environmentMetaIsEmpty(out) {
		return nil
	}
	return out
}

func dedupeServices(in []models.ServiceInfo) []models.ServiceInfo {
	if len(in) <= 1 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]models.ServiceInfo, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s.Name]; ok {
			continue
		}
		seen[s.Name] = struct{}{}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// strongerConfigSource returns the higher-precedence origin (etcd > kubernetes > file) when both are set.
func strongerConfigSource(a, b string) string {
	if configSourceRank(b) > configSourceRank(a) {
		return b
	}
	return a
}

func configSourceRank(s string) int {
	switch s {
	case "etcd_grpc":
		return 3
	case "kubernetes":
		return 2
	case "file":
		return 1
	default:
		return 0
	}
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
