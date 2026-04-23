package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// IdP access cache event labels (ADR 0002 §5, roadmap ш. 25). Low cardinality only — never tokens or session_id.
const (
	IdpAccessCacheHit        = "hit"
	IdpAccessCacheMiss       = "miss"
	IdpAccessCachePut        = "put"
	IdpAccessCacheInvalidate = "invalidate"
)

var idpAccessCacheEvents = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "api_server_auth_idp_access_cache_events_total",
		Help: "IdP access token in-memory cache: lookups (hit|miss) and mutations (put|invalidate).",
	},
	[]string{"result"},
)

// RecordIdpAccessCacheEvent increments cache events when metrics are enabled.
func RecordIdpAccessCacheEvent(enabled bool, result string) {
	if !enabled {
		return
	}
	idpAccessCacheEvents.WithLabelValues(result).Inc()
}
