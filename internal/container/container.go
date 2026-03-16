package container

import (
	"fmt"
	"log"
	"strconv"

	"merionyx/api-gateway/control-plane/internal/config"
	"merionyx/api-gateway/control-plane/internal/delivery/grpc/handler"
	"merionyx/api-gateway/control-plane/internal/delivery/http/router"
	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/repository/git"
	"merionyx/api-gateway/control-plane/internal/usecase"

	xdscache "merionyx/api-gateway/control-plane/internal/xds/cache"
	xdsserver "merionyx/api-gateway/control-plane/internal/xds/server"
	"merionyx/api-gateway/control-plane/internal/xds/snapshot"
)

// Container DI for all dependencies
type Container struct {
	// Config
	Config *config.Config

	// Repository Manager
	GitRepositoryManager *git.RepositoryManager

	// Repositories
	// TenantRepository      interfaces.TenantRepository
	// EnvironmentRepository interfaces.EnvironmentRepository
	// ListenerRepository    interfaces.ListenerRepository

	// Use Cases
	// TenantUseCase      interfaces.TenantUseCase
	// EnvironmentUseCase interfaces.EnvironmentUseCase
	// ListenerUseCase    interfaces.ListenerUseCase
	SnapshotsUseCase interfaces.SnapshotsUseCase

	// HTTP Handlers
	// TenantHTTPHandler      *httpHandler.TenantHandler
	// EnvironmentHTTPHandler *httpHandler.EnvironmentHandler
	// ListenerHTTPHandler    *httpHandler.ListenerHandler

	// gRPC Handlers
	// TenantGRPCHandler      *handler.TenantHandler
	// EnvironmentGRPCHandler *handler.EnvironmentHandler
	// ListenerGRPCHandler    *handler.ListenerHandler
	SnapshotGRPCHandler *handler.SnapshotHandler

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

	container.initXDS()
	container.initGitRepositoryManager()
	container.initUseCases()
	container.initHandlers()
	container.initRouter()

	container.playgroundInit()

	return container, nil
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
	// c.TenantUseCase = usecase.NewTenantUseCase(c.TenantRepository)
	// c.EnvironmentUseCase = usecase.NewEnvironmentUseCase(c.EnvironmentRepository, c.TenantRepository)
	// c.ListenerUseCase = usecase.NewListenerUseCase(c.ListenerRepository, c.EnvironmentRepository)
	c.SnapshotsUseCase = usecase.NewSnapshotsUseCase(&c.Environments, c.XDSSnapshotManager)

	log.Println("SnapshotsUseCase:", c.SnapshotsUseCase)

	log.Println("Use cases initialized")
}

// initHandlers initializes the handlers
func (c *Container) initHandlers() {
	// HTTP handlers
	// c.TenantHTTPHandler = httpHandler.NewTenantHandler(c.TenantUseCase)
	// c.EnvironmentHTTPHandler = httpHandler.NewEnvironmentHandler(c.EnvironmentUseCase)
	// c.ListenerHTTPHandler = httpHandler.NewListenerHandler(c.ListenerUseCase)

	// gRPC handlers
	// c.TenantGRPCHandler = handler.NewTenantHandler(c.TenantUseCase)
	// c.EnvironmentGRPCHandler = handler.NewEnvironmentHandler(c.EnvironmentUseCase)
	// c.ListenerGRPCHandler = handler.NewListenerHandler(c.ListenerUseCase)
	c.SnapshotGRPCHandler = handler.NewSnapshotHandler(c.SnapshotsUseCase)

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
			Contracts: &models.EnvironmentContractConfig{
				Static: make([]models.StaticContractConfig, 0),
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
		for _, contract := range environment.Contracts.Static {
			c.Environments[environment.Name].Contracts.Static = append(c.Environments[environment.Name].Contracts.Static, models.StaticContractConfig{
				Name:       contract.Name,
				Repository: contract.Repository,
				Ref:        contract.Ref,
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
		for _, contract := range environment.Contracts.Static {
			snapshots, err := c.GitRepositoryManager.GetRepositorySnapshots(contract.Repository, contract.Ref, "openapi")
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
		log.Println("Contracts:", environment.Contracts.Static)
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

}
