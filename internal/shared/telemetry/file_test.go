package telemetry

import (
	"testing"
)

func TestBuildConfig_FileEnables(t *testing.T) {
	enabled := true
	c := BuildConfig("api-server", FileBlock{Enabled: &enabled})
	if !c.Enabled {
		t.Fatal("file enabled must apply")
	}
}

func TestBuildConfig_FileOnlyNoEnv(t *testing.T) {
	enabled := true
	c := BuildConfig("api-server", FileBlock{
		Enabled:      &enabled,
		ServiceName:  "from-yaml",
		OTLPEndpoint: "http://c:4317",
	})
	if c.ServiceName != "from-yaml" {
		t.Fatalf("name: %q", c.ServiceName)
	}
	if c.OTELExporterOTLPEndpoint != "c:4317" {
		t.Fatalf("endpoint: %q", c.OTELExporterOTLPEndpoint)
	}
}

func TestBuildConfig_EnvOverridesFile(t *testing.T) {
	enabled := true
	t.Setenv(EnvTelemetryEnabled, "0")

	c := BuildConfig("x", FileBlock{Enabled: &enabled})
	if c.Enabled {
		t.Fatal("TELEMETRY_ENABLED=0 must override file true")
	}
}
