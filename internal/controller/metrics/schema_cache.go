package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var schemaListCache = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "gateway_controller_schema_list_contract_snapshots_cache_total",
		Help: "ListContractSnapshots cache lookups.",
	},
	[]string{"result"},
)

// RecordSchemaListCacheHit records hit=true or miss=false when metrics are enabled.
func RecordSchemaListCacheHit(enabled bool, hit bool) {
	if !enabled {
		return
	}
	if hit {
		schemaListCache.WithLabelValues("hit").Inc()
	} else {
		schemaListCache.WithLabelValues("miss").Inc()
	}
}
