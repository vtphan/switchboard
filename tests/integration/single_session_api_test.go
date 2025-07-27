package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"switchboard/internal/api"
	"switchboard/internal/session"
	wsocket "switchboard/internal/websocket"
	"switchboard/pkg/types"
)

// Test suite for Phase 5: End-to-End Single Session API Integration
// Following TDD methodology from .claude/commands/implement-step.md

// ARCHITECTURAL VALIDATION TESTS

func TestE2E_API_SingleSession_ArchitecturalCompliance(t *testing.T) {
	// Verify complete single session architecture via API endpoints
	server := createAPITestServer(t)
	defer server.Cleanup()
	
	// Test should fail initially - complete workflow not validated yet
	// This validates the entire single session workflow via API
	
	// Phase 1: Create first session
	session1 := createSessionViaAPI(t, server, "Math Class", "instructor1", []string{"alice", "bob"})
	if session1 == nil {
		t.Fatal("Failed to create first session")
	}
	
	// Verify session is active
	if session1.Status != "active" {
		t.Errorf("Expected session to be active, got %s", session1.Status)
	}
	
	// Phase 2: Try to create second session - should fail with 409
	resp := makeCreateSessionRequest(t, server, "Science Class", "instructor2", []string{"charlie"})
	if resp.Code != http.StatusConflict {
		t.Errorf("Expected HTTP 409 Conflict, got %d", resp.Code)
	}
	
	// This will fail if conflict response doesn't include session details
	var conflictResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&conflictResp); err != nil {
		t.Fatalf("Failed to decode conflict response: %v", err)
	}
	
	if conflictResp["error"] != "Active session exists" {
		t.Error("Expected detailed conflict error message")
	}
	
	activeSession := conflictResp["active_session"].(map[string]interface{})
	if activeSession["name"] != "Math Class" {
		t.Error("Conflict response should include active session details")
	}
	
	// Phase 3: End session and verify cleanup
	endResp := makeEndSessionRequest(t, server, session1.ID)
	if endResp.Code != http.StatusOK {
		t.Errorf("Failed to end session: HTTP %d", endResp.Code)
	}
	
	// Phase 4: Create new session after ending - should succeed
	session2 := createSessionViaAPI(t, server, "Chemistry Class", "instructor2", []string{"charlie"})
	if session2 == nil {
		t.Error("Should be able to create session after previous ended")
	}
}

// FUNCTIONAL VALIDATION TESTS

func TestE2E_API_SingleSession_CompleteWorkflow(t *testing.T) {
	// Should fail - complete workflow validation not implemented yet
	server := createAPITestServer(t)
	defer server.Cleanup()
	
	// Test realistic classroom API workflow
	// 1. Create session
	session := createSessionViaAPI(t, server, "Physics Lab", "prof_wilson", []string{"alice", "bob", "charlie"})
	if session == nil {
		t.Fatal("Failed to create session")
	}
	
	// 2. Get session details
	getResp := makeGetSessionRequest(t, server, session.ID)
	if getResp.Code != http.StatusOK {
		t.Errorf("Failed to get session details: HTTP %d", getResp.Code)
	}
	
	var sessionResponse map[string]interface{}
	if err := json.NewDecoder(getResp.Body).Decode(&sessionResponse); err != nil {
		t.Fatalf("Failed to decode session response: %v", err)
	}
	
	sessionData := sessionResponse["session"].(map[string]interface{})
	if sessionData["name"] != "Physics Lab" {
		t.Error("Session details should match creation request")
	}
	
	// Connection count should be included (even if 0 for API-only test)
	if _, exists := sessionResponse["connection_count"]; !exists {
		t.Error("Session response should include connection count")
	}
	
	// 3. List active sessions
	listResp := makeListSessionsRequest(t, server)
	if listResp.Code != http.StatusOK {
		t.Errorf("Failed to list sessions: HTTP %d", listResp.Code)
	}
	
	var listResponse map[string]interface{}
	if err := json.NewDecoder(listResp.Body).Decode(&listResponse); err != nil {
		t.Fatalf("Failed to decode list response: %v", err)
	}
	
	sessions := listResponse["sessions"].([]interface{})
	if len(sessions) != 1 {
		t.Errorf("Expected 1 active session, got %d", len(sessions))
	}
	
	// 4. End session
	endResp := makeEndSessionRequest(t, server, session.ID)
	if endResp.Code != http.StatusOK {
		t.Errorf("Failed to end session: HTTP %d", endResp.Code)
	}
	
	// 5. Verify session is no longer active
	listResp2 := makeListSessionsRequest(t, server)
	if listResp2.Code != http.StatusOK {
		t.Errorf("Failed to list sessions after end: HTTP %d", listResp2.Code)
	}
	
	var listResponse2 map[string]interface{}
	if err := json.NewDecoder(listResp2.Body).Decode(&listResponse2); err != nil {
		t.Fatalf("Failed to decode list response: %v", err)
	}
	
	sessions2 := listResponse2["sessions"].([]interface{})
	if len(sessions2) != 0 {
		t.Errorf("Expected 0 active sessions after end, got %d", len(sessions2))
	}
}

func TestE2E_API_SingleSession_StudentDuplicateHandling(t *testing.T) {
	// Should pass - tests duplicate student ID removal
	server := createAPITestServer(t)
	defer server.Cleanup()
	
	// Create session with duplicate student IDs
	reqBody := map[string]interface{}{
		"name":         "Duplicate Test",
		"instructor_id": "instructor1",
		"student_ids":   []string{"alice", "bob", "alice", "charlie", "bob", "alice"},
	}
	
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	
	req := httptest.NewRequest("POST", "/api/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	server.apiServer.ServeHTTP(recorder, req)
	
	if recorder.Code != http.StatusCreated {
		t.Errorf("Expected HTTP 201, got %d", recorder.Code)
	}
	
	var response map[string]interface{}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	sessionData := response["session"].(map[string]interface{})
	studentIDs := sessionData["student_ids"].([]interface{})
	
	// Should have exactly 3 unique students
	if len(studentIDs) != 3 {
		t.Errorf("Expected 3 unique students after deduplication, got %d", len(studentIDs))
	}
	
	// Verify specific students are present
	studentSet := make(map[string]bool)
	for _, id := range studentIDs {
		studentSet[id.(string)] = true
	}
	
	expectedStudents := []string{"alice", "bob", "charlie"}
	for _, expected := range expectedStudents {
		if !studentSet[expected] {
			t.Errorf("Expected student %s to be in session", expected)
		}
	}
}

func TestE2E_API_SingleSession_ErrorHandling(t *testing.T) {
	// Should pass - tests proper error responses
	server := createAPITestServer(t)
	defer server.Cleanup()
	
	// Test missing required fields
	testCases := []struct {
		name           string
		body           map[string]interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Missing session name",
			body:           map[string]interface{}{"instructor_id": "inst1", "student_ids": []string{"s1"}},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Session name is required",
		},
		{
			name:           "Missing instructor ID",
			body:           map[string]interface{}{"name": "Test", "student_ids": []string{"s1"}},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Instructor ID is required",
		},
		{
			name:           "Empty student list",
			body:           map[string]interface{}{"name": "Test", "instructor_id": "inst1", "student_ids": []string{}},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "At least one student ID is required",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.body)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}
			
			req := httptest.NewRequest("POST", "/api/sessions", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			
			recorder := httptest.NewRecorder()
			server.apiServer.ServeHTTP(recorder, req)
			
			if recorder.Code != tc.expectedStatus {
				t.Errorf("Expected HTTP %d, got %d", tc.expectedStatus, recorder.Code)
			}
			
			var errorResp map[string]interface{}
			if err := json.NewDecoder(recorder.Body).Decode(&errorResp); err != nil {
				t.Fatalf("Failed to decode error response: %v", err)
			}
			
			if errorResp["message"] != tc.expectedError {
				t.Errorf("Expected error message '%s', got '%s'", tc.expectedError, errorResp["message"])
			}
		})
	}
}

// TECHNICAL VALIDATION TESTS

func TestE2E_API_SingleSession_ConcurrencyStress(t *testing.T) {
	// Should pass - tests concurrent session creation
	server := createAPITestServer(t)
	defer server.Cleanup()
	
	const numConcurrent = 20
	results := make(chan int, numConcurrent)
	
	// Try to create multiple sessions concurrently
	for i := 0; i < numConcurrent; i++ {
		go func(id int) {
			resp := makeCreateSessionRequest(t, server, 
				"Concurrent Session", "instructor1", []string{"student1"})
			results <- resp.Code
		}(i)
	}
	
	// Collect results
	var successCount, conflictCount int
	for i := 0; i < numConcurrent; i++ {
		status := <-results
		switch status {
		case http.StatusCreated:
			successCount++
		case http.StatusConflict:
			conflictCount++
		default:
			t.Errorf("Unexpected status code: %d", status)
		}
	}
	
	// Exactly one should succeed due to single session enforcement
	if successCount != 1 {
		t.Errorf("Expected exactly 1 success, got %d", successCount)
	}
	if conflictCount != numConcurrent-1 {
		t.Errorf("Expected %d conflicts, got %d", numConcurrent-1, conflictCount)
	}
}

func TestE2E_API_SingleSession_Performance(t *testing.T) {
	// Should pass - tests API performance under normal load
	server := createAPITestServer(t)
	defer server.Cleanup()
	
	// Create session with many students
	const numStudents = 100
	studentIDs := make([]string, numStudents)
	for i := 0; i < numStudents; i++ {
		studentIDs[i] = "student" + string(rune('0'+i%10)) + string(rune('0'+(i/10)%10))
	}
	
	start := time.Now()
	session := createSessionViaAPI(t, server, "Large Class", "instructor1", studentIDs)
	duration := time.Since(start)
	
	if session == nil {
		t.Fatal("Failed to create session with many students")
	}
	
	// Should handle large student lists efficiently (< 100ms)
	if duration > 100*time.Millisecond {
		t.Errorf("Session creation too slow: %v, expected < 100ms", duration)
	}
	
	// Verify all students are in session
	if len(session.StudentIDs) != numStudents {
		t.Errorf("Expected %d students, got %d", numStudents, len(session.StudentIDs))
	}
}

// Helper types and functions

type APITestServer struct {
	apiServer *api.Server
	sessionMgr *session.Manager
	cleanup   func()
}

func (s *APITestServer) Cleanup() {
	if s.cleanup != nil {
		s.cleanup()
	}
}

func createAPITestServer(t *testing.T) *APITestServer {
	// Create test database manager
	dbManager := &TestDBManager{
		sessions: make(map[string]*types.Session),
		messages: make(map[string][]*types.Message),
	}
	
	// Create session manager
	sessionMgr := session.NewManager(dbManager)
	
	// Create WebSocket registry (for API interface compliance)
	wsRegistry := wsocket.NewRegistry()
	
	// Create API server
	apiServer := api.NewServer(sessionMgr, dbManager, wsRegistry)
	
	cleanup := func() {
		sessionMgr.Close()
	}
	
	t.Cleanup(cleanup)
	
	return &APITestServer{
		apiServer:  apiServer,
		sessionMgr: sessionMgr,
		cleanup:    cleanup,
	}
}

func createSessionViaAPI(t *testing.T, server *APITestServer, name, instructorID string, studentIDs []string) *types.Session {
	resp := makeCreateSessionRequest(t, server, name, instructorID, studentIDs)
	if resp.Code != http.StatusCreated {
		return nil
	}
	
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode session response: %v", err)
	}
	
	sessionData := response["session"].(map[string]interface{})
	return &types.Session{
		ID:         sessionData["id"].(string),
		Name:       sessionData["name"].(string),
		CreatedBy:  sessionData["created_by"].(string),
		StudentIDs: convertToStringSlice(sessionData["student_ids"].([]interface{})),
		Status:     sessionData["status"].(string),
	}
}

func makeCreateSessionRequest(t *testing.T, server *APITestServer, name, instructorID string, studentIDs []string) *httptest.ResponseRecorder {
	reqBody := map[string]interface{}{
		"name":         name,
		"instructor_id": instructorID,
		"student_ids":   studentIDs,
	}
	
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	
	req := httptest.NewRequest("POST", "/api/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	server.apiServer.ServeHTTP(recorder, req)
	
	return recorder
}

func makeGetSessionRequest(t *testing.T, server *APITestServer, sessionID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/api/sessions/"+sessionID, nil)
	recorder := httptest.NewRecorder()
	server.apiServer.ServeHTTP(recorder, req)
	return recorder
}

func makeListSessionsRequest(t *testing.T, server *APITestServer) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/api/sessions", nil)
	recorder := httptest.NewRecorder()
	server.apiServer.ServeHTTP(recorder, req)
	return recorder
}

func makeEndSessionRequest(t *testing.T, server *APITestServer, sessionID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("DELETE", "/api/sessions/"+sessionID, nil)
	recorder := httptest.NewRecorder()
	server.apiServer.ServeHTTP(recorder, req)
	return recorder
}

func convertToStringSlice(slice []interface{}) []string {
	result := make([]string, len(slice))
	for i, v := range slice {
		result[i] = v.(string)
	}
	return result
}

// Test database manager for API tests

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