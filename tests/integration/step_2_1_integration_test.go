package integration

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"

	"switchboard/internal/database"
	"switchboard/internal/session"
)

// TestStep21_SessionDatabaseIntegration verifies integration between
// SessionManager and DatabaseManager from Phase 1
func TestStep21_SessionDatabaseIntegration(t *testing.T) {
	// This test should FAIL until Step 2.1 implementation exists
	
	// Setup in-memory database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Apply database schema from Phase 1
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

	// Setup DatabaseManager from Phase 1
	dbManager, err := database.NewSQLiteDatabaseManager(db)
	require.NoError(t, err)
	err = dbManager.Start()
	require.NoError(t, err)
	defer func() { _ = dbManager.Stop() }()

	// Setup SessionManager from Step 2.1 (should fail until implemented)
	sessionManager := session.NewSessionManager(dbManager)

	// Test: Initially no active session
	activeSession := sessionManager.GetActiveSession()
	assert.Nil(t, activeSession)
	assert.False(t, sessionManager.HasActiveSession())

	// Test: Set active session
	testSession := &database.Session{
		ID:        "integration-test-session",
		Name:      "Integration Test Session",
		CreatedBy: "instructor123",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}

	err = sessionManager.SetActiveSession(testSession)
	require.NoError(t, err)

	// Verify session is active in manager
	activeSession = sessionManager.GetActiveSession()
	require.NotNil(t, activeSession)
	assert.Equal(t, testSession.ID, activeSession.ID)
	assert.Equal(t, testSession.Name, activeSession.Name)
	assert.True(t, sessionManager.HasActiveSession())

	// Test: Cannot set another session when one is active
	anotherSession := &database.Session{
		ID:        "another-session",
		Name:      "Another Session", 
		CreatedBy: "instructor456",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}

	err = sessionManager.SetActiveSession(anotherSession)
	assert.Error(t, err) // Should fail with ErrSessionAlreadyActive

	// Test: Clear active session
	err = sessionManager.ClearActiveSession()
	require.NoError(t, err)

	// Verify session is cleared
	activeSession = sessionManager.GetActiveSession()
	assert.Nil(t, activeSession)
	assert.False(t, sessionManager.HasActiveSession())

	// Test: Cannot clear when no session is active
	err = sessionManager.ClearActiveSession()
	assert.Error(t, err) // Should fail with ErrNoActiveSession
}

// TestStep21_CrossPhaseContractValidation verifies that SessionManager
// provides the contracts required by future phases
func TestStep21_CrossPhaseContractValidation(t *testing.T) {
	// This test should FAIL until Step 2.1 implementation exists
	
	// Setup minimal database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	dbManager, err := database.NewSQLiteDatabaseManager(db)
	require.NoError(t, err)
	sessionManager := session.NewSessionManager(dbManager)

	// Contract 1: MessageProcessor will call GetActiveSession() before processing messages
	// Verify method exists and returns correct type
	activeSession := sessionManager.GetActiveSession()
	// Should be nil initially, but method must exist and return *Session
	assert.Nil(t, activeSession)

	// Contract 2: HTTP API will call SetActiveSession()/ClearActiveSession() for session lifecycle
	testSession := &database.Session{
		ID:        "contract-test-session",
		Name:      "Contract Test",
		CreatedBy: "instructor789",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}

	// API must be able to set session
	err = sessionManager.SetActiveSession(testSession)
	assert.NoError(t, err)

	// API must be able to clear session
	err = sessionManager.ClearActiveSession()
	assert.NoError(t, err)

	// Contract 3: WebSocket connections will call GetActiveSession() on connection establishment
	// Plus HasActiveSession() for quick status checks
	hasSession := sessionManager.HasActiveSession()
	assert.False(t, hasSession) // Should be false after clearing

	// Set session again
	err = sessionManager.SetActiveSession(testSession)
	require.NoError(t, err)

	hasSession = sessionManager.HasActiveSession()
	assert.True(t, hasSession) // Should be true after setting

	// GetActiveSession should return the set session
	retrievedSession := sessionManager.GetActiveSession()
	require.NotNil(t, retrievedSession)
	assert.Equal(t, testSession.ID, retrievedSession.ID)
}

// TestStep21_ThreadSafetyIntegration verifies that SessionManager is thread-safe
// as required for integration with concurrent WebSocket connections
func TestStep21_ThreadSafetyIntegration(t *testing.T) {
	// This test should FAIL until Step 2.1 implementation exists
	
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	dbManager, err := database.NewSQLiteDatabaseManager(db)
	require.NoError(t, err)
	sessionManager := session.NewSessionManager(dbManager)

	testSession := &database.Session{
		ID:        "thread-safety-test",
		Name:      "Thread Safety Test",
		CreatedBy: "instructor999",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}

	// Set session
	err = sessionManager.SetActiveSession(testSession)
	require.NoError(t, err)

	// Simulate multiple WebSocket connections reading session state concurrently
	const numConnections = 20
	const readsPerConnection = 50

	done := make(chan bool, numConnections)
	for i := 0; i < numConnections; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < readsPerConnection; j++ {
				// Operations that WebSocket connections will perform
				session := sessionManager.GetActiveSession()
				if session == nil {
					t.Error("Session should not be nil during concurrent reads")
					return
				}

				hasSession := sessionManager.HasActiveSession()
				if !hasSession {
					t.Error("HasActiveSession should return true during concurrent reads")
					return
				}

				if session.ID != testSession.ID {
					t.Error("Session ID mismatch during concurrent reads")
					return
				}
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numConnections; i++ {
		<-done
	}

	// Verify session is still accessible after concurrent access
	finalSession := sessionManager.GetActiveSession()
	assert.NotNil(t, finalSession)
	assert.Equal(t, testSession.ID, finalSession.ID)
}