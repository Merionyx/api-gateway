package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/idempotency"
	apimetrics "github.com/merionyx/api-gateway/internal/api-server/metrics"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/bundle"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/registry"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

// RegistryHandler implements registry / bundle / tenant HTTP operations from the OpenAPI spec.
type RegistryHandler struct {
	bundles     *bundle.BundleReadUseCase
	controllers *registry.ControllerReadUseCase
	tenants     *registry.TenantReadUseCase
	sync        *bundle.BundleHTTPSyncUseCase
	status      *registry.StatusReadUseCase
	// readinessRequireContractSyncer enables Contract Syncer in GET /ready (etcd is always required).
	readinessRequireContractSyncer bool
	bundleSyncIdempotency          *idempotency.Store
}

func NewRegistryHandler(
	bundles *bundle.BundleReadUseCase,
	controllers *registry.ControllerReadUseCase,
	tenants *registry.TenantReadUseCase,
	sync *bundle.BundleHTTPSyncUseCase,
	status *registry.StatusReadUseCase,
	readinessRequireContractSyncer bool,
	bundleSyncIdempotency *idempotency.Store,
) *RegistryHandler {
	return &RegistryHandler{
		bundles:                        bundles,
		controllers:                    controllers,
		tenants:                        tenants,
		sync:                           sync,
		status:                         status,
		readinessRequireContractSyncer: readinessRequireContractSyncer,
		bundleSyncIdempotency:          bundleSyncIdempotency,
	}
}

func (h *RegistryHandler) ListBundleKeys(c fiber.Ctx, params apiserver.ListBundleKeysParams) error {
	items, next, hasMore, err := h.bundles.ListBundleKeys(c.Context(), params.Limit, params.Cursor)
	if err != nil {
		return problem.RespondError(c, err)
	}
	body := apiserver.BundleKeyListResponse{Items: items, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return problem.RespondError(c, err)
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
	return c.JSON(body)
}

func (h *RegistryHandler) SyncBundle(c fiber.Ctx, params apiserver.SyncBundleParams) error {
	var req apiserver.BundleSyncRequest
	if err := c.Bind().Body(&req); err != nil {
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeInvalidJSONBody, "", problem.DetailInvalidJSONBody))
	}
	if req.Repository == "" || req.Ref == "" || req.Bundle == "" {
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeSyncBundleParamsRequired, "", problem.DetailSyncBundleParamsRequired))
	}

	if params.IdempotencyKey != nil && *params.IdempotencyKey != "" && h.bundleSyncIdempotency != nil {
		if hash := idempotency.HashBundleSyncRequest(req); hash != "" {
			res, err := h.bundleSyncIdempotency.Execute(*params.IdempotencyKey, hash, func() (*idempotency.HTTPResult, error) {
				return h.syncBundleHTTPResult(c, &req)
			})
			if errors.Is(err, idempotency.ErrConflict) {
				return problem.Write(c, http.StatusConflict, problem.Conflict(problem.CodeIdempotencyKeyMismatch, "", problem.DetailIdempotencyKeyMismatch))
			}
			if err != nil {
				return problem.WriteInternal(c, err)
			}
			return writeCachedHTTPResult(c, res)
		}
	}

	res, err := h.syncBundleHTTPResult(c, &req)
	if err != nil {
		return problem.WriteInternal(c, err)
	}
	return writeCachedHTTPResult(c, res)
}

func writeCachedHTTPResult(c fiber.Ctx, res *idempotency.HTTPResult) error {
	c.Response().Header.Set("Content-Type", res.ContentType)
	return c.Status(res.StatusCode).Send(res.Body)
}

func (h *RegistryHandler) syncBundleHTTPResult(c fiber.Ctx, req *apiserver.BundleSyncRequest) (*idempotency.HTTPResult, error) {
	force := req.Force != nil && *req.Force
	fromCache, snaps, err := h.sync.Sync(c.Context(), req.Repository, req.Ref, req.Bundle, force)
	if err != nil {
		apimetrics.RecordContractPipelineOutcome(apimetrics.MetricsEnabledFromCtx(c), err)
		st, p := problem.FromContractSyncPipeline(err)
		logHTTPProblem(st, &p, err)
		body, merr := json.Marshal(p)
		if merr != nil {
			return nil, merr
		}
		return &idempotency.HTTPResult{StatusCode: st, ContentType: problem.ContentType, Body: body}, nil
	}
	apiSnaps, err := snapshotsToAPI(snaps)
	if err != nil {
		apimetrics.RecordDomainOutcome(apimetrics.MetricsEnabledFromCtx(c), apimetrics.TransportHTTP, err)
		st, p := problem.FromDomain(err)
		logHTTPProblem(st, &p, err)
		body, merr := json.Marshal(p)
		if merr != nil {
			return nil, merr
		}
		return &idempotency.HTTPResult{StatusCode: st, ContentType: problem.ContentType, Body: body}, nil
	}
	body, err := json.Marshal(apiserver.BundleSyncResponse{FromCache: fromCache, Snapshots: apiSnaps})
	if err != nil {
		return nil, err
	}
	return &idempotency.HTTPResult{StatusCode: http.StatusOK, ContentType: "application/json", Body: body}, nil
}

func logHTTPProblem(st int, p *apiserver.Problem, cause error) {
	code := ""
	if p != nil && p.Code != nil {
		code = *p.Code
	}
	slog.Error("http problem response", "status", st, "code", code, "err", cause)
}

func (h *RegistryHandler) ListContractsInBundle(c fiber.Ctx, bundleKey apiserver.BundleKey, params apiserver.ListContractsInBundleParams) error {
	bk, err := url.PathUnescape(string(bundleKey))
	if err != nil {
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeInvalidBundleKeyPath, "", problem.DetailInvalidBundleKeyPath))
	}
	items, next, hasMore, err := h.bundles.ListContractNames(c.Context(), bk, params.Limit, params.Cursor)
	if err != nil {
		return problem.RespondError(c, err)
	}
	body := apiserver.ContractNameListResponse{Items: items, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return problem.RespondError(c, err)
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
	return c.JSON(body)
}

func (h *RegistryHandler) GetContractInBundle(c fiber.Ctx, bundleKey apiserver.BundleKey, contractName apiserver.ContractName, params apiserver.GetContractInBundleParams) error {
	bk, err := url.PathUnescape(string(bundleKey))
	if err != nil {
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeInvalidBundleKeyPath, "", problem.DetailInvalidBundleKeyPath))
	}
	cn, err := url.PathUnescape(string(contractName))
	if err != nil {
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeInvalidContractNamePath, "", problem.DetailInvalidContractNamePath))
	}
	doc, err := h.bundles.GetContractDocument(c.Context(), bk, cn)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			apimetrics.RecordDomainOutcome(apimetrics.MetricsEnabledFromCtx(c), apimetrics.TransportHTTP, err)
			return problem.Write(c, http.StatusNotFound, problem.NotFound(problem.CodeContractNotInBundle, "", problem.DetailContractNotInBundle))
		}
		return problem.RespondError(c, err)
	}
	etag, err := jsonETag(doc)
	if err != nil {
		return problem.RespondError(c, err)
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
	return c.JSON(doc)
}

func (h *RegistryHandler) ListControllers(c fiber.Ctx, params apiserver.ListControllersParams) error {
	items, next, hasMore, err := h.controllers.ListControllers(c.Context(), params.Limit, params.Cursor)
	if err != nil {
		return problem.RespondError(c, err)
	}
	apiItems := make([]apiserver.Controller, 0, len(items))
	for i := range items {
		apiItems = append(apiItems, controllerToAPI(items[i]))
	}
	body := apiserver.ControllerListResponse{Items: apiItems, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return problem.RespondError(c, err)
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
	return c.JSON(body)
}

func (h *RegistryHandler) GetController(c fiber.Ctx, controllerID apiserver.ControllerId, params apiserver.GetControllerParams) error {
	info, err := h.controllers.GetController(c.Context(), string(controllerID))
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			apimetrics.RecordDomainOutcome(apimetrics.MetricsEnabledFromCtx(c), apimetrics.TransportHTTP, err)
			return problem.Write(c, http.StatusNotFound, problem.NotFound(problem.CodeControllerNotFound, "", problem.DetailControllerNotFound))
		}
		return problem.RespondError(c, err)
	}
	body := controllerToAPI(*info)
	etag, err := jsonETag(body)
	if err != nil {
		return problem.RespondError(c, err)
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
	return c.JSON(body)
}

func (h *RegistryHandler) GetControllerHeartbeat(c fiber.Ctx, controllerID apiserver.ControllerId, params apiserver.GetControllerHeartbeatParams) error {
	ts, err := h.controllers.GetHeartbeat(c.Context(), string(controllerID))
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			apimetrics.RecordDomainOutcome(apimetrics.MetricsEnabledFromCtx(c), apimetrics.TransportHTTP, err)
			return problem.Write(c, http.StatusNotFound, problem.NotFound(problem.CodeControllerHeartbeatNotFound, "", problem.DetailControllerHeartbeatNotFound))
		}
		return problem.RespondError(c, err)
	}
	body := apiserver.ControllerHeartbeat{Timestamp: ts}
	etag, err := jsonETag(body)
	if err != nil {
		return problem.RespondError(c, err)
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
	return c.JSON(body)
}

func (h *RegistryHandler) GetReady(c fiber.Ctx) error {
	r := h.status.Readiness(c.Context(), h.readinessRequireContractSyncer)
	body := apiserver.ReadinessStatus{
		Status:         r.Status,
		Etcd:           r.Etcd,
		ContractSyncer: r.ContractSyncer,
	}
	if r.Status != "ok" {
		return c.Status(http.StatusServiceUnavailable).JSON(body)
	}
	return c.JSON(body)
}

func (h *RegistryHandler) GetStatus(c fiber.Ctx, params apiserver.GetStatusParams) error {
	etcd := h.status.CheckEtcd(c.Context())
	syncer := h.status.CheckContractSyncer(c.Context())
	body := apiserver.StatusResponse{
		ApiServer:      "ok",
		Etcd:           &etcd,
		ContractSyncer: &syncer,
	}
	etag, err := jsonETag(body)
	if err != nil {
		return problem.RespondError(c, err)
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
	return c.JSON(body)
}

func (h *RegistryHandler) ListTenants(c fiber.Ctx, params apiserver.ListTenantsParams) error {
	items, next, hasMore, err := h.tenants.ListTenants(c.Context(), params.Limit, params.Cursor)
	if err != nil {
		return problem.RespondError(c, err)
	}
	body := apiserver.TenantListResponse{Items: items, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return problem.RespondError(c, err)
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
	return c.JSON(body)
}

func (h *RegistryHandler) ListBundlesByTenant(c fiber.Ctx, tenant apiserver.Tenant, params apiserver.ListBundlesByTenantParams) error {
	items, next, hasMore, err := h.tenants.ListBundlesByTenant(c.Context(), string(tenant), params.Limit, params.Cursor)
	if err != nil {
		return problem.RespondError(c, err)
	}
	apiItems := make([]apiserver.Bundle, 0, len(items))
	for i := range items {
		apiItems = append(apiItems, bundleToAPI(items[i]))
	}
	body := apiserver.BundleRefListResponse{Items: apiItems, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return problem.RespondError(c, err)
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
	return c.JSON(body)
}

func (h *RegistryHandler) ListControllersByTenant(c fiber.Ctx, tenant apiserver.Tenant, params apiserver.ListControllersByTenantParams) error {
	items, next, hasMore, err := h.controllers.ListControllersByTenant(c.Context(), string(tenant), params.Limit, params.Cursor)
	if err != nil {
		return problem.RespondError(c, err)
	}
	apiItems := make([]apiserver.Controller, 0, len(items))
	for i := range items {
		apiItems = append(apiItems, controllerToAPI(items[i]))
	}
	body := apiserver.ControllerListResponse{Items: apiItems, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return problem.RespondError(c, err)
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
	return c.JSON(body)
}

func (h *RegistryHandler) ListEnvironmentsByTenant(c fiber.Ctx, tenant apiserver.Tenant, params apiserver.ListEnvironmentsByTenantParams) error {
	items, next, hasMore, err := h.tenants.ListEnvironmentsByTenant(c.Context(), string(tenant), params.Limit, params.Cursor)
	if err != nil {
		return problem.RespondError(c, err)
	}
	apiItems := make([]apiserver.Environment, 0, len(items))
	for i := range items {
		apiItems = append(apiItems, environmentToAPI(items[i]))
	}
	body := apiserver.EnvironmentListResponse{Items: apiItems, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return problem.RespondError(c, err)
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
	return c.JSON(body)
}

func controllerToAPI(c models.ControllerInfo) apiserver.Controller {
	envs := make([]apiserver.Environment, 0, len(c.Environments))
	for _, e := range c.Environments {
		bundles := make([]apiserver.Bundle, 0, len(e.Bundles))
		for _, b := range e.Bundles {
			nm := b.Name
			repo := b.Repository
			ref := b.Ref
			path := b.Path
			bundles = append(bundles, apiserver.Bundle{
				Name:       &nm,
				Repository: &repo,
				Ref:        &ref,
				Path:       &path,
			})
		}
		envs = append(envs, apiserver.Environment{Name: e.Name, Bundles: &bundles})
	}
	return apiserver.Controller{
		ControllerId: c.ControllerID,
		Tenant:       c.Tenant,
		Environments: &envs,
	}
}

func environmentToAPI(e models.EnvironmentInfo) apiserver.Environment {
	bundles := make([]apiserver.Bundle, 0, len(e.Bundles))
	for _, b := range e.Bundles {
		nm := b.Name
		repo := b.Repository
		ref := b.Ref
		path := b.Path
		bundles = append(bundles, apiserver.Bundle{
			Name:       &nm,
			Repository: &repo,
			Ref:        &ref,
			Path:       &path,
		})
	}
	return apiserver.Environment{Name: e.Name, Bundles: &bundles}
}

func bundleToAPI(b models.BundleInfo) apiserver.Bundle {
	nm := b.Name
	repo := b.Repository
	ref := b.Ref
	path := b.Path
	return apiserver.Bundle{
		Name:       &nm,
		Repository: &repo,
		Ref:        &ref,
		Path:       &path,
	}
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
