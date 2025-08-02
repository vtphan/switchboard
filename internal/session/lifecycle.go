package session

import (
	"fmt"
	"log"
	"time"

	"switchboard/internal/database"
	"switchboard/pkg/errors"
)

// SessionLifecycleInterface defines the interface for session lifecycle operations
type SessionLifecycleInterface interface {
	StartSession(sessionName, instructorID string) (*database.Session, error)
	EndSession(instructorID string) (*database.Session, error)
}

// SystemBroadcaster defines a simplified interface for broadcasting system messages
// This avoids import cycles by having a focused interface just for session events
type SystemBroadcaster interface {
	BroadcastSessionStarted(sessionID, sessionName, startedBy string, startTime time.Time) error
	BroadcastSessionEnded(sessionID, endedBy string, endTime time.Time) error
}

// SessionLifecycle handles session start/end operations following the exact
// implementation pattern specified in tech specs lines 349-396.
type SessionLifecycle struct {
	sessionManager    SessionManager
	dbManager         database.DatabaseManager
	systemBroadcaster SystemBroadcaster
}

// NewSessionLifecycle creates a new SessionLifecycle instance with required dependencies
func NewSessionLifecycle(sessionManager SessionManager, dbManager database.DatabaseManager, systemBroadcaster SystemBroadcaster) *SessionLifecycle {
	return &SessionLifecycle{
		sessionManager:    sessionManager,
		dbManager:         dbManager,
		systemBroadcaster: systemBroadcaster,
	}
}

// StartSession creates and activates a new session using database-first approach
// to ensure atomicity and prevent race conditions.
func (sl *SessionLifecycle) StartSession(sessionName, instructorID string) (*database.Session, error) {
	// Step 1: Validate input parameters
	if len(sessionName) < 1 || len(sessionName) > 200 {
		return nil, fmt.Errorf("session name must be 1-200 characters")
	}
	if instructorID == "" {
		return nil, fmt.Errorf("instructor ID required")
	}
	
	// Step 2: Check database manager availability
	if sl.dbManager == nil {
		return nil, fmt.Errorf("database error: database manager is nil")
	}
	
	// Step 3: Create new session object
	session := &database.Session{
		ID:        generateSessionID(), // Use crypto/rand for uniqueness
		Name:      sessionName,
		CreatedBy: instructorID,
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}
	
	// Step 4: Database-first atomic creation
	// The database constraint will prevent multiple active sessions
	err := sl.dbManager.CreateSession(session)
	if err != nil {
		// Check if it's due to an active session already existing
		if activeSession, getErr := sl.dbManager.GetActiveSession(); getErr == nil && activeSession != nil {
			return nil, errors.ErrSessionAlreadyActive
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	
	// Step 5: Update in-memory state only after successful database write
	// This is safe because the database is the source of truth
	err = sl.sessionManager.SetActiveSession(session)
	if err != nil {
		// This should not happen since we just created the session in the database
		// Log the error but don't fail - database is source of truth
		fmt.Printf("Warning: in-memory state out of sync with database: %v\n", err)
		// Force sync by clearing and retrying
		_ = sl.sessionManager.ClearActiveSession()
		err = sl.sessionManager.SetActiveSession(session)
		if err != nil {
			// Critical error - database and memory are out of sync
			return nil, fmt.Errorf("critical: failed to sync in-memory state: %w", err)
		}
	}
	
	// Step 6: Broadcast session_started system message to all connected users
	if sl.systemBroadcaster != nil {
		if err := sl.systemBroadcaster.BroadcastSessionStarted(session.ID, session.Name, session.CreatedBy, session.StartTime); err != nil {
			// Log broadcast failure but don't fail the session creation
			log.Printf("Warning: Failed to broadcast session_started message: %v", err)
		}
	}
	
	// Step 7: Return session details
	return session, nil
}

// EndSession ends the current active session following database-first approach
// to maintain consistency.
func (sl *SessionLifecycle) EndSession(instructorID string) (*database.Session, error) {
	// Step 1: Atomically check and clear in-memory session
	session, err := sl.sessionManager.CheckAndClearActiveSession()
	if err != nil {
		return nil, err  // Only one goroutine will succeed here
	}
	
	// Step 2: Create ended session copy
	endTime := time.Now()
	sessionCopy := *session
	sessionCopy.Status = database.SessionStatusEnded
	sessionCopy.EndTime = &endTime
	
	// Step 3: Persist to database (with proper rollback on failure)
	err = sl.dbManager.UpdateSession(&sessionCopy)
	if err != nil {
		// Rollback: restore the session in memory since database update failed
		restoreErr := sl.sessionManager.SetActiveSession(session)
		if restoreErr != nil {
			log.Printf("Critical error: failed to restore session after database failure: %v", restoreErr)
		}
		return nil, fmt.Errorf("failed to end session in database: %w", err)
	}
	
	// Step 4: Broadcast session_ended system message to all connected users
	if sl.systemBroadcaster != nil {
		if err := sl.systemBroadcaster.BroadcastSessionEnded(sessionCopy.ID, sessionCopy.CreatedBy, *sessionCopy.EndTime); err != nil {
			// Log broadcast failure but don't fail the session ending
			log.Printf("Warning: Failed to broadcast session_ended message: %v", err)
		}
	}
	
	// Step 5: Return ended session
	return &sessionCopy, nil
}