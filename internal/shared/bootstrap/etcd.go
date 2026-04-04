package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"merionyx/api-gateway/internal/shared/etcd"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// DefaultEtcdStatusTimeout is used for the initial etcd.Status check after dial.
const DefaultEtcdStatusTimeout = 5 * time.Second

// ConnectEtcd creates a client, verifies connectivity with Status against the first endpoint,
// and closes the client if the check fails.
func ConnectEtcd(ctx context.Context, cfg etcd.EtcdConfig, statusTimeout time.Duration) (*clientv3.Client, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("etcd: no endpoints configured")
	}
	if statusTimeout <= 0 {
		statusTimeout = DefaultEtcdStatusTimeout
	}

	client, err := etcd.NewEtcdClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("initialize etcd client: %w", err)
	}

	statusCtx, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()

	if _, err := client.Status(statusCtx, cfg.Endpoints[0]); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect to etcd at %q: %w", cfg.Endpoints[0], err)
	}

	slog.Info("etcd client initialized and connected successfully")
	return client, nil
}

// CloseEtcdClient closes the client if non-nil and logs the outcome.
func CloseEtcdClient(c *clientv3.Client) {
	if c == nil {
		return
	}
	if err := c.Close(); err != nil {
		slog.Error("failed to close etcd client", "error", err)
	}
	slog.Info("etcd client closed")
}
