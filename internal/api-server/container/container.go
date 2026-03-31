package container

import (
	"context"
	"log"
	"time"

	"merionyx/api-gateway/internal/api-server/config"
	"merionyx/api-gateway/internal/api-server/delivery/http/handler"
	"merionyx/api-gateway/internal/api-server/usecase"
	sharedetcd "merionyx/api-gateway/internal/shared/etcd"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Container DI for all dependencies
type Container struct {
	// Config
	Config *config.Config

	// etcd
	EtcdClient *clientv3.Client

	// Use Cases
	JWTUseCase *usecase.JWTUseCase

	// Handlers
	JWTHandler *handler.JWTHandler
}

// NewContainer creates and initializes a new DI container
func NewContainer(cfg *config.Config) (*Container, error) {
	container := &Container{
		Config: cfg,
	}

	container.initEtcd()
	container.initUseCases()
	container.initHandlers()

	return container, nil
}

func (c *Container) initEtcd() {
	etcdClient, err := sharedetcd.NewEtcdClient(c.Config.Etcd)
	if err != nil {
		log.Fatalf("Failed to initialize etcd client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := etcdClient.Status(ctx, c.Config.Etcd.Endpoints[0]); err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
	}

	c.EtcdClient = etcdClient
	log.Println("etcd client initialized and connected successfully")
}

func (c *Container) initUseCases() {
	var err error

	c.JWTUseCase, err = usecase.NewJWTUseCase(
		c.Config.JWT.KeysDir,
		c.Config.JWT.Issuer,
	)
	if err != nil {
		log.Fatalf("Failed to initialize JWT use case: %v", err)
	}

	log.Println("Use cases initialized")
}

func (c *Container) initHandlers() {
	c.JWTHandler = handler.NewJWTHandler(c.JWTUseCase)

	log.Println("Handlers initialized")
}

// Close closes all resources in the container
func (c *Container) Close() {
	if c.EtcdClient != nil {
		if err := c.EtcdClient.Close(); err != nil {
			log.Printf("Failed to close etcd client: %v", err)
		}
		log.Println("etcd client closed")
	}
}
