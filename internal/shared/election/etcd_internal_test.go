package election

import (
	"context"
	"testing"
	"time"
)

func TestEtcdGate_Run_ContextAlreadyCancelled(t *testing.T) {
	g := NewEtcdGate(nil, "/p", "id", 5)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		g.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return")
	}
}

func TestEtcdGate_notifyLeaderChanged_Coalesces(t *testing.T) {
	g := NewEtcdGate(nil, "/p", "id", 5)
	g.notifyLeaderChanged()
	g.notifyLeaderChanged()
	select {
	case <-g.LeaderChanged():
	default:
		t.Fatal("expected first notify on channel")
	}
	select {
	case <-g.LeaderChanged():
		t.Fatal("second notify should have been coalesced")
	default:
	}
}
