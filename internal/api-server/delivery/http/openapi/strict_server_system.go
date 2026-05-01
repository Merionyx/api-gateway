package openapi

import (
	"context"
	"net/http"

	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/version"
)

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
