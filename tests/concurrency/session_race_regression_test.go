// Regression tests for session creation race condition fix
// These tests ensure the database-first approach prevents race conditions
package concurrency

import (
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/pkg/errors"
)

// TestDatabaseFirstSessionCreation validates that database constraint prevents
// multiple active sessions even under extreme concurrency
func TestDatabaseFirstSessionCreation(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	const numGoroutines = 100 // Extreme concurrency
	var successCount int32
	var wg sync.WaitGroup

	// All goroutines start simultaneously
	startSignal := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Wait for start signal to ensure true concurrency
			<-startSignal
			
			sessionName := fmt.Sprintf("Concurrent Session %d", id)
			instructorID := fmt.Sprintf("instructor%d", id)
			
			_, err := env.sessionLifecycle.StartSession(sessionName, instructorID)
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}

	// Start all goroutines simultaneously
	close(startSignal)
	wg.Wait()

	// Exactly one session should succeed
	assert.Equal(t, int32(1), successCount, 
		"Exactly one session creation should succeed under extreme concurrency")

	// Verify database state
	sessions := getActiveSessions(t, env.db)
	assert.Len(t, sessions, 1, "Database should have exactly one active session")
}

// TestSessionCreationWithDatabaseErrors validates proper error handling
// and rollback when database operations fail
func TestSessionCreationWithDatabaseErrors(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	// Create first session successfully
	session1, err := env.sessionLifecycle.StartSession("Session 1", "instructor1")
	require.NoError(t, err)
	require.NotNil(t, session1)

	// Attempt to create second session (should fail due to constraint)
	session2, err := env.sessionLifecycle.StartSession("Session 2", "instructor2")
	assert.Error(t, err)
	assert.Nil(t, session2)
	assert.Equal(t, errors.ErrSessionAlreadyActive, err)

	// Verify in-memory state is consistent
	activeSession := env.sessionManager.GetActiveSession()
	assert.NotNil(t, activeSession)
	assert.Equal(t, session1.ID, activeSession.ID)
}

// TestSessionEndingConcurrency validates that concurrent session endings
// are handled properly with database-first approach
func TestSessionEndingConcurrency(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	// Create a session first
	_, err := env.sessionLifecycle.StartSession("Test Session", "instructor1")
	require.NoError(t, err)

	const numGoroutines = 50
	var successCount int32
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			instructorID := fmt.Sprintf("instructor%d", id)
			_, err := env.sessionLifecycle.EndSession(instructorID)
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	// Exactly one end operation should succeed
	assert.Equal(t, int32(1), successCount, 
		"Exactly one session end should succeed under concurrency")

	// Verify no active sessions remain
	activeSession := env.sessionManager.GetActiveSession()
	assert.Nil(t, activeSession)

	sessions := getActiveSessions(t, env.db)
	assert.Len(t, sessions, 0, "No active sessions should remain in database")
}

// TestRapidSessionCycling validates that rapid create/end cycles maintain
// consistency and don't cause race conditions
func TestRapidSessionCycling(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	const numCycles = 20
	const concurrentOps = 10

	for cycle := 0; cycle < numCycles; cycle++ {
		var wg sync.WaitGroup
		var createSuccess int32
		var endSuccess int32

		// Concurrent session creation attempts
		for i := 0; i < concurrentOps; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				sessionName := fmt.Sprintf("Cycle %d Session %d", cycle, id)
				instructorID := fmt.Sprintf("instructor%d", id)
				_, err := env.sessionLifecycle.StartSession(sessionName, instructorID)
				if err == nil {
					atomic.AddInt32(&createSuccess, 1)
				}
			}(i)
		}
		wg.Wait()

		// Verify exactly one creation succeeded
		assert.Equal(t, int32(1), createSuccess, 
			"Cycle %d: Exactly one session creation should succeed", cycle)

		// Concurrent session ending attempts
		for i := 0; i < concurrentOps; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				instructorID := fmt.Sprintf("instructor%d", id)
				_, err := env.sessionLifecycle.EndSession(instructorID)
				if err == nil {
					atomic.AddInt32(&endSuccess, 1)
				}
			}(i)
		}
		wg.Wait()

		// Verify exactly one end succeeded
		assert.Equal(t, int32(1), endSuccess, 
			"Cycle %d: Exactly one session end should succeed", cycle)

		// Verify clean state after each cycle
		activeSession := env.sessionManager.GetActiveSession()
		assert.Nil(t, activeSession, "Cycle %d: No active session should remain", cycle)
	}

	// Final verification
	sessions := getActiveSessions(t, env.db)
	assert.Len(t, sessions, 0, "No active sessions should remain after all cycles")
}

// TestDatabaseConstraintEnforcement directly tests that the database
// constraint prevents multiple active sessions
func TestDatabaseConstraintEnforcement(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	// Directly insert an active session
	_, err := env.db.Exec(`
		INSERT INTO sessions (id, name, created_by, start_time, status)
		VALUES (?, ?, ?, ?, ?)
	`, "test-session-1", "Test Session 1", "instructor1", time.Now(), "active")
	require.NoError(t, err)

	// Attempt to insert another active session (should fail)
	_, err = env.db.Exec(`
		INSERT INTO sessions (id, name, created_by, start_time, status)
		VALUES (?, ?, ?, ?, ?)
	`, "test-session-2", "Test Session 2", "instructor2", time.Now(), "active")
	
	// Should fail due to unique constraint
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UNIQUE", 
		"Database should enforce unique active session constraint")

	// Verify only one active session exists
	sessions := getActiveSessions(t, env.db)
	assert.Len(t, sessions, 1, "Only one active session should exist")
}

// TestMemoryDatabaseSyncRecovery tests that the system can recover
// from in-memory and database state mismatches
func TestMemoryDatabaseSyncRecovery(t *testing.T) {
	env := setupTestEnvironment(t)
	defer env.cleanup()

	// Create a session normally
	session1, err := env.sessionLifecycle.StartSession("Session 1", "instructor1")
	require.NoError(t, err)

	// Manually clear in-memory state (simulating a bug or crash recovery)
	err = env.sessionManager.ClearActiveSession()
	require.NoError(t, err)

	// Attempt to create another session
	// This should fail because database still has active session
	session2, err := env.sessionLifecycle.StartSession("Session 2", "instructor2")
	assert.Error(t, err)
	assert.Nil(t, session2)
	assert.Equal(t, errors.ErrSessionAlreadyActive, err)

	// Verify database state is still correct
	sessions := getActiveSessions(t, env.db)
	assert.Len(t, sessions, 1)
	assert.Equal(t, session1.ID, sessions[0].ID)
}

// Helper function to get active sessions directly from database
func getActiveSessions(t *testing.T, db *sql.DB) []*database.Session {
	rows, err := db.Query(`
		SELECT id, name, created_by, start_time, status 
		FROM sessions 
		WHERE status = 'active'
	`)
	require.NoError(t, err)
	defer func() {
		if err := rows.Close(); err != nil {
			t.Logf("Error closing rows: %v", err)
		}
	}()

	var sessions []*database.Session
	for rows.Next() {
		var s database.Session
		err := rows.Scan(&s.ID, &s.Name, &s.CreatedBy, &s.StartTime, &s.Status)
		require.NoError(t, err)
		sessions = append(sessions, &s)
	}
	return sessions
}