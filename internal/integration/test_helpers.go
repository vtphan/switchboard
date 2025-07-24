package integration

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	
	_ "github.com/mattn/go-sqlite3"
)

// InitializeTestDatabase creates a test database and applies migrations
func InitializeTestDatabase(t *testing.T, dbPath string) {
	// Open database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()
	
	// Read migration file
	migrationPath := filepath.Join("..", "..", "migrations", "001_initial_schema.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("Failed to read migration file: %v", err)
	}
	
	// Apply migration
	if _, err := db.Exec(string(migration)); err != nil {
		t.Fatalf("Failed to apply migration: %v", err)
	}
}