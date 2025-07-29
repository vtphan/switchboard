package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"switchboard/pkg/types"
	"switchboard/internal/websocket"
)

// MockSessionManager for testing
type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) CreateSession(ctx context.Context, name string, createdBy string, studentIDs []string) (*types.Session, error) {
	args := m.Called(ctx, name, createdBy, studentIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Session), args.Error(1)
}

func (m *MockSessionManager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Session), args.Error(1)
}

func (m *MockSessionManager) GetActiveSession(ctx context.Context) (*types.Session, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Session), args.Error(1)
}

func (m *MockSessionManager) EndSession(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockSessionManager) HasActiveSession(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (m *MockSessionManager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Session), args.Error(1)
}

func (m *MockSessionManager) ValidateSessionMembership(sessionID, userID, role string) error {
	args := m.Called(sessionID, userID, role)
	return args.Error(0)
}

func (m *MockSessionManager) LoadActiveSessions(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSessionManager) GetStats() map[string]interface{} {
	args := m.Called()
	return args.Get(0).(map[string]interface{})
}

func (m *MockSessionManager) RefreshCache(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSessionManager) IsSessionActive(sessionID string) bool {
	args := m.Called(sessionID)
	return args.Bool(0)
}

// MockDatabaseManager for testing
type MockDatabaseManager struct {
	mock.Mock
}

func (m *MockDatabaseManager) CreateSession(ctx context.Context, session *types.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockDatabaseManager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Session), args.Error(1)
}

func (m *MockDatabaseManager) UpdateSession(ctx context.Context, session *types.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockDatabaseManager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Session), args.Error(1)
}

func (m *MockDatabaseManager) StoreMessage(ctx context.Context, message *types.Message) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockDatabaseManager) StoreMessageBatch(ctx context.Context, messages []*types.Message) error {
	args := m.Called(ctx, messages)
	return args.Error(0)
}

func (m *MockDatabaseManager) GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Message), args.Error(1)
}

func (m *MockDatabaseManager) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDatabaseManager) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockRegistry for testing
type MockRegistry struct {
	mock.Mock
}

func (m *MockRegistry) GetSessionConnections(sessionID string) []*websocket.Connection {
	args := m.Called(sessionID)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]*websocket.Connection)
}

func (m *MockRegistry) GetStats() map[string]int {
	args := m.Called()
	return args.Get(0).(map[string]int)
}

func (m *MockRegistry) BroadcastToUsers(userIDs []string, message interface{}) {
	m.Called(userIDs, message)
}

func (m *MockRegistry) GetLobbyConnections() []*websocket.Connection {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]*websocket.Connection)
}

func (m *MockRegistry) TransitionUserToSession(userID, sessionID string) error {
	args := m.Called(userID, sessionID)
	return args.Error(0)
}

func (m *MockRegistry) TransitionUserToLobby(userID string) error {
	args := m.Called(userID)
	return args.Error(0)
}

// Test DELETE /api/sessions/active endpoint
func TestEndActiveSession_Success(t *testing.T) {
	// ARCHITECTURAL VALIDATION: Test new endpoint routing
	sessionManager := new(MockSessionManager)
	dbManager := new(MockDatabaseManager)
	registry := new(MockRegistry)
	server := NewServer(sessionManager, dbManager, registry)

	// Setup: Active session exists
	activeSession := &types.Session{
		ID:         "test-session-123",
		Name:       "Test Session",
		CreatedBy:  "teacher_001",
		StudentIDs: []string{"student_001", "student_002"},
		Status:     "active",
	}

	// FUNCTIONAL VALIDATION: Test successful active session ending
	sessionManager.On("GetActiveSession", mock.Anything).Return(activeSession, nil)
	sessionManager.On("GetSession", mock.Anything, "test-session-123").Return(activeSession, nil)
	sessionManager.On("EndSession", mock.Anything, "test-session-123").Return(nil)
	
	// Mock registry transitions
	registry.On("BroadcastToUsers", mock.Anything, mock.Anything).Return()
	registry.On("TransitionUserToLobby", "teacher_001").Return(nil)
	registry.On("TransitionUserToLobby", "student_001").Return(nil)
	registry.On("TransitionUserToLobby", "student_002").Return(nil)

	// Make request
	req := httptest.NewRequest("DELETE", "/api/sessions/active", nil)
	w := httptest.NewRecorder()

	// This should fail initially (RED phase)
	server.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "Session ended successfully", response["message"])

	// Verify all participants were transitioned to lobby
	registry.AssertCalled(t, "TransitionUserToLobby", "teacher_001")
	registry.AssertCalled(t, "TransitionUserToLobby", "student_001")
	registry.AssertCalled(t, "TransitionUserToLobby", "student_002")
}

func TestEndActiveSession_Idempotent(t *testing.T) {
	// FUNCTIONAL VALIDATION: Test idempotent behavior
	sessionManager := new(MockSessionManager)
	dbManager := new(MockDatabaseManager)
	registry := new(MockRegistry)
	server := NewServer(sessionManager, dbManager, registry)

	// Setup: No active session
	sessionManager.On("GetActiveSession", mock.Anything).Return(nil, nil)

	// Make request
	req := httptest.NewRequest("DELETE", "/api/sessions/active", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	// Should return 200 OK for idempotency
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "No active session to end", response["message"])
}

func TestEndActiveSession_ConcurrentRequests(t *testing.T) {
	// TECHNICAL VALIDATION: Test concurrent access
	sessionManager := new(MockSessionManager)
	dbManager := new(MockDatabaseManager)
	registry := new(MockRegistry)
	server := NewServer(sessionManager, dbManager, registry)

	activeSession := &types.Session{
		ID:         "concurrent-session",
		Name:       "Concurrent Test",
		CreatedBy:  "teacher_001",
		StudentIDs: []string{"student_001"},
		Status:     "active",
	}

	// First call returns session, second returns nil (already ended)
	sessionManager.On("GetActiveSession", mock.Anything).Return(activeSession, nil).Once()
	sessionManager.On("GetActiveSession", mock.Anything).Return(nil, nil).Once()
	sessionManager.On("GetSession", mock.Anything, "concurrent-session").Return(activeSession, nil)
	sessionManager.On("EndSession", mock.Anything, "concurrent-session").Return(nil)
	
	registry.On("BroadcastToUsers", mock.Anything, mock.Anything).Return()
	registry.On("TransitionUserToLobby", mock.Anything).Return(nil)

	// Run two concurrent requests
	done := make(chan bool, 2)
	
	for i := 0; i < 2; i++ {
		go func() {
			req := httptest.NewRequest("DELETE", "/api/sessions/active", nil)
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}()
	}

	// Wait for both to complete
	<-done
	<-done
}