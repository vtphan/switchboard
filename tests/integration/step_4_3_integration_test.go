package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"switchboard/internal/database"
	"switchboard/internal/message"
	"switchboard/internal/rate"
	"switchboard/internal/session"
	websocketPkg "switchboard/internal/websocket"
	"switchboard/pkg/errors"
)

// Mock broadcast system for testing
type mockBroadcastSystem struct {
	messages []*database.Message
}

func (m *mockBroadcastSystem) BroadcastMessage(message *database.Message, recipients []message.Recipient) error {
	m.messages = append(m.messages, message)
	return nil
}

// TestStep43_WebSocketHandler_Integration tests the complete Step 4.3 integration
// This provides comprehensive coverage of all WebSocket handler paths
func TestStep43_WebSocketHandler_Integration(t *testing.T) {
	t.Run("complete_websocket_connection_flow", func(t *testing.T) {
		// Setup complete dependency chain
		dbManager := &mockDatabaseManager{}
		sessionManager := session.NewSessionManager(dbManager)
		rateLimiter := rate.NewRateLimiter()
		router := &mockMessageRouter{}
		broadcastSystem := &mockBroadcastSystem{}
		messageProcessor := message.NewMessageProcessor(sessionManager, dbManager, rateLimiter, router, broadcastSystem)
		
		registry := websocketPkg.NewConnectionRegistry(sessionManager)
		registry.Start()
		defer func() {
			registry.Stop()
		}()
		
		handler := websocketPkg.NewWebSocketHandler(registry, sessionManager, messageProcessor, dbManager)
		
		// Create test HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler.HandleWebSocketUpgrade(w, r)
			if err != nil {
				t.Logf("WebSocket upgrade error: %v", err)
			}
		}))
		defer server.Close()
		
		// Convert HTTP URL to WebSocket URL
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?user_id=test_student&role=student"
		
		// Test connection establishment
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer func() { _ = conn.Close() }()
		
		// Should receive waiting message since no active session
		var waitingMsg map[string]interface{}
		err = conn.ReadJSON(&waitingMsg)
		if err != nil {
			t.Fatalf("Failed to read waiting message: %v", err)
		}
		
		// Verify waiting message structure
		if waitingMsg["type"] != "system" {
			t.Errorf("Expected system message, got %v", waitingMsg["type"])
		}
		
		content, ok := waitingMsg["content"].(map[string]interface{})
		if !ok {
			t.Fatal("Invalid content structure")
		}
		
		if content["event"] != "waiting_for_session" {
			t.Errorf("Expected waiting_for_session event, got %v", content["event"])
		}
		
		// Verify connection is registered
		time.Sleep(50 * time.Millisecond) // Allow connection setup
		recipient, err := registry.GetUserByID("test_student")
		if err != nil {
			t.Fatalf("Connection not registered: %v", err)
		}
		
		if recipient.GetRole() != "student" {
			t.Errorf("Expected student role, got %s", recipient.GetRole())
		}
		
		t.Log("✅ Complete WebSocket connection flow successful")
	})
	
	t.Run("websocket_with_active_session", func(t *testing.T) {
		// Setup with active session
		dbManager := &mockDatabaseManager{}
		sessionManager := session.NewSessionManager(dbManager)
		rateLimiter := rate.NewRateLimiter()
		router := &mockMessageRouter{}
		broadcastSystem := &mockBroadcastSystem{}
		messageProcessor := message.NewMessageProcessor(sessionManager, dbManager, rateLimiter, router, broadcastSystem)
		
		// Create active session
		activeSession, _ := database.NewSession("test-session", "Test Session", "instructor1")
		err := sessionManager.SetActiveSession(activeSession)
		if err != nil {
			t.Fatalf("Failed to set active session: %v", err)
		}
		
		registry := websocketPkg.NewConnectionRegistry(sessionManager)
		registry.Start()
		defer func() {
			registry.Stop()
		}()
		
		handler := websocketPkg.NewWebSocketHandler(registry, sessionManager, messageProcessor, dbManager)
		
		// Create test HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler.HandleWebSocketUpgrade(w, r)
			if err != nil {
				t.Logf("WebSocket upgrade error: %v", err)
			}
		}))
		defer server.Close()
		
		// Connect as instructor
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?user_id=instructor1&role=instructor"
		
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer func() { _ = conn.Close() }()
		
		// Should receive session active message
		var sessionMsg map[string]interface{}
		err = conn.ReadJSON(&sessionMsg)
		if err != nil {
			t.Fatalf("Failed to read session message: %v", err)
		}
		
		// Verify session active message
		content, ok := sessionMsg["content"].(map[string]interface{})
		if !ok {
			t.Fatal("Invalid content structure")
		}
		
		if content["event"] != "session_active" {
			t.Errorf("Expected session_active event, got %v", content["event"])
		}
		
		if content["session_id"] != "test-session" {
			t.Errorf("Expected session ID test-session, got %v", content["session_id"])
		}
		
		// Should also receive history message
		var historyMsg map[string]interface{}
		err = conn.ReadJSON(&historyMsg)
		if err != nil {
			t.Fatalf("Failed to read history message: %v", err)
		}
		
		historyContent, ok := historyMsg["content"].(map[string]interface{})
		if !ok {
			t.Fatal("Invalid history content structure")
		}
		
		if historyContent["event"] != "history_delivered" {
			t.Errorf("Expected history_delivered event, got %v", historyContent["event"])
		}
		
		t.Log("✅ WebSocket with active session flow successful")
	})
	
	t.Run("websocket_message_processing", func(t *testing.T) {
		// Setup complete dependency chain
		dbManager := &mockDatabaseManager{}
		sessionManager := session.NewSessionManager(dbManager)
		rateLimiter := rate.NewRateLimiter()
		router := &mockMessageRouter{}
		broadcastSystem := &mockBroadcastSystem{}
		messageProcessor := message.NewMessageProcessor(sessionManager, dbManager, rateLimiter, router, broadcastSystem)
		
		// Create active session for message processing
		activeSession, _ := database.NewSession("test-session", "Test Session", "instructor1")
		err := sessionManager.SetActiveSession(activeSession)
		if err != nil {
			t.Fatalf("Failed to set active session: %v", err)
		}
		
		registry := websocketPkg.NewConnectionRegistry(sessionManager)
		registry.Start()
		defer func() {
			registry.Stop()
		}()
		
		handler := websocketPkg.NewWebSocketHandler(registry, sessionManager, messageProcessor, dbManager)
		
		// Create test HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler.HandleWebSocketUpgrade(w, r)
			if err != nil {
				t.Logf("WebSocket upgrade error: %v", err)
			}
		}))
		defer server.Close()
		
		// Connect as student
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?user_id=student1&role=student"
		
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer func() { _ = conn.Close() }()
		
		// Read initial messages (session state + history)
		var initialMsg map[string]interface{}
		_ = conn.ReadJSON(&initialMsg) // session_active
		_ = conn.ReadJSON(&initialMsg) // history_delivered
		
		// Send a test message
		testMessage := map[string]interface{}{
			"type":    "broadcast_to_instructors",
			"context": "question", 
			"content": map[string]interface{}{
				"text": "I have a question about the assignment",
			},
		}
		
		err = conn.WriteJSON(testMessage)
		if err != nil {
			t.Fatalf("Failed to send message: %v", err)
		}
		
		// Give time for message processing
		time.Sleep(100 * time.Millisecond)
		
		// Verify message was processed (stored in mock database)
		if len(dbManager.writtenMessages) == 0 {
			t.Error("Message was not processed and stored")
		} else {
			storedMsg := dbManager.writtenMessages[0]
			if storedMsg.Type != "broadcast_to_instructors" {
				t.Errorf("Expected broadcast_to_instructors, got %s", storedMsg.Type)
			}
			if storedMsg.FromUser != "student1" {
				t.Errorf("Expected student1, got %s", storedMsg.FromUser)
			}
		}
		
		t.Log("✅ WebSocket message processing flow successful")
	})
	
	t.Run("websocket_connection_cleanup", func(t *testing.T) {
		// Setup
		dbManager := &mockDatabaseManager{}
		sessionManager := session.NewSessionManager(dbManager)
		rateLimiter := rate.NewRateLimiter()
		router := &mockMessageRouter{}
		broadcastSystem := &mockBroadcastSystem{}
		messageProcessor := message.NewMessageProcessor(sessionManager, dbManager, rateLimiter, router, broadcastSystem)
		
		registry := websocketPkg.NewConnectionRegistry(sessionManager)
		registry.Start()
		defer func() {
			registry.Stop()
		}()
		
		handler := websocketPkg.NewWebSocketHandler(registry, sessionManager, messageProcessor, dbManager)
		
		// Create test HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler.HandleWebSocketUpgrade(w, r)
			if err != nil {
				t.Logf("WebSocket upgrade error: %v", err)
			}
		}))
		defer server.Close()
		
		// Connect and immediately disconnect
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?user_id=cleanup_test&role=student"
		
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		
		// Verify connection is registered
		time.Sleep(50 * time.Millisecond)
		_, err = registry.GetUserByID("cleanup_test")
		if err != nil {
			t.Fatalf("Connection not registered: %v", err)
		}
		
		// Close connection
		_ = conn.Close()
		
		// Wait for cleanup to happen
		time.Sleep(200 * time.Millisecond)
		
		// Verify connection is cleaned up
		_, err = registry.GetUserByID("cleanup_test")
		if err != errors.ErrConnectionNotFound {
			t.Errorf("Expected connection to be cleaned up, got error: %v", err)
		}
		
		t.Log("✅ WebSocket connection cleanup successful")
	})
}

// Mock implementations for integration testing

type mockDatabaseManager struct {
	mu             sync.Mutex
	writtenMessages []*database.Message
}

func (mdm *mockDatabaseManager) CreateSession(session *database.Session) error {
	return nil
}

func (mdm *mockDatabaseManager) UpdateSession(session *database.Session) error {
	return nil
}

func (mdm *mockDatabaseManager) GetActiveSession() (*database.Session, error) {
	return nil, nil // Not used in these tests
}

func (mdm *mockDatabaseManager) WriteMessage(msg *database.Message) error {
	mdm.mu.Lock()
	defer mdm.mu.Unlock()
	mdm.writtenMessages = append(mdm.writtenMessages, msg)
	return nil
}

func (mdm *mockDatabaseManager) WriteBatch(msgs []*database.Message) error {
	mdm.mu.Lock()
	defer mdm.mu.Unlock()
	mdm.writtenMessages = append(mdm.writtenMessages, msgs...)
	return nil
}

func (mdm *mockDatabaseManager) GetSessionMessages(sessionID string) ([]*database.Message, error) {
	return []*database.Message{}, nil
}

func (mdm *mockDatabaseManager) Start() error {
	return nil
}

func (mdm *mockDatabaseManager) Stop() error {
	return nil
}

func (mdm *mockDatabaseManager) WaitForPendingWrites() error {
	return nil
}

type mockMessageRouter struct{}

func (mmr *mockMessageRouter) GetRecipients(msg *database.Message) ([]message.Recipient, error) {
	// Return empty recipients for testing
	return []message.Recipient{}, nil
}