package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabaseType represents the type of database
type DatabaseType string

const (
	DatabaseTypePostgreSQL DatabaseType = "postgresql"
)

// Connection represents a universal connection to a database
type Connection interface {
	Type() DatabaseType
	Name() string
	Health(ctx context.Context) error
	Close() error
	GetNativeConnection() interface{}
}

// ConnectionInfo contains information about the connection
type ConnectionInfo struct {
	Name            string       `json:"name"`
	Type            DatabaseType `json:"type"`
	Host            string       `json:"host"`
	Port            int          `json:"port"`
	Database        string       `json:"database"`
	Status          string       `json:"status"`
	ConnectedAt     time.Time    `json:"connected_at"`
	LastHealthCheck time.Time    `json:"last_health_check"`
}

// PostgreSQLConnection implements Connection for PostgreSQL
type PostgreSQLConnection struct {
	name        string
	pool        *pgxpool.Pool
	connectedAt time.Time
	config      PostgreSQLConfig
}

// PostgreSQLConfig contains the configuration for PostgreSQL
type PostgreSQLConfig struct {
	Host           string `yaml:"host" mapstructure:"host"`
	Port           int    `yaml:"port" mapstructure:"port"`
	Username       string `yaml:"username" mapstructure:"username"`
	Password       string `yaml:"password" mapstructure:"password"`
	Database       string `yaml:"database" mapstructure:"database"`
	SSLMode        string `yaml:"ssl_mode" mapstructure:"ssl_mode"`
	MaxOpenConns   int    `yaml:"max_open_conns" mapstructure:"max_open_conns"`
	MaxIdleConns   int    `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
	MigrationsPath string `yaml:"migrations_path" mapstructure:"migrations_path"`
}

// GetConnectionString returns the connection string to PostgreSQL
func (c *PostgreSQLConfig) GetConnectionString() string {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.Username, c.Password, c.Host, c.Port, c.Database, sslMode)
}

// NewPostgreSQLConnection creates a new connection to PostgreSQL
func NewPostgreSQLConnection(ctx context.Context, name string, config PostgreSQLConfig) (*PostgreSQLConnection, error) {
	// Create the configuration of the connection pool
	poolConfig, err := pgxpool.ParseConfig(config.GetConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to parse PostgreSQL config: %w", err)
	}

	// Configure the connection pool
	if config.MaxOpenConns > 0 {
		poolConfig.MaxConns = int32(config.MaxOpenConns)
	} else {
		poolConfig.MaxConns = 10 // default value
	}

	if config.MaxIdleConns > 0 {
		poolConfig.MinConns = int32(config.MaxIdleConns)
	} else {
		poolConfig.MinConns = 2 // default value
	}

	// Configure the timeouts
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = time.Minute * 30

	// Create the connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL connection pool: %w", err)
	}

	// Check the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
	}

	slog.Info(fmt.Sprintf("Successfully connected to PostgreSQL database %s: %s:%d/%s", name, config.Host, config.Port, config.Database))

	return &PostgreSQLConnection{
		name:        name,
		pool:        pool,
		connectedAt: time.Now(),
		config:      config,
	}, nil
}

// Type returns the type of the database
func (c *PostgreSQLConnection) Type() DatabaseType {
	return DatabaseTypePostgreSQL
}

// Name returns the name of the connection
func (c *PostgreSQLConnection) Name() string {
	return c.name
}

// Health checks the state of the connection
func (c *PostgreSQLConnection) Health(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

// Close closes the connection
func (c *PostgreSQLConnection) Close() error {
	if c.pool != nil {
		c.pool.Close()
	}
	return nil
}

// GetNativeConnection returns the native connection pgxpool.Pool
func (c *PostgreSQLConnection) GetNativeConnection() interface{} {
	return c.pool
}

// GetPool returns the connection pool of PostgreSQL (typed method)
func (c *PostgreSQLConnection) GetPool() *pgxpool.Pool {
	return c.pool
}

// GetConfig returns the configuration of PostgreSQL
func (c *PostgreSQLConnection) GetConfig() PostgreSQLConfig {
	return c.config
}

// GetConnectionInfo returns the information about the connection
func (c *PostgreSQLConnection) GetConnectionInfo() ConnectionInfo {
	return ConnectionInfo{
		Name:            c.name,
		Type:            c.Type(),
		Host:            c.config.Host,
		Port:            c.config.Port,
		Database:        c.config.Database,
		Status:          "connected",
		ConnectedAt:     c.connectedAt,
		LastHealthCheck: time.Now(),
	}
}
