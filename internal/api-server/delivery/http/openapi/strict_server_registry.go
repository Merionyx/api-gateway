package openapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

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
