package database

import (
	"context"
	"fmt"
	"log/slog"

	"merionyx/api-gateway/control-plane/internal/config"
	"merionyx/api-gateway/control-plane/internal/database/postgres"
)

// Pseudo-types for avoiding circular imports
type DatabaseType = postgres.DatabaseType
type Connection = postgres.Connection
type ConnectionInfo = postgres.ConnectionInfo

const (
	DatabaseTypePostgreSQL = postgres.DatabaseTypePostgreSQL
)

// ConnectionFactory creates connections to different database types
type ConnectionFactory interface {
	CreateConnection(ctx context.Context, name string, config config.DatabaseConfig) (Connection, error)
	SupportedTypes() []DatabaseType
}

// DatabaseManager manages multiple connections to databases
type DatabaseManager struct {
	connections map[string]Connection
	factories   map[DatabaseType]ConnectionFactory
}

// NewDatabaseManager creates a new database manager
func NewDatabaseManager() *DatabaseManager {
	dm := &DatabaseManager{
		connections: make(map[string]Connection),
		factories:   make(map[DatabaseType]ConnectionFactory),
	}

	// Register factories for supported database types
	dm.RegisterFactory(DatabaseTypePostgreSQL, postgres.NewPostgreSQLFactory())

	return dm
}

// RegisterFactory registers a factory for a specific database type
func (dm *DatabaseManager) RegisterFactory(dbType DatabaseType, factory ConnectionFactory) {
	dm.factories[dbType] = factory
}

// AddConnection adds a new connection to a database
func (dm *DatabaseManager) AddConnection(name string, conn Connection) {
	dm.connections[name] = conn
}

// GetConnection returns a connection to a database by name
func (dm *DatabaseManager) GetConnection(name string) Connection {
	return dm.connections[name]
}

// GetDefaultConnection returns the default database connection
func (dm *DatabaseManager) GetDefaultConnection() Connection {
	return dm.connections["default"]
}

// GetPostgreSQLConnection returns a PostgreSQL connection by name (typed method)
func (dm *DatabaseManager) GetPostgreSQLConnection(name string) *postgres.PostgreSQLConnection {
	conn := dm.connections[name]
	if conn != nil && conn.Type() == DatabaseTypePostgreSQL {
		if pgConn, ok := conn.(*postgres.PostgreSQLConnection); ok {
			return pgConn
		}
	}
	return nil
}

// GetDefaultPostgreSQLConnection returns the default PostgreSQL connection
func (dm *DatabaseManager) GetDefaultPostgreSQLConnection() *postgres.PostgreSQLConnection {
	return dm.GetPostgreSQLConnection("default")
}

// GetAllConnections returns all connections
func (dm *DatabaseManager) GetAllConnections() map[string]Connection {
	return dm.connections
}

// GetConnectionsByType returns connections of a specific type
func (dm *DatabaseManager) GetConnectionsByType(dbType DatabaseType) map[string]Connection {
	result := make(map[string]Connection)
	for name, conn := range dm.connections {
		if conn.Type() == dbType {
			result[name] = conn
		}
	}
	return result
}

// Close closes all connections to databases
func (dm *DatabaseManager) Close() {
	for name, conn := range dm.connections {
		slog.Info(fmt.Sprintf("Closing database connection %s: %s", name, conn.Type()))
		if err := conn.Close(); err != nil {
			slog.Error(fmt.Sprintf("Error closing connection %s: %v", name, err))
		}
	}
}

// Health checks the status of all connections to databases
func (dm *DatabaseManager) Health(ctx context.Context) map[string]error {
	results := make(map[string]error)
	for name, conn := range dm.connections {
		results[name] = conn.Health(ctx)
	}
	return results
}

// GetConnectionsInfo returns information about all connections
func (dm *DatabaseManager) GetConnectionsInfo() []ConnectionInfo {
	var infos []ConnectionInfo
	for name, conn := range dm.connections {
		info := ConnectionInfo{
			Name:   name,
			Type:   conn.Type(),
			Status: "connected",
		}

		// For PostgreSQL, get additional information
		if pgConn, ok := conn.(*postgres.PostgreSQLConnection); ok {
			pgInfo := pgConn.GetConnectionInfo()
			info.Host = pgInfo.Host
			info.Port = pgInfo.Port
			info.Database = pgInfo.Database
			info.ConnectedAt = pgInfo.ConnectedAt
		}

		infos = append(infos, info)
	}
	return infos
}

// InitializeDatabase initializes a single database from the configuration
func InitializeDatabase(ctx context.Context, cfg *config.Config) (*DatabaseManager, error) {
	dm := NewDatabaseManager()

	// Use the single database configuration
	dbConfig := cfg.Database
	name := "default" // Используем стандартное имя для единственной БД

	slog.Info(fmt.Sprintf("Initializing %s database: %s:%d/%s",
		dbConfig.Type, dbConfig.Host, dbConfig.Port, dbConfig.Database))

	conn, err := dm.createConnection(ctx, name, dbConfig)
	if err != nil {
		dm.Close()
		return nil, fmt.Errorf("failed to connect to %s database: %w", dbConfig.Type, err)
	}

	dm.AddConnection(name, conn)
	slog.Info(fmt.Sprintf("Successfully connected to %s database", dbConfig.Type))

	return dm, nil
}

// createConnection creates a connection using the corresponding factory
func (dm *DatabaseManager) createConnection(ctx context.Context, name string, dbConfig config.DatabaseConfig) (Connection, error) {
	dbType := DatabaseType(dbConfig.Type)

	factory, exists := dm.factories[dbType]
	if !exists {
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	return factory.CreateConnection(ctx, name, dbConfig)
}
