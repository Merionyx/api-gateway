package problem

import (
	"errors"
	"net/http"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

// FromDomain maps known domain sentinel errors. Unknown errors become 500 Internal.
func FromDomain(err error) (status int, p apiserver.Problem) {
	if err == nil {
		return http.StatusOK, apiserver.Problem{}
	}
	switch {
	case errors.Is(err, apierrors.ErrNotFound):
		return http.StatusNotFound, NotFound(CodeNotFound, "", DetailNotFound)
	case errors.Is(err, apierrors.ErrInvalidInput):
		return http.StatusBadRequest, BadRequest(CodeInvalidInput, "", DetailInvalidInput)
	case errors.Is(err, apierrors.ErrContractSyncerRejected):
		return http.StatusBadRequest, BadRequest(CodeContractSyncerRejected, "", DetailContractSyncRejected(err))
	case errors.Is(err, apierrors.ErrNoActiveSigningKey):
		return http.StatusServiceUnavailable, ServiceUnavailable(CodeNoActiveSigningKey, "", DetailNoActiveSigningKey)
	case errors.Is(err, apierrors.ErrUnsupportedSigningAlgorithm):
		return http.StatusInternalServerError, InternalError(CodeUnsupportedSigningAlgorithm, "", DetailUnsupportedSigningAlgorithm)
	case errors.Is(err, apierrors.ErrSigningOperationFailed):
		return http.StatusInternalServerError, InternalError(CodeSigningOperationFailed, "", DetailSigningOperationFailed)
	case errors.Is(err, apierrors.ErrStoreAccess):
		return http.StatusServiceUnavailable, ServiceUnavailable(CodeStoreUnavailable, "", DetailStoreUnavailable)
	case errors.Is(err, apierrors.ErrContractSyncerUnavailable):
		return http.StatusBadGateway, BadGateway(CodeContractSyncerUnavailable, "", DetailContractSyncerUnavailable)
	default:
		return http.StatusInternalServerError, InternalError(CodeInternalError, "", DetailInternalError)
	}
}

// FromContractSyncPipeline maps Contract Syncer call errors: rejected → 400, store → 503, syncer transport → 502.
func FromContractSyncPipeline(err error) (status int, p apiserver.Problem) {
	if err == nil {
		return http.StatusOK, apiserver.Problem{}
	}
	switch {
	case errors.Is(err, apierrors.ErrContractSyncerRejected):
		return http.StatusBadRequest, BadRequest(CodeContractSyncerRejected, "", DetailContractSyncRejected(err))
	case errors.Is(err, apierrors.ErrStoreAccess):
		return http.StatusServiceUnavailable, ServiceUnavailable(CodeStoreUnavailable, "", DetailStoreUnavailable)
	case errors.Is(err, apierrors.ErrContractSyncerUnavailable):
		return http.StatusBadGateway, BadGateway(CodeContractSyncerUnavailable, "", DetailContractSyncerUnavailable)
	default:
		return http.StatusBadGateway, BadGateway(CodeContractSyncPipelineFailed, "", DetailContractSyncPipelineFailed)
	}
}
