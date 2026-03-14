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

	// Router
	Router *router.Router
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

	return container, nil
}

// initGitRepositoryManager initializes the git repository manager
func (c *Container) initGitRepositoryManager() {
	c.GitRepositoryManager = git.NewRepositoryManager()

	if err := c.GitRepositoryManager.InitializeRepositories(c.Config.Repositories); err != nil {
		log.Fatalf("Failed to initialize repositories: %v", err)
	}

	files, err := c.GitRepositoryManager.GetRepositoryFiles("api-gateway-schemas-https", "master")
	if err != nil {
		log.Fatalf("Failed to get repository files: %v", err)
	}

	log.Println("Repository files:", files)
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

// Close closes all resources in the container
func (c *Container) Close() {

}
