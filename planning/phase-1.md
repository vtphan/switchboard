# Phase 1: Foundation - Data & Configuration

**Estimated Time:** 1-2 days  
**Dependencies:** None (foundation layer)  
**Provides:** DatabaseManager, Session, Message, Configuration, ErrorTypes

## Phase Overview

Establishes the foundational data structures, database layer, and system configuration that all other phases depend on. Implements the single-writer database pattern and core domain models.

---

## Step 1.1: Core Data Models (Estimated: 3h)

### EXACT REQUIREMENTS:
Read **exactly lines 456-491** of `/Users/vinhthuyphan/Apps/switchboard/docs/tech-specs.md` for complete Message and Session struct definitions. Implement exactly as specified with all validation constraints.

### ARCHITECTURAL VALIDATION:
- No imports from: `internal/websocket`, `internal/session`, `internal/message` (foundation layer)
- Location: Must be in `internal/database/models.go` 
- No business logic in data models - pure data structures only
- JSON and database tags exactly as specified in tech specs

### FUNCTIONAL VALIDATION:
- Session validation: name 1-200 chars, valid status enum
- Message validation: content ≤64KB, valid type enum, required fields
- Time handling: Use `time.Time` with proper JSON marshaling
- Pointer fields: `ToUser` and `EndTime` correctly handled as nullable

### INTEGRATION CONTRACTS:
- DatabaseManager can serialize/deserialize these structs to SQLite
- SessionManager can create Session structs with validation
- MessageProcessor can create Message structs with type validation

### MANDATORY INTERFACE:
```go
// Session represents an active teaching session
type Session struct {
    ID        string     `json:"id" db:"id"`
    Name      string     `json:"name" db:"name"`
    CreatedBy string     `json:"created_by" db:"created_by"`
    StartTime time.Time  `json:"start_time" db:"start_time"`
    EndTime   *time.Time `json:"end_time,omitempty" db:"end_time"`
    Status    string     `json:"status" db:"status"`
}

// Message represents a communication message in the system
type Message struct {
    ID        string                 `json:"id" db:"id"`
    SessionID string                 `json:"session_id" db:"session_id"`
    Type      string                 `json:"type" db:"type"`
    Context   string                 `json:"context" db:"context"`
    FromUser  string                 `json:"from_user" db:"from_user"`
    ToUser    *string                `json:"to_user,omitempty" db:"to_user"`
    Content   map[string]interface{} `json:"content" db:"content"`
    Timestamp time.Time              `json:"timestamp" db:"timestamp"`
}

// Constants exactly as specified in tech specs lines 471-490
const (
    MessageTypeBroadcastToInstructors = "broadcast_to_instructors"
    MessageTypeDirectMessage         = "direct_message"
    MessageTypeBroadcastToStudents   = "broadcast_to_students"
    MessageTypeSystem                = "system"
)

const (
    ContextQuestion      = "question"
    ContextSubmission    = "submission"
    ContextAnalytics     = "analytics"
    ContextResponse      = "response"
    ContextRequest       = "request" 
    ContextPeerHelp      = "peer_help"
    ContextAnnouncement  = "announcement"
    ContextInstruction   = "instruction"
    ContextEmergency     = "emergency"
    ContextGeneral       = "general"
)
```

### SUCCESS CRITERIA:
- [ ] No circular dependencies: `go mod graph | grep cycle` returns empty
- [ ] Structs serialize/deserialize correctly to/from JSON and SQLite
- [ ] All validation constraints enforced (length limits, enums)
- [ ] `go test -race` passes on model tests
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `internal/database/models.go`
- `internal/database/models_test.go`

---

## Step 1.2: Database Manager Interface (Estimated: 2h)

### EXACT REQUIREMENTS:
Read **exactly lines 112-138** of `/Users/vinhthuyphan/Apps/switchboard/docs/tech-specs.md` for complete DatabaseManager interface. Implement interface exactly as specified - no modifications.

### ARCHITECTURAL VALIDATION:
- Interface in `internal/database/manager.go`
- No implementation details in interface file - pure interface definition
- No imports from business logic packages (`session`, `message`, `websocket`)
- Interface uses only domain models from Step 1.1

### FUNCTIONAL VALIDATION:
- All methods return proper Go error types
- Batch operations support exactly 100 messages (DatabaseBatchSize)
- Start()/Stop() lifecycle methods for proper resource management
- Session operations support create/update/retrieve patterns

### INTEGRATION CONTRACTS:
- SessionManager will call session-related methods
- MessageProcessor will call message-related methods
- HTTP handlers will call through other managers, never directly

### MANDATORY INTERFACE:
```go
// DatabaseManager provides unified interface for all database operations
type DatabaseManager interface {
    // Session operations
    CreateSession(session *Session) error
    UpdateSession(session *Session) error
    GetActiveSession() (*Session, error)
    
    // Message operations  
    WriteMessage(msg *Message) error
    WriteBatch(msgs []*Message) error
    GetSessionMessages(sessionID string) ([]*Message, error)
    
    // Lifecycle
    Start() error
    Stop() error
}
```

### SUCCESS CRITERIA:
- [ ] Interface compiles without errors
- [ ] No concrete implementation in interface file
- [ ] Interface methods match tech specs exactly
- [ ] Proper Go idioms: errors, pointer receivers, etc.

### FILES TO CREATE:
- `internal/database/manager.go` (interface only)

---

## Step 1.3: SQLite Database Implementation (Estimated: 4h)

### EXACT REQUIREMENTS:
Read **exactly lines 539-632** of `/Users/vinhthuyphan/Apps/switchboard/docs/tech-specs.md` for complete SQLiteDatabaseManager implementation patterns. Implement single-writer pattern with batching and retry logic exactly as specified.

### ARCHITECTURAL VALIDATION:
- Implementation in `internal/database/sqlite.go`
- Implements DatabaseManager interface from Step 1.2
- Single-writer pattern: All writes go through one goroutine
- WAL mode configuration applied exactly as specified

### FUNCTIONAL VALIDATION:
- Database batching: Collects up to 100 messages or flushes after 200ms
- Retry logic: Exponential backoff with max 3 retries
- Dead letter queue: 50 message buffer for failed writes
- SQLite pragmas applied: WAL mode, cache size, synchronous mode

### INTEGRATION CONTRACTS:
- Accepts Session and Message structs from Step 1.1
- Thread-safe for concurrent access from multiple managers
- Proper lifecycle: Start() initializes, Stop() flushes and closes

### MANDATORY INTERFACE:
```go
type SQLiteDatabaseManager struct {
    db              *sql.DB
    writeChannel    chan writeRequest    // Buffer: DatabaseWriteBuffer (100)
    deadLetterQueue chan writeRequest    // Buffer: DeadLetterQueueSize (50)
    metrics         *DatabaseMetrics
    stopCh          chan struct{}
    wg              sync.WaitGroup
}

type writeRequest struct {
    operation string
    data      interface{}
    responseCh chan error
}

type DatabaseMetrics struct {
    SuccessfulWrites    int64
    FailedWrites        int64  
    RetriedWrites       int64
    DeadLetterCount     int64
    PermanentLossCount  int64
}
```

### SUCCESS CRITERIA:
- [ ] Single-writer pattern enforced: only one goroutine writes to SQLite
- [ ] Batching works: 100 messages or 200ms timeout triggers flush
- [ ] Retry logic: Failed writes retried up to 3 times with exponential backoff
- [ ] Dead letter queue: Failed writes queued for manual intervention
- [ ] WAL mode: Concurrent reads work during writes
- [ ] `go test -race` passes on all database tests
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `internal/database/sqlite.go`
- `internal/database/sqlite_test.go`
- `internal/database/batch.go` (message batching logic)
- `internal/database/batch_test.go`

---

## Step 1.4: Configuration System (Estimated: 1h)

### EXACT REQUIREMENTS:
Configuration constants are **already implemented** in `pkg/config/config.go`. Verify all constants match **exactly lines 33-57** of tech specs. No modifications needed if they match.

### ARCHITECTURAL VALIDATION:
- Constants in `pkg/config/config.go` (already exists)
- No business logic in configuration - pure constants only
- Values exactly as specified in tech specs

### FUNCTIONAL VALIDATION:
- All timing constants use `time.Duration` types
- Buffer sizes are integers matching tech specs exactly
- SQLite configuration string matches tech specs exactly

### INTEGRATION CONTRACTS:
- Database layer uses these constants for buffer sizes and timeouts
- WebSocket layer uses these constants for connection management
- Rate limiter uses these constants for rate limiting

### SUCCESS CRITERIA:
- [ ] All constants match tech specs exactly
- [ ] Proper Go types: `time.Duration` for timeouts, `int` for sizes
- [ ] No modifications needed if current implementation correct

### FILES TO VERIFY:
- `pkg/config/config.go` (should already exist and be correct)

---

## Step 1.5: Standard Error Types (Estimated: 30min)

### EXACT REQUIREMENTS:
Error definitions are **already implemented** in `pkg/errors/errors.go`. Verify all errors match **exactly lines 411-418** of tech specs. No modifications needed if they match.

### ARCHITECTURAL VALIDATION:
- Errors in `pkg/errors/errors.go` (already exists)
- Standard Go error types using `errors.New()`
- No custom error types - use standard library

### FUNCTIONAL VALIDATION:
- All error messages exactly as specified in tech specs
- Error names follow Go naming conventions

### INTEGRATION CONTRACTS:
- All internal packages will use these standard error types
- HTTP handlers will translate these to client-appropriate messages
- Database layer will return these errors for business logic failures

### SUCCESS CRITERIA:
- [ ] All error messages match tech specs exactly
- [ ] Proper Go error type usage
- [ ] No custom error types or unnecessary complexity

### FILES TO VERIFY:
- `pkg/errors/errors.go` (should already exist and be correct)

---

## Phase 1 Integration Tests

### Phase 1 Integration Test: Database Models
```go
// Auto-generate: tests/integration/phase1_models_integration_test.go
func TestSessionMessageSerialization_Integration(t *testing.T) {
    session := &Session{
        ID:        "test-session-001",
        Name:      "Test Session",
        CreatedBy: "instructor123", 
        StartTime: time.Now(),
        Status:    "active",
    }
    
    // Test JSON serialization
    jsonData, err := json.Marshal(session)
    require.NoError(t, err)
    
    var deserializedSession Session
    err = json.Unmarshal(jsonData, &deserializedSession)
    require.NoError(t, err)
    assert.Equal(t, session.ID, deserializedSession.ID)
    
    // Test message with all 3 types
    for _, msgType := range []string{
        MessageTypeBroadcastToInstructors,
        MessageTypeDirectMessage, 
        MessageTypeBroadcastToStudents,
    } {
        msg := &Message{
            ID:        fmt.Sprintf("msg-%s", msgType),
            SessionID: session.ID,
            Type:      msgType,
            Context:   ContextGeneral,
            FromUser:  "user123",
            Content:   map[string]interface{}{"text": "test"},
            Timestamp: time.Now(),
        }
        
        jsonData, err := json.Marshal(msg)
        require.NoError(t, err)
        
        var deserializedMessage Message
        err = json.Unmarshal(jsonData, &deserializedMessage)
        require.NoError(t, err)
        assert.Equal(t, msg.Type, deserializedMessage.Type)
    }
}
```

### Phase 1 Integration Test: Database Implementation
```go
// Auto-generate: tests/integration/phase1_database_integration_test.go
func TestSQLiteDatabaseManager_Integration(t *testing.T) {
    // Setup
    dbPath := ":memory:"
    db, err := sql.Open("sqlite3", dbPath)
    require.NoError(t, err)
    defer db.Close()
    
    // Apply schema
    schema, err := os.ReadFile("../../../internal/database/migrations.sql")
    require.NoError(t, err)
    _, err = db.Exec(string(schema))
    require.NoError(t, err)
    
    dbManager := NewSQLiteDatabaseManager(db)
    err = dbManager.Start()
    require.NoError(t, err)
    defer dbManager.Stop()
    
    // Test session operations
    session := &Session{
        ID:        "integration-test-session",
        Name:      "Integration Test Session",
        CreatedBy: "test-instructor",
        StartTime: time.Now(),
        Status:    "active",
    }
    
    err = dbManager.CreateSession(session)
    require.NoError(t, err)
    
    retrievedSession, err := dbManager.GetActiveSession()
    require.NoError(t, err)
    require.NotNil(t, retrievedSession)
    assert.Equal(t, session.ID, retrievedSession.ID)
    
    // Test message operations
    messages := []*Message{}
    for i := 0; i < 5; i++ {
        msg := &Message{
            ID:        fmt.Sprintf("msg-%d", i),
            SessionID: session.ID,
            Type:      MessageTypeBroadcastToStudents,
            Context:   ContextGeneral,
            FromUser:  "test-user",
            Content:   map[string]interface{}{"text": fmt.Sprintf("Message %d", i)},
            Timestamp: time.Now(),
        }
        messages = append(messages, msg)
    }
    
    err = dbManager.WriteBatch(messages)
    require.NoError(t, err)
    
    retrievedMessages, err := dbManager.GetSessionMessages(session.ID)
    require.NoError(t, err)
    assert.Len(t, retrievedMessages, 5)
}
```

---

## Phase 1 Success Criteria

### ARCHITECTURAL VALIDATION ✓
- [ ] No circular dependencies between packages
- [ ] DatabaseManager interface complete and unambiguous
- [ ] Models contain no business logic
- [ ] Configuration and errors properly separated

### FUNCTIONAL VALIDATION ✓
- [ ] Session and Message structs handle all validation constraints
- [ ] Database implements single-writer pattern correctly
- [ ] Batching and retry logic work under load
- [ ] SQLite WAL mode enables concurrent reads

### TECHNICAL VALIDATION ✓
- [ ] Test coverage ≥85% statements for all foundation code
- [ ] `go test -race` passes on all tests
- [ ] Integration tests verify database operations end-to-end
- [ ] golangci-lint clean on all new code

### INTEGRATION READINESS ✓
- [ ] SessionManager can use DatabaseManager interface (Phase 2)
- [ ] MessageProcessor can create and validate Messages (Phase 3)
- [ ] All packages can use standard error types consistently
- [ ] Configuration constants available for buffer sizing and timeouts

**Phase 1 establishes the rock-solid foundation for all subsequent phases.**