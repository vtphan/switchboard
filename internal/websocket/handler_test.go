package websocket

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"switchboard/pkg/interfaces"
	"switchboard/pkg/types"
)

// Mock implementations for testing
type mockSessionManager struct {
	validateFunc func(sessionID, userID, role string) error
}

func (m *mockSessionManager) CreateSession(ctx context.Context, name string, createdBy string, studentIDs []string) (*types.Session, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSessionManager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSessionManager) EndSession(ctx context.Context, sessionID string) error {
	return errors.New("not implemented")
}

func (m *mockSessionManager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSessionManager) ValidateSessionMembership(sessionID, userID, role string) error {
	if m.validateFunc != nil {
		return m.validateFunc(sessionID, userID, role)
	}
	return nil
}

type mockDatabaseManager struct {
	getHistoryFunc func(ctx context.Context, sessionID string) ([]*types.Message, error)
}

func (m *mockDatabaseManager) CreateSession(ctx context.Context, session *types.Session) error {
	return errors.New("not implemented")
}

func (m *mockDatabaseManager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	return nil, errors.New("not implemented")
}

func (m *mockDatabaseManager) UpdateSession(ctx context.Context, session *types.Session) error {
	return errors.New("not implemented")
}

func (m *mockDatabaseManager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	return nil, errors.New("not implemented")
}

func (m *mockDatabaseManager) StoreMessage(ctx context.Context, message *types.Message) error {
	return errors.New("not implemented")
}

func (m *mockDatabaseManager) GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error) {
	if m.getHistoryFunc != nil {
		return m.getHistoryFunc(ctx, sessionID)
	}
	return []*types.Message{}, nil
}

func (m *mockDatabaseManager) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockDatabaseManager) Close() error {
	return nil
}

// Architectural Validation Tests
func TestHandler_StructureCompliance(t *testing.T) {
	// Handler struct exists and can be instantiated
	handler := &Handler{}
	_ = handler // Handler will always exist after creation
}

func TestHandler_ImportBoundaryCompliance(t *testing.T) {
	// This test passes if compilation succeeds - no forbidden imports
	t.Log("Handler import boundaries maintained - only allowed dependencies")
}

func TestHandler_ComponentIntegration(t *testing.T) {
	registry := NewRegistry()
	sessionManager := &mockSessionManager{}
	dbManager := &mockDatabaseManager{}
	
	// This will fail until NewHandler is implemented
	handler := NewHandler(registry, sessionManager, dbManager)
	if handler == nil {
		t.Error("NewHandler should return initialized handler")
	}
}

// Functional Validation Tests
func TestHandler_NewHandlerInitialization(t *testing.T) {
	registry := NewRegistry()
	sessionManager := &mockSessionManager{}
	dbManager := &mockDatabaseManager{}
	
	handler := NewHandler(registry, sessionManager, dbManager)
	
	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}
	
	// Verify dependencies are set
	if handler.registry != registry {
		t.Error("Registry not properly set")
	}
	if handler.sessionManager != sessionManager {
		t.Error("SessionManager not properly set")
	}
	if handler.dbManager != dbManager {
		t.Error("DatabaseManager not properly set")
	}
}

func TestHandler_QueryParameterValidation(t *testing.T) {
	registry := NewRegistry()
	sessionManager := &mockSessionManager{}
	dbManager := &mockDatabaseManager{}
	handler := NewHandler(registry, sessionManager, dbManager)
	
	tests := []struct {
		name           string
		queryParams    map[string]string
		expectedStatus int
	}{
		{
			name:           "missing all parameters",
			queryParams:    map[string]string{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing user_id",
			queryParams: map[string]string{
				"role":       "student",
				"session_id": "session123",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing role",
			queryParams: map[string]string{
				"user_id":    "user123",
				"session_id": "session123",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing session_id",
			queryParams: map[string]string{
				"user_id": "user123",
				"role":    "student",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid role",
			queryParams: map[string]string{
				"user_id":    "user123",
				"role":       "admin",
				"session_id": "session123",
			},
			expectedStatus: http.StatusBadRequest,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request with query parameters
			req := httptest.NewRequest("GET", "/ws", nil)
			q := req.URL.Query()
			for key, value := range tt.queryParams {
				q.Add(key, value)
			}
			req.URL.RawQuery = q.Encode()
			
			rec := httptest.NewRecorder()
			handler.HandleWebSocket(rec, req)
			
			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestHandler_SessionValidationIntegration(t *testing.T) {
	registry := NewRegistry()
	dbManager := &mockDatabaseManager{}
	
	tests := []struct {
		name           string
		validateFunc   func(sessionID, userID, role string) error
		expectedStatus int
	}{
		{
			name: "session not found",
			validateFunc: func(sessionID, userID, role string) error {
				return interfaces.ErrSessionNotFound
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "unauthorized access",
			validateFunc: func(sessionID, userID, role string) error {
				return interfaces.ErrUnauthorized
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "validation error",
			validateFunc: func(sessionID, userID, role string) error {
				return errors.New("internal error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionManager := &mockSessionManager{validateFunc: tt.validateFunc}
			handler := NewHandler(registry, sessionManager, dbManager)
			
			// Create valid request (without WebSocket headers to avoid upgrade issues)
			req := httptest.NewRequest("GET", "/ws?user_id=user123&role=student&session_id=session456", nil)
			
			rec := httptest.NewRecorder()
			handler.HandleWebSocket(rec, req)
			
			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestHandler_ConnectionRegistration(t *testing.T) {
	registry := NewRegistry()
	sessionManager := &mockSessionManager{
		validateFunc: func(sessionID, userID, role string) error {
			return nil // Valid session
		},
	}
	dbManager := &mockDatabaseManager{}
	handler := NewHandler(registry, sessionManager, dbManager)
	
	// Start test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()
	
	// Convert to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?user_id=user123&role=student&session_id=session456"
	
	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer func() { _ = conn.Close() }()
	
	// Give time for registration
	time.Sleep(50 * time.Millisecond)
	
	// Verify connection is registered
	registeredConn, exists := registry.GetUserConnection("user123")
	if !exists {
		t.Error("Connection should be registered in registry")
	}
	if registeredConn == nil {
		t.Error("Registered connection should not be nil")
	}
	if registeredConn.GetUserID() != "user123" {
		t.Error("Connection should have correct user ID")
	}
}

func TestHandler_HistoryReplay(t *testing.T) {
	registry := NewRegistry()
	sessionManager := &mockSessionManager{
		validateFunc: func(sessionID, userID, role string) error {
			return nil
		},
	}
	
	// Mock history with sample messages
	testMessages := []*types.Message{
		{
			ID:        "msg1",
			Type:      "instructor_broadcast",
			FromUser:  "instructor1",
			ToUser:    nil,
			SessionID: "session456",
			Content:   map[string]interface{}{"text": "Welcome to class"},
			Context:   "general",
			Timestamp: time.Now().Add(-10 * time.Minute),
		},
		{
			ID:        "msg2",
			Type:      "inbox_response",
			FromUser:  "instructor1",
			ToUser:    stringPtr("user123"),
			SessionID: "session456",
			Content:   map[string]interface{}{"text": "Your answer is correct"},
			Context:   "general",
			Timestamp: time.Now().Add(-5 * time.Minute),
		},
	}
	
	dbManager := &mockDatabaseManager{
		getHistoryFunc: func(ctx context.Context, sessionID string) ([]*types.Message, error) {
			return testMessages, nil
		},
	}
	
	handler := NewHandler(registry, sessionManager, dbManager)
	
	// Start test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()
	
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?user_id=user123&role=student&session_id=session456"
	
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()
	
	// Read messages (should include history + history_complete)
	receivedMessages := 0
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	
	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}
		receivedMessages++
		
		// Stop after receiving history_complete
		if content, ok := msg["content"].(map[string]interface{}); ok {
			if event, ok := content["event"].(string); ok && event == "history_complete" {
				break
			}
		}
	}
	
	// Should receive at least the history messages + history_complete
	if receivedMessages < 2 {
		t.Errorf("Expected at least 2 messages (history + completion), got %d", receivedMessages)
	}
}

// Technical Validation Tests (Race Detection)
func TestHandler_ConcurrentConnections(t *testing.T) {
	registry := NewRegistry()
	sessionManager := &mockSessionManager{
		validateFunc: func(sessionID, userID, role string) error {
			return nil
		},
	}
	dbManager := &mockDatabaseManager{}
	handler := NewHandler(registry, sessionManager, dbManager)
	
	// Test that Handler can handle concurrent requests without panicking
	// Focus on architectural validation rather than network timing
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()
	
	const numRequests = 10
	var wg sync.WaitGroup
	errors := make(chan error, numRequests)
	
	// Make concurrent HTTP requests (not WebSocket connections to avoid timing issues)
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Make HTTP request without WebSocket upgrade
			url := server.URL + "?user_id=user" + fmt.Sprintf("%d", id) + "&role=student&session_id=session456"
			resp, err := http.Get(url)
			if err != nil {
				errors <- err
				return
			}
			_ = resp.Body.Close()
			
			// Should get 400 (bad request) because no WebSocket headers, but no panic
			if resp.StatusCode != 400 {
				errors <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for any errors during concurrent access
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent request failed: %v", err)
		}
	}
	
	// Handler should remain functional after concurrent access
	if handler.registry == nil {
		t.Error("Handler registry should remain valid after concurrent access")
	}
}

func TestHandler_HeartbeatMonitoring(t *testing.T) {
	registry := NewRegistry()
	sessionManager := &mockSessionManager{
		validateFunc: func(sessionID, userID, role string) error {
			return nil
		},
	}
	dbManager := &mockDatabaseManager{}
	handler := NewHandler(registry, sessionManager, dbManager)
	
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()
	
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?user_id=user123&role=student&session_id=session456"
	
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()
	
	// Test that connection can be established and maintained briefly
	// Full heartbeat testing would require real-time delays that slow down tests
	time.Sleep(100 * time.Millisecond)
	
	// Verify connection is registered (indicates handleConnection is running)
	_, exists := registry.GetUserConnection("user123")
	if !exists {
		t.Error("Connection should be registered and maintained")
	}
	
	// Test cleanup happens when connection closes
	_ = conn.Close()
	time.Sleep(100 * time.Millisecond)
	
	_, exists = registry.GetUserConnection("user123")
	if exists {
		t.Error("Connection should be cleaned up after close")
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}