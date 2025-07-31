package integration

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"

	"switchboard/internal/database"
	"switchboard/internal/session"
)

// TestStep22_SessionLifecycleDatabaseIntegration verifies integration between
// SessionLifecycle, SessionManager and DatabaseManager
func TestStep22_SessionLifecycleDatabaseIntegration(t *testing.T) {
	// This test should FAIL until Step 2.2 implementation exists
	
	// Setup in-memory database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Apply database schema
	schema := `
	CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_by TEXT NOT NULL, 
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		status TEXT NOT NULL DEFAULT 'active',
		CONSTRAINT sessions_name_length CHECK (length(name) >= 1 AND length(name) <= 200),
		CONSTRAINT sessions_status_valid CHECK (status IN ('active', 'ended'))
	);

	CREATE TABLE messages (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		type TEXT NOT NULL,
		context TEXT NOT NULL,
		from_user TEXT NOT NULL,
		to_user TEXT,
		content TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		CONSTRAINT messages_session_id_fkey FOREIGN KEY (session_id) REFERENCES sessions (id),
		CONSTRAINT messages_type_valid CHECK (type IN ('broadcast_to_instructors', 'direct_message', 'broadcast_to_students', 'system')),
		CONSTRAINT messages_context_valid CHECK (context IN ('question', 'submission', 'analytics', 'response', 'request', 'peer_help', 'announcement', 'instruction', 'emergency', 'general'))
	);`
	
	_, err = db.Exec(schema)
	require.NoError(t, err)

	// Setup components
	dbManager, err := database.NewSQLiteDatabaseManager(db)
	require.NoError(t, err)
	err = dbManager.Start()
	require.NoError(t, err)
	defer func() { _ = dbManager.Stop() }()

	sessionManager := session.NewSessionManager(dbManager)
	lifecycle := session.NewSessionLifecycle(sessionManager, dbManager)

	// Test: Start session
	sessionName := "Integration Test Session"
	instructorID := "instructor123"
	
	startedSession, err := lifecycle.StartSession(sessionName, instructorID)
	require.NoError(t, err)
	require.NotNil(t, startedSession)
	
	// Verify session properties
	assert.Equal(t, sessionName, startedSession.Name)
	assert.Equal(t, instructorID, startedSession.CreatedBy)
	assert.Equal(t, database.SessionStatusActive, startedSession.Status)
	assert.NotEmpty(t, startedSession.ID)
	assert.False(t, startedSession.StartTime.IsZero())
	assert.Nil(t, startedSession.EndTime)

	// Verify session is active in SessionManager
	activeSession := sessionManager.GetActiveSession()
	require.NotNil(t, activeSession)
	assert.Equal(t, startedSession.ID, activeSession.ID)
	assert.True(t, sessionManager.HasActiveSession())

	// Verify session persisted to database
	retrievedSession, err := dbManager.GetActiveSession()
	require.NoError(t, err)
	require.NotNil(t, retrievedSession)
	assert.Equal(t, startedSession.ID, retrievedSession.ID)
	assert.Equal(t, sessionName, retrievedSession.Name)

	// Test: Cannot start another session when one is active
	_, err = lifecycle.StartSession("Another Session", "instructor456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session already active")

	// Test: End session
	endedSession, err := lifecycle.EndSession(instructorID)
	require.NoError(t, err)
	require.NotNil(t, endedSession)
	
	// Verify session ended properly
	assert.Equal(t, startedSession.ID, endedSession.ID)
	assert.Equal(t, database.SessionStatusEnded, endedSession.Status)
	assert.NotNil(t, endedSession.EndTime)
	assert.True(t, endedSession.EndTime.After(endedSession.StartTime))

	// Verify session cleared from SessionManager
	activeSession = sessionManager.GetActiveSession()
	assert.Nil(t, activeSession)
	assert.False(t, sessionManager.HasActiveSession())

	// Verify session update persisted to database
	retrievedSession, err = dbManager.GetActiveSession()
	assert.NoError(t, err)
	assert.Nil(t, retrievedSession) // Should be nil since session ended

	// Test: Cannot end session when none active
	_, err = lifecycle.EndSession(instructorID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

// TestStep22_DatabaseRollbackIntegration verifies rollback behavior on database failures
func TestStep22_DatabaseRollbackIntegration(t *testing.T) {
	// This test should FAIL until Step 2.2 implementation exists
	
	// Setup components with a failing database
	sessionManager := session.NewSessionManager(nil) // Nil database will cause failures
	lifecycle := session.NewSessionLifecycle(sessionManager, nil)

	// Test: StartSession should rollback SessionManager state on database failure
	_, err := lifecycle.StartSession("Test Session", "instructor123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")

	// Verify SessionManager state was rolled back
	activeSession := sessionManager.GetActiveSession()
	assert.Nil(t, activeSession, "SessionManager should have no active session after rollback")
	assert.False(t, sessionManager.HasActiveSession())
}

// TestStep22_SessionValidationIntegration verifies input validation integration
func TestStep22_SessionValidationIntegration(t *testing.T) {
	// This test should FAIL until Step 2.2 implementation exists
	
	// Setup minimal components for validation testing
	sessionManager := session.NewSessionManager(nil)
	lifecycle := session.NewSessionLifecycle(sessionManager, nil)

	// Test: Session name validation
	testCases := []struct {
		name         string
		sessionName  string
		instructorID string
		expectError  bool
		errorText    string
	}{
		{"Empty session name", "", "instructor123", true, "session name must be 1-200 characters"},
		{"Session name too long", string(make([]byte, 201)), "instructor123", true, "session name must be 1-200 characters"},
		{"Empty instructor ID", "Valid Session", "", true, "instructor ID required"},
		{"Valid inputs", "Valid Session", "instructor123", false, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			session, err := lifecycle.StartSession(tc.sessionName, tc.instructorID)
			
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, session)
				if tc.errorText != "" {
					assert.Contains(t, err.Error(), tc.errorText)
				}
			} else {
				// For valid inputs, we expect database error since dbManager is nil
				// but the validation should pass
				if err != nil {
					assert.Contains(t, err.Error(), "database error")
				}
			}
		})
	}
}

// TestStep22_ConcurrentLifecycleIntegration verifies thread safety of lifecycle operations
func TestStep22_ConcurrentLifecycleIntegration(t *testing.T) {
	// This test should FAIL until Step 2.2 implementation exists
	
	// Setup in-memory database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Apply schema
	schema := `
	CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_by TEXT NOT NULL, 
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		status TEXT NOT NULL DEFAULT 'active'
	);`
	
	_, err = db.Exec(schema)
	require.NoError(t, err)

	// Setup components
	dbManager, err := database.NewSQLiteDatabaseManager(db)
	require.NoError(t, err)
	err = dbManager.Start()
	require.NoError(t, err)
	defer func() { _ = dbManager.Stop() }()

	sessionManager := session.NewSessionManager(dbManager)
	lifecycle := session.NewSessionLifecycle(sessionManager, dbManager)

	// Test: Multiple concurrent StartSession calls - only one should succeed
	const numConcurrent = 10
	results := make(chan error, numConcurrent)
	
	for i := 0; i < numConcurrent; i++ {
		go func(id int) {
			_, err := lifecycle.StartSession("Concurrent Session", "instructor123")
			results <- err
		}(i)
	}

	// Collect results
	successCount := 0
	errorCount := 0
	for i := 0; i < numConcurrent; i++ {
		err := <-results
		if err == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	// Exactly one should succeed, others should fail with "session already active"
	assert.Equal(t, 1, successCount, "Exactly one concurrent StartSession should succeed")
	assert.Equal(t, numConcurrent-1, errorCount, "Other StartSession calls should fail")

	// Verify session is active
	assert.True(t, sessionManager.HasActiveSession())

	// Clean up by ending session
	_, err = lifecycle.EndSession("instructor123")
	assert.NoError(t, err)
}

// TestStep22_CrossPhaseContractValidation verifies contracts for future phases
func TestStep22_CrossPhaseContractValidation(t *testing.T) {
	// This test should FAIL until Step 2.2 implementation exists
	
	// Setup minimal components
	sessionManager := session.NewSessionManager(nil)
	lifecycle := session.NewSessionLifecycle(sessionManager, nil)

	// Contract validation for Phase 5 HTTP API integration
	// Verify SessionLifecycle provides the methods HTTP handlers need
	lifecycleType := reflect.TypeOf(lifecycle)
	
	// StartSession method should exist for POST /api/session/start
	startMethod, startExists := lifecycleType.MethodByName("StartSession")
	assert.True(t, startExists, "StartSession method required for HTTP API integration")
	
	// EndSession method should exist for POST /api/session/end  
	endMethod, endExists := lifecycleType.MethodByName("EndSession")
	assert.True(t, endExists, "EndSession method required for HTTP API integration")

	// Methods should return proper types for JSON serialization
	// StartSession should return (*Session, error)
	if startExists {
		assert.Equal(t, 2, startMethod.Type.NumOut())
		assert.True(t, startMethod.Type.Out(0).Kind() == reflect.Ptr) // *Session
		assert.Equal(t, "error", startMethod.Type.Out(1).Name())
	}

	// EndSession should return (*Session, error)
	if endExists {
		assert.Equal(t, 2, endMethod.Type.NumOut())
		assert.True(t, endMethod.Type.Out(0).Kind() == reflect.Ptr) // *Session
		assert.Equal(t, "error", endMethod.Type.Out(1).Name())
	}
}