# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Switchboard V4 is a real-time educational communication system with an ultra-simplified architecture based on a **single global session state**. The system uses Go with SQLite and WebSockets to provide real-time messaging between students and instructors in educational settings.

## Core Architecture Principles

### Single-Session State Model
- **One active session at a time** - Simple on/off state eliminates complex session management
- **Pre-connection support** - Users can connect before sessions start (waiting state)  
- **Message gating** - No messages accepted without active session
- **Complete history** - Late joiners receive full session history (role-filtered)

### 3-Message Type System
1. `broadcast_to_instructors` - Student questions visible only to instructors
2. `direct_message` - Private conversations between specific users
3. `broadcast_to_students` - Instructor announcements to all students

### Concurrency Architecture
- **Separate mutexes**: `activeSessionMu` (RWMutex) and `connectionsMu` (RWMutex) for optimal performance
- **Single-writer database pattern**: Eliminates SQLite lock contention using batched writes
- **WAL mode SQLite**: Enables concurrent reads during writes
- **Buffered channels**: Non-blocking message delivery with configurable buffer sizes

## Development Commands

### Setup & Build
```bash
make setup          # Setup development environment and dependencies
make build          # Build the application binary
make init-db        # Initialize SQLite database with schema
```

### Development Workflow  
```bash
make dev            # Run with auto-reload (requires air)
make run            # Run development server
make test           # Run all tests
make test-coverage  # Run tests with coverage report
go test ./tests/unit/session_test.go         # Run single unit test
go test -run TestSpecificFunction ./...      # Run specific test function
```

### Database Management
```bash
make db-shell       # Open SQLite shell for database inspection
make load-fixtures  # Load test data for development
sqlite3 db/switchboard.db ".tables"          # View database tables
sqlite3 db/switchboard.db ".schema messages" # View table schema
```

### Code Quality  
```bash
make fmt            # Format Go code
make lint           # Run golangci-lint
make clean          # Clean build artifacts and database
```

### Web Test Client
```bash
# Start server and access test client
make run
# Open http://localhost:8080 in browser
# Test client provides WebSocket connection testing, session management, and message sending
```

### Environment Setup
```bash
# Required development tools
go install github.com/cosmtrek/air@latest  # Auto-reload
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest  # Linter
```

## Project Structure

```
switchboard-v4/
├── cmd/server/main.go              # Application entry point
├── internal/                       # Private application code
│   ├── database/                   # DatabaseManager interface & SQLite implementation
│   ├── websocket/                  # WebSocket connection management  
│   ├── session/                    # Session lifecycle (start/end)
│   ├── message/                    # Message processing, routing, filtering
│   └── rate/                       # Rate limiting implementation
├── pkg/                           # Public packages
│   ├── config/                    # System configuration constants
│   └── errors/                    # Standard error definitions  
├── web/                           # HTTP handlers and test client
└── db/                            # SQLite database and migrations
```

## Key Implementation Constraints

### Database Layer
- **Unified DatabaseManager interface** - Single abstraction for all database operations
- **Batched writes** - Messages collected and written in batches (100 messages or 200ms timeout)
- **Retry logic** - Exponential backoff with dead letter queue for failed writes
- **SQLite configuration** - WAL mode with specific pragmas for performance

### Session State Management  
- **Standardized access** - Use SessionManager interface methods, never direct mutex operations
- **Atomic operations** - Session state changes must be atomic with database persistence
- **Rollback on failure** - Database failures must rollback in-memory state changes

### Connection Management
- **Automatic cleanup** - Background goroutine removes stale connections every 30 seconds
- **Heartbeat monitoring** - WebSocket ping/pong detects dead connections
- **Graceful shutdown** - Multi-phase shutdown ensures clean resource cleanup

### Message Processing
- **Role-based filtering** - Students see limited messages for privacy (instructors see all)
- **Rate limiting** - 100 messages per minute per user
- **Non-blocking delivery** - Failed sends to slow clients don't block others

## Critical Configuration Constants

Located in `pkg/config/config.go`:

```go
// Connection Management  
ConnectionSendBufferSize = 100     // Channel buffer per WebSocket connection
ConnectionTimeout = 120 * time.Second  // Stale connection cleanup threshold

// Database Performance
DatabaseBatchSize = 100            // Messages per batch write
DatabaseFlushInterval = 200 * time.Millisecond  // Max batch wait time

// Rate Limiting
RateLimitMaxMessages = 100         // Messages per user per minute
```

## Testing Strategy

### Unit Tests (`tests/unit/`)
- Session state transitions and thread safety
- Message filtering logic by role
- Database batching and retry mechanisms

### Integration Tests (`tests/integration/`)  
- WebSocket connection flows
- Complete message processing pipeline
- API endpoint functionality

### Load Tests (`tests/load/`)
- Concurrent user connections (target: 50+ users)
- Message throughput (target: 500+ messages/second)
- Memory stability under load

## Development Guidelines

### Error Handling
- Use standard error types from `pkg/errors/errors.go`
- WebSocket errors formatted as JSON with consistent structure
- Internal errors logged but not exposed to clients

### Concurrency Patterns
- Reference `docs/go-patterns-reference.md` for approved Go patterns
- Always use the established mutex separation pattern
- Prefer buffered channels with appropriate sizing

### Database Operations  
- All writes must go through DatabaseManager interface
- Never bypass the single-writer pattern
- Use the established retry and dead letter queue mechanisms

## Current Implementation Status

**Completed:**
- Project structure with proper Go module organization  
- Database schema with SQLite migrations (sessions and messages tables)
- Configuration system with development settings (`config/development.yaml`)
- Build system with comprehensive Makefile
- Basic test client interface (`web/static/index.html` and `web/static/js/client.js`)
- Error definitions (`pkg/errors/errors.go`) and configuration constants (`pkg/config/config.go`)

**To Be Implemented:**
- Core Go implementations (DatabaseManager, SessionManager, WebSocket handlers)
- Message processing pipeline (`internal/message/`)
- Rate limiting system (`internal/rate/`)
- Connection management (`internal/websocket/`)
- HTTP API handlers (`web/api/`)
- Unit, integration, and load tests

## Architecture Documentation

- **Complete specification**: `docs/tech-specs.md` - Detailed algorithms in pseudocode format
- **Setup instructions**: `docs/setup.md` - Comprehensive development workflow  
- **Go patterns reference**: `docs/go-patterns-reference.md` - Approved concurrency patterns

The tech specs contain detailed algorithms showing essential logic flow while allowing implementation flexibility for error handling and specific Go idioms.