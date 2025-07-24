package router

import (
	"context"
	"testing"

	"switchboard/pkg/types"
	"switchboard/internal/websocket"
)


// TestRouter_MessageProcessing tests core message processing without registry dependencies
func TestRouter_MessageProcessing(t *testing.T) {
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil) // No database manager for this test
	
	// Test message ID generation and timestamp setting
	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeInstructorInbox,
		FromUser:  "student1",
		Context:   "",  // Should default to "general"
		Content:   map[string]interface{}{"text": "Help needed"},
	}
	
	// This will fail due to sender not connected, but we can check processing
	_ = router.RouteMessage(context.Background(), message)
	
	// Verify message was processed (ID and timestamp set)
	if message.ID == "" {
		t.Error("Expected server-generated message ID")
	}
	if message.Timestamp.IsZero() {
		t.Error("Expected server-generated timestamp")
	}
	if message.Context != "general" {
		t.Errorf("Expected context 'general', got '%s'", message.Context)
	}
}

// TestRouter_EmptyRegistry tests router behavior with empty registry
func TestRouter_EmptyRegistry(t *testing.T) {
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)
	
	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeInstructorInbox,
		FromUser:  "student1",
		Context:   "general",
		Content:   map[string]interface{}{"text": "test"},
	}
	
	// Should fail because sender is not connected
	err := router.RouteMessage(context.Background(), message)
	if err != ErrSenderNotConnected {
		t.Errorf("Expected ErrSenderNotConnected for empty registry, got %v", err)
	}
}

// TestRateLimiter_ConcurrentAccess tests rate limiter under concurrent access
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewRateLimiter()
	
	// Test that concurrent access doesn't cause panics or corruption
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			userID := "user" + string(rune('0'+id))
			// Each goroutine tests a different user to avoid contention
			limiter.Allow(userID)
			limiter.Allow(userID)
			done <- true
		}(i)
	}
	
	// Wait for completion
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// If we reach here without panic, concurrent access is safe
}

// TestRouter_GetRecipients_EmptyRegistry tests recipient calculation with empty registry
func TestRouter_GetRecipients_EmptyRegistry(t *testing.T) {
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)
	
	testCases := []struct {
		name        string
		messageType string
	}{
		{"instructor_inbox", types.MessageTypeInstructorInbox},
		{"request_response", types.MessageTypeRequestResponse},
		{"analytics", types.MessageTypeAnalytics},
		{"instructor_broadcast", types.MessageTypeInstructorBroadcast},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			message := &types.Message{
				SessionID: "session1",
				Type:      tc.messageType,
				FromUser:  "test_user",
				Context:   "general",
			}
			
			recipients, err := router.GetRecipients(message)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			// Empty registry should return empty recipient list
			if len(recipients) != 0 {
				t.Errorf("Expected 0 recipients for empty registry, got %d", len(recipients))
			}
		})
	}
}

// TestRouter_DirectMessage tests direct message validation logic
func TestRouter_DirectMessage(t *testing.T) {
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)
	
	// Test missing recipient
	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeInboxResponse,
		FromUser:  "instructor1",
		ToUser:    nil, // Missing recipient
		Context:   "general",
		Content:   map[string]interface{}{"text": "Response"},
	}
	
	_, err := router.GetRecipients(message)
	if err != ErrMissingRecipient {
		t.Errorf("Expected ErrMissingRecipient for message without ToUser, got %v", err)
	}
	
	// Test non-existent recipient
	toUser := "nonexistent_user"
	message.ToUser = &toUser
	
	_, err = router.GetRecipients(message)
	if err != ErrRecipientNotFound {
		t.Errorf("Expected ErrRecipientNotFound for non-existent user, got %v", err)
	}
}

// TestRateLimiter_CleanupSimple tests cleanup functionality without time delays
func TestRateLimiter_CleanupSimple(t *testing.T) {
	limiter := NewRateLimiter()
	limiter.Allow("user1")
	limiter.Allow("user2")
	
	// Manually trigger cleanup
	limiter.Cleanup()
	
	// Should still work after cleanup
	if !limiter.Allow("user3") {
		t.Error("Rate limiter should work after cleanup")
	}
}