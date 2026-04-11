package metrics

import (
	"errors"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

func TestRecordDomainOutcome(t *testing.T) {
	t.Parallel()
	RecordDomainOutcome(false, TransportHTTP, errors.New("x")) // no-op
	RecordDomainOutcome(true, TransportHTTP, nil)              // no-op
	RecordDomainOutcome(true, TransportGRPC, apierrors.ErrNotFound)
	RecordDomainOutcome(true, TransportHTTP, errors.New("unmapped"))
}

func TestRecordContractPipelineOutcome(t *testing.T) {
	t.Parallel()
	RecordContractPipelineOutcome(false, errors.New("x"))
	RecordContractPipelineOutcome(true, nil)
	RecordContractPipelineOutcome(true, errors.New("pipeline"))
	RecordContractPipelineOutcome(true, apierrors.ErrContractSyncerRejected)
}
