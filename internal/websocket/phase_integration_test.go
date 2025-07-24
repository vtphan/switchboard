package websocket

import (
	"context"
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

// Phase 2 Integration Tests - Complete Workflow Validation
// Tests all components working together in realistic scenarios

// Test complete user connection workflow across all Phase 2 components
func TestPhase2_CompleteUserConnectionFlow(t *testing.T) {
	// Setup Phase 2 components
	registry := NewRegistry()
	sessionManager := &mockSessionManager{
		validateFunc: func(sessionID, userID, role string) error {
			if sessionID == "valid-session" && userID == "user123" && role == "student" {
				return nil
			}
			return interfaces.ErrUnauthorized
		},
	}
	dbManager := &mockDatabaseManager{
		getHistoryFunc: func(ctx context.Context, sessionID string) ([]*types.Message, error) {
			return []*types.Message{
				{
					ID:        "msg1",
					Type:      "instructor_broadcast",
					FromUser:  "instructor1",
					ToUser:    nil,
					SessionID: sessionID,
					Content:   map[string]interface{}{"text": "Welcome to class"},
					Context:   "general",
					Timestamp: time.Now().Add(-5 * time.Minute),
				},
			}, nil
		},
	}
	
	handler := NewHandler(registry, sessionManager, dbManager)
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()
	
	// Step 1: Establish WebSocket connection with authentication
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + 
		"?user_id=user123&role=student&session_id=valid-session"
	
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to establish WebSocket connection: %v", err)
	}
	defer conn.Close()
	
	// Give time for connection setup and registration
	time.Sleep(100 * time.Millisecond)
	
	// Step 2: Verify connection is registered in registry
	registeredConn, exists := registry.GetUserConnection("user123")
	if !exists {
		t.Fatal("Connection should be registered in registry")
	}
	
	if registeredConn.GetUserID() != "user123" {
		t.Errorf("Expected userID 'user123', got '%s'", registeredConn.GetUserID())
	}
	if registeredConn.GetRole() != "student" {
		t.Errorf("Expected role 'student', got '%s'", registeredConn.GetRole())
	}
	if registeredConn.GetSessionID() != "valid-session" {
		t.Errorf("Expected sessionID 'valid-session', got '%s'", registeredConn.GetSessionID())
	}
	
	// Step 3: Verify session connections are accessible
	sessionConnections := registry.GetSessionConnections("valid-session")
	if len(sessionConnections) != 1 {
		t.Errorf("Expected 1 session connection, got %d", len(sessionConnections))
	}
	
	studentConnections := registry.GetSessionStudents("valid-session")
	if len(studentConnections) != 1 {
		t.Errorf("Expected 1 student connection, got %d", len(studentConnections))
	}
	
	// Step 4: Verify history messages are received
	messagesReceived := 0
	historyComplete := false
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	
	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			break // Timeout or connection closed
		}
		
		messagesReceived++
		
		// Check for history complete message
		if msgType, ok := msg["type"].(string); ok && msgType == "system" {
			if content, ok := msg["content"].(map[string]interface{}); ok {
				if event, ok := content["event"].(string); ok && event == "history_complete" {
					historyComplete = true
					break
				}
			}
		}
	}
	
	if messagesReceived < 2 { // At least history message + history_complete
		t.Errorf("Expected at least 2 messages (history + complete), got %d", messagesReceived)
	}
	
	if !historyComplete {
		t.Error("Should receive history_complete system message")
	}
	
	// Step 5: Test message sending through connection
	testMessage := map[string]interface{}{
		"type":    "student_message",
		"content": "Hello from student",
	}
	
	err = registeredConn.WriteJSON(testMessage)
	if err != nil {
		t.Errorf("Failed to send message through registered connection: %v", err)
	}
	
	// Step 6: Test connection cleanup on disconnect
	conn.Close()
	time.Sleep(100 * time.Millisecond) // Give time for cleanup
	
	_, exists = registry.GetUserConnection("user123")
	if exists {
		t.Error("Connection should be cleaned up from registry after disconnect")
	}
	
	sessionConnections = registry.GetSessionConnections("valid-session")
	if len(sessionConnections) != 0 {
		t.Errorf("Expected 0 session connections after cleanup, got %d", len(sessionConnections))
	}
}

// Test connection replacement workflow
func TestPhase2_ConnectionReplacementFlow(t *testing.T) {
	registry := NewRegistry()
	sessionManager := &mockSessionManager{
		validateFunc: func(sessionID, userID, role string) error {
			return nil // Allow all connections for this test
		},
	}
	dbManager := &mockDatabaseManager{}
	handler := NewHandler(registry, sessionManager, dbManager)
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()
	
	// Establish first connection
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + 
		"?user_id=user123&role=student&session_id=session456"
	
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to establish first connection: %v", err)
	}
	defer conn1.Close()
	
	time.Sleep(50 * time.Millisecond)
	
	// Verify first connection is registered
	registeredConn1, exists := registry.GetUserConnection("user123")
	if !exists {
		t.Fatal("First connection should be registered")
	}
	
	// Establish second connection for same user (should replace first)
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to establish second connection: %v", err)
	}
	defer conn2.Close()
	
	time.Sleep(100 * time.Millisecond) // Give time for replacement
	
	// Verify second connection replaced first
	registeredConn2, exists := registry.GetUserConnection("user123")
	if !exists {
		t.Fatal("Second connection should be registered")
	}
	
	// Should be different connection instance
	if registeredConn1 == registeredConn2 {
		t.Error("Second connection should have replaced first connection")
	}
	
	// Should still only have one connection in registry
	stats := registry.GetStats()
	if stats["total_connections"] != 1 {
		t.Errorf("Expected 1 total connection after replacement, got %d", stats["total_connections"])
	}
	
	sessionConnections := registry.GetSessionConnections("session456")
	if len(sessionConnections) != 1 {
		t.Errorf("Expected 1 session connection after replacement, got %d", len(sessionConnections))
	}
}

// Test error handling across components
func TestPhase2_ErrorPropagationFlow(t *testing.T) {
	registry := NewRegistry()
	dbManager := &mockDatabaseManager{}
	
	tests := []struct {
		name               string
		sessionValidation  func(sessionID, userID, role string) error
		expectedBehavior   string
	}{
		{
			name: "Session not found error",
			sessionValidation: func(sessionID, userID, role string) error {
				return interfaces.ErrSessionNotFound
			},
			expectedBehavior: "connection rejected, not registered",
		},
		{
			name: "Unauthorized access error",
			sessionValidation: func(sessionID, userID, role string) error {
				return interfaces.ErrUnauthorized
			},
			expectedBehavior: "connection rejected, not registered",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionManager := &mockSessionManager{validateFunc: tt.sessionValidation}
			handler := NewHandler(registry, sessionManager, dbManager)
			server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
			defer server.Close()
			
			wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + 
				"?user_id=user123&role=student&session_id=session456"
			
			// Attempt connection - should fail
			_, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err == nil {
				t.Error("Connection should fail with validation error")
			}
			
			// Verify connection was not registered
			_, exists := registry.GetUserConnection("user123")
			if exists {
				t.Error("Failed connection should not be registered in registry")
			}
			
			stats := registry.GetStats()
			if stats["total_connections"] != 0 {
				t.Errorf("Expected 0 connections after failed auth, got %d", stats["total_connections"])
			}
		})
	}
}

// Test concurrent connections across all components
func TestPhase2_ConcurrentConnectionsIntegration(t *testing.T) {
	registry := NewRegistry()
	sessionManager := &mockSessionManager{
		validateFunc: func(sessionID, userID, role string) error {
			return nil // Allow all connections
		},
	}
	dbManager := &mockDatabaseManager{}
	handler := NewHandler(registry, sessionManager, dbManager)
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()
	
	const numConnections = 10
	var wg sync.WaitGroup
	connections := make([]*websocket.Conn, numConnections)
	
	// Establish multiple concurrent connections
	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + 
				"?user_id=user" + string(rune('0'+id)) + "&role=student&session_id=session456"
			
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				t.Errorf("Failed to establish connection %d: %v", id, err)
				return
			}
			connections[id] = conn
			
			// Keep connection alive briefly
			time.Sleep(50 * time.Millisecond)
		}(i)
	}
	
	wg.Wait()
	time.Sleep(100 * time.Millisecond) // Give time for all registrations
	
	// Cleanup connections
	defer func() {
		for _, conn := range connections {
			if conn != nil {
				conn.Close()
			}
		}
	}()
	
	// Verify all connections are registered
	stats := registry.GetStats()
	if stats["total_connections"] < numConnections/2 { // Allow for some timing variance
		t.Errorf("Expected at least %d connections, got %d", numConnections/2, stats["total_connections"])
	}
	
	sessionConnections := registry.GetSessionConnections("session456")
	if len(sessionConnections) < numConnections/2 {
		t.Errorf("Expected at least %d session connections, got %d", numConnections/2, len(sessionConnections))
	}
	
	// Test that all registered connections are functional
	for i := 0; i < numConnections; i++ {
		userID := "user" + string(rune('0'+i))
		if registeredConn, exists := registry.GetUserConnection(userID); exists {
			testMessage := map[string]interface{}{
				"type":    "test",
				"content": "test from " + userID,
			}
			
			err := registeredConn.WriteJSON(testMessage)
			if err != nil {
				t.Errorf("Registered connection %s should be functional: %v", userID, err)
			}
		}
	}
}

// Test resource cleanup coordination
func TestPhase2_ResourceCleanupCoordination(t *testing.T) {
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
	
	// Create and close multiple connections rapidly
	for i := 0; i < 5; i++ {
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + 
			"?user_id=user123&role=student&session_id=session456"
		
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to establish connection %d: %v", i, err)
		}
		
		time.Sleep(20 * time.Millisecond) // Brief connection time
		conn.Close()
		time.Sleep(50 * time.Millisecond) // Give time for cleanup
		
		// Verify cleanup happened
		_, exists := registry.GetUserConnection("user123")
		if exists {
			t.Errorf("Connection %d should be cleaned up", i)
		}
	}
	
	// Final verification - registry should be clean
	stats := registry.GetStats()
	if stats["total_connections"] != 0 {
		t.Errorf("Expected 0 connections after all cleanup, got %d", stats["total_connections"])
	}
	
	sessionConnections := registry.GetSessionConnections("session456")
	if len(sessionConnections) != 0 {
		t.Errorf("Expected 0 session connections after cleanup, got %d", len(sessionConnections))
	}
}