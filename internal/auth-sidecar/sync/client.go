package sync

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"merionyx/api-gateway/internal/auth-sidecar/config"
	"merionyx/api-gateway/internal/auth-sidecar/storage"
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
	conn, err := grpc.NewClient(
		c.config.Controller.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                20 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to controller: %w", err)
	}

	c.conn = conn
	c.client = authv1.NewAuthServiceClient(conn)

	log.Printf("[SYNC] Connected to Controller at %s", c.config.Controller.Address)

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
			log.Printf("[SYNC] Reconnecting after backoff %v", backoff)
			if err := grpcutil.SleepOrDone(ctx, backoff); err != nil {
				return err
			}
		}

		if err := c.syncLoop(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("[SYNC] Sync error: %v", err)
			backoff = grpcutil.NextReconnectBackoff(backoff, initialBackoff, maxBackoff)
		}
	}
}

func (c *SyncClient) syncLoop(ctx context.Context) error {
	// Create a bidirectional stream
	stream, err := c.client.SyncAccess(ctx)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	// Send the initial request
	if err := stream.Send(&authv1.SyncAccessRequest{
		Environment: c.config.Controller.Environment,
		SidecarId:   c.sidecarID,
	}); err != nil {
		return fmt.Errorf("failed to send initial request: %w", err)
	}

	c.setConnected(true)
	log.Printf("[SYNC] Started sync stream for environment: %s", c.config.Controller.Environment)

	// Listen for updates from the Controller
	for {
		resp, err := stream.Recv()
		if err != nil {
			c.setConnected(false)
			return fmt.Errorf("stream recv error: %w", err)
		}

		switch msg := resp.Message.(type) {
		case *authv1.SyncAccessResponse_InitialConfig:
			// Initial configuration
			log.Printf("[SYNC] Received initial config: %d contracts", len(msg.InitialConfig.Contracts))
			c.storage.SetAccessConfig(msg.InitialConfig)

		case *authv1.SyncAccessResponse_Update:
			// Incremental update
			log.Printf("[SYNC] Received update: +%d =%d -%d contracts",
				len(msg.Update.AddedContracts),
				len(msg.Update.UpdatedContracts),
				len(msg.Update.RemovedContracts))
			c.storage.ApplyUpdate(msg.Update)

		case *authv1.SyncAccessResponse_Heartbeat:
			// Heartbeat for keep-alive
			// log.Printf("[SYNC] Heartbeat received")
		}
	}
}

func (c *SyncClient) setConnected(connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = connected
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
