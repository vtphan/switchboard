package session

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
	
	"github.com/google/uuid"
	"switchboard/pkg/interfaces"
	"switchboard/pkg/types"
)

// Manager implements the SessionManager interface
type Manager struct {
	dbManager     interfaces.DatabaseManager
	activeSessions map[string]*types.Session // sessionID -> Session
	mu            sync.RWMutex
}

// NewManager creates a new session manager
func NewManager(dbManager interfaces.DatabaseManager) *Manager {
	return &Manager{
		dbManager:      dbManager,
		activeSessions: make(map[string]*types.Session),
	}
}

// LoadActiveSessions loads all active sessions from database into memory
func (m *Manager) LoadActiveSessions(ctx context.Context) error {
	sessions, err := m.dbManager.ListActiveSessions(ctx)
	if err != nil {
		return fmt.Errorf("failed to load active sessions: %w", err)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, session := range sessions {
		m.activeSessions[session.ID] = session
	}
	
	log.Printf("Loaded %d active sessions", len(sessions))
	return nil
}

// CreateSession creates a new session
func (m *Manager) CreateSession(ctx context.Context, name string, createdBy string, studentIDs []string) (*types.Session, error) {
	// Validate input parameters
	if name == "" || len(name) > 200 {
		return nil, ErrInvalidSessionName
	}
	
	if !types.IsValidUserID(createdBy) {
		return nil, ErrInvalidCreatedBy
	}
	
	if len(studentIDs) == 0 {
		return nil, ErrEmptyStudentList
	}
	
	// Remove duplicate student IDs
	uniqueStudents := removeDuplicates(studentIDs)
	
	// Validate all student IDs
	for _, studentID := range uniqueStudents {
		if !types.IsValidUserID(studentID) {
			return nil, fmt.Errorf("%w: invalid student ID %s", ErrInvalidStudentID, studentID)
		}
	}
	
	// Create session object
	session := &types.Session{
		ID:         uuid.New().String(),
		Name:       name,
		CreatedBy:  createdBy,
		StudentIDs: uniqueStudents,
		StartTime:  time.Now(),
		EndTime:    nil,
		Status:     "active",
	}
	
	// Persist to database
	if err := m.dbManager.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	
	// Add to in-memory cache
	m.mu.Lock()
	m.activeSessions[session.ID] = session
	m.mu.Unlock()
	
	log.Printf("Created session: id=%s name=%s students=%d", session.ID, session.Name, len(session.StudentIDs))
	return session, nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	// Check in-memory cache first
	m.mu.RLock()
	if session, exists := m.activeSessions[sessionID]; exists {
		m.mu.RUnlock()
		return session, nil
	}
	m.mu.RUnlock()
	
	// Query database for ended sessions or cache misses
	session, err := m.dbManager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	
	return session, nil
}

// EndSession ends an active session
func (m *Manager) EndSession(ctx context.Context, sessionID string) error {
	// Get session from cache
	m.mu.RLock()
	session, exists := m.activeSessions[sessionID]
	m.mu.RUnlock()
	
	if !exists {
		// Check if session exists in database but not active
		dbSession, err := m.dbManager.GetSession(ctx, sessionID)
		if err != nil {
			return ErrSessionNotFound
		}
		if dbSession.Status == "ended" {
			return ErrSessionAlreadyEnded
		}
		session = dbSession
	}
	
	// Update session status
	now := time.Now()
	session.EndTime = &now
	session.Status = "ended"
	
	// Persist to database
	if err := m.dbManager.UpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to end session: %w", err)
	}
	
	// Remove from active sessions cache
	m.mu.Lock()
	delete(m.activeSessions, sessionID)
	m.mu.Unlock()
	
	log.Printf("Ended session: id=%s name=%s", session.ID, session.Name)
	return nil
}

// ListActiveSessions returns all active sessions
func (m *Manager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	m.mu.RLock()
	sessions := make([]*types.Session, 0, len(m.activeSessions))
	for _, session := range m.activeSessions {
		sessions = append(sessions, session)
	}
	m.mu.RUnlock()
	
	return sessions, nil
}

// ValidateSessionMembership checks if user can join session
func (m *Manager) ValidateSessionMembership(sessionID, userID, role string) error {
	// Get session (check cache first)
	m.mu.RLock()
	session, exists := m.activeSessions[sessionID]
	m.mu.RUnlock()
	
	if !exists {
		// Check database for ended sessions
		dbSession, err := m.dbManager.GetSession(context.Background(), sessionID)
		if err != nil {
			return ErrSessionNotFound
		}
		if dbSession.Status == "ended" {
			return ErrSessionEnded
		}
		session = dbSession
	}
	
	// Check session is active
	if session.Status != "active" {
		return ErrSessionEnded
	}
	
	// Validate role-based access
	switch role {
	case "instructor":
		// Instructors have universal access to all active sessions
		return nil
		
	case "student":
		// Students must be in session's student_ids list
		for _, studentID := range session.StudentIDs {
			if studentID == userID {
				return nil
			}
		}
		return ErrUnauthorized
		
	default:
		return ErrInvalidRole
	}
}

// GetStats returns session manager statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return map[string]interface{}{
		"active_sessions": len(m.activeSessions),
		"cache_size":     len(m.activeSessions),
	}
}

// RefreshCache reloads active sessions from database
func (m *Manager) RefreshCache(ctx context.Context) error {
	sessions, err := m.dbManager.ListActiveSessions(ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh session cache: %w", err)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Clear current cache
	m.activeSessions = make(map[string]*types.Session)
	
	// Reload from database
	for _, session := range sessions {
		m.activeSessions[session.ID] = session
	}
	
	log.Printf("Refreshed session cache: %d active sessions", len(sessions))
	return nil
}

// IsSessionActive checks if a session is active (cache-only check)
func (m *Manager) IsSessionActive(sessionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	session, exists := m.activeSessions[sessionID]
	return exists && session.Status == "active"
}

// Helper function to remove duplicate student IDs
func removeDuplicates(studentIDs []string) []string {
	seen := make(map[string]bool)
	unique := make([]string, 0, len(studentIDs))
	
	for _, id := range studentIDs {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	
	return unique
}