package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	// AuthRefreshIDPUp is a successful refresh using the IdP token endpoint (roadmap ш. 17).
	AuthRefreshIDPUp = "idp_up"
	// AuthRefreshDegraded is a successful refresh without IdP (roadmap ш. 18).
	AuthRefreshDegraded = "degraded"
)

var authRefreshTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "api_server_auth_refresh_total",
		Help: "OAuth refresh_token grant on POST /v1/auth/token successful outcomes (idp_up vs degraded).",
	},
	[]string{"result"},
)

// RecordAuthRefresh increments refresh outcome when metrics are enabled (roadmap ш. 18, 25).
func RecordAuthRefresh(enabled bool, result string) {
	if !enabled {
		return
	}
	authRefreshTotal.WithLabelValues(result).Inc()
}
