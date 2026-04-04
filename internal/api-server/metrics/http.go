package metrics

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_server_http_requests_total",
			Help: "Total HTTP requests handled by API Server.",
		},
		[]string{"method", "route", "status"},
	)
	httpDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_server_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status"},
	)
)

// HTTPMiddleware records request counts and latency when enabled.
func HTTPMiddleware(enabled bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		if !enabled {
			return c.Next()
		}
		start := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()
		if status == 0 {
			status = fiber.StatusInternalServerError
		}
		route := routeLabel(c)
		method := c.Method()
		statusStr := strconv.Itoa(status)
		httpRequests.WithLabelValues(method, route, statusStr).Inc()
		httpDuration.WithLabelValues(method, route, statusStr).Observe(time.Since(start).Seconds())
		return err
	}
}

func routeLabel(c fiber.Ctx) string {
	if !c.Matched() {
		return "unmatched"
	}
	p := c.FullPath()
	if p == "" {
		return "unknown"
	}
	return p
}
