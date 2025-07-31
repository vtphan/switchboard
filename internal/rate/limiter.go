package rate

import (
	"sync"
	"time"

	"switchboard/pkg/config"
)

// RateLimiter implements a sliding window rate limiter for user messages
// This uses the token bucket algorithm with automatic cleanup as specified in tech specs
type RateLimiter struct {
	mu          sync.RWMutex
	users       map[string]*userRateData
	maxMessages int
	window      time.Duration
	cleanupDone chan struct{}
	stopCleanup chan struct{}
}

// userRateData tracks rate limiting data for a single user
type userRateData struct {
	timestamps []time.Time
	lastUpdate time.Time
}

// NewRateLimiter creates a new RateLimiter with the specified limits
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		users:       make(map[string]*userRateData),
		maxMessages: config.RateLimitMaxMessages,
		window:      config.RateLimitWindow,
		cleanupDone: make(chan struct{}),
		stopCleanup: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// NewRateLimiterWithConfig creates a new RateLimiter with custom limits
// This is useful for testing scenarios where different rate limits are needed
func NewRateLimiterWithConfig(maxMessages int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		users:       make(map[string]*userRateData),
		maxMessages: maxMessages,
		window:      window,
		cleanupDone: make(chan struct{}),
		stopCleanup: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a user is allowed to send a message based on rate limits
// Returns true if the user is within their rate limit, false if exceeded
func (rl *RateLimiter) Allow(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Get or create user rate data
	userData, exists := rl.users[userID]
	if !exists {
		userData = &userRateData{
			timestamps: make([]time.Time, 0),
			lastUpdate: now,
		}
		rl.users[userID] = userData
	}

	// Remove timestamps outside the current window
	cutoff := now.Add(-rl.window)
	userData.timestamps = rl.removeOldTimestamps(userData.timestamps, cutoff)
	userData.lastUpdate = now

	// Check if user has exceeded the rate limit
	if len(userData.timestamps) >= rl.maxMessages {
		return false
	}

	// Allow the message and record the timestamp
	userData.timestamps = append(userData.timestamps, now)
	return true
}

// GetUserMessageCount returns the current number of messages in the user's window
// This is useful for monitoring and testing purposes
func (rl *RateLimiter) GetUserMessageCount(userID string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	userData, exists := rl.users[userID]
	if !exists {
		return 0
	}

	// Remove old timestamps before counting
	now := time.Now()
	cutoff := now.Add(-rl.window)
	validTimestamps := rl.removeOldTimestamps(userData.timestamps, cutoff)

	return len(validTimestamps)
}

// Reset clears all rate limiting data for a user
// This is useful for testing and administrative purposes
func (rl *RateLimiter) Reset(userID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.users, userID)
}

// ResetAll clears all rate limiting data for all users
// This is useful for testing purposes
func (rl *RateLimiter) ResetAll() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.users = make(map[string]*userRateData)
}

// Stop stops the rate limiter and its cleanup goroutine
// This should be called when shutting down the application
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
	<-rl.cleanupDone
}

// removeOldTimestamps removes timestamps that are older than the cutoff time
// This implements the sliding window behavior
func (rl *RateLimiter) removeOldTimestamps(timestamps []time.Time, cutoff time.Time) []time.Time {
	// Find the first timestamp that's still valid
	startIndex := 0
	for i, ts := range timestamps {
		if ts.After(cutoff) {
			startIndex = i
			break
		}
		startIndex = i + 1
	}

	// If all timestamps are old, return empty slice
	if startIndex >= len(timestamps) {
		return []time.Time{}
	}

	// Return slice with only valid timestamps
	return timestamps[startIndex:]
}

// cleanupLoop runs periodically to remove stale user data
// This prevents memory leaks from inactive users
func (rl *RateLimiter) cleanupLoop() {
	defer close(rl.cleanupDone)

	ticker := time.NewTicker(config.RateLimitCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanupStaleUsers()
		case <-rl.stopCleanup:
			return
		}
	}
}

// cleanupStaleUsers removes user data that hasn't been accessed recently
// This prevents memory buildup from users who are no longer active
func (rl *RateLimiter) cleanupStaleUsers() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	staleThreshold := now.Add(-config.RateLimitCleanupInterval * 2) // 10 minutes

	for userID, userData := range rl.users {
		// Remove users who haven't sent messages recently
		if userData.lastUpdate.Before(staleThreshold) {
			delete(rl.users, userID)
		}
	}
}
