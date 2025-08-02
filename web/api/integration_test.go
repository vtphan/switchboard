package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/internal/session"
)

// mockSystemBroadcaster for tests
type mockSystemBroadcaster struct{}

func (m *mockSystemBroadcaster) BroadcastSessionStarted(sessionID, sessionName, startedBy string, startTime time.Time) error {
	return nil
}

func (m *mockSystemBroadcaster) BroadcastSessionEnded(sessionID, endedBy string, endTime time.Time) error {
	return nil
}
// MockDatabaseManager provides a test double for DatabaseManager
type MockDatabaseManager struct {
	createSessionFunc func(*database.Session) error
	updateSessionFunc func(*database.Session) error
}

func (m *MockDatabaseManager) CreateSession(s *database.Session) error {
	if m.createSessionFunc != nil {
		return m.createSessionFunc(s)
	}
	return nil
}

func (m *MockDatabaseManager) UpdateSession(s *database.Session) error {
	if m.updateSessionFunc != nil {
		return m.updateSessionFunc(s)
	}
	return nil
}

func (m *MockDatabaseManager) GetActiveSession() (*database.Session, error) {
	return nil, nil
}

func (m *MockDatabaseManager) WriteMessage(msg *database.Message) error {
	return nil
}

func (m *MockDatabaseManager) WriteBatch(msgs []*database.Message) error {
	return nil
}

func (m *MockDatabaseManager) GetSessionMessages(sessionID string) ([]*database.Message, error) {
	return nil, nil
}

func (m *MockDatabaseManager) Start() error {
	return nil
}

func (m *MockDatabaseManager) Stop() error {
	return nil
}

func (m *MockDatabaseManager) WaitForPendingWrites() error {
	return nil
}

func TestIntegration_SessionAPIHandler_WithRealSessionLifecycle(t *testing.T) {
	// Setup real SessionLifecycle with mock dependencies
	dbManager := &MockDatabaseManager{}
	sessionManager := session.NewSessionManager(dbManager)
	sessionLifecycle := session.NewSessionLifecycle(sessionManager, dbManager, &mockSystemBroadcaster{})
	
	// Create API handler with real SessionLifecycle
	handler := NewSessionAPIHandler(sessionLifecycle)
	
	// Test StartSession
	requestBody := map[string]string{
		"name":          "Integration Test Session",
		"instructor_id": "prof_integration",
	}
	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	handler.StartSession(w, req)
	
	// Verify successful session creation
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	
	var startResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &startResponse)
	require.NoError(t, err)
	
	sessionData := startResponse["session"].(map[string]interface{})
	assert.Equal(t, "Integration Test Session", sessionData["name"])
	assert.Equal(t, "prof_integration", sessionData["created_by"])
	assert.Equal(t, "active", sessionData["status"])
	assert.NotEmpty(t, sessionData["id"])
	assert.NotEmpty(t, sessionData["start_time"])
	
	// Test EndSession
	endRequestBody := map[string]string{
		"instructor_id": "prof_integration",
	}
	endBody, _ := json.Marshal(endRequestBody)
	endReq := httptest.NewRequest(http.MethodPost, "/api/session/end", bytes.NewReader(endBody))
	endW := httptest.NewRecorder()
	
	handler.EndSession(endW, endReq)
	
	// Verify successful session end
	assert.Equal(t, http.StatusOK, endW.Code)
	assert.Equal(t, "application/json", endW.Header().Get("Content-Type"))
	
	var endResponse map[string]interface{}
	err = json.Unmarshal(endW.Body.Bytes(), &endResponse)
	require.NoError(t, err)
	
	assert.Equal(t, sessionData["id"], endResponse["session_id"])
	assert.Equal(t, "ended", endResponse["status"])
	assert.NotEmpty(t, endResponse["ended_at"])
}

func TestIntegration_SessionAPIHandler_ConflictHandling(t *testing.T) {
	// Setup real SessionLifecycle
	dbManager := &MockDatabaseManager{}
	sessionManager := session.NewSessionManager(dbManager)
	sessionLifecycle := session.NewSessionLifecycle(sessionManager, dbManager, &mockSystemBroadcaster{})
	handler := NewSessionAPIHandler(sessionLifecycle)
	
	// Start first session
	requestBody := map[string]string{
		"name":          "First Session",
		"instructor_id": "prof_test",
	}
	body, _ := json.Marshal(requestBody)
	req1 := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
	w1 := httptest.NewRecorder()
	
	handler.StartSession(w1, req1)
	assert.Equal(t, http.StatusCreated, w1.Code)
	
	// Try to start second session (should fail with conflict)
	requestBody2 := map[string]string{
		"name":          "Second Session",
		"instructor_id": "prof_test2",
	}
	body2, _ := json.Marshal(requestBody2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	
	handler.StartSession(w2, req2)
	
	// Verify conflict response
	assert.Equal(t, http.StatusConflict, w2.Code)
	
	var conflictResponse map[string]interface{}
	err := json.Unmarshal(w2.Body.Bytes(), &conflictResponse)
	require.NoError(t, err)
	
	errorData := conflictResponse["error"].(map[string]interface{})
	assert.Equal(t, "session_already_active", errorData["type"])
	assert.Equal(t, "A session is already active", errorData["message"])
}