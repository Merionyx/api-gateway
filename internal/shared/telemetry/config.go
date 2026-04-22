package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OTLP export protocols supported by this package.
const (
	OTLPProtocolGRPC = "grpc"
	OTLPProtocolHTTP = "http"
)

// Config controls OpenTelemetry for a process. It is a plain value object; the
// caller (CLI/config layer) is expected to set fields from environment or
// static defaults (e.g. [DefaultConfig] for tests).
type Config struct {
	// Enabled turns on OTLP export. When false, a noop TracerProvider is
	// installed: no network and no span data retained.
	Enabled bool

	// ServiceName is the value of the service.name resource attribute. If empty,
	// BinaryName is used, then the basename of os.Args[0] (see ResolvedServiceName).
	ServiceName string

	// BinaryName is typically filepath.Base(os.Args[0]) from the service main.
	// It is the fallback for ServiceName.
	BinaryName string

	// OTELExporterOTLPEndpoint is host:port (no scheme), e.g. 127.0.0.1:4317 for gRPC
	// and 127.0.0.1:4318 for HTTP, unless EmptyEndpoint defaults apply.
	OTELExporterOTLPEndpoint string

	// OTLPInsecure disables TLS for the OTLP client (typical in dev and cluster-local collectors).
	OTLPInsecure bool

	// OTLPProtocol is OTLPProtocolGRPC or OTLPProtocolHTTP (or http/protobuf, case-insensitive).
	// Empty defaults to gRPC.
	OTLPProtocol string
}

// ResolvedServiceName returns the effective service.name for the resource.
func (c Config) ResolvedServiceName() string {
	if c.ServiceName != "" {
		return c.ServiceName
	}
	if c.BinaryName != "" {
		return c.BinaryName
	}
	if len(os.Args) > 0 {
		return filepath.Base(os.Args[0])
	}
	return "unknown"
}

func (c Config) otlpEndpoint() string {
	if c.OTELExporterOTLPEndpoint != "" {
		return c.OTELExporterOTLPEndpoint
	}
	proto, _ := c.normalizedOTLPProtocol()
	if proto == "http" {
		return "127.0.0.1:4318"
	}
	return "127.0.0.1:4317"
}

func (c Config) normalizedOTLPProtocol() (string, error) {
	s := strings.ToLower(strings.TrimSpace(c.OTLPProtocol))
	switch s {
	case "", "grpc":
		return OTLPProtocolGRPC, nil
	case "http", "http/protobuf":
		return OTLPProtocolHTTP, nil
	default:
		return "", fmt.Errorf("telemetry: unsupported OTLP protocol %q (want grpc, http, or http/protobuf)", c.OTLPProtocol)
	}
}
