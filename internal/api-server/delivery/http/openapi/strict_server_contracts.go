package openapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/idempotency"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/bundle"
)

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
