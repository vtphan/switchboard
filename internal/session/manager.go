package session

import (
	"sync"

	"switchboard/internal/database"
	"switchboard/pkg/errors"
)

// SessionManager provides thread-safe session state management
type SessionManager interface {
	GetActiveSession() *database.Session
	SetActiveSession(session *database.Session) error
	ClearActiveSession() error
	HasActiveSession() bool
	CheckAndClearActiveSession() (*database.Session, error)
}

// SessionManagerImpl implements thread-safe session management
type SessionManagerImpl struct {
	activeSessionMu sync.RWMutex
	activeSession   *database.Session
	dbManager       database.DatabaseManager
}

// NewSessionManager creates a new SessionManager instance
func NewSessionManager(dbManager database.DatabaseManager) SessionManager {
	return &SessionManagerImpl{
		dbManager: dbManager,
	}
}

// GetActiveSession returns the currently active session
func (sm *SessionManagerImpl) GetActiveSession() *database.Session {
	sm.activeSessionMu.RLock()
	defer sm.activeSessionMu.RUnlock()
	
	// Return copy of session pointer (may be nil)
	return sm.activeSession
}

// SetActiveSession sets the active session, rejecting if session already active
func (sm *SessionManagerImpl) SetActiveSession(session *database.Session) error {
	sm.activeSessionMu.Lock()
	defer sm.activeSessionMu.Unlock()
	
	if sm.activeSession != nil {
		return errors.ErrSessionAlreadyActive
	}
	
	sm.activeSession = session
	return nil
}

// ClearActiveSession clears the active session, failing if no active session
func (sm *SessionManagerImpl) ClearActiveSession() error {
	sm.activeSessionMu.Lock()
	defer sm.activeSessionMu.Unlock()
	
	if sm.activeSession == nil {
		return errors.ErrNoActiveSession
	}
	
	sm.activeSession = nil
	return nil
}

// HasActiveSession returns true if there is an active session
func (sm *SessionManagerImpl) HasActiveSession() bool {
	sm.activeSessionMu.RLock()
	defer sm.activeSessionMu.RUnlock()
	
	return sm.activeSession != nil
}

// CheckAndClearActiveSession atomically checks for and clears the active session
// Returns the session that was cleared, or nil if no active session
func (sm *SessionManagerImpl) CheckAndClearActiveSession() (*database.Session, error) {
	sm.activeSessionMu.Lock()
	defer sm.activeSessionMu.Unlock()
	
	if sm.activeSession == nil {
		return nil, errors.ErrNoActiveSession
	}
	
	session := sm.activeSession
	sm.activeSession = nil
	return session, nil
}