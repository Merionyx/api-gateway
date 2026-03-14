package git

import (
	"fmt"
	"log/slog"
	"merionyx/api-gateway/control-plane/internal/config"
	"sync"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage/memory"
)

type RepositoryManager struct {
	repos map[string]*git.Repository
	mu    sync.RWMutex
}

func NewRepositoryManager() *RepositoryManager {
	return &RepositoryManager{
		repos: make(map[string]*git.Repository),
	}
}

func (rm *RepositoryManager) InitializeRepositories(repos []config.RepositoryConfig) error {
	slog.Info("Initializing repositories", "repositories", repos)

	rm.mu.Lock()
	defer rm.mu.Unlock()

	for _, repository := range repos {
		slog.Info("Initializing repository", "name", repository.Name, "url", repository.URL, "auth", repository.Auth)

		var auth transport.AuthMethod
		var err error

		// Setup authentication depending on the type
		switch repository.Auth.Type {
		case "ssh":
			auth, err = setupSSHAuth(repository.Auth)
			if err != nil {
				return fmt.Errorf("failed to setup SSH auth for repository %s: %w", repository.Name, err)
			}
		case "token":
			auth, err = setupTokenAuth(repository.Auth)
			if err != nil {
				return fmt.Errorf("failed to setup token auth for repository %s: %w", repository.Name, err)
			}
		case "none", "":
			auth = nil
		default:
			return fmt.Errorf("unsupported auth type %s for repository %s", repository.Auth.Type, repository.Name)
		}

		// Clone repository
		repo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
			URL:  repository.URL,
			Auth: auth,
		})

		if err != nil {
			return fmt.Errorf("failed to clone repository %s: %w", repository.Name, err)
		}

		rm.repos[repository.Name] = repo

		slog.Info("Successfully cloned repository", "name", repository.Name)
	}

	return nil
}

func (rm *RepositoryManager) GetRepository(name string) (*git.Repository, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	repo, ok := rm.repos[name]
	if !ok {
		return nil, fmt.Errorf("repository %s not found", name)
	}
	return repo, nil
}
