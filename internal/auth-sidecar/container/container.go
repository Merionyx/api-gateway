package container

import (
	"log"

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
	log.Println("Access storage initialized")
}

func (c *Container) initJWTValidator() {
	c.JWTValidator = jwt.NewJWTValidator(c.Config.JWT.JWKSURL)
	log.Println("JWT validator initialized")
}

func (c *Container) initSyncClient() {
	c.SyncClient = sync.NewSyncClient(c.Config, c.AccessStorage)
	log.Println("Sync client initialized")
}

func (c *Container) Close() {
	if c.SyncClient != nil {
		c.SyncClient.Close()
	}
}
