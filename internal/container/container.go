package container

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"merionyx/api-gateway/control-plane/internal/config"
	"merionyx/api-gateway/control-plane/internal/delivery/grpc/handler"
	"merionyx/api-gateway/control-plane/internal/delivery/http/router"
	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/repository/etcd"
	"merionyx/api-gateway/control-plane/internal/repository/git"
	"merionyx/api-gateway/control-plane/internal/usecase"

	xdscache "merionyx/api-gateway/control-plane/internal/xds/cache"
	xdsserver "merionyx/api-gateway/control-plane/internal/xds/server"
	"merionyx/api-gateway/control-plane/internal/xds/snapshot"

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

	// Use Cases
	SnapshotsUseCase    interfaces.SnapshotsUseCase
	EnvironmentsUseCase interfaces.EnvironmentsUseCase
	SchemasUseCase      interfaces.SchemasUseCase

	// HTTP Handlers

	// gRPC Handlers
	SnapshotGRPCHandler     *handler.SnapshotHandler
	EnvironmentsGRPCHandler *handler.EnvironmentsHandler
	SchemasGRPCHandler      *handler.SchemasHandler

	// Playground
	Environments map[string]*models.Environment

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

	container.initRepositories()
	container.initGitRepositoryManager()
	container.initUseCases()
	container.initHandlers()
	container.initRouter()

	container.playgroundInit()

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

func (c *Container) initRepositories() {
	c.SchemaRepository = etcd.NewSchemaRepository(c.EtcdClient)
	c.EnvironmentRepository = etcd.NewEnvironmentRepository(c.EtcdClient)

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

	c.EnvironmentsUseCase.SetDependencies(c.EnvironmentRepository, c.SchemasUseCase, c.XDSSnapshotManager)
	c.SnapshotsUseCase.SetDependencies(c.EnvironmentsUseCase, c.XDSSnapshotManager)
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

func (c *Container) playgroundInit() {

	c.Environments = make(map[string]*models.Environment)

	// Initialize environments from config
	for _, configEnv := range c.Config.Environments {
		env := &models.Environment{
			Name: configEnv.Name,
			Bundles: &models.EnvironmentBundleConfig{
				Static: make([]models.StaticContractBundleConfig, 0),
			},
			Services: &models.EnvironmentServiceConfig{
				Static: make([]models.StaticServiceConfig, 0),
			},
			Snapshots: make([]git.ContractSnapshot, 0),
		}
		c.Environments[configEnv.Name] = env
	}

	// Initialize contracts from config
	for _, environment := range c.Config.Environments {
		for _, bundle := range environment.Bundles.Static {
			c.Environments[environment.Name].Bundles.Static = append(c.Environments[environment.Name].Bundles.Static, models.StaticContractBundleConfig{
				Name:       bundle.Name,
				Repository: bundle.Repository,
				Ref:        bundle.Ref,
				Path:       bundle.Path,
			})
		}
	}

	// Initialize services from config
	for _, environment := range c.Config.Environments {
		for _, service := range environment.Services.Static {
			c.Environments[environment.Name].Services.Static = append(c.Environments[environment.Name].Services.Static, models.StaticServiceConfig{
				Name:     service.Name,
				Upstream: service.Upstream,
			})
		}
	}

	for _, environment := range c.Environments {
		for _, bundle := range environment.Bundles.Static {
			snapshots, err := c.GitRepositoryManager.GetRepositorySnapshots(bundle.Repository, bundle.Ref, bundle.Path)
			if err != nil {
				log.Fatalf("Failed to get repository snapshots: %v", err)
			}
			for _, snapshot := range snapshots {
				environment.Snapshots = append(environment.Snapshots, snapshot)
			}
		}
	}

	for _, environment := range c.Environments {
		log.Println("Environment:", environment.Name)
		log.Println("Bundles:", environment.Bundles.Static)
		log.Println("Services:", environment.Services.Static)
		log.Println("Snapshots:", environment.Snapshots)
	}

	for envName, env := range c.Environments {
		snapshot := snapshot.BuildEnvoySnapshot(env)
		nodeID := fmt.Sprintf("envoy-%s", envName)

		if err := c.XDSSnapshotManager.UpdateSnapshot(nodeID, snapshot); err != nil {
			log.Fatalf("Failed to update snapshot for %s: %v", nodeID, err)
		}

		log.Printf("Created xDS snapshot for environment: %s (nodeID: %s)", envName, nodeID)
	}
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
