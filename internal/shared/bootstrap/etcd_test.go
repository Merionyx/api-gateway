package bootstrap

import (
	"context"
	"strings"
	"testing"
	"time"

	"merionyx/api-gateway/internal/shared/etcd"
)

func TestConnectEtcd_NoEndpoints(t *testing.T) {
	_, err := ConnectEtcd(context.Background(), etcd.EtcdConfig{}, time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no endpoints") {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestConnectEtcd_InvalidEndpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := ConnectEtcd(ctx, etcd.EtcdConfig{
		Endpoints:   []string{"http://127.0.0.1:1"},
		DialTimeout: 200 * time.Millisecond,
	}, 200*time.Millisecond)
	if err == nil {
		t.Fatal("expected error")
	}
}
