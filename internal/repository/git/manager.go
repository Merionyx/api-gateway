package git

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"merionyx/api-gateway/control-plane/internal/config"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/util"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage/memory"
	"go.yaml.in/yaml/v3"
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

type RepositoryFile struct {
	Path    string
	Content []byte
}

type ContractSnapshot struct {
	Name string
	// Routes                []ContractRoute
	Prefix                string
	Upstream              ContractUpstream
	AllowUndefinedMethods bool
}

type ContractRoute struct {
	Path   string
	Method string
}

type ContractUpstream struct {
	Name string
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

func (rm *RepositoryManager) GetRepositorySnapshots(name string, ref string, path string) ([]ContractSnapshot, error) {
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
	var files []RepositoryFile

	err = util.Walk(w.Filesystem, path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Filter out files without .yaml, .json, .yml extension
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".json") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		if !info.IsDir() {
			file, err := w.Filesystem.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", path, err)
			}
			defer file.Close()

			content, err := io.ReadAll(file)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", path, err)
			}

			files = append(files, RepositoryFile{
				Path:    path,
				Content: content,
			})
		}
		return nil
	})

	var snapshots []ContractSnapshot

	for _, file := range files {
		contractSchema, err := parseContractSchema(file.Path, file.Content)
		if err != nil {
			log.Fatalf("Failed to parse x-api-gateway: %v", err)
		}
		log.Println("ContractSchema:", contractSchema)

		upstream := ContractUpstream{
			Name: contractSchema.XApiGateway.Service.Name,
		}

		snapshots = append(snapshots, ContractSnapshot{
			Name:                  contractSchema.XApiGateway.Contract.Name,
			Prefix:                contractSchema.XApiGateway.Prefix,
			Upstream:              upstream,
			AllowUndefinedMethods: contractSchema.XApiGateway.AllowUndefinedMethods,
		})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory for repository %s: %w", name, err)
	}
	return snapshots, nil
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
