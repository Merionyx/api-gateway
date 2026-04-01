package container

import (
	"context"
	"log"
	"time"

	"merionyx/api-gateway/internal/api-server/config"
	grpchandler "merionyx/api-gateway/internal/api-server/delivery/grpc/handler"
	httphandler "merionyx/api-gateway/internal/api-server/delivery/http/handler"
	"merionyx/api-gateway/internal/api-server/domain/interfaces"
	"merionyx/api-gateway/internal/api-server/repository/etcd"
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

	// Repositories
	SnapshotRepository   interfaces.SnapshotRepository
	ControllerRepository interfaces.ControllerRepository

	// Use Cases
	JWTUseCase                *usecase.JWTUseCase
	ControllerRegistryUseCase interfaces.ControllerRegistryUseCase
	BundleSyncUseCase         interfaces.BundleSyncUseCase

	// HTTP Handlers
	JWTHandler *httphandler.JWTHandler

	// gRPC Handlers
	ControllerRegistryHandler *grpchandler.ControllerRegistryHandler
}

// NewContainer creates and initializes a new DI container
func NewContainer(cfg *config.Config) (*Container, error) {
	container := &Container{
		Config: cfg,
	}

	container.initEtcd()
	container.initRepositories()
	container.initUseCases()
	container.initHandlers()
	container.startWatchers()

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

func (c *Container) initRepositories() {
	c.SnapshotRepository = etcd.NewSnapshotRepository(c.EtcdClient)
	c.ControllerRepository = etcd.NewControllerRepository(c.EtcdClient)

	log.Println("Repositories initialized")
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

	c.ControllerRegistryUseCase = usecase.NewControllerRegistryUseCase(
		c.ControllerRepository,
		c.SnapshotRepository,
	)

	bundleSyncUseCase := usecase.NewBundleSyncUseCase(
		c.SnapshotRepository,
		c.ControllerRepository,
		c.Config.ContractSyncer.Address,
	)

	bundleSyncUseCase.SetRegistryUseCase(c.ControllerRegistryUseCase.(*usecase.ControllerRegistryUseCase))
	
	c.BundleSyncUseCase = bundleSyncUseCase

	log.Println("Use cases initialized")
}

func (c *Container) initHandlers() {
	c.JWTHandler = httphandler.NewJWTHandler(c.JWTUseCase)
	c.ControllerRegistryHandler = grpchandler.NewControllerRegistryHandler(c.ControllerRegistryUseCase)

	log.Println("Handlers initialized")
}

func (c *Container) startWatchers() {
	go c.BundleSyncUseCase.StartBundleWatcher(context.Background())
	log.Println("Bundle watcher started")
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
