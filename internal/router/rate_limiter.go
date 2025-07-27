package router

import (
	"context"
	"log"
	"sync"
	"time"
)

// RateLimiter implements per-client rate limiting
// MEMORY LEAK FIX: Auto-cleanup prevents memory leaks from disconnected clients
type RateLimiter struct {
	mu           sync.RWMutex
	clients      map[string]*ClientLimit
	stopCleanup  chan struct{}
	cleanupDone  sync.WaitGroup
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
// PERFORMANCE FIX: Use RWMutex pattern - read lock first, upgrade to write lock only when needed
func (rl *RateLimiter) Allow(userID string) bool {
	now := time.Now()
	
	// First, try with read lock for existing clients
	rl.mu.RLock()
	limit, exists := rl.clients[userID]
	if exists {
		// Check if we can proceed with just read lock (window hasn't expired and under limit)
		if now.Sub(limit.windowStart) < time.Minute && limit.messageCount < 100 {
			rl.mu.RUnlock()
			
			// Upgrade to write lock for the increment
			rl.mu.Lock()
			defer rl.mu.Unlock()
			
			// Recheck after acquiring write lock (double-checked locking pattern)
			if now.Sub(limit.windowStart) < time.Minute && limit.messageCount < 100 {
				limit.messageCount++
				return true
			}
			// Fall through to handle window reset or rate limit exceeded
			if now.Sub(limit.windowStart) >= time.Minute {
				limit.messageCount = 1
				limit.windowStart = now
				return true
			}
			return false // Rate limit exceeded
		}
		
		// Window expired, need write lock for reset
		if now.Sub(limit.windowStart) >= time.Minute {
			rl.mu.RUnlock()
			rl.mu.Lock()
			defer rl.mu.Unlock()
			
			// Recheck after acquiring write lock
			if now.Sub(limit.windowStart) >= time.Minute {
				limit.messageCount = 1
				limit.windowStart = now
				return true
			}
			// Window was reset by another goroutine, check limit
			if limit.messageCount < 100 {
				limit.messageCount++
				return true
			}
			return false
		}
		
		// Rate limit exceeded
		rl.mu.RUnlock()
		return false
	}
	rl.mu.RUnlock()
	
	// New client - need write lock
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	// Double-check after acquiring write lock
	limit, exists = rl.clients[userID]
	if !exists {
		// FUNCTIONAL DISCOVERY: First message always allowed, initialize tracking
		rl.clients[userID] = &ClientLimit{
			messageCount: 1,
			windowStart:  now,
		}
		return true
	}
	
	// Client was added by another goroutine, apply normal logic
	if now.Sub(limit.windowStart) >= time.Minute {
		limit.messageCount = 1
		limit.windowStart = now
		return true
	}
	
	if limit.messageCount >= 100 {
		return false
	}
	
	limit.messageCount++
	return true
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
	close(rl.stopCleanup)
	rl.cleanupDone.Wait()
}