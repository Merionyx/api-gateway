package git

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
)

func setupSSHAuth(config AuthConfig) (transport.AuthMethod, error) {
	var privateKeyPath string

	if config.SSHKeyPath != "" {
		privateKeyPath = config.SSHKeyPath
	} else if config.SSHKeyEnv != "" {
		privateKeyPath = os.Getenv(config.SSHKeyEnv)
	} else {
		return nil, fmt.Errorf("no SSH key path or environment variable specified")
	}

	auth, err := ssh.NewPublicKeysFromFile("git", privateKeyPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key: %w", err)
	}

	return auth, nil
}

func setupTokenAuth(config AuthConfig) (transport.AuthMethod, error) {
	if config.TokenEnv == "" {
		return nil, fmt.Errorf("no token environment variable specified")
	}

	token := os.Getenv(config.TokenEnv)
	if token == "" {
		return nil, fmt.Errorf("token environment variable %s is empty", config.TokenEnv)
	}

	return &http.BasicAuth{
		Username: "git",
		Password: token,
	}, nil
}
