package election

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

const spanElectionPkg = "internal/shared/election"

// campaignProposal is the JSON document written as the concurrency election value in etcd
// (so every key under the election prefix holds valid JSON, not a bare string).
type campaignProposal struct {
	Identity string `json:"identity"`
}

// EtcdGate implements leader election against etcd (client/v3 concurrency).
// Only one candidate holds leadership per election prefix at a time.
type EtcdGate struct {
	client   *clientv3.Client
	prefix   string
	identity string
	ttlSec   int

	isLeader atomic.Bool

	// leaderNotify signals after isLeader is updated (buffer 1 coalesces bursts).
	leaderNotify chan struct{}
}

// NewEtcdGate creates a gate. ttlSec is the session TTL in seconds (keepalive refreshes it).
func NewEtcdGate(client *clientv3.Client, prefix, identity string, ttlSec int) *EtcdGate {
	if ttlSec <= 0 {
		ttlSec = 5
	}
	return &EtcdGate{
		client:       client,
		prefix:       prefix,
		identity:     identity,
		ttlSec:       ttlSec,
		leaderNotify: make(chan struct{}, 1),
	}
}

func (g *EtcdGate) IsLeader() bool {
	return g.isLeader.Load()
}

// LeaderChanged notifies subscribers when leadership may have changed.
func (g *EtcdGate) LeaderChanged() <-chan struct{} {
	return g.leaderNotify
}

func (g *EtcdGate) notifyLeaderChanged() {
	select {
	case g.leaderNotify <- struct{}{}:
	default:
	}
}

// Run blocks until ctx is cancelled, campaigning in a loop when leadership is lost.
func (g *EtcdGate) Run(ctx context.Context) {
	for {
		if err := ctx.Err(); err != nil {
			slog.Info("leader election stopped", "reason", err)
			return
		}

		campCtx, campSpan := telemetry.Start(ctx, telemetry.SpanName(spanElectionPkg, "Campaign"))
		sess, err := concurrency.NewSession(g.client, concurrency.WithTTL(g.ttlSec))
		if err != nil {
			telemetry.MarkError(campSpan, err)
			campSpan.End()
			slog.Warn("leader election: session failed", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}

		el := concurrency.NewElection(sess, g.prefix)

		proposal, mErr := json.Marshal(campaignProposal{Identity: g.identity})
		if mErr != nil {
			_ = sess.Close()
			telemetry.MarkError(campSpan, mErr)
			campSpan.End()
			slog.Error("leader election: marshal proposal", "error", mErr)
			continue
		}
		err = el.Campaign(campCtx, string(proposal))
		if err != nil {
			_ = sess.Close()
			telemetry.MarkError(campSpan, err)
			campSpan.End()
			if ctx.Err() != nil {
				return
			}
			slog.Warn("leader election: campaign ended", "error", err)
			continue
		}
		campSpan.End()

		g.isLeader.Store(true)
		g.notifyLeaderChanged()
		slog.Info("leader election: became leader", "identity", g.identity, "prefix", g.prefix)

		select {
		case <-sess.Done():
			slog.Warn("leader election: lost leadership (session expired)")
		case <-ctx.Done():
			_ = el.Resign(context.Background())
		}

		g.isLeader.Store(false)
		g.notifyLeaderChanged()
		slog.Info("leader election: stepped down", "identity", g.identity)
		_ = sess.Close()
	}
}
