// Package telemetry configures OpenTelemetry tracing for services in this
// module: global TracerProvider, OTLP export, W3C TraceContext propagation, and
// small helpers for manual span naming and error status.
//
// # Initialization
//
// Call [Init] with [BuildConfig] (YAML [FileBlock] from Viper, then environment)
// or [ConfigFromEnv] (environment only) once at process startup. When
// [Config.Enabled] is false, a noop TracerProvider is used. Use the returned
// shutdown or [Shutdown] on exit. [EnvTelemetryEnabled] in the environment
// overrides a config file for the same key.
//
// Default service name: pass a stable id as the first argument to [BuildConfig]
// (e.g. "api-server", "controller"); it is [Config.BinaryName] and the default
// for [Config.ResolvedServiceName] when [OTEL_SERVICE_NAME] and the
// `SERVICE_NAME` environment variable are not set.
//
// # gRPC
//
// On the server, use [ServerSpan] (or [ExtractIncomingGRPC] then [Start]) at the
// start of each method. On the client, [Start] then [OutgoingContextWithTrace]
// with the gRPC call context.
//
// # HTTP
//
// At the start of a request, use [ExtractIncomingHTTP] on the request headers, then
// [Start] (or a framework middleware that does both). [SetHTTPStatus] is optional
// for recording the response code before [trace.Span.End].
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
// Use [SpanName] with the path of the package within the module (no module
// prefix) and the function or method name, e.g. telemetry.SpanName("internal/.../usecase/bundle", "Sync").
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
