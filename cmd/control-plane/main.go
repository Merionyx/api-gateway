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
