package container

import (
	"context"
	"log"
	"strconv"
	"time"

	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/delivery/grpc/handler"
	"merionyx/api-gateway/internal/controller/delivery/http/router"
	"merionyx/api-gateway/internal/controller/domain/interfaces"
	etcd_repository "merionyx/api-gateway/internal/controller/repository/etcd"
	"merionyx/api-gateway/internal/controller/repository/git"
	"merionyx/api-gateway/internal/controller/repository/memory"
	"merionyx/api-gateway/internal/controller/usecase"
	"merionyx/api-gateway/internal/shared/etcd"

	"merionyx/api-gateway/internal/controller/xds/builder"
	xdscache "merionyx/api-gateway/internal/controller/xds/cache"
	xdsserver "merionyx/api-gateway/internal/controller/xds/server"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Container DI for all dependencies
type Container struct {
	// Config
	Config *config.Config

	// etcd
	EtcdClient *clientv3.Client

	// Repository Manager
	GitRepositoryManager *git.RepositoryManager

	// Repositories
	SchemaRepository      interfaces.SchemaRepository
	EnvironmentRepository interfaces.EnvironmentRepository

	// In-memory repositories
	InMemoryServiceRepository      interfaces.InMemoryServiceRepository
	InMemoryEnvironmentsRepository interfaces.InMemoryEnvironmentsRepository

	// xDS Builder
	XDSBuilder interfaces.XDSBuilder

	// Use Cases
	SnapshotsUseCase    interfaces.SnapshotsUseCase
	EnvironmentsUseCase interfaces.EnvironmentsUseCase
	SchemasUseCase      interfaces.SchemasUseCase

	// HTTP Handlers

	// gRPC Handlers
	SnapshotGRPCHandler     *handler.SnapshotHandler
	EnvironmentsGRPCHandler *handler.EnvironmentsHandler
	SchemasGRPCHandler      *handler.SchemasHandler
	AuthGRPCHandler         *handler.AuthHandler

	// Router
	Router *router.Router

	// xDS Components
	XDSSnapshotManager *xdscache.SnapshotManager
	XDSServer          *xdsserver.Server
}

// NewContainer creates and initializes a new DI container
func NewContainer(cfg *config.Config) (*Container, error) {
	container := &Container{
		Config: cfg,
	}

	container.initEtcd()
	container.initXDS()

	container.createInMemoryServiceRepository()
	container.initRepositories()
	container.initGitRepositoryManager()

	container.initXDSBuilder()

	container.initUseCases()
	container.initHandlers()
	container.initRouter()
	container.startWatchers()

	container.initInMemoryRepositories()

	// container.startEtcdWatcher()

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

func (c *Container) createInMemoryServiceRepository() {
	c.InMemoryServiceRepository = memory.NewServiceRepository()
	c.InMemoryEnvironmentsRepository = memory.NewEnvironmentsRepository()
}

func (c *Container) initInMemoryRepositories() {
	c.InMemoryEnvironmentsRepository.SetDependencies(c.XDSSnapshotManager, c.XDSBuilder, c.GitRepositoryManager)

	if err := c.InMemoryServiceRepository.Initialize(c.Config); err != nil {
		log.Fatalf("Failed to initialize service repository: %v", err)
	}

	if err := c.InMemoryEnvironmentsRepository.Initialize(c.Config); err != nil {
		log.Fatalf("Failed to initialize environments repository: %v", err)
	}

	log.Println("In-memory repositories initialized")
}

func (c *Container) initXDSBuilder() {
	c.XDSBuilder = builder.NewXDSBuilder(c.InMemoryServiceRepository)

	log.Println("xDS builder initialized")
}

func (c *Container) initRepositories() {
	c.SchemaRepository = etcd_repository.NewSchemaRepository(c.EtcdClient)
	c.EnvironmentRepository = etcd_repository.NewEnvironmentRepository(c.EtcdClient)

	log.Println("etcd repositories initialized")
}

// initGitRepositoryManager initializes the git repository manager
func (c *Container) initGitRepositoryManager() {
	c.GitRepositoryManager = git.NewRepositoryManager()

	if err := c.GitRepositoryManager.InitializeRepositories(c.Config.Repositories); err != nil {
		log.Fatalf("Failed to initialize repositories: %v", err)
	}
}

// initUseCases initializes the use cases
func (c *Container) initUseCases() {
	c.SnapshotsUseCase = usecase.NewSnapshotsUseCase()
	c.EnvironmentsUseCase = usecase.NewEnvironmentsUseCase()
	c.SchemasUseCase = usecase.NewSchemasUseCase()

	c.EnvironmentsUseCase.SetDependencies(c.EnvironmentRepository, c.SchemasUseCase, c.XDSSnapshotManager, c.XDSBuilder)
	c.SnapshotsUseCase.SetDependencies(c.EnvironmentsUseCase, c.XDSSnapshotManager, c.XDSBuilder)
	c.SchemasUseCase.SetDependencies(c.SchemaRepository, c.EnvironmentRepository, c.GitRepositoryManager)

	log.Println("SnapshotsUseCase:", c.SnapshotsUseCase)

	log.Println("Use cases initialized")
}

// initHandlers initializes the handlers
func (c *Container) initHandlers() {
	// HTTP handlers

	// gRPC handlers
	c.SnapshotGRPCHandler = handler.NewSnapshotHandler(c.SnapshotsUseCase)
	c.EnvironmentsGRPCHandler = handler.NewEnvironmentsHandler(c.EnvironmentsUseCase)
	c.SchemasGRPCHandler = handler.NewSchemasHandler(c.SchemasUseCase)
	c.AuthGRPCHandler = handler.NewAuthHandler(c.EnvironmentRepository, c.InMemoryEnvironmentsRepository, c.SchemaRepository, c.EtcdClient)

	log.Println("Handlers initialized")
}

// initRouter initializes the router
func (c *Container) initRouter() {
	c.Router = router.NewRouter(
	// c.TenantUseCase,
	// c.EnvironmentUseCase,
	// c.ListenerUseCase,
	)

	log.Println("Router initialized")
}

func (c *Container) initXDS() {
	c.XDSSnapshotManager = xdscache.NewSnapshotManager()
	xdsPort, err := strconv.Atoi(c.Config.Server.XDSPort)
	if err != nil {
		log.Fatalf("Failed to convert xDS port to int: %v", err)
	}
	c.XDSServer = xdsserver.NewXDSServer(c.XDSSnapshotManager.GetCache(), xdsPort)

	log.Println("xDS server initialized")
}

// startWatchers starts watchers for watching changes of environments and schemas
func (c *Container) startWatchers() {
	go c.EnvironmentsUseCase.WatchSnapshotsUpdates(context.Background())
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
