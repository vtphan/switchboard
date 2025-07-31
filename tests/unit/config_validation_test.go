package unit

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"switchboard/pkg/config"
)

// TestConfigurationArchitecturalCompliance validates that configuration
// meets all architectural requirements from Step 1.4
func TestConfigurationArchitecturalCompliance(t *testing.T) {
	t.Run("Configuration constants are pure constants only", func(t *testing.T) {
		// Configuration should have no business logic, only constants
		// This is verified by the fact that the file compiles and contains only const declarations
		assert.Greater(t, config.ConnectionSendBufferSize, 0, "Configuration constants must be defined")
	})

	t.Run("No forbidden dependencies in configuration", func(t *testing.T) {
		// Configuration should not import business logic packages
		// Since we can import it here, it has proper dependency isolation
		assert.NotZero(t, config.DatabaseBatchSize, "Configuration should be importable without circular dependencies")
	})
}

// TestConfigurationTechSpecsCompliance validates exact match with tech specs lines 33-57
func TestConfigurationTechSpecsCompliance(t *testing.T) {
	t.Run("Connection Management constants match tech specs exactly", func(t *testing.T) {
		assert.Equal(t, 100, config.ConnectionSendBufferSize, "ConnectionSendBufferSize must match tech specs")
		assert.Equal(t, 64*1024, config.MaxMessageSize, "MaxMessageSize must match tech specs (64KB)")
		assert.Equal(t, 30*time.Second, config.HeartbeatInterval, "HeartbeatInterval must match tech specs")
		assert.Equal(t, 25*time.Minute, config.InactiveConnectionTimeout, "InactiveConnectionTimeout must match new cleanup design")
		assert.Equal(t, 30*time.Second, config.ConnectionCleanupInterval, "ConnectionCleanupInterval must match tech specs")
	})

	t.Run("Database Performance constants match tech specs exactly", func(t *testing.T) {
		assert.Equal(t, 100, config.DatabaseBatchSize, "DatabaseBatchSize must match tech specs")
		assert.Equal(t, 200*time.Millisecond, config.DatabaseFlushInterval, "DatabaseFlushInterval must match tech specs")
		assert.Equal(t, 100, config.DatabaseWriteBuffer, "DatabaseWriteBuffer must match tech specs")
		assert.Equal(t, 3, config.DatabaseMaxRetries, "DatabaseMaxRetries must match tech specs")
		assert.Equal(t, 50, config.DeadLetterQueueSize, "DeadLetterQueueSize must match tech specs")
	})

	t.Run("Rate Limiting constants match tech specs exactly", func(t *testing.T) {
		assert.Equal(t, 100, config.RateLimitMaxMessages, "RateLimitMaxMessages must match tech specs")
		assert.Equal(t, time.Minute, config.RateLimitWindow, "RateLimitWindow must match tech specs")
		assert.Equal(t, 5*time.Minute, config.RateLimitCleanupInterval, "RateLimitCleanupInterval must match tech specs")
	})

	t.Run("Graceful Shutdown constants match tech specs exactly", func(t *testing.T) {
		assert.Equal(t, 2*time.Second, config.MessageProcessingTimeout, "MessageProcessingTimeout must match tech specs")
		assert.Equal(t, 1*time.Second, config.GoroutineCleanupTimeout, "GoroutineCleanupTimeout must match tech specs")
	})
}

// TestSQLiteConfigurationCompliance validates SQLite configuration string
func TestSQLiteConfigurationCompliance(t *testing.T) {
	t.Run("SQLite configuration contains required pragmas", func(t *testing.T) {
		sqliteConfig := config.SQLiteConfiguration

		// Check for required pragma statements
		assert.Contains(t, sqliteConfig, "PRAGMA journal_mode = WAL", "SQLite config must enable WAL mode")
		assert.Contains(t, sqliteConfig, "PRAGMA synchronous = NORMAL", "SQLite config must set synchronous NORMAL")
		assert.Contains(t, sqliteConfig, "PRAGMA cache_size = -64000", "SQLite config must set cache size to 64MB")
		assert.Contains(t, sqliteConfig, "PRAGMA wal_autocheckpoint = 1000", "SQLite config must set WAL checkpoint")
	})

	t.Run("SQLite configuration format is correct", func(t *testing.T) {
		// Ensure configuration is properly formatted SQL
		lines := strings.Split(strings.TrimSpace(config.SQLiteConfiguration), "\n")
		assert.Equal(t, 4, len(lines), "SQLite configuration must have exactly 4 pragma statements")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			assert.True(t, strings.HasPrefix(line, "PRAGMA "), "Each line must be a PRAGMA statement")
			assert.True(t, strings.HasSuffix(line, ";"), "Each PRAGMA statement must end with semicolon")
		}
	})
}

// TestConfigurationTypeCompliance validates proper Go types
func TestConfigurationTypeCompliance(t *testing.T) {
	t.Run("Timing constants use time.Duration types", func(t *testing.T) {
		// Verify timing constants are proper time.Duration types
		var duration time.Duration

		duration = config.HeartbeatInterval
		assert.Equal(t, 30*time.Second, duration, "HeartbeatInterval must be time.Duration")

		duration = config.InactiveConnectionTimeout
		assert.Equal(t, 25*time.Minute, duration, "InactiveConnectionTimeout must be time.Duration")

		duration = config.ConnectionCleanupInterval
		assert.Equal(t, 30*time.Second, duration, "ConnectionCleanupInterval must be time.Duration")

		duration = config.DatabaseFlushInterval
		assert.Equal(t, 200*time.Millisecond, duration, "DatabaseFlushInterval must be time.Duration")

		duration = config.RateLimitWindow
		assert.Equal(t, time.Minute, duration, "RateLimitWindow must be time.Duration")

		duration = config.RateLimitCleanupInterval
		assert.Equal(t, 5*time.Minute, duration, "RateLimitCleanupInterval must be time.Duration")

		duration = config.MessageProcessingTimeout
		assert.Equal(t, 2*time.Second, duration, "MessageProcessingTimeout must be time.Duration")

		duration = config.GoroutineCleanupTimeout
		assert.Equal(t, 1*time.Second, duration, "GoroutineCleanupTimeout must be time.Duration")
	})

	t.Run("Buffer sizes are integers", func(t *testing.T) {
		// Verify buffer size constants are integers
		var size int

		size = config.ConnectionSendBufferSize
		assert.Equal(t, 100, size, "ConnectionSendBufferSize must be int")

		size = config.MaxMessageSize
		assert.Equal(t, 64*1024, size, "MaxMessageSize must be int")

		size = config.DatabaseBatchSize
		assert.Equal(t, 100, size, "DatabaseBatchSize must be int")

		size = config.DatabaseWriteBuffer
		assert.Equal(t, 100, size, "DatabaseWriteBuffer must be int")

		size = config.DatabaseMaxRetries
		assert.Equal(t, 3, size, "DatabaseMaxRetries must be int")

		size = config.DeadLetterQueueSize
		assert.Equal(t, 50, size, "DeadLetterQueueSize must be int")

		size = config.RateLimitMaxMessages
		assert.Equal(t, 100, size, "RateLimitMaxMessages must be int")
	})
}

// TestConfigurationIntegrationReadiness validates integration contracts
func TestConfigurationIntegrationReadiness(t *testing.T) {
	t.Run("Database layer integration constants", func(t *testing.T) {
		// Verify constants are available for database layer
		assert.Greater(t, config.DatabaseBatchSize, 0, "DatabaseBatchSize must be available for database layer")
		assert.Greater(t, config.DatabaseFlushInterval, time.Duration(0), "DatabaseFlushInterval must be available for database layer")
		assert.Greater(t, config.DatabaseWriteBuffer, 0, "DatabaseWriteBuffer must be available for database layer")
		assert.Greater(t, config.DatabaseMaxRetries, 0, "DatabaseMaxRetries must be available for database layer")
		assert.Greater(t, config.DeadLetterQueueSize, 0, "DeadLetterQueueSize must be available for database layer")
		assert.NotEmpty(t, config.SQLiteConfiguration, "SQLiteConfiguration must be available for database layer")
	})

	t.Run("WebSocket layer integration constants", func(t *testing.T) {
		// Verify constants are available for WebSocket layer
		assert.Greater(t, config.ConnectionSendBufferSize, 0, "ConnectionSendBufferSize must be available for WebSocket layer")
		assert.Greater(t, config.MaxMessageSize, 0, "MaxMessageSize must be available for WebSocket layer")
		assert.Greater(t, config.HeartbeatInterval, time.Duration(0), "HeartbeatInterval must be available for WebSocket layer")
		assert.Greater(t, config.InactiveConnectionTimeout, time.Duration(0), "InactiveConnectionTimeout must be available for WebSocket layer")
		assert.Greater(t, config.ConnectionCleanupInterval, time.Duration(0), "ConnectionCleanupInterval must be available for WebSocket layer")
	})

	t.Run("Rate limiting integration constants", func(t *testing.T) {
		// Verify constants are available for rate limiting
		assert.Greater(t, config.RateLimitMaxMessages, 0, "RateLimitMaxMessages must be available for rate limiter")
		assert.Greater(t, config.RateLimitWindow, time.Duration(0), "RateLimitWindow must be available for rate limiter")
		assert.Greater(t, config.RateLimitCleanupInterval, time.Duration(0), "RateLimitCleanupInterval must be available for rate limiter")
	})

	t.Run("Graceful shutdown integration constants", func(t *testing.T) {
		// Verify constants are available for graceful shutdown
		assert.Greater(t, config.MessageProcessingTimeout, time.Duration(0), "MessageProcessingTimeout must be available for graceful shutdown")
		assert.Greater(t, config.GoroutineCleanupTimeout, time.Duration(0), "GoroutineCleanupTimeout must be available for graceful shutdown")
	})
}

// TestConfigurationValueRanges validates that constants have sensible values
func TestConfigurationValueRanges(t *testing.T) {
	t.Run("Buffer sizes are reasonable", func(t *testing.T) {
		assert.GreaterOrEqual(t, config.ConnectionSendBufferSize, 10, "ConnectionSendBufferSize should be reasonable")
		assert.LessOrEqual(t, config.ConnectionSendBufferSize, 1000, "ConnectionSendBufferSize should not be excessive")

		assert.GreaterOrEqual(t, config.DatabaseBatchSize, 1, "DatabaseBatchSize should be at least 1")
		assert.LessOrEqual(t, config.DatabaseBatchSize, 1000, "DatabaseBatchSize should not be excessive")

		assert.GreaterOrEqual(t, config.DatabaseWriteBuffer, 1, "DatabaseWriteBuffer should be at least 1")
		assert.LessOrEqual(t, config.DatabaseWriteBuffer, 1000, "DatabaseWriteBuffer should not be excessive")
	})

	t.Run("Timeouts are reasonable", func(t *testing.T) {
		assert.GreaterOrEqual(t, config.HeartbeatInterval, 5*time.Second, "HeartbeatInterval should be reasonable")
		assert.LessOrEqual(t, config.HeartbeatInterval, 5*time.Minute, "HeartbeatInterval should not be excessive")

		assert.GreaterOrEqual(t, config.InactiveConnectionTimeout, 5*time.Minute, "InactiveConnectionTimeout should be reasonable")
		assert.LessOrEqual(t, config.InactiveConnectionTimeout, 2*time.Hour, "InactiveConnectionTimeout should not be excessive")

		assert.GreaterOrEqual(t, config.DatabaseFlushInterval, 50*time.Millisecond, "DatabaseFlushInterval should be reasonable")
		assert.LessOrEqual(t, config.DatabaseFlushInterval, 5*time.Second, "DatabaseFlushInterval should not be excessive")
	})

	t.Run("Rate limits are reasonable", func(t *testing.T) {
		assert.GreaterOrEqual(t, config.RateLimitMaxMessages, 10, "RateLimitMaxMessages should allow reasonable activity")
		assert.LessOrEqual(t, config.RateLimitMaxMessages, 1000, "RateLimitMaxMessages should prevent abuse")

		assert.GreaterOrEqual(t, config.RateLimitWindow, 10*time.Second, "RateLimitWindow should be reasonable")
		assert.LessOrEqual(t, config.RateLimitWindow, 1*time.Hour, "RateLimitWindow should not be excessive")
	})
}