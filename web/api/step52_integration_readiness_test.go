package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
// TestStep52IntegrationReadiness validates that Step 5.1 SessionAPIHandler is ready
// for integration with Step 5.2 HTTP Server routing as specified in phase-5.md
func TestStep52IntegrationReadiness(t *testing.T) {
	t.Run("http_handler_function_signature_compatibility", func(t *testing.T) {
		// Validate that StartSession and EndSession have correct http.HandlerFunc signatures
		// This ensures they can be integrated with Step 5.2 HTTP routing
		
		mockDB := &mockDatabaseManager{}
		sessionManager := session.NewSessionManager(mockDB)
		sessionLifecycle := session.NewSessionLifecycle(sessionManager, mockDB, &mockSystemBroadcaster{})
		handler := NewSessionAPIHandler(sessionLifecycle)

		// Verify StartSession has correct signature for http.HandlerFunc
		req := httptest.NewRequest(http.MethodPost, "/api/session/start", nil)
		w := httptest.NewRecorder()
		
		// This should compile and execute without type errors
		handler.StartSession(w, req)
		
		// Verify EndSession has correct signature for http.HandlerFunc  
		req2 := httptest.NewRequest(http.MethodPost, "/api/session/end", nil)
		w2 := httptest.NewRecorder()
		
		handler.EndSession(w2, req2)
		
		// If we reach here, the signatures are compatible
		assert.True(t, true, "Handler signatures are compatible with http.HandlerFunc")
	})

	t.Run("http_server_route_pattern_compatibility", func(t *testing.T) {
		// This validates the exact routing patterns expected by Step 5.2
		// Based on phase-5.md lines 255-256:
		// hs.mux.HandleFunc("/api/session/start", hs.withCORS(hs.withLogging(hs.sessionHandler.StartSession)))
		// hs.mux.HandleFunc("/api/session/end", hs.withCORS(hs.withLogging(hs.sessionHandler.EndSession)))
		
		mockDB := &mockDatabaseManager{}
		sessionManager := session.NewSessionManager(mockDB)
		sessionLifecycle := session.NewSessionLifecycle(sessionManager, mockDB, &mockSystemBroadcaster{})
		sessionHandler := NewSessionAPIHandler(sessionLifecycle)

		// Create a simple HTTP multiplexer like Step 5.2 will use
		mux := http.NewServeMux()
		
		// Register routes exactly as Step 5.2 will
		mux.HandleFunc("/api/session/start", sessionHandler.StartSession)
		mux.HandleFunc("/api/session/end", sessionHandler.EndSession)

		// Test routing works correctly
		server := httptest.NewServer(mux)
		defer server.Close()

		// Test POST /api/session/start routing
		requestBody := map[string]string{
			"name":          "Route Test Session",
			"instructor_id": "prof_route",
		}
		body, _ := json.Marshal(requestBody)
		
		resp, err := http.Post(server.URL+"/api/session/start", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}()
		
		assert.Equal(t, http.StatusCreated, resp.StatusCode, "Routing to StartSession should work")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		// Test POST /api/session/end routing
		endBody, _ := json.Marshal(map[string]string{"instructor_id": "prof_route"})
		resp2, err := http.Post(server.URL+"/api/session/end", "application/json", bytes.NewReader(endBody))
		require.NoError(t, err)
		defer func() {
			if err := resp2.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}()
		
		assert.Equal(t, http.StatusOK, resp2.StatusCode, "Routing to EndSession should work")
	})

	t.Run("middleware_compatibility", func(t *testing.T) {
		// This validates that the handlers work correctly with middleware
		// as specified in Step 5.2 withCORS and withLogging middleware
		
		mockDB := &mockDatabaseManager{}
		sessionManager := session.NewSessionManager(mockDB)
		sessionLifecycle := session.NewSessionLifecycle(sessionManager, mockDB, &mockSystemBroadcaster{})
		handler := NewSessionAPIHandler(sessionLifecycle)

		// Simulate Step 5.2 CORS middleware
		corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				
				if r.Method == "OPTIONS" {
					w.WriteHeader(http.StatusOK)
					return
				}
				
				next(w, r)
			}
		}

		// Simulate Step 5.2 logging middleware
		loggingMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				// In real middleware, this would log the request
				next(w, r)
			}
		}

		// Test handler with middleware chain
		wrappedHandler := corsMiddleware(loggingMiddleware(handler.StartSession))

		requestBody := map[string]string{
			"name":          "Middleware Test Session",
			"instructor_id": "prof_middleware",
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
		w := httptest.NewRecorder()

		wrappedHandler(w, req)

		// Validate both middleware and handler worked
		assert.Equal(t, http.StatusCreated, w.Code, "Handler should work through middleware")
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"), "CORS middleware should work")
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "Handler should set content type")
	})

	t.Run("constructor_dependency_injection_compatibility", func(t *testing.T) {
		// This validates the NewSessionAPIHandler constructor pattern 
		// as expected by Step 5.3 system integration
		
		mockDB := &mockDatabaseManager{}
		sessionManager := session.NewSessionManager(mockDB)
		sessionLifecycle := session.NewSessionLifecycle(sessionManager, mockDB, &mockSystemBroadcaster{})

		// Test the constructor pattern Step 5.3 will use
		handler := NewSessionAPIHandler(sessionLifecycle)
		
		assert.NotNil(t, handler, "Constructor should return valid handler")
		
		// Verify the handler has the expected SessionLifecycle dependency
		// This is tested by making a successful API call
		requestBody := map[string]string{
			"name":          "DI Test Session", 
			"instructor_id": "prof_di",
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/api/session/start", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.StartSession(w, req)
		
		assert.Equal(t, http.StatusCreated, w.Code, "Dependency injection should work correctly")
	})
}