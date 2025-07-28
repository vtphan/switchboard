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

// Standard timeout values for different database operations
const (
	sessionLoadTimeout   = 10 * time.Second // For bulk loading operations
	sessionWriteTimeout  = 5 * time.Second  // For individual write operations
	historyQueryTimeout  = 10 * time.Second // For history queries
)

// Manager implements the SessionManager interface
type Manager struct {
	dbManager     interfaces.DatabaseManager
	activeSessions map[string]*types.Session // sessionID -> Session
	mu            sync.Mutex // Simplified to regular mutex for single active session
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
	// Add database timeout for session loading
	dbCtx, cancel := context.WithTimeout(ctx, sessionLoadTimeout)
	defer cancel()
	sessions, err := m.dbManager.ListActiveSessions(dbCtx)
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

// HasActiveSession checks if any session is currently active
func (m *Manager) HasActiveSession(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Fast boolean check using in-memory cache
	for _, session := range m.activeSessions {
		if session.Status == "active" {
			return true, nil
		}
	}
	
	return false, nil
}

// GetActiveSession returns the currently active session or nil
func (m *Manager) GetActiveSession(ctx context.Context) (*types.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Single session enforcement means at most one active session
	for _, session := range m.activeSessions {
		if session.Status == "active" {
			return session, nil
		}
	}
	
	return nil, nil
}

// CreateSession creates a new session with direct mutex protection
func (m *Manager) CreateSession(ctx context.Context, name string, createdBy string, studentIDs []string) (*types.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check for existing active session
	for _, session := range m.activeSessions {
		if session.Status == "active" {
			return nil, ErrActiveSessionExists
		}
	}
	
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
	
	// Add to in-memory cache optimistically to prevent race conditions
	// This ensures concurrent CreateSession calls see the session immediately
	m.activeSessions[session.ID] = session
	
	// Persist to database with timeout
	dbCtx, cancel := context.WithTimeout(ctx, sessionWriteTimeout)
	defer cancel()
	if err := m.dbManager.CreateSession(dbCtx, session); err != nil {
		// Remove from cache if database operation fails
		delete(m.activeSessions, session.ID)
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	
	log.Printf("Created session: id=%s name=%s students=%d", session.ID, session.Name, len(session.StudentIDs))
	return session, nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	// Check in-memory cache first
	m.mu.Lock()
	if session, exists := m.activeSessions[sessionID]; exists {
		m.mu.Unlock()
		return session, nil
	}
	m.mu.Unlock()
	
	// Query database for ended sessions or cache misses
	session, err := m.dbManager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	
	return session, nil
}

// EndSession ends an active session with direct mutex protection  
func (m *Manager) EndSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Get session from cache
	session, exists := m.activeSessions[sessionID]
	
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
	
	// Persist to database with timeout
	dbCtx, cancel := context.WithTimeout(ctx, sessionWriteTimeout)
	defer cancel()
	if err := m.dbManager.UpdateSession(dbCtx, session); err != nil {
		return fmt.Errorf("failed to end session: %w", err)
	}
	
	// Remove from active sessions cache (no lock needed - single goroutine)
	delete(m.activeSessions, sessionID)
	
	log.Printf("Ended session: id=%s name=%s", session.ID, session.Name)
	return nil
}

// ListActiveSessions returns all active sessions
func (m *Manager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	sessions := make([]*types.Session, 0, len(m.activeSessions))
	for _, session := range m.activeSessions {
		sessions = append(sessions, session)
	}
	
	return sessions, nil
}

// ValidateSessionMembership checks if user can join session
func (m *Manager) ValidateSessionMembership(sessionID, userID, role string) error {
	// Get session (check cache first)
	m.mu.Lock()
	session, exists := m.activeSessions[sessionID]
	m.mu.Unlock()
	
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
	m.mu.Lock()
	defer m.mu.Unlock()
	
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
	m.mu.Lock()
	defer m.mu.Unlock()
	
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

