package postgres

import (
	"context"
	"fmt"

	"merionyx/api-gateway/control-plane/internal/config"
)

// PostgreSQLFactory creates connections to PostgreSQL
type PostgreSQLFactory struct{}

// NewPostgreSQLFactory creates a new factory for PostgreSQL
func NewPostgreSQLFactory() *PostgreSQLFactory {
	return &PostgreSQLFactory{}
}

// CreateConnection creates a connection to PostgreSQL
func (f *PostgreSQLFactory) CreateConnection(ctx context.Context, name string, config config.DatabaseConfig) (Connection, error) {
	if config.Type != "postgresql" {
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}

	// Convert the general configuration to PostgreSQL-specific
	pgConfig := PostgreSQLConfig{
		Host:     config.Host,
		Port:     config.Port,
		Username: config.Username,
		Password: config.Password,
		Database: config.Database,
	}

	// Extract PostgreSQL-specific options
	if config.Options != nil {
		if sslMode, ok := config.Options["ssl_mode"].(string); ok {
			pgConfig.SSLMode = sslMode
		}
		if maxOpenConns, ok := config.Options["max_open_conns"].(int); ok {
			pgConfig.MaxOpenConns = maxOpenConns
		}
		if maxIdleConns, ok := config.Options["max_idle_conns"].(int); ok {
			pgConfig.MaxIdleConns = maxIdleConns
		}
		if migrationsPath, ok := config.Options["migrations_path"].(string); ok {
			pgConfig.MigrationsPath = migrationsPath
		}
	}

	return NewPostgreSQLConnection(ctx, name, pgConfig)
}

// SupportedTypes returns the supported types of databases
func (f *PostgreSQLFactory) SupportedTypes() []DatabaseType {
	return []DatabaseType{DatabaseTypePostgreSQL}
}
