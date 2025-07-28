package router

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRateLimiter_NewRateLimiter tests rate limiter creation
func TestRateLimiter_NewRateLimiter(t *testing.T) {
	rl := NewRateLimiter()
	require.NotNil(t, rl)
	assert.NotNil(t, rl.clients)
	assert.NotNil(t, rl.stopCleanup)
	
	// Clean shutdown
	rl.Stop()
}

func TestRateLimiter_NewRateLimiterWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	rl := NewRateLimiterWithContext(ctx)
	require.NotNil(t, rl)
	assert.NotNil(t, rl.clients)
	assert.NotNil(t, rl.stopCleanup)
	
	// Clean shutdown
	rl.Stop()
}

// TestRateLimiter_FirstMessageAlwaysAllowed tests first message behavior
func TestRateLimiter_FirstMessageAlwaysAllowed(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	
	// First message should always be allowed for any user
	assert.True(t, rl.Allow("user1"))
	assert.True(t, rl.Allow("user2"))
	assert.True(t, rl.Allow("user3"))
	
	// Verify client entries were created
	rl.mu.Lock()
	assert.Len(t, rl.clients, 3)
	assert.Contains(t, rl.clients, "user1")
	assert.Contains(t, rl.clients, "user2")
	assert.Contains(t, rl.clients, "user3")
	rl.mu.Unlock()
}

// TestRateLimiter_AllowWithinLimit tests normal operation within limits
func TestRateLimiter_AllowWithinLimit(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	
	userID := "test-user"
	
	// Send 50 messages - all should be allowed
	for i := 0; i < 50; i++ {
		assert.True(t, rl.Allow(userID), "Message %d should be allowed", i+1)
	}
	
	// Verify client state
	rl.mu.Lock()
	limit, exists := rl.clients[userID]
	assert.True(t, exists)
	assert.Equal(t, 50, limit.messageCount)
	assert.WithinDuration(t, time.Now(), limit.windowStart, time.Second)
	rl.mu.Unlock()
}

// TestRateLimiter_ExceedLimit tests rate limit enforcement
func TestRateLimiter_ExceedLimit(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	
	userID := "heavy-user"
	
	// Send exactly 100 messages - all should be allowed
	for i := 0; i < 100; i++ {
		assert.True(t, rl.Allow(userID), "Message %d should be allowed", i+1)
	}
	
	// 101st message should be rejected
	assert.False(t, rl.Allow(userID), "Message 101 should be rejected")
	
	// Multiple additional attempts should also be rejected
	for i := 0; i < 10; i++ {
		assert.False(t, rl.Allow(userID), "Additional message %d should be rejected", i+1)
	}
	
	// Verify final count is still 100 (rejected messages don't increment)
	rl.mu.Lock()
	limit, exists := rl.clients[userID]
	assert.True(t, exists)
	assert.Equal(t, 100, limit.messageCount)
	rl.mu.Unlock()
}

// TestRateLimiter_WindowReset tests sliding window behavior
func TestRateLimiter_WindowReset(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	
	userID := "window-test-user"
	
	// Fill the rate limit
	for i := 0; i < 100; i++ {
		assert.True(t, rl.Allow(userID))
	}
	
	// Should be at limit
	assert.False(t, rl.Allow(userID))
	
	// Manually advance the window start time to simulate time passage
	rl.mu.Lock()
	rl.clients[userID].windowStart = time.Now().Add(-61 * time.Second)
	rl.mu.Unlock()
	
	// After window reset, should be allowed again
	assert.True(t, rl.Allow(userID), "First message after window reset should be allowed")
	
	// Verify window was reset
	rl.mu.Lock()
	limit, exists := rl.clients[userID]
	assert.True(t, exists)
	assert.Equal(t, 1, limit.messageCount) // Reset to 1 after the new message
	assert.WithinDuration(t, time.Now(), limit.windowStart, time.Second)
	rl.mu.Unlock()
}

// TestRateLimiter_MultipleUsers tests independent user tracking
func TestRateLimiter_MultipleUsers(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	
	// Send messages from multiple users
	users := []string{"user1", "user2", "user3"}
	messagesPerUser := 30
	
	for _, userID := range users {
		for i := 0; i < messagesPerUser; i++ {
			assert.True(t, rl.Allow(userID), "User %s message %d should be allowed", userID, i+1)
		}
	}
	
	// Verify each user has independent limits
	rl.mu.Lock()
	for _, userID := range users {
		limit, exists := rl.clients[userID]
		assert.True(t, exists, "User %s should exist", userID)
		assert.Equal(t, messagesPerUser, limit.messageCount, "User %s should have %d messages", userID, messagesPerUser)
	}
	rl.mu.Unlock()
	
	// Each user should still be able to send more messages
	for _, userID := range users {
		assert.True(t, rl.Allow(userID), "User %s should be able to send more messages", userID)
	}
}

// TestRateLimiter_ConcurrentAccess tests thread safety
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	
	userID := "concurrent-user"
	numGoroutines := 10
	messagesPerGoroutine := 20
	
	var wg sync.WaitGroup
	
	// Concurrent access from multiple goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < messagesPerGoroutine; j++ {
				rl.Allow(userID) // Just call Allow, we'll check final state
			}
		}(i)
	}
	
	wg.Wait()
	
	// Check final state - should not exceed 100 messages
	rl.mu.Lock()
	limit, exists := rl.clients[userID]
	assert.True(t, exists)
	assert.LessOrEqual(t, limit.messageCount, 100, "Rate limit should not be exceeded even under concurrent access")
	assert.Greater(t, limit.messageCount, 0, "Some messages should have been allowed")
	rl.mu.Unlock()
	
	// Verify rate limit is still enforced
	if limit.messageCount >= 100 {
		assert.False(t, rl.Allow(userID), "Rate limit should still be enforced after concurrent access")
	}
}

// TestRateLimiter_Cleanup tests manual cleanup functionality
func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	
	// Create several users with messages
	users := []string{"user1", "user2", "user3", "user4"}
	for _, userID := range users {
		assert.True(t, rl.Allow(userID))
	}
	
	// Verify all users exist
	rl.mu.Lock()
	assert.Len(t, rl.clients, len(users))
	rl.mu.Unlock()
	
	// Age some users beyond cleanup threshold (5 minutes)
	rl.mu.Lock()
	rl.clients["user1"].windowStart = time.Now().Add(-6 * time.Minute)
	rl.clients["user2"].windowStart = time.Now().Add(-10 * time.Minute)
	// user3 and user4 remain recent
	rl.mu.Unlock()
	
	// Run cleanup
	rl.Cleanup()
	
	// Verify old users were removed
	rl.mu.Lock()
	assert.Len(t, rl.clients, 2, "Should have 2 users remaining after cleanup")
	assert.NotContains(t, rl.clients, "user1", "user1 should be cleaned up")
	assert.NotContains(t, rl.clients, "user2", "user2 should be cleaned up")
	assert.Contains(t, rl.clients, "user3", "user3 should remain")
	assert.Contains(t, rl.clients, "user4", "user4 should remain")
	rl.mu.Unlock()
}

// TestRateLimiter_AutoCleanup tests automatic cleanup
func TestRateLimiter_AutoCleanup(t *testing.T) {
	// This test would take too long to run the full 5-minute cycle
	// Instead, we test that the cleanup goroutine is running
	rl := NewRateLimiter()
	
	// Add a user
	assert.True(t, rl.Allow("test-user"))
	
	// Verify user exists
	rl.mu.Lock()
	assert.Len(t, rl.clients, 1)
	rl.mu.Unlock()
	
	// Stop should wait for cleanup goroutine to finish
	done := make(chan struct{})
	go func() {
		rl.Stop()
		close(done)
	}()
	
	// Should complete within reasonable time
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Stop() should complete quickly")
	}
}

// TestRateLimiter_ContextCancellation tests context-controlled cleanup
func TestRateLimiter_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	
	rl := NewRateLimiterWithContext(ctx)
	
	// Add a user
	assert.True(t, rl.Allow("test-user"))
	
	// Cancel context
	cancel()
	
	// Stop should complete quickly since context was cancelled
	done := make(chan struct{})
	go func() {
		rl.Stop()
		close(done)
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Stop() should complete quickly after context cancellation")
	}
}

// TestRateLimiter_EdgeCases tests edge cases and boundary conditions
func TestRateLimiter_EdgeCases(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	
	// Empty user ID should work
	assert.True(t, rl.Allow(""))
	
	// Very long user ID should work
	longUserID := "user" + string(make([]byte, 1000))
	assert.True(t, rl.Allow(longUserID))
	
	// Special characters in user ID
	specialUserID := "user@domain.com/with:special#chars"
	assert.True(t, rl.Allow(specialUserID))
	
	// Verify all users exist
	rl.mu.Lock()
	assert.Contains(t, rl.clients, "")
	assert.Contains(t, rl.clients, longUserID)
	assert.Contains(t, rl.clients, specialUserID)
	rl.mu.Unlock()
}

// TestRateLimiter_WindowBoundary tests exact window boundary behavior
func TestRateLimiter_WindowBoundary(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	
	userID := "boundary-test"
	
	// Send initial message to establish window
	assert.True(t, rl.Allow(userID))
	
	// Get the exact window start time
	rl.mu.Lock()
	windowStart := rl.clients[userID].windowStart
	rl.mu.Unlock()
	
	// Manually set window start to exactly 1 minute ago
	rl.mu.Lock()
	rl.clients[userID].windowStart = time.Now().Add(-time.Minute)
	rl.clients[userID].messageCount = 50
	rl.mu.Unlock()
	
	// Next message should reset the window
	assert.True(t, rl.Allow(userID))
	
	// Verify window was reset
	rl.mu.Lock()
	limit := rl.clients[userID]
	assert.Equal(t, 1, limit.messageCount, "Message count should reset to 1")
	assert.True(t, limit.windowStart.After(windowStart), "Window start should be updated")
	rl.mu.Unlock()
}

// TestRateLimiter_Stop tests graceful shutdown
func TestRateLimiter_Stop(t *testing.T) {
	rl := NewRateLimiter()
	
	// Add some users
	for i := 0; i < 5; i++ {
		assert.True(t, rl.Allow("user"+string(rune('0'+i))))
	}
	
	// Stop should complete without hanging
	start := time.Now()
	rl.Stop()
	elapsed := time.Since(start)
	
	assert.Less(t, elapsed, time.Second, "Stop should complete quickly")
	
	// Multiple stops should be safe
	rl.Stop()
	rl.Stop()
}

// TestRateLimiter_MemoryUsage tests memory efficiency
func TestRateLimiter_MemoryUsage(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	
	// Create many users
	numUsers := 1000
	for i := 0; i < numUsers; i++ {
		userID := "user" + string(rune('0'+(i%10))) + string(rune('0'+((i/10)%10))) + string(rune('0'+((i/100)%10)))
		assert.True(t, rl.Allow(userID))
	}
	
	// Verify all users exist
	rl.mu.Lock()
	assert.Len(t, rl.clients, numUsers)
	rl.mu.Unlock()
	
	// Age all users
	rl.mu.Lock()
	for _, limit := range rl.clients {
		limit.windowStart = time.Now().Add(-6 * time.Minute)
	}
	rl.mu.Unlock()
	
	// Cleanup should remove all users
	rl.Cleanup()
	
	rl.mu.Lock()
	assert.Len(t, rl.clients, 0, "All users should be cleaned up")
	rl.mu.Unlock()
}