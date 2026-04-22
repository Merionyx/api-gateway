package telemetry

import (
	"context"
	"net/http"
	"testing"
)

func TestExtractIncomingHTTP_Empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if got := ExtractIncomingHTTP(ctx, nil); got != ctx {
		t.Fatalf("nil header: want same context")
	}
	if got := ExtractIncomingHTTP(ctx, http.Header{}); got != ctx {
		t.Fatalf("empty header: want same context")
	}
}

func TestExtractIncomingHTTP_WithTraceparent_DoesNotPanic(t *testing.T) {
	t.Parallel()
	setTextMapPropagator() // like other tests: ensure TraceContext is registered
	h := make(http.Header)
	// 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
	h.Set("Traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	_ = ExtractIncomingHTTP(context.Background(), h)
}
