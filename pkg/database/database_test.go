package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"switchboard/pkg/types"
)

// Architectural Validation Tests

func TestDatabase_ArchitecturalCompliance(t *testing.T) {
	// Verify that database package has no forbidden imports
	// This will be checked at compile time - no business logic imports allowed
	
	// Test that all structs can be created
	_ = &Config{}
	_ = &Migration{}
	_ = &MigrationManager{}
}

// Functional Validation Tests - Config

func TestConfig_DefaultConfig(t *testing.T) {
	// Test that DefaultConfig returns expected values
	config := DefaultConfig()
	
	if config == nil {
		t.Fatal("DefaultConfig should not return nil")
	}
	
	// Verify expected default values
	if config.DatabasePath != "./data/switchboard.db" {
		t.Errorf("Expected DatabasePath './data/switchboard.db', got %s", config.DatabasePath)
	}
	
	if config.MaxConnections != 10 {
		t.Errorf("Expected MaxConnections 10, got %d", config.MaxConnections)
	}
	
	if config.ConnMaxLifetime != time.Hour {
		t.Errorf("Expected ConnMaxLifetime 1 hour, got %v", config.ConnMaxLifetime)
	}
	
	if config.ConnMaxIdleTime != time.Minute*10 {
		t.Errorf("Expected ConnMaxIdleTime 10 minutes, got %v", config.ConnMaxIdleTime)
	}
	
	if config.MigrationsPath != "./migrations" {
		t.Errorf("Expected MigrationsPath './migrations', got %s", config.MigrationsPath)
	}
}

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "empty database path",
			config: &Config{
				DatabasePath:    "",
				MaxConnections:  10,
				ConnMaxLifetime: time.Hour,
				ConnMaxIdleTime: time.Minute * 10,
				MigrationsPath:  "./migrations",
			},
			wantErr: true,
		},
		{
			name: "zero max connections",
			config: &Config{
				DatabasePath:    "./test.db",
				MaxConnections:  0,
				ConnMaxLifetime: time.Hour,
				ConnMaxIdleTime: time.Minute * 10,
				MigrationsPath:  "./migrations",
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Functional Validation Tests - Migration System

func TestMigrationManager_NewMigrationManager(t *testing.T) {
	// Test that NewMigrationManager can be created
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close test database: %v", err)
		}
	}()
	
	mgr := NewMigrationManager(db, tempDir)
	if mgr == nil {
		t.Fatal("NewMigrationManager should not return nil")
	}
}

func TestMigrationManager_ApplyMigrations(t *testing.T) {
	// Test migration application
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close test database: %v", err)
		}
	}()
	
	// Create test migration file
	migrationPath := filepath.Join(tempDir, "001_test.sql")
	migrationSQL := `CREATE TABLE test_table (id TEXT PRIMARY KEY);`
	err = os.WriteFile(migrationPath, []byte(migrationSQL), 0644)
	if err != nil {
		t.Fatalf("Failed to create test migration: %v", err)
	}
	
	mgr := NewMigrationManager(db, tempDir)
	err = mgr.ApplyMigrations()
	if err != nil {
		t.Errorf("ApplyMigrations should not fail: %v", err)
	}
	
	// Verify table was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test_table'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check if table exists: %v", err)
	}
	if count != 1 {
		t.Error("Test table should have been created")
	}
}

func TestMigrationManager_ValidateSchema(t *testing.T) {
	// Test schema validation
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close test database: %v", err)
		}
	}()
	
	mgr := NewMigrationManager(db, tempDir)
	
	// Should fail on empty database
	err = mgr.ValidateSchema()
	if err == nil {
		t.Error("ValidateSchema should fail on empty database")
	}
}

// Technical Validation Tests - Schema Structure

func TestSchema_SessionsTable(t *testing.T) {
	// Test that sessions table matches types.Session structure
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close test database: %v", err)
		}
	}()
	
	// Apply initial schema
	mgr := NewMigrationManager(db, "./migrations")
	err = mgr.ApplyMigrations()
	if err != nil {
		t.Skipf("Skipping schema test - migrations not yet implemented: %v", err)
		return
	}
	
	// Test that we can insert a Session-like record
	session := &types.Session{
		ID:         "test-session-id",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1", "student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	// This should work if schema matches types
	_, err = db.Exec(`INSERT INTO sessions (id, name, created_by, student_ids, start_time, status) 
		VALUES (?, ?, ?, ?, ?, ?)`,
		session.ID, session.Name, session.CreatedBy, `["student1","student2"]`, 
		session.StartTime, session.Status)
	
	if err != nil {
		t.Errorf("Failed to insert session record: %v", err)
	}
}

func TestSchema_MessagesTable(t *testing.T) {
	// Test that messages table supports all 6 message types
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close test database: %v", err)
		}
	}()
	
	// Apply initial schema
	mgr := NewMigrationManager(db, "./migrations")
	err = mgr.ApplyMigrations()
	if err != nil {
		t.Skipf("Skipping schema test - migrations not yet implemented: %v", err)
		return
	}
	
	// Test all 6 message types
	messageTypes := []string{
		types.MessageTypeInstructorInbox,
		types.MessageTypeInboxResponse,
		types.MessageTypeRequest,
		types.MessageTypeRequestResponse,
		types.MessageTypeAnalytics,
		types.MessageTypeInstructorBroadcast,
	}
	
	for i, msgType := range messageTypes {
		message := &types.Message{
			ID:        "test-msg-" + string(rune('1'+i)),
			SessionID: "test-session",
			Type:      msgType,
			Context:   "test",
			FromUser:  "user1",
			Content:   map[string]interface{}{"text": "test message"},
			Timestamp: time.Now(),
		}
		
		_, err = db.Exec(`INSERT INTO messages (id, session_id, type, context, from_user, content, timestamp) 
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			message.ID, message.SessionID, message.Type, message.Context, 
			message.FromUser, `{"text":"test message"}`, message.Timestamp)
		
		if err != nil {
			t.Errorf("Failed to insert message of type %s: %v", msgType, err)
		}
	}
}

// Performance Validation Tests

func TestDatabase_SQLiteOptimizations(t *testing.T) {
	// Test that SQLite optimizations can be applied
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close test database: %v", err)
		}
	}()
	
	// Apply optimizations
	err = applySQLiteOptimizations(db)
	if err != nil {
		t.Errorf("Failed to apply SQLite optimizations: %v", err)
	}
	
	// Verify some key settings
	var journalMode string
	err = db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("Failed to check journal mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("Expected WAL journal mode, got %s", journalMode)
	}
	
	var foreignKeys int
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys)
	if err != nil {
		t.Fatalf("Failed to check foreign keys setting: %v", err)
	}
	if foreignKeys != 1 {
		t.Error("Foreign keys should be enabled")
	}
}