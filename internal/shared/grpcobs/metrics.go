package grpcobs

import (
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultMetricsPath = "/metrics"
)

var (
	serverHandled = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_server_handled_total",
			Help: "Total gRPC server calls completed by code.",
		},
		[]string{"grpc_method", "grpc_code"},
	)
	serverDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_server_handling_seconds",
			Help:    "Histogram of gRPC server call duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"grpc_method"},
	)
)

// RegisterMetricsHandler attaches Prometheus scrape on mux at path (default /metrics).
func RegisterMetricsHandler(mux *http.ServeMux, path string) {
	if mux == nil {
		return
	}
	p := strings.TrimSpace(path)
	if p == "" {
		p = defaultMetricsPath
	}
	mux.Handle(p, promhttp.Handler())
}
