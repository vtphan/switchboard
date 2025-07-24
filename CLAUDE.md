# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Key Commands

### Build and Run
```bash
make build                  # Build the application
make run                    # Build and run the application  
make dev                    # Run in development mode
```

### Testing (Validation-Driven TDD)
```bash
make test                   # Run all tests
make test-race              # Run tests with race detection (CRITICAL for WebSocket code)
make coverage               # Generate test coverage report (target: 85%+ for critical code)
make benchmark              # Run performance benchmarks
make load-test              # Run load tests for real-time components

# Run a single test
go test -run TestName ./path/to/package
```

### Validation (BLOCKING for TDD)
```bash
make validate               # Run ALL validation checks (must pass before commit)
make lint                   # Run static analysis (golangci-lint)
make security               # Run security analysis (gosec)
make vulnerability          # Check for vulnerabilities (govulncheck)
```

### Resource Leak Detection
```bash
make leak-test              # Check for memory leaks
make goroutine-test         # Check for goroutine leaks (critical for WebSocket handlers)
```

### Database Operations
```bash
make migrate-up             # Apply database migrations
make migrate-down           # Rollback database (destructive)
```

## Architecture Overview

### Layer-by-Layer Architecture

The system is built in 5 phases to prevent circular dependencies:

1. **Foundation Layer** (`pkg/`)
   - `pkg/types` - Core data structures (Session, Message, Client, ConnectionManager)
   - `pkg/interfaces` - Interface definitions (Connection, SessionManager, MessageRouter, DatabaseManager)
   - `pkg/database` - Database configuration and migration system

2. **WebSocket Infrastructure** (`internal/websocket/`)
   - Single-writer pattern for connection writes (100-message buffer)
   - Connection registry with O(1) lookups for routing
   - Heartbeat monitoring (30s ping/pong, 120s stale cleanup)

3. **Message Routing System** (`internal/router/`, `internal/hub/`)
   - 6 message types with specific routing patterns
   - Rate limiting: 100 messages/minute per client
   - Persist-then-route pattern (DB write MUST complete before delivery)
   - Hub goroutine coordinates all message flow

4. **Session Management** (`internal/session/`, `internal/database/`)
   - In-memory cache for <1ms validation performance
   - Immutable sessions after creation
   - Single-writer goroutine for SQLite (prevents write contention)

5. **API Layer** (`internal/api/`, `cmd/switchboard/`)
   - REST endpoints for session management
   - WebSocket endpoint with query parameter authentication
   - Health monitoring endpoint

### Critical Concurrency Patterns

**Single-Writer Pattern (MANDATORY)**:
- WebSocket connections: One writeLoop goroutine per connection
- Database operations: One writeLoop goroutine for all DB writes
- Prevents race conditions and data corruption

**Channel Communication**:
- Hub coordinates via channels: messageChannel (1000 buffer), registerChannel, unregisterChannel
- Database write coordination via writeChannel (100 buffer)
- Use select with context for graceful shutdown

**Mutex Protection**:
- Connection registry: RWMutex for concurrent reads during routing
- Rate limiter: Mutex for per-client state updates
- Session cache: RWMutex for validation lookups

### Message Routing Rules

| Message Type | Sender | Recipients | Key Requirement |
|-------------|--------|------------|-----------------|
| instructor_inbox | Student | All session instructors | No to_user field |
| inbox_response | Instructor | Specific student | Requires to_user |
| request | Instructor | Specific student | Requires to_user |
| request_response | Student | All session instructors | No to_user field |
| analytics | Student | All session instructors | No to_user field |
| instructor_broadcast | Instructor | All session students | No to_user field |

### Performance Targets

- Session validation: <1ms (using in-memory cache)
- Message routing: <10ms for broadcasts
- Database writes: <50ms (single-writer pattern)
- WebSocket message throughput: 1000+ messages/second per connection
- Memory usage: ~1MB for 50 concurrent users

### Validation Requirements

**Architectural Validation (BLOCKING)**:
- No circular dependencies: `go mod graph | grep cycle` must be empty
- Interface compliance: All implementations match interfaces exactly
- Clean boundaries: No business logic in infrastructure layers

**Functional Validation (BLOCKING)**:
- Test coverage: 85%+ for critical components (router, session, websocket)
- Race detection: ALL concurrent code must pass `go test -race`
- Error handling: Every error must be handled with context

**Resource Management**:
- Connection cleanup: Close() must be idempotent
- Goroutine cleanup: No leaks after shutdown
- Channel cleanup: Proper closure signaling

### Common Pitfalls to Avoid

1. **WebSocket Write Races**: NEVER write directly to WebSocket from multiple goroutines. Always use the Connection wrapper's WriteJSON() method which sends to the write channel.

2. **Database Write Contention**: NEVER write to SQLite from multiple goroutines. Always use DatabaseManager's executeWrite() method.

3. **Session Cache Staleness**: ALWAYS update the in-memory cache immediately when ending a session to prevent stale access.

4. **Rate Limiter Memory Leak**: MUST periodically call Cleanup() to remove disconnected client entries.

5. **Message ID Trust**: NEVER trust client-provided message IDs. Always generate server-side UUIDs.

### Testing Strategy

**Unit Tests**: Test individual components with mocks
```bash
go test ./internal/router -run TestRouteMessage
```

**Integration Tests**: Test component interactions
```bash
go test ./tests/integration -run TestMessageFlow
```

**Load Tests**: Validate performance under classroom scale
```bash
go test ./tests/load -run TestConcurrentConnections -count=50
```

**Race Tests**: Detect concurrency issues
```bash
go test -race ./internal/websocket -run TestConcurrentWrites
```

### Project Status

Implementation follows the phase-based plan in `planning/`:
- Phase 1: Foundation Layer ⏳
- Phase 2: WebSocket Infrastructure ⏳
- Phase 3: Message Routing ⏳
- Phase 4: Session Management ⏳
- Phase 5: API Integration ⏳

See `planning/code-inventory.md` for detailed function tracking and `planning/spec-source.md` for requirement traceability.