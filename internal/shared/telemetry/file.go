package telemetry

import (
	"os"
	"strings"
)

// FileBlock is the optional "telemetry" section in a service config file
// (Viper / mapstructure). It is merged with environment variables by
// [BuildConfig]: the file is applied first, then non-empty standard env vars
// override (so you can turn tracing on in k8s without editing the mounted YAML).
type FileBlock struct {
	// Enabled turns on OTLP export. If nil, the value is not taken from the
	// file (remains false until [applyEnvToConfig] or a default); set the
	// pointer explicitly to true or false in YAML.
	Enabled *bool `mapstructure:"enabled" json:"enabled,omitempty"`

	// ServiceName is the service.name resource attribute. If empty, [Config.BinaryName] is used.
	ServiceName string `mapstructure:"service_name" json:"service_name,omitempty"`

	// OTLPEndpoint is host:port (e.g. 127.0.0.1:4317) or a full URL
	// (http://otel-collector:4317) like OTEL_EXPORTER_OTLP_* env vars.
	OTLPEndpoint string `mapstructure:"otlp_endpoint" json:"otlp_endpoint,omitempty"`

	// OTLPInsecure disables TLS to the collector. If nil, a http:// URL in
	// OTLPEndpoint implies insecure; if set, it wins over URL inference.
	OTLPInsecure *bool `mapstructure:"otlp_insecure" json:"otlp_insecure,omitempty"`

	// OTLPProtocol: grpc, http, http/protobuf (default grpc when empty).
	OTLPProtocol string `mapstructure:"otlp_protocol" json:"otlp_protocol,omitempty"`
}

// BuildConfig returns [Config] for [Init] from a config file block and
// environment. Order: start from binaryName, apply file, then apply env
// overrides (see [ConfigFromEnv] for variable names). Env always wins for any
// variable that is set in the process environment.
func BuildConfig(binaryName string, file FileBlock) Config {
	c := Config{BinaryName: binaryName}
	applyFileToConfig(&c, file)
	applyEnvToConfig(&c)
	return c
}

func applyFileToConfig(c *Config, f FileBlock) {
	if f.Enabled != nil {
		c.Enabled = *f.Enabled
	}
	if f.ServiceName != "" {
		c.ServiceName = f.ServiceName
	}
	raw := strings.TrimSpace(f.OTLPEndpoint)
	if ep, _ := parseOTLPEndpoint(raw); ep != "" {
		c.OTELExporterOTLPEndpoint = ep
	}
	if f.OTLPInsecure != nil {
		c.OTLPInsecure = *f.OTLPInsecure
	} else if raw != "" {
		_, c.OTLPInsecure = parseOTLPEndpoint(raw)
	}
	if f.OTLPProtocol != "" {
		c.OTLPProtocol = f.OTLPProtocol
	}
}

// applyEnvToConfig overwrites c from standard env when each variable is set.
func applyEnvToConfig(c *Config) {
	if _, ok := os.LookupEnv(EnvTelemetryEnabled); ok {
		c.Enabled = parseBoolish(os.Getenv(EnvTelemetryEnabled), false)
	}
	if s := firstNonEmpty(os.Getenv("OTEL_SERVICE_NAME"), os.Getenv("SERVICE_NAME")); s != "" {
		c.ServiceName = s
	}
	if v := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"); v != "" {
		c.OTLPProtocol = v
	}
	raw := firstNonEmpty(
		strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")),
		strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
	)
	if raw == "" {
		return
	}
	if ep, _ := parseOTLPEndpoint(raw); ep != "" {
		c.OTELExporterOTLPEndpoint = ep
	}
	if v, set := os.LookupEnv("OTEL_EXPORTER_OTLP_INSECURE"); set {
		c.OTLPInsecure = parseBoolish(v, false)
	} else {
		_, c.OTLPInsecure = parseOTLPEndpoint(raw)
	}
}
