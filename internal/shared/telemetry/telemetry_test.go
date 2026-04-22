package telemetry

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSpanName(t *testing.T) {
	t.Parallel()
	got := SpanName("internal/x/pkg", "Handler")
	if got != "internal/x/pkg.Handler" {
		t.Fatalf("SpanName: %q", got)
	}
}

func TestConfig_ResolvedServiceName(t *testing.T) {
	t.Parallel()
	if got := (Config{ServiceName: "api"}).ResolvedServiceName(); got != "api" {
		t.Fatalf("service: %q", got)
	}
	if got := (Config{BinaryName: "b"}).ResolvedServiceName(); got != "b" {
		t.Fatalf("binary: %q", got)
	}
}

func TestInit_Disabled(t *testing.T) {
	ctx := context.Background()
	shutdown, err := Init(ctx, Config{Enabled: false, BinaryName: "x"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = shutdown(ctx)
	})

	_, s := Start(ctx, SpanName("a", "b"))
	s.End()
}

func TestInit_BadOTLPProtocol(t *testing.T) {
	_, err := Init(context.Background(), Config{
		Enabled:      true,
		BinaryName:   "b",
		OTLPProtocol: "ftp",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMarkError(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sr),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
	})

	ctx := context.Background()
	_, span := Start(ctx, "t")
	MarkError(span, errors.New("boom"))
	span.End()

	ended := sr.Ended()
	if len(ended) != 1 {
		t.Fatalf("spans: %d", len(ended))
	}
	if ended[0].Status().Code != codes.Error {
		t.Fatalf("status: %#v", ended[0].Status())
	}
}
