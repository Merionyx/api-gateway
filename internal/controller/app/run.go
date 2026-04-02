package controller

import (
	"context"
	"fmt"
	"log/slog"
	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/container"
	"merionyx/api-gateway/internal/controller/server"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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
	logger.Info("Config loade", "config", cfg)

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

	container.StartKubernetesDiscovery(ctx)

	// Start HTTP server
	go func() {
		if err := server.StartHTTPServer(container); err != nil {
			logger.Error(fmt.Sprintf("HTTP server error: %v", err))
			cancel()
		}
	}()

	// Start gRPC server
	go func() {
		if err := server.StartGRPCServer(container); err != nil {
			logger.Error(fmt.Sprintf("gRPC server error: %v", err))
			cancel()
		}
	}()

	// Start xDS server
	go func() {
		xdsPort, err := strconv.Atoi(container.Config.Server.XDSPort)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to convert xDS port to int: %v", err))
			cancel()
		}
		if err := container.XDSServer.Start(xdsPort); err != nil {
			logger.Error(fmt.Sprintf("xDS server error: %v", err))
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
