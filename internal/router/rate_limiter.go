package router

import (
	"context"
	"log"
	"sync"
	"time"
)

// RateLimiter implements per-client rate limiting using sliding window
// MEMORY LEAK FIX: Auto-cleanup prevents memory leaks from disconnected clients
// Implements MessageRateLimiter interface
type RateLimiter struct {
	mu           sync.Mutex
	clients      map[string]*ClientLimit
	stopCleanup  chan struct{}
	cleanupDone  sync.WaitGroup
	stopped      bool
}

// ClientLimit tracks rate limiting for a single client
// FUNCTIONAL DISCOVERY: Sliding window with minute-based reset provides exact 100 messages/minute limit
type ClientLimit struct {
	messageCount int
	windowStart  time.Time
}

// NewRateLimiter creates a new rate limiter with automatic cleanup
// MEMORY LEAK FIX: Start background cleanup goroutine to prevent memory leaks
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		clients:     make(map[string]*ClientLimit),
		stopCleanup: make(chan struct{}),
	}
	
	// Start automatic cleanup goroutine
	rl.cleanupDone.Add(1)
	go rl.autoCleanup()
	
	return rl
}

// NewRateLimiterWithContext creates a rate limiter with context-controlled cleanup
// CONTEXT SUPPORT: Allow external context to control cleanup lifecycle
func NewRateLimiterWithContext(ctx context.Context) *RateLimiter {
	rl := &RateLimiter{
		clients:     make(map[string]*ClientLimit),
		stopCleanup: make(chan struct{}),
	}
	
	// Start cleanup with context support
	rl.cleanupDone.Add(1)
	go rl.autoCleanupWithContext(ctx)
	
	return rl
}

// Allow checks if client can send a message (100 per minute limit)
// Simplified single-lock approach for classroom environments
func (rl *RateLimiter) Allow(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	limit := rl.getOrCreateLimitUnsafe(userID, now)
	
	// Reset window if expired
	if now.Sub(limit.windowStart) >= time.Minute {
		limit.windowStart = now
		limit.messageCount = 1
		return true
	}
	
	// Check rate limit
	if limit.messageCount >= 100 {
		return false
	}
	
	limit.messageCount++
	return true
}

// getOrCreateLimitUnsafe gets or creates a client limit (must be called with lock held)
func (rl *RateLimiter) getOrCreateLimitUnsafe(userID string, now time.Time) *ClientLimit {
	if limit, exists := rl.clients[userID]; exists {
		return limit
	}
	
	// Create new limit for first-time user
	limit := &ClientLimit{
		messageCount: 0,
		windowStart:  now,
	}
	rl.clients[userID] = limit
	return limit
}

// Cleanup removes old client entries (manual cleanup)
// ARCHITECTURAL DISCOVERY: Prevent memory leaks by removing stale client state
// after 5 minutes of inactivity (5x the rate limit window)
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	removedCount := 0
	for userID, limit := range rl.clients {
		if now.Sub(limit.windowStart) > 5*time.Minute {
			delete(rl.clients, userID)
			removedCount++
		}
	}
	
	if removedCount > 0 {
		log.Printf("Rate limiter cleanup: removed %d stale client entries", removedCount)
	}
}

// autoCleanup runs automatic cleanup every 5 minutes
// MEMORY LEAK FIX: Background cleanup prevents unlimited memory growth
func (rl *RateLimiter) autoCleanup() {
	defer rl.cleanupDone.Done()
	
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			rl.Cleanup()
		case <-rl.stopCleanup:
			return
		}
	}
}

// autoCleanupWithContext runs automatic cleanup with context cancellation support
// CONTEXT SUPPORT: Respect external context for graceful shutdown
func (rl *RateLimiter) autoCleanupWithContext(ctx context.Context) {
	defer rl.cleanupDone.Done()
	
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			rl.Cleanup()
		case <-ctx.Done():
			return
		case <-rl.stopCleanup:
			return
		}
	}
}

// Stop gracefully stops the rate limiter and cleanup goroutine
// RESOURCE CLEANUP: Ensure proper cleanup goroutine termination
func (rl *RateLimiter) Stop() {
	rl.mu.Lock()
	if rl.stopped {
		rl.mu.Unlock()
		return
	}
	rl.stopped = true
	rl.mu.Unlock()
	
	close(rl.stopCleanup)
	rl.cleanupDone.Wait()
}