# Switchboard - Real-Time Educational Communication System

A real-time messaging system designed for educational environments, enabling seamless communication between instructors and students through WebSocket connections with **single active session architecture** and **auto-assignment** capabilities.

## 🏗️ Architecture Overview

Switchboard implements a single active session architecture with lobby system and auto-assignment for simplified classroom management:

```
                    ┌─────────────────────────────────────────────────────────────┐
                    │                    Switchboard Server                        │
                    │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
                    │  │   Session   │  │   Message   │  │     Database        │  │
                    │  │  Management │  │   Router    │  │   (SQLite + WAL)    │  │
                    │  │  (Single)   │  │    Hub      │  │                     │  │
                    │  └─────────────┘  └─────────────┘  └─────────────────────┘  │                          │                    
    ┌───────────────┤  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
    │  WebSocket    │  │    REST     │  │ Connection  │  │   Auto-Assignment   │  │
    │   Handler     │  │     API     │  │  Registry   │  │   & Presence Mgmt   │  │
    │ (Auto-Assign) │  │             │  │             │  │                     │  │
    └───────────────┤  └─────────────┘  └─────────────┘  └─────────────────────┘  │
                    └─────────────────────────────────────────────────────────────┘
                                            │
                           ┌────────────────┼────────────────┐
                           │                │                │
                    ┌──────▼──────┐         │         ┌──────▼──────┐
                    │  LOBBY      │         │         │  LOBBY      │
              ┌─────┤ Instructor  │         │         │   Student   │─────┐
              │     │   Client    │         │         │   Client    │     │
              │     │             │         │         │             │     │
              │     │ Universal   │         │         │ Enrolled    │     │
              │     │ Access +    │         │         │ Students    │     │
              │     │ Session     │    Auto-Assign    │ Auto-Assign │     │
              │     │ Control     │         │         │ to Session  │     │
              │     └─────────────┘         │         └─────────────┘     │
              │            │                │                │            │
              │     ┌──────▼──────┐         │         ┌──────▼──────┐     │
              │     │  SESSION    │         │         │  SESSION    │     │
              └─────┤ Instructor  │         │         │   Student   │─────┘
                    │   Client    │         │         │   Client    │
                    │             │         │         │             │
                    │ - Session   │         │         │ - Message   │
                    │   Management│         │         │   Exchange  │
                    │ - All Msg   │         │         │ - Analytics │
                    │   Types     │         │         │ - History   │
                    │ - History   │         │         │   Replay    │
                    └─────────────┘         │         └─────────────┘
                                            │
                                     ┌──────▼──────┐
                                     │  Non-Enrolled│
                                     │   Student    │
                                     │ (Lobby Only) │
                                     └─────────────┘

Single Session Flow:
├─ instructor_broadcast: Instructor → All Students (in session)
├─ instructor_inbox:     Student → All Instructors (in session)  
├─ request:              Instructor → Specific Student
├─ request_response:     Student → All Instructors
├─ analytics:            Student → All Instructors
└─ inbox_response:       Instructor → Specific Student

Auto-Assignment Logic:
├─ Instructors: Auto-assigned to any active session (universal access)
├─ Students: Auto-assigned to active session if enrolled, otherwise lobby
├─ Lobby System: Persistent connections for presence and session notifications
└─ Single Session: Only one session can be active at any time
```

## 🚀 Key Features

### **Single Session Architecture**
- **Only one active session** at any time for simplified classroom management
- **Conflict detection** with helpful HTTP 409 responses for session creation attempts
- **Clear session lifecycle** with explicit start/end control

### **Auto-Assignment System**
- **Instructors**: Automatically assigned to any active session
- **Students**: Auto-assigned to active session if enrolled, otherwise placed in lobby
- **Seamless transitions** between lobby and session states without reconnection

### **Lobby System**
- **Persistent WebSocket connections** for users not in active sessions
- **Real-time presence updates** across all connected users
- **Session notifications** when sessions become available

### **Message Types & Routing**
6 message types with role-based permissions and dynamic routing:

| Message Type | From | To | Purpose |
|-------------|------|----|---------| 
| `instructor_inbox` | Student | All Instructors | Questions, help requests |
| `inbox_response` | Instructor | Specific Student | Answers, guidance |
| `request` | Instructor | Specific Student | Code/output requests |
| `request_response` | Student | All Instructors | Submissions, responses |
| `analytics` | Student | All Instructors | Engagement, progress data |
| `instructor_broadcast` | Instructor | All Students | Announcements, instructions |

### **Performance & Reliability**
- **Single-writer patterns** prevent race conditions (WebSocket writes, database operations)
- **Rate limiting**: 100 messages/minute per client
- **Message persistence** with history replay for new connections
- **Resource leak prevention** with proper cleanup patterns

## 📋 Prerequisites

- **Go 1.21 or later**
- **SQLite3**
- **Make** (recommended)

## 🔧 Quick Start

### Building and Running

```bash
# Build the application
make build

# Run in development mode (with logging)
make dev

# Run with production settings
make run
```

### Manual Build

```bash
# Build
go build -o bin/switchboard cmd/switchboard/*.go

# Run
./bin/switchboard
```

### Configuration

The server uses environment variables for configuration:

```bash
# Server configuration
HTTP_HOST=127.0.0.1
HTTP_PORT=8080

# Database configuration  
DATABASE_PATH=./switchboard.db
DATABASE_TIMEOUT=30s

# WebSocket configuration
WEBSOCKET_PING_INTERVAL=30s
WEBSOCKET_READ_TIMEOUT=60s
WEBSOCKET_WRITE_TIMEOUT=10s
WEBSOCKET_BUFFER_SIZE=100
```

## 🧪 Testing

### **Core Test Commands**

```bash
# Run all tests
make test

# Run tests with race detection (CRITICAL for concurrent code)
make test-race

# Run tests with verbose output
make test-verbose

# Generate test coverage report (target: 85%+ for critical code)
make coverage
```

### **Individual Component Tests**

```bash
# Test WebSocket connection handling
go test ./internal/websocket -v

# Test message routing and rate limiting
go test ./internal/router -v

# Test session management (single session enforcement)
go test ./internal/session -v

# Test database operations and persistence
go test ./internal/database -v

# Test integration scenarios
go test ./tests/integration -v
```

### **Testing with Race Detection**

Race detection is **essential** for WebSocket and concurrent code:

```bash
# Test specific components with race detection
go test -race ./internal/websocket -v
go test -race ./internal/hub -v
go test -race ./internal/router -v

# Run multiple iterations to catch intermittent races
go test -race -count=10 ./internal/websocket -run TestConcurrentWrites
```

### **Test Categories**

The test suite includes:

#### **Unit Tests**
- **Connection Management**: WebSocket connection lifecycle, cleanup, error handling
- **Message Routing**: All 6 message types, role permissions, rate limiting
- **Session Management**: Single session enforcement, validation, state transitions
- **Database Operations**: CRUD operations, concurrent access, transaction handling

#### **Integration Tests**
- **Component Integration**: System initialization and component interaction
- **End-to-End Flows**: Complete message flow from sender to recipient
- **Auto-Assignment**: Lobby to session transitions and role-based assignment

#### **Concurrency Tests**
- **Thread Safety**: Concurrent WebSocket writes, connection replacement
- **Resource Management**: Goroutine cleanup, memory leak prevention
- **Single-Writer Patterns**: Database write coordination, connection registry updates

### **Performance Testing**

```bash
# Run performance benchmarks
make benchmark

# Profile memory usage
go test -memprofile=mem.prof ./internal/websocket -run TestConcurrentWrites
go tool pprof mem.prof

# Profile CPU usage  
go test -cpuprofile=cpu.prof ./internal/router -run TestRouteMessage
go tool pprof cpu.prof

# Trace goroutine behavior
go test -trace=trace.out ./internal/hub -run TestMessageRouting
go tool trace trace.out
```

### **Test Execution Patterns**

```bash
# Quick validation during development
go test -short ./... -v

# Test specific functionality
go test ./internal/session -run TestCreateSession -v

# Debug test failures with maximum verbosity
go test -v -failfast ./internal/database -run TestConcurrentWrites

# Run tests multiple times to catch race conditions
go test -count=20 -race ./internal/websocket

# Test with custom timeout for integration tests
go test ./tests/integration -timeout=5m -v
```

## ✅ Validation & Quality Assurance

### **Comprehensive Validation**

```bash
# Run all validation checks (REQUIRED before commit)
make validate

# Individual validation steps
make vet           # Go vet analysis
make lint          # Static analysis (golangci-lint) 
make vulnerability # Security vulnerability check (govulncheck)
```

### **Code Quality Standards**

The validation pipeline enforces:
- **No race conditions** in concurrent code
- **85%+ test coverage** for critical components  
- **Static analysis** compliance (golangci-lint)
- **Security vulnerability** checks
- **Memory and goroutine leak** prevention

### **Development Workflow**

```bash
# Quick development check
go test -short ./... && make vet

# Pre-commit validation
make validate

# Performance regression testing
go test -bench=. -benchmem ./... > current_bench.txt
```

## 🌐 API Endpoints

### **WebSocket Connection**

```
ws://localhost:8080/ws?user_id=<id>&role=<instructor|student>[&session_id=<id>]
```

**Connection Examples:**
```bash
# Auto-assignment (recommended)
ws://localhost:8080/ws?user_id=student123&role=student

# Specific session connection
ws://localhost:8080/ws?user_id=instructor1&role=instructor&session_id=abc123

# Explicit lobby connection
ws://localhost:8080/ws?user_id=student456&role=student&session_id=lobby
```

**Auto-Assignment Behavior:**
- **Instructors**: Assigned to active session if exists, otherwise lobby
- **Students**: Assigned to active session if enrolled, otherwise lobby
- **No Active Session**: All users assigned to lobby

### **REST API**

```bash
# Health check
GET /health

# Session management
POST /api/sessions              # Create new session
GET  /api/sessions/{id}         # Get session details  
GET  /api/sessions              # List active sessions
DELETE /api/sessions/{id}       # End session
```

**Session Creation:**
```bash
curl -X POST http://localhost:8080/api/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Math Class - Chapter 5",
    "instructor_id": "instructor1", 
    "student_ids": ["student1", "student2", "student3"]
  }'
```

**Single Session Enforcement:**
```bash
# Returns HTTP 409 if active session exists
{
  "error": "Active session exists",
  "message": "Cannot create new session. Active session 'Previous Class' must be ended first.",
  "active_session": {
    "id": "existing-session-id",
    "name": "Previous Class",
    "created_by": "instructor2",
    "status": "active"
  }
}
```

## 📁 Project Structure

```
switchboard/
├── cmd/switchboard/           # Application entry point
│   └── main.go               # Server startup and configuration
├── internal/                 # Private application code
│   ├── api/                  # REST API handlers
│   │   └── server.go         # Session management endpoints
│   ├── app/                  # Application coordination
│   │   └── application.go    # Startup orchestration
│   ├── config/               # Configuration management
│   │   └── config.go         # Environment variable handling
│   ├── database/             # Database operations
│   │   ├── manager.go        # Single-writer database manager
│   │   └── manager_test.go   # Database operation tests
│   ├── hub/                  # Message coordination
│   │   ├── hub.go           # Message hub orchestration
│   │   └── errors.go        # Hub-specific errors
│   ├── router/               # Message routing logic
│   │   ├── router.go         # Message type routing
│   │   ├── rate_limiter.go   # Per-client rate limiting
│   │   ├── errors.go         # Router-specific errors
│   │   └── simple_router_test.go  # Router unit tests
│   ├── session/              # Session management
│   │   ├── manager.go        # Single session enforcement
│   │   ├── errors.go         # Session-specific errors  
│   │   └── manager_test.go   # Session management tests
│   └── websocket/            # WebSocket handling
│       ├── connection.go     # Connection wrapper (single-writer)
│       ├── handler.go        # WebSocket endpoints & auto-assignment
│       ├── registry.go       # Connection registry & presence
│       ├── errors.go         # WebSocket-specific errors
│       ├── connection_test.go      # Connection handling tests
│       └── simple_registry_test.go # Registry functionality tests
├── pkg/                      # Public library code  
│   ├── database/             # Database configuration
│   │   ├── config.go         # Database connection settings
│   │   ├── migrations.go     # Migration management
│   │   └── schema.go         # Database schema definitions
│   ├── interfaces/           # Interface definitions
│   │   ├── connection.go     # Connection interface
│   │   ├── database.go       # Database manager interface
│   │   ├── router.go         # Message router interface
│   │   ├── session.go        # Session manager interface
│   │   └── errors.go         # Standard error definitions
│   └── types/                # Core data structures
│       ├── types.go          # Message, Session, Client types
│       ├── validation.go     # Input validation functions
│       └── errors.go         # Type-specific errors
├── tests/                    # Test suites
│   └── integration/          # Integration tests
│       └── simple_integration_test.go  # System integration tests
├── migrations/               # Database migrations
│   └── 001_initial_schema.sql     # Initial database schema
├── sdk/                      # Client SDKs
│   ├── python/               # Python SDK
│   ├── javascript/           # JavaScript/TypeScript SDK
│   └── hint-master/          # Example AI expert clients
├── docs/                     # Documentation
│   └── switchboard-tech-specs.md  # Technical specifications
├── planning/                 # Project documentation  
├── Makefile                  # Build and test commands
├── CLAUDE.md                 # Development instructions for Claude
└── README.md                 # This file
```

## 🎯 Performance Targets

The system is designed for classroom-scale performance:

- **Session validation**: <1ms (in-memory cache)
- **Message routing**: <10ms for broadcasts  
- **Database writes**: <50ms (single-writer pattern)
- **WebSocket throughput**: 1000+ messages/second per connection
- **Memory usage**: ~1MB for 50 concurrent users
- **Auto-assignment**: <10ms for session assignment checks

## 🏛️ Architecture Principles

### **Single-Writer Patterns (MANDATORY)**
- **WebSocket connections**: One writeLoop goroutine per connection
- **Database operations**: One writeLoop goroutine for all DB writes  
- **Prevents race conditions** and ensures data consistency

### **Channel-Based Communication**
- **Hub coordination**: Channels with appropriate buffer sizes (1000-message buffer)
- **Database writes**: Channel-based write coordination (100-message buffer)
- **Graceful shutdown**: Context-based cancellation

### **Resource Management**  
- **Connection cleanup**: Idempotent Close() methods
- **Goroutine cleanup**: No leaks after shutdown
- **Memory management**: Efficient cleanup of stale connections

### **Single Session Enforcement**
- **Business logic**: Only one session can be active at any time
- **Database constraint**: Status-based session validation
- **API behavior**: HTTP 409 conflicts guide proper workflow

## 🤝 Contributing

### **Development Workflow**

1. **Run tests**: `make test`
2. **Check race conditions**: `make test-race`  
3. **Validate code quality**: `make validate`
4. **Maintain coverage**: `make coverage`

### **Code Quality Requirements**

- **All tests must pass** including race detection
- **85%+ test coverage** for critical components
- **No static analysis violations** (golangci-lint)
- **No security vulnerabilities** (govulncheck)
- **Proper resource cleanup** in all error paths

### **Testing Guidelines**

- **Always use race detection** for concurrent code
- **Test single-writer patterns** thoroughly  
- **Validate auto-assignment logic** with various scenarios
- **Test error handling paths** and resource cleanup
- **Include integration tests** for component interactions

For detailed architectural information, see `docs/switchboard-tech-specs.md` and `CLAUDE.md`.

## 📄 License

[License information to be added]