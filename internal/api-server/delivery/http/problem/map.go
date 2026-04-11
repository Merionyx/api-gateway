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
	case errors.Is(err, apierrors.ErrContractSyncerRejected):
		// Default for bare rejection; prefer FromContractSyncPipeline for sync/export.
		return http.StatusBadRequest, BadRequest("", err.Error())
	default:
		return http.StatusInternalServerError, Internal(err.Error())
	}
}

// FromContractSyncPipeline maps Contract Syncer call errors: rejected → 400, else → 502.
func FromContractSyncPipeline(err error) (status int, p apiserver.Problem) {
	if err == nil {
		return http.StatusOK, apiserver.Problem{}
	}
	if errors.Is(err, apierrors.ErrContractSyncerRejected) {
		return http.StatusBadRequest, BadRequest("", err.Error())
	}
	return http.StatusBadGateway, BadGateway(err.Error())
}
