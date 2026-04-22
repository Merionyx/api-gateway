package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	registryEnvironmentsBuildWarnings = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_controller_registry_environments_build_warnings_total",
			Help: "Degraded steps while building controller_registry environments (partial lists, skip env, materialized get, list snapshots).",
		},
		[]string{"kind"},
	)
)

// RecordRegistryEnvironmentsBuildWarning counts one degraded step. kind — стабильная метка, см. usecase.
func RecordRegistryEnvironmentsBuildWarning(enabled bool, kind string) {
	if !enabled || kind == "" {
		return
	}
	registryEnvironmentsBuildWarnings.WithLabelValues(kind).Inc()
}
