package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	RebuildPhaseInitial   = "initial"
	RebuildPhaseDebounced = "debounced"
)

var (
	etcdWatchEvents = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "gateway_controller_etcd_watch_events_total",
			Help: "etcd watch events received (one increment per event in a batch).",
		},
	)
	etcdWatchErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "gateway_controller_etcd_watch_errors_total",
			Help: "etcd watch channel errors.",
		},
	)
	xdsRebuildFlushes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_controller_xds_rebuild_flushes_total",
			Help: "xDS full rebuild from etcd runs after follower watch.",
		},
		[]string{"phase", "result"},
	)
	xdsRebuildDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_controller_xds_rebuild_duration_seconds",
			Help:    "Duration of rebuildAllXDS.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"phase"},
	)
)

// AddEtcdWatchEvents adds n event observations (enabled).
func AddEtcdWatchEvents(enabled bool, n int) {
	if !enabled || n <= 0 {
		return
	}
	etcdWatchEvents.Add(float64(n))
}

// RecordEtcdWatchError increments on watch response error.
func RecordEtcdWatchError(enabled bool) {
	if !enabled {
		return
	}
	etcdWatchErrors.Inc()
}

// RecordXDSRebuildFlush records outcome of a rebuildAllXDS run.
func RecordXDSRebuildFlush(enabled bool, phase, result string) {
	if !enabled {
		return
	}
	xdsRebuildFlushes.WithLabelValues(phase, result).Inc()
}

// ObserveXDSRebuildDuration records rebuild duration.
func ObserveXDSRebuildDuration(enabled bool, phase string, d time.Duration) {
	if !enabled {
		return
	}
	xdsRebuildDuration.WithLabelValues(phase).Observe(d.Seconds())
}
