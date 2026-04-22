package git

import (
	"fmt"
	"os"

	gitclient "github.com/go-git/go-git/v6/plumbing/client"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
)

func resolveTransportAuth(cfg AuthConfig, repoName string) ([]gitclient.Option, error) {
	switch cfg.Type {
	case "ssh":
		opts, err := setupSSHAuth(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to setup SSH auth for repository %s: %w", repoName, err)
		}
		return opts, nil
	case "token":
		opts, err := setupTokenAuth(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to setup token auth for repository %s: %w", repoName, err)
		}
		return opts, nil
	case "none", "":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported auth type %s for repository %s", cfg.Type, repoName)
	}
}

func setupSSHAuth(config AuthConfig) ([]gitclient.Option, error) {
	var privateKeyPath string

	if config.SSHKeyPath != "" {
		privateKeyPath = config.SSHKeyPath
	} else if config.SSHKeyEnv != "" {
		privateKeyPath = os.Getenv(config.SSHKeyEnv)
	} else {
		return nil, fmt.Errorf("no SSH key path or environment variable specified")
	}

	pk, err := ssh.NewPublicKeysFromFile("git", privateKeyPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key: %w", err)
	}

	return []gitclient.Option{gitclient.WithSSHAuth(pk)}, nil
}

func setupTokenAuth(config AuthConfig) ([]gitclient.Option, error) {
	if config.TokenEnv == "" {
		return nil, fmt.Errorf("no token environment variable specified")
	}

	token := os.Getenv(config.TokenEnv)
	if token == "" {
		return nil, fmt.Errorf("token environment variable %s is empty", config.TokenEnv)
	}

	return []gitclient.Option{
		gitclient.WithHTTPAuth(&http.BasicAuth{
			Username: "git",
			Password: token,
		}),
	}, nil
}
