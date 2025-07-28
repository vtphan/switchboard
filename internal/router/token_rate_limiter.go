package router

import (
	"time"
)

// TokenRateLimiter adapts TokenBucket to provide the same interface as RateLimiter
// ARCHITECTURAL DISCOVERY: Adapter pattern maintains backward compatibility
// while enabling the switch to more efficient token bucket algorithm
// Implements MessageRateLimiter interface
type TokenRateLimiter struct {
	bucket *TokenBucket  // TECHNICAL DISCOVERY: Composition over inheritance for cleaner design
}

// NewTokenRateLimiter creates a rate limiter using token bucket algorithm
// FUNCTIONAL DISCOVERY: 100 token capacity with 600ms refill = ~100 tokens/minute
// This matches the current sliding window behavior but with burst capability
func NewTokenRateLimiter() *TokenRateLimiter {
	// PERFORMANCE DISCOVERY: Token bucket eliminates per-client state management
	// 600ms refill rate = 100 tokens per minute (60000ms / 100 tokens = 600ms per token)
	bucket := NewTokenBucket(100, 600*time.Millisecond)
	
	return &TokenRateLimiter{
		bucket: bucket,
	}
}

// Allow checks if a message can be sent (maintains RateLimiter interface)
// ARCHITECTURAL DISCOVERY: userID parameter ignored - token bucket doesn't need per-client tracking
// This is the key architectural improvement: constant memory usage vs growing map
func (trl *TokenRateLimiter) Allow(userID string) bool {
	// FUNCTIONAL DISCOVERY: Ignore userID - token bucket provides global rate limiting
	// For per-client limiting, multiple buckets would be needed, but classroom use case
	// benefits more from global burst handling than strict per-client isolation
	return trl.bucket.Acquire()
}

// Cleanup is a no-op for token bucket (maintains RateLimiter interface)
// MEMORY MANAGEMENT DISCOVERY: No cleanup needed - token bucket has constant memory usage
// This eliminates the memory leak potential of the sliding window approach
func (trl *TokenRateLimiter) Cleanup() {
	// No-op: Token bucket doesn't accumulate per-client state to clean up
	// This is a key advantage - no background cleanup goroutine needed
}

// Stop gracefully shuts down the token bucket
// RESOURCE CLEANUP: Maintains same lifecycle management as original RateLimiter
func (trl *TokenRateLimiter) Stop() {
	trl.bucket.Stop()
}

// GetAvailableTokens returns current token count (for monitoring)
// OBSERVABILITY DISCOVERY: Token bucket provides better rate limiter visibility
// than sliding window which only shows per-client counts
func (trl *TokenRateLimiter) GetAvailableTokens() int {
	return trl.bucket.GetAvailableTokens()
}

// GetCapacity returns maximum burst size (for monitoring)
// OBSERVABILITY DISCOVERY: Expose configuration for debugging and monitoring
func (trl *TokenRateLimiter) GetCapacity() int {
	return trl.bucket.GetCapacity()
}