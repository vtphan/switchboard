# Switchboard V4

**Ultra-simplified real-time educational communication system** designed for classroom environments. Switchboard V4 provides instant messaging between students and instructors with role-based privacy, session management, and educational-focused features.

## 🎯 System Overview

Switchboard V4 is built on an **ultra-simplified architecture** with a single global session state, eliminating complex session management while preserving essential educational communication patterns. The system supports real-time WebSocket communication with automatic message filtering, batched database writes, and graceful connection management.

### Key Features

- **Single Active Session Model** - One classroom session at a time with simple on/off state
- **Pre-Connection Support** - Students can connect before sessions start (waiting state)
- **3-Message Type System** - Simple, purpose-driven message types for educational use
- **Role-Based Privacy** - Students see filtered messages, instructors see everything
- **Real-Time Delivery** - WebSocket-based instant messaging with <10ms latency
- **Complete Session History** - Late joiners receive full context (role-filtered)
- **Message Gating** - No messages accepted without active session (clean boundaries)

## 🏗️ Core Architecture

### Single-Session State Model
- **One active session at a time** - Eliminates complex multi-session management
- **Atomic session operations** - Session start/end operations are database-consistent
- **Pre-connection waiting** - Users can connect before instructor starts session
- **Message gating** - Clean boundary: no messages without active session

### 3-Message Type System
```
1. broadcast_to_instructors  → Messages visible to all instructors
2. direct_message           → Private conversations between specific users  
3. broadcast_to_students    → Messeages visible to all students
```

### High-Performance Concurrency
- **Separate Mutexes** - `activeSessionMu` (RWMutex) and `connectionsMu` (RWMutex) for optimal performance
- **Single-Writer Database Pattern** - Eliminates SQLite lock contention using batched writes
- **WAL Mode SQLite** - Enables concurrent reads during writes
- **Buffered Channels** - Non-blocking message delivery (100-message buffers per connection)

### Educational Privacy Model
- **Instructors see all messages** - Complete oversight for classroom management
- **Students see filtered messages** - Only messages involving them directly or public announcements
- **Immutable audit trail** - All messages persisted with complete metadata
- **Role-based message routing** - Automatic filtering based on sender/recipient roles

## ⚡ Technical Requirements

### Performance Characteristics
- **Message Routing**: <10ms average latency for real-time delivery
- **Database Throughput**: 500+ messages/second sustained (batch processing)
- **Memory Usage**: ~2KB per connection, 15MB total for 50 concurrent users
- **Connection Capacity**: 50+ concurrent students and instructors
- **Message Size Limit**: 64KB per WebSocket message

### Core Technology Stack
- **Backend**: Go 1.21+ with Gorilla WebSocket
- **Database**: SQLite with WAL mode, optimized for educational workloads
- **Transport**: WebSocket over HTTP/HTTPS with automatic heartbeat monitoring
- **Message Format**: JSON with structured content and educational context
- **Configuration**: YAML-based with environment variable overrides

### Database Architecture
```sql
-- Ultra-simple schema optimized for educational use
sessions: id, name, created_by, start_time, end_time, status
messages: id, session_id, type, context, from_user, to_user, content, timestamp
```

### Built-in Resilience
- **Database Batching** - Messages collected and written in batches (100 messages or 200ms timeout)
- **Retry Logic** - Exponential backoff with dead letter queue for failed writes
- **Connection Recovery** - Automatic reconnection with session state restoration
- **Graceful Shutdown** - Multi-phase shutdown ensures data integrity

## 🚀 Quick Start

```bash
# Setup environment and dependencies
make setup
make init-db

# Run development server
make run

# Open web test client
open http://localhost:8080

# Run test suite
make test
```

## 📡 Client SDKs

The **[Switchboard SDK collection](./sdk/)** provides client libraries for connecting to Switchboard servers across multiple programming languages with consistent APIs and educational-focused features.

### JavaScript SDK (Complete)
- **Package**: `switchboard-client` on npm
- **Features**: WebSocket client, message search/filtering, TypeScript support
- **Examples**: Separate teacher/student applications with educational UI patterns
- **Usage**: Browser and Node.js environments

```javascript
// Student asking for help
student.broadcast_to_instructors('helpRequest')
  .withText("How do loops work?")
  .withCode("for i := 0; i < 10; i++", 'go')
  .send();

// Instructor responding
instructor.direct_message('student123', 'response')
  .withText("Here's how loops work...")
  .referencingMessage('msg-456')
  .send();
```

### SDK Architecture
- **Protocol Transparency** - Method names match exact WebSocket message types
- **Educational Context** - Semantic method names reflect classroom terminology  
- **Rich Content Support** - Text, code snippets, structured data, message references
- **No Role Restrictions** - Server handles all filtering, clients can send any message type

See **[SDK Documentation](./sdk/README.md)** for complete client implementation details.

## 📁 Project Structure

```
switchboard-v4/
├── cmd/server/main.go              # Application entry point
├── internal/                       # Private application code
│   ├── database/                   # DatabaseManager interface & SQLite implementation
│   ├── websocket/                  # WebSocket connection management with heartbeat
│   ├── session/                    # Session lifecycle (start/end) with atomic operations
│   ├── message/                    # Message processing, routing, role-based filtering
│   └── rate/                       # Rate limiting (100 messages/minute per user)
├── pkg/                           # Public packages
│   ├── config/                    # System configuration constants
│   └── errors/                    # Standard error definitions
├── web/                           # HTTP handlers and test client
│   ├── api/                       # REST API for session management
│   └── static/                    # Web-based test client interface
├── sdk/                           # Multi-language client SDKs
│   ├── javascript/                # Complete JavaScript/TypeScript SDK
│   └── python/                    # Planned Python SDK
├── tests/                         # Comprehensive test suite
│   ├── unit/                      # Component-level tests
│   ├── integration/               # End-to-end message flow tests
│   └── performance/               # Load testing (50+ concurrent users)
├── docs/                          # Architecture and setup documentation
└── db/                            # SQLite database and schema migrations
```

## 🔧 Development Workflow

### Common Commands
```bash
# Development with auto-reload
make dev                    # Requires: go install github.com/cosmtrek/air@latest

# Testing
make test                   # Run all tests
make test-coverage          # Generate coverage report
go test -run TestSpecific   # Run specific test

# Database management
make db-shell              # Open SQLite interactive shell
make load-fixtures         # Load test data
sqlite3 db/switchboard.db ".schema messages"  # View schema

# Code quality
make fmt                   # Format Go code
make lint                  # Run golangci-lint
```

### Testing Strategy
- **Unit Tests** - Session state management, message filtering, database batching
- **Integration Tests** - WebSocket flows, API endpoints, complete message pipeline
- **Performance Tests** - 50+ concurrent connections, message throughput validation
- **Load Testing** - Memory stability, connection cleanup, database performance

## ⚙️ Configuration

### Critical Constants (pkg/config/config.go)
```go
// Connection Management
ConnectionSendBufferSize = 100     // Channel buffer per WebSocket connection
InactiveConnectionTimeout = 25 * time.Minute  // Cleanup inactive connections

// Database Performance  
DatabaseBatchSize = 100            // Messages per batch write
DatabaseFlushInterval = 200 * time.Millisecond  // Max batch wait time

// Rate Limiting
RateLimitMaxMessages = 100         // Messages per user per minute
```

### Environment Configuration
```bash
# Database
export DATABASE_PATH="./db/switchboard.db"

# Server
export HOST="localhost"
export PORT="8080"

# Logging
export LOG_LEVEL="info"
```

## 📖 Documentation

- **[Technical Specifications](docs/tech-specs.md)** - Complete architecture with algorithms
- **[Setup Instructions](docs/setup.md)** - Detailed development environment setup
- **[Go Patterns Reference](docs/go-patterns-reference.md)** - Approved concurrency patterns
- **[SDK Documentation](sdk/README.md)** - Client library implementation guide

## 🎓 Educational Use Cases

Switchboard V4 is optimized for classroom scenarios:

- **Live Coding Sessions** - Students ask questions, instructors provide real-time help
- **Code Review** - Students submit code, receive private feedback from instructors
- **Announcements** - Instructors broadcast important information to entire class
- **Office Hours** - One-on-one help sessions with message history
- **Group Exercises** - Structured communication during collaborative work

## 🚦 Current Status

**✅ Fully Implemented:**
- Core messaging system with 3-message types
- WebSocket connection management with session-aware cleanup
- SQLite database with automatic batching (500+ messages/second)
- HTTP API for session management
- Role-based message filtering for educational privacy
- Rate limiting and connection monitoring
- JavaScript SDK with educational examples
- Comprehensive test suite (unit, integration, performance)

**🔄 Production Ready:**
- Multi-phase graceful shutdown
- Database retry logic with dead letter queue
- WebSocket heartbeat monitoring
- Memory-efficient connection management
- Educational-focused error handling

See [docs/setup.md](docs/setup.md) for detailed development setup instructions.