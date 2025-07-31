# Switchboard V4 - Implementation Dependency Graph

This document provides a visual representation of the phase dependencies and integration points for systematic implementation.

## Phase Dependency Chain

```
Phase 1: Foundation
├── DatabaseManager (SQLite + batching)
├── Session & Message models  
├── Configuration constants
└── Standard error types
    │
    ▼
Phase 2: Session Management
├── SessionManager (thread-safe state)
├── SessionLifecycle (start/end operations)
└── Session validation
    │ (depends on Phase 1: DatabaseManager, Session model, ErrorTypes)
    ▼
Phase 3: Message Processing & Rate Limiting
├── MessageProcessor (session gating + persistence)
├── MessageRouter (3-type routing)
├── RoleBasedFilter (educational privacy)
└── RateLimiter (100 msgs/minute/user)
    │ (depends on Phase 1: DatabaseManager, Message model)
    │ (depends on Phase 2: SessionManager)
    ▼
Phase 4: WebSocket Infrastructure  
├── Connection (single-writer pattern)
├── ConnectionRegistry (thread-safe registry)
├── WebSocketHandler (upgrade + lifecycle)
└── BroadcastSystem (filtered delivery)
    │ (depends on Phase 1: Configuration, ErrorTypes)
    │ (depends on Phase 2: SessionManager)
    │ (depends on Phase 3: MessageProcessor, RoleBasedFilter)
    ▼
Phase 5: HTTP API & System Integration
├── SessionAPI (RESTful endpoints)
├── HTTPServer (routing + static files)
├── SystemIntegration (main application)
└── GracefulShutdown (multi-phase cleanup)
    │ (depends on ALL previous phases)
```

## Critical Integration Points

### 1. Database Layer Integration (Phase 1 → All)
```
DatabaseManager Interface
├── Used by SessionManager (Phase 2) for session persistence
├── Used by MessageProcessor (Phase 3) for message storage
└── Provides unified batching and retry logic for all data operations

Key Contract: Single-writer pattern eliminates SQLite lock contention
```

### 2. Session State Integration (Phase 2 → Phase 3, 4, 5)
```
SessionManager Interface
├── Called by MessageProcessor for session gating
├── Called by WebSocketHandler for session state delivery
└── Called by HTTP API for session management

Key Contract: Thread-safe atomic operations with RWMutex
```

### 3. Message Flow Integration (Phase 3 → Phase 4)
```
Message Processing Pipeline
├── MessageProcessor validates and routes messages
├── ConnectionRegistry provides recipients via RecipientRegistry interface
├── RoleBasedFilter preserves educational privacy
└── BroadcastSystem delivers filtered messages to connections

Key Contract: Role-based filtering applied before delivery
```

### 4. WebSocket Integration (Phase 4 → Phase 5)
```
Connection Management
├── WebSocketHandler upgrades HTTP to WebSocket
├── Connection implements single-writer pattern
├── ConnectionRegistry manages connection lifecycle
└── HTTP server routes WebSocket upgrades

Key Contract: 100-message buffered channels for non-blocking delivery  
```

## Interface Contracts Summary

### Core Interfaces (Must Implement Exactly)

**DatabaseManager** (Phase 1)
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

**SessionManager** (Phase 2)
```go
type SessionManager interface {
    GetActiveSession() *Session
    SetActiveSession(session *Session) error
    ClearActiveSession() error
    HasActiveSession() bool
}
```

**RecipientRegistry** (Phase 3/4 Bridge)
```go
type RecipientRegistry interface {
    GetInstructors() []Recipient
    GetStudents() []Recipient
    GetUserByID(userID string) (Recipient, error)
    GetAllUsers() []Recipient
}
```

**Connection** (Phase 4)
```go
type ConnectionInterface interface {
    WriteJSON(v interface{}) error
    Close() error
    GetUserID() string
    GetRole() string
    SetCredentials(username, role string) error
    UpdateActivity()
    GetLastSeen() time.Time
}
```

## Implementation Validation Checkpoints

### Phase 1 Validation
- [ ] No circular dependencies: `go mod graph | grep cycle` returns empty
- [ ] DatabaseManager interface complete and unambiguous
- [ ] SQLite WAL mode configuration applied
- [ ] Message and Session structs match tech specs exactly

### Phase 2 Validation  
- [ ] SessionManager uses exact RWMutex patterns from tech specs
- [ ] SetActiveSession() atomically checks for existing session
- [ ] Database rollback on session creation failure
- [ ] Thread-safe under concurrent access: `go test -race` passes

### Phase 3 Validation
- [ ] Messages rejected without active session (gating works)
- [ ] 3-message type routing: broadcast_to_instructors, direct_message, broadcast_to_students
- [ ] Role-based filtering: students don't see other students' questions
- [ ] Rate limiting: exactly 100 messages/minute per user

### Phase 4 Validation
- [ ] Single-writer pattern: only writeLoop() writes to WebSocket
- [ ] Buffered channels sized exactly 100 messages (ConnectionSendBufferSize)
- [ ] Connection cleanup removes stale connections within 30 seconds
- [ ] RecipientRegistry interface implemented correctly

### Phase 5 Validation
- [ ] HTTP API endpoints match tech specs exactly
- [ ] WebSocket upgrade works end-to-end
- [ ] Graceful shutdown completes within 10 seconds
- [ ] Complete system passes end-to-end tests

## Testing Integration Strategy

### Unit Tests (Per Phase)
- Each phase includes comprehensive unit tests
- Interfaces mocked for isolated testing
- Race condition testing with `go test -race`
- Coverage target: ≥85% statements

### Integration Tests (Cross-Phase)
- Phase 1+2: Session database persistence
- Phase 2+3: Message processing with session gating
- Phase 3+4: Message routing and delivery
- Phase 4+5: WebSocket upgrade and HTTP integration
- Complete end-to-end: All phases working together

### Load Tests (System-Wide)
- 50+ concurrent WebSocket connections
- 500+ messages/second throughput
- Memory stability under load
- Connection cleanup under high churn

## Risk Mitigation

### High-Risk Integration Points
1. **Database Concurrency** (Phase 1)
   - Risk: SQLite lock contention
   - Mitigation: Single-writer pattern with WAL mode

2. **Session State Consistency** (Phase 2)
   - Risk: Race conditions in session state
   - Mitigation: Atomic operations with RWMutex

3. **Message Delivery Ordering** (Phase 3+4)
   - Risk: Message delivery out of order
   - Mitigation: Buffered channels maintain order

4. **Connection Cleanup** (Phase 4)
   - Risk: Memory leaks from dead connections
   - Mitigation: Heartbeat monitoring + automatic cleanup

5. **Graceful Shutdown** (Phase 5)
   - Risk: Data loss or connection leaks
   - Mitigation: Multi-phase shutdown with timeouts

### Validation Strategy
- Each phase must pass architectural, functional, and technical validation
- Integration tests verify contracts between phases
- Load testing confirms performance characteristics
- End-to-end testing validates complete system behavior

This dependency graph ensures systematic implementation with clear validation checkpoints at each phase boundary.