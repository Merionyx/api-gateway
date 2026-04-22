package telemetry

import (
	"net/http"
	"strings"
)

// SkipProbePath reports request paths (URL path only) for which a root HTTP server span
// is usually not wanted: high-frequency liveness/readiness-style probes. Matches /health
// and /ready exactly and paths with a /health or /ready suffix (e.g. /v1/ready).
func SkipProbePath(path string) bool {
	if path == "" {
		return false
	}
	if path == "/health" || path == "/ready" {
		return true
	}
	if len(path) > len("/health") && strings.HasSuffix(path, "/health") {
		return true
	}
	if len(path) > len("/ready") && strings.HasSuffix(path, "/ready") {
		return true
	}
	return false
}

// WrapHandlerHTTP adds a server span per request: W3C extract, [Start], [SetHTTPStatus], End.
// The span name is "{METHOD} {raw path}" (for std [net/http] there is no route template).
// If skip is non-nil and skip(r) is true, the next handler runs without a new span
// (e.g. use [SkipProbePath] for liveness/readiness).
func WrapHandlerHTTP(h http.Handler, skip func(*http.Request) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if skip != nil && skip(r) {
			h.ServeHTTP(w, r)
			return
		}
		ctx := ExtractIncomingHTTP(r.Context(), r.Header)
		name := r.Method + " " + r.URL.Path
		ctx, span := Start(ctx, name)
		defer span.End()
		rw := &captureStatus{ResponseWriter: w}
		h.ServeHTTP(rw, r.WithContext(ctx))
		SetHTTPStatus(span, rw.status())
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
// 200 if the handler never set one.
func (c *captureStatus) status() int {
	if c.code == 0 {
		return http.StatusOK
	}
	return c.code
}
