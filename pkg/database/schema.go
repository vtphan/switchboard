package database

import (
	"database/sql"
	"fmt"
)

// SchemaValidator provides database schema validation functionality
// ARCHITECTURAL DISCOVERY: Separate validation component enables testing
// and deployment verification without coupling to migration system
type SchemaValidator struct {
	db *sql.DB
}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator(db *sql.DB) *SchemaValidator {
	return &SchemaValidator{db: db}
}

// ValidateTablesExist verifies that all required tables exist
// FUNCTIONAL DISCOVERY: Explicit table validation prevents runtime errors
// from missing tables during database operations
func (v *SchemaValidator) ValidateTablesExist() error {
	requiredTables := map[string]string{
		"sessions": "Session data storage",
		"messages": "Message data storage",
		"schema_migrations": "Migration tracking",
	}

	for table, description := range requiredTables {
		exists, err := v.tableExists(table)
		if err != nil {
			return fmt.Errorf("error checking table %s (%s): %w", table, description, err)
		}
		if !exists {
			return fmt.Errorf("required table %s (%s) does not exist", table, description)
		}
	}

	return nil
}

// ValidateTableStructure verifies table column structure matches expectations
// TECHNICAL DISCOVERY: Column validation ensures type compatibility between
// Go structs and database schema
func (v *SchemaValidator) ValidateTableStructure() error {
	// Validate sessions table structure
	sessionColumns := map[string]string{
		"id":          "TEXT",
		"name":        "TEXT",
		"created_by":  "TEXT",
		"student_ids": "TEXT",
		"start_time":  "DATETIME",
		"end_time":    "DATETIME",
		"status":      "TEXT",
	}

	err := v.validateColumns("sessions", sessionColumns)
	if err != nil {
		return fmt.Errorf("sessions table structure invalid: %w", err)
	}

	// Validate messages table structure
	messageColumns := map[string]string{
		"id":         "TEXT",
		"session_id": "TEXT", 
		"type":       "TEXT",
		"context":    "TEXT",
		"from_user":  "TEXT",
		"to_user":    "TEXT",
		"content":    "TEXT",
		"timestamp":  "DATETIME",
	}

	err = v.validateColumns("messages", messageColumns)
	if err != nil {
		return fmt.Errorf("messages table structure invalid: %w", err)
	}

	return nil
}

// ValidateIndexes verifies that all performance indexes exist
// FUNCTIONAL DISCOVERY: Index validation ensures query performance expectations
// are met in production deployments
func (v *SchemaValidator) ValidateIndexes() error {
	requiredIndexes := map[string]string{
		"idx_sessions_status":        "Session status lookups",
		"idx_sessions_created_by":    "Session ownership queries",
		"idx_messages_session_time":  "Message history retrieval",
		"idx_messages_session_type":  "Message type filtering", 
		"idx_messages_to_user":       "Direct message queries",
	}

	for index, purpose := range requiredIndexes {
		exists, err := v.indexExists(index)
		if err != nil {
			return fmt.Errorf("error checking index %s (%s): %w", index, purpose, err)
		}
		if !exists {
			return fmt.Errorf("required index %s (%s) does not exist", index, purpose)
		}
	}

	return nil
}

// ValidateConstraints verifies that database constraints are properly enforced
// ARCHITECTURAL DISCOVERY: Constraint validation ensures data integrity rules
// are enforced at the database level
func (v *SchemaValidator) ValidateConstraints() error {
	// Test foreign key constraint (messages.session_id -> sessions.id)
	_, err := v.db.Exec(`
		INSERT INTO messages (id, session_id, type, context, from_user, content) 
		VALUES ('test', 'nonexistent', 'instructor_inbox', 'test', 'user1', '{}')
	`)
	if err == nil {
		// Clean up the test record if it somehow got inserted
		if _, err := v.db.Exec("DELETE FROM messages WHERE id = 'test'"); err != nil {
				// Ignore cleanup errors - test constraint validation is primary concern
			_ = err
		}
		return fmt.Errorf("foreign key constraint not enforced: messages.session_id")
	}

	// Test check constraint for message types
	_, err = v.db.Exec(`
		INSERT INTO sessions (id, name, created_by, student_ids) 
		VALUES ('test-session', 'Test', 'instructor1', '[]')
	`)
	if err != nil {
		return fmt.Errorf("failed to create test session: %w", err)
	}

	_, err = v.db.Exec(`
		INSERT INTO messages (id, session_id, type, context, from_user, content) 
		VALUES ('test', 'test-session', 'invalid_type', 'test', 'user1', '{}')
	`)
	if err == nil {
		// Clean up test records
		if _, err := v.db.Exec("DELETE FROM messages WHERE id = 'test'"); err != nil {
				// Ignore cleanup errors - test constraint validation is primary concern
			_ = err
		}
		if _, err := v.db.Exec("DELETE FROM sessions WHERE id = 'test-session'"); err != nil {
				// Ignore cleanup errors - test constraint validation is primary concern
			_ = err
		}
		return fmt.Errorf("check constraint not enforced: message type validation")
	}

	// Clean up test session
	if _, err := v.db.Exec("DELETE FROM sessions WHERE id = 'test-session'"); err != nil {
			// Ignore cleanup errors - test constraint validation is primary concern
			_ = err
	}

	return nil
}

// tableExists checks if a table exists in the database
func (v *SchemaValidator) tableExists(tableName string) (bool, error) {
	var count int
	err := v.db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
		tableName,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// indexExists checks if an index exists in the database
func (v *SchemaValidator) indexExists(indexName string) (bool, error) {
	var count int
	err := v.db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?",
		indexName,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// validateColumns checks that a table has the expected columns with correct types
func (v *SchemaValidator) validateColumns(tableName string, expectedColumns map[string]string) error {
	rows, err := v.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Ignore cleanup errors to avoid masking the primary error
			_ = err
		}
	}()

	foundColumns := make(map[string]string)
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue interface{}
		var pk int

		err = rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
		if err != nil {
			return err
		}

		foundColumns[name] = dataType
	}

	// Check that all expected columns exist with correct types
	for expectedCol, expectedType := range expectedColumns {
		foundType, exists := foundColumns[expectedCol]
		if !exists {
			return fmt.Errorf("column %s not found", expectedCol)
		}
		if foundType != expectedType {
			return fmt.Errorf("column %s has type %s, expected %s", expectedCol, foundType, expectedType)
		}
	}

	return rows.Err()
}