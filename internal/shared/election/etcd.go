package election

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// EtcdGate implements leader election against etcd (client/v3 concurrency).
// Only one candidate holds leadership per election prefix at a time.
type EtcdGate struct {
	client   *clientv3.Client
	prefix   string
	identity string
	ttlSec   int

	isLeader atomic.Bool
}

// NewEtcdGate creates a gate. ttlSec is the session TTL in seconds (keepalive refreshes it).
func NewEtcdGate(client *clientv3.Client, prefix, identity string, ttlSec int) *EtcdGate {
	if ttlSec <= 0 {
		ttlSec = 5
	}
	return &EtcdGate{
		client:   client,
		prefix:   prefix,
		identity: identity,
		ttlSec:   ttlSec,
	}
}

func (g *EtcdGate) IsLeader() bool {
	return g.isLeader.Load()
}

// Run blocks until ctx is cancelled, campaigning in a loop when leadership is lost.
func (g *EtcdGate) Run(ctx context.Context) {
	for {
		if err := ctx.Err(); err != nil {
			slog.Info("leader election stopped", "reason", err)
			return
		}

		sess, err := concurrency.NewSession(g.client, concurrency.WithTTL(g.ttlSec))
		if err != nil {
			slog.Warn("leader election: session failed", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}

		el := concurrency.NewElection(sess, g.prefix)

		err = el.Campaign(ctx, g.identity)
		if err != nil {
			_ = sess.Close()
			if ctx.Err() != nil {
				return
			}
			slog.Warn("leader election: campaign ended", "error", err)
			continue
		}

		g.isLeader.Store(true)
		slog.Info("leader election: became leader", "identity", g.identity, "prefix", g.prefix)

		select {
		case <-sess.Done():
			slog.Warn("leader election: lost leadership (session expired)")
		case <-ctx.Done():
			_ = el.Resign(context.Background())
		}

		g.isLeader.Store(false)
		slog.Info("leader election: stepped down", "identity", g.identity)
		_ = sess.Close()
	}
}
