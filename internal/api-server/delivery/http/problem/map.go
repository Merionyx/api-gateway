package problem

import (
	"net/http"

	"github.com/merionyx/api-gateway/internal/api-server/domain/errmapping"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

// FromDomain maps known domain sentinel errors via errmapping. Unknown errors become 500 Internal.
func FromDomain(err error) (status int, p apiserver.Problem) {
	if err == nil {
		return http.StatusOK, apiserver.Problem{}
	}
	st, code, detail := errmapping.ResolveDomainProblem(err)
	return st, WithCode(st, code, "", detail)
}

// FromContractSyncPipeline maps Contract Syncer / etcd pipeline errors via errmapping.
func FromContractSyncPipeline(err error) (status int, p apiserver.Problem) {
	if err == nil {
		return http.StatusOK, apiserver.Problem{}
	}
	st, code, detail := errmapping.ResolveContractPipelineProblem(err)
	return st, WithCode(st, code, "", detail)
}
