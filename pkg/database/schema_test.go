package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"switchboard/pkg/types"
)

func TestSchemaValidator_ValidateTablesExist(t *testing.T) {
	// Create temporary database
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
	
	validator := NewSchemaValidator(db)
	
	// Should fail on empty database
	err = validator.ValidateTablesExist()
	if err == nil {
		t.Error("ValidateTablesExist should fail on empty database")
	}
	
	// Apply migrations
	migrationSQL := `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_by TEXT NOT NULL,
			student_ids TEXT NOT NULL,
			start_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			end_time DATETIME,
			status TEXT NOT NULL DEFAULT 'active'
		);
		
		CREATE TABLE messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			type TEXT NOT NULL,
			context TEXT NOT NULL DEFAULT 'general',
			from_user TEXT NOT NULL,
			to_user TEXT,
			content TEXT NOT NULL,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
	
	_, err = db.Exec(migrationSQL)
	if err != nil {
		t.Fatalf("Failed to create tables: %v", err)
	}
	
	// Should pass now
	err = validator.ValidateTablesExist()
	if err != nil {
		t.Errorf("ValidateTablesExist should pass with all tables present: %v", err)
	}
}

func TestSchemaValidator_ValidateTableStructure(t *testing.T) {
	// Create temporary database with proper schema
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
	
	// Read and apply the actual migration
	migrationContent, err := os.ReadFile("../../migrations/001_initial_schema.sql")
	if err != nil {
		t.Skipf("Skipping test - migration file not found: %v", err)
		return
	}
	
	_, err = db.Exec(string(migrationContent))
	if err != nil {
		t.Fatalf("Failed to apply migration: %v", err)
	}
	
	validator := NewSchemaValidator(db)
	
	// Validate structure
	err = validator.ValidateTableStructure()
	if err != nil {
		t.Errorf("ValidateTableStructure should pass with correct schema: %v", err)
	}
}

func TestSchemaValidator_ValidateIndexes(t *testing.T) {
	// Create temporary database with proper schema
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
	
	// Read and apply the actual migration
	migrationContent, err := os.ReadFile("../../migrations/001_initial_schema.sql")
	if err != nil {
		t.Skipf("Skipping test - migration file not found: %v", err)
		return
	}
	
	_, err = db.Exec(string(migrationContent))
	if err != nil {
		t.Fatalf("Failed to apply migration: %v", err)
	}
	
	validator := NewSchemaValidator(db)
	
	// Validate indexes
	err = validator.ValidateIndexes()
	if err != nil {
		t.Errorf("ValidateIndexes should pass with all indexes present: %v", err)
	}
}

func TestDatabase_IntegrationWithTypes(t *testing.T) {
	// Test that database schema works with actual types
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
		t.Fatalf("Failed to apply optimizations: %v", err)
	}
	
	// Read and apply the actual migration
	migrationContent, err := os.ReadFile("../../migrations/001_initial_schema.sql")
	if err != nil {
		t.Skipf("Skipping test - migration file not found: %v", err)
		return
	}
	
	_, err = db.Exec(string(migrationContent))
	if err != nil {
		t.Fatalf("Failed to apply migration: %v", err)
	}
	
	// Test inserting and retrieving a session
	session := &types.Session{
		ID:         "test-session-123",
		Name:       "Integration Test Session",
		CreatedBy:  "instructor_test",
		StudentIDs: []string{"student1", "student2", "student3"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	// Insert session
	_, err = db.Exec(`
		INSERT INTO sessions (id, name, created_by, student_ids, start_time, status) 
		VALUES (?, ?, ?, ?, ?, ?)`,
		session.ID, session.Name, session.CreatedBy, 
		`["student1","student2","student3"]`, session.StartTime, session.Status)
	if err != nil {
		t.Fatalf("Failed to insert session: %v", err)
	}
	
	// Test inserting messages of all types
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
			SessionID: session.ID,
			Type:      msgType,
			Context:   "integration",
			FromUser:  "user_test",
			Content:   map[string]interface{}{"text": "Integration test message", "type": msgType},
			Timestamp: time.Now(),
		}
		
		_, err = db.Exec(`
			INSERT INTO messages (id, session_id, type, context, from_user, content, timestamp) 
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			message.ID, message.SessionID, message.Type, message.Context,
			message.FromUser, `{"text":"Integration test message"}`, message.Timestamp)
		if err != nil {
			t.Errorf("Failed to insert message of type %s: %v", msgType, err)
		}
	}
	
	// Test retrieving session history (simulating actual query patterns)
	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM messages WHERE session_id = ? ORDER BY timestamp",
		session.ID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count messages: %v", err)
	}
	if count != len(messageTypes) {
		t.Errorf("Expected %d messages, got %d", len(messageTypes), count)
	}
	
	// Test session lookup by status (simulating active sessions query)
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sessions WHERE status = 'active'",
	).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count active sessions: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 active session, got %d", count)
	}
}