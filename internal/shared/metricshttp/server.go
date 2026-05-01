// Package metricshttp serves Prometheus metrics on a dedicated HTTP listener.
//
// Business and library metrics registered on prometheus.DefaultRegisterer (including promauto)
// are exposed on the configured path together with Go runtime and process collectors.
package metricshttp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/merionyx/api-gateway/internal/shared/serviceapp"
)

var registerDefaults sync.Once

// registerCollector skips registration if the same metrics are already on DefaultRegisterer
// (Fiber, controller-runtime, or other deps may register Go/process collectors first).
func registerCollector(reg prometheus.Registerer, c prometheus.Collector) {
	if err := reg.Register(c); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			return
		}
		slog.Debug("metricshttp: optional collector not registered", "error", err)
	}
}

func registerDefaultCollectors() {
	registerDefaults.Do(func() {
		reg := prometheus.DefaultRegisterer
		registerCollector(reg, collectors.NewGoCollector())
		registerCollector(reg, collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	})
}

// ListenAndServe blocks, serving /metrics (or cfg.Path) on cfg.Host:cfg.Port.
// It is a no-op when cfg.Enabled is false.
func ListenAndServe(cfg Config) error {
	return ListenAndServeUntil(context.Background(), cfg)
}

// ListenAndServeUntil serves until ctx is cancelled, then shuts down gracefully.
// It is a no-op when cfg.Enabled is false.
func ListenAndServeUntil(ctx context.Context, cfg Config) error {
	if !cfg.Enabled {
		return nil
	}
	registerDefaultCollectors()

	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		path = "/metrics"
	}
	port := strings.TrimSpace(cfg.Port)
	if port == "" {
		port = "9090"
	}
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		host = "0.0.0.0"
	}

	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())

	addr := net.JoinHostPort(host, port)
	slog.Info("metrics HTTP listening", "addr", addr, "path", path)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := serviceapp.RunHTTPServerUntil(ctx, srv, addr); err != nil {
		return fmt.Errorf("metrics http %s: %w", addr, err)
	}
	return nil
}
