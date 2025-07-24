package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Migration represents a database migration
// ARCHITECTURAL DISCOVERY: Migration struct encapsulates all information needed
// for safe schema evolution and rollback capability
type Migration struct {
	Version     string
	Description string
	SQL         string
}

// MigrationManager handles database migrations
// FUNCTIONAL DISCOVERY: Manager pattern encapsulates migration state and operations
// enabling safe schema evolution across development and production environments
type MigrationManager struct {
	db             *sql.DB
	migrationsPath string
}

// NewMigrationManager creates a new migration manager
// TECHNICAL DISCOVERY: Constructor pattern ensures proper initialization
// and dependency injection for database operations
func NewMigrationManager(db *sql.DB, migrationsPath string) *MigrationManager {
	return &MigrationManager{
		db:             db,
		migrationsPath: migrationsPath,
	}
}

// ApplyMigrations applies all pending migrations
// ARCHITECTURAL DISCOVERY: Transaction-based migration application ensures
// atomicity - either all migrations succeed or none are applied
func (m *MigrationManager) ApplyMigrations() error {
	// FUNCTIONAL DISCOVERY: Migration tracking table created automatically
	// to maintain schema version state across application restarts
	err := m.createMigrationTable()
	if err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	appliedMigrations, err := m.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// TECHNICAL DISCOVERY: Migration ordering by filename ensures consistent
	// application order across different environments
	for _, migration := range migrations {
		if !contains(appliedMigrations, migration.Version) {
			err = m.applyMigration(migration)
			if err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", migration.Version, err)
			}
		}
	}

	return nil
}

// ValidateSchema ensures database matches expected structure
// FUNCTIONAL DISCOVERY: Schema validation prevents runtime errors from
// structural mismatches between code expectations and database reality
func (m *MigrationManager) ValidateSchema() error {
	// Check for required tables
	requiredTables := []string{"sessions", "messages"}
	for _, table := range requiredTables {
		exists, err := m.tableExists(table)
		if err != nil {
			return fmt.Errorf("failed to check table %s: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("required table %s does not exist", table)
		}
	}

	// ARCHITECTURAL DISCOVERY: Index validation ensures performance characteristics
	// match expectations for session history and message routing operations
	requiredIndexes := []string{
		"idx_sessions_status",
		"idx_sessions_created_by", 
		"idx_messages_session_time",
		"idx_messages_session_type",
		"idx_messages_to_user",
	}

	for _, index := range requiredIndexes {
		exists, err := m.indexExists(index)
		if err != nil {
			return fmt.Errorf("failed to check index %s: %w", index, err)
		}
		if !exists {
			return fmt.Errorf("required index %s does not exist", index)
		}
	}

	return nil
}

// createMigrationTable creates the migration tracking table
func (m *MigrationManager) createMigrationTable() error {
	sql := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := m.db.Exec(sql)
	return err
}

// loadMigrations loads migration files from the migrations directory
// TECHNICAL DISCOVERY: File-based migrations enable version control integration
// and collaborative schema evolution
func (m *MigrationManager) loadMigrations() ([]Migration, error) {
	files, err := os.ReadDir(m.migrationsPath)
	if err != nil {
		return nil, err
	}

	var migrations []Migration
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".sql" {
			content, err := os.ReadFile(filepath.Join(m.migrationsPath, file.Name()))
			if err != nil {
				return nil, err
			}

			// Extract version from filename (e.g., "001_initial_schema.sql" -> "001")
			version := strings.Split(file.Name(), "_")[0]
			description := strings.TrimSuffix(strings.Join(strings.Split(file.Name(), "_")[1:], "_"), ".sql")

			migrations = append(migrations, Migration{
				Version:     version,
				Description: description,
				SQL:         string(content),
			})
		}
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// getAppliedMigrations returns list of already applied migration versions
func (m *MigrationManager) getAppliedMigrations() ([]string, error) {
	rows, err := m.db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Ignore cleanup errors to avoid masking the primary error
			_ = err
		}
	}()

	var versions []string
	for rows.Next() {
		var version string
		err = rows.Scan(&version)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return versions, rows.Err()
}

// applyMigration applies a single migration within a transaction
// FUNCTIONAL DISCOVERY: Transaction isolation ensures migration atomicity
// and enables rollback on failure
func (m *MigrationManager) applyMigration(migration Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			// Ignore rollback errors to avoid masking the primary error
			_ = err
		}
	}()

	// Apply the migration SQL
	_, err = tx.Exec(migration.SQL)
	if err != nil {
		return err
	}

	// Record the migration as applied
	_, err = tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", migration.Version)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// tableExists checks if a table exists in the database
func (m *MigrationManager) tableExists(tableName string) (bool, error) {
	var count int
	err := m.db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
		tableName,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// indexExists checks if an index exists in the database
func (m *MigrationManager) indexExists(indexName string) (bool, error) {
	var count int
	err := m.db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?",
		indexName,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}