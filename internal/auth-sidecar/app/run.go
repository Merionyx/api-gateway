package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"merionyx/api-gateway/internal/auth-sidecar/config"
	"merionyx/api-gateway/internal/auth-sidecar/container"
	"merionyx/api-gateway/internal/auth-sidecar/server"
)

func Run() error {
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
	logger.Info("Config loaded", "config", cfg)

	// Initialize DI container
	cnt, err := container.NewContainer(cfg)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to initialize container: %v", err))
		os.Exit(1)
	}
	defer cnt.Close()

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start gRPC server (ext_authz)
	go func() {
		if err := server.StartExtAuthzServer(cnt); err != nil {
			logger.Error(fmt.Sprintf("ExtAuthz server error: %v", err))
			cancel()
		}
	}()

	// Start sync client (connection to Controller)
	go func() {
		if err := cnt.SyncClient.Start(ctx); err != nil {
			logger.Error(fmt.Sprintf("Sync client error: %v", err))
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
	return nil
}
