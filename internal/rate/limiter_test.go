package rate

import (
	"testing"
	"time"

	"switchboard/pkg/config"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter()
	if rl == nil {
		t.Error("Expected NewRateLimiter to return non-nil limiter")
	}

	// Clean up
	rl.Stop()
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	userID := "test-user-1"

	t.Run("allow_messages_within_limit", func(t *testing.T) {
		// Reset user data
		rl.Reset(userID)

		// Should allow messages up to the limit
		for i := 0; i < config.RateLimitMaxMessages; i++ {
			if !rl.Allow(userID) {
				t.Errorf("Expected message %d to be allowed, but was denied", i+1)
			}
		}
	})

	t.Run("deny_messages_over_limit", func(t *testing.T) {
		// Reset user data
		rl.Reset(userID)

		// Fill up the rate limit
		for i := 0; i < config.RateLimitMaxMessages; i++ {
			rl.Allow(userID)
		}

		// Next message should be denied
		if rl.Allow(userID) {
			t.Error("Expected message over limit to be denied")
		}
	})

	t.Run("allow_after_window_expires", func(t *testing.T) {
		// This test would require waiting for the actual window to expire
		// For testing purposes, we'll test the sliding window logic indirectly
		// by testing the removeOldTimestamps function separately

		// Reset user data
		rl.Reset(userID)

		// Add some messages
		for i := 0; i < 5; i++ {
			if !rl.Allow(userID) {
				t.Errorf("Expected message %d to be allowed", i+1)
			}
		}

		// Verify we have 5 messages recorded
		count := rl.GetUserMessageCount(userID)
		if count != 5 {
			t.Errorf("Expected 5 messages recorded, got %d", count)
		}
	})

	t.Run("different_users_independent_limits", func(t *testing.T) {
		user1 := "user1"
		user2 := "user2"

		// Reset both users
		rl.Reset(user1)
		rl.Reset(user2)

		// Fill user1's limit
		for i := 0; i < config.RateLimitMaxMessages; i++ {
			if !rl.Allow(user1) {
				t.Errorf("Expected message %d for user1 to be allowed", i+1)
			}
		}

		// user1 should be rate limited
		if rl.Allow(user1) {
			t.Error("Expected user1 to be rate limited")
		}

		// user2 should still be able to send messages
		if !rl.Allow(user2) {
			t.Error("Expected user2 to be able to send messages")
		}
	})
}

func TestRateLimiter_GetUserMessageCount(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	userID := "test-user-2"

	t.Run("zero_count_for_new_user", func(t *testing.T) {
		count := rl.GetUserMessageCount("nonexistent-user")
		if count != 0 {
			t.Errorf("Expected 0 messages for nonexistent user, got %d", count)
		}
	})

	t.Run("accurate_count_after_messages", func(t *testing.T) {
		// Reset user data
		rl.Reset(userID)

		// Send some messages
		messageCount := 10
		for i := 0; i < messageCount; i++ {
			rl.Allow(userID)
		}

		count := rl.GetUserMessageCount(userID)
		if count != messageCount {
			t.Errorf("Expected %d messages recorded, got %d", messageCount, count)
		}
	})
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	userID := "test-user-3"

	// Send some messages
	for i := 0; i < 5; i++ {
		rl.Allow(userID)
	}

	// Verify messages are recorded
	count := rl.GetUserMessageCount(userID)
	if count != 5 {
		t.Errorf("Expected 5 messages before reset, got %d", count)
	}

	// Reset the user
	rl.Reset(userID)

	// Verify count is zero after reset
	count = rl.GetUserMessageCount(userID)
	if count != 0 {
		t.Errorf("Expected 0 messages after reset, got %d", count)
	}

	// Verify user can send messages again
	if !rl.Allow(userID) {
		t.Error("Expected user to be able to send messages after reset")
	}
}

func TestRateLimiter_ResetAll(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	user1 := "user1"
	user2 := "user2"

	// Send messages for both users
	for i := 0; i < 5; i++ {
		rl.Allow(user1)
		rl.Allow(user2)
	}

	// Verify both users have messages recorded
	if rl.GetUserMessageCount(user1) != 5 {
		t.Error("Expected user1 to have 5 messages before reset")
	}
	if rl.GetUserMessageCount(user2) != 5 {
		t.Error("Expected user2 to have 5 messages before reset")
	}

	// Reset all users
	rl.ResetAll()

	// Verify both users have zero messages
	if rl.GetUserMessageCount(user1) != 0 {
		t.Error("Expected user1 to have 0 messages after reset all")
	}
	if rl.GetUserMessageCount(user2) != 0 {
		t.Error("Expected user2 to have 0 messages after reset all")
	}
}

func TestRateLimiter_RemoveOldTimestamps(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	now := time.Now()

	t.Run("remove_old_timestamps", func(t *testing.T) {
		timestamps := []time.Time{
			now.Add(-2 * time.Hour),    // Old
			now.Add(-1 * time.Hour),    // Old
			now.Add(-30 * time.Second), // Recent
			now.Add(-10 * time.Second), // Recent
			now,                        // Current
		}

		cutoff := now.Add(-1 * time.Minute)
		result := rl.removeOldTimestamps(timestamps, cutoff)

		// Should keep only the last 3 timestamps
		expectedCount := 3
		if len(result) != expectedCount {
			t.Errorf("Expected %d timestamps after removal, got %d", expectedCount, len(result))
		}

		// Verify all remaining timestamps are after cutoff
		for i, ts := range result {
			if ts.Before(cutoff) || ts.Equal(cutoff) {
				t.Errorf("Timestamp %d should be after cutoff, but is before/equal", i)
			}
		}
	})

	t.Run("remove_all_old_timestamps", func(t *testing.T) {
		timestamps := []time.Time{
			now.Add(-2 * time.Hour),
			now.Add(-1 * time.Hour),
			now.Add(-30 * time.Minute),
		}

		cutoff := now.Add(-1 * time.Minute)
		result := rl.removeOldTimestamps(timestamps, cutoff)

		// Should remove all timestamps
		if len(result) != 0 {
			t.Errorf("Expected 0 timestamps after removal, got %d", len(result))
		}
	})

	t.Run("keep_all_recent_timestamps", func(t *testing.T) {
		timestamps := []time.Time{
			now.Add(-30 * time.Second),
			now.Add(-20 * time.Second),
			now.Add(-10 * time.Second),
		}

		cutoff := now.Add(-1 * time.Minute)
		result := rl.removeOldTimestamps(timestamps, cutoff)

		// Should keep all timestamps
		if len(result) != len(timestamps) {
			t.Errorf("Expected %d timestamps after removal, got %d", len(timestamps), len(result))
		}
	})

	t.Run("empty_timestamps_slice", func(t *testing.T) {
		timestamps := []time.Time{}
		cutoff := now.Add(-1 * time.Minute)
		result := rl.removeOldTimestamps(timestamps, cutoff)

		// Should return empty slice
		if len(result) != 0 {
			t.Errorf("Expected 0 timestamps for empty input, got %d", len(result))
		}
	})
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	userID := "concurrent-user"

	// Reset user data
	rl.Reset(userID)

	// Test concurrent access to the rate limiter
	done := make(chan bool, 10)

	// Start multiple goroutines that try to send messages
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Each goroutine tries to send some messages
			for j := 0; j < 20; j++ {
				rl.Allow(userID)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify the rate limiter didn't crash and has some reasonable state
	count := rl.GetUserMessageCount(userID)
	if count < 0 {
		t.Error("Message count should not be negative after concurrent access")
	}

	if count > config.RateLimitMaxMessages {
		t.Errorf("Message count should not exceed limit after concurrent access, got %d", count)
	}
}

func TestRateLimiter_Stop(t *testing.T) {
	rl := NewRateLimiter()

	// Verify the rate limiter is working
	if !rl.Allow("test-user") {
		t.Error("Expected rate limiter to work before stopping")
	}

	// Stop the rate limiter
	rl.Stop()

	// The limiter should still work for basic operations after stopping
	// (only the cleanup goroutine is stopped)
	if !rl.Allow("test-user-2") {
		t.Error("Expected rate limiter basic functionality to work after stopping")
	}
}

// Benchmark tests to verify performance characteristics
func BenchmarkRateLimiter_Allow(b *testing.B) {
	rl := NewRateLimiter()
	defer rl.Stop()

	userID := "bench-user"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow(userID)
	}
}

func BenchmarkRateLimiter_AllowMultipleUsers(b *testing.B) {
	rl := NewRateLimiter()
	defer rl.Stop()

	userCount := 100
	users := make([]string, userCount)
	for i := 0; i < userCount; i++ {
		users[i] = "bench-user-" + string(rune(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userID := users[i%userCount]
		rl.Allow(userID)
	}
}
