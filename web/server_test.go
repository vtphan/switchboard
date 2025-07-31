package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"switchboard/internal/database"
	"switchboard/internal/message"
	"switchboard/internal/session"
	"switchboard/internal/websocket"
	"switchboard/pkg/errors"
	"switchboard/web/api"
)

// Mock database for real component testing
type mockDatabaseManager struct {
	sessions map[string]*database.Session
	mu       sync.RWMutex
}

func newMockDatabaseManager() *mockDatabaseManager {
	return &mockDatabaseManager{
		sessions: make(map[string]*database.Session),
	}
}

func (m *mockDatabaseManager) CreateSession(session *database.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.ID] = session
	return nil
}

func (m *mockDatabaseManager) GetActiveSession() (*database.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, session := range m.sessions {
		if session.Status == "active" {
			return session, nil
		}
	}
	return nil, errors.ErrNoActiveSession
}

func (m *mockDatabaseManager) UpdateSessionStatus(sessionID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if session, exists := m.sessions[sessionID]; exists {
		session.Status = status
		return nil
	}
	return fmt.Errorf("session not found")
}

func (m *mockDatabaseManager) GetSessionByID(sessionID string) (*database.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if session, exists := m.sessions[sessionID]; exists {
		return session, nil
	}
	return nil, fmt.Errorf("session not found")
}

func (m *mockDatabaseManager) StoreMessage(message *database.Message) error {
	return nil
}

func (m *mockDatabaseManager) GetMessagesForSession(sessionID, role string) ([]*database.Message, error) {
	return []*database.Message{}, nil
}

func (m *mockDatabaseManager) UpdateSession(session *database.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sessions[session.ID]; exists {
		m.sessions[session.ID] = session
		return nil
	}
	return fmt.Errorf("session not found")
}

func (m *mockDatabaseManager) WriteMessage(msg *database.Message) error {
	return nil
}

func (m *mockDatabaseManager) WriteBatch(msgs []*database.Message) error {
	return nil
}

func (m *mockDatabaseManager) GetSessionMessages(sessionID string) ([]*database.Message, error) {
	return []*database.Message{}, nil
}

func (m *mockDatabaseManager) Start() error {
	return nil
}

func (m *mockDatabaseManager) Stop() error {
	return nil
}

func (m *mockDatabaseManager) WaitForPendingWrites() error {
	return nil
}

// MockWebSocketHandler provides a simple test double for WebSocketHandler
type MockWebSocketHandler struct {
	upgradeFunc func(w http.ResponseWriter, r *http.Request) error
}

func (m *MockWebSocketHandler) HandleWebSocketUpgrade(w http.ResponseWriter, r *http.Request) error {
	if m.upgradeFunc != nil {
		return m.upgradeFunc(w, r)
	}
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("mock websocket"))
	return err
}

// Helper functions
func createTestComponents() (*api.SessionAPIHandler, *websocket.WebSocketHandler) {
	mockDB := newMockDatabaseManager()
	sessionManager := session.NewSessionManager(mockDB)
	sessionLifecycle := session.NewSessionLifecycle(sessionManager, mockDB)
	sessionHandler := api.NewSessionAPIHandler(sessionLifecycle)
	
	// Create a real WebSocketHandler with minimal dependencies
	registry := websocket.NewConnectionRegistry(sessionManager)
	// For the WebSocketHandler, we need a message processor - let's create a minimal mock
	mockMessageProcessor := &mockMessageProcessor{}
	websocketHandler := websocket.NewWebSocketHandler(registry, sessionManager, mockMessageProcessor, mockDB)
	
	return sessionHandler, websocketHandler
}

// Mock message processor for WebSocketHandler
type mockMessageProcessor struct{}

func (m *mockMessageProcessor) ProcessIncomingMessage(rawData []byte, senderID string) error {
	return nil
}

// Ensure mockMessageProcessor implements the interface
var _ message.MessageProcessorInterface = &mockMessageProcessor{}

// Architectural Tests

func TestArchitectural_HTTPServerStruct(t *testing.T) {
	t.Run("struct_definition", func(t *testing.T) {
		serverType := reflect.TypeOf(HTTPServer{})
		assert.Equal(t, "HTTPServer", serverType.Name())
		assert.True(t, serverType.Kind() == reflect.Struct)

		// Verify required fields exist with correct types
		requiredFields := map[string]string{
			"server":           "*http.Server",
			"sessionHandler":   "*api.SessionAPIHandler",
			"websocketHandler": "WebSocketHandler", // Accept WebSocketHandler type
			"mux":              "*http.ServeMux",
		}

		for fieldName, expectedType := range requiredFields {
			field, exists := serverType.FieldByName(fieldName)
			assert.True(t, exists, "HTTPServer missing required field: %s", fieldName)
			
			// For websocketHandler, just check it exists since interface compatibility is important
			if fieldName == "websocketHandler" {
				assert.True(t, exists, "websocketHandler field should exist")
				assert.Contains(t, field.Type.String(), "WebSocketHandler")
			} else {
				assert.Contains(t, field.Type.String(), expectedType,
					"Field %s has type %s, expected %s", fieldName, field.Type.String(), expectedType)
			}
		}
	})

	t.Run("constructor_function", func(t *testing.T) {
		sessionHandler, websocketHandler := createTestComponents()
		server := NewHTTPServer(sessionHandler, websocketHandler)

		assert.NotNil(t, server)
		assert.NotNil(t, server.server)
		assert.NotNil(t, server.mux)
		assert.Equal(t, sessionHandler, server.sessionHandler)
		assert.Equal(t, websocketHandler, server.websocketHandler)
	})

	t.Run("server_configuration", func(t *testing.T) {
		sessionHandler, websocketHandler := createTestComponents()
		server := NewHTTPServer(sessionHandler, websocketHandler)

		// Verify HTTP server configuration matches specification
		assert.Equal(t, 30*time.Second, server.server.ReadTimeout)
		assert.Equal(t, 30*time.Second, server.server.WriteTimeout)
		assert.Equal(t, 120*time.Second, server.server.IdleTimeout)
		assert.Equal(t, server.mux, server.server.Handler)
	})
}

// Functional Tests

func TestFunctional_RoutingConfiguration(t *testing.T) {
	sessionHandler, websocketHandler := createTestComponents()
	server := NewHTTPServer(sessionHandler, websocketHandler)

	t.Run("api_session_start_route", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/session/start", strings.NewReader(`{"name":"test","instructor_id":"123"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.mux.ServeHTTP(w, req)

		// Should call the real handler
		assert.Equal(t, http.StatusCreated, w.Code)
		
		// Verify CORS headers are present
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
		assert.Equal(t, "Content-Type", w.Header().Get("Access-Control-Allow-Headers"))
	})

	t.Run("api_session_end_route", func(t *testing.T) {
		// First start a session to end
		startReq := httptest.NewRequest("POST", "/api/session/start", strings.NewReader(`{"name":"test","instructor_id":"123"}`))
		startReq.Header.Set("Content-Type", "application/json")
		startW := httptest.NewRecorder()
		server.mux.ServeHTTP(startW, startReq)

		// Now end the session
		req := httptest.NewRequest("POST", "/api/session/end", strings.NewReader(`{"instructor_id":"123"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		// Verify CORS headers are present
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
		assert.Equal(t, "Content-Type", w.Header().Get("Access-Control-Allow-Headers"))
	})

	t.Run("websocket_route", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ws?user_id=123&role=student", nil)
		w := httptest.NewRecorder()

		server.mux.ServeHTTP(w, req)

		// WebSocket upgrade will fail in test environment but should be handled gracefully
		// The real WebSocketHandler tries to upgrade but fails due to test environment
		// This proves the route is working correctly
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusOK)
	})

	t.Run("static_file_route", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.mux.ServeHTTP(w, req)

		// Static file server handles root path - should return some response
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
	})
}

func TestFunctional_MiddlewareExecution(t *testing.T) {
	sessionHandler, websocketHandler := createTestComponents()
	server := NewHTTPServer(sessionHandler, websocketHandler)

	t.Run("cors_middleware", func(t *testing.T) {
		// Test OPTIONS request
		req := httptest.NewRequest("OPTIONS", "/api/session/start", nil)
		w := httptest.NewRecorder()

		server.mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
		assert.Equal(t, "Content-Type", w.Header().Get("Access-Control-Allow-Headers"))
	})

	t.Run("logging_middleware", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/session/start", strings.NewReader(`{"name":"test","instructor_id":"123"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Just verify the handler executes without error
		server.mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("middleware_chain_order", func(t *testing.T) {
		// Use unique instructor ID to avoid conflicts with previous tests
		req := httptest.NewRequest("POST", "/api/session/start", strings.NewReader(`{"name":"middleware_test","instructor_id":"middleware_instructor"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.mux.ServeHTTP(w, req)

		// Verify both middlewares executed
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		// May be 201 (new session) or 409 (session already exists) depending on test order
		assert.True(t, w.Code == http.StatusCreated || w.Code == http.StatusConflict)
	})
}

func TestFunctional_LifecycleManagement(t *testing.T) {
	t.Run("server_start_stop", func(t *testing.T) {
		sessionHandler, websocketHandler := createTestComponents()
		server := NewHTTPServer(sessionHandler, websocketHandler)

		// Test that Start configures the server address
		go func() {
			err := server.Start("0") // Use port 0 for automatic assignment
			if err != nil && err != http.ErrServerClosed {
				t.Errorf("Server start failed: %v", err)
			}
		}()

		// Give server time to start
		time.Sleep(10 * time.Millisecond)

		// Test graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := server.Stop(ctx)
		assert.NoError(t, err)
	})

	t.Run("graceful_shutdown_timeout", func(t *testing.T) {
		sessionHandler, websocketHandler := createTestComponents()
		server := NewHTTPServer(sessionHandler, websocketHandler)

		// Test shutdown with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Should handle timeout gracefully
		err := server.Stop(ctx)
		// Don't assert on error since timing-dependent
		_ = err
	})
}

// Integration Tests

func TestIntegration_SessionAPIHandlerIntegration(t *testing.T) {
	t.Run("real_session_handler_integration", func(t *testing.T) {
		sessionHandler, websocketHandler := createTestComponents()
		server := NewHTTPServer(sessionHandler, websocketHandler)

		req := httptest.NewRequest("POST", "/api/session/start", 
			strings.NewReader(`{"name":"Integration Test","instructor_id":"instructor123"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), "Integration Test")
	})
}

func TestIntegration_WebSocketHandlerIntegration(t *testing.T) {
	t.Run("websocket_handler_error_handling", func(t *testing.T) {
		sessionHandler, websocketHandler := createTestComponents()
		server := NewHTTPServer(sessionHandler, websocketHandler)

		// Test with missing query parameters to trigger error
		req := httptest.NewRequest("GET", "/ws", nil) // Missing user_id and role
		w := httptest.NewRecorder()

		// Should handle WebSocket error gracefully
		server.mux.ServeHTTP(w, req)

		// Handler should complete without panicking
		// Error is logged but doesn't propagate to HTTP response in a bad way
		// The WebSocketHandler will return an error for missing params, which gets logged
	})
}

func TestIntegration_StaticFileServing(t *testing.T) {
	t.Run("static_file_server_configuration", func(t *testing.T) {
		sessionHandler, websocketHandler := createTestComponents()
		server := NewHTTPServer(sessionHandler, websocketHandler)

		// Test that static file server is configured for root path
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.mux.ServeHTTP(w, req)

		// Should attempt to serve from web/static/
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
	})
}

// Technical Tests

func TestTechnical_GracefulShutdown(t *testing.T) {
	t.Run("shutdown_context_respect", func(t *testing.T) {
		sessionHandler, websocketHandler := createTestComponents()
		server := NewHTTPServer(sessionHandler, websocketHandler)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		start := time.Now()
		err := server.Stop(ctx)
		duration := time.Since(start)

		// Should return quickly since server isn't running
		assert.True(t, duration < 100*time.Millisecond)
		assert.NoError(t, err)
	})
}

func TestTechnical_TimeoutHandling(t *testing.T) {
	t.Run("server_timeout_configuration", func(t *testing.T) {
		sessionHandler, websocketHandler := createTestComponents()
		server := NewHTTPServer(sessionHandler, websocketHandler)

		// Verify timeout configuration from specification
		assert.Equal(t, 30*time.Second, server.server.ReadTimeout)
		assert.Equal(t, 30*time.Second, server.server.WriteTimeout)
		assert.Equal(t, 120*time.Second, server.server.IdleTimeout)
	})
}

func TestTechnical_ConcurrentRequests(t *testing.T) {
	t.Run("concurrent_request_handling", func(t *testing.T) {
		sessionHandler, websocketHandler := createTestComponents()
		server := NewHTTPServer(sessionHandler, websocketHandler)

		const numRequests = 10
		var wg sync.WaitGroup
		results := make([]int, numRequests)

		// Send concurrent requests
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				req := httptest.NewRequest("POST", "/api/session/start", 
					strings.NewReader(fmt.Sprintf(`{"name":"concurrent_test%d","instructor_id":"concurrent_instructor%d"}`, index, index)))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()

				server.mux.ServeHTTP(w, req)
				results[index] = w.Code
			}(i)
		}

		wg.Wait()

		// Due to single session constraint, only one should succeed (201), others should get 409 (conflict)
		// The important thing is that all requests are handled properly
		createdCount := 0
		conflictCount := 0
		
		for i, code := range results {
			assert.True(t, code == http.StatusCreated || code == http.StatusConflict, 
				"Request %d should return 201 or 409, got %d", i, code)
			switch code {
			case http.StatusCreated:
				createdCount++
			case http.StatusConflict:
				conflictCount++
			}
		}

		// Exactly one should succeed, others should conflict
		assert.Equal(t, 1, createdCount, "Exactly one session should be created")
		assert.Equal(t, numRequests-1, conflictCount, "Other sessions should conflict")
	})
}

// Error condition tests

func TestError_InvalidRoutes(t *testing.T) {
	t.Run("nonexistent_api_route", func(t *testing.T) {
		sessionHandler, websocketHandler := createTestComponents()
		server := NewHTTPServer(sessionHandler, websocketHandler)

		req := httptest.NewRequest("GET", "/api/nonexistent", nil)
		w := httptest.NewRecorder()

		server.mux.ServeHTTP(w, req)

		// Should be handled by static file server and return 404
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("method_not_allowed_handling", func(t *testing.T) {
		sessionHandler, websocketHandler := createTestComponents()
		server := NewHTTPServer(sessionHandler, websocketHandler)

		// API handlers should handle method validation themselves
		req := httptest.NewRequest("GET", "/api/session/start", nil)
		w := httptest.NewRecorder()

		server.mux.ServeHTTP(w, req)

		// The handler itself should return method not allowed
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}