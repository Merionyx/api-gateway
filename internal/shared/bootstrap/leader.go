package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"merionyx/api-gateway/internal/shared/election"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// LeaderElectionSettings configures etcd-based leader election for a service replica.
type LeaderElectionSettings struct {
	Enabled           bool
	Identity          string
	KeyPrefix         string
	SessionTTLSeconds int
	// DefaultKeyPrefix is used when KeyPrefix is empty after trimming.
	DefaultKeyPrefix string
	// FallbackIDPrefix is used in generated identities when hostname is unavailable (e.g. "controller").
	FallbackIDPrefix string
	// Service is a short name for structured logs (e.g. "gateway-controller", "api-server").
	Service string
}

// StartLeaderElection returns a noop gate when disabled; otherwise starts campaigning in a
// background goroutine and returns an *election.EtcdGate (as election.LeaderGate).
func StartLeaderElection(ctx context.Context, client *clientv3.Client, s LeaderElectionSettings) election.LeaderGate {
	if !s.Enabled {
		slog.Info("leader election disabled (noop gate)", "service", s.Service)
		return election.NoopGate{}
	}

	id := resolveElectionIdentity(s.Identity, s.FallbackIDPrefix)
	prefix := strings.TrimSpace(s.KeyPrefix)
	if prefix == "" {
		prefix = s.DefaultKeyPrefix
	}

	g := election.NewEtcdGate(client, prefix, id, s.SessionTTLSeconds)
	go g.Run(ctx)
	slog.Info("leader election started", "service", s.Service, "prefix", prefix, "identity", id)
	return g
}

func resolveElectionIdentity(configuredIdentity, fallbackPrefix string) string {
	id := strings.TrimSpace(configuredIdentity)
	if id != "" {
		return id
	}
	host, err := os.Hostname()
	if err == nil && host != "" {
		return host
	}
	p := strings.TrimSpace(fallbackPrefix)
	if p == "" {
		p = "service"
	}
	return fmt.Sprintf("%s-%d", p, time.Now().UnixNano())
}
