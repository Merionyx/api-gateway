package httpdelivery

import (
	"encoding/json"
	"net/http"

	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// NewMux returns HTTP routes for the Gateway Controller: operational probes only (no REST control plane).
func NewMux() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", telemetry.WrapHandlerHTTP(http.HandlerFunc(health), nil))
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
