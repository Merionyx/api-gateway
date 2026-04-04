package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"merionyx/api-gateway/internal/auth-sidecar/config"
	authmetrics "merionyx/api-gateway/internal/auth-sidecar/metrics"
	"merionyx/api-gateway/internal/auth-sidecar/storage"
	"merionyx/api-gateway/internal/shared/grpcobs"
	"merionyx/api-gateway/internal/shared/grpcutil"
	authv1 "merionyx/api-gateway/pkg/api/auth/v1"
)

type SyncClient struct {
	config    *config.Config
	storage   *storage.AccessStorage
	conn      *grpc.ClientConn
	client    authv1.AuthServiceClient
	sidecarID string
	connected bool
	mu        sync.RWMutex
}

func NewSyncClient(cfg *config.Config, storage *storage.AccessStorage) *SyncClient {
	// Generate a unique ID for the sidecar
	sidecarID := fmt.Sprintf("sidecar-%d", time.Now().UnixNano())

	return &SyncClient{
		config:    cfg,
		storage:   storage,
		sidecarID: sidecarID,
	}
}

// Start starts the synchronization with the Controller
func (c *SyncClient) Start(ctx context.Context) error {
	tlsOpts, err := grpcobs.DialOptions(c.config.GRPCControllerClient)
	if err != nil {
		return fmt.Errorf("controller dial options: %w", err)
	}
	dialOpts := append(tlsOpts,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                20 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	conn, err := grpc.NewClient(c.config.Controller.Address, dialOpts...)
	if err != nil {
		return fmt.Errorf("failed to connect to controller: %w", err)
	}

	c.conn = conn
	c.client = authv1.NewAuthServiceClient(conn)

	slog.Info("auth sidecar sync connected to controller", "address", c.config.Controller.Address)

	const (
		initialBackoff = 5 * time.Second
		maxBackoff     = 60 * time.Second
	)
	backoff := time.Duration(0)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if backoff > 0 {
			slog.Info("auth sidecar sync reconnecting after backoff", "backoff", backoff)
			if err := grpcutil.SleepOrDone(ctx, backoff); err != nil {
				return err
			}
		}

		if err := c.syncLoop(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("auth sidecar sync error", "error", err)
			authmetrics.RecordControllerReconnect(c.config.MetricsHTTP.Enabled)
			backoff = grpcutil.NextReconnectBackoff(backoff, initialBackoff, maxBackoff)
		}
	}
}

func (c *SyncClient) syncLoop(ctx context.Context) error {
	en := c.config.MetricsHTTP.Enabled
	// Create a bidirectional stream
	stream, err := c.client.SyncAccess(ctx)
	if err != nil {
		authmetrics.RecordControllerStreamClose(en, authmetrics.SyncCloseDialError)
		return fmt.Errorf("failed to create stream: %w", err)
	}

	// Send the initial request
	if err := stream.Send(&authv1.SyncAccessRequest{
		Environment: c.config.Controller.Environment,
		SidecarId:   c.sidecarID,
	}); err != nil {
		authmetrics.RecordControllerStreamClose(en, authmetrics.SyncCloseSendError)
		return fmt.Errorf("failed to send initial request: %w", err)
	}

	authmetrics.RecordControllerStreamOpen(en)
	c.setConnected(true)
	slog.Info("auth sidecar sync stream started", "environment", c.config.Controller.Environment)

	// Listen for updates from the Controller
	for {
		resp, err := stream.Recv()
		if err != nil {
			c.setConnected(false)
			reason := authmetrics.SyncCloseRecvError
			if errors.Is(err, context.Canceled) {
				reason = authmetrics.SyncCloseOK
			}
			authmetrics.RecordControllerStreamClose(en, reason)
			return fmt.Errorf("stream recv error: %w", err)
		}

		switch msg := resp.Message.(type) {
		case *authv1.SyncAccessResponse_InitialConfig:
			// Initial configuration
			slog.Info("auth sidecar sync received initial config", "contracts", len(msg.InitialConfig.Contracts))
			c.storage.SetAccessConfig(msg.InitialConfig)
			authmetrics.RecordControllerSyncMessage(en, authmetrics.SyncMsgInitial)
			authmetrics.SetAccessContractsCount(en, c.storage.GetContractsCount())

		case *authv1.SyncAccessResponse_Update:
			// Incremental update
			slog.Info("auth sidecar sync received update",
				"added", len(msg.Update.AddedContracts),
				"updated", len(msg.Update.UpdatedContracts),
				"removed", len(msg.Update.RemovedContracts))
			c.storage.ApplyUpdate(msg.Update)
			authmetrics.RecordControllerSyncMessage(en, authmetrics.SyncMsgUpdate)
			authmetrics.SetAccessContractsCount(en, c.storage.GetContractsCount())

		case *authv1.SyncAccessResponse_Heartbeat:
			authmetrics.RecordControllerSyncMessage(en, authmetrics.SyncMsgHeartbeat)
		}
	}
}

func (c *SyncClient) setConnected(connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = connected
	authmetrics.SetControllerConnected(c.config.MetricsHTTP.Enabled, connected)
}

func (c *SyncClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *SyncClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
