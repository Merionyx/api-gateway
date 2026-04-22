package telemetry

import (
	"context"
	"testing"
)

func TestConfigFromEnv_TelemetryEnabled(t *testing.T) {
	t.Setenv(EnvTelemetryEnabled, "1")
	c := ConfigFromEnv("p")
	if !c.Enabled {
		t.Fatal("expected enabled")
	}
}

func TestConfigFromEnv_OtelServiceName(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "x")
	c := ConfigFromEnv("p")
	if c.ServiceName != "x" {
		t.Fatalf("service: %q", c.ServiceName)
	}
}

func TestConfigFromEnv_FallbackServiceName(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("SERVICE_NAME", "s")
	c := ConfigFromEnv("p")
	if c.ServiceName != "s" {
		t.Fatalf("service: %q", c.ServiceName)
	}
}

func TestConfigFromEnv_EndpointInsecure(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4317")
	c := ConfigFromEnv("b")
	if c.OTELExporterOTLPEndpoint != "collector:4317" {
		t.Fatalf("host:port: %q", c.OTELExporterOTLPEndpoint)
	}
	if !c.OTLPInsecure {
		t.Fatal("expected http URL -> insecure")
	}
}

func TestConfigFromEnv_EndpointInsecureOverride(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://x:1")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "false")
	c := ConfigFromEnv("b")
	if c.OTLPInsecure {
		t.Fatal("explicit false must win")
	}
}

func TestParseOTLPEndpoint_Bare(t *testing.T) {
	h, in := parseOTLPEndpoint("127.0.0.1:4317")
	if h != "127.0.0.1:4317" || in {
		t.Fatalf("%q, %v", h, in)
	}
}

func TestInstallFromConfig_DoesNotPanic(t *testing.T) {
	t.Setenv(EnvTelemetryEnabled, "false")
	_, err := InstallFromConfig(context.Background(), "t", FileBlock{})
	if err != nil {
		t.Fatal(err)
	}
}
