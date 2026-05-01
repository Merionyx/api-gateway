package openapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	httpauthz "github.com/merionyx/api-gateway/internal/api-server/delivery/http/authz"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/idempotency"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/bundle"
	"github.com/merionyx/api-gateway/internal/api-server/version"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"

	"github.com/merionyx/api-gateway/internal/api-server/container"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
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

func claimStrings(mc jwt.MapClaims, key string) []string {
	v, ok := mc[key]
	if !ok || v == nil {
		return nil
	}
	switch x := v.(type) {
	case []string:
		return uniqueSortedStrings(x)
	case []any:
		out := make([]string, 0, len(x))
		for i := range x {
			s, _ := x[i].(string)
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		return uniqueSortedStrings(out)
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return nil
		}
		return []string{s}
	default:
		return nil
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

func hasPermission(have map[string]struct{}, required string) bool {
	required = strings.TrimSpace(required)
	if required == "" {
		return false
	}
	if _, ok := have[permissions.Wildcard]; ok {
		return true
	}
	_, ok := have[required]
	return ok
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

func clientIDFromBasicAuth(h string) string {
	raw := strings.TrimSpace(h)
	if len(raw) < len("Basic ") || !strings.EqualFold(raw[:len("Basic ")], "Basic ") {
		return ""
	}
	payload := strings.TrimSpace(raw[len("Basic "):])
	b, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return ""
	}
	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
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

// StrictOpenAPIServer implements generated StrictServerInterface and calls usecase/domain logic directly.
type StrictOpenAPIServer struct {
	c *container.Container
}

func NewStrictOpenAPIServer(c *container.Container) apiserver.StrictServerInterface {
	if c == nil {
		panic("strict openapi server requires container")
	}
	if c.PermissionEvaluator == nil {
		panic("strict openapi server requires permission evaluator")
	}
	return &StrictOpenAPIServer{c: c}
}

func (s *StrictOpenAPIServer) GetJwksEdge(ctx context.Context, request apiserver.GetJwksEdgeRequestObject) (apiserver.GetJwksEdgeResponseObject, error) {
	jwks, err := s.c.JWTUseCase.GetJWKSEdge(ctx)
	if err != nil {
		st, p := mapDomainProblem(err, http.StatusBadRequest)
		if st == http.StatusBadRequest {
			return apiserver.GetJwksEdge400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		return apiserver.GetJwksEdge500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	body, err := modelJWKSToAPI(jwks)
	if err != nil {
		return apiserver.GetJwksEdge500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: asInternalProblemResponse(),
		}, nil
	}
	return apiserver.GetJwksEdge200JSONResponse{Body: body}, nil
}

func (s *StrictOpenAPIServer) GetJwks(ctx context.Context, request apiserver.GetJwksRequestObject) (apiserver.GetJwksResponseObject, error) {
	jwks, err := s.c.JWTUseCase.GetJWKS(ctx)
	if err != nil {
		st, p := mapDomainProblem(err, http.StatusBadRequest)
		if st == http.StatusBadRequest {
			return apiserver.GetJwks400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		return apiserver.GetJwks500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	body, err := modelJWKSToAPI(jwks)
	if err != nil {
		return apiserver.GetJwks500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: asInternalProblemResponse(),
		}, nil
	}
	return apiserver.GetJwks200JSONResponse{Body: body}, nil
}

func (s *StrictOpenAPIServer) GetHealth(ctx context.Context, request apiserver.GetHealthRequestObject) (apiserver.GetHealthResponseObject, error) {
	return apiserver.GetHealth200JSONResponse{
		Data: apiserver.HealthStatus{Status: "ok"},
	}, nil
}

func (s *StrictOpenAPIServer) GetReady(ctx context.Context, request apiserver.GetReadyRequestObject) (apiserver.GetReadyResponseObject, error) {
	r := s.c.StatusReadUseCase.Readiness(ctx, s.c.Config.Readiness.RequireContractSyncer)
	body := apiserver.ReadinessStatus{
		Status:         r.Status,
		Etcd:           r.Etcd,
		ContractSyncer: r.ContractSyncer,
	}
	if r.Status != "ok" {
		return apiserver.GetReady503JSONResponse(body), nil
	}
	return apiserver.GetReady200JSONResponse{Data: body}, nil
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *StrictOpenAPIServer) AuthorizeOidc(ctx context.Context, request apiserver.AuthorizeOidcRequestObject) (apiserver.AuthorizeOidcResponseObject, error) {
	fc, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	tctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	nonce := ""
	if request.Params.Nonce != nil {
		nonce = *request.Params.Nonce
	}

	loc, err := s.c.OIDCLoginUseCase.Start(tctx, auth.OIDCLoginStartRequest{
		ProviderID:          stringOrEmpty(request.Params.ProviderId),
		RedirectURI:         request.Params.RedirectUri,
		ServerCallbackURI:   fc.BaseURL() + "/v1/auth/callback",
		Nonce:               nonce,
		ResponseType:        string(request.Params.ResponseType),
		ClientID:            request.Params.ClientId,
		State:               stringOrEmpty(request.Params.State),
		CodeChallenge:       request.Params.CodeChallenge,
		CodeChallengeMethod: string(request.Params.CodeChallengeMethod),
	})
	if err != nil {
		st, code, detail := auth.MapStartError(err)
		switch st {
		case http.StatusBadRequest:
			p := problem.WithCode(st, code, "", detail)
			return apiserver.AuthorizeOidc400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		case http.StatusBadGateway:
			p := problem.BadGateway(code, "", detail)
			return apiserver.AuthorizeOidc502ApplicationProblemPlusJSONResponse{
				BadGatewayApplicationProblemPlusJSONResponse: apiserver.BadGatewayApplicationProblemPlusJSONResponse(p),
			}, nil
		case http.StatusServiceUnavailable:
			p := problem.ServiceUnavailable(code, "", detail)
			return apiserver.AuthorizeOidc503ApplicationProblemPlusJSONResponse{
				ServiceUnavailableApplicationProblemPlusJSONResponse: apiserver.ServiceUnavailableApplicationProblemPlusJSONResponse(p),
			}, nil
		default:
			p := internalProblem()
			return apiserver.AuthorizeOidc500ApplicationProblemPlusJSONResponse{
				InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
			}, nil
		}
	}

	return apiserver.AuthorizeOidc302Response{Headers: apiserver.AuthorizeOidc302ResponseHeaders{Location: loc}}, nil
}

func (s *StrictOpenAPIServer) CallbackOidc(ctx context.Context, request apiserver.CallbackOidcRequestObject) (apiserver.CallbackOidcResponseObject, error) {
	tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out, err := s.c.OIDCCallbackUseCase.CompleteWithResult(tctx, request.Params.Code, request.Params.State)
	if err != nil {
		st, code, detail := auth.MapCallbackError(err)
		switch st {
		case http.StatusBadRequest:
			p := problem.WithCode(st, code, "", detail)
			return apiserver.CallbackOidc400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		case http.StatusUnauthorized:
			p := problem.WithCode(st, code, "", detail)
			return apiserver.CallbackOidc401ApplicationProblemPlusJSONResponse{
				UnauthorizedApplicationProblemPlusJSONResponse: apiserver.UnauthorizedApplicationProblemPlusJSONResponse(p),
			}, nil
		default:
			p := internalProblem()
			return apiserver.CallbackOidc500ApplicationProblemPlusJSONResponse{
				InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
			}, nil
		}
	}

	if out.RedirectURL == "" {
		p := problem.WithCode(http.StatusInternalServerError, "INTERNAL_ERROR", "", "callback produced no redirect URL")
		return apiserver.CallbackOidc500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	return apiserver.CallbackOidc302Response{Headers: apiserver.CallbackOidc302ResponseHeaders{Location: out.RedirectURL}}, nil
}

func (s *StrictOpenAPIServer) ListOidcProviders(ctx context.Context, request apiserver.ListOidcProvidersRequestObject) (apiserver.ListOidcProvidersResponseObject, error) {
	rows := s.c.OIDCLoginUseCase.ListPublicOIDCProviders()
	out := make([]apiserver.OidcProviderDescriptor, len(rows))
	for i, r := range rows {
		out[i] = apiserver.OidcProviderDescriptor{Id: r.ID, Name: r.Name, Kind: r.Kind, Issuer: r.Issuer}
	}
	return apiserver.ListOidcProviders200JSONResponse{Data: out}, nil
}

func (s *StrictOpenAPIServer) ListAuthPermissions(ctx context.Context, request apiserver.ListAuthPermissionsRequestObject) (apiserver.ListAuthPermissionsResponseObject, error) {
	byID := make(map[string]string)
	for _, d := range permissions.ListDescriptors() {
		byID[d.ID] = d.Description
	}
	for _, roleRow := range s.c.RoleCatalog.ListRolePermissions() {
		for _, permissionID := range roleRow.Permissions {
			if _, ok := byID[permissionID]; ok {
				continue
			}
			byID[permissionID] = permissions.Describe(permissionID)
		}
	}

	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]apiserver.PermissionDescriptor, 0, len(ids))
	for _, id := range ids {
		out = append(out, apiserver.PermissionDescriptor{Id: id, Description: byID[id]})
	}
	return apiserver.ListAuthPermissions200JSONResponse{Data: out}, nil
}

func (s *StrictOpenAPIServer) ListAuthRoles(ctx context.Context, request apiserver.ListAuthRolesRequestObject) (apiserver.ListAuthRolesResponseObject, error) {
	roleRows := s.c.RoleCatalog.ListRolePermissions()
	out := make([]apiserver.RolePermissions, 0, len(roleRows))
	for i := range roleRows {
		out = append(out, apiserver.RolePermissions{
			Role:        roleRows[i].RoleID,
			Permissions: permissionDescriptorsFromIDs(roleRows[i].Permissions),
		})
	}
	return apiserver.ListAuthRoles200JSONResponse{Data: out}, nil
}

func (s *StrictOpenAPIServer) TokenOidc(ctx context.Context, request apiserver.TokenOidcRequestObject) (apiserver.TokenOidcResponseObject, error) {
	if s.c.OAuthTokenUseCase == nil {
		return apiserver.TokenOidc500JSONResponse{
			Error:            "server_error",
			ErrorDescription: stringPtr("OAuth token endpoint is not configured."),
		}, nil
	}

	body := request.Body
	if body == nil {
		return apiserver.TokenOidc400JSONResponse{
			Error:            "invalid_request",
			ErrorDescription: stringPtr("request body is required"),
		}, nil
	}

	grantType := strings.TrimSpace(string(body.GrantType))
	clientID := ""
	if body.ClientId != nil {
		clientID = strings.TrimSpace(*body.ClientId)
	}
	if clientID == "" {
		if fc, err := fiberCtxFromStrictContext(ctx); err == nil {
			clientID = clientIDFromBasicAuth(fc.Get(fiber.HeaderAuthorization))
			if grantType == "" {
				grantType = strings.TrimSpace(fc.Query("grant_type"))
			}
		}
	}

	req := auth.OAuthTokenRequest{
		GrantType:    grantType,
		Code:         stringOrEmpty(body.Code),
		RedirectURI:  stringOrEmpty(body.RedirectUri),
		ClientID:     clientID,
		CodeVerifier: stringOrEmpty(body.CodeVerifier),
		RefreshToken: stringOrEmpty(body.RefreshToken),
		AccessTTL:    durationFromOptionalFormSeconds(body.AccessTtl),
		RefreshTTL:   durationFromOptionalFormSeconds(body.RefreshTtl),
	}

	tctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	out, err := s.c.OAuthTokenUseCase.Exchange(tctx, req)
	if err != nil {
		status, oauthErr, description := auth.MapOAuthTokenError(err)
		resp := apiserver.OAuthTokenError{Error: oauthErr, ErrorDescription: stringPtr(description)}
		switch status {
		case http.StatusBadRequest:
			return apiserver.TokenOidc400JSONResponse(resp), nil
		case http.StatusServiceUnavailable:
			return apiserver.TokenOidc503JSONResponse(resp), nil
		default:
			return apiserver.TokenOidc500JSONResponse(resp), nil
		}
	}

	return apiserver.TokenOidc200JSONResponse{Data: out}, nil
}

func (s *StrictOpenAPIServer) InspectTokenPermissions(ctx context.Context, request apiserver.InspectTokenPermissionsRequestObject) (apiserver.InspectTokenPermissionsResponseObject, error) {
	if s.c.JWTUseCase == nil {
		p := internalProblem()
		return apiserver.InspectTokenPermissions500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	if request.Body == nil {
		p := problem.BadRequest(problem.CodeInvalidJSONBody, "", problem.DetailInvalidJSONBody)
		return apiserver.InspectTokenPermissions400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	rawToken := strings.TrimSpace(request.Body.Data.AccessToken)
	if rawToken == "" {
		p := problem.BadRequest("ACCESS_TOKEN_REQUIRED", "", "Field access_token is required.")
		return apiserver.InspectTokenPermissions400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	claims, err := s.c.JWTUseCase.ParseAndValidateAPIProfileBearerToken(rawToken)
	if err != nil {
		p := problem.Unauthorized("INVALID_ACCESS_TOKEN", "", "Provided access token is invalid or expired.")
		return apiserver.InspectTokenPermissions401ApplicationProblemPlusJSONResponse{
			UnauthorizedApplicationProblemPlusJSONResponse: apiserver.UnauthorizedApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	subject := claimString(claims, "sub")
	if subject == "" {
		subject = claimString(claims, "email")
	}

	tokenRoles := uniqueSortedStrings(httpauthz.NormalizeRolesValue(claims["roles"]))
	effective := s.c.RoleCatalog.ResolvePermissions(tokenRoles)
	for _, permissionID := range claimStrings(claims, "permissions") {
		effective[permissionID] = struct{}{}
	}
	for _, permissionID := range claimStrings(claims, "scopes") {
		effective[permissionID] = struct{}{}
	}

	permissionIDs := mapKeysSorted(effective)
	return apiserver.InspectTokenPermissions200JSONResponse{Data: apiserver.TokenPermissionsResponse{
		Subject:     subject,
		Roles:       tokenRoles,
		Permissions: permissionDescriptorsFromIDs(permissionIDs),
	}}, nil
}

func (s *StrictOpenAPIServer) ListBundleKeys(ctx context.Context, request apiserver.ListBundleKeysRequestObject) (apiserver.ListBundleKeysResponseObject, error) {
	items, next, hasMore, err := s.c.BundleReadUseCase.ListBundleKeys(ctx, request.Params.Limit, request.Params.Cursor)
	if err != nil {
		st, p := mapDomainProblem(err, http.StatusBadRequest)
		if st == http.StatusBadRequest {
			return apiserver.ListBundleKeys400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		return apiserver.ListBundleKeys500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	apiItems := make([]apiserver.Bundle, 0, len(items))
	for _, key := range items {
		b, berr := bundleFromCanonicalKey(key)
		if berr != nil {
			continue
		}
		apiItems = append(apiItems, b)
	}
	body := apiserver.BundleRefListResponse{Items: apiItems, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		p := internalProblem()
		return apiserver.ListBundleKeys500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if request.Params.IfNoneMatch != nil && ifNoneMatchMatches(*request.Params.IfNoneMatch, etag) {
		return apiserver.ListBundleKeys304Response{}, nil
	}
	out := apiserver.ListBundleKeys200JSONResponse{Headers: apiserver.ListBundleKeys200ResponseHeaders{ETag: etag}}
	out.Body.Data = body
	return out, nil
}

func (s *StrictOpenAPIServer) ListContractsInBundle(ctx context.Context, request apiserver.ListContractsInBundleRequestObject) (apiserver.ListContractsInBundleResponseObject, error) {
	bk, err := bundleKeyFromContractBundleParams(request.Params.BundleKey, request.Params.Repo, request.Params.Ref, request.Params.Path)
	if err != nil {
		p := problem.BadRequest(problem.CodeInvalidBundleQueryParams, "", problem.DetailInvalidBundleQueryParams)
		return apiserver.ListContractsInBundle400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	items, next, hasMore, err := s.c.BundleReadUseCase.ListContractNames(ctx, bk, request.Params.Limit, request.Params.Cursor)
	if err != nil {
		st, p := mapDomainProblem(err, http.StatusNotFound, http.StatusBadRequest)
		switch st {
		case http.StatusBadRequest:
			return apiserver.ListContractsInBundle400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		case http.StatusNotFound:
			return apiserver.ListContractsInBundle404ApplicationProblemPlusJSONResponse{
				NotFoundApplicationProblemPlusJSONResponse: apiserver.NotFoundApplicationProblemPlusJSONResponse(p),
			}, nil
		default:
			return apiserver.ListContractsInBundle500ApplicationProblemPlusJSONResponse{
				InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
			}, nil
		}
	}
	body := apiserver.ContractNameListResponse{Items: items, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		p := internalProblem()
		return apiserver.ListContractsInBundle500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if request.Params.IfNoneMatch != nil && ifNoneMatchMatches(*request.Params.IfNoneMatch, etag) {
		return apiserver.ListContractsInBundle304Response{}, nil
	}
	out := apiserver.ListContractsInBundle200JSONResponse{Headers: apiserver.ListContractsInBundle200ResponseHeaders{ETag: etag}}
	out.Body.Data = body
	return out, nil
}

func (s *StrictOpenAPIServer) GetContractInBundle(ctx context.Context, request apiserver.GetContractInBundleRequestObject) (apiserver.GetContractInBundleResponseObject, error) {
	bk, err := bundleKeyFromContractBundleParams(request.Params.BundleKey, request.Params.Repo, request.Params.Ref, request.Params.Path)
	if err != nil {
		p := problem.BadRequest(problem.CodeInvalidBundleQueryParams, "", problem.DetailInvalidBundleQueryParams)
		return apiserver.GetContractInBundle500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	cn, err := url.PathUnescape(string(request.ContractName))
	if err != nil {
		p := problem.BadRequest(problem.CodeInvalidContractNamePath, "", problem.DetailInvalidContractNamePath)
		return apiserver.GetContractInBundle500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	doc, err := s.c.BundleReadUseCase.GetContractDocument(ctx, bk, cn)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			p := problem.NotFound(problem.CodeContractNotInBundle, "", problem.DetailContractNotInBundle)
			return apiserver.GetContractInBundle404ApplicationProblemPlusJSONResponse{
				NotFoundApplicationProblemPlusJSONResponse: apiserver.NotFoundApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		_, p := mapDomainProblem(err)
		return apiserver.GetContractInBundle500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	etag, err := jsonETag(doc)
	if err != nil {
		p := internalProblem()
		return apiserver.GetContractInBundle500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if request.Params.IfNoneMatch != nil && ifNoneMatchMatches(*request.Params.IfNoneMatch, etag) {
		return apiserver.GetContractInBundle304Response{}, nil
	}
	out := apiserver.GetContractInBundle200JSONResponse{Headers: apiserver.GetContractInBundle200ResponseHeaders{ETag: etag}}
	out.Body.Data = doc
	return out, nil
}

func (s *StrictOpenAPIServer) syncBundleHTTPResult(ctx context.Context, req *apiserver.BundleSyncRequest) (*idempotency.HTTPResult, error) {
	force := req.Force != nil && *req.Force
	fromCache, snaps, err := s.c.BundleHTTPSyncUseCase.Sync(ctx, req.Repository, req.Ref, req.Bundle, force)
	if err != nil {
		st, p := mapContractPipelineProblem(err, http.StatusBadRequest, http.StatusBadGateway, http.StatusConflict, http.StatusInternalServerError)
		body, merr := json.Marshal(p)
		if merr != nil {
			return nil, merr
		}
		return &idempotency.HTTPResult{StatusCode: st, ContentType: problem.ContentType, Body: body}, nil
	}
	apiSnaps, err := snapshotsToAPI(snaps)
	if err != nil {
		p := internalProblem()
		body, merr := json.Marshal(p)
		if merr != nil {
			return nil, merr
		}
		return &idempotency.HTTPResult{StatusCode: http.StatusInternalServerError, ContentType: problem.ContentType, Body: body}, nil
	}
	body, err := json.Marshal(apiserver.SyncBundle200JSONResponse{Data: apiserver.BundleSyncResponse{FromCache: fromCache, Snapshots: apiSnaps}})
	if err != nil {
		return nil, err
	}
	return &idempotency.HTTPResult{StatusCode: http.StatusOK, ContentType: "application/json", Body: body}, nil
}

func (s *StrictOpenAPIServer) SyncBundle(ctx context.Context, request apiserver.SyncBundleRequestObject) (apiserver.SyncBundleResponseObject, error) {
	if request.Body == nil {
		p := problem.BadRequest(problem.CodeInvalidJSONBody, "", problem.DetailInvalidJSONBody)
		return apiserver.SyncBundle400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	req := request.Body.Data
	if req.Repository == "" || req.Ref == "" || req.Bundle == "" {
		p := problem.BadRequest(problem.CodeSyncBundleParamsRequired, "", problem.DetailSyncBundleParamsRequired)
		return apiserver.SyncBundle400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	var res *idempotency.HTTPResult
	var err error
	if request.Params.IdempotencyKey != nil && *request.Params.IdempotencyKey != "" && s.c.BundleSyncIdempotency != nil {
		if hash := idempotency.HashBundleSyncRequest(req); hash != "" {
			res, err = s.c.BundleSyncIdempotency.Execute(ctx, *request.Params.IdempotencyKey, hash, func() (*idempotency.HTTPResult, error) {
				return s.syncBundleHTTPResult(ctx, &req)
			})
			if errors.Is(err, idempotency.ErrConflict) {
				p := problem.Conflict(problem.CodeIdempotencyKeyMismatch, "", problem.DetailIdempotencyKeyMismatch)
				return apiserver.SyncBundle409ApplicationProblemPlusJSONResponse(p), nil
			}
			if err != nil {
				p := internalProblem()
				return apiserver.SyncBundle500ApplicationProblemPlusJSONResponse{
					InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
				}, nil
			}
		}
	}
	if res == nil {
		res, err = s.syncBundleHTTPResult(ctx, &req)
		if err != nil {
			p := internalProblem()
			return apiserver.SyncBundle500ApplicationProblemPlusJSONResponse{
				InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
			}, nil
		}
	}

	switch res.StatusCode {
	case http.StatusOK:
		var okResp apiserver.SyncBundle200JSONResponse
		if err := json.Unmarshal(res.Body, &okResp); err != nil {
			p := internalProblem()
			return apiserver.SyncBundle500ApplicationProblemPlusJSONResponse{
				InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		return okResp, nil
	case http.StatusBadRequest:
		var p apiserver.Problem
		if err := json.Unmarshal(res.Body, &p); err != nil {
			p = internalProblem()
		}
		return apiserver.SyncBundle400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	case http.StatusConflict:
		var p apiserver.Problem
		if err := json.Unmarshal(res.Body, &p); err != nil {
			p = internalProblem()
		}
		return apiserver.SyncBundle409ApplicationProblemPlusJSONResponse(p), nil
	case http.StatusBadGateway:
		var p apiserver.Problem
		if err := json.Unmarshal(res.Body, &p); err != nil {
			p = internalProblem()
		}
		return apiserver.SyncBundle502ApplicationProblemPlusJSONResponse(p), nil
	default:
		var p apiserver.Problem
		if err := json.Unmarshal(res.Body, &p); err != nil {
			p = internalProblem()
		}
		return apiserver.SyncBundle500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
}

func (s *StrictOpenAPIServer) ExportContracts(ctx context.Context, request apiserver.ExportContractsRequestObject) (apiserver.ExportContractsResponseObject, error) {
	if request.Body == nil {
		p := problem.BadRequest(problem.CodeInvalidJSONBody, "", problem.DetailInvalidJSONBody)
		return apiserver.ExportContracts400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	fc, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	have, perr := s.c.PermissionEvaluator.SubjectPermissions(fc)
	if perr != nil {
		p := internalProblem()
		return apiserver.ExportContracts500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if !hasPermission(have, permissions.ContractsExport) {
		p := problem.Forbidden(problem.CodeInsufficientPermissions, "", "The caller does not have any required permission for this operation.")
		return apiserver.ExportContracts403ApplicationProblemPlusJSONResponse{
			ForbiddenApplicationProblemPlusJSONResponse: apiserver.ForbiddenApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	req := request.Body.Data
	if req.Repository == "" || req.Ref == "" {
		p := problem.BadRequest(problem.CodeExportRepositoryRefRequired, "", problem.DetailExportRepositoryRefRequired)
		return apiserver.ExportContracts400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	exportUC := bundle.NewContractExportUseCase(s.c.ContractSyncerGRPC)
	path := stringOrEmpty(req.Path)
	contractName := stringOrEmpty(req.ContractName)
	files, err := exportUC.Export(ctx, req.Repository, req.Ref, path, contractName)
	if err != nil {
		st, p := mapContractPipelineProblem(err, http.StatusBadRequest, http.StatusBadGateway)
		switch st {
		case http.StatusBadRequest:
			return apiserver.ExportContracts400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		case http.StatusBadGateway:
			return apiserver.ExportContracts502ApplicationProblemPlusJSONResponse(p), nil
		default:
			return apiserver.ExportContracts500ApplicationProblemPlusJSONResponse{
				InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
			}, nil
		}
	}

	resp := apiserver.ExportContracts200JSONResponse{Data: apiserver.ContractsExportResponse{
		Files: make([]apiserver.ContractsExportFile, 0, len(files)),
	}}
	for i := range files {
		f := files[i]
		resp.Data.Files = append(resp.Data.Files, apiserver.ContractsExportFile{
			ContractName:  f.ContractName,
			SourcePath:    f.SourcePath,
			ContentBase64: base64.StdEncoding.EncodeToString(f.Content),
		})
	}
	return resp, nil
}

func (s *StrictOpenAPIServer) ListControllers(ctx context.Context, request apiserver.ListControllersRequestObject) (apiserver.ListControllersResponseObject, error) {
	items, next, hasMore, err := s.c.ControllerReadUseCase.ListControllers(ctx, request.Params.Limit, request.Params.Cursor)
	if err != nil {
		st, p := mapDomainProblem(err, http.StatusBadRequest)
		if st == http.StatusBadRequest {
			return apiserver.ListControllers400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		return apiserver.ListControllers500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	apiItems := make([]apiserver.Controller, 0, len(items))
	for i := range items {
		apiItems = append(apiItems, controllerToAPI(items[i]))
	}
	body := apiserver.ControllerListResponse{Items: apiItems, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		p := internalProblem()
		return apiserver.ListControllers500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if request.Params.IfNoneMatch != nil && ifNoneMatchMatches(*request.Params.IfNoneMatch, etag) {
		return apiserver.ListControllers304Response{}, nil
	}
	out := apiserver.ListControllers200JSONResponse{Headers: apiserver.ListControllers200ResponseHeaders{ETag: etag}}
	out.Body.Data = body
	return out, nil
}

func (s *StrictOpenAPIServer) GetController(ctx context.Context, request apiserver.GetControllerRequestObject) (apiserver.GetControllerResponseObject, error) {
	info, err := s.c.ControllerReadUseCase.GetController(ctx, string(request.ControllerId))
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			p := problem.NotFound(problem.CodeControllerNotFound, "", problem.DetailControllerNotFound)
			return apiserver.GetController404ApplicationProblemPlusJSONResponse{
				NotFoundApplicationProblemPlusJSONResponse: apiserver.NotFoundApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		_, p := mapDomainProblem(err)
		return apiserver.GetController500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	body := controllerToAPI(*info)
	etag, err := jsonETag(body)
	if err != nil {
		p := internalProblem()
		return apiserver.GetController500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if request.Params.IfNoneMatch != nil && ifNoneMatchMatches(*request.Params.IfNoneMatch, etag) {
		return apiserver.GetController304Response{}, nil
	}
	out := apiserver.GetController200JSONResponse{Headers: apiserver.GetController200ResponseHeaders{ETag: etag}}
	out.Body.Data = body
	return out, nil
}

func (s *StrictOpenAPIServer) GetControllerHeartbeat(ctx context.Context, request apiserver.GetControllerHeartbeatRequestObject) (apiserver.GetControllerHeartbeatResponseObject, error) {
	ts, err := s.c.ControllerReadUseCase.GetHeartbeat(ctx, string(request.ControllerId))
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			p := problem.NotFound(problem.CodeControllerHeartbeatNotFound, "", problem.DetailControllerHeartbeatNotFound)
			return apiserver.GetControllerHeartbeat404ApplicationProblemPlusJSONResponse{
				NotFoundApplicationProblemPlusJSONResponse: apiserver.NotFoundApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		_, p := mapDomainProblem(err)
		return apiserver.GetControllerHeartbeat500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	body := apiserver.ControllerHeartbeat{Timestamp: ts}
	etag, err := jsonETag(body)
	if err != nil {
		p := internalProblem()
		return apiserver.GetControllerHeartbeat500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if request.Params.IfNoneMatch != nil && ifNoneMatchMatches(*request.Params.IfNoneMatch, etag) {
		return apiserver.GetControllerHeartbeat304Response{}, nil
	}
	out := apiserver.GetControllerHeartbeat200JSONResponse{Headers: apiserver.GetControllerHeartbeat200ResponseHeaders{ETag: etag}}
	out.Body.Data = body
	return out, nil
}

func (s *StrictOpenAPIServer) ListSigningKeys(ctx context.Context, request apiserver.ListSigningKeysRequestObject) (apiserver.ListSigningKeysResponseObject, error) {
	keys := s.c.JWTUseCase.GetSigningKeys(ctx)
	outKeys := make([]apiserver.SigningKey, 0, len(keys))
	for i := range keys {
		outKeys = append(outKeys, apiserver.SigningKey{
			Kid:       keys[i].Kid,
			Algorithm: keys[i].Algorithm,
			Active:    keys[i].Active,
			CreatedAt: keys[i].CreatedAt,
		})
	}
	etagBody := struct {
		Data []apiserver.SigningKey `json:"data"`
	}{Data: outKeys}
	etag, err := jsonETag(etagBody)
	if err != nil {
		p := internalProblem()
		return apiserver.ListSigningKeys500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if request.Params.IfNoneMatch != nil && ifNoneMatchMatches(*request.Params.IfNoneMatch, etag) {
		return apiserver.ListSigningKeys304Response{}, nil
	}
	out := apiserver.ListSigningKeys200JSONResponse{Headers: apiserver.ListSigningKeys200ResponseHeaders{ETag: etag}}
	out.Body.Data = outKeys
	return out, nil
}

func (s *StrictOpenAPIServer) GetStatus(ctx context.Context, request apiserver.GetStatusRequestObject) (apiserver.GetStatusResponseObject, error) {
	etcd := s.c.StatusReadUseCase.CheckEtcd(ctx)
	syncer := s.c.StatusReadUseCase.CheckContractSyncer(ctx)
	body := apiserver.StatusResponse{
		ApiServer:      "ok",
		Etcd:           &etcd,
		ContractSyncer: &syncer,
	}
	etag, err := jsonETag(body)
	if err != nil {
		p := internalProblem()
		return apiserver.GetStatus500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if request.Params.IfNoneMatch != nil && ifNoneMatchMatches(*request.Params.IfNoneMatch, etag) {
		return apiserver.GetStatus304Response{}, nil
	}
	out := apiserver.GetStatus200JSONResponse{Headers: apiserver.GetStatus200ResponseHeaders{ETag: etag}}
	out.Body.Data = body
	return out, nil
}

func (s *StrictOpenAPIServer) ListTenants(ctx context.Context, request apiserver.ListTenantsRequestObject) (apiserver.ListTenantsResponseObject, error) {
	items, next, hasMore, err := s.c.TenantReadUseCase.ListTenants(ctx, request.Params.Limit, request.Params.Cursor)
	if err != nil {
		st, p := mapDomainProblem(err, http.StatusBadRequest)
		if st == http.StatusBadRequest {
			return apiserver.ListTenants400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		return apiserver.ListTenants500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	body := apiserver.TenantListResponse{Items: items, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		p := internalProblem()
		return apiserver.ListTenants500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if request.Params.IfNoneMatch != nil && ifNoneMatchMatches(*request.Params.IfNoneMatch, etag) {
		return apiserver.ListTenants304Response{}, nil
	}
	out := apiserver.ListTenants200JSONResponse{Headers: apiserver.ListTenants200ResponseHeaders{ETag: etag}}
	out.Body.Data = body
	return out, nil
}

func (s *StrictOpenAPIServer) ListBundlesByTenant(ctx context.Context, request apiserver.ListBundlesByTenantRequestObject) (apiserver.ListBundlesByTenantResponseObject, error) {
	items, next, hasMore, err := s.c.TenantReadUseCase.ListBundlesByTenant(ctx, string(request.Tenant), request.Params.Limit, request.Params.Cursor)
	if err != nil {
		st, p := mapDomainProblem(err, http.StatusBadRequest)
		if st == http.StatusBadRequest {
			return apiserver.ListBundlesByTenant400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		return apiserver.ListBundlesByTenant500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	apiItems := make([]apiserver.Bundle, 0, len(items))
	for i := range items {
		apiItems = append(apiItems, bundleToAPI(items[i]))
	}
	body := apiserver.BundleRefListResponse{Items: apiItems, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		p := internalProblem()
		return apiserver.ListBundlesByTenant500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if request.Params.IfNoneMatch != nil && ifNoneMatchMatches(*request.Params.IfNoneMatch, etag) {
		return apiserver.ListBundlesByTenant304Response{}, nil
	}
	out := apiserver.ListBundlesByTenant200JSONResponse{Headers: apiserver.ListBundlesByTenant200ResponseHeaders{ETag: etag}}
	out.Body.Data = body
	return out, nil
}

func (s *StrictOpenAPIServer) ListControllersByTenant(ctx context.Context, request apiserver.ListControllersByTenantRequestObject) (apiserver.ListControllersByTenantResponseObject, error) {
	items, next, hasMore, err := s.c.ControllerReadUseCase.ListControllersByTenant(ctx, string(request.Tenant), request.Params.Limit, request.Params.Cursor)
	if err != nil {
		st, p := mapDomainProblem(err, http.StatusBadRequest)
		if st == http.StatusBadRequest {
			return apiserver.ListControllersByTenant400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		return apiserver.ListControllersByTenant500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	apiItems := make([]apiserver.Controller, 0, len(items))
	for i := range items {
		apiItems = append(apiItems, controllerToAPI(items[i]))
	}
	body := apiserver.ControllerListResponse{Items: apiItems, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		p := internalProblem()
		return apiserver.ListControllersByTenant500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if request.Params.IfNoneMatch != nil && ifNoneMatchMatches(*request.Params.IfNoneMatch, etag) {
		return apiserver.ListControllersByTenant304Response{}, nil
	}
	out := apiserver.ListControllersByTenant200JSONResponse{Headers: apiserver.ListControllersByTenant200ResponseHeaders{ETag: etag}}
	out.Body.Data = body
	return out, nil
}

func (s *StrictOpenAPIServer) ListEnvironmentsByTenant(ctx context.Context, request apiserver.ListEnvironmentsByTenantRequestObject) (apiserver.ListEnvironmentsByTenantResponseObject, error) {
	items, next, hasMore, err := s.c.TenantReadUseCase.ListEnvironmentsByTenant(ctx, string(request.Tenant), request.Params.Limit, request.Params.Cursor)
	if err != nil {
		st, p := mapDomainProblem(err, http.StatusBadRequest)
		if st == http.StatusBadRequest {
			return apiserver.ListEnvironmentsByTenant400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		return apiserver.ListEnvironmentsByTenant500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	apiItems := make([]apiserver.Environment, 0, len(items))
	for i := range items {
		apiItems = append(apiItems, environmentToAPI(items[i]))
	}
	body := apiserver.EnvironmentListResponse{Items: apiItems, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		p := internalProblem()
		return apiserver.ListEnvironmentsByTenant500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if request.Params.IfNoneMatch != nil && ifNoneMatchMatches(*request.Params.IfNoneMatch, etag) {
		return apiserver.ListEnvironmentsByTenant304Response{}, nil
	}
	out := apiserver.ListEnvironmentsByTenant200JSONResponse{Headers: apiserver.ListEnvironmentsByTenant200ResponseHeaders{ETag: etag}}
	out.Body.Data = body
	return out, nil
}

func (s *StrictOpenAPIServer) IssueApiAccessToken(ctx context.Context, request apiserver.IssueApiAccessTokenRequestObject) (apiserver.IssueApiAccessTokenResponseObject, error) {
	fc, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}

	mc, ok := middleware.APIJWTClaimsFromCtx(fc)
	if !ok {
		p := problem.Forbidden("API_TOKEN_ISSUER_MUST_BE_HUMAN", "", "API access tokens can be issued only by an interactive human Bearer token.")
		return apiserver.IssueApiAccessToken403ApplicationProblemPlusJSONResponse{
			ForbiddenApplicationProblemPlusJSONResponse: apiserver.ForbiddenApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if !hasAnyRoleClaim(mc) {
		p := problem.Forbidden("API_TOKEN_ISSUER_MUST_BE_HUMAN", "", "API access tokens can be issued only by an interactive human Bearer token with role claims.")
		return apiserver.IssueApiAccessToken403ApplicationProblemPlusJSONResponse{
			ForbiddenApplicationProblemPlusJSONResponse: apiserver.ForbiddenApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	have, perr := s.c.PermissionEvaluator.SubjectPermissions(fc)
	if perr != nil {
		p := internalProblem()
		return apiserver.IssueApiAccessToken500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if !hasPermission(have, permissions.APIAccessTokenIssue) {
		p := problem.Forbidden(problem.CodeInsufficientPermissions, "", "The caller does not have any required permission for this operation.")
		return apiserver.IssueApiAccessToken403ApplicationProblemPlusJSONResponse{
			ForbiddenApplicationProblemPlusJSONResponse: apiserver.ForbiddenApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	var requestedPermissions []string
	var requestedExpiresAt *time.Time
	if request.Body != nil {
		requestedPermissions = normalizeRequestedPermissions(request.Body.Data.Permissions)
		requestedExpiresAt = request.Body.Data.ExpiresAt
		if !hasPermission(have, permissions.Wildcard) {
			for i := range requestedPermissions {
				if hasPermission(have, requestedPermissions[i]) {
					continue
				}
				p := problem.Forbidden(problem.CodeRequestedPermissionsNotAllowed, "", "The caller cannot delegate one or more requested permissions.")
				return apiserver.IssueApiAccessToken403ApplicationProblemPlusJSONResponse{
					ForbiddenApplicationProblemPlusJSONResponse: apiserver.ForbiddenApplicationProblemPlusJSONResponse(p),
				}, nil
			}
		}
	}

	subject := subjectFromAPIJWTClaims(mc)
	if subject == "" {
		p := problem.BadRequest("API_TOKEN_SUBJECT_MISSING", "", "Bearer token has no usable sub/email for API access issuance.")
		return apiserver.IssueApiAccessToken400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	now := time.Now().UTC()
	ttl, err := resolveIssuedAPIAccessTTL(now, config.EffectiveInteractiveAccessTokenTTL(s.c.Config.Auth.InteractiveAccessTokenTTL), mc, requestedExpiresAt)
	if err != nil {
		p := problem.BadRequest("API_TOKEN_EXPIRES_AT_INVALID", "", err.Error())
		return apiserver.IssueApiAccessToken400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	requestedAny := stringsToAny(requestedPermissions)
	basePermissions := permissionsFromAPIJWTClaims(mc)
	snap, err := snapshotForAPIAccess(mergeAnyUnique(basePermissions, requestedAny), mc)
	if err != nil {
		p := internalProblem()
		return apiserver.IssueApiAccessToken500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	token, _, exp, err := s.c.JWTUseCase.MintInteractiveAPIAccessJWTFromSnapshot(ctx, subject, snap, ttl)
	if err != nil {
		st, p := mapDomainProblem(err, http.StatusUnauthorized)
		if st == http.StatusUnauthorized {
			return apiserver.IssueApiAccessToken401ApplicationProblemPlusJSONResponse{
				UnauthorizedApplicationProblemPlusJSONResponse: apiserver.UnauthorizedApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		return apiserver.IssueApiAccessToken500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	out := apiserver.ApiAccessTokenIssued{AccessToken: token, ExpiresAt: exp}
	return apiserver.IssueApiAccessToken201JSONResponse{Data: out}, nil
}

func (s *StrictOpenAPIServer) IssueEdgeToken(ctx context.Context, request apiserver.IssueEdgeTokenRequestObject) (apiserver.IssueEdgeTokenResponseObject, error) {
	if request.Body == nil {
		p := problem.BadRequest(problem.CodeInvalidJSONBody, "", problem.DetailInvalidJSONBody)
		return apiserver.IssueEdgeToken400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	fc, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}

	have, perr := s.c.PermissionEvaluator.SubjectPermissions(fc)
	if perr != nil {
		p := internalProblem()
		return apiserver.IssueEdgeToken500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if !hasPermission(have, permissions.EdgeTokenIssue) {
		p := problem.Unauthorized(problem.CodeInsufficientPermissions, "", "The caller does not have any required permission for this operation.")
		return apiserver.IssueEdgeToken401ApplicationProblemPlusJSONResponse{
			UnauthorizedApplicationProblemPlusJSONResponse: apiserver.UnauthorizedApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	req := models.GenerateTokenRequest{
		AppID:        request.Body.Data.AppId,
		Environments: request.Body.Data.Environments,
		ExpiresAt:    request.Body.Data.ExpiresAt,
	}
	if req.AppID == "" {
		p := problem.BadRequest(problem.CodeTokenAppIDRequired, "", problem.DetailTokenAppIDRequired)
		return apiserver.IssueEdgeToken400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if len(req.Environments) == 0 {
		p := problem.BadRequest(problem.CodeTokenEnvironmentsRequired, "", problem.DetailTokenEnvironmentsRequired)
		return apiserver.IssueEdgeToken400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	for _, env := range req.Environments {
		if env == "" {
			p := problem.BadRequest(problem.CodeTokenEnvironmentEmpty, "", problem.DetailTokenEnvironmentEmpty)
			return apiserver.IssueEdgeToken400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		}
	}
	if req.ExpiresAt.Before(time.Now()) {
		p := problem.BadRequest(problem.CodeTokenExpiresAtPast, "", problem.DetailTokenExpiresAtPast)
		return apiserver.IssueEdgeToken400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	token, err := s.c.JWTUseCase.GenerateToken(ctx, &req)
	if err != nil {
		_, p := mapDomainProblem(err)
		return apiserver.IssueEdgeToken500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	return apiserver.IssueEdgeToken201JSONResponse{Data: apiserver.GenerateTokenResponse{
		Id:           token.ID,
		Token:        token.Token,
		AppId:        token.AppID,
		Environments: token.Environments,
		ExpiresAt:    token.ExpiresAt,
		CreatedAt:    token.CreatedAt,
	}}, nil
}

func (s *StrictOpenAPIServer) GetVersion(ctx context.Context, request apiserver.GetVersionRequestObject) (apiserver.GetVersionResponseObject, error) {
	body := apiserver.VersionResponse{
		ApiSchemaVersion: version.APISchemaVersion(),
		GitRevision:      version.GitRevision,
		BuildTime:        version.BuildTime,
	}
	if version.Release != "" {
		r := version.Release
		body.Release = &r
	}
	return apiserver.GetVersion200JSONResponse{Data: body}, nil
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

var _ apiserver.StrictServerInterface = (*StrictOpenAPIServer)(nil)
