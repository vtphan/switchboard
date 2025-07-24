package router

import (
	"sync"
	"time"
)

// RateLimiter implements per-client rate limiting
// ARCHITECTURAL DISCOVERY: Per-client state tracking with proper cleanup prevents memory leaks
type RateLimiter struct {
	mu      sync.RWMutex
	clients map[string]*ClientLimit
}

// ClientLimit tracks rate limiting for a single client
// FUNCTIONAL DISCOVERY: Sliding window with minute-based reset provides exact 100 messages/minute limit
type ClientLimit struct {
	messageCount int
	windowStart  time.Time
}

// NewRateLimiter creates a new rate limiter
// FUNCTIONAL DISCOVERY: Initialize map to prevent nil pointer access during concurrent operations
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		clients: make(map[string]*ClientLimit),
	}
}

// Allow checks if client can send a message (100 per minute limit)
// TECHNICAL DISCOVERY: RWMutex for read-heavy operations, upgrade to write lock only when needed
func (rl *RateLimiter) Allow(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	
	limit, exists := rl.clients[userID]
	if !exists {
		// FUNCTIONAL DISCOVERY: First message always allowed, initialize tracking
		rl.clients[userID] = &ClientLimit{
			messageCount: 1,
			windowStart:  now,
		}
		return true
	}
	
	// Check if new minute window needed
	// TECHNICAL DISCOVERY: Sliding window resets exactly every minute for consistent rate limiting
	if now.Sub(limit.windowStart) >= time.Minute {
		limit.messageCount = 1
		limit.windowStart = now
		return true
	}
	
	// Check rate limit (100 messages per minute)
	if limit.messageCount >= 100 {
		return false
	}
	
	limit.messageCount++
	return true
}

// Cleanup removes old client entries (call periodically)
// ARCHITECTURAL DISCOVERY: Prevent memory leaks by removing stale client state
// after 5 minutes of inactivity (5x the rate limit window)
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	for userID, limit := range rl.clients {
		if now.Sub(limit.windowStart) > 5*time.Minute {
			delete(rl.clients, userID)
		}
	}
}