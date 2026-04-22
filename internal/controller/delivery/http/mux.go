package httpdelivery

import (
	"encoding/json"
	"net/http"

	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// NewMux returns HTTP routes for the Gateway Controller: operational probes only (no REST control plane).
func NewMux() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", withHTTPTrace("internal/controller/delivery/http", http.HandlerFunc(health)))
	return mux
}

// withHTTPTrace is a minimal W3C TraceContext + request span for std [net/http] handlers
// (extract parent, [telemetry.Start], pass ctx on the request, record status, end).
func withHTTPTrace(pkg string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := telemetry.ExtractIncomingHTTP(r.Context(), r.Header)
		name := telemetry.SpanName(pkg, r.Method+" "+r.URL.Path)
		ctx, span := telemetry.Start(ctx, name)
		defer span.End()
		rw := &captureStatus{ResponseWriter: w}
		h.ServeHTTP(rw, r.WithContext(ctx))
		telemetry.SetHTTPStatus(span, rw.status())
	})
}

type captureStatus struct {
	http.ResponseWriter
	code int
}

func (c *captureStatus) WriteHeader(status int) {
	c.code = status
	c.ResponseWriter.WriteHeader(status)
}

// status is the first status passed to [http.ResponseWriter.WriteHeader], or
// 200 if the handler never set one (e.g. JSON body with implicit 200).
func (c *captureStatus) status() int {
	if c.code == 0 {
		return http.StatusOK
	}
	return c.code
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
