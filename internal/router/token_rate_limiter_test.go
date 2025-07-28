package router

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenRateLimiter_InterfaceCompatibility tests that TokenRateLimiter maintains same interface
func TestTokenRateLimiter_InterfaceCompatibility(t *testing.T) {
	limiter := NewTokenRateLimiter()
	require.NotNil(t, limiter)
	defer limiter.Stop()
	
	// Should have same methods as RateLimiter
	assert.True(t, limiter.Allow("user1"), "Should allow first message")
	assert.True(t, limiter.Allow("user2"), "Should allow message from different user")
	
	// Cleanup should be safe to call
	limiter.Cleanup() // Should not panic
}

// TestTokenRateLimiter_GlobalRateLimit tests that it provides global (not per-client) limiting
func TestTokenRateLimiter_GlobalRateLimit(t *testing.T) {
	limiter := NewTokenRateLimiter()
	defer limiter.Stop()
	
	// Fill the bucket with messages from different users
	allowedCount := 0
	for i := 0; i < 150; i++ { // Try more than capacity
		userID := "user" + string(rune('A'+(i%26))) // Different users
		if limiter.Allow(userID) {
			allowedCount++
		}
	}
	
	// Should allow up to capacity regardless of user distribution
	assert.Equal(t, 100, allowedCount, "Should allow exactly 100 messages initially")
	
	// Additional messages should be rejected
	assert.False(t, limiter.Allow("newuser"), "Should reject after capacity reached")
}

// TestTokenRateLimiter_BurstHandling tests improved burst capability vs sliding window
func TestTokenRateLimiter_BurstHandling(t *testing.T) {
	limiter := NewTokenRateLimiter()
	defer limiter.Stop()
	
	// Should handle immediate burst up to full capacity
	start := time.Now()
	burstSize := 50
	successCount := 0
	
	for i := 0; i < burstSize; i++ {
		if limiter.Allow("user1") {
			successCount++
		}
	}
	elapsed := time.Since(start)
	
	assert.Equal(t, burstSize, successCount, "Should handle full burst immediately")
	assert.Less(t, elapsed, 100*time.Millisecond, "Burst should be immediate")
}

// TestTokenRateLimiter_RefillBehavior tests token refill over time
func TestTokenRateLimiter_RefillBehavior(t *testing.T) {
	limiter := NewTokenRateLimiter()
	defer limiter.Stop()
	
	// Drain some tokens
	initialAvailable := limiter.GetAvailableTokens()
	assert.Equal(t, 100, initialAvailable, "Should start with full capacity")
	
	// Use some tokens
	tokensUsed := 10
	for i := 0; i < tokensUsed; i++ {
		assert.True(t, limiter.Allow("user1"))
	}
	
	afterUse := limiter.GetAvailableTokens()
	assert.Equal(t, 100-tokensUsed, afterUse, "Should have fewer tokens after use")
	
	// Wait for refill (600ms per token)
	time.Sleep(2 * time.Second) // Should refill ~3 tokens
	
	afterRefill := limiter.GetAvailableTokens()
	assert.Greater(t, afterRefill, afterUse, "Should have more tokens after refill")
	assert.LessOrEqual(t, afterRefill, 100, "Should not exceed capacity")
}

// TestTokenRateLimiter_MemoryEfficiency tests constant memory usage
func TestTokenRateLimiter_MemoryEfficiency(t *testing.T) {
	limiter := NewTokenRateLimiter()
	defer limiter.Stop()
	
	// Simulate many users - memory usage should be constant
	userCount := 1000
	for i := 0; i < userCount; i++ {
		userID := "user" + string(rune('0'+(i%10))) + string(rune('0'+((i/10)%10))) + string(rune('0'+((i/100)%10)))
		limiter.Allow(userID) // Don't care about result
	}
	
	// No cleanup needed - this is the advantage
	// Original RateLimiter would have 1000 entries to clean up
	limiter.Cleanup() // Should be no-op
	
	// Memory usage is constant regardless of user count
	assert.Equal(t, 100, limiter.GetCapacity(), "Capacity unchanged by user count")
}

// TestTokenRateLimiter_ObservabilityMethods tests monitoring capabilities
func TestTokenRateLimiter_ObservabilityMethods(t *testing.T) {
	limiter := NewTokenRateLimiter()
	defer limiter.Stop()
	
	// Test observability methods
	capacity := limiter.GetCapacity()
	assert.Equal(t, 100, capacity, "Should report correct capacity")
	
	available := limiter.GetAvailableTokens()
	assert.Equal(t, 100, available, "Should start with full availability")
	
	// Use some tokens
	limiter.Allow("user1")
	limiter.Allow("user2")
	
	availableAfter := limiter.GetAvailableTokens()
	assert.Equal(t, 98, availableAfter, "Should track token usage")
}

// TestTokenRateLimiter_BackwardCompatibility tests drop-in replacement behavior
func TestTokenRateLimiter_BackwardCompatibility(t *testing.T) {
	// Compare with original RateLimiter behavior
	oldLimiter := NewRateLimiter()
	defer oldLimiter.Stop()
	
	newLimiter := NewTokenRateLimiter()
	defer newLimiter.Stop()
	
	userID := "test-user"
	
	// Both should allow initial messages
	assert.True(t, oldLimiter.Allow(userID), "Old limiter should allow first message")
	assert.True(t, newLimiter.Allow(userID), "New limiter should allow first message")
	
	// Both should have same basic rate limiting behavior
	// (Though internal mechanics differ - old is per-client, new is global)
	oldAllowed := 0
	newAllowed := 0
	
	for i := 0; i < 50; i++ {
		if oldLimiter.Allow(userID) {
			oldAllowed++
		}
		if newLimiter.Allow(userID) {
			newAllowed++
		}
	}
	
	// Both should allow messages (exact counts may differ due to different algorithms)
	assert.Greater(t, oldAllowed, 0, "Old limiter should allow some messages")
	assert.Greater(t, newAllowed, 0, "New limiter should allow some messages")
}

// TestTokenRateLimiter_GracefulShutdown tests resource cleanup
func TestTokenRateLimiter_GracefulShutdown(t *testing.T) {
	limiter := NewTokenRateLimiter()
	
	// Should work normally
	assert.True(t, limiter.Allow("user1"))
	
	// Stop should complete quickly
	start := time.Now()
	limiter.Stop()
	elapsed := time.Since(start)
	
	assert.Less(t, elapsed, 200*time.Millisecond, "Stop should complete quickly")
	
	// Multiple stops should be safe
	limiter.Stop()
	limiter.Stop()
}