package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	XDSResultOK    = "ok"
	XDSResultError = "error"

	SessionReasonCanceled = "canceled"
	SessionReasonError    = "error"
)

var (
	xdsSnapshotUpdates = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_controller_xds_snapshot_updates_total",
			Help: "xDS snapshot cache SetSnapshot calls (excludes no-op same-version skips).",
		},
		[]string{"result"},
	)
	apiServerSessionEnds = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_controller_api_server_session_ends_total",
			Help: "API Server gRPC sync session outcomes before reconnect or shutdown.",
		},
		[]string{"reason"},
	)
)

// RecordXDSnapshotUpdate increments when a snapshot is written to the cache.
func RecordXDSnapshotUpdate(enabled bool, result string) {
	if !enabled {
		return
	}
	xdsSnapshotUpdates.WithLabelValues(result).Inc()
}

// RecordAPIServerSessionEnd increments when a sync session ends (error or canceled).
func RecordAPIServerSessionEnd(enabled bool, reason string) {
	if !enabled {
		return
	}
	apiServerSessionEnds.WithLabelValues(reason).Inc()
}
