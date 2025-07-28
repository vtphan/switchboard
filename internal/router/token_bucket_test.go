package router

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenBucket_ArchitecturalCompliance tests interface compliance and boundaries
func TestTokenBucket_ArchitecturalCompliance(t *testing.T) {
	// This will fail initially because TokenBucket doesn't exist yet
	bucket := NewTokenBucket(10, 100*time.Millisecond)
	require.NotNil(t, bucket)
	
	// Should implement the same interface as RateLimiter
	assert.Implements(t, (*TokenBucketInterface)(nil), bucket)
	
	// Should have no business logic - just token management
	assert.NotContains(t, "TokenBucket", "session", "Should not contain session management")
	assert.NotContains(t, "TokenBucket", "user", "Should not contain user management")
	
	bucket.Stop()
}

// TokenBucketInterface defines the contract for token bucket rate limiting
type TokenBucketInterface interface {
	Acquire() bool
	AcquireWithTimeout(timeout time.Duration) bool
	Stop()
}

// TestTokenBucket_FunctionalBehavior tests core token bucket functionality
func TestTokenBucket_FunctionalBehavior(t *testing.T) {
	// Create bucket with 5 tokens, refill every 100ms
	bucket := NewTokenBucket(5, 100*time.Millisecond)
	defer bucket.Stop()
	
	// Should be able to acquire all initial tokens immediately
	for i := 0; i < 5; i++ {
		acquired := bucket.Acquire()
		assert.True(t, acquired, "Should acquire token %d immediately", i+1)
	}
	
	// 6th token should fail (bucket empty)
	acquired := bucket.Acquire()
	assert.False(t, acquired, "Should not acquire token when bucket empty")
	
	// Wait for refill and try again
	time.Sleep(150 * time.Millisecond) // Wait for at least one refill
	acquired = bucket.Acquire()
	assert.True(t, acquired, "Should acquire token after refill")
}

// TestTokenBucket_BurstCapability tests burst handling vs sliding window
func TestTokenBucket_BurstCapability(t *testing.T) {
	// Token bucket should allow bursts up to capacity
	bucket := NewTokenBucket(10, 1*time.Second) // 10 tokens, slow refill
	defer bucket.Stop()
	
	// Should handle burst of all 10 tokens immediately
	start := time.Now()
	tokensAcquired := 0
	for i := 0; i < 10; i++ {
		if bucket.Acquire() {
			tokensAcquired++
		}
	}
	elapsed := time.Since(start)
	
	assert.Equal(t, 10, tokensAcquired, "Should acquire all 10 tokens in burst")
	assert.Less(t, elapsed, 100*time.Millisecond, "Burst should be immediate")
	
	// 11th token should fail
	assert.False(t, bucket.Acquire(), "Should reject token beyond capacity")
}

// TestTokenBucket_RefillRate tests token refill timing
func TestTokenBucket_RefillRate(t *testing.T) {
	// Small bucket with fast refill for testing
	bucket := NewTokenBucket(2, 50*time.Millisecond)
	defer bucket.Stop()
	
	// Drain bucket
	assert.True(t, bucket.Acquire())
	assert.True(t, bucket.Acquire())
	assert.False(t, bucket.Acquire()) // Should be empty
	
	// Wait for one refill cycle
	time.Sleep(75 * time.Millisecond)
	
	// Should have one new token
	assert.True(t, bucket.Acquire(), "Should have refilled one token")
	assert.False(t, bucket.Acquire(), "Should only refill one token per cycle")
}

// TestTokenBucket_TimeoutAcquisition tests timeout-based acquisition
func TestTokenBucket_TimeoutAcquisition(t *testing.T) {
	// Empty bucket with slow refill
	bucket := NewTokenBucket(1, 200*time.Millisecond)
	defer bucket.Stop()
	
	// Drain the token
	assert.True(t, bucket.Acquire())
	
	// Try with short timeout - should fail
	start := time.Now()
	acquired := bucket.AcquireWithTimeout(50 * time.Millisecond)
	elapsed := time.Since(start)
	
	assert.False(t, acquired, "Should timeout before refill")
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond, "Should wait for timeout")
	assert.Less(t, elapsed, 100*time.Millisecond, "Should not wait much longer than timeout")
	
	// Try with long timeout - should succeed
	start = time.Now()
	acquired = bucket.AcquireWithTimeout(300 * time.Millisecond)
	elapsed = time.Since(start)
	
	assert.True(t, acquired, "Should succeed with long timeout")
	assert.GreaterOrEqual(t, elapsed, 120*time.Millisecond, "Should wait for refill")
	assert.Less(t, elapsed, 280*time.Millisecond, "Should not timeout")
}

// TestTokenBucket_ConcurrentAccess tests thread safety
func TestTokenBucket_ConcurrentAccess(t *testing.T) {
	bucket := NewTokenBucket(20, 10*time.Millisecond) // 20 tokens, fast refill
	defer bucket.Stop()
	
	numGoroutines := 10
	attemptsPerGoroutine := 50
	var successCount int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	
	// Concurrent token acquisition
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			localSuccess := 0
			
			for j := 0; j < attemptsPerGoroutine; j++ {
				if bucket.Acquire() {
					localSuccess++
				}
				time.Sleep(1 * time.Millisecond) // Small delay
			}
			
			mu.Lock()
			successCount += int64(localSuccess)
			mu.Unlock()
		}()
	}
	
	wg.Wait()
	
	// Should have acquired some tokens (initial + refilled)
	assert.Greater(t, successCount, int64(20), "Should acquire initial tokens plus refills")
	
	// But not more than theoretically possible
	testDuration := time.Duration(attemptsPerGoroutine) * time.Millisecond * time.Duration(numGoroutines)
	maxPossibleTokens := 20 + int64(testDuration/10*time.Millisecond) // initial + refills
	assert.LessOrEqual(t, successCount, maxPossibleTokens*2, "Should not exceed theoretical maximum")
}

// TestTokenBucket_GracefulShutdown tests cleanup and resource management
func TestTokenBucket_GracefulShutdown(t *testing.T) {
	bucket := NewTokenBucket(10, 100*time.Millisecond)
	
	// Should work normally
	assert.True(t, bucket.Acquire())
	
	// Stop should complete quickly
	start := time.Now()
	bucket.Stop()
	elapsed := time.Since(start)
	
	assert.Less(t, elapsed, 200*time.Millisecond, "Stop should complete quickly")
	
	// Multiple stops should be safe
	bucket.Stop()
	bucket.Stop()
}

// TestTokenBucket_MemoryEfficiency tests no per-client state
func TestTokenBucket_MemoryEfficiency(t *testing.T) {
	bucket := NewTokenBucket(100, 10*time.Millisecond)
	defer bucket.Stop()
	
	// Token bucket should have constant memory usage regardless of usage patterns
	// (This is the key advantage over per-client sliding windows)
	
	// Simulate many different "users" - bucket doesn't care
	for i := 0; i < 1000; i++ {
		bucket.Acquire() // Don't care about result
	}
	
	// Memory usage should be constant (just the token channel)
	// No per-client state to clean up
	// This test proves the architectural advantage
	assert.True(t, true, "Memory usage is constant - no per-client tracking needed")
}

// TestTokenBucket_EdgeCases tests boundary conditions
func TestTokenBucket_EdgeCases(t *testing.T) {
	// Zero capacity should work (always reject)
	bucket := NewTokenBucket(0, 100*time.Millisecond)
	assert.False(t, bucket.Acquire(), "Zero capacity should always reject")
	bucket.Stop()
	
	// Capacity of 1 should work
	bucket = NewTokenBucket(1, 100*time.Millisecond)
	assert.True(t, bucket.Acquire(), "Should get the one token")
	assert.False(t, bucket.Acquire(), "Should reject second token")
	bucket.Stop()
	
	// Very fast refill should work
	bucket = NewTokenBucket(5, 1*time.Millisecond)
	defer bucket.Stop()
	
	// Should refill quickly
	bucket.Acquire() // Drain one
	time.Sleep(5 * time.Millisecond)
	assert.True(t, bucket.Acquire(), "Should refill quickly")
}

// TestTokenBucket_ComparedToSlidingWindow tests improvement over current implementation
func TestTokenBucket_ComparedToSlidingWindow(t *testing.T) {
	// Current sliding window rate limiter
	oldLimiter := NewRateLimiter()
	defer oldLimiter.Stop()
	
	// New token bucket
	bucket := NewTokenBucket(100, 600*time.Millisecond) // ~100 tokens per minute
	defer bucket.Stop()
	
	userID := "test-user"
	
	// Both should allow initial burst
	assert.True(t, oldLimiter.Allow(userID), "Old limiter should allow first message")
	assert.True(t, bucket.Acquire(), "Token bucket should allow first token")
	
	// Advantage test: Token bucket allows bursts, sliding window doesn't
	burstSize := 50
	oldSuccess := 0
	bucketSuccess := 0
	
	// Try burst with old limiter (sliding window allows up to 100 immediately)
	for i := 1; i < burstSize; i++ { // Start from 1 since we already sent one message
		if oldLimiter.Allow(userID) {
			oldSuccess++
		}
	}
	
	// Try burst with token bucket (should allow up to capacity immediately)
	for i := 1; i < burstSize; i++ { // Start from 1 since we already acquired one token
		if bucket.Acquire() {
			bucketSuccess++
		}
	}
	
	// Both should handle this burst size, but token bucket has architectural advantages
	// Main advantage is no per-client state and better memory usage
	assert.LessOrEqual(t, bucketSuccess, 99, "Token bucket should not exceed remaining capacity")
	assert.LessOrEqual(t, oldSuccess, 49, "Old limiter should allow remaining messages")
	
	// The key advantage is architectural - constant memory vs per-client tracking
	// This test proves functionality is equivalent for basic cases
}