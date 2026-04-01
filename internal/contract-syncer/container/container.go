package container

import (
	"log"

	"merionyx/api-gateway/internal/contract-syncer/config"
	"merionyx/api-gateway/internal/contract-syncer/delivery/grpc/handler"
	"merionyx/api-gateway/internal/contract-syncer/domain/interfaces"
	"merionyx/api-gateway/internal/contract-syncer/usecase"
	sharedgit "merionyx/api-gateway/internal/shared/git"
)

type Container struct {
	Config *config.Config

	GitRepositoryManager *sharedgit.RepositoryManager

	SyncUseCase interfaces.SyncUseCase

	SyncGRPCHandler *handler.SyncHandler
}

func NewContainer(cfg *config.Config) (*Container, error) {
	container := &Container{
		Config: cfg,
	}

	container.initGitRepositoryManager()
	container.initUseCases()
	container.initHandlers()

	return container, nil
}

func (c *Container) initGitRepositoryManager() {
	c.GitRepositoryManager = sharedgit.NewRepositoryManager()

	var repos []sharedgit.RepositoryConfig
	for _, repo := range c.Config.Repositories {
		repos = append(repos, sharedgit.RepositoryConfig{
			Name:   repo.Name,
			Source: repo.Source,
			URL:    repo.URL,
			Path:   repo.Path,
			Auth: sharedgit.AuthConfig{
				Type:       repo.Auth.Type,
				SSHKeyPath: repo.Auth.SSHKeyPath,
				SSHKeyEnv:  repo.Auth.SSHKeyEnv,
				TokenEnv:   repo.Auth.TokenEnv,
			},
		})
	}

	if err := c.GitRepositoryManager.InitializeRepositories(repos); err != nil {
		log.Fatalf("Failed to initialize repositories: %v", err)
	}

	log.Println("Git repository manager initialized")
}

func (c *Container) initUseCases() {
	c.SyncUseCase = usecase.NewSyncUseCase(c.GitRepositoryManager)

	log.Println("Use cases initialized")
}

func (c *Container) initHandlers() {
	c.SyncGRPCHandler = handler.NewSyncHandler(c.SyncUseCase)

	log.Println("Handlers initialized")
}

func (c *Container) Close() {
	log.Println("Container closed")
}
