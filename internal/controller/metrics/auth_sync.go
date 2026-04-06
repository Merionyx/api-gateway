package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	authBuildAccessConfigDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "gateway_controller_auth_build_access_config_duration_seconds",
			Help:    "Time to build access config for one environment (auth sync).",
			Buckets: prometheus.DefBuckets,
		},
	)
	bundleEnvIndexRebuilds = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "gateway_controller_bundle_env_index_rebuilds_total",
			Help: "Bundle→environment index rebuilds.",
		},
	)
)

// ObserveAuthBuildAccessConfig records duration when metrics enabled.
func ObserveAuthBuildAccessConfig(enabled bool, d time.Duration) {
	if !enabled {
		return
	}
	authBuildAccessConfigDuration.Observe(d.Seconds())
}

// RecordBundleEnvIndexRebuild increments when index is rebuilt.
func RecordBundleEnvIndexRebuild(enabled bool) {
	if !enabled {
		return
	}
	bundleEnvIndexRebuilds.Inc()
}
