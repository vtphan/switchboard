# Phase 4: Session Management System

## Overview
Implements session lifecycle management, validation, and coordination with database operations. Provides the SessionManager interface implementation and handles session-scoped operations like member validation and connection cleanup.

## Step 4.1: Session Manager Implementation (Estimated: 2.5h)

### EXACT REQUIREMENTS (do not exceed scope):
- Implement SessionManager interface with all CRUD operations
- Session validation: students must be in student_ids list, instructors have universal access
- Immutable sessions: no modifications to student_ids after creation
- Active session tracking in memory for efficient validation
- Integration with DatabaseManager for persistence operations
- UUID generation for session IDs, automatic timestamp handling

### ARCHITECTURAL VALIDATION REQUIREMENTS:
**Dependencies** (BLOCKING):
- Only import: pkg/interfaces, pkg/types, standard library (context, time, sync)
- No imports from: internal/websocket, internal/router, internal/hub (avoid coupling)
- Verify: `go mod graph | grep -E "cycle|circular"` returns empty

**Boundaries** (BLOCKING):
- Pure session management logic - no WebSocket handling, message routing, or database implementation
- Clean separation: session operations vs database operations vs connection handling
- Interface compliance: exact implementation of SessionManager interface

**Integration Contracts** (BLOCKING):
- Uses DatabaseManager interface for all persistence operations
- Provides session validation for WebSocket authentication
- Maintains in-memory session cache for efficient validation
- Coordinates with connection cleanup during session termination

### FUNCTIONAL VALIDATION REQUIREMENTS:
**Core Behaviors** (BLOCKING):
- CreateSession() generates unique ID, validates input, persists to database
- GetSession() returns session with current status from cache or database
- EndSession() updates database, cleans up connections, removes from cache
- ValidateSessionMembership() enforces role-based access rules correctly
- ListActiveSessions() returns only sessions with status='active'

**Error Handling** (BLOCKING):
- Duplicate student IDs automatically removed during creation
- Invalid session data returns ErrInvalidSessionData with details
- Non-existent session returns ErrSessionNotFound
- Ended session access returns ErrSessionEnded
- Database errors wrapped and returned with context

**Integration Contracts** (BLOCKING):
- Database operations use provided DatabaseManager interface
- Session cache maintained for efficient repeated validation
- Session termination coordinates with connection cleanup
- Thread-safe operations for concurrent access

### TECHNICAL VALIDATION REQUIREMENTS:
**Performance** (WARNING):
- Session validation completes in <1ms using in-memory cache
- Session creation completes in <100ms including database write
- Memory usage efficient for tracking active sessions (typically <10 sessions)

**Concurrency** (WARNING):
- Thread-safe session cache operations
- Atomic session creation (ID generation, validation, persistence)
- No race conditions in session state transitions

### MANDATORY INTERFACE (implement exactly):
```go
package session

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/switchboard/pkg/interfaces"
    "github.com/switchboard/pkg/types"
)

// Manager implements the SessionManager interface
type Manager struct {
    dbManager     interfaces.DatabaseManager
    activeSessions map[string]*types.Session // sessionID -> Session
    mu            sync.RWMutex
}

// NewManager creates a new session manager
func NewManager(dbManager interfaces.DatabaseManager) *Manager {
    return &Manager{
        dbManager:      dbManager,
        activeSessions: make(map[string]*types.Session),
    }
}

// LoadActiveSessions loads all active sessions from database into memory
func (m *Manager) LoadActiveSessions(ctx context.Context) error {
    sessions, err := m.dbManager.ListActiveSessions(ctx)
    if err != nil {
        return fmt.Errorf("failed to load active sessions: %w", err)
    }
    
    m.mu.Lock()
    defer m.mu.Unlock()
    
    for _, session := range sessions {
        m.activeSessions[session.ID] = session
    }
    
    log.Printf("Loaded %d active sessions", len(sessions))
    return nil
}

// CreateSession creates a new session
func (m *Manager) CreateSession(ctx context.Context, name string, createdBy string, studentIDs []string) (*types.Session, error) {
    // Validate input parameters
    if name == "" || len(name) > 200 {
        return nil, ErrInvalidSessionName
    }
    
    if !types.IsValidUserID(createdBy) {
        return nil, ErrInvalidCreatedBy
    }
    
    if len(studentIDs) == 0 {
        return nil, ErrEmptyStudentList
    }
    
    // Remove duplicate student IDs
    uniqueStudents := removeDuplicates(studentIDs)
    
    // Validate all student IDs
    for _, studentID := range uniqueStudents {
        if !types.IsValidUserID(studentID) {
            return nil, fmt.Errorf("%w: invalid student ID %s", ErrInvalidStudentID, studentID)
        }
    }
    
    // Create session object
    session := &types.Session{
        ID:         uuid.New().String(),
        Name:       name,
        CreatedBy:  createdBy,
        StudentIDs: uniqueStudents,
        StartTime:  time.Now(),
        EndTime:    nil,
        Status:     "active",
    }
    
    // Persist to database
    if err := m.dbManager.CreateSession(ctx, session); err != nil {
        return nil, fmt.Errorf("failed to create session: %w", err)
    }
    
    // Add to in-memory cache
    m.mu.Lock()
    m.activeSessions[session.ID] = session
    m.mu.Unlock()
    
    log.Printf("Created session: id=%s name=%s students=%d", session.ID, session.Name, len(session.StudentIDs))
    return session, nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
    // Check in-memory cache first
    m.mu.RLock()
    if session, exists := m.activeSessions[sessionID]; exists {
        m.mu.RUnlock()
        return session, nil
    }
    m.mu.RUnlock()
    
    // Query database for ended sessions or cache misses
    session, err := m.dbManager.GetSession(ctx, sessionID)
    if err != nil {
        return nil, err
    }
    
    return session, nil
}
```

### CRITICAL SESSION OPERATIONS (must follow):
```go
// EndSession ends an active session
func (m *Manager) EndSession(ctx context.Context, sessionID string) error {
    // Get session from cache
    m.mu.RLock()
    session, exists := m.activeSessions[sessionID]
    m.mu.RUnlock()
    
    if !exists {
        // Check if session exists in database but not active
        dbSession, err := m.dbManager.GetSession(ctx, sessionID)
        if err != nil {
            return ErrSessionNotFound
        }
        if dbSession.Status == "ended" {
            return ErrSessionAlreadyEnded
        }
        session = dbSession
    }
    
    // Update session status
    now := time.Now()
    session.EndTime = &now
    session.Status = "ended"
    
    // Persist to database
    if err := m.dbManager.UpdateSession(ctx, session); err != nil {
        return fmt.Errorf("failed to end session: %w", err)
    }
    
    // Remove from active sessions cache
    m.mu.Lock()
    delete(m.activeSessions, sessionID)
    m.mu.Unlock()
    
    log.Printf("Ended session: id=%s name=%s", session.ID, session.Name)
    return nil
}

// ListActiveSessions returns all active sessions
func (m *Manager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
    m.mu.RLock()
    sessions := make([]*types.Session, 0, len(m.activeSessions))
    for _, session := range m.activeSessions {
        sessions = append(sessions, session)
    }
    m.mu.RUnlock()
    
    return sessions, nil
}

// ValidateSessionMembership checks if user can join session
func (m *Manager) ValidateSessionMembership(sessionID, userID, role string) error {
    // Get session (check cache first)
    m.mu.RLock()
    session, exists := m.activeSessions[sessionID]
    m.mu.RUnlock()
    
    if !exists {
        return ErrSessionNotFound
    }
    
    // Check session is active
    if session.Status != "active" {
        return ErrSessionEnded
    }
    
    // Validate role-based access
    switch role {
    case "instructor":
        // Instructors have universal access to all active sessions
        return nil
        
    case "student":
        // Students must be in session's student_ids list
        for _, studentID := range session.StudentIDs {
            if studentID == userID {
                return nil
            }
        }
        return ErrUnauthorized
        
    default:
        return ErrInvalidRole
    }
}

// Helper function to remove duplicate student IDs
func removeDuplicates(studentIDs []string) []string {
    seen := make(map[string]bool)
    unique := make([]string, 0, len(studentIDs))
    
    for _, id := range studentIDs {
        if !seen[id] {
            seen[id] = true
            unique = append(unique, id)
        }
    }
    
    return unique
}
```

### SESSION CACHE MANAGEMENT (implement exactly):
```go
// GetStats returns session manager statistics
func (m *Manager) GetStats() map[string]interface{} {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    return map[string]interface{}{
        "active_sessions": len(m.activeSessions),
        "cache_size":     len(m.activeSessions),
    }
}

// RefreshCache reloads active sessions from database
func (m *Manager) RefreshCache(ctx context.Context) error {
    sessions, err := m.dbManager.ListActiveSessions(ctx)
    if err != nil {
        return fmt.Errorf("failed to refresh session cache: %w", err)
    }
    
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Clear current cache
    m.activeSessions = make(map[string]*types.Session)
    
    // Reload from database
    for _, session := range sessions {
        m.activeSessions[session.ID] = session
    }
    
    log.Printf("Refreshed session cache: %d active sessions", len(sessions))
    return nil
}

// IsSessionActive checks if a session is active (cache-only check)
func (m *Manager) IsSessionActive(sessionID string) bool {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    session, exists := m.activeSessions[sessionID]
    return exists && session.Status == "active"
}
```

### ERROR TYPES (define exactly these):
```go
var (
    ErrInvalidSessionName  = errors.New("session name must be 1-200 characters")
    ErrInvalidCreatedBy    = errors.New("created_by must be valid user ID")
    ErrEmptyStudentList    = errors.New("student list cannot be empty")
    ErrInvalidStudentID    = errors.New("invalid student ID format")
    ErrSessionNotFound     = errors.New("session not found")
    ErrSessionEnded        = errors.New("session has ended")
    ErrSessionAlreadyEnded = errors.New("session is already ended")
    ErrUnauthorized        = errors.New("user not authorized for this session")
    ErrInvalidRole         = errors.New("invalid role: must be 'student' or 'instructor'")
)
```

### SUCCESS CRITERIA:
**Architectural** (BLOCKING):
- [ ] No circular dependencies: clean integration with database layer only
- [ ] Interface compliance: implements SessionManager interface exactly
- [ ] Boundary separation: pure session logic, no WebSocket or routing concerns
- [ ] Cache consistency: in-memory state matches database state

**Functional** (BLOCKING):
- [ ] CreateSession() handles duplicate student IDs and validation correctly
- [ ] ValidateSessionMembership() enforces role-based access rules
- [ ] EndSession() updates database and cleans up cache atomically
- [ ] Session immutability: no modifications after creation

**Technical** (WARNING):
- [ ] Session validation completes in <1ms using cache
- [ ] Thread-safe cache operations under concurrent access
- [ ] Memory efficient for typical classroom usage (10 sessions)
- [ ] Proper error wrapping with context

### INTEGRATION CONTRACTS:
**What Step 4.2 (Database Manager) provides:**
- CreateSession(), GetSession(), UpdateSession(), ListActiveSessions() operations
- Error handling for database failures
- Transaction support for atomic operations

**What WebSocket Handler expects:**
- ValidateSessionMembership() for connection authentication
- Fast validation using in-memory cache
- Clear error types for different failure scenarios

### READ ONLY THESE REFERENCES:
- switchboard-tech-specs.md lines 192-212 (session management algorithm)
- switchboard-tech-specs.md lines 631-644 (session rules and constraints)
- switchboard-tech-specs.md lines 217-237 (client connection algorithm, session validation)
- pkg/interfaces/session.go (SessionManager interface from Step 1.2)

### IGNORE EVERYTHING ELSE
Do not read sections about WebSocket handling, message routing, or database implementation details. Focus only on session management logic.

### FILES TO CREATE:
- internal/session/manager.go (session manager implementation)
- internal/session/errors.go (error definitions)
- internal/session/manager_test.go (comprehensive session management tests)

---

## Step 4.2: Database Manager Implementation (Estimated: 3h)

### EXACT REQUIREMENTS (do not exceed scope):
- Implement DatabaseManager interface with all session and message operations
- Single-writer goroutine pattern to prevent SQLite write contention
- Transaction support for atomic session operations
- Error handling with retry logic (once after 5 seconds)
- SQLite optimizations from Phase 1 schema configuration
- Health check functionality for system monitoring

### ARCHITECTURAL VALIDATION REQUIREMENTS:
**Dependencies** (BLOCKING):
- Only import: pkg/interfaces, pkg/types, pkg/database, database/sql, sqlite3 driver, standard library
- No imports from: internal/websocket, internal/router, internal/session (avoid coupling)
- Verify: `go mod graph | grep -E "cycle|circular"` returns empty

**Boundaries** (BLOCKING):
- Pure database operations - no business logic, session management, or WebSocket handling
- Clean separation: database operations vs business logic
- Interface compliance: exact implementation of DatabaseManager interface

**Integration Contracts** (BLOCKING):
- Uses database configuration from Phase 1 Step 1.3
- Implements all interface methods for session and message operations
- Provides thread-safe database access with single-writer pattern
- Error handling suitable for higher-level components

### FUNCTIONAL VALIDATION REQUIREMENTS:
**Core Behaviors** (BLOCKING):
- Single-writer goroutine processes all write operations via channel
- Concurrent read operations supported (SQLite handles read concurrency)
- Transaction support for atomic session create/update operations
- Message storage with proper JSON serialization of content and student_ids
- Health check validates database connectivity and basic operations

**Error Handling** (BLOCKING):
- Write failures logged and retried once after 5 seconds
- Database connection errors handled gracefully with reconnection
- Transaction rollback on any failure during atomic operations
- Clear error messages with sufficient context for debugging

**Integration Contracts** (BLOCKING):
- Session operations match SessionManager expectations exactly
- Message operations support all 6 message types and routing patterns
- Error types align with interface specifications
- Database operations complete within reasonable time limits

### TECHNICAL VALIDATION REQUIREMENTS:
**Performance** (WARNING):
- Write operations complete in <50ms for typical classroom usage
- Read operations (session history) complete in <100ms for 1000 messages
- Connection pool configured appropriately for concurrent read access

**Reliability** (WARNING):
- Database file corruption handled gracefully
- Connection recovery after temporary failures
- Proper cleanup of resources on shutdown

### MANDATORY IMPLEMENTATION (implement exactly):
```go
package database

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "sync"
    "time"
    
    _ "github.com/mattn/go-sqlite3"
    "github.com/switchboard/pkg/interfaces"
    "github.com/switchboard/pkg/types"
    dbconfig "github.com/switchboard/pkg/database"
)

// Manager implements the DatabaseManager interface
type Manager struct {
    db       *sql.DB
    config   *dbconfig.Config
    writeChannel chan writeOperation
    shutdown chan struct{}
    wg       sync.WaitGroup
}

// writeOperation represents a database write operation
type writeOperation struct {
    operation func(*sql.DB) error
    result    chan error
}

// NewManager creates a new database manager
func NewManager(config *dbconfig.Config) (*Manager, error) {
    // Open database connection
    db, err := sql.Open("sqlite3", config.DatabasePath+"?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on")
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    
    // Configure connection pool
    db.SetMaxOpenConns(config.MaxConnections)
    db.SetConnMaxLifetime(config.ConnMaxLifetime)
    db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
    
    // Apply SQLite optimizations
    if err := applySQLiteOptimizations(db); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to apply SQLite optimizations: %w", err)
    }
    
    manager := &Manager{
        db:           db,
        config:       config,
        writeChannel: make(chan writeOperation, 100), // Buffer for write operations
        shutdown:     make(chan struct{}),
    }
    
    // Start single-writer goroutine
    manager.wg.Add(1)
    go manager.writeLoop()
    
    return manager, nil
}

// writeLoop processes all write operations in a single goroutine
func (m *Manager) writeLoop() {
    defer m.wg.Done()
    
    for {
        select {
        case op := <-m.writeChannel:
            err := op.operation(m.db)
            if err != nil {
                log.Printf("Database write failed, retrying in 5 seconds: %v", err)
                time.Sleep(5 * time.Second)
                err = op.operation(m.db) // Retry once
                if err != nil {
                    log.Printf("Database write failed after retry: %v", err)
                }
            }
            op.result <- err
            
        case <-m.shutdown:
            log.Println("Database write loop shutting down")
            return
        }
    }
}

// executeWrite queues a write operation and waits for completion
func (m *Manager) executeWrite(operation func(*sql.DB) error) error {
    result := make(chan error, 1)
    
    select {
    case m.writeChannel <- writeOperation{operation: operation, result: result}:
        return <-result
    case <-time.After(30 * time.Second):
        return fmt.Errorf("write operation timeout")
    }
}
```

### CRITICAL DATABASE OPERATIONS (must follow):
```go
// CreateSession creates a new session in the database
func (m *Manager) CreateSession(ctx context.Context, session *types.Session) error {
    return m.executeWrite(func(db *sql.DB) error {
        // Begin transaction
        tx, err := db.BeginTx(ctx, nil)
        if err != nil {
            return fmt.Errorf("failed to begin transaction: %w", err)
        }
        defer tx.Rollback()
        
        // Serialize student IDs to JSON
        studentIDsJSON, err := json.Marshal(session.StudentIDs)
        if err != nil {
            return fmt.Errorf("failed to marshal student IDs: %w", err)
        }
        
        // Insert session
        query := `
            INSERT INTO sessions (id, name, created_by, student_ids, start_time, status)
            VALUES (?, ?, ?, ?, ?, ?)
        `
        _, err = tx.ExecContext(ctx, query,
            session.ID,
            session.Name,
            session.CreatedBy,
            string(studentIDsJSON),
            session.StartTime,
            session.Status,
        )
        if err != nil {
            return fmt.Errorf("failed to insert session: %w", err)
        }
        
        // Commit transaction
        if err = tx.Commit(); err != nil {
            return fmt.Errorf("failed to commit session creation: %w", err)
        }
        
        return nil
    })
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
    query := `
        SELECT id, name, created_by, student_ids, start_time, end_time, status
        FROM sessions
        WHERE id = ?
    `
    
    row := m.db.QueryRowContext(ctx, query, sessionID)
    
    var session types.Session
    var studentIDsJSON string
    var endTime sql.NullTime
    
    err := row.Scan(
        &session.ID,
        &session.Name,
        &session.CreatedBy,
        &studentIDsJSON,
        &session.StartTime,
        &endTime,
        &session.Status,
    )
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, interfaces.ErrSessionNotFound
        }
        return nil, fmt.Errorf("failed to query session: %w", err)
    }
    
    // Deserialize student IDs
    if err := json.Unmarshal([]byte(studentIDsJSON), &session.StudentIDs); err != nil {
        return nil, fmt.Errorf("failed to unmarshal student IDs: %w", err)
    }
    
    // Handle nullable end_time
    if endTime.Valid {
        session.EndTime = &endTime.Time
    }
    
    return &session, nil
}

// UpdateSession updates an existing session
func (m *Manager) UpdateSession(ctx context.Context, session *types.Session) error {
    return m.executeWrite(func(db *sql.DB) error {
        query := `
            UPDATE sessions
            SET end_time = ?, status = ?
            WHERE id = ?
        `
        
        _, err := db.ExecContext(ctx, query,
            session.EndTime,
            session.Status,
            session.ID,
        )
        if err != nil {
            return fmt.Errorf("failed to update session: %w", err)
        }
        
        return nil
    })
}

// ListActiveSessions returns all active sessions
func (m *Manager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
    query := `
        SELECT id, name, created_by, student_ids, start_time, end_time, status
        FROM sessions
        WHERE status = 'active'
        ORDER BY start_time DESC
    `
    
    rows, err := m.db.QueryContext(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("failed to query active sessions: %w", err)
    }
    defer rows.Close()
    
    var sessions []*types.Session
    
    for rows.Next() {
        var session types.Session
        var studentIDsJSON string
        var endTime sql.NullTime
        
        err := rows.Scan(
            &session.ID,
            &session.Name,
            &session.CreatedBy,
            &studentIDsJSON,
            &session.StartTime,
            &endTime,
            &session.Status,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan session row: %w", err)
        }
        
        // Deserialize student IDs
        if err := json.Unmarshal([]byte(studentIDsJSON), &session.StudentIDs); err != nil {
            return nil, fmt.Errorf("failed to unmarshal student IDs: %w", err)
        }
        
        // Handle nullable end_time
        if endTime.Valid {
            session.EndTime = &endTime.Time
        }
        
        sessions = append(sessions, &session)
    }
    
    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating session rows: %w", err)
    }
    
    return sessions, nil
}
```

### MESSAGE OPERATIONS (implement exactly):
```go
// StoreMessage stores a message in the database
func (m *Manager) StoreMessage(ctx context.Context, message *types.Message) error {
    return m.executeWrite(func(db *sql.DB) error {
        // Serialize message content to JSON
        contentJSON, err := json.Marshal(message.Content)
        if err != nil {
            return fmt.Errorf("failed to marshal message content: %w", err)
        }
        
        query := `
            INSERT INTO messages (id, session_id, type, context, from_user, to_user, content, timestamp)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        `
        
        _, err = db.ExecContext(ctx, query,
            message.ID,
            message.SessionID,
            message.Type,
            message.Context,
            message.FromUser,
            message.ToUser,
            string(contentJSON),
            message.Timestamp,
        )
        if err != nil {
            return fmt.Errorf("failed to insert message: %w", err)
        }
        
        return nil
    })
}

// GetSessionHistory retrieves all messages for a session
func (m *Manager) GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error) {
    query := `
        SELECT id, session_id, type, context, from_user, to_user, content, timestamp
        FROM messages
        WHERE session_id = ?
        ORDER BY timestamp ASC
    `
    
    rows, err := m.db.QueryContext(ctx, query, sessionID)
    if err != nil {
        return nil, fmt.Errorf("failed to query session history: %w", err)
    }
    defer rows.Close()
    
    var messages []*types.Message
    
    for rows.Next() {
        var message types.Message
        var contentJSON string
        var toUser sql.NullString
        
        err := rows.Scan(
            &message.ID,
            &message.SessionID,
            &message.Type,
            &message.Context,
            &message.FromUser,
            &toUser,
            &contentJSON,
            &message.Timestamp,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan message row: %w", err)
        }
        
        // Handle nullable to_user
        if toUser.Valid {
            message.ToUser = &toUser.String
        }
        
        // Deserialize message content
        if err := json.Unmarshal([]byte(contentJSON), &message.Content); err != nil {
            return nil, fmt.Errorf("failed to unmarshal message content: %w", err)
        }
        
        messages = append(messages, &message)
    }
    
    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating message rows: %w", err)
    }
    
    return messages, nil
}

// HealthCheck validates database connectivity
func (m *Manager) HealthCheck(ctx context.Context) error {
    // Test database connectivity
    if err := m.db.PingContext(ctx); err != nil {
        return fmt.Errorf("database ping failed: %w", err)
    }
    
    // Test read operation
    _, err := m.db.QueryContext(ctx, "SELECT COUNT(*) FROM sessions LIMIT 1")
    if err != nil {
        return fmt.Errorf("database read test failed: %w", err)
    }
    
    return nil
}

// Close shuts down the database manager
func (m *Manager) Close() error {
    close(m.shutdown)
    m.wg.Wait() // Wait for write loop to finish
    
    if err := m.db.Close(); err != nil {
        return fmt.Errorf("failed to close database: %w", err)
    }
    
    return nil
}

// applySQLiteOptimizations applies performance optimizations
func applySQLiteOptimizations(db *sql.DB) error {
    pragmas := []string{
        "PRAGMA journal_mode = WAL",          // Write-Ahead Logging
        "PRAGMA synchronous = NORMAL",        // Balance safety and performance
        "PRAGMA cache_size = -64000",         // 64MB cache
        "PRAGMA temp_store = MEMORY",         // Use memory for temp tables
        "PRAGMA foreign_keys = ON",           // Enforce foreign keys
        "PRAGMA busy_timeout = 5000",         // 5 second timeout
    }
    
    for _, pragma := range pragmas {
        if _, err := db.Exec(pragma); err != nil {
            return fmt.Errorf("failed to execute pragma %s: %w", pragma, err)
        }
    }
    
    return nil
}
```

### SUCCESS CRITERIA:
**Architectural** (BLOCKING):
- [ ] No circular dependencies: clean database layer implementation
- [ ] Interface compliance: implements DatabaseManager interface exactly
- [ ] Single-writer pattern: all write operations through dedicated goroutine
- [ ] Resource management: proper connection pool and cleanup

**Functional** (BLOCKING):
- [ ] Session CRUD operations work correctly with JSON serialization
- [ ] Message storage handles all 6 message types and content serialization
- [ ] Transaction support ensures atomic session operations
- [ ] Error handling with retry logic as specified

**Technical** (WARNING):
- [ ] Write operations complete in <50ms for typical usage
- [ ] Read operations efficient for session history queries
- [ ] SQLite optimizations applied correctly
- [ ] Health check validates database functionality

### INTEGRATION CONTRACTS:
**What SessionManager expects:**
- CreateSession(), GetSession(), UpdateSession(), ListActiveSessions() operations
- Proper error handling for business logic decisions
- Thread-safe operations for concurrent access

**What MessageRouter expects:**
- StoreMessage() for persist-then-route pattern
- GetSessionHistory() for connection history replay
- Fast persistence operations to minimize routing delay

### READ ONLY THESE REFERENCES:
- switchboard-tech-specs.md lines 347-398 (database design and operations)
- switchboard-tech-specs.md lines 388-398 (database error recovery algorithm)
- pkg/database/config.go (database configuration from Step 1.3)
- pkg/interfaces/database.go (DatabaseManager interface from Step 1.2)

### IGNORE EVERYTHING ELSE
Do not read sections about session management logic, WebSocket handling, or message routing. Focus only on database operations.

### FILES TO CREATE:
- internal/database/manager.go (database manager implementation)
- internal/database/manager_test.go (comprehensive database operation tests)
- Add migration validation to ensure schema matches implementation