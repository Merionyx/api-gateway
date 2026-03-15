package container

import (
	"log"

	"merionyx/api-gateway/control-plane/internal/config"
	"merionyx/api-gateway/control-plane/internal/delivery/grpc/handler"
	httpHandler "merionyx/api-gateway/control-plane/internal/delivery/http/handler"
	"merionyx/api-gateway/control-plane/internal/delivery/http/router"
	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/repository/git"
	"merionyx/api-gateway/control-plane/internal/usecase"
)

// Container DI for all dependencies
type Container struct {
	// Config
	Config *config.Config

	// Repository Manager
	GitRepositoryManager *git.RepositoryManager

	// Repositories
	TenantRepository      interfaces.TenantRepository
	EnvironmentRepository interfaces.EnvironmentRepository
	ListenerRepository    interfaces.ListenerRepository

	// Use Cases
	TenantUseCase      interfaces.TenantUseCase
	EnvironmentUseCase interfaces.EnvironmentUseCase
	ListenerUseCase    interfaces.ListenerUseCase

	// HTTP Handlers
	TenantHTTPHandler      *httpHandler.TenantHandler
	EnvironmentHTTPHandler *httpHandler.EnvironmentHandler
	ListenerHTTPHandler    *httpHandler.ListenerHandler

	// gRPC Handlers
	TenantGRPCHandler      *handler.TenantHandler
	EnvironmentGRPCHandler *handler.EnvironmentHandler
	ListenerGRPCHandler    *handler.ListenerHandler

	// Playground
	Environments map[string]*Environment

	// Router
	Router *router.Router
}

type Environment struct {
	Name      string
	Snapshots []git.ContractSnapshot
	Services  *EnvironmentServiceConfig
	Contracts *EnvironmentContractConfig
}

type EnvironmentServiceConfig struct {
	Type string
	List []EnvironmentService
}

type EnvironmentService struct {
	Name     string
	Upstream string
}

type EnvironmentContractConfig struct {
	Type string
	List []EnvironmentContract
}

type EnvironmentContract struct {
	Name       string
	Repository string
	Ref        string
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
	c.TenantUseCase = usecase.NewTenantUseCase(c.TenantRepository)
	c.EnvironmentUseCase = usecase.NewEnvironmentUseCase(c.EnvironmentRepository, c.TenantRepository)
	c.ListenerUseCase = usecase.NewListenerUseCase(c.ListenerRepository, c.EnvironmentRepository)

	log.Println("Use cases initialized")
}

// initHandlers initializes the handlers
func (c *Container) initHandlers() {
	// HTTP handlers
	c.TenantHTTPHandler = httpHandler.NewTenantHandler(c.TenantUseCase)
	c.EnvironmentHTTPHandler = httpHandler.NewEnvironmentHandler(c.EnvironmentUseCase)
	c.ListenerHTTPHandler = httpHandler.NewListenerHandler(c.ListenerUseCase)

	// gRPC handlers
	c.TenantGRPCHandler = handler.NewTenantHandler(c.TenantUseCase)
	c.EnvironmentGRPCHandler = handler.NewEnvironmentHandler(c.EnvironmentUseCase)
	c.ListenerGRPCHandler = handler.NewListenerHandler(c.ListenerUseCase)

	log.Println("Handlers initialized")
}

// initRouter initializes the router
func (c *Container) initRouter() {
	c.Router = router.NewRouter(
		c.TenantUseCase,
		c.EnvironmentUseCase,
		c.ListenerUseCase,
	)

	log.Println("Router initialized")
}

func (c *Container) playgroundInit() {

	c.Environments = make(map[string]*Environment)

	// Initialize environments from config
	for _, configEnv := range c.Config.Environments {
		env := &Environment{ // Создаём сразу указатель
			Name: configEnv.Name,
			Contracts: &EnvironmentContractConfig{
				Type: configEnv.Contracts.Type,
				List: make([]EnvironmentContract, 0),
			},
			Services: &EnvironmentServiceConfig{
				Type: configEnv.Services.Type,
				List: make([]EnvironmentService, 0),
			},
			Snapshots: make([]git.ContractSnapshot, 0),
		}
		c.Environments[configEnv.Name] = env
	}

	// Initialize contracts from config
	for _, environment := range c.Config.Environments {
		for _, contract := range environment.Contracts.List {
			c.Environments[environment.Name].Contracts.List = append(c.Environments[environment.Name].Contracts.List, EnvironmentContract{
				Name:       contract.Name,
				Repository: contract.Repository,
				Ref:        contract.Ref,
			})
		}
	}

	// Initialize services from config
	for _, environment := range c.Config.Environments {
		for _, service := range environment.Services.List {
			c.Environments[environment.Name].Services.List = append(c.Environments[environment.Name].Services.List, EnvironmentService{
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
}

// Close closes all resources in the container
func (c *Container) Close() {

}
