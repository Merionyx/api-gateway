package container

import (
	"context"
	"log"
	"time"

	"merionyx/api-gateway/internal/api-server/config"
	"merionyx/api-gateway/internal/shared/etcd"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Container DI for all dependencies
type Container struct {
	// Config
	Config *config.Config

	// etcd
	EtcdClient *clientv3.Client
}

// NewContainer creates and initializes a new DI container
func NewContainer(cfg *config.Config) (*Container, error) {
	container := &Container{
		Config: cfg,
	}

	container.initEtcd()

	return container, nil
}

func (c *Container) initEtcd() {
	etcdClient, err := etcd.NewEtcdClient(c.Config.Etcd)
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

// Close closes all resources in the container
func (c *Container) Close() {
	if c.EtcdClient != nil {
		if err := c.EtcdClient.Close(); err != nil {
			log.Printf("Failed to close etcd client: %v", err)
		}
		log.Println("etcd client closed")
	}
}
