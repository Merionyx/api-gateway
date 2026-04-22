package telemetry

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// Environment for OpenTelemetry in long-running services (the project uses
// [EnvTelemetryEnabled] in addition to standard OTel variables):
//
//   - TELEMETRY_ENABLED — "true" / "1" / "yes" / "on" enables export; unset or
//     anything else: noop tracer (default).
//   - OTEL_SERVICE_NAME — [semconv] service.name (optional); if empty, falls
//     back to [SERVICE_NAME], then to [Config.BinaryName].
//   - OTEL_EXPORTER_OTLP_TRACES_ENDPOINT — if set, overrides
//   - OTEL_EXPORTER_OTLP_ENDPOINT
//   - OTEL_EXPORTER_OTLP_PROTOCOL — grpc, http, http/protobuf (default grpc).
//   - OTEL_EXPORTER_OTLP_INSECURE — if set, "true" / "false"; if unset, a
//     http:// URL in the endpoint implies insecure; https:// does not.
//
// When a variable is unset, [ConfigFromEnv] does not change values that
// [BuildConfig] set from a [FileBlock] (unlike a plain read of the env, which
// is why [BuildConfig] is preferred in services).
//
// [semconv]: https://opentelemetry.io/docs/specs/semconv
func ConfigFromEnv(binaryName string) Config {
	return BuildConfig(binaryName, FileBlock{})
}

// EnvTelemetryEnabled is the name of the env that turns tracing export on for this module.
// OpenTelemetry has no single "enabled" flag; this is a project switch on top of OTLP.
const EnvTelemetryEnabled = "TELEMETRY_ENABLED"

// InstallFromConfig calls [BuildConfig] (file block from YAML, then env) and [Init].
// Pass an empty [FileBlock] to behave like [ConfigFromEnv] (env only).
func InstallFromConfig(ctx context.Context, binaryName string, file FileBlock) (func(context.Context) error, error) {
	return Init(ctx, BuildConfig(binaryName, file))
}

// InstallFromEnv calls [ConfigFromEnv] (env only) and [Init].
// Prefer [InstallFromConfig] in services with a Viper YAML config.
func InstallFromEnv(ctx context.Context, binaryName string) (func(context.Context) error, error) {
	return Init(ctx, ConfigFromEnv(binaryName))
}

// Shutdown is a best-effort flush with a fixed 5s timeout, suitable for defer
// in main. Logs a warning on failure and does not return an error to avoid
// complicating return paths; use the raw shutdown with your own context if
// you need to propagate errors.
func Shutdown(shut func(context.Context) error) {
	if shut == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shut(ctx); err != nil {
		slog.Warn("telemetry shutdown", "err", err)
	}
}

func firstNonEmpty(values ...string) string {
	for _, s := range values {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// parseBoolish is true for true/1/yes/y/on (case-insensitive, trimmed).
func parseBoolish(s string, def bool) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return def
	}
	switch s {
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	default:
		return def
	}
}

// parseOTLPEndpoint returns host:port for the OTLP client and true if the URL
// used http: (suggesting insecure, no TLS) when the value was a full URL.
// Bare "host:port" returns raw and insecure=false.
func parseOTLPEndpoint(raw string) (endpoint string, insecure bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if !strings.Contains(raw, "://") {
		return raw, false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw, false
	}
	if u.Host == "" {
		return raw, false
	}
	insecure = u.Scheme == "http"
	return u.Host, insecure
}
