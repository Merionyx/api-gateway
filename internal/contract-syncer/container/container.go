package container

import (
	"fmt"
	"log/slog"

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

	if err := container.initGitRepositoryManager(); err != nil {
		return nil, err
	}
	container.initUseCases()
	container.initHandlers()

	return container, nil
}

func (c *Container) initGitRepositoryManager() error {
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
		return fmt.Errorf("initialize git repositories: %w", err)
	}

	slog.Info("git repository manager initialized")
	return nil
}

func (c *Container) initUseCases() {
	c.SyncUseCase = usecase.NewSyncUseCase(c.GitRepositoryManager)

	slog.Info("use cases initialized")
}

func (c *Container) initHandlers() {
	c.SyncGRPCHandler = handler.NewSyncHandler(c.SyncUseCase, c.Config.MetricsHTTP.Enabled)

	slog.Info("handlers initialized")
}

func (c *Container) Close() {
	slog.Info("contract syncer container closed")
}
