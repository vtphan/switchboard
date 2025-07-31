# Switchboard V4 - Specification Source Tracking

This document tracks the source references for all implementation requirements, ensuring traceability back to the original technical specifications.

## Source Document

**Primary Specification**: `/Users/vinhthuyphan/Apps/switchboard/docs/tech-specs.md`
**Last Updated**: Planning phase analysis
**Total Lines**: 851 lines of detailed technical specifications

---

## Line-by-Line Requirements Mapping

### Phase 1: Foundation Components

#### Core Data Models
- **Session struct definition**: Lines 91-98
  ```go
  type Session struct {
      ID        string     `json:"id" db:"id"`
      Name      string     `json:"name" db:"name"`
      CreatedBy string     `json:"created_by" db:"created_by"`
      StartTime time.Time  `json:"start_time" db:"start_time"`
      EndTime   *time.Time `json:"end_time,omitempty" db:"end_time"`
      Status    string     `json:"status" db:"status"`
  }
  ```

- **Message struct definition**: Lines 458-468
  ```go
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
  ```

- **Message type constants**: Lines 471-476
- **Context constants**: Lines 479-490

#### Database Manager Interface
- **DatabaseManager interface**: Lines 112-128
  ```go
  type DatabaseManager interface {
      CreateSession(session *Session) error
      UpdateSession(session *Session) error
      GetActiveSession() (*Session, error)
      WriteMessage(msg *Message) error
      WriteBatch(msgs []*Message) error
      GetSessionMessages(sessionID string) ([]*Message, error)
      Start() error
      Stop() error
  }
  ```

#### Database Implementation
- **SQLiteDatabaseManager structure**: Lines 541-548
- **Database configuration**: Lines 59-66 (SQLite pragmas)
- **Database batching algorithm**: Lines 609-632
- **Retry logic implementation**: Lines 594-607

#### Configuration Constants
- **System constants**: Lines 33-57
  ```go
  const (
      ConnectionSendBufferSize = 100
      DatabaseBatchSize       = 100
      DatabaseFlushInterval   = 200 * time.Millisecond
      // ... all other constants
  )
  ```

#### Database Schema
- **Complete database schema**: Lines 497-533
  ```sql
  CREATE TABLE sessions (
      id TEXT PRIMARY KEY,
      name TEXT NOT NULL,
      -- ... complete schema
  );
  
  CREATE TABLE messages (
      id TEXT PRIMARY KEY,
      session_id TEXT NOT NULL,
      -- ... complete schema with constraints
  );
  ```

### Phase 2: Session Management

#### SessionManager Interface
- **SessionManager interface**: Lines 162-168
  ```go
  type SessionManager interface {
      GetActiveSession() *Session
      SetActiveSession(session *Session) error
      ClearActiveSession() error
      HasActiveSession() bool
  }
  ```

#### Session State Access Patterns
- **GetActiveSession() algorithm**: Lines 173-177
- **SetActiveSession() algorithm**: Lines 179-185
- **ClearActiveSession() algorithm**: Lines 187-192
- **HasActiveSession() algorithm**: Lines 194-198

#### Session Lifecycle Operations
- **StartSession() algorithm**: Lines 350-376
- **EndSession() algorithm**: Lines 378-396

### Phase 3: Message Processing & Rate Limiting

#### Message Processing Pipeline
- **ProcessIncomingMessage() algorithm**: Lines 305-345
  ```
  Function ProcessIncomingMessage(rawData, senderID):
    1. Atomic session state capture
    2. Parse and validate message
    3. Prepare message with session context
    4. Rate limiting check
    5. Route message and determine recipients
    6. Real-time broadcast (immediate delivery)
    7. Asynchronous persistence
    8. Return success to sender
  ```

#### Role-Based Message Filtering
- **shouldReceiveMessage() algorithm**: Lines 720-749
  ```go
  func shouldReceiveMessage(message *Message, recipientRole, recipientUserID string) bool {
      // Complete filtering logic with educational privacy rules
  }
  ```

#### Error Handling Patterns
- **Standard error types**: Lines 411-418
- **Error handling interface**: Lines 404-409
- **Error response patterns**: Lines 422-451

### Phase 4: WebSocket Infrastructure

#### Connection Lifecycle Management
- **HandleClientConnection() pattern**: Lines 203-237
- **Connection cleanup algorithm**: Lines 241-269
- **Connection goroutine patterns**: Lines 271-300

#### System State Model
- **Switchboard core structure**: Lines 73-89
  ```go
  type Switchboard struct {
      activeSessionMu  sync.RWMutex
      activeSession    *Session
      connectionsMu    sync.RWMutex
      connections      map[string]*Connection
      // ... complete structure
  }
  ```

### Phase 5: HTTP API & System Integration

#### HTTP API Specifications
- **POST /api/session/start**: Lines 640-657
  ```http
  POST /api/session/start
  Content-Type: application/json
  
  {
    "name": "Exercise 3: For Loops",
    "instructor_id": "prof_smith"
  }
  ```

- **POST /api/session/end**: Lines 660-675
  ```http
  POST /api/session/end
  Content-Type: application/json
  
  {
    "instructor_id": "prof_smith"
  }
  ```

#### WebSocket Connection Specification
- **WebSocket URL format**: Lines 680-686
- **Connection response messages**: Lines 690-715

#### Graceful Shutdown Implementation
- **Graceful shutdown algorithm**: Lines 754-805
  ```
  Function Shutdown(shutdownContext):
    Phase 1 - Stop accepting new connections
    Phase 2 - Wait for in-flight messages (with timeout)
    Phase 3 - Close all WebSocket connections
    Phase 4 - Stop database manager
    Phase 5 - Wait for all goroutines (with timeout)
  ```

---

## Validation Source References

### Architectural Validation Requirements
- **Ultra-simplified architecture principles**: Lines 8-15
- **Single global session state**: Lines 70-71
- **Concurrency safety patterns**: Lines 15, 74-78

### Functional Validation Requirements
- **Core behavioral rules**: Lines 17-21
  1. No messages without active session
  2. Users can connect before session starts
  3. Messages broadcast to all connected users
  4. Late joiners get complete session history

### Performance Requirements
- **Measured performance characteristics**: Lines 810-815
- **Resource requirements**: Lines 817-828
- **Target performance metrics**: Lines 831-836

### Security and Privacy Requirements
- **Security model**: Lines 839-844
- **Educational privacy preservation**: Lines 845-850

---

## Implementation Patterns Source References

### Concurrency Patterns
- **Separate mutexes pattern**: Lines 74-78
- **RWMutex for read-heavy access**: Lines 74-76, 194-198
- **Single-writer database pattern**: Lines 129-137
- **Buffered channel patterns**: Lines 36, 104

### Database Patterns
- **WAL mode configuration**: Lines 60-66
- **Batching and retry logic**: Lines 42-50, 577-632
- **Dead letter queue**: Lines 47, 134

### Error Handling Patterns
- **Standard Go error types**: Lines 411-418
- **Consistent error response format**: Lines 422-451
- **Database rollback patterns**: Lines 365-369

### Testing Patterns
- **Coverage requirements**: Lines 85% statements mentioned in validation
- **Race condition testing**: `go test -race` requirement
- **Integration test patterns**: Cross-phase validation requirements

---

## Configuration Source References

### System Constants
All configuration constants sourced from lines 33-57:
- **Connection management**: Lines 35-40
- **Database performance**: Lines 42-47  
- **Rate limiting**: Lines 49-52
- **Graceful shutdown timeouts**: Lines 54-56

### Database Configuration
- **SQLite pragma settings**: Lines 60-66
- **Schema with constraints**: Lines 497-533
- **Index optimization**: Lines 527-532

---

## API Specification Sources

### RESTful Endpoints
- **Session start endpoint**: Lines 640-657
- **Session end endpoint**: Lines 660-675
- **Error response format**: Derived from error handling patterns

### WebSocket Protocol
- **Connection parameters**: Lines 680-686
- **Message format**: Lines 690-715
- **Connection lifecycle**: Lines 203-237

---

## Quality Assurance Sources

### Code Quality Requirements
- **No circular dependencies**: Architecture principle
- **Interface-first design**: Lines 112-128, 162-168
- **Proper error handling**: Lines 404-451
- **Thread safety**: Lines 15, concurrency patterns throughout

### Performance Validation
- **Message routing latency**: Lines 811-812 (83.992µs target)
- **Concurrent user support**: Lines 832-833 (50+ users)
- **Memory usage targets**: Lines 818-822 (15MB for 50 users)

### Testing Requirements
- **Unit test coverage**: 85%+ statements for critical code
- **Race condition testing**: `go test -race` must pass
- **Integration testing**: Cross-phase contract validation
- **Load testing**: Performance characteristics validation

---

## Traceability Matrix

| Implementation Component | Spec Lines | Validation Lines | Test Requirements |
|-------------------------|------------|------------------|-------------------|
| Session struct | 91-98 | 17-21 | JSON/DB serialization |
| Message struct | 458-468 | 471-490 | 3-type validation |
| DatabaseManager | 112-128 | 577-632 | Single-writer, batching | 
| SessionManager | 162-168 | 173-198 | Thread-safe, atomic |
| MessageProcessor | 305-345 | 720-749 | Pipeline, filtering |
| Connection | 203-237 | 271-300 | Single-writer, cleanup |
| HTTP API | 640-675 | 422-451 | RESTful, error handling |
| Graceful Shutdown | 754-805 | 54-56 | Multi-phase, timeouts |

---

## Implementation Completeness Checklist

### Requirements Coverage
- [ ] All struct definitions implemented exactly as specified
- [ ] All interface methods implemented with exact signatures
- [ ] All algorithms implemented following pseudocode patterns
- [ ] All constants match specification values exactly
- [ ] All database constraints implemented in schema and validation
- [ ] All API endpoints match request/response formats exactly

### Validation Coverage
- [ ] All architectural validation requirements addressed
- [ ] All functional validation requirements tested
- [ ] All performance requirements measured and validated
- [ ] All security and privacy requirements implemented

### Source Traceability
- [ ] Every implementation decision traceable to specification lines
- [ ] All validation criteria sourced from specifications
- [ ] All test requirements derived from specification requirements
- [ ] Complete implementation matches specification exactly

This source tracking ensures that every aspect of the implementation can be traced back to specific requirements in the technical specifications, maintaining complete traceability and preventing scope drift.