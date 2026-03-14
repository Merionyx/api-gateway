package container

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"

	"merionyx/api-gateway/control-plane/internal/config"
	"merionyx/api-gateway/control-plane/internal/delivery/grpc/handler"
	httpHandler "merionyx/api-gateway/control-plane/internal/delivery/http/handler"
	"merionyx/api-gateway/control-plane/internal/delivery/http/router"
	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/repository/git"
	"merionyx/api-gateway/control-plane/internal/usecase"

	"go.yaml.in/yaml/v3"
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

type ContractSchema struct {
	Paths       map[string]struct{} `mapstructure:"paths" json:"paths" yaml:"paths"`
	XApiGateway XApiGatewaySchema   `mapstructure:"x-api-gateway" json:"x-api-gateway" yaml:"x-api-gateway"`
}

type XApiGatewaySchema struct {
	Prefix                string `mapstructure:"prefix" json:"prefix" yaml:"prefix"`
	AllowUndefinedMethods bool   `mapstructure:"allow_undefined_methods" json:"allow_undefined_methods" yaml:"allow_undefined_methods"`
	Contract              struct {
		Name string `mapstructure:"name" json:"name" yaml:"name"`
	} `mapstructure:"contract" json:"contract" yaml:"contract"`
	Service struct {
		Name string `mapstructure:"name" json:"name" yaml:"name"`
	} `mapstructure:"service" json:"service" yaml:"service"`
}

// initGitRepositoryManager initializes the git repository manager
func (c *Container) initGitRepositoryManager() {
	c.GitRepositoryManager = git.NewRepositoryManager()

	if err := c.GitRepositoryManager.InitializeRepositories(c.Config.Repositories); err != nil {
		log.Fatalf("Failed to initialize repositories: %v", err)
	}

	files, err := c.GitRepositoryManager.GetRepositoryFiles("api-gateway-schemas-https", "master", "openapi")
	if err != nil {
		log.Fatalf("Failed to get repository files: %v", err)
	}

	for _, file := range files {
		contractSchema, err := parseContractSchema(file.Path, file.Content)
		if err != nil {
			log.Fatalf("Failed to parse x-api-gateway: %v", err)
		}
		log.Println("ContractSchema:", contractSchema)
	}

	log.Println("Repository files:", files)
}

func parseContractSchema(filename string, content []byte) (*ContractSchema, error) {
	ext := filepath.Ext(filename)

	switch ext {
	case ".json":
		log.Println("Parsing JSON file:", filename)
		return parseJSON(content)
	case ".yaml", ".yml":
		log.Println("Parsing YAML file:", filename)
		return parseYAML(content)
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}
}
func parseJSON(content []byte) (*ContractSchema, error) {
	var doc ContractSchema
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}
func parseYAML(content []byte) (*ContractSchema, error) {
	var contract ContractSchema
	if err := yaml.Unmarshal(content, &contract); err != nil {
		return nil, err
	}
	return &contract, nil
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
