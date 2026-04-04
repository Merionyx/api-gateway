package app

import (
	"context"
	"fmt"
	"os"

	"merionyx/api-gateway/internal/api-server/config"
	"merionyx/api-gateway/internal/api-server/container"
	"merionyx/api-gateway/internal/api-server/server"
	"merionyx/api-gateway/internal/shared/metricshttp"
	"merionyx/api-gateway/internal/shared/serviceapp"
)

func Run() error {
	logger := serviceapp.NewJSONLogger()
	serviceapp.SetDefaultLogger(logger)

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

	// Start HTTP server
	go func() {
		if err := server.StartHTTPServer(cnt); err != nil {
			logger.Error(fmt.Sprintf("HTTP server error: %v", err))
			cancel()
		}
	}()

	go func() {
		if err := metricshttp.ListenAndServe(cfg.MetricsHTTP); err != nil {
			logger.Error(fmt.Sprintf("metrics HTTP error: %v", err))
			cancel()
		}
	}()

	// Start gRPC server
	go func() {
		if err := server.StartGRPCServer(cnt); err != nil {
			logger.Error(fmt.Sprintf("gRPC server error: %v", err))
			cancel()
		}
	}()

	serviceapp.WaitSignalOrContext(ctx)

	logger.Info("Shutting down servers...")
	return nil
}
