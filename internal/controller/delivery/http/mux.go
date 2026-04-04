package httpdelivery

import (
	"encoding/json"
	"net/http"
	"strings"

	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/shared/grpcobs"
)

// NewMux returns HTTP routes for the Gateway Controller. There is no REST control plane here —
// configuration is gRPC, xDS, and etcd; HTTP is limited to operational probes and optional /metrics.
func NewMux(cfg *config.Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
	if cfg != nil {
		cp := cfg.GRPCControlPlane.Observability
		xds := cfg.GRPCXDS.Observability
		if cp.MetricsEnabled || xds.MetricsEnabled {
			path := strings.TrimSpace(cp.MetricsPath)
			if path == "" {
				path = "/metrics"
			}
			grpcobs.RegisterMetricsHandler(mux, path)
		}
	}
	return mux
}

func health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "gateway-controller",
	})
}
