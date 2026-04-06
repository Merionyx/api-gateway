package container

import (
	"log/slog"

	"merionyx/api-gateway/internal/auth-sidecar/config"
	"merionyx/api-gateway/internal/auth-sidecar/jwt"
	"merionyx/api-gateway/internal/auth-sidecar/storage"
	"merionyx/api-gateway/internal/auth-sidecar/sync"
)

type Container struct {
	Config *config.Config

	// Storage
	AccessStorage *storage.AccessStorage

	// JWT Validator
	JWTValidator *jwt.JWTValidator

	// Sync Client
	SyncClient *sync.SyncClient
}

func NewContainer(cfg *config.Config) (*Container, error) {
	container := &Container{
		Config: cfg,
	}

	container.initStorage()
	container.initJWTValidator()
	container.initSyncClient()

	return container, nil
}

func (c *Container) initStorage() {
	c.AccessStorage = storage.NewAccessStorage()
	slog.Info("access storage initialized")
}

func (c *Container) initJWTValidator() {
	c.JWTValidator = jwt.NewJWTValidator(c.Config.JWT.JWKSURL)
	slog.Info("JWT validator initialized")
}

func (c *Container) initSyncClient() {
	c.SyncClient = sync.NewSyncClient(c.Config, c.AccessStorage)
	slog.Info("sync client initialized")
}

func (c *Container) Close() {
	if c.SyncClient != nil {
		if err := c.SyncClient.Close(); err != nil {
			slog.Warn("sync client close", "error", err)
		}
	}
}
