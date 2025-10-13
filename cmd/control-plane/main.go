package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"merionyx/api-gateway/control-plane/internal/config"
	"merionyx/api-gateway/control-plane/internal/database"
	"merionyx/api-gateway/control-plane/internal/database/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	slog.SetDefault(logger)

	slog.Info("Info message")

	// Load config
	cfg, err := config.LoadConfig(os.Getenv("CONFIG_PATH"))
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to load config: %v", err))
	}

	// Initialize all databases from the configuration
	dbManager, err := database.InitializeAllDatabases(context.Background(), cfg)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to initialize databases: %v", err))
	}
	defer dbManager.Close()

	// Log all initialized databases
	connections := dbManager.GetAllConnections()
	if len(connections) > 0 {
		logger.Info(fmt.Sprintf("Successfully initialized %d database connection(s):", len(connections)), "connections", connections)
	} else {
		logger.Info("No databases configured, running with mock repositories")
	}

	// Run migrations for all databases
	if err := migrations.RunMigrations(cfg); err != nil {
		logger.Error(fmt.Sprintf("Failed to run migrations: %v", err))
	}
}
