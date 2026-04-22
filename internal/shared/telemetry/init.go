package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/trace/noop"
)

// Init installs the global TracerProvider and W3C TraceContext propagation.
// When cfg.Enabled is false, a noop provider is set and shutdown is a no-op.
// When cfg.Enabled is true, OTLP gRPC (default) or HTTP export is configured.
// Call the returned function on exit to flush and release resources.
func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	setTextMapPropagator()
	if !cfg.Enabled {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	proto, err := cfg.normalizedOTLPProtocol()
	if err != nil {
		return nil, err
	}

	svcName := cfg.ResolvedServiceName()
	if svcName == "" {
		return nil, fmt.Errorf("telemetry: empty service name")
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(semconv.ServiceNameKey.String(svcName)),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	switch proto {
	case OTLPProtocolHTTP:
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cfg.otlpEndpoint())}
		if cfg.OTLPInsecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		e, err := otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("telemetry: otlp http exporter: %w", err)
		}
		exporter = e
	default:
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.otlpEndpoint())}
		if cfg.OTLPInsecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		e, err := otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("telemetry: otlp grpc exporter: %w", err)
		}
		exporter = e
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

func setTextMapPropagator() {
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
		),
	)
}
