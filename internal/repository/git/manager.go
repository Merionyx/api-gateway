package git

import (
	"fmt"
	"log/slog"
	"merionyx/api-gateway/control-plane/internal/config"
	"os"
	"strings"
	"sync"

	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/util"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
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
		repo, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
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

func (rm *RepositoryManager) GetRepositoryFiles(name string, ref string, path string) ([]string, error) {
	if path == "" {
		path = "."
	}

	repo, err := rm.GetRepository(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository %s: %w", name, err)
	}
	w, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree for repository %s: %w", name, err)
	}
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/remotes/origin/" + ref),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to checkout repository %s: %w", name, err)
	}
	var filesNames []string

	err = util.Walk(w.Filesystem, path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Filter out files without .yaml, .json, .yml extension
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".json") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		if !info.IsDir() {
			filesNames = append(filesNames, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory for repository %s: %w", name, err)
	}
	return filesNames, nil
}
