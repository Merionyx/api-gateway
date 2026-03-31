package sync

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"merionyx/api-gateway/internal/auth-sidecar/config"
	"merionyx/api-gateway/internal/auth-sidecar/storage"
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
	// Генерируем уникальный ID для sidecar
	sidecarID := fmt.Sprintf("sidecar-%d", time.Now().UnixNano())

	return &SyncClient{
		config:    cfg,
		storage:   storage,
		sidecarID: sidecarID,
	}
}

// Start запускает синхронизацию с Controller
func (c *SyncClient) Start(ctx context.Context) error {
	// Подключаемся к Controller
	conn, err := grpc.NewClient(
		c.config.Controller.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to controller: %w", err)
	}

	c.conn = conn
	c.client = authv1.NewAuthServiceClient(conn)

	log.Printf("[SYNC] Connected to Controller at %s", c.config.Controller.Address)

	// Запускаем sync loop с reconnect логикой
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := c.syncLoop(ctx); err != nil {
				log.Printf("[SYNC] Sync error: %v, reconnecting in 5s...", err)
				time.Sleep(5 * time.Second)
			}
		}
	}
}

func (c *SyncClient) syncLoop(ctx context.Context) error {
	// Создаем bidirectional stream
	stream, err := c.client.SyncAccess(ctx)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	// Отправляем начальный запрос
	if err := stream.Send(&authv1.SyncAccessRequest{
		Environment: c.config.Controller.Environment,
		SidecarId:   c.sidecarID,
	}); err != nil {
		return fmt.Errorf("failed to send initial request: %w", err)
	}

	c.setConnected(true)
	log.Printf("[SYNC] Started sync stream for environment: %s", c.config.Controller.Environment)

	// Слушаем обновления от Controller
	for {
		resp, err := stream.Recv()
		if err != nil {
			c.setConnected(false)
			return fmt.Errorf("stream recv error: %w", err)
		}

		switch msg := resp.Message.(type) {
		case *authv1.SyncAccessResponse_InitialConfig:
			// Начальная конфигурация
			log.Printf("[SYNC] Received initial config: %d contracts", len(msg.InitialConfig.Contracts))
			c.storage.SetAccessConfig(msg.InitialConfig)

		case *authv1.SyncAccessResponse_Update:
			// Инкрементальное обновление
			log.Printf("[SYNC] Received update: +%d =%d -%d contracts",
				len(msg.Update.AddedContracts),
				len(msg.Update.UpdatedContracts),
				len(msg.Update.RemovedContracts))
			c.storage.ApplyUpdate(msg.Update)

		case *authv1.SyncAccessResponse_Heartbeat:
			// Heartbeat
			log.Printf("[SYNC] Heartbeat received")
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
