package router

import (
	"sync"
	"time"
)

// TokenBucket implements rate limiting using the token bucket algorithm
// ARCHITECTURAL DISCOVERY: Channel-based design eliminates mutex contention on token acquisition
// compared to the sliding window approach which requires locks on every Allow() call
type TokenBucket struct {
	tokens     chan struct{}         // FUNCTIONAL DISCOVERY: Channel capacity = burst limit, self-regulating
	refillRate time.Duration         // TECHNICAL DISCOVERY: Refill interval balances precision vs CPU usage
	capacity   int                   // Maximum burst size - classroom-friendly default
	ticker     *time.Ticker          // RESOURCE MANAGEMENT: Controlled refill timing
	closeOnce  sync.Once             // CONCURRENT SAFETY: Ensure single shutdown
	closed     chan struct{}         // GRACEFUL SHUTDOWN: Coordinate cleanup
}

// NewTokenBucket creates a token bucket with specified capacity and refill rate
// FUNCTIONAL DISCOVERY: Pre-filling bucket allows immediate bursts for better user experience
// Pattern from docs/go_concurrency_patterns_llm.md lines 544-605
func NewTokenBucket(capacity int, refillRate time.Duration) *TokenBucket {
	// ARCHITECTURAL DISCOVERY: Zero capacity creates always-rejecting limiter (useful for maintenance mode)
	if capacity < 0 {
		capacity = 0
	}
	
	tb := &TokenBucket{
		tokens:     make(chan struct{}, capacity),
		refillRate: refillRate,
		capacity:   capacity,
		closed:     make(chan struct{}),
	}
	
	// FUNCTIONAL DISCOVERY: Pre-fill bucket to allow immediate bursts
	// This is key advantage over sliding window - natural burst handling
	for i := 0; i < capacity; i++ {
		tb.tokens <- struct{}{}
	}
	
	// TECHNICAL DISCOVERY: Start refill process only if refill rate > 0
	// Prevents unnecessary goroutine for one-shot buckets
	if refillRate > 0 {
		tb.ticker = time.NewTicker(refillRate)
		go tb.refill()
	}
	
	return tb
}

// refill continuously adds tokens to the bucket at the specified rate
// CONCURRENCY PATTERN: Single refill goroutine prevents race conditions
// MEMORY EFFICIENCY: No per-client state - constant memory usage regardless of user count
func (tb *TokenBucket) refill() {
	defer func() {
		if tb.ticker != nil {
			tb.ticker.Stop()
		}
	}()
	
	for {
		select {
		case <-tb.ticker.C:
			// FUNCTIONAL DISCOVERY: Non-blocking send prevents overflow
			// If bucket is full, extra tokens are naturally discarded
			select {
			case tb.tokens <- struct{}{}:
				// Token added successfully
			default:
				// Bucket full - discard token (natural overflow handling)
				// This is expected behavior - no action needed
			}
			
		case <-tb.closed:
			return
		}
	}
}

// Acquire attempts to get a token immediately
// ARCHITECTURAL DISCOVERY: Channel operations are atomic and lock-free
// This eliminates the mutex contention seen in sliding window approach
func (tb *TokenBucket) Acquire() bool {
	select {
	case <-tb.tokens:
		return true // Got token
	case <-tb.closed:
		return false // Bucket closed
	default:
		// No tokens available immediately
		return false
	}
}

// AcquireWithTimeout attempts to get a token within the specified timeout
// FUNCTIONAL DISCOVERY: Timeout pattern allows graceful degradation under load
// Better than blocking indefinitely or busy-waiting
func (tb *TokenBucket) AcquireWithTimeout(timeout time.Duration) bool {
	select {
	case <-tb.tokens:
		return true // Got token
	case <-time.After(timeout):
		return false // Timeout
	case <-tb.closed:
		return false // Bucket closed
	}
}

// Stop gracefully shuts down the token bucket
// RESOURCE CLEANUP: Ensures proper cleanup of goroutines and resources
// CONCURRENT SAFETY: Uses sync.Once to prevent multiple shutdown attempts
func (tb *TokenBucket) Stop() {
	tb.closeOnce.Do(func() {
		close(tb.closed)
		
		// TECHNICAL DISCOVERY: Ticker cleanup must happen after signaling shutdown
		// to prevent race between refill goroutine and Stop()
		if tb.ticker != nil {
			tb.ticker.Stop()
		}
		
		// FUNCTIONAL DISCOVERY: Don't close tokens channel - let it drain naturally
		// This prevents panics if there are concurrent Acquire() calls
	})
}

// GetCapacity returns the bucket capacity (for testing/monitoring)
// OBSERVABILITY: Enable monitoring of rate limiter configuration
func (tb *TokenBucket) GetCapacity() int {
	return tb.capacity
}

// GetRefillRate returns the refill rate (for testing/monitoring)
// OBSERVABILITY: Enable monitoring of rate limiter timing
func (tb *TokenBucket) GetRefillRate() time.Duration {
	return tb.refillRate
}

// GetAvailableTokens returns approximate number of available tokens
// OBSERVABILITY: Enable monitoring of current bucket state
// TECHNICAL DISCOVERY: len() on channel is safe but may be stale
func (tb *TokenBucket) GetAvailableTokens() int {
	return len(tb.tokens)
}