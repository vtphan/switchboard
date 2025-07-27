package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"switchboard/internal/session"
	"switchboard/internal/websocket"
	"switchboard/pkg/types"
)

// Test suite for single session enforcement at API layer
// Following TDD methodology from .claude/commands/implement-step.md

// ARCHITECTURAL VALIDATION TESTS

func TestAPI_SingleSessionEnforcement_ArchitecturalCompliance(t *testing.T) {
	// Verify API layer maintains clean separation of concerns
	server := createTestAPIServer(t)
	
	// Test should fail initially - no conflict handling implemented yet
	// This verifies that HTTP 409 responses are properly structured
	req := createSessionRequest{
		Name:         "Test Session",
		InstructorID: "instructor1", 
		StudentIDs:   []string{"student1"},
	}
	
	// Create first session - should succeed
	resp1 := makeCreateSessionRequest(t, server, req)
	if resp1.Code != http.StatusCreated {
		t.Errorf("First session creation should succeed, got status %d", resp1.Code)
	}
	
	// Create second session - should fail with 409
	resp2 := makeCreateSessionRequest(t, server, req)
	if resp2.Code == http.StatusCreated {
		t.Error("Expected second session creation to fail with 409 Conflict")
	}
	
	// This test will fail because API doesn't handle ErrActiveSessionExists yet
	if resp2.Code != http.StatusConflict {
		t.Errorf("Expected HTTP 409 Conflict, got %d", resp2.Code)
	}
}

// FUNCTIONAL VALIDATION TESTS

func TestAPI_CreateSession_Returns409WhenActiveExists(t *testing.T) {
	// Should fail - API doesn't check for active sessions yet
	server := createTestAPIServer(t)
	
	// Create first session
	req := createSessionRequest{
		Name:         "Math Class",
		InstructorID: "instructor1",
		StudentIDs:   []string{"student1", "student2"},
	}
	
	resp1 := makeCreateSessionRequest(t, server, req)
	if resp1.Code != http.StatusCreated {
		t.Fatalf("First session creation failed: %d", resp1.Code)
	}
	
	// Try to create second session - should return 409
	req2 := createSessionRequest{
		Name:         "Science Class", 
		InstructorID: "instructor2",
		StudentIDs:   []string{"student3", "student4"},
	}
	
	resp2 := makeCreateSessionRequest(t, server, req2)
	
	// Test will fail because server doesn't handle conflict yet
	if resp2.Code != http.StatusConflict {
		t.Errorf("Expected HTTP 409 Conflict, got %d", resp2.Code)
	}
	
	// Verify error response structure
	var errorResp ActiveSessionConflictResponse
	if err := json.NewDecoder(resp2.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}
	
	// Test will fail because ActiveSessionConflictResponse doesn't exist yet
	if errorResp.Error != "Active session exists" {
		t.Errorf("Expected 'Active session exists', got '%s'", errorResp.Error)
	}
	
	if errorResp.ActiveSession == nil {
		t.Error("Expected active session details in error response")
	}
}

func TestAPI_CreateSession_SucceedsAfterSessionEnded(t *testing.T) {
	// Should pass - tests proper workflow after session is ended
	server := createTestAPIServer(t)
	
	// Create first session
	req := createSessionRequest{
		Name:         "First Session",
		InstructorID: "instructor1",
		StudentIDs:   []string{"student1"},
	}
	
	resp1 := makeCreateSessionRequest(t, server, req)
	if resp1.Code != http.StatusCreated {
		t.Fatalf("First session creation failed: %d", resp1.Code)
	}
	
	// Get session ID from response
	var session1Resp CreateSessionResponse
	if err := json.NewDecoder(resp1.Body).Decode(&session1Resp); err != nil {
		t.Fatalf("Failed to decode session response: %v", err)
	}
	
	// End the session
	endReq := httptest.NewRequest(http.MethodDelete, "/api/sessions/"+session1Resp.Session.ID, nil)
	endResp := httptest.NewRecorder()
	server.ServeHTTP(endResp, endReq)
	
	if endResp.Code != http.StatusOK {
		t.Fatalf("Failed to end session: %d", endResp.Code)
	}
	
	// Now second session should succeed
	req2 := createSessionRequest{
		Name:         "Second Session",
		InstructorID: "instructor2", 
		StudentIDs:   []string{"student2"},
	}
	
	resp2 := makeCreateSessionRequest(t, server, req2)
	if resp2.Code != http.StatusCreated {
		t.Errorf("Second session creation should succeed after first ended, got %d", resp2.Code)
	}
}

func TestAPI_ErrorResponse_IncludesActiveSessionDetails(t *testing.T) {
	// Should fail - proper error response structure not implemented yet
	server := createTestAPIServer(t)
	
	// Create first session with specific details
	req1 := createSessionRequest{
		Name:         "Advanced Mathematics",
		InstructorID: "prof_smith",
		StudentIDs:   []string{"alice", "bob", "charlie"},
	}
	
	resp1 := makeCreateSessionRequest(t, server, req1)
	if resp1.Code != http.StatusCreated {
		t.Fatalf("First session creation failed: %d", resp1.Code)
	}
	
	// Try to create conflicting session
	req2 := createSessionRequest{
		Name:         "Basic Physics",
		InstructorID: "prof_jones", 
		StudentIDs:   []string{"david", "eve"},
	}
	
	resp2 := makeCreateSessionRequest(t, server, req2)
	
	// Should return detailed conflict information
	if resp2.Code != http.StatusConflict {
		t.Fatalf("Expected 409 Conflict, got %d", resp2.Code)
	}
	
	var errorResp ActiveSessionConflictResponse
	if err := json.NewDecoder(resp2.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}
	
	// Test will fail because detailed error response not implemented
	expectedMessage := "Cannot create new session. Active session 'Advanced Mathematics' must be ended first."
	if errorResp.Message != expectedMessage {
		t.Errorf("Expected message '%s', got '%s'", expectedMessage, errorResp.Message)
	}
	
	if errorResp.ActiveSession.Name != "Advanced Mathematics" {
		t.Errorf("Expected active session name 'Advanced Mathematics', got '%s'", errorResp.ActiveSession.Name)
	}
	
	if errorResp.ActiveSession.CreatedBy != "prof_smith" {
		t.Errorf("Expected instructor 'prof_smith', got '%s'", errorResp.ActiveSession.CreatedBy)
	}
}

// TECHNICAL VALIDATION TESTS

func TestAPI_ConflictHandling_Performance(t *testing.T) {
	// Should pass - tests that conflict detection is fast
	server := createTestAPIServer(t)
	
	// Create first session
	req1 := createSessionRequest{
		Name:         "Performance Test Session",
		InstructorID: "instructor1",
		StudentIDs:   []string{"student1"},
	}
	
	makeCreateSessionRequest(t, server, req1)
	
	// Measure conflict detection performance
	req2 := createSessionRequest{
		Name:         "Conflicting Session",
		InstructorID: "instructor2",
		StudentIDs:   []string{"student2"},
	}
	
	start := time.Now()
	resp := makeCreateSessionRequest(t, server, req2)
	duration := time.Since(start)
	
	// Should be very fast (< 10ms) due to in-memory cache
	if duration > 10*time.Millisecond {
		t.Errorf("Conflict detection too slow: %v, expected < 10ms", duration)
	}
	
	if resp.Code != http.StatusConflict {
		t.Errorf("Expected 409 Conflict, got %d", resp.Code)
	}
}

func TestAPI_ConflictHandling_Concurrency(t *testing.T) {
	// Should pass - tests concurrent requests are handled safely
	server := createTestAPIServer(t)
	
	const numRequests = 10
	results := make(chan int, numRequests)
	
	req := createSessionRequest{
		Name:         "Concurrent Test Session",
		InstructorID: "instructor1",
		StudentIDs:   []string{"student1"},
	}
	
	// Send multiple creation requests concurrently
	for i := 0; i < numRequests; i++ {
		go func() {
			resp := makeCreateSessionRequest(t, server, req)
			results <- resp.Code
		}()
	}
	
	// Collect results
	var createdCount, conflictCount int
	for i := 0; i < numRequests; i++ {
		status := <-results
		switch status {
		case http.StatusCreated:
			createdCount++
		case http.StatusConflict:
			conflictCount++
		default:
			t.Errorf("Unexpected status code: %d", status)
		}
	}
	
	// Exactly one should succeed, rest should conflict
	if createdCount != 1 {
		t.Errorf("Expected exactly 1 creation, got %d", createdCount)
	}
	
	if conflictCount != numRequests-1 {
		t.Errorf("Expected %d conflicts, got %d", numRequests-1, conflictCount)
	}
}

// Helper functions and types

type createSessionRequest struct {
	Name         string   `json:"name"`
	InstructorID string   `json:"instructor_id"`
	StudentIDs   []string `json:"student_ids"`
}

// ActiveSessionConflictResponse is now defined in server.go

func createTestAPIServer(t *testing.T) *Server {
	// Create test database manager
	dbManager := &TestDBManager{
		sessions: make(map[string]*types.Session),
		messages: make(map[string][]*types.Message),
	}
	
	// Create session manager
	sessionManager := session.NewManager(dbManager)
	
	// Clean up session manager
	t.Cleanup(func() {
		_ = sessionManager.Close()
	})
	
	// Create test registry
	registry := &TestRegistry{}
	
	// Create API server
	server := NewServer(sessionManager, dbManager, registry)
	
	return server
}

func makeCreateSessionRequest(t *testing.T, server *Server, req createSessionRequest) *httptest.ResponseRecorder {
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	
	httpReq := httptest.NewRequest(http.MethodPost, "/api/sessions", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httpReq)
	
	return recorder
}

// Test mocks

type TestDBManager struct {
	mu       sync.RWMutex
	sessions map[string]*types.Session
	messages map[string][]*types.Message
}

func (db *TestDBManager) CreateSession(ctx context.Context, session *types.Session) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.sessions[session.ID] = session
	return nil
}

func (db *TestDBManager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if session, exists := db.sessions[sessionID]; exists {
		return session, nil
	}
	return nil, session.ErrSessionNotFound
}

func (db *TestDBManager) UpdateSession(ctx context.Context, session *types.Session) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.sessions[session.ID] = session
	return nil
}

func (db *TestDBManager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var active []*types.Session
	for _, session := range db.sessions {
		if session.Status == "active" {
			active = append(active, session)
		}
	}
	return active, nil
}

func (db *TestDBManager) StoreMessage(ctx context.Context, message *types.Message) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.messages[message.SessionID] = append(db.messages[message.SessionID], message)
	return nil
}

func (db *TestDBManager) GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.messages[sessionID], nil
}

func (db *TestDBManager) HealthCheck(ctx context.Context) error {
	return nil
}

func (db *TestDBManager) Close() error {
	return nil
}

type TestRegistry struct{}

func (r *TestRegistry) GetSessionConnections(sessionID string) []*websocket.Connection {
	return nil
}

func (r *TestRegistry) GetStats() map[string]int {
	return map[string]int{"total_connections": 0, "active_sessions": 0}
}

func (r *TestRegistry) BroadcastToUsers(userIDs []string, message interface{}) {
	// No-op for testing
}

// New methods for auto-transition (Phase 4)
func (r *TestRegistry) GetLobbyConnections() []*websocket.Connection {
	return []*websocket.Connection{}
}

func (r *TestRegistry) TransitionUserToSession(userID, sessionID string) error {
	return nil
}