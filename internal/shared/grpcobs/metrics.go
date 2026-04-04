package grpcobs

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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
