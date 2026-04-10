//go:build integration

package integration

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/shared/bootstrap"
	"github.com/merionyx/api-gateway/internal/shared/election"
	"github.com/merionyx/api-gateway/internal/shared/etcd"
)

func TestBootstrap_ConnectEtcd(t *testing.T) {
	eps := EtcdEndpoints()
	cli, err := bootstrap.ConnectEtcd(context.Background(), etcd.EtcdConfig{
		Endpoints:   eps,
		DialTimeout: 5 * time.Second,
	}, 5*time.Second)
	if err != nil {
		t.Fatalf("ConnectEtcd: %v", err)
	}
	bootstrap.CloseEtcdClient(cli)
}

func TestElection_EtcdGateBecomesLeader(t *testing.T) {
	cli := NewEtcdClient(t)
	defer bootstrap.CloseEtcdClient(cli)

	prefix := "/integration/" + strings.ReplaceAll(t.Name(), "/", "_")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	g := election.NewEtcdGate(cli, prefix, "integration-test", 5)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		g.Run(ctx)
	}()

	deadline := time.After(30 * time.Second)
	for !g.IsLeader() {
		select {
		case <-deadline:
			t.Fatal("did not become leader in time")
		case <-time.After(50 * time.Millisecond):
		}
	}

	cancel()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
