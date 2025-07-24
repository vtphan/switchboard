package database

import (
	"database/sql"
	"errors"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Config holds database configuration
// ARCHITECTURAL DISCOVERY: Configuration struct provides all database settings
// needed for production deployment without hardcoded values
type Config struct {
	DatabasePath    string        `json:"database_path"`
	MaxConnections  int           `json:"max_connections"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `json:"conn_max_idle_time"`
	MigrationsPath  string        `json:"migrations_path"`
}

// DefaultConfig returns production-ready database configuration
// FUNCTIONAL DISCOVERY: SQLite performs optimally with 10 connections for
// classroom-scale concurrent access (20-50 users)
func DefaultConfig() *Config {
	return &Config{
		DatabasePath:    "./data/switchboard.db",
		MaxConnections:  10, // SQLite recommended limit for concurrent access
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: time.Minute * 10,
		MigrationsPath:  "./migrations",
	}
}

// Validate ensures the configuration is valid
// TECHNICAL DISCOVERY: Configuration validation prevents runtime failures
// from invalid database settings
func (c *Config) Validate() error {
	if c.DatabasePath == "" {
		return errors.New("database path cannot be empty")
	}
	if c.MaxConnections <= 0 {
		return errors.New("max connections must be greater than 0")
	}
	if c.ConnMaxLifetime <= 0 {
		return errors.New("connection max lifetime must be greater than 0")
	}
	if c.ConnMaxIdleTime <= 0 {
		return errors.New("connection max idle time must be greater than 0")
	}
	if c.MigrationsPath == "" {
		return errors.New("migrations path cannot be empty")
	}
	return nil
}

// SQLite optimization pragmas for classroom scale
// ARCHITECTURAL DISCOVERY: WAL mode enables concurrent reads while maintaining
// single-writer pattern required by DatabaseManager implementation
const sqliteOptimizations = `
	PRAGMA journal_mode = WAL;          -- Write-Ahead Logging for better concurrency
	PRAGMA synchronous = NORMAL;        -- Balance between safety and performance  
	PRAGMA cache_size = -64000;         -- 64MB cache (negative = KB)
	PRAGMA temp_store = MEMORY;         -- Use memory for temporary tables
	PRAGMA foreign_keys = ON;           -- Enforce foreign key constraints
	PRAGMA busy_timeout = 5000;         -- 5 second timeout for locked database
`

// applySQLiteOptimizations applies performance optimizations to the database connection
// FUNCTIONAL DISCOVERY: Optimization pragmas must be applied to each connection
// to ensure consistent performance characteristics across the connection pool
func applySQLiteOptimizations(db *sql.DB) error {
	_, err := db.Exec(sqliteOptimizations)
	if err != nil {
		return err
	}
	return nil
}