package router

import (
	"testing"
	"time"

	"switchboard/pkg/types"
)

// TestValidateMessage_RolePermissions tests role-based message type validation
// This test focuses on the core validation logic without registry dependencies
func TestValidateMessage_RolePermissions(t *testing.T) {
	// Test just the permission checking logic directly
	router := &Router{}
	
	testCases := []struct {
		name        string
		senderRole  string
		messageType string
		expected    bool
	}{
		// Student valid permissions
		{"student instructor_inbox", "student", types.MessageTypeInstructorInbox, true},
		{"student request_response", "student", types.MessageTypeRequestResponse, true},
		{"student analytics", "student", types.MessageTypeAnalytics, true},
		
		// Student invalid permissions  
		{"student inbox_response", "student", types.MessageTypeInboxResponse, false},
		{"student request", "student", types.MessageTypeRequest, false},
		{"student instructor_broadcast", "student", types.MessageTypeInstructorBroadcast, false},
		
		// Instructor valid permissions
		{"instructor inbox_response", "instructor", types.MessageTypeInboxResponse, true},
		{"instructor request", "instructor", types.MessageTypeRequest, true},
		{"instructor instructor_broadcast", "instructor", types.MessageTypeInstructorBroadcast, true},
		
		// Instructor invalid permissions
		{"instructor instructor_inbox", "instructor", types.MessageTypeInstructorInbox, false},
		{"instructor request_response", "instructor", types.MessageTypeRequestResponse, false},
		{"instructor analytics", "instructor", types.MessageTypeAnalytics, false},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := router.canSendMessageType(tc.senderRole, tc.messageType)
			if result != tc.expected {
				t.Errorf("Expected %v for role %s sending %s, got %v", 
					tc.expected, tc.senderRole, tc.messageType, result)
			}
		})
	}
}

// TestRateLimiter_ExactLimits tests exact rate limiting behavior
func TestRateLimiter_ExactLimits(t *testing.T) {
	limiter := NewRateLimiter()
	userID := "user1"
	
	// Test first message
	if !limiter.Allow(userID) {
		t.Error("First message should always be allowed")
	}
	
	// Test exactly 99 more messages (total 100)
	for i := 1; i < 100; i++ {
		if !limiter.Allow(userID) {
			t.Errorf("Message %d should be allowed (within 100 limit)", i+1)
		}
	}
	
	// Test 101st message should be denied
	if limiter.Allow(userID) {
		t.Error("101st message should be denied")
	}
	
	// Test multiple denials
	for i := 0; i < 10; i++ {
		if limiter.Allow(userID) {
			t.Errorf("Message after limit should be denied (attempt %d)", i+1)
		}
	}
}

// TestRateLimiter_MultipleUsers tests rate limiting with multiple users
func TestRateLimiter_MultipleUsers(t *testing.T) {
	limiter := NewRateLimiter()
	
	// Each user gets their own limit
	users := []string{"user1", "user2", "user3"}
	
	for _, userID := range users {
		// Each user can send 100 messages
		for i := 0; i < 100; i++ {
			if !limiter.Allow(userID) {
				t.Errorf("Message %d for %s should be allowed", i+1, userID)
			}
		}
		
		// 101st message should be denied for each user
		if limiter.Allow(userID) {
			t.Errorf("101st message for %s should be denied", userID)
		}
	}
}

// TestIsValidMessageType tests message type validation
func TestIsValidMessageType(t *testing.T) {
	router := &Router{}
	
	validTypes := []string{
		types.MessageTypeInstructorInbox,
		types.MessageTypeInboxResponse,
		types.MessageTypeRequest,
		types.MessageTypeRequestResponse,
		types.MessageTypeAnalytics,
		types.MessageTypeInstructorBroadcast,
	}
	
	// Test all valid types
	for _, messageType := range validTypes {
		if !router.isValidMessageType(messageType) {
			t.Errorf("Message type %s should be valid", messageType)
		}
	}
	
	// Test invalid types
	invalidTypes := []string{
		"invalid_type",
		"",
		"message",
		"broadcast",
		"instructor",
		"student_message",
	}
	
	for _, messageType := range invalidTypes {
		if router.isValidMessageType(messageType) {
			t.Errorf("Message type %s should be invalid", messageType)
		}
	}
}

// TestCanSendMessageType tests role-based message type permissions
func TestCanSendMessageType(t *testing.T) {
	router := &Router{}
	
	testCases := []struct {
		role        string
		messageType string
		canSend     bool
	}{
		// Student permissions
		{"student", types.MessageTypeInstructorInbox, true},
		{"student", types.MessageTypeRequestResponse, true},
		{"student", types.MessageTypeAnalytics, true},
		{"student", types.MessageTypeInboxResponse, false},
		{"student", types.MessageTypeRequest, false},
		{"student", types.MessageTypeInstructorBroadcast, false},
		
		// Instructor permissions
		{"instructor", types.MessageTypeInboxResponse, true},
		{"instructor", types.MessageTypeRequest, true},
		{"instructor", types.MessageTypeInstructorBroadcast, true},
		{"instructor", types.MessageTypeInstructorInbox, false},
		{"instructor", types.MessageTypeRequestResponse, false},
		{"instructor", types.MessageTypeAnalytics, false},
		
		// Invalid role
		{"admin", types.MessageTypeInstructorInbox, false},
		{"", types.MessageTypeInstructorInbox, false},
		{"unknown", types.MessageTypeInstructorInbox, false},
	}
	
	for _, tc := range testCases {
		result := router.canSendMessageType(tc.role, tc.messageType)
		if result != tc.canSend {
			t.Errorf("Role %s should %s send %s", 
				tc.role, 
				map[bool]string{true: "be able to", false: "NOT be able to"}[tc.canSend],
				tc.messageType)
		}
	}
}

// TestRateLimiter_Cleanup tests cleanup functionality
func TestRateLimiter_Cleanup(t *testing.T) {
	limiter := NewRateLimiter()
	
	// Add some users
	limiter.Allow("user1")
	limiter.Allow("user2") 
	limiter.Allow("user3")
	
	// Check that users exist
	limiter.mu.RLock()
	initialCount := len(limiter.clients)
	limiter.mu.RUnlock()
	
	if initialCount != 3 {
		t.Errorf("Expected 3 clients before cleanup, got %d", initialCount)
	}
	
	// Run cleanup (won't remove recent entries)
	limiter.Cleanup()
	
	// Should still have all clients (they're recent)
	limiter.mu.RLock()
	afterCount := len(limiter.clients)
	limiter.mu.RUnlock()
	
	if afterCount != 3 {
		t.Errorf("Expected 3 clients after cleanup (recent entries), got %d", afterCount)
	}
	
	// Manually set old timestamps to test cleanup
	limiter.mu.Lock()
	oldTime := time.Now().Add(-10 * time.Minute) // 10 minutes ago
	for _, client := range limiter.clients {
		client.windowStart = oldTime
	}
	limiter.mu.Unlock()
	
	// Now cleanup should remove them
	limiter.Cleanup()
	
	limiter.mu.RLock()
	finalCount := len(limiter.clients)
	limiter.mu.RUnlock()
	
	if finalCount != 0 {
		t.Errorf("Expected 0 clients after cleanup (old entries), got %d", finalCount)
	}
}

// Test focusing on isolated router logic without external dependencies