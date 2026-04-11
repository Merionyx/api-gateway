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
		return http.StatusNotFound, NotFound("", err.Error())
	case errors.Is(err, apierrors.ErrInvalidInput):
		return http.StatusBadRequest, BadRequest("", err.Error())
	case errors.Is(err, apierrors.ErrContractSyncerRejected):
		return http.StatusBadRequest, BadRequest("", err.Error())
	case errors.Is(err, apierrors.ErrNoActiveSigningKey):
		return http.StatusServiceUnavailable, ServiceUnavailable(err.Error())
	case errors.Is(err, apierrors.ErrUnsupportedSigningAlgorithm),
		errors.Is(err, apierrors.ErrSigningOperationFailed):
		return http.StatusInternalServerError, Internal(err.Error())
	case errors.Is(err, apierrors.ErrStoreAccess):
		return http.StatusServiceUnavailable, ServiceUnavailable(err.Error())
	case errors.Is(err, apierrors.ErrContractSyncerUnavailable):
		return http.StatusBadGateway, BadGateway(err.Error())
	default:
		return http.StatusInternalServerError, Internal(err.Error())
	}
}

// FromContractSyncPipeline maps Contract Syncer call errors: rejected → 400, store → 503, syncer transport → 502.
func FromContractSyncPipeline(err error) (status int, p apiserver.Problem) {
	if err == nil {
		return http.StatusOK, apiserver.Problem{}
	}
	switch {
	case errors.Is(err, apierrors.ErrContractSyncerRejected):
		return http.StatusBadRequest, BadRequest("", err.Error())
	case errors.Is(err, apierrors.ErrStoreAccess):
		return http.StatusServiceUnavailable, ServiceUnavailable(err.Error())
	case errors.Is(err, apierrors.ErrContractSyncerUnavailable):
		return http.StatusBadGateway, BadGateway(err.Error())
	default:
		return http.StatusBadGateway, BadGateway(err.Error())
	}
}
