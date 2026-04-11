package handler

import (
	"encoding/json"
	"errors"
	"net/url"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
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
}

func NewRegistryHandler(
	bundles *bundle.BundleReadUseCase,
	controllers *registry.ControllerReadUseCase,
	tenants *registry.TenantReadUseCase,
	sync *bundle.BundleHTTPSyncUseCase,
	status *registry.StatusReadUseCase,
) *RegistryHandler {
	return &RegistryHandler{
		bundles:     bundles,
		controllers: controllers,
		tenants:     tenants,
		sync:        sync,
		status:      status,
	}
}

func (h *RegistryHandler) ListBundleKeys(c fiber.Ctx, params apiserver.ListBundleKeysParams) error {
	items, next, hasMore, err := h.bundles.ListBundleKeys(c.Context(), params.Limit, params.Cursor)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	body := apiserver.BundleKeyListResponse{Items: items, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
	return c.JSON(body)
}

func (h *RegistryHandler) SyncBundle(c fiber.Ctx, _ apiserver.SyncBundleParams) error {
	var req apiserver.BundleSyncRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(problemBadRequest("invalid request body"))
	}
	if req.Repository == "" || req.Ref == "" || req.Bundle == "" {
		return c.Status(fiber.StatusBadRequest).JSON(problemBadRequest("repository, ref, and bundle are required"))
	}
	force := req.Force != nil && *req.Force
	fromCache, snaps, err := h.sync.Sync(c.Context(), req.Repository, req.Ref, req.Bundle, force)
	if err != nil {
		if errors.Is(err, apierrors.ErrContractSyncerRejected) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	apiSnaps, err := snapshotsToAPI(snaps)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	return c.JSON(apiserver.BundleSyncResponse{
		FromCache: fromCache,
		Snapshots: apiSnaps,
	})
}

func (h *RegistryHandler) ListContractsInBundle(c fiber.Ctx, bundleKey apiserver.BundleKey, params apiserver.ListContractsInBundleParams) error {
	bk, err := url.PathUnescape(string(bundleKey))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(problemBadRequest("invalid bundle_key"))
	}
	items, next, hasMore, err := h.bundles.ListContractNames(c.Context(), bk, params.Limit, params.Cursor)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	body := apiserver.ContractNameListResponse{Items: items, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
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
		return c.Status(fiber.StatusBadRequest).JSON(problemBadRequest("invalid bundle_key"))
	}
	cn, err := url.PathUnescape(string(contractName))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(problemBadRequest("invalid contract_name"))
	}
	doc, err := h.bundles.GetContractDocument(c.Context(), bk, cn)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(problemNotFound("contract not found in bundle"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	etag, err := jsonETag(doc)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
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
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	apiItems := make([]apiserver.Controller, 0, len(items))
	for i := range items {
		apiItems = append(apiItems, controllerToAPI(items[i]))
	}
	body := apiserver.ControllerListResponse{Items: apiItems, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
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
			return c.Status(fiber.StatusNotFound).JSON(problemNotFound("controller not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	body := controllerToAPI(*info)
	etag, err := jsonETag(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
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
			return c.Status(fiber.StatusNotFound).JSON(problemNotFound("controller heartbeat not found"))
		}
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	body := apiserver.ControllerHeartbeat{Timestamp: ts}
	etag, err := jsonETag(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
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
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
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
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	body := apiserver.TenantListResponse{Items: items, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
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
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	apiItems := make([]apiserver.Bundle, 0, len(items))
	for i := range items {
		apiItems = append(apiItems, bundleToAPI(items[i]))
	}
	body := apiserver.BundleRefListResponse{Items: apiItems, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
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
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	apiItems := make([]apiserver.Controller, 0, len(items))
	for i := range items {
		apiItems = append(apiItems, controllerToAPI(items[i]))
	}
	body := apiserver.ControllerListResponse{Items: apiItems, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
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
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	apiItems := make([]apiserver.Environment, 0, len(items))
	for i := range items {
		apiItems = append(apiItems, environmentToAPI(items[i]))
	}
	body := apiserver.EnvironmentListResponse{Items: apiItems, HasMore: hasMore, NextCursor: next}
	etag, err := jsonETag(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(problemInternal(err.Error()))
	}
	if params.IfNoneMatch != nil && ifNoneMatchMatches(*params.IfNoneMatch, etag) {
		c.Response().Header.Set("ETag", etag)
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Response().Header.Set("ETag", etag)
	return c.JSON(body)
}

func problemBadRequest(detail string) apiserver.Problem {
	st := fiber.StatusBadRequest
	return apiserver.Problem{Title: "Bad Request", Status: st, Detail: &detail}
}

func problemNotFound(detail string) apiserver.Problem {
	st := fiber.StatusNotFound
	return apiserver.Problem{Title: "Not Found", Status: st, Detail: &detail}
}

func problemInternal(detail string) apiserver.Problem {
	st := fiber.StatusInternalServerError
	return apiserver.Problem{Title: "Internal Server Error", Status: st, Detail: &detail}
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
