package postgres

import (
	"context"
	"fmt"

	"merionyx/api-gateway/control-plane/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Factory simple factory for creating connections to PostgreSQL
type Factory struct {
	config *config.Config
}

// NewFactory creates a new factory
func NewFactory(cfg *config.Config) *Factory {
	return &Factory{
		config: cfg,
	}
}

// CreateConnection creates a connection to PostgreSQL
func (f *Factory) CreateConnection() (*pgxpool.Pool, error) {
	// Use the database configuration from config
	dbConfig := f.config.Database
	
	// Form the connection string from the configuration
	dsn := dbConfig.GetPostgresConnectionString()

	// Create the connection pool configuration
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Configure connection pool settings from options
	if dbConfig.Options != nil {
		if maxOpenConns, ok := dbConfig.Options["max_open_conns"].(int); ok && maxOpenConns > 0 {
			poolConfig.MaxConns = int32(maxOpenConns)
		}
		if maxIdleConns, ok := dbConfig.Options["max_idle_conns"].(int); ok && maxIdleConns > 0 {
			poolConfig.MinConns = int32(maxIdleConns)
		}
	}

	// Create the connection pool
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Check the connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}
