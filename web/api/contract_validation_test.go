package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/internal/session"
	"switchboard/pkg/errors"
)

// mockSystemBroadcaster for tests
type mockSystemBroadcaster struct{}

func (m *mockSystemBroadcaster) BroadcastSessionStarted(sessionID, sessionName, startedBy string, startTime time.Time) error {
	return nil
}

func (m *mockSystemBroadcaster) BroadcastSessionEnded(sessionID, endedBy string, endTime time.Time) error {
	return nil
}
// TestContractValidation_Step51_IntegrationContracts validates all integration contracts
// for Step 5.1 as specified in integration-graph.yaml
func TestContractValidation_Step51_IntegrationContracts(t *testing.T) {
	t.Run("session_api_management_flow", func(t *testing.T) {
		// This validates the flow: HTTP POST /api/session/start → SessionManager.SetActiveSession()
		
		// Setup mock dependencies
		mockDB := &mockDatabaseManager{}
		sessionManager := session.NewSessionManager(mockDB)
		sessionLifecycle := session.NewSessionLifecycle(sessionManager, mockDB, &mockSystemBroadcaster{})
		handler := NewSessionAPIHandler(sessionLifecycle)

		// Test contract: HTTP POST /api/session/start creates session
		requestBody := map[string]string{
			"name":          "Integration Test Session",
			"instructor_id": "prof_integration",
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
		w := httptest.NewRecorder()

		// Execute
		handler.StartSession(w, req)

		// Validate HTTP response
		assert.Equal(t, http.StatusCreated, w.Code, "HTTP status should be 201 Created")
		
		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err, "Response should be valid JSON")
		
		sessionData := response["session"].(map[string]interface{})
		assert.Equal(t, "Integration Test Session", sessionData["name"])
		assert.Equal(t, "prof_integration", sessionData["created_by"])
		assert.Equal(t, "active", sessionData["status"])

		// Validate integration contract: SessionManager should have active session
		activeSession := sessionManager.GetActiveSession()
		require.NotNil(t, activeSession, "SessionManager should have active session")
		assert.Equal(t, "Integration Test Session", activeSession.Name)
		assert.Equal(t, "prof_integration", activeSession.CreatedBy)
		assert.Equal(t, database.SessionStatusActive, activeSession.Status)
	})

	t.Run("session_end_integration_flow", func(t *testing.T) {
		// This validates the flow: HTTP POST /api/session/end → SessionManager.ClearActiveSession()
		
		// Setup with active session
		mockDB := &mockDatabaseManager{}
		sessionManager := session.NewSessionManager(mockDB)
		sessionLifecycle := session.NewSessionLifecycle(sessionManager, mockDB, &mockSystemBroadcaster{})
		handler := NewSessionAPIHandler(sessionLifecycle)

		// First create a session
		startSession, err := sessionLifecycle.StartSession("Test Session", "prof_test")
		require.NoError(t, err)
		assert.True(t, sessionManager.HasActiveSession())

		// Test contract: HTTP POST /api/session/end ends session
		requestBody := map[string]string{
			"instructor_id": "prof_test",
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/api/session/end", bytes.NewReader(body))
		w := httptest.NewRecorder()

		// Execute
		handler.EndSession(w, req)

		// Validate HTTP response
		assert.Equal(t, http.StatusOK, w.Code, "HTTP status should be 200 OK")
		
		var response map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err, "Response should be valid JSON")
		
		assert.Equal(t, startSession.ID, response["session_id"])
		assert.Equal(t, "ended", response["status"])

		// Validate integration contract: SessionManager should have no active session
		assert.False(t, sessionManager.HasActiveSession(), "SessionManager should have no active session")
		assert.Nil(t, sessionManager.GetActiveSession(), "GetActiveSession should return nil")
	})

	t.Run("error_propagation_contract", func(t *testing.T) {
		// This validates error propagation from SessionLifecycle to HTTP API
		
		// Setup with existing active session
		mockDB := &mockDatabaseManager{}
		sessionManager := session.NewSessionManager(mockDB)
		sessionLifecycle := session.NewSessionLifecycle(sessionManager, mockDB, &mockSystemBroadcaster{})
		handler := NewSessionAPIHandler(sessionLifecycle)

		// Create initial session
		_, err := sessionLifecycle.StartSession("Existing Session", "prof_existing")
		require.NoError(t, err)

		// Test contract: ErrSessionAlreadyActive → 409 Conflict
		requestBody := map[string]string{
			"name":          "Another Session",
			"instructor_id": "prof_another",
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.StartSession(w, req)

		// Validate error propagation
		assert.Equal(t, http.StatusConflict, w.Code, "Should return 409 Conflict")
		
		var response map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		errorData := response["error"].(map[string]interface{})
		assert.Equal(t, "session_already_active", errorData["type"])
		assert.Equal(t, "A session is already active", errorData["message"])
	})

	t.Run("no_active_session_error_contract", func(t *testing.T) {
		// This validates ErrNoActiveSession → 409 Conflict
		
		// Setup with no active session
		mockDB := &mockDatabaseManager{}
		sessionManager := session.NewSessionManager(mockDB)
		sessionLifecycle := session.NewSessionLifecycle(sessionManager, mockDB, &mockSystemBroadcaster{})
		handler := NewSessionAPIHandler(sessionLifecycle)

		// Ensure no active session
		assert.False(t, sessionManager.HasActiveSession())

		// Test contract: ErrNoActiveSession → 409 Conflict
		requestBody := map[string]string{
			"instructor_id": "prof_test",
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/api/session/end", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.EndSession(w, req)

		// Validate error propagation
		assert.Equal(t, http.StatusConflict, w.Code, "Should return 409 Conflict")
		
		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		errorData := response["error"].(map[string]interface{})
		assert.Equal(t, "no_active_session", errorData["type"])
		assert.Equal(t, "No active session to end", errorData["message"])
	})

	t.Run("json_format_compliance_contract", func(t *testing.T) {
		// This validates exact JSON format compliance from tech specs
		
		mockDB := &mockDatabaseManager{}
		sessionManager := session.NewSessionManager(mockDB)
		sessionLifecycle := session.NewSessionLifecycle(sessionManager, mockDB, &mockSystemBroadcaster{})
		handler := NewSessionAPIHandler(sessionLifecycle)

		// Test StartSession response format
		requestBody := map[string]string{
			"name":          "Format Test Session",
			"instructor_id": "prof_format",
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.StartSession(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		
		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		// Validate exact format from tech specs lines 649-657
		sessionData := response["session"].(map[string]interface{})
		requiredFields := []string{"id", "name", "created_by", "status", "start_time"}
		for _, field := range requiredFields {
			assert.Contains(t, sessionData, field, "Response should contain %s field", field)
		}
		
		// Validate RFC3339 timestamp format
		startTime := sessionData["start_time"].(string)
		_, err = time.Parse(time.RFC3339, startTime)
		assert.NoError(t, err, "start_time should be RFC3339 format")
	})
}

// mockDatabaseManager for integration testing
type mockDatabaseManager struct {
	sessions []database.Session
	messages []database.Message
}

func (m *mockDatabaseManager) CreateSession(session *database.Session) error {
	m.sessions = append(m.sessions, *session)
	return nil
}

func (m *mockDatabaseManager) UpdateSession(session *database.Session) error {
	for i, s := range m.sessions {
		if s.ID == session.ID {
			m.sessions[i] = *session
			return nil
		}
	}
	return errors.ErrNoActiveSession
}

func (m *mockDatabaseManager) GetActiveSession() (*database.Session, error) {
	for _, s := range m.sessions {
		if s.Status == database.SessionStatusActive {
			return &s, nil
		}
	}
	return nil, errors.ErrNoActiveSession
}

func (m *mockDatabaseManager) WriteMessage(msg *database.Message) error {
	m.messages = append(m.messages, *msg)
	return nil
}

func (m *mockDatabaseManager) WriteBatch(msgs []*database.Message) error {
	for _, msg := range msgs {
		m.messages = append(m.messages, *msg)
	}
	return nil
}

func (m *mockDatabaseManager) GetSessionMessages(sessionID string) ([]*database.Message, error) {
	var result []*database.Message
	for _, msg := range m.messages {
		if msg.SessionID == sessionID {
			result = append(result, &msg)
		}
	}
	return result, nil
}

func (m *mockDatabaseManager) Start() error { return nil }
func (m *mockDatabaseManager) Stop() error  { return nil }
func (m *mockDatabaseManager) WaitForPendingWrites() error { return nil }