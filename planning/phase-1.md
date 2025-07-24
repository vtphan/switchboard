# Phase 1: Foundation Layer

## Overview
Establishes core types, interfaces, and database schema. Provides the fundamental building blocks for all other components while preventing circular dependencies through clean interface definitions.

## Step 1.1: Core Types and Data Models (Estimated: 1.5h)

### EXACT REQUIREMENTS (do not exceed scope):
- Define all core structs: Session, Message, Client, ConnectionManager
- Implement JSON serialization/deserialization with proper tags
- Add validation methods for each type (ValidateUserID, ValidateSessionName, etc.)
- Define exactly 6 message types as constants: instructor_inbox, inbox_response, request, request_response, analytics, instructor_broadcast
- Context field validation: 1-50 chars, alphanumeric + underscore/hyphen, defaults to "general"
- Message content size limit: exactly 64KB
- User ID validation: 1-50 chars, alphanumeric + underscore/hyphen only

### ARCHITECTURAL VALIDATION REQUIREMENTS:
**Dependencies** (BLOCKING):
- Only import: standard library packages (encoding/json, time, errors, regexp)
- No imports from: internal/websocket, internal/session, internal/router (prevent circular deps)
- Verify: `go mod graph | grep -E "cycle|circular"` returns empty

**Boundaries** (BLOCKING):
- No business logic in types (no session creation, message routing, connection handling)
- Pure data structures with validation only
- Clean separation: types vs operations

**Integration Contracts** (BLOCKING):
- All other phases can safely import pkg/types without circular dependencies
- Validation methods return specific error types for proper error handling
- JSON marshaling works correctly for API endpoints and message serialization

### FUNCTIONAL VALIDATION REQUIREMENTS:
**Core Behaviors** (BLOCKING):
- Session.Validate() enforces: name 1-200 chars, non-empty student_ids, valid created_by
- Message.Validate() enforces: valid type, content ≤64KB, context 1-50 chars or defaults to "general"
- Client.ValidateUserID() enforces: 1-50 chars, alphanumeric + underscore/hyphen only
- Message type constants match exactly the 6 types from specs
- JSON marshaling preserves all fields correctly

**Error Handling** (BLOCKING):
- Validation errors return specific error types (ErrInvalidUserID, ErrInvalidMessageType, etc.)
- Empty/nil input validation returns appropriate errors
- Invalid JSON during unmarshal returns clear error messages

**Integration Contracts** (BLOCKING):
- Session struct matches database schema exactly (same field names and types)
- Message struct supports all 6 message routing patterns
- Client struct provides all fields needed for connection management and rate limiting

### TECHNICAL VALIDATION REQUIREMENTS:
**Performance** (WARNING):
- Validation methods complete in <1ms for typical inputs
- JSON marshaling handles 64KB messages efficiently
- Memory usage reasonable for concurrent access

**Code Quality** (WARNING):
- All exported types and methods have documentation comments
- Consistent naming conventions throughout
- Static analysis clean (go vet, golint)

### MANDATORY TYPES (implement exactly):
```go
package types

import (
    "encoding/json"
    "time"
)

// Message types - exactly these 6 constants
const (
    MessageTypeInstructorInbox     = "instructor_inbox"
    MessageTypeInboxResponse       = "inbox_response"  
    MessageTypeRequest             = "request"
    MessageTypeRequestResponse     = "request_response"
    MessageTypeAnalytics           = "analytics"
    MessageTypeInstructorBroadcast = "instructor_broadcast"
)

// Session represents an educational session
type Session struct {
    ID         string    `json:"id" db:"id"`
    Name       string    `json:"name" db:"name"`
    CreatedBy  string    `json:"created_by" db:"created_by"`
    StudentIDs []string  `json:"student_ids" db:"student_ids"`
    StartTime  time.Time `json:"start_time" db:"start_time"`
    EndTime    *time.Time `json:"end_time,omitempty" db:"end_time"`
    Status     string    `json:"status" db:"status"`
}

// Message represents a communication message
type Message struct {
    ID        string                 `json:"id"`
    SessionID string                 `json:"session_id"`
    Type      string                 `json:"type"`
    Context   string                 `json:"context"`
    FromUser  string                 `json:"from_user"`
    ToUser    *string                `json:"to_user,omitempty"`
    Content   map[string]interface{} `json:"content"`
    Timestamp time.Time              `json:"timestamp"`
}

// Client represents a connected WebSocket client
type Client struct {
    ID            string          `json:"id"`
    Role          string          `json:"role"`
    SessionID     string          `json:"session_id"`
    SendChannel   chan Message    `json:"-"`
    LastHeartbeat time.Time       `json:"last_heartbeat"`
    MessageCount  int             `json:"message_count"`
    WindowStart   time.Time       `json:"window_start"`
    CleanedUp     bool            `json:"cleaned_up"`
}

// ConnectionManager manages client connections
type ConnectionManager struct {
    GlobalConnections   map[string]*Client            `json:"global_connections"`
    SessionInstructors  map[string]map[string]*Client `json:"session_instructors"`
    SessionStudents     map[string]map[string]*Client `json:"session_students"`
}
```

### CRITICAL VALIDATION PATTERNS (must follow):
```go
// Session validation
func (s *Session) Validate() error {
    if len(s.Name) < 1 || len(s.Name) > 200 {
        return ErrInvalidSessionName
    }
    if len(s.StudentIDs) == 0 {
        return ErrEmptyStudentList
    }
    if !IsValidUserID(s.CreatedBy) {
        return ErrInvalidCreatedBy
    }
    return nil
}

// Message validation
func (m *Message) Validate() error {
    if !IsValidMessageType(m.Type) {
        return ErrInvalidMessageType
    }
    if m.Context == "" {
        m.Context = "general"  // Default value
    }
    if !IsValidContext(m.Context) {
        return ErrInvalidContext
    }
    
    // Check content size (64KB limit)
    contentBytes, err := json.Marshal(m.Content)
    if err != nil {
        return ErrInvalidContent
    }
    if len(contentBytes) > 65536 {  // 64KB = 65536 bytes
        return ErrContentTooLarge
    }
    return nil
}

// User ID validation
func IsValidUserID(userID string) bool {
    if len(userID) < 1 || len(userID) > 50 {
        return false
    }
    // Alphanumeric + underscore/hyphen only
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, userID)
    return matched
}
```

### ERROR TYPES (define exactly these):
```go
var (
    ErrInvalidUserID       = errors.New("user ID must be 1-50 characters, alphanumeric + underscore/hyphen only")
    ErrInvalidSessionName  = errors.New("session name must be 1-200 characters")
    ErrEmptyStudentList    = errors.New("student list cannot be empty")
    ErrInvalidCreatedBy    = errors.New("created_by must be valid user ID")
    ErrInvalidMessageType  = errors.New("invalid message type")
    ErrInvalidContext      = errors.New("context must be 1-50 characters, alphanumeric + underscore/hyphen")
    ErrInvalidContent      = errors.New("invalid JSON content")
    ErrContentTooLarge     = errors.New("message content exceeds 64KB limit")
)
```

### SUCCESS CRITERIA:
**Architectural** (BLOCKING):
- [ ] No circular dependencies: `go mod graph | grep cycle` empty
- [ ] Clean imports: Only standard library packages
- [ ] No business logic: Pure data structures with validation only
- [ ] Safe for concurrent import by all other phases

**Functional** (BLOCKING):
- [ ] All 6 message type constants defined correctly
- [ ] Session.Validate() enforces all spec requirements
- [ ] Message.Validate() handles content size limit and context defaults
- [ ] IsValidUserID() enforces format requirements correctly
- [ ] JSON marshaling preserves all fields

**Technical** (WARNING):
- [ ] All validation methods complete in <1ms
- [ ] Documentation comments on all exported types
- [ ] `go vet ./pkg/types` passes without warnings
- [ ] Coverage ≥85% statements

### INTEGRATION CONTRACTS:
**What Step 1.2 (Interfaces) expects:**
- Client struct with all fields needed for connection interface
- Message struct supporting all routing patterns
- Error types for proper interface error handling

**What Step 1.3 (Database) expects:**
- Session struct with db tags matching exact schema
- Message struct with fields matching database columns
- JSON serialization working for student_ids array and content

### READ ONLY THESE REFERENCES:
- switchboard-tech-specs.md lines 94-143 (data models)
- switchboard-tech-specs.md lines 146-158 (message types and context)
- switchboard-tech-specs.md lines 593-600 (validation rules)
- switchboard-tech-specs.md lines 339-343 (essential limits)

### IGNORE EVERYTHING ELSE
Do not read sections about WebSocket handling, routing logic, or session management. Focus only on data structure definitions and validation.

### FILES TO CREATE:
- pkg/types/types.go (all struct definitions)
- pkg/types/validation.go (validation methods and helper functions)
- pkg/types/errors.go (error type definitions)
- pkg/types/types_test.go (comprehensive validation tests)

---

## Step 1.2: Interface Definitions (Estimated: 1h)

### EXACT REQUIREMENTS (do not exceed scope):
- Define Connection interface for WebSocket connections
- Define SessionManager interface for session operations  
- Define MessageRouter interface for message routing
- Define DatabaseManager interface for persistence operations
- All interfaces must be minimal and focused on single responsibility
- No implementation - interfaces only to prevent circular dependencies

### ARCHITECTURAL VALIDATION REQUIREMENTS:
**Dependencies** (BLOCKING):
- Only import: pkg/types, standard library (context)
- No imports from: internal/websocket, internal/session, internal/router
- Verify: `go mod graph | grep -E "cycle|circular"` returns empty

**Boundaries** (BLOCKING):
- Pure interface definitions with no implementation
- Each interface focused on single responsibility
- Clean abstraction boundaries between components

**Integration Contracts** (BLOCKING):
- All implementation phases can implement these interfaces
- Interface signatures match exactly what implementations will provide
- No leaky abstractions - interfaces don't expose internal details

### FUNCTIONAL VALIDATION REQUIREMENTS:
**Core Behaviors** (BLOCKING):
- Connection interface supports thread-safe WebSocket operations
- SessionManager interface supports all session lifecycle operations
- MessageRouter interface handles all 6 message types correctly
- DatabaseManager interface provides all needed persistence operations

**Error Handling** (BLOCKING):
- All interface methods return appropriate error types
- Error signatures consistent with implementation needs
- Context cancellation supported where appropriate

**Integration Contracts** (BLOCKING):
- Interface methods match expected usage patterns from specs
- Return types align with consumer needs
- Method signatures support all required functionality

### TECHNICAL VALIDATION REQUIREMENTS:
**Code Quality** (WARNING):
- All interfaces have comprehensive documentation
- Method signatures are clear and unambiguous
- Consistent naming conventions across interfaces

### MANDATORY INTERFACES (implement exactly):
```go
package interfaces

import (
    "context"
    "github.com/switchboard/pkg/types"
)

// Connection represents a WebSocket client connection
type Connection interface {
    // WriteJSON sends a JSON message to the client (thread-safe)
    WriteJSON(v interface{}) error
    
    // Close closes the connection and cleans up resources
    Close() error
    
    // GetUserID returns the connected user's ID
    GetUserID() string
    
    // GetRole returns the user's role ("student" or "instructor")
    GetRole() string
    
    // GetSessionID returns the session ID this connection belongs to
    GetSessionID() string
    
    // IsAuthenticated returns true if connection is authenticated
    IsAuthenticated() bool
    
    // SetCredentials sets user credentials after authentication
    SetCredentials(userID, role, sessionID string) error
}

// SessionManager handles session lifecycle operations
type SessionManager interface {
    // CreateSession creates a new session
    CreateSession(ctx context.Context, name string, createdBy string, studentIDs []string) (*types.Session, error)
    
    // GetSession retrieves a session by ID
    GetSession(ctx context.Context, sessionID string) (*types.Session, error)
    
    // EndSession ends an active session
    EndSession(ctx context.Context, sessionID string) error
    
    // ListActiveSessions returns all active sessions
    ListActiveSessions(ctx context.Context) ([]*types.Session, error)
    
    // ValidateSessionMembership checks if user can join session
    ValidateSessionMembership(sessionID, userID, role string) error
}

// MessageRouter handles message routing between clients
type MessageRouter interface {
    // RouteMessage routes a message to appropriate recipients
    RouteMessage(ctx context.Context, message *types.Message) error
    
    // GetRecipients determines recipients for a message
    GetRecipients(message *types.Message) ([]*types.Client, error)
    
    // ValidateMessage validates message content and permissions
    ValidateMessage(message *types.Message, sender *types.Client) error
}

// DatabaseManager handles all database operations
type DatabaseManager interface {
    // Sessions
    CreateSession(ctx context.Context, session *types.Session) error
    GetSession(ctx context.Context, sessionID string) (*types.Session, error)
    UpdateSession(ctx context.Context, session *types.Session) error
    ListActiveSessions(ctx context.Context) ([]*types.Session, error)
    
    // Messages  
    StoreMessage(ctx context.Context, message *types.Message) error
    GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error)
    
    // Health
    HealthCheck(ctx context.Context) error
    
    // Lifecycle
    Close() error
}
```

### SUCCESS CRITERIA:
**Architectural** (BLOCKING):
- [ ] No circular dependencies: interfaces safely importable by all phases
- [ ] Clean boundaries: each interface focused on single responsibility
- [ ] No implementation details: pure abstractions only
- [ ] Import safety: only pkg/types and standard library

**Functional** (BLOCKING):
- [ ] Connection interface supports all WebSocket operations needed
- [ ] SessionManager interface covers complete session lifecycle
- [ ] MessageRouter interface handles all 6 message types
- [ ] DatabaseManager interface provides all persistence needs

**Technical** (WARNING):
- [ ] Comprehensive documentation on all interfaces and methods
- [ ] Consistent error handling patterns across interfaces
- [ ] Method signatures clear and unambiguous

### INTEGRATION CONTRACTS:
**What Step 2.1 (Connection Wrapper) expects:**
- Connection interface with exact method signatures shown
- Error handling patterns for network operations
- Thread-safety requirements clearly documented

**What Step 2.2 (Session Manager) expects:**
- SessionManager interface with context support
- Database operations interface for persistence
- Clear validation method signatures

### READ ONLY THESE REFERENCES:
- switchboard-tech-specs.md lines 63-90 (core components)
- switchboard-tech-specs.md lines 214-238 (client connection algorithm)
- switchboard-tech-specs.md lines 276-302 (message validation algorithm)

### IGNORE EVERYTHING ELSE
Do not read implementation details. Focus only on what operations each component needs to provide.

### FILES TO CREATE:
- pkg/interfaces/connection.go (Connection interface)
- pkg/interfaces/session.go (SessionManager interface)
- pkg/interfaces/router.go (MessageRouter interface)
- pkg/interfaces/database.go (DatabaseManager interface)
- pkg/interfaces/interfaces_test.go (interface compliance tests)

---

## Step 1.3: Database Schema and Configuration (Estimated: 1.5h)

### EXACT REQUIREMENTS (do not exceed scope):
- Create exactly the SQL schema from specs (sessions and messages tables)
- Add exactly the specified indexes for performance
- Create database migration system for schema updates
- Implement database connection configuration with proper settings
- Add schema validation to ensure database matches expected structure
- SQLite-specific optimizations for single-writer, multi-reader pattern

### ARCHITECTURAL VALIDATION REQUIREMENTS:
**Dependencies** (BLOCKING):
- Only import: database/sql, sqlite3 driver, pkg/types, standard library
- No imports from: internal/websocket, internal/session, internal/router
- Verify: `go mod graph | grep -E "cycle|circular"` returns empty

**Boundaries** (BLOCKING):
- Pure database schema and configuration - no business logic
- No WebSocket handling, session management, or routing logic
- Clean separation: schema vs database operations

**Integration Contracts** (BLOCKING):
- Schema matches pkg/types struct definitions exactly
- Database configuration supports all needed operations
- Migration system allows safe schema evolution

### FUNCTIONAL VALIDATION REQUIREMENTS:
**Core Behaviors** (BLOCKING):
- Sessions table matches Session struct exactly (same field names, types)
- Messages table supports all 6 message types and routing patterns
- Indexes provide efficient lookup for session history queries
- Foreign key constraints maintain data integrity
- Check constraints enforce business rules (status values, context length)

**Error Handling** (BLOCKING):
- Database connection errors handled gracefully
- Migration failures rollback cleanly
- Schema validation detects mismatches clearly
- Connection timeout and retry logic included

**Integration Contracts** (BLOCKING):
- Database configuration supports single-writer pattern needed by DB Manager
- Schema supports all operations needed by SessionManager and MessageRouter
- Connection settings optimized for classroom-scale concurrent access

### TECHNICAL VALIDATION REQUIREMENTS:
**Performance** (WARNING):
- Indexes provide efficient session history retrieval (<100ms for 1000 messages)
- Connection pool configured for concurrent read access
- SQLite optimizations for classroom-scale usage (20-50 concurrent users)

**Code Quality** (WARNING):
- Migration files properly versioned and documented
- Schema validation comprehensive and clear
- Database configuration externalized and configurable

### MANDATORY SCHEMA (implement exactly):
```sql
-- Version 001: Initial schema
-- File: migrations/001_initial_schema.sql

-- Sessions table
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_by TEXT NOT NULL,
    student_ids TEXT NOT NULL, -- JSON array of strings
    start_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    end_time DATETIME,
    status TEXT NOT NULL DEFAULT 'active',
    CHECK (status IN ('active', 'ended')),
    CHECK (length(name) >= 1 AND length(name) <= 200)
);

-- Messages table  
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    type TEXT NOT NULL,
    context TEXT NOT NULL DEFAULT 'general',
    from_user TEXT NOT NULL,
    to_user TEXT, -- NULL for broadcasts
    content TEXT NOT NULL, -- JSON, max 64KB enforced by application
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    CHECK (type IN ('instructor_inbox', 'inbox_response', 'request', 'request_response', 'analytics', 'instructor_broadcast')),
    CHECK (length(context) >= 1 AND length(context) <= 50)
);

-- Performance indexes
CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_sessions_created_by ON sessions(created_by);
CREATE INDEX idx_messages_session_time ON messages(session_id, timestamp);
CREATE INDEX idx_messages_session_type ON messages(session_id, type);
CREATE INDEX idx_messages_to_user ON messages(to_user) WHERE to_user IS NOT NULL;
```

### CRITICAL DATABASE CONFIGURATION (must follow):
```go
package database

import (
    "database/sql"
    "time"
    _ "github.com/mattn/go-sqlite3"
)

// Config holds database configuration
type Config struct {
    DatabasePath    string        `json:"database_path"`
    MaxConnections  int           `json:"max_connections"`
    ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
    ConnMaxIdleTime time.Duration `json:"conn_max_idle_time"`
    MigrationsPath  string        `json:"migrations_path"`
}

// DefaultConfig returns production-ready database configuration
func DefaultConfig() *Config {
    return &Config{
        DatabasePath:    "./data/switchboard.db",
        MaxConnections:  10,  // SQLite recommended limit
        ConnMaxLifetime: time.Hour,
        ConnMaxIdleTime: time.Minute * 10,
        MigrationsPath:  "./migrations",
    }
}

// SQLite optimization pragmas for classroom scale
const sqliteOptimizations = `
    PRAGMA journal_mode = WAL;          -- Write-Ahead Logging for better concurrency
    PRAGMA synchronous = NORMAL;        -- Balance between safety and performance  
    PRAGMA cache_size = -64000;         -- 64MB cache (negative = KB)
    PRAGMA temp_store = MEMORY;         -- Use memory for temporary tables
    PRAGMA foreign_keys = ON;           -- Enforce foreign key constraints
    PRAGMA busy_timeout = 5000;         -- 5 second timeout for locked database
`
```

### MIGRATION SYSTEM (implement exactly):
```go
// Migration represents a database migration
type Migration struct {
    Version     string
    Description string
    SQL         string
}

// MigrationManager handles database migrations
type MigrationManager struct {
    db           *sql.DB
    migrationsPath string
}

// ApplyMigrations applies all pending migrations
func (m *MigrationManager) ApplyMigrations() error {
    // Create migrations tracking table if needed
    // Read migration files from directory
    // Apply migrations in order
    // Track applied migrations
    // Rollback on failure
}

// ValidateSchema ensures database matches expected structure
func (m *MigrationManager) ValidateSchema() error {
    // Check table existence and structure
    // Verify indexes are present
    // Validate constraints are enforced
    // Return detailed errors for mismatches
}
```

### SUCCESS CRITERIA:
**Architectural** (BLOCKING):
- [ ] No circular dependencies: safe import by all phases needing database
- [ ] Clean separation: schema and config only, no business operations
- [ ] Struct alignment: database schema matches pkg/types exactly
- [ ] Migration safety: rollback on failure, version tracking

**Functional** (BLOCKING):
- [ ] Sessions table supports all session operations from specs
- [ ] Messages table handles all 6 message types correctly
- [ ] Foreign keys maintain referential integrity
- [ ] Check constraints enforce business rules (status values, lengths)
- [ ] Indexes provide efficient query performance

**Technical** (WARNING):
- [ ] SQLite optimizations applied for classroom-scale performance
- [ ] Connection pool configured appropriately (10 connections max)
- [ ] Migration system handles errors gracefully
- [ ] Schema validation detects structural issues

### INTEGRATION CONTRACTS:
**What Step 2.3 (Database Manager) expects:**
- Database configured with single-writer optimizations
- Schema supporting all CRUD operations on sessions and messages
- Migration system for safe schema evolution
- Connection configuration ready for production use

**What all phases expect:**
- Database schema matches types.Session and types.Message exactly
- Indexes support efficient session history retrieval
- Configuration externalized for different environments

### READ ONLY THESE REFERENCES:
- switchboard-tech-specs.md lines 347-398 (database design and schema)
- switchboard-tech-specs.md lines 94-119 (Session and Message data models)
- switchboard-tech-specs.md lines 382-386 (concurrency strategy)

### IGNORE EVERYTHING ELSE
Do not read sections about WebSocket handling, message routing, or session management logic. Focus only on database structure and configuration.

### FILES TO CREATE:
- pkg/database/config.go (database configuration)
- pkg/database/migrations.go (migration system)
- pkg/database/schema.go (schema validation)
- migrations/001_initial_schema.sql (SQL schema file)
- pkg/database/database_test.go (schema and migration tests)