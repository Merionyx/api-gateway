// Package telemetry configures OpenTelemetry tracing for services in this
// module: global TracerProvider, OTLP export, W3C TraceContext propagation, and
// small helpers for manual span naming and error status.
//
// # Initialization
//
// Call [Init] once at process startup. When [Config.Enabled] is false, a noop
// TracerProvider is used (no export, minimal overhead). Use the returned
// shutdown on exit.
//
// # Handlers
//
// In each gRPC or HTTP handler, start a span and pass the returned
// [context.Context] to downstream calls for correlation:
//
//	ctx, span := telemetry.Start(ctx, telemetry.SpanName("path/to/pkg", "MyHandler"))
//	defer span.End()
//
// # Span names
//
// Use [SpanName] with the Go import path (or a stable path segment) and the
// function or method name, e.g. telemetry.SpanName("internal/.../usecase/bundle", "Sync").
//
// # Errors
//
// After an error, call [MarkError] before the span ends (e.g. before return):
//
//	if err != nil {
//	    telemetry.MarkError(span, err)
//	}
//	return err
package telemetry
