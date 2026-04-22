package telemetry

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrapHandlerHTTP_SkipHealth(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

	skip := func(r *http.Request) bool { return r.URL.Path == "/health" }
	h := WrapHandlerHTTP(mux, skip)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if got := rr.Code; got != http.StatusOK {
		t.Fatalf("health: status %d", got)
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api", nil))
	if got := rr2.Code; got != http.StatusNoContent {
		t.Fatalf("api: status %d", got)
	}
}
