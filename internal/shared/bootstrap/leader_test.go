package bootstrap

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/shared/election"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestResolveElectionIdentity_Explicit(t *testing.T) {
	if got := resolveElectionIdentity("  replica-1  ", ""); got != "replica-1" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveElectionIdentity_WhitespaceFallsBack(t *testing.T) {
	got := resolveElectionIdentity("   ", "ctrl")
	host, err := os.Hostname()
	if err == nil && strings.TrimSpace(host) != "" {
		if got != host {
			t.Fatalf("expected hostname %q, got %q", host, got)
		}
		return
	}
	if !strings.HasPrefix(got, "ctrl-") {
		t.Fatalf("expected ctrl-<nanos>, got %q", got)
	}
}

func TestResolveElectionIdentity_EmptyFallbackPrefix(t *testing.T) {
	got := resolveElectionIdentity("", "")
	host, err := os.Hostname()
	if err == nil && strings.TrimSpace(host) != "" {
		if got != host {
			t.Fatalf("expected hostname %q, got %q", host, got)
		}
		return
	}
	if !strings.HasPrefix(got, "service-") {
		t.Fatalf("expected service-<nanos>, got %q", got)
	}
}

func TestResolveElectionIdentity_FallbackPrefix(t *testing.T) {
	got := resolveElectionIdentity("", "gw")
	host, err := os.Hostname()
	if err == nil && strings.TrimSpace(host) != "" {
		if got != host {
			t.Fatalf("expected hostname %q, got %q", host, got)
		}
		return
	}
	if !strings.HasPrefix(got, "gw-") {
		t.Fatalf("expected gw-<nanos>, got %q", got)
	}
}

func TestStartLeaderElection_DisabledReturnsNoopGate(t *testing.T) {
	g := StartLeaderElection(context.Background(), nil, LeaderElectionSettings{
		Enabled: false,
		Service: "test",
	})
	if _, ok := g.(election.NoopGate); !ok {
		t.Fatalf("expected NoopGate, got %T", g)
	}
	if !g.IsLeader() {
		t.Fatal("noop should be leader")
	}
}

func TestStartLeaderElection_CancelledCtxDoesNotBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	_ = StartLeaderElection(ctx, cli, LeaderElectionSettings{
		Enabled:          true,
		KeyPrefix:        "/test",
		DefaultKeyPrefix: "/default",
		Identity:         "id",
		Service:          "test",
	})

	time.Sleep(50 * time.Millisecond)
}
