package openapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

type strictFiberCtxKey struct{}

func BindFiberContextForStrictHandlers() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.SetContext(context.WithValue(c.Context(), strictFiberCtxKey{}, c))
		return c.Next()
	}
}

func fiberCtxFromStrictContext(ctx context.Context) (fiber.Ctx, error) {
	if ctx == nil {
		return nil, fmt.Errorf("missing strict request context")
	}
	fc, ok := ctx.Value(strictFiberCtxKey{}).(fiber.Ctx)
	if !ok || fc == nil {
		return nil, fmt.Errorf("missing fiber context in strict request context")
	}
	return fc, nil
}

func jsonETag(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf(`"%x"`, sum), nil
}

func ifNoneMatchMatches(ifNoneMatch string, etag string) bool {
	in := strings.TrimSpace(ifNoneMatch)
	in = strings.TrimPrefix(in, "W/")
	in = strings.Trim(in, `"`)
	want := strings.Trim(etag, `"`)
	return in == want
}

func internalProblem() apiserver.Problem {
	return problem.WithCode(http.StatusInternalServerError, problem.CodeInternalError, "", problem.DetailInternalError)
}

func asInternalProblemResponse() apiserver.InternalErrorApplicationProblemPlusJSONResponse {
	return apiserver.InternalErrorApplicationProblemPlusJSONResponse(internalProblem())
}

func mapDomainProblem(err error, allowed ...int) (int, apiserver.Problem) {
	st, p := problem.FromDomain(err)
	for i := range allowed {
		if st == allowed[i] {
			return st, p
		}
	}
	return http.StatusInternalServerError, internalProblem()
}

func mapContractPipelineProblem(err error, allowed ...int) (int, apiserver.Problem) {
	st, p := problem.FromContractSyncPipeline(err)
	for i := range allowed {
		if st == allowed[i] {
			return st, p
		}
	}
	return http.StatusInternalServerError, internalProblem()
}

func modelJWKSToAPI(in *models.JWKS) (apiserver.Jwks, error) {
	if in == nil {
		return apiserver.Jwks{}, nil
	}
	var out apiserver.Jwks
	b, err := json.Marshal(in)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	return out, nil
}

func subjectFromAPIJWTClaims(mc jwt.MapClaims) string {
	if e, _ := mc["email"].(string); strings.TrimSpace(e) != "" {
		return strings.TrimSpace(e)
	}
	if p, _ := mc["preferred_username"].(string); strings.TrimSpace(p) != "" {
		return strings.TrimSpace(p)
	}
	if s, _ := mc["sub"].(string); strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	return ""
}

func permissionsFromAPIJWTClaims(mc jwt.MapClaims) []any {
	return mergeAnyUnique(claimSliceToAny(mc, "permissions"), claimSliceToAny(mc, "scopes"))
}

func hasAnyRoleClaim(mc jwt.MapClaims) bool {
	return len(claimSliceToAny(mc, "roles")) > 0
}

func claimSliceToAny(mc jwt.MapClaims, key string) []any {
	v, ok := mc[key]
	if !ok || v == nil {
		return []any{}
	}
	switch x := v.(type) {
	case []any:
		return append([]any(nil), x...)
	case []string:
		out := make([]any, len(x))
		for i := range x {
			out[i] = x[i]
		}
		return out
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return []any{}
		}
		return []any{s}
	default:
		return []any{}
	}
}

func stringsToAny(in []string) []any {
	if len(in) == 0 {
		return []any{}
	}
	out := make([]any, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for i := range in {
		s := strings.TrimSpace(in[i])
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func mergeAnyUnique(base, add []any) []any {
	out := make([]any, 0, len(base)+len(add))
	seen := make(map[string]struct{}, len(base)+len(add))
	appendSlice := func(in []any) {
		for i := range in {
			s, _ := in[i].(string)
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	appendSlice(base)
	appendSlice(add)
	return out
}

func snapshotForAPIAccess(perms []any, mc jwt.MapClaims) ([]byte, error) {
	m := map[string]any{"omit_roles": true}
	if len(perms) > 0 {
		m["permissions"] = perms
	}
	if mc != nil {
		if s, _ := mc["idp_iss"].(string); strings.TrimSpace(s) != "" {
			m["idp_iss"] = strings.TrimSpace(s)
		}
		if s, _ := mc["idp_sub"].(string); strings.TrimSpace(s) != "" {
			m["idp_sub"] = strings.TrimSpace(s)
		}
	}
	return json.Marshal(m)
}

func numericUnixClaimToTime(mc jwt.MapClaims, key string) (time.Time, bool) {
	v, ok := mc[key]
	if !ok || v == nil {
		return time.Time{}, false
	}
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return time.Time{}, false
		}
		return time.Unix(int64(x), 0).UTC(), true
	case int64:
		return time.Unix(x, 0).UTC(), true
	case int:
		return time.Unix(int64(x), 0).UTC(), true
	default:
		return time.Time{}, false
	}
}

func resolveIssuedAPIAccessTTL(now time.Time, policyTTL time.Duration, callerClaims map[string]any, requestedExpiresAt *time.Time) (time.Duration, error) {
	callerExp, ok := numericUnixClaimToTime(callerClaims, "exp")
	if !ok {
		return 0, fmt.Errorf("caller token has no valid exp claim")
	}
	policyExp := now.Add(policyTTL)
	maxExp := policyExp
	if callerExp.Before(maxExp) {
		maxExp = callerExp
	}
	if !maxExp.After(now) {
		return 0, fmt.Errorf("caller token is too close to expiry")
	}

	targetExp := maxExp
	if requestedExpiresAt != nil {
		reqExp := requestedExpiresAt.UTC()
		if !reqExp.After(now) {
			return 0, fmt.Errorf("expires_at must be in the future")
		}
		if reqExp.After(maxExp) {
			return 0, fmt.Errorf("expires_at exceeds caller or policy limits")
		}
		targetExp = reqExp
	}
	ttl := targetExp.Sub(now)
	if ttl <= 0 {
		return 0, fmt.Errorf("computed token ttl is non-positive")
	}
	return ttl, nil
}

func normalizeRequestedPermissions(in *[]string) []string {
	if in == nil || len(*in) == 0 {
		return nil
	}
	out := make([]string, 0, len(*in))
	seen := make(map[string]struct{}, len(*in))
	for i := range *in {
		s := strings.TrimSpace((*in)[i])
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func permissionDescriptorsFromIDs(ids []string) []apiserver.PermissionDescriptor {
	if len(ids) == 0 {
		return []apiserver.PermissionDescriptor{}
	}
	unique := uniqueSortedStrings(ids)
	out := make([]apiserver.PermissionDescriptor, 0, len(unique))
	for _, permissionID := range unique {
		out = append(out, apiserver.PermissionDescriptor{
			Id:          permissionID,
			Description: permissions.Describe(permissionID),
		})
	}
	return out
}

func claimString(mc jwt.MapClaims, key string) string {
	v, ok := mc[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return ""
	}
}

func uniqueSortedStrings(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for i := range in {
		s := strings.TrimSpace(in[i])
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func mapKeysSorted(set map[string]struct{}) []string {
	if len(set) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func durationFromOptionalFormSeconds(v *int) time.Duration {
	if v == nil {
		return 0
	}
	if *v <= 0 {
		return 0
	}
	return time.Duration(*v) * time.Second
}

func usesBasicAuthorizationHeader(h string) bool {
	raw := strings.TrimSpace(h)
	if raw == "" {
		return false
	}
	if strings.EqualFold(raw, "Basic") {
		return true
	}
	if len(raw) < len("Basic ")+1 {
		return false
	}
	return strings.EqualFold(raw[:len("Basic ")], "Basic ")
}

func controllerToAPI(c models.ControllerInfo) apiserver.Controller {
	envs := make([]apiserver.Environment, 0, len(c.Environments))
	for _, e := range c.Environments {
		envs = append(envs, environmentToAPI(e))
	}
	out := apiserver.Controller{
		ControllerId: c.ControllerID,
		Tenant:       c.Tenant,
		Environments: &envs,
	}
	if c.RegistryPayloadVersion > 0 {
		v := c.RegistryPayloadVersion
		out.RegistryPayloadVersion = &v
	}
	return out
}

func environmentToAPI(e models.EnvironmentInfo) apiserver.Environment {
	bundles := make([]apiserver.Bundle, 0, len(e.Bundles))
	for _, b := range e.Bundles {
		bundles = append(bundles, bundleToAPI(b))
	}
	svcs := make([]apiserver.RegistryService, 0, len(e.Services))
	for _, s := range e.Services {
		svcs = append(svcs, staticServiceToAPI(s))
	}
	out := apiserver.Environment{Name: e.Name, Bundles: &bundles, Services: &svcs}
	if m := environmentMetaToAPI(e.Meta); m != nil {
		out.Meta = m
	}
	return out
}

func environmentMetaToAPI(m *models.EnvironmentMeta) *apiserver.EnvironmentMeta {
	if m == nil {
		return nil
	}
	out := &apiserver.EnvironmentMeta{}
	if p := provenanceToAPI(m.Provenance); p != nil {
		out.Provenance = p
	}
	if m.EffectiveGeneration != nil {
		g := *m.EffectiveGeneration
		out.EffectiveGeneration = &g
	}
	if m.SourcesFingerprint != "" {
		s := m.SourcesFingerprint
		out.SourcesFingerprint = &s
	}
	if m.EnvironmentType != "" {
		et := m.EnvironmentType
		out.EnvironmentType = &et
	}
	if m.MaterializedUpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, m.MaterializedUpdatedAt); err == nil {
			out.MaterializedUpdatedAt = &t
		} else if t, err := time.Parse(time.RFC3339, m.MaterializedUpdatedAt); err == nil {
			out.MaterializedUpdatedAt = &t
		}
	}
	if m.MaterializedSchemaVersion != nil {
		sv := *m.MaterializedSchemaVersion
		out.MaterializedSchemaVersion = &sv
	}
	if m.MaterializedMismatch != nil {
		mm := *m.MaterializedMismatch
		out.MaterializedMismatch = &mm
	}
	if out.Provenance == nil && out.EffectiveGeneration == nil && out.SourcesFingerprint == nil && out.EnvironmentType == nil && out.MaterializedUpdatedAt == nil && out.MaterializedSchemaVersion == nil && out.MaterializedMismatch == nil {
		return nil
	}
	return out
}

func provenanceToAPI(p *models.Provenance) *apiserver.Provenance {
	if p == nil {
		return nil
	}
	var out apiserver.Provenance
	if p.ConfigSource != "" {
		if cs := modelConfigSourceToAPI(p.ConfigSource); cs != nil {
			out.ConfigSource = cs
		}
	}
	if p.LayerDetail != "" {
		d := p.LayerDetail
		out.LayerDetail = &d
	}
	if out.ConfigSource == nil && out.LayerDetail == nil {
		return nil
	}
	return &out
}

func staticServiceToAPI(s models.ServiceInfo) apiserver.RegistryService {
	n, u := s.Name, s.Upstream
	out := apiserver.RegistryService{Name: n, Upstream: u}
	if s.Scope != "" {
		scope := apiserver.ServiceLineScope(s.Scope)
		if scope.Valid() {
			out.Scope = &scope
		} else {
			unspec := apiserver.ServiceLineScopeUnspecified
			out.Scope = &unspec
		}
	}
	if sm := s.Meta; sm != nil {
		if p := provenanceToAPI(sm.Provenance); p != nil || sm.K8sServiceRef != "" {
			m := &apiserver.ServiceMeta{}
			if p != nil {
				m.Provenance = p
			}
			if sm.K8sServiceRef != "" {
				ksr := sm.K8sServiceRef
				m.K8sServiceRef = &ksr
			}
			if m.Provenance != nil || m.K8sServiceRef != nil {
				out.Meta = m
			}
		}
	}
	return out
}

func bundleFromCanonicalKey(bundleKey string) (apiserver.Bundle, error) {
	repo, ref, path, err := bundlekey.Parse(bundleKey)
	if err != nil {
		return apiserver.Bundle{}, err
	}
	k := bundlekey.Build(repo, ref, path)
	repoP, refP, pathP := repo, ref, path
	return apiserver.Bundle{
		Key:        &k,
		Repository: &repoP,
		Ref:        &refP,
		Path:       &pathP,
	}, nil
}

func bundleToAPI(b models.BundleInfo) apiserver.Bundle {
	nm := b.Name
	repo := b.Repository
	ref := b.Ref
	path := b.Path
	k := bundlekey.Build(b.Repository, b.Ref, b.Path)
	out := apiserver.Bundle{
		Key:        &k,
		Name:       &nm,
		Repository: &repo,
		Ref:        &ref,
		Path:       &path,
	}
	if bm := b.Meta; bm != nil {
		if p := provenanceToAPI(bm.Provenance); p != nil || bm.ResolvedRef != "" || bm.LastSyncUTC != "" || bm.SyncError != "" || bm.K8SResourceRef != "" {
			m := &apiserver.BundleMeta{}
			if p != nil {
				m.Provenance = p
			}
			if bm.ResolvedRef != "" {
				r := bm.ResolvedRef
				m.ResolvedRef = &r
			}
			if bm.LastSyncUTC != "" {
				if t, err := time.Parse(time.RFC3339Nano, bm.LastSyncUTC); err == nil {
					m.LastSyncUtc = &t
				} else if t, err := time.Parse(time.RFC3339, bm.LastSyncUTC); err == nil {
					m.LastSyncUtc = &t
				}
			}
			if bm.SyncError != "" {
				se := bm.SyncError
				m.SyncError = &se
			}
			if bm.K8SResourceRef != "" {
				k := bm.K8SResourceRef
				m.K8sResourceRef = &k
			}
			if m.Provenance != nil || m.ResolvedRef != nil || m.LastSyncUtc != nil || m.SyncError != nil || m.K8sResourceRef != nil {
				out.Meta = m
			}
		}
	}
	return out
}

func modelConfigSourceToAPI(s string) *apiserver.ConfigSource {
	if s == "" {
		return nil
	}
	var c apiserver.ConfigSource
	switch s {
	case "file":
		c = apiserver.ConfigSourceFile
	case "kubernetes":
		c = apiserver.ConfigSourceKubernetes
	case "etcd_grpc":
		c = apiserver.ConfigSourceEtcdGrpc
	case "unspecified":
		c = apiserver.ConfigSourceUnspecified
	default:
		c = apiserver.ConfigSource(s)
	}
	return &c
}

func snapshotsToAPI(in []sharedgit.ContractSnapshot) ([]apiserver.ContractSnapshot, error) {
	out := make([]apiserver.ContractSnapshot, len(in))
	for i := range in {
		b, err := json.Marshal(in[i])
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func bundleKeyFromContractBundleParams(
	bk *apiserver.BundleKeyQuery,
	repo *apiserver.BundleRepoQuery,
	ref *apiserver.BundleRefQuery,
	path *apiserver.BundlePathQuery,
) (string, error) {
	var kb, r, f, p string
	if bk != nil {
		kb = string(*bk)
	}
	if repo != nil {
		r = string(*repo)
	}
	if ref != nil {
		f = string(*ref)
	}
	if path != nil {
		p = string(*path)
	}
	return bundlekey.ResolveFromHTTPQuery(kb, r, f, p)
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
