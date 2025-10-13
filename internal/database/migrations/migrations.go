package migrations

import (
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"merionyx/api-gateway/control-plane/internal/config"
)

// RunMigrations runs migrations for all configured databases
func RunMigrations(cfg *config.Config) error {
	for dbName, dbConfig := range cfg.Databases {
		if dbConfig.Type != "postgresql" {
			slog.Info(fmt.Sprintf("Skipping non-postgresql database %s", dbName))
			continue
		}

		slog.Info(fmt.Sprintf("Running migrations for database %s", dbName))

		if err := runMigrationForDB(dbName, &dbConfig); err != nil {
			return fmt.Errorf("failed to run migrations for %s: %w", dbName, err)
		}

		slog.Info(fmt.Sprintf("Migrations completed successfully for database %s", dbName))
	}

	return nil
}

// RunMigrationForDatabase runs migrations for a specific database
func RunMigrationForDatabase(cfg *config.Config, dbName string) error {
	dbConfig, exists := cfg.Databases[dbName]
	if !exists {
		slog.Info(fmt.Sprintf("Database configuration not found %s", dbName))
		return nil
	}

	return runMigrationForDB(dbName, &dbConfig)
}

func runMigrationForDB(dbName string, dbConfig *config.DatabaseConfig) error {
	// Form the path to the migrations
	migrationsPath := dbConfig.Options["migrations_path"].(string)
	if migrationsPath == "" {
		migrationsPath = fmt.Sprintf("./migrations/postgres/%s", dbName)
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

// RollbackMigrations rolls back migrations for all databases
func RollbackMigrations(cfg *config.Config, steps int) error {
	for dbName, dbConfig := range cfg.Databases {
		if dbConfig.Type != "postgresql" {
			slog.Info(fmt.Sprintf("Skipping non-postgresql database %s", dbName))
			continue
		}

		slog.Info(fmt.Sprintf("Rolling back migrations for database %s", dbName))

		if err := rollbackMigrationForDB(dbName, &dbConfig, steps); err != nil {
			return fmt.Errorf("failed to rollback migrations for %s: %w", dbName, err)
		}

		slog.Info(fmt.Sprintf("Rollback completed successfully for database %s", dbName))
	}

	return nil
}

func rollbackMigrationForDB(dbName string, dbConfig *config.DatabaseConfig, steps int) error {
	migrationsPath := dbConfig.Options["migrations_path"].(string)
	if migrationsPath == "" {
		migrationsPath = fmt.Sprintf("./migrations/postgres/%s", dbName)
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
