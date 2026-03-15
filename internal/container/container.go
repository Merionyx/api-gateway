package container

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"merionyx/api-gateway/control-plane/internal/config"
	"merionyx/api-gateway/control-plane/internal/delivery/http/router"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/repository/git"

	"merionyx/api-gateway/control-plane/internal/xds/builder"
	xdscache "merionyx/api-gateway/control-plane/internal/xds/cache"
	xdsserver "merionyx/api-gateway/control-plane/internal/xds/server"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

// Container DI for all dependencies
type Container struct {
	// Config
	Config *config.Config

	// Repository Manager
	GitRepositoryManager *git.RepositoryManager

	// // Repositories
	// TenantRepository      interfaces.TenantRepository
	// EnvironmentRepository interfaces.EnvironmentRepository
	// ListenerRepository    interfaces.ListenerRepository

	// // Use Cases
	// TenantUseCase      interfaces.TenantUseCase
	// EnvironmentUseCase interfaces.EnvironmentUseCase
	// ListenerUseCase    interfaces.ListenerUseCase

	// // HTTP Handlers
	// TenantHTTPHandler      *httpHandler.TenantHandler
	// EnvironmentHTTPHandler *httpHandler.EnvironmentHandler
	// ListenerHTTPHandler    *httpHandler.ListenerHandler

	// // gRPC Handlers
	// TenantGRPCHandler      *handler.TenantHandler
	// EnvironmentGRPCHandler *handler.EnvironmentHandler
	// ListenerGRPCHandler    *handler.ListenerHandler

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

	container.initGitRepositoryManager()
	container.initUseCases()
	container.initHandlers()
	container.initRouter()
	container.initXDS()

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
				Type: configEnv.Contracts.Type,
				List: make([]models.EnvironmentContract, 0),
			},
			Services: &models.EnvironmentServiceConfig{
				Type: configEnv.Services.Type,
				List: make([]models.EnvironmentService, 0),
			},
			Snapshots: make([]git.ContractSnapshot, 0),
		}
		c.Environments[configEnv.Name] = env
	}

	// Initialize contracts from config
	for _, environment := range c.Config.Environments {
		for _, contract := range environment.Contracts.List {
			c.Environments[environment.Name].Contracts.List = append(c.Environments[environment.Name].Contracts.List, models.EnvironmentContract{
				Name:       contract.Name,
				Repository: contract.Repository,
				Ref:        contract.Ref,
			})
		}
	}

	// Initialize services from config
	for _, environment := range c.Config.Environments {
		for _, service := range environment.Services.List {
			c.Environments[environment.Name].Services.List = append(c.Environments[environment.Name].Services.List, models.EnvironmentService{
				Name:     service.Name,
				Upstream: service.Upstream,
			})
		}
	}

	for _, environment := range c.Environments {
		for _, contract := range environment.Contracts.List {
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
		log.Println("Contracts:", environment.Contracts.List)
		log.Println("Services:", environment.Services.List)
		log.Println("Snapshots:", environment.Snapshots)
	}

	for envName, env := range c.Environments {
		snapshot := c.buildEnvoySnapshot(env)
		nodeID := fmt.Sprintf("envoy-%s", envName)

		if err := c.XDSSnapshotManager.UpdateSnapshot(nodeID, snapshot); err != nil {
			log.Fatalf("Failed to update snapshot for %s: %v", nodeID, err)
		}

		log.Printf("Created xDS snapshot for environment: %s (nodeID: %s)", envName, nodeID)
	}
}

func (c *Container) buildEnvoySnapshot(env *models.Environment) *cache.Snapshot {
	version := fmt.Sprintf("v%d", time.Now().Unix())

	listeners := builder.BuildListeners(env)
	clusters := builder.BuildClusters(env)
	routes := builder.BuildRoutes(env)
	endpoints := builder.BuildEndpoints(env)

	listenerResources := make([]types.Resource, len(listeners))
	for i, l := range listeners {
		listenerResources[i] = l
	}

	clusterResources := make([]types.Resource, len(clusters))
	for i, c := range clusters {
		clusterResources[i] = c
	}

	routeResources := make([]types.Resource, len(routes))
	for i, r := range routes {
		routeResources[i] = r
	}

	endpointResources := make([]types.Resource, len(endpoints))
	for i, e := range endpoints {
		endpointResources[i] = e
	}

	snapshot, err := cache.NewSnapshot(
		version,
		map[resource.Type][]types.Resource{
			resource.ListenerType: listenerResources,
			resource.ClusterType:  clusterResources,
			resource.RouteType:    routeResources,
			resource.EndpointType: endpointResources,
		},
	)
	if err != nil {
		log.Fatalf("Failed to create snapshot: %v", err)
	}

	return snapshot
}

// Close closes all resources in the container
func (c *Container) Close() {

}
