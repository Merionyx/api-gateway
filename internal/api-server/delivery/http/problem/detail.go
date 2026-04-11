package problem

import (
	"github.com/merionyx/api-gateway/internal/api-server/domain/errmapping"
)

// DetailContractSyncRejected returns client-facing detail for CONTRACT_SYNCER_REJECTED (delegates to errmapping).
func DetailContractSyncRejected(err error) string {
	return errmapping.DetailContractSyncRejected(err)
}
