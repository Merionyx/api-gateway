package grpcobs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerTLS_EnabledMissingCertFiles(t *testing.T) {
	_, err := ServerTLS(ServerTLSConfig{Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "cert_file and key_file") {
		t.Fatalf("expected cert/key required error, got %v", err)
	}
}

func TestServerTLS_LoadKeyPairError(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "s.pem")
	keyPath := filepath.Join(dir, "s.key")
	if err := os.WriteFile(certPath, []byte("not pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("not pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ServerTLS(ServerTLSConfig{Enabled: true, CertFile: certPath, KeyFile: keyPath})
	if err == nil || !strings.Contains(err.Error(), "load key pair") {
		t.Fatalf("expected load error, got %v", err)
	}
}

func TestServerTLS_ClientCAReadError(t *testing.T) {
	_, srvCert, srvKey, _, _, _ := writeTestTLSFiles(t)
	_, err := ServerTLS(ServerTLSConfig{
		Enabled:      true,
		CertFile:     srvCert,
		KeyFile:      srvKey,
		ClientCAFile: filepath.Join(t.TempDir(), "nope.pem"),
	})
	if err == nil || !strings.Contains(err.Error(), "read client_ca_file") {
		t.Fatalf("expected read client_ca error, got %v", err)
	}
}

func TestServerTLS_ClientCAInvalidPEM(t *testing.T) {
	_, srvCert, srvKey, _, _, _ := writeTestTLSFiles(t)
	badCA := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(badCA, []byte("not a cert"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ServerTLS(ServerTLSConfig{
		Enabled:      true,
		CertFile:     srvCert,
		KeyFile:      srvKey,
		ClientCAFile: badCA,
	})
	if err == nil || !strings.Contains(err.Error(), "no certificates in client_ca_file") {
		t.Fatalf("expected no certificates in client_ca, got %v", err)
	}
}

func TestServerTLS_SuccessWithClientCA(t *testing.T) {
	_, srvCert, srvKey, caPEM, _, _ := writeTestTLSFiles(t)
	cfg, err := ServerTLS(ServerTLSConfig{
		Enabled:      true,
		CertFile:     srvCert,
		KeyFile:      srvKey,
		ClientCAFile: caPEM,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || len(cfg.Certificates) != 1 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestServerTLS_SuccessNoClientCA(t *testing.T) {
	_, srvCert, srvKey, _, _, _ := writeTestTLSFiles(t)
	cfg, err := ServerTLS(ServerTLSConfig{Enabled: true, CertFile: srvCert, KeyFile: srvKey})
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected tls.Config")
	}
}

func TestClientTransportCredentials_CAReadError(t *testing.T) {
	_, err := ClientTransportCredentials(ClientTLSConfig{
		Enabled: true,
		CAFile:  filepath.Join(t.TempDir(), "missing.pem"),
	})
	if err == nil || !strings.Contains(err.Error(), "read ca_file") {
		t.Fatalf("expected read ca error, got %v", err)
	}
}

func TestClientTransportCredentials_CAInvalidPEM(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(p, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ClientTransportCredentials(ClientTLSConfig{Enabled: true, CAFile: p})
	if err == nil || !strings.Contains(err.Error(), "no certificates in ca_file") {
		t.Fatalf("expected empty pool error, got %v", err)
	}
}

func TestClientTransportCredentials_ClientCertLoadError(t *testing.T) {
	_, _, _, caPEM, _, _ := writeTestTLSFiles(t)
	_, err := ClientTransportCredentials(ClientTLSConfig{
		Enabled:  true,
		CAFile:   caPEM,
		CertFile: filepath.Join(t.TempDir(), "c.pem"),
		KeyFile:  filepath.Join(t.TempDir(), "c.key"),
	})
	if err == nil || !strings.Contains(err.Error(), "load client cert") {
		t.Fatalf("expected client cert load error, got %v", err)
	}
}

func TestClientTransportCredentials_EnabledNoCAOrCerts(t *testing.T) {
	creds, err := ClientTransportCredentials(ClientTLSConfig{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if creds == nil {
		t.Fatal("expected creds")
	}
}

func TestClientTransportCredentials_CAOnlyNoMTLS(t *testing.T) {
	_, _, _, caPEM, _, _ := writeTestTLSFiles(t)
	creds, err := ClientTransportCredentials(ClientTLSConfig{Enabled: true, CAFile: caPEM})
	if err != nil {
		t.Fatal(err)
	}
	if creds == nil {
		t.Fatal("expected creds")
	}
}

func TestClientTransportCredentials_TLSWithCAAndServerName(t *testing.T) {
	_, _, _, caPEM, clCert, clKey := writeTestTLSFiles(t)
	creds, err := ClientTransportCredentials(ClientTLSConfig{
		Enabled:    true,
		CAFile:     caPEM,
		CertFile:   clCert,
		KeyFile:    clKey,
		ServerName: "localhost",
	})
	if err != nil {
		t.Fatal(err)
	}
	if creds == nil {
		t.Fatal("expected creds")
	}
}

func TestDialOptions_TLSConfigError(t *testing.T) {
	_, err := DialOptions(ClientTLSConfig{
		Enabled: true,
		CAFile:  filepath.Join(t.TempDir(), "missing.pem"),
	})
	if err == nil || !strings.Contains(err.Error(), "grpc client tls") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}
