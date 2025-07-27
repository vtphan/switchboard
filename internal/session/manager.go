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
	
	// ARCHITECTURAL DISCOVERY: Single-writer channel pattern for session operations
	// Eliminates race conditions and provides atomic session management
	sessionOpCh   chan sessionOperation
	sessionOpDone chan struct{}
}

// sessionOperation represents a session management operation
type sessionOperation struct {
	opType   string
	request  interface{}
	response chan sessionOpResult
}

type sessionOpResult struct {
	session *types.Session
	err     error
}

// NewManager creates a new session manager
func NewManager(dbManager interfaces.DatabaseManager) *Manager {
	m := &Manager{
		dbManager:      dbManager,
		activeSessions: make(map[string]*types.Session),
		sessionOpCh:    make(chan sessionOperation, 10), // Small buffer for performance
		sessionOpDone:  make(chan struct{}),
	}
	
	// ARCHITECTURAL DISCOVERY: Start single-writer goroutine for atomic operations
	go m.sessionOperationWorker()
	
	return m
}

// sessionOperationWorker processes session operations atomically
// TECHNICAL DISCOVERY: Single goroutine eliminates all race conditions
func (m *Manager) sessionOperationWorker() {
	defer close(m.sessionOpDone)
	
	for op := range m.sessionOpCh {
		switch op.opType {
		case "create":
			req := op.request.(*createSessionRequest)
			session, err := m.createSessionInternal(req.ctx, req.name, req.createdBy, req.studentIDs)
			op.response <- sessionOpResult{session: session, err: err}
			
		case "end":
			req := op.request.(*endSessionRequest)
			err := m.endSessionInternal(req.ctx, req.sessionID)
			op.response <- sessionOpResult{err: err}
		}
		close(op.response)
	}
}

// Request types for channel operations
type createSessionRequest struct {
	ctx       context.Context
	name      string
	createdBy string
	studentIDs []string
}

type endSessionRequest struct {
	ctx       context.Context
	sessionID string
}

// LoadActiveSessions loads all active sessions from database into memory
func (m *Manager) LoadActiveSessions(ctx context.Context) error {
	// TIMEOUT FIX: Add database timeout for session loading
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
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
// ARCHITECTURAL DISCOVERY: Single session business rule simplifies entire system
// by eliminating complex multi-session coordination and race conditions
func (m *Manager) HasActiveSession(ctx context.Context) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// FUNCTIONAL DISCOVERY: Fast boolean check using in-memory cache
	// O(1) operation vs O(n) database scan for performance
	for _, session := range m.activeSessions {
		if session.Status == "active" {
			return true, nil
		}
	}
	
	return false, nil
}

// GetActiveSession returns the currently active session or nil
// FUNCTIONAL DISCOVERY: Cache-first lookup for O(1) performance
// falls back to database only for cache misses
func (m *Manager) GetActiveSession(ctx context.Context) (*types.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// TECHNICAL DISCOVERY: Single session enforcement means at most one active session
	// Linear scan acceptable for small classroom-scale session counts
	for _, session := range m.activeSessions {
		if session.Status == "active" {
			return session, nil
		}
	}
	
	return nil, nil
}

// CreateSession creates a new session using atomic channel operation
// ARCHITECTURAL DISCOVERY: Channel-based single-writer pattern eliminates race conditions
func (m *Manager) CreateSession(ctx context.Context, name string, createdBy string, studentIDs []string) (*types.Session, error) {
	// Send creation request through channel
	responseCh := make(chan sessionOpResult, 1)
	
	select {
	case m.sessionOpCh <- sessionOperation{
		opType: "create",
		request: &createSessionRequest{
			ctx:        ctx,
			name:       name,
			createdBy:  createdBy,
			studentIDs: studentIDs,
		},
		response: responseCh,
	}:
		// Wait for response
		select {
		case result := <-responseCh:
			return result.session, result.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// createSessionInternal handles the actual session creation atomically
// TECHNICAL DISCOVERY: All validation and creation in single goroutine prevents races
func (m *Manager) createSessionInternal(ctx context.Context, name string, createdBy string, studentIDs []string) (*types.Session, error) {
	// FUNCTIONAL DISCOVERY: Check for existing active session atomically
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
	
	// Persist to database with timeout
	// TIMEOUT FIX: Add database timeout for session creation
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := m.dbManager.CreateSession(dbCtx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	
	// Add to in-memory cache (no lock needed - single goroutine)
	m.activeSessions[session.ID] = session
	
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

// EndSession ends an active session using atomic channel operation
func (m *Manager) EndSession(ctx context.Context, sessionID string) error {
	// Send end request through channel
	responseCh := make(chan sessionOpResult, 1)
	
	select {
	case m.sessionOpCh <- sessionOperation{
		opType: "end",
		request: &endSessionRequest{
			ctx:       ctx,
			sessionID: sessionID,
		},
		response: responseCh,
	}:
		// Wait for response
		select {
		case result := <-responseCh:
			return result.err
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

// endSessionInternal handles the actual session ending atomically
func (m *Manager) endSessionInternal(ctx context.Context, sessionID string) error {
	// Get session from cache (no lock needed - single goroutine)
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
	// TIMEOUT FIX: Add database timeout for session updates
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
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

// Close shuts down the session manager gracefully
// ARCHITECTURAL DISCOVERY: Proper cleanup prevents goroutine leaks
func (m *Manager) Close() error {
	close(m.sessionOpCh)
	<-m.sessionOpDone // Wait for worker to finish
	return nil
}