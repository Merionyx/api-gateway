package etcd

import (
	"strings"
	"testing"
	"time"
)

func TestNewEtcdClient_HTTPNoTLS(t *testing.T) {
	cli, err := NewEtcdClient(EtcdConfig{
		Endpoints:   []string{"http://127.0.0.1:2379"},
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewEtcdClient: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })
}

func TestNewEtcdClient_TLSMissingCerts(t *testing.T) {
	_, err := NewEtcdClient(EtcdConfig{
		Endpoints:   []string{"https://127.0.0.1:2379"},
		DialTimeout: time.Second,
		TLS: EtcdTLSConfig{
			Enabled:  true,
			CertFile: "/nonexistent/cert.pem",
			KeyFile:  "/nonexistent/key.pem",
			CAFile:   "/nonexistent/ca.pem",
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to load TLS") {
		t.Fatalf("unexpected: %v", err)
	}
}
