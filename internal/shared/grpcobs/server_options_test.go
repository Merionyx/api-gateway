package grpcobs

import (
	"strings"
	"testing"
)

func TestServerOptions_NoTLSConfig(t *testing.T) {
	opts, err := ServerOptions(nil, ObservabilityConfig{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts) < 2 {
		t.Fatalf("expected interceptors, got %d options", len(opts))
	}
}

func TestServerOptions_TLSDisabledPointer(t *testing.T) {
	disabled := ServerTLSConfig{Enabled: false}
	opts, err := ServerOptions(&disabled, ObservabilityConfig{LogRequests: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts) < 2 {
		t.Fatal("expected options")
	}
}

func TestServerOptions_TLSLoadError(t *testing.T) {
	bad := ServerTLSConfig{Enabled: true, CertFile: "/nonexistent/a.pem", KeyFile: "/nonexistent/b.pem"}
	_, err := ServerOptions(&bad, ObservabilityConfig{}, false)
	if err == nil || !strings.Contains(err.Error(), "grpc server tls") {
		t.Fatalf("expected tls error, got %v", err)
	}
}

func TestServerOptions_TLSValidAddsCreds(t *testing.T) {
	_, srvCert, srvKey, _, _, _ := writeTestTLSFiles(t)
	cfg := &ServerTLSConfig{Enabled: true, CertFile: srvCert, KeyFile: srvKey}
	opts, err := ServerOptions(cfg, ObservabilityConfig{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts) < 3 {
		t.Fatalf("expected creds + 2 chain interceptors, got %d", len(opts))
	}
}
