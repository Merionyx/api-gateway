package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	OutcomeOK            = "ok"
	OutcomeResponseError = "response_error"
)

var (
	syncRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "contract_syncer_sync_requests_total",
			Help: "Contract Syncer Sync RPC outcomes (business errors use response_error).",
		},
		[]string{"outcome"},
	)
	syncDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "contract_syncer_sync_duration_seconds",
			Help:    "Duration of Sync use case execution.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"outcome"},
	)
)

// RecordSync records outcome and duration when metrics are enabled.
func RecordSync(enabled bool, outcome string, elapsed time.Duration) {
	if !enabled {
		return
	}
	syncRequests.WithLabelValues(outcome).Inc()
	syncDuration.WithLabelValues(outcome).Observe(elapsed.Seconds())
}
