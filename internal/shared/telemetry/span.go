package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationScope is the name of the [trace.Tracer] this package returns.
// Spans in application code should use stable names; see [SpanName].
const instrumentationScope = "github.com/merionyx/api-gateway/internal/shared/telemetry"

// Tracer returns the global OpenTelemetry [trace.Tracer] (after [Init]).
func Tracer() trace.Tracer {
	return otel.Tracer(instrumentationScope)
}

// Start begins a new span. Typical usage:
//
//	ctx, span := telemetry.Start(ctx, telemetry.SpanName("path/to/pkg", "Handler"))
//	defer span.End()
func Start(ctx context.Context, spanName string) (context.Context, trace.Span) {
	return Tracer().Start(ctx, spanName)
}

// SpanName returns a span name in the form "{importPath}.{funcName}".
func SpanName(importPath, funcName string) string {
	return importPath + "." + funcName
}

// MarkError sets [codes.Error] on the span if err is non-nil. It does not call
// [trace.Span.End]; keep using defer [trace.Span.End] in the same scope.
// Do not set attributes on the span; only status, per project convention.
func MarkError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.SetStatus(codes.Error, err.Error())
}
