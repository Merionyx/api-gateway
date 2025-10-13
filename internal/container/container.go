package container

import (
	"log"

	"merionyx/api-gateway/control-plane/internal/config"
	"merionyx/api-gateway/control-plane/internal/database/postgres"
	"merionyx/api-gateway/control-plane/internal/delivery/grpc/handler"
	httpHandler "merionyx/api-gateway/control-plane/internal/delivery/http/handler"
	"merionyx/api-gateway/control-plane/internal/delivery/http/router"
	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/repository"
	"merionyx/api-gateway/control-plane/internal/usecase"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Container DI for all dependencies
type Container struct {
	// Config
	Config *config.Config

	// Database
	DB *pgxpool.Pool

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

	// Initialize components in the correct order
	if err := container.initDatabase(); err != nil {
		return nil, err
	}

	container.initRepositories()
	container.initUseCases()
	container.initHandlers()
	container.initRouter()

	return container, nil
}

// initDatabase initializes the database connection
func (c *Container) initDatabase() error {
	dbFactory := postgres.NewFactory(c.Config)
	db, err := dbFactory.CreateConnection()
	if err != nil {
		return err
	}

	c.DB = db
	log.Println("Database connection initialized")
	return nil
}

// initRepositories initializes the repositories
func (c *Container) initRepositories() {
	c.TenantRepository = repository.NewTenantRepository(c.DB)
	c.EnvironmentRepository = repository.NewEnvironmentRepository(c.DB)
	c.ListenerRepository = repository.NewListenerRepository(c.DB)

	log.Println("Repositories initialized")
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
	if c.DB != nil {
		c.DB.Close()
		log.Println("Database connection closed")
	}
}
