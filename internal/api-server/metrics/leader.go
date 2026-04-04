package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var apiServerLeader = promauto.NewGauge(
	prometheus.GaugeOpts{
		Name: "api_server_leader",
		Help: "1 if this API Server replica holds leader election, else 0.",
	},
)

// SetLeader sets api_server_leader to 1 or 0 when metrics enabled.
func SetLeader(enabled bool, isLeader bool) {
	if !enabled {
		return
	}
	if isLeader {
		apiServerLeader.Set(1)
	} else {
		apiServerLeader.Set(0)
	}
}
