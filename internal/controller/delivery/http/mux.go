package httpdelivery

import (
	"encoding/json"
	"net/http"
)

// NewMux returns HTTP routes for the Gateway Controller. There is no REST control plane here —
// configuration is gRPC, xDS, and etcd; HTTP is limited to operational probes.
func NewMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
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
