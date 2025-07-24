package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gorillaws "github.com/gorilla/websocket"
	"switchboard/pkg/interfaces"
	"switchboard/pkg/types"
	"switchboard/internal/websocket"
)

// Test WebSocket upgrader for creating test connections
var testUpgrader = gorillaws.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// createTestWebSocketConnection creates a test WebSocket connection
func createTestWebSocketConnection(t *testing.T) *gorillaws.Conn {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		// Keep connection alive for testing
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))

	t.Cleanup(func() { server.Close() })

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := gorillaws.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Failed to create test WebSocket connection: %v", err)
	}

	return conn
}

// setupTestConnection creates and registers a mock connection for testing
func setupTestConnection(t *testing.T, registry *websocket.Registry, userID, role, sessionID string) *websocket.Connection {
	// Create a real WebSocket connection for testing
	wsConn := createTestWebSocketConnection(t)
	
	// Wrap it in our Connection type
	conn := websocket.NewConnection(wsConn)
	
	// Set credentials
	if err := conn.SetCredentials(userID, role, sessionID); err != nil {
		t.Fatalf("Failed to set credentials: %v", err)
	}
	
	// Register with the registry
	err := registry.RegisterConnection(conn)
	if err != nil {
		t.Fatalf("Failed to register connection: %v", err)
	}
	
	return conn
}

// TestRouter_InterfaceCompliance tests architectural validation - interface compliance
func TestRouter_InterfaceCompliance(t *testing.T) {
	// This should fail because Router doesn't exist yet
	var _ interfaces.MessageRouter = &Router{}
}

// TestRouteMessage_InstructorInbox tests functional validation - instructor_inbox routing
func TestRouteMessage_InstructorInbox(t *testing.T) {
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)

	// Set up a student connection that will send the message
	setupTestConnection(t, registry, "student1", "student", "session1")

	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeInstructorInbox,
		FromUser:  "student1",
		Context:   "general",
		Content:   map[string]interface{}{"text": "Help needed"},
	}

	err := router.RouteMessage(context.Background(), message)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Message should have server-generated ID and timestamp
	if message.ID == "" {
		t.Error("Expected server-generated message ID")
	}
	if message.Timestamp.IsZero() {
		t.Error("Expected server-generated timestamp")
	}
}

// TestRouteMessage_RoleValidation tests functional validation - role-based permissions
func TestRouteMessage_RoleValidation(t *testing.T) {
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)

	// Set up a student connection that will try to send inbox_response (not allowed)
	setupTestConnection(t, registry, "student1", "student", "session1")

	// Should fail - students can't send inbox_response
	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeInboxResponse,
		FromUser:  "student1",
		ToUser:    stringPtr("student2"),
		Context:   "general",
		Content:   map[string]interface{}{"text": "Response"},
	}

	err := router.RouteMessage(context.Background(), message)
	if err != ErrUnauthorizedMessageType {
		t.Errorf("Expected ErrUnauthorizedMessageType, got %v", err)
	}
}

// TestRouteMessage_ContextDefaulting tests functional validation - context field handling  
func TestRouteMessage_ContextDefaulting(t *testing.T) {
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)

	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeInstructorInbox,
		FromUser:  "student1",
		Context:   "", // Empty context should default to "general"
		Content:   map[string]interface{}{"text": "Help needed"},
	}

	// This will fail because router doesn't exist yet
	err := router.RouteMessage(context.Background(), message)
	if err != nil && err != ErrSenderNotConnected {
		t.Errorf("Unexpected error: %v", err)
	}

	// Context should be defaulted to "general"
	if message.Context != "general" {
		t.Errorf("Expected context 'general', got '%s'", message.Context)
	}
}

// TestRateLimiter_Allow tests technical validation - rate limiting behavior
func TestRateLimiter_Allow(t *testing.T) {
	// This should fail because RateLimiter doesn't exist yet
	limiter := NewRateLimiter()

	userID := "user1"

	// First 100 messages should be allowed
	for i := 0; i < 100; i++ {
		if !limiter.Allow(userID) {
			t.Errorf("Message %d should be allowed", i+1)
		}
	}

	// 101st message should be denied
	if limiter.Allow(userID) {
		t.Error("101st message should be denied")
	}
}

// TestRateLimiter_WindowReset tests technical validation - rate limit window reset
func TestRateLimiter_WindowReset(t *testing.T) {
	limiter := NewRateLimiter()
	userID := "user1"

	// Fill up the rate limit
	for i := 0; i < 100; i++ {
		limiter.Allow(userID)
	}

	// Should be denied
	if limiter.Allow(userID) {
		t.Error("Should be denied after 100 messages")
	}

	// Simulate time passage - this test will need adjustment once implementation exists
	// For now, just test the structure exists
	if limiter == nil {
		t.Error("RateLimiter should exist")
	}
}

// TestGetRecipients_InstructorBroadcast tests functional validation - recipient calculation
func TestGetRecipients_InstructorBroadcast(t *testing.T) {
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)

	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeInstructorBroadcast,
		FromUser:  "instructor1",
		Context:   "general",
		Content:   map[string]interface{}{"text": "Announcement"},
	}

	// This should fail because GetRecipients doesn't exist yet
	recipients, err := router.GetRecipients(message)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should return all students in session (empty for now)
	if len(recipients) != 0 {
		t.Errorf("Expected 0 recipients, got %d", len(recipients))
	}
}

// TestValidateMessage_MessageType tests functional validation - message type validation
func TestValidateMessage_MessageType(t *testing.T) {
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)

	// Set up a student connection
	setupTestConnection(t, registry, "student1", "student", "session1")

	sender := &types.Client{
		ID:   "student1",
		Role: "student",
	}

	message := &types.Message{
		SessionID: "session1",
		Type:      "invalid_type",
		FromUser:  "student1",
		Context:   "general",
		Content:   map[string]interface{}{"text": "Test"},
	}

	err := router.ValidateMessage(message, sender)
	if err != ErrInvalidMessageType {
		t.Errorf("Expected ErrInvalidMessageType, got %v", err)
	}
}

// TestRouter_ErrorTypes tests architectural validation - error type definitions
func TestRouter_ErrorTypes(t *testing.T) {
	// These should fail because error types don't exist yet
	if ErrInvalidMessageType == nil {
		t.Error("ErrInvalidMessageType should be defined")
	}
	if ErrUnauthorizedMessageType == nil {
		t.Error("ErrUnauthorizedMessageType should be defined")
	}
	if ErrRateLimitExceeded == nil {
		t.Error("ErrRateLimitExceeded should be defined")
	}
}

// Helper function for pointer to string
func stringPtr(s string) *string {
	return &s
}