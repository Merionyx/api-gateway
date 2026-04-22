package telemetry

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// ExtractIncomingHTTP returns ctx with remote W3C TraceContext from headers
// (e.g. traceparent) applied on top of ctx. Pass [http.Request.Header] from the
// incoming request; a nil or empty map is a no-op.
func ExtractIncomingHTTP(ctx context.Context, h http.Header) context.Context {
	if len(h) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(h))
}

// SetHTTPStatus records HTTP response status on the server span. It does not
// call [trace.Span.End]. Status >= 500 is [codes.Error]; else [codes.Ok].
// Use when the response status is known, typically after the handler returns.
func SetHTTPStatus(span trace.Span, status int) {
	if status >= 500 {
		span.SetStatus(codes.Error, http.StatusText(status))
		return
	}
	span.SetStatus(codes.Ok, "")
}
