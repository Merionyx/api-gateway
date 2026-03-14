package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"merionyx/api-gateway/control-plane/internal/config"
	"merionyx/api-gateway/control-plane/internal/container"
	environmentv1 "merionyx/api-gateway/control-plane/pkg/api/environment/v1"
	listenerv1 "merionyx/api-gateway/control-plane/pkg/api/listener/v1"
	tenantv1 "merionyx/api-gateway/control-plane/pkg/api/tenant/v1"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/transport"
	git_http "github.com/go-git/go-git/v6/plumbing/transport/http"
	git_ssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"github.com/go-git/go-git/v6/storage/memory"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	slog.SetDefault(logger)

	// Load config
	cfg, err := config.LoadConfig(os.Getenv("CONFIG_PATH"))
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to load config: %v", err))
		os.Exit(1)
	}
	logger.Info("Config loade", "config", cfg)

	// Initialize repositories
	if err := initializeRepositories(cfg.Repositories); err != nil {
		logger.Error(fmt.Sprintf("Failed to initialize repositories: %v", err))
		os.Exit(1)
	}

	// Initialize DI container
	container, err := container.NewContainer(cfg)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to initialize container: %v", err))
		os.Exit(1)
	}
	defer container.Close()

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start HTTP server
	go func() {
		if err := startHTTPServer(container); err != nil {
			logger.Error(fmt.Sprintf("HTTP server error: %v", err))
			cancel()
		}
	}()

	// Start gRPC server
	go func() {
		if err := startGRPCServer(container); err != nil {
			logger.Error(fmt.Sprintf("gRPC server error: %v", err))
			cancel()
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		logger.Info("Shutdown signal received")
	case <-ctx.Done():
		logger.Info("Context cancelled")
	}

	logger.Info("Shutting down servers...")
}

func startHTTPServer(container *container.Container) error {
	// Setup routes
	handler := container.Router.SetupRoutes()

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	log.Printf("HTTP server starting on :8080")
	return server.ListenAndServe()
}

func startGRPCServer(container *container.Container) error {
	lis, err := net.Listen("tcp", ":"+container.Config.Server.GRPCPort)
	if err != nil {
		return fmt.Errorf("failed to listen on :%s: %w", container.Config.Server.GRPCPort, err)
	}

	server := grpc.NewServer()

	// Register services
	tenantv1.RegisterTenantServiceServer(server, container.TenantGRPCHandler)
	environmentv1.RegisterEnvironmentServiceServer(server, container.EnvironmentGRPCHandler)
	listenerv1.RegisterListenerServiceServer(server, container.ListenerGRPCHandler)

	reflection.Register(server)

	log.Printf("gRPC server starting on :9090")
	return server.Serve(lis)
}

func initializeRepositories(repos []config.RepositoryConfig) error {
	slog.Info("Initializing repositories", "repositories", repos)

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
			// Without authentication
			auth = nil
		default:
			return fmt.Errorf("unsupported auth type %s for repository %s", repository.Auth.Type, repository.Name)
		}

		// Clone repository
		_, err = git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
			URL:  repository.URL,
			Auth: auth,
		})

		if err != nil {
			return fmt.Errorf("failed to clone repository %s: %w", repository.Name, err)
		}

		slog.Info("Successfully cloned repository", "name", repository.Name)
	}

	return nil
}

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
