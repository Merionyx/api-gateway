package git

import (
	"sync"

	gogit "github.com/go-git/go-git/v6"
	gitclient "github.com/go-git/go-git/v6/plumbing/client"
)

// Supported RepositoryConfig.Source values (keep in sync with CRD / config docs).
const (
	RepositorySourceGit      = "git"
	RepositorySourceLocalGit = "local-git"
	RepositorySourceLocalDir = "local-dir"
)

type RepositoryConfig struct {
	Name   string
	Source string
	URL    string
	Path   string
	Auth   AuthConfig
}

type AuthConfig struct {
	Type       string
	SSHKeyPath string
	SSHKeyEnv  string
	TokenEnv   string
}

type ManagedRepo struct {
	Name          string
	Repo          *gogit.Repository
	ClientOptions []gitclient.Option
	Source        string
	Path          string
}

type bundleSnapshotCacheEntry struct {
	commitHash string
	snapshots  []ContractSnapshot
}

type RepositoryManager struct {
	repos           map[string]*ManagedRepo
	mu              sync.RWMutex
	bundleSnapCache map[string]bundleSnapshotCacheEntry
	cacheMu         sync.RWMutex
}

type RepositoryFile struct {
	Path    string `json:"path"`
	Content []byte `json:"content"`
}

type ContractSnapshot struct {
	Name                  string           `json:"name"`
	Prefix                string           `json:"prefix"`
	Upstream              ContractUpstream `json:"upstream"`
	AllowUndefinedMethods bool             `json:"allow_undefined_methods"`
	Access                Access           `json:"access"`
}

type ContractUpstream struct {
	Name string `json:"name"`
}

type Access struct {
	Secure bool  `json:"secure" yaml:"secure" mapstructure:"secure"`
	Apps   []App `json:"apps" yaml:"apps" mapstructure:"apps"`
}

type App struct {
	AppID        string   `json:"app_id" yaml:"app_id" mapstructure:"app_id"`
	Environments []string `json:"environments,omitempty" yaml:"environments,omitempty" mapstructure:"environments,omitempty"`
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
	Access Access `mapstructure:"access" json:"access" yaml:"access"`
}
