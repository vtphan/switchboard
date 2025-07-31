package config

import "time"

// Core system configuration constants
const (
    // Connection Management
    ConnectionSendBufferSize     = 100
    MaxMessageSize              = 64 * 1024
    HeartbeatInterval           = 30 * time.Second
    InactiveConnectionTimeout   = 25 * time.Minute  // Timeout for inactive connections when no session active
    ConnectionCleanupInterval   = 30 * time.Second  // How often to check for inactive connections
    
    // Database Performance
    DatabaseBatchSize           = 100
    DatabaseFlushInterval       = 200 * time.Millisecond
    DatabaseWriteBuffer         = 100
    DatabaseMaxRetries          = 3
    DeadLetterQueueSize         = 50
    
    // Rate Limiting
    RateLimitMaxMessages        = 100
    RateLimitWindow             = time.Minute
    RateLimitCleanupInterval    = 5 * time.Minute
    
    // Graceful Shutdown Timeouts
    MessageProcessingTimeout    = 2 * time.Second
    GoroutineCleanupTimeout     = 1 * time.Second
)

// SQLite optimization configuration
const SQLiteConfiguration = `
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = -64000;
PRAGMA wal_autocheckpoint = 1000;
`