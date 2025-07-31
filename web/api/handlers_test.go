package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/pkg/errors"
)

// MockSessionLifecycle provides a test double for SessionLifecycleInterface
type MockSessionLifecycle struct {
	startSessionFunc func(name, instructorID string) (*database.Session, error)
	endSessionFunc   func(instructorID string) (*database.Session, error)
}

func (m *MockSessionLifecycle) StartSession(name, instructorID string) (*database.Session, error) {
	if m.startSessionFunc != nil {
		return m.startSessionFunc(name, instructorID)
	}
	return nil, fmt.Errorf("mock not configured")
}

func (m *MockSessionLifecycle) EndSession(instructorID string) (*database.Session, error) {
	if m.endSessionFunc != nil {
		return m.endSessionFunc(instructorID)
	}
	return nil, fmt.Errorf("mock not configured")
}

// Architectural Tests

func TestArchitectural_SessionAPIHandlerStruct(t *testing.T) {
	handlerType := reflect.TypeOf(SessionAPIHandler{})
	assert.Equal(t, "SessionAPIHandler", handlerType.Name())
	assert.True(t, handlerType.Kind() == reflect.Struct)

	// Verify required field exists with correct type
	sessionLifecycleField, exists := handlerType.FieldByName("sessionLifecycle")
	require.True(t, exists, "sessionLifecycle field must exist")
	assert.Contains(t, sessionLifecycleField.Type.String(), "SessionLifecycleInterface")
}

func TestArchitectural_NewSessionAPIHandlerFunction(t *testing.T) {
	funcType := reflect.TypeOf(NewSessionAPIHandler)
	assert.Equal(t, "func", funcType.Kind().String())
	
	// Verify function signature: NewSessionAPIHandler(SessionLifecycleInterface) *SessionAPIHandler
	assert.Equal(t, 1, funcType.NumIn())
	assert.Equal(t, 1, funcType.NumOut())
	assert.Contains(t, funcType.In(0).String(), "SessionLifecycleInterface")
	assert.Equal(t, "*api.SessionAPIHandler", funcType.Out(0).String())
}

func TestArchitectural_StartSessionMethod(t *testing.T) {
	handlerType := reflect.TypeOf(&SessionAPIHandler{})
	
	startMethod, exists := handlerType.MethodByName("StartSession")
	require.True(t, exists, "StartSession method must exist")
	
	// Verify method signature: StartSession(w http.ResponseWriter, r *http.Request)
	assert.Equal(t, 2, startMethod.Type.NumIn()-1) // Subtract receiver
	assert.Equal(t, 0, startMethod.Type.NumOut())
	
	// Input parameters should match HTTP handler signature
	assert.True(t, startMethod.Type.In(1).Implements(reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()))
	assert.Equal(t, "*http.Request", startMethod.Type.In(2).String())
}

func TestArchitectural_EndSessionMethod(t *testing.T) {
	handlerType := reflect.TypeOf(&SessionAPIHandler{})
	
	endMethod, exists := handlerType.MethodByName("EndSession")
	require.True(t, exists, "EndSession method must exist")
	
	// Verify method signature: EndSession(w http.ResponseWriter, r *http.Request)
	assert.Equal(t, 2, endMethod.Type.NumIn()-1) // Subtract receiver
	assert.Equal(t, 0, endMethod.Type.NumOut())
	
	// Input parameters should match HTTP handler signature
	assert.True(t, endMethod.Type.In(1).Implements(reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()))
	assert.Equal(t, "*http.Request", endMethod.Type.In(2).String())
}

func TestArchitectural_WriteErrorResponseFunction(t *testing.T) {
	funcType := reflect.TypeOf(writeErrorResponse)
	assert.Equal(t, "func", funcType.Kind().String())
	
	// Verify function signature: writeErrorResponse(w http.ResponseWriter, statusCode int, errorType, message string)
	assert.Equal(t, 4, funcType.NumIn())
	assert.Equal(t, 0, funcType.NumOut())
	assert.True(t, funcType.In(0).Implements(reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()))
	assert.Equal(t, "int", funcType.In(1).String())
	assert.Equal(t, "string", funcType.In(2).String())
	assert.Equal(t, "string", funcType.In(3).String())
}

// Functional Tests

func TestFunctional_NewSessionAPIHandler(t *testing.T) {
	mockLifecycle := &MockSessionLifecycle{}
	
	handler := NewSessionAPIHandler(mockLifecycle)
	
	require.NotNil(t, handler)
	assert.Equal(t, mockLifecycle, handler.sessionLifecycle)
}

func TestFunctional_StartSession_Success(t *testing.T) {
	// Setup mock with successful response
	mockSession := &database.Session{
		ID:        "test-session-id",
		Name:      "Test Session",
		CreatedBy: "instructor-123",
		Status:    "active",
		StartTime: time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC),
	}
	
	mockLifecycle := &MockSessionLifecycle{
		startSessionFunc: func(name, instructorID string) (*database.Session, error) {
			assert.Equal(t, "Test Session", name)
			assert.Equal(t, "instructor-123", instructorID)
			return mockSession, nil
		},
	}
	
	handler := NewSessionAPIHandler(mockLifecycle)
	
	// Create request
	requestBody := map[string]string{
		"name":          "Test Session",
		"instructor_id": "instructor-123",
	}
	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	// Execute
	handler.StartSession(w, req)
	
	// Verify response
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	sessionData := response["session"].(map[string]interface{})
	assert.Equal(t, "test-session-id", sessionData["id"])
	assert.Equal(t, "Test Session", sessionData["name"])
	assert.Equal(t, "instructor-123", sessionData["created_by"])
	assert.Equal(t, "active", sessionData["status"])
	assert.Equal(t, "2025-01-15T14:30:00Z", sessionData["start_time"])
}

func TestFunctional_StartSession_SessionAlreadyActive(t *testing.T) {
	mockLifecycle := &MockSessionLifecycle{
		startSessionFunc: func(name, instructorID string) (*database.Session, error) {
			return nil, errors.ErrSessionAlreadyActive
		},
	}
	
	handler := NewSessionAPIHandler(mockLifecycle)
	
	requestBody := map[string]string{
		"name":          "Test Session",
		"instructor_id": "instructor-123",
	}
	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	handler.StartSession(w, req)
	
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	errorData := response["error"].(map[string]interface{})
	assert.Equal(t, "session_already_active", errorData["type"])
	assert.Equal(t, "A session is already active", errorData["message"])
	assert.NotEmpty(t, response["timestamp"])
}

func TestFunctional_StartSession_InternalError(t *testing.T) {
	mockLifecycle := &MockSessionLifecycle{
		startSessionFunc: func(name, instructorID string) (*database.Session, error) {
			return nil, fmt.Errorf("database connection failed")
		},
	}
	
	handler := NewSessionAPIHandler(mockLifecycle)
	
	requestBody := map[string]string{
		"name":          "Test Session",
		"instructor_id": "instructor-123",
	}
	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	handler.StartSession(w, req)
	
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	errorData := response["error"].(map[string]interface{})
	assert.Equal(t, "internal_error", errorData["type"])
	assert.Equal(t, "Failed to create session", errorData["message"])
}

func TestFunctional_EndSession_Success(t *testing.T) {
	endTime := time.Date(2025, 1, 15, 16, 30, 0, 0, time.UTC)
	mockSession := &database.Session{
		ID:        "test-session-id",
		Name:      "Test Session",
		CreatedBy: "instructor-123",
		Status:    "ended",
		StartTime: time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC),
		EndTime:   &endTime,
	}
	
	mockLifecycle := &MockSessionLifecycle{
		endSessionFunc: func(instructorID string) (*database.Session, error) {
			assert.Equal(t, "instructor-123", instructorID)
			return mockSession, nil
		},
	}
	
	handler := NewSessionAPIHandler(mockLifecycle)
	
	requestBody := map[string]string{
		"instructor_id": "instructor-123",
	}
	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/session/end", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	handler.EndSession(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "test-session-id", response["session_id"])
	assert.Equal(t, "ended", response["status"])
	assert.Equal(t, "2025-01-15T16:30:00Z", response["ended_at"])
}

func TestFunctional_EndSession_NoActiveSession(t *testing.T) {
	mockLifecycle := &MockSessionLifecycle{
		endSessionFunc: func(instructorID string) (*database.Session, error) {
			return nil, errors.ErrNoActiveSession
		},
	}
	
	handler := NewSessionAPIHandler(mockLifecycle)
	
	requestBody := map[string]string{
		"instructor_id": "instructor-123",
	}
	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/session/end", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	handler.EndSession(w, req)
	
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	errorData := response["error"].(map[string]interface{})
	assert.Equal(t, "no_active_session", errorData["type"])
	assert.Equal(t, "No active session to end", errorData["message"])
}

// Technical Tests

func TestTechnical_StartSession_InvalidMethod(t *testing.T) {
	handler := NewSessionAPIHandler(&MockSessionLifecycle{})
	
	req := httptest.NewRequest(http.MethodGet, "/api/session/start", nil)
	w := httptest.NewRecorder()
	
	handler.StartSession(w, req)
	
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Contains(t, w.Body.String(), "Method not allowed")
}

func TestTechnical_StartSession_InvalidJSON(t *testing.T) {
	handler := NewSessionAPIHandler(&MockSessionLifecycle{})
	
	req := httptest.NewRequest(http.MethodPost, "/api/session/start", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()
	
	handler.StartSession(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	errorData := response["error"].(map[string]interface{})
	assert.Equal(t, "validation_error", errorData["type"])
	assert.Equal(t, "Invalid JSON format", errorData["message"])
}

func TestTechnical_StartSession_MissingName(t *testing.T) {
	handler := NewSessionAPIHandler(&MockSessionLifecycle{})
	
	requestBody := map[string]string{
		"instructor_id": "instructor-123",
	}
	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	handler.StartSession(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	errorData := response["error"].(map[string]interface{})
	assert.Equal(t, "validation_error", errorData["type"])
	assert.Equal(t, "Session name is required", errorData["message"])
}

func TestTechnical_StartSession_MissingInstructorID(t *testing.T) {
	handler := NewSessionAPIHandler(&MockSessionLifecycle{})
	
	requestBody := map[string]string{
		"name": "Test Session",
	}
	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	handler.StartSession(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	errorData := response["error"].(map[string]interface{})
	assert.Equal(t, "validation_error", errorData["type"])
	assert.Equal(t, "Instructor ID is required", errorData["message"])
}

func TestTechnical_EndSession_InvalidMethod(t *testing.T) {
	handler := NewSessionAPIHandler(&MockSessionLifecycle{})
	
	req := httptest.NewRequest(http.MethodGet, "/api/session/end", nil)
	w := httptest.NewRecorder()
	
	handler.EndSession(w, req)
	
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Contains(t, w.Body.String(), "Method not allowed")
}

func TestTechnical_EndSession_InvalidJSON(t *testing.T) {
	handler := NewSessionAPIHandler(&MockSessionLifecycle{})
	
	req := httptest.NewRequest(http.MethodPost, "/api/session/end", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()
	
	handler.EndSession(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	errorData := response["error"].(map[string]interface{})
	assert.Equal(t, "validation_error", errorData["type"])
	assert.Equal(t, "Invalid JSON format", errorData["message"])
}

func TestTechnical_EndSession_MissingInstructorID(t *testing.T) {
	handler := NewSessionAPIHandler(&MockSessionLifecycle{})
	
	requestBody := map[string]string{}
	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/session/end", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	handler.EndSession(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	errorData := response["error"].(map[string]interface{})
	assert.Equal(t, "validation_error", errorData["type"])
	assert.Equal(t, "Instructor ID is required", errorData["message"])
}

func TestTechnical_WriteErrorResponse_Format(t *testing.T) {
	w := httptest.NewRecorder()
	
	writeErrorResponse(w, http.StatusBadRequest, "test_error", "Test error message")
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	// Verify error structure
	require.Contains(t, response, "error")
	require.Contains(t, response, "timestamp")
	
	errorData := response["error"].(map[string]interface{})
	assert.Equal(t, "test_error", errorData["type"])
	assert.Equal(t, "Test error message", errorData["message"])
	
	// Verify timestamp is valid RFC3339
	timestamp := response["timestamp"].(string)
	_, err = time.Parse(time.RFC3339, timestamp)
	assert.NoError(t, err)
}

// Integration Tests with Real SessionLifecycle

func TestIntegration_StartSession_WithRealSessionLifecycle(t *testing.T) {
	// This test would require setting up a real SessionLifecycle with mock dependencies
	// For now, we'll skip it as it requires database setup
	t.Skip("Integration test requires database setup")
}

func TestIntegration_EndSession_WithRealSessionLifecycle(t *testing.T) {
	// This test would require setting up a real SessionLifecycle with mock dependencies
	// For now, we'll skip it as it requires database setup
	t.Skip("Integration test requires database setup")
}

// Edge Case Tests

func TestEdgeCase_StartSession_EmptyStringFields(t *testing.T) {
	handler := NewSessionAPIHandler(&MockSessionLifecycle{})
	
	requestBody := map[string]string{
		"name":          "",
		"instructor_id": "",
	}
	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
	w := httptest.NewRecorder()
	
	handler.StartSession(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	errorData := response["error"].(map[string]interface{})
	assert.Equal(t, "validation_error", errorData["type"])
	assert.Equal(t, "Session name is required", errorData["message"])
}

func TestEdgeCase_ConcurrentRequests(t *testing.T) {
	mockLifecycle := &MockSessionLifecycle{
		startSessionFunc: func(name, instructorID string) (*database.Session, error) {
			return &database.Session{
				ID:        "test-id",
				Name:      name,
				CreatedBy: instructorID,
				Status:    "active",
				StartTime: time.Now(),
			}, nil
		},
	}
	
	handler := NewSessionAPIHandler(mockLifecycle)
	
	// Make multiple concurrent requests
	const numRequests = 10
	responses := make([]*httptest.ResponseRecorder, numRequests)
	var wg sync.WaitGroup
	
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			requestBody := map[string]string{
				"name":          fmt.Sprintf("Session %d", index),
				"instructor_id": fmt.Sprintf("instructor-%d", index),
			}
			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
			responses[index] = httptest.NewRecorder()
			
			handler.StartSession(responses[index], req)
		}(i)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	// Verify all requests completed successfully
	for i, w := range responses {
		assert.Equal(t, http.StatusCreated, w.Code, "Request %d failed", i)
	}
}