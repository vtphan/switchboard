package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"switchboard/internal/session"
	"switchboard/pkg/types"
)

// Test suite for WebSocket auto-assignment functionality
// Following TDD methodology from .claude/commands/implement-step.md

// ARCHITECTURAL VALIDATION TESTS

func TestWebSocket_AutoAssignment_ArchitecturalCompliance(t *testing.T) {
	// Verify WebSocket handler maintains clean separation of concerns
	handler := createTestWebSocketHandler(t)
	
	// Test should fail initially - no auto-assignment implemented yet
	// This verifies that students connecting without session_id get auto-assigned
	
	// Create a test session first
	sessionManager := handler.sessionManager.(*TestSessionManager)
	session := &types.Session{
		ID:         "test-session-123",
		Name:       "Math Class",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"alice", "bob"},
		Status:     "active",
		StartTime:  time.Now(),
	}
	sessionManager.activeSession = session
	
	// Student connects without session_id - should get auto-assigned
	req := createWebSocketRequest("alice", "student", "")
	recorder := httptest.NewRecorder()
	
	// This test will fail because auto-assignment logic doesn't exist yet
	handler.HandleWebSocket(recorder, req)
	
	// Should have been upgraded to WebSocket (not rejected)
	if recorder.Code != 0 && recorder.Code != http.StatusSwitchingProtocols {
		t.Errorf("Expected WebSocket upgrade, got HTTP %d", recorder.Code)
	}
	
	// Verify connection was registered to the session (not lobby)
	// This will fail because current implementation assigns to lobby
	connections := handler.registry.GetSessionConnections("test-session-123")
	if len(connections) == 0 {
		t.Error("Expected student to be auto-assigned to active session")
	}
	
	// Verify not assigned to lobby
	lobbyConnections := handler.registry.GetLobbyConnections()
	if len(lobbyConnections) > 0 {
		t.Error("Expected student to be assigned to session, not lobby")
	}
}

// FUNCTIONAL VALIDATION TESTS

func TestWebSocket_AutoAssignment_StudentToActiveSession(t *testing.T) {
	// Should fail - auto-assignment logic doesn't exist yet
	handler := createTestWebSocketHandler(t)
	
	// Setup: Create active session with specific students
	sessionManager := handler.sessionManager.(*TestSessionManager)
	session := &types.Session{
		ID:         "session-abc",
		Name:       "Physics Class",
		CreatedBy:  "prof_jones",
		StudentIDs: []string{"alice", "bob", "charlie"},
		Status:     "active",
		StartTime:  time.Now(),
	}
	sessionManager.activeSession = session
	
	// Test Case 1: Student in session connects without session_id
	req1 := createWebSocketRequest("alice", "student", "")
	recorder1 := httptest.NewRecorder()
	
	// This will fail because current implementation assigns to lobby
	handler.HandleWebSocket(recorder1, req1)
	
	// Verify alice was assigned to the session
	sessionConnections := handler.registry.GetSessionConnections("session-abc")
	aliceInSession := false
	for _, conn := range sessionConnections {
		if conn.GetUserID() == "alice" && conn.GetSessionID() == "session-abc" {
			aliceInSession = true
			break
		}
	}
	if !aliceInSession {
		t.Error("Expected alice to be auto-assigned to session-abc")
	}
	
	// Test Case 2: Student NOT in session connects without session_id
	req2 := createWebSocketRequest("david", "student", "")
	recorder2 := httptest.NewRecorder()
	
	handler.HandleWebSocket(recorder2, req2)
	
	// Verify david was assigned to lobby (correct behavior)
	lobbyConnections := handler.registry.GetLobbyConnections()
	davidInLobby := false
	for _, conn := range lobbyConnections {
		if conn.GetUserID() == "david" && conn.GetSessionID() == "lobby" {
			davidInLobby = true
			break
		}
	}
	if !davidInLobby {
		t.Error("Expected david to be assigned to lobby (not in session)")
	}
}

func TestWebSocket_AutoAssignment_InstructorBehavior(t *testing.T) {
	// Should pass - instructors have universal access
	handler := createTestWebSocketHandler(t)
	
	// Setup: Create active session
	sessionManager := handler.sessionManager.(*TestSessionManager)
	session := &types.Session{
		ID:         "session-xyz",
		Name:       "Chemistry Class", 
		CreatedBy:  "prof_smith",
		StudentIDs: []string{"alice", "bob"},
		Status:     "active",
		StartTime:  time.Now(),
	}
	sessionManager.activeSession = session
	
	// Test Case 1: Original instructor connects without session_id
	req1 := createWebSocketRequest("prof_smith", "instructor", "")
	recorder1 := httptest.NewRecorder()
	
	// This will fail because current implementation assigns to lobby
	handler.HandleWebSocket(recorder1, req1)
	
	// Verify instructor was assigned to the session
	sessionConnections := handler.registry.GetSessionConnections("session-xyz")
	instructorInSession := false
	for _, conn := range sessionConnections {
		if conn.GetUserID() == "prof_smith" && conn.GetSessionID() == "session-xyz" {
			instructorInSession = true
			break
		}
	}
	if !instructorInSession {
		t.Error("Expected instructor to be auto-assigned to their session")
	}
	
	// Test Case 2: Different instructor connects without session_id  
	req2 := createWebSocketRequest("prof_jones", "instructor", "")
	recorder2 := httptest.NewRecorder()
	
	handler.HandleWebSocket(recorder2, req2)
	
	// Verify other instructor was also assigned to session (universal access)
	instructorInSession2 := false
	sessionConnections2 := handler.registry.GetSessionConnections("session-xyz")
	for _, conn := range sessionConnections2 {
		if conn.GetUserID() == "prof_jones" && conn.GetSessionID() == "session-xyz" {
			instructorInSession2 = true
			break
		}
	}
	if !instructorInSession2 {
		t.Error("Expected any instructor to be auto-assigned to active session")
	}
}

func TestWebSocket_AutoAssignment_NoActiveSession(t *testing.T) {
	// Should pass - current behavior when no active session
	handler := createTestWebSocketHandler(t)
	
	// Setup: No active session
	sessionManager := handler.sessionManager.(*TestSessionManager)
	sessionManager.activeSession = nil
	
	// Student connects without session_id
	req := createWebSocketRequest("alice", "student", "")
	recorder := httptest.NewRecorder()
	
	handler.HandleWebSocket(recorder, req)
	
	// Verify student was assigned to lobby (correct fallback)
	lobbyConnections := handler.registry.GetLobbyConnections()
	aliceInLobby := false
	for _, conn := range lobbyConnections {
		if conn.GetUserID() == "alice" && conn.GetSessionID() == "lobby" {
			aliceInLobby = true
			break
		}
	}
	if !aliceInLobby {
		t.Error("Expected student to be assigned to lobby when no active session")
	}
}

func TestWebSocket_AutoAssignment_ExplicitSessionID(t *testing.T) {
	// Should pass - explicit session_id should override auto-assignment  
	handler := createTestWebSocketHandler(t)
	
	// Setup: Create active session
	sessionManager := handler.sessionManager.(*TestSessionManager)
	session := &types.Session{
		ID:         "session-auto",
		Name:       "Auto Assignment Test",
		CreatedBy:  "instructor1", 
		StudentIDs: []string{"alice"},
		Status:     "active",
		StartTime:  time.Now(),
	}
	sessionManager.activeSession = session
	
	// Student connects with explicit lobby session_id (should override auto-assignment)
	req := createWebSocketRequest("alice", "student", "lobby")
	recorder := httptest.NewRecorder()
	
	handler.HandleWebSocket(recorder, req)
	
	// Verify alice was assigned to lobby (explicit choice honored)
	lobbyConnections := handler.registry.GetLobbyConnections()
	aliceInLobby := false
	for _, conn := range lobbyConnections {
		if conn.GetUserID() == "alice" && conn.GetSessionID() == "lobby" {
			aliceInLobby = true
			break
		}
	}
	if !aliceInLobby {
		t.Error("Expected explicit lobby session_id to override auto-assignment")
	}
	
	// Verify alice was NOT auto-assigned to session
	sessionConnections := handler.registry.GetSessionConnections("session-auto")
	for _, conn := range sessionConnections {
		if conn.GetUserID() == "alice" {
			t.Error("Explicit lobby choice should override auto-assignment")
		}
	}
}

// TECHNICAL VALIDATION TESTS

func TestWebSocket_AutoAssignment_Performance(t *testing.T) {
	// Should pass - auto-assignment should be fast
	handler := createTestWebSocketHandler(t)
	
	// Setup: Create session
	sessionManager := handler.sessionManager.(*TestSessionManager)
	session := &types.Session{
		ID:         "perf-session",
		Name:       "Performance Test",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"alice"},
		Status:     "active",
		StartTime:  time.Now(),
	}
	sessionManager.activeSession = session
	
	// Measure auto-assignment performance
	req := createWebSocketRequest("alice", "student", "")
	recorder := httptest.NewRecorder()
	
	start := time.Now()
	handler.HandleWebSocket(recorder, req)
	duration := time.Since(start)
	
	// Should be fast (< 10ms) using in-memory cache
	if duration > 10*time.Millisecond {
		t.Errorf("Auto-assignment too slow: %v, expected < 10ms", duration)
	}
}

func TestWebSocket_AutoAssignment_Concurrency(t *testing.T) {
	// Should pass - concurrent auto-assignments should be safe
	handler := createTestWebSocketHandler(t)
	
	// Setup: Create session with multiple students
	sessionManager := handler.sessionManager.(*TestSessionManager)
	session := &types.Session{
		ID:         "concurrent-session",
		Name:       "Concurrency Test",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"alice", "bob", "charlie", "david", "eve"},
		Status:     "active",
		StartTime:  time.Now(),
	}
	sessionManager.activeSession = session
	
	const numStudents = 5
	students := []string{"alice", "bob", "charlie", "david", "eve"}
	results := make(chan bool, numStudents)
	
	// Connect all students concurrently
	for _, studentID := range students {
		go func(id string) {
			req := createWebSocketRequest(id, "student", "")
			recorder := httptest.NewRecorder()
			handler.HandleWebSocket(recorder, req)
			
			// Check if assigned to session
			sessionConnections := handler.registry.GetSessionConnections("concurrent-session")
			assigned := false
			for _, conn := range sessionConnections {
				if conn.GetUserID() == id {
					assigned = true
					break
				}
			}
			results <- assigned
		}(studentID)
	}
	
	// Verify all students were auto-assigned
	for i := 0; i < numStudents; i++ {
		assigned := <-results
		if !assigned {
			t.Errorf("Student %d failed concurrent auto-assignment", i)
		}
	}
}

// Helper functions and mocks

func createTestWebSocketHandler(t *testing.T) *Handler {
	registry := NewRegistry()
	sessionManager := &TestSessionManager{}
	dbManager := &TestDBManager{}
	hub := &TestHub{}
	
	handler := NewHandler(registry, sessionManager, dbManager, hub)
	
	// Clean up registry
	t.Cleanup(func() {
		// No specific cleanup needed for registry
	})
	
	return handler
}

func createWebSocketRequest(userID, role, sessionID string) *http.Request {
	// Create proper WebSocket upgrade request
	req := httptest.NewRequest("GET", "/ws", nil)
	
	// Add required headers for WebSocket upgrade
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	
	// Add query parameters
	q := url.Values{}
	q.Set("user_id", userID)
	q.Set("role", role)
	if sessionID != "" {
		q.Set("session_id", sessionID)
	}
	req.URL.RawQuery = q.Encode()
	
	return req
}

// Test mocks

type TestSessionManager struct {
	activeSession *types.Session
}

func (m *TestSessionManager) CreateSession(ctx context.Context, name, createdBy string, studentIDs []string) (*types.Session, error) {
	return nil, nil // Not used in auto-assignment tests
}

func (m *TestSessionManager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	if m.activeSession != nil && m.activeSession.ID == sessionID {
		return m.activeSession, nil
	}
	return nil, session.ErrSessionNotFound
}

func (m *TestSessionManager) EndSession(ctx context.Context, sessionID string) error {
	if m.activeSession != nil && m.activeSession.ID == sessionID {
		m.activeSession = nil
	}
	return nil
}

func (m *TestSessionManager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	if m.activeSession != nil {
		return []*types.Session{m.activeSession}, nil
	}
	return []*types.Session{}, nil
}

func (m *TestSessionManager) ValidateSessionMembership(sessionID, userID, role string) error {
	if m.activeSession == nil {
		return session.ErrSessionNotFound
	}
	
	if sessionID != m.activeSession.ID {
		return session.ErrSessionNotFound
	}
	
	if role == "instructor" {
		return nil // Instructors have universal access
	}
	
	if role == "student" {
		for _, studentID := range m.activeSession.StudentIDs {
			if studentID == userID {
				return nil
			}
		}
		return session.ErrUnauthorized
	}
	
	return session.ErrInvalidRole
}

// New methods for single session enforcement
func (m *TestSessionManager) HasActiveSession(ctx context.Context) (bool, error) {
	return m.activeSession != nil, nil
}

func (m *TestSessionManager) GetActiveSession(ctx context.Context) (*types.Session, error) {
	return m.activeSession, nil
}

type TestDBManager struct{}

func (m *TestDBManager) CreateSession(ctx context.Context, session *types.Session) error {
	return nil
}

func (m *TestDBManager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	return nil, session.ErrSessionNotFound
}

func (m *TestDBManager) UpdateSession(ctx context.Context, session *types.Session) error {
	return nil
}

func (m *TestDBManager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	return []*types.Session{}, nil
}

func (m *TestDBManager) StoreMessage(ctx context.Context, message *types.Message) error {
	return nil
}

func (m *TestDBManager) GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error) {
	return []*types.Message{}, nil
}

func (m *TestDBManager) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *TestDBManager) Close() error {
	return nil
}

type TestHub struct{}

func (h *TestHub) SendMessage(message *types.Message, senderID string) error {
	return nil
}