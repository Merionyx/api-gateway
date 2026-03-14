package git

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v6/plumbing/transport"
	git_http "github.com/go-git/go-git/v6/plumbing/transport/http"
	git_ssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"

	"merionyx/api-gateway/control-plane/internal/config"
)

// setupSSHAuth setup SSH authentication
func setupSSHAuth(authConfig config.AuthConfig) (transport.AuthMethod, error) {
	var privateKey []byte
	var err error

	// Get private key from file or environment variable
	if authConfig.SSHKeyPath != "" {
		privateKey, err = os.ReadFile(authConfig.SSHKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read SSH key from file %s: %w", authConfig.SSHKeyPath, err)
		}
	} else if authConfig.SSHKeyEnv != "" {
		keyContent := os.Getenv(authConfig.SSHKeyEnv)
		if keyContent == "" {
			return nil, fmt.Errorf("SSH key environment variable %s is empty", authConfig.SSHKeyEnv)
		}
		privateKey = []byte(keyContent)
	} else {
		return nil, fmt.Errorf("neither ssh_key_path nor ssh_key_env is specified")
	}

	// Create SSH authentication
	var publicKeys *git_ssh.PublicKeys
	if authConfig.SSHKeyPath != "" {
		publicKeys, err = git_ssh.NewPublicKeysFromFile("git", authConfig.SSHKeyPath, "")
	} else {
		publicKeys, err = git_ssh.NewPublicKeys("git", privateKey, "")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH public keys: %w", err)
	}

	return publicKeys, nil
}

// setupTokenAuth setup token authentication (for HTTPS)
func setupTokenAuth(authConfig config.AuthConfig) (transport.AuthMethod, error) {
	token := os.Getenv(authConfig.TokenEnv)
	if token == "" {
		return nil, fmt.Errorf("token environment variable %s is empty", authConfig.TokenEnv)
	}

	return &git_http.BasicAuth{
		Username: token,
		Password: "",
	}, nil
}
