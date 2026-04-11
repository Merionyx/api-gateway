package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/merionyx/api-gateway/internal/api-server/domain/errmapping"
)

const (
	TransportHTTP = "http"
	TransportGRPC = "grpc"

	// OutcomeUnmapped is used when no apierrors sentinel matched (same as generic 500 / gRPC Internal).
	OutcomeUnmapped = "unmapped"

	// OutcomeContractSyncPipelineFailed labels the default sync/export pipeline error (502).
	OutcomeContractSyncPipelineFailed = "ContractSyncPipelineFailed"
)

var domainErrorsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "api_server_domain_errors_total",
		Help: "Error responses classified by domain rules (errors.Is to apierrors sentinels), by transport.",
	},
	[]string{"transport", "outcome"},
)

// RecordDomainOutcome increments the domain error counter for generic HTTP / gRPC handlers using DomainRules().
func RecordDomainOutcome(enabled bool, transport string, err error) {
	if !enabled || err == nil {
		return
	}
	outcome := errmapping.DomainRuleName(err)
	if outcome == "" {
		outcome = OutcomeUnmapped
	}
	domainErrorsTotal.WithLabelValues(transport, outcome).Inc()
}

// RecordContractPipelineOutcome increments for sync/export pipeline responses (ContractPipelineRules + default).
func RecordContractPipelineOutcome(enabled bool, err error) {
	if !enabled || err == nil {
		return
	}
	outcome := errmapping.ContractPipelineRuleName(err)
	if outcome == "" {
		outcome = OutcomeContractSyncPipelineFailed
	}
	domainErrorsTotal.WithLabelValues(TransportHTTP, outcome).Inc()
}
