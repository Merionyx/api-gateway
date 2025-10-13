package migrations

import (
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"merionyx/api-gateway/control-plane/internal/config"
)

// RunMigrations runs migrations for the configured database
func RunMigrations(cfg *config.Config) error {
	dbConfig := cfg.Database
	
	if dbConfig.Type != "postgresql" {
		slog.Info(fmt.Sprintf("Skipping non-postgresql database type: %s", dbConfig.Type))
		return nil
	}

	slog.Info("Running migrations for database")

	if err := runMigrationForDB(&dbConfig); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("Migrations completed successfully")
	return nil
}

// RunMigrationForDatabase runs migrations for the database (deprecated, use RunMigrations)
func RunMigrationForDatabase(cfg *config.Config, dbName string) error {
	slog.Info("RunMigrationForDatabase is deprecated, using single database configuration")
	return RunMigrations(cfg)
}

func runMigrationForDB(dbConfig *config.DatabaseConfig) error {
	// Form the path to the migrations
	migrationsPath := dbConfig.Options["migrations_path"].(string)
	if migrationsPath == "" {
		migrationsPath = "./databases/postgres/migrations"
	}

	// Create a migrate instance
	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		dbConfig.GetPostgresConnectionString(),
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// RollbackMigrations rolls back migrations for the database
func RollbackMigrations(cfg *config.Config, steps int) error {
	dbConfig := cfg.Database
	
	if dbConfig.Type != "postgresql" {
		slog.Info(fmt.Sprintf("Skipping non-postgresql database type: %s", dbConfig.Type))
		return nil
	}

	slog.Info("Rolling back migrations for database")

	if err := rollbackMigrationForDB(&dbConfig, steps); err != nil {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}

	slog.Info("Rollback completed successfully")
	return nil
}

func rollbackMigrationForDB(dbConfig *config.DatabaseConfig, steps int) error {
	migrationsPath := dbConfig.Options["migrations_path"].(string)
	if migrationsPath == "" {
		migrationsPath = "./databases/postgres/migrations"
	}

	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		dbConfig.GetPostgresConnectionString(),
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Steps(-steps); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}

	return nil
}
