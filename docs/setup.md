# Switchboard V4 - Installation & Setup Guide

## Prerequisites

### Required Software
- **Go 1.21+** - [Download from golang.org](https://golang.org/dl/)
- **SQLite 3.35+** - Usually pre-installed on macOS/Linux
- **Git** - For version control

### Optional Development Tools
- **VS Code** with Go extension
- **Postman** or **curl** for API testing
- **WebSocket client** for connection testing

## Quick Start

### 1. Initialize Project Structure

```bash
# Create project directory
mkdir switchboard-v4
cd switchboard-v4

# Initialize Go module
go mod init switchboard

# Create directory structure
mkdir -p {cmd,internal,pkg,web,db,logs,config,docs,tests}
mkdir -p internal/{database,websocket,session,message,rate}
mkdir -p web/{static,templates}
mkdir -p tests/{unit,integration,load}
```

### 2. Project Directory Structure

```
switchboard-v4/
├── cmd/
│   └── server/
│       └── main.go                 # Application entry point
├── internal/
│   ├── database/
│   │   ├── manager.go              # DatabaseManager interface & implementation
│   │   ├── models.go               # Session and Message structs
│   │   └── migrations.sql          # Database schema
│   ├── websocket/
│   │   ├── connection.go           # Connection management
│   │   ├── handler.go              # WebSocket upgrade and routing
│   │   └── cleanup.go              # Connection lifecycle
│   ├── session/
│   │   ├── manager.go              # SessionManager interface
│   │   └── lifecycle.go            # Start/end session logic
│   ├── message/
│   │   ├── processor.go            # Message processing pipeline
│   │   ├── router.go               # Message routing by type
│   │   └── filter.go               # Role-based filtering
│   └── rate/
│       └── limiter.go              # Rate limiting implementation
├── pkg/
│   ├── config/
│   │   └── config.go               # Configuration constants
│   └── errors/
│       └── errors.go               # Standard error definitions
├── web/
│   ├── static/
│   │   ├── css/
│   │   ├── js/
│   │   └── index.html              # Test client interface
│   └── api/
│       └── handlers.go             # HTTP API endpoints
├── db/
│   ├── switchboard.db              # SQLite database (created at runtime)
│   ├── migrations/
│   │   └── 001_initial_schema.sql  # Database migrations
│   └── fixtures/
│       └── test_data.sql           # Test data for development
├── config/
│   ├── development.yaml            # Development configuration
│   ├── production.yaml             # Production configuration
│   └── test.yaml                   # Test configuration
├── tests/
│   ├── unit/
│   │   ├── session_test.go
│   │   ├── message_test.go
│   │   └── database_test.go
│   ├── integration/
│   │   ├── websocket_test.go
│   │   └── api_test.go
│   └── load/
│       └── concurrent_users_test.go
├── logs/
│   └── .gitkeep                    # Keep logs directory in git
├── docs/
│   ├── tech-specs-revised.md       # Architecture specification
│   ├── setup.md                    # This file
│   └── api.md                      # API documentation
├── go.mod                          # Go module definition
├── go.sum                          # Go module checksums
├── Makefile                        # Build and development commands
├── docker-compose.yml              # Development environment
├── Dockerfile                      # Production container
└── README.md                       # Project overview
```

### 3. Create Essential Files

```bash
# Create main application entry point
cat > cmd/server/main.go << 'EOF'
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    log.Println("Switchboard V4 starting...")
    
    // TODO: Initialize configuration
    // TODO: Initialize database
    // TODO: Initialize switchboard system
    // TODO: Start HTTP server
    // TODO: Setup graceful shutdown
    
    // Placeholder signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    log.Println("Server running. Press Ctrl+C to shutdown.")
    <-sigChan
    
    log.Println("Shutting down...")
}
EOF

# Create Go module dependencies
cat > go.mod << 'EOF'
module switchboard

go 1.21

require (
    github.com/gorilla/websocket v1.5.1
    github.com/mattn/go-sqlite3 v1.14.18
    gopkg.in/yaml.v3 v3.0.1
)
EOF

# Create database schema
cat > internal/database/migrations.sql << 'EOF'
-- Switchboard V4 Database Schema
-- Ultra-simple sessions and messages tables

-- Sessions table
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK (length(name) >= 1 AND length(name) <= 200),
    created_by TEXT NOT NULL,
    start_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    end_time DATETIME,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'ended'))
);

-- Messages table  
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('broadcast_to_instructors', 'direct_message', 'broadcast_to_students')),
    context TEXT NOT NULL DEFAULT 'general' CHECK (length(context) >= 1 AND length(context) <= 50),
    from_user TEXT NOT NULL CHECK (length(from_user) >= 1 AND length(from_user) <= 50),
    to_user TEXT,
    content TEXT NOT NULL CHECK (length(content) <= 65536),
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_start_time ON sessions(start_time DESC);
CREATE INDEX IF NOT EXISTS idx_messages_session_time ON messages(session_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_type_context ON messages(type, context);
CREATE INDEX IF NOT EXISTS idx_messages_to_user ON messages(to_user) WHERE to_user IS NOT NULL;

-- SQLite optimization pragmas
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = -64000;
PRAGMA wal_autocheckpoint = 1000;
EOF

# Create configuration structure
cat > pkg/config/config.go << 'EOF'
package config

import "time"

// Core system configuration constants
const (
    // Connection Management
    ConnectionSendBufferSize     = 100
    MaxMessageSize              = 64 * 1024
    HeartbeatInterval           = 30 * time.Second
    ConnectionTimeout           = 120 * time.Second
    ConnectionCleanupInterval   = 30 * time.Second
    
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
EOF

# Create standard error definitions
cat > pkg/errors/errors.go << 'EOF'
package errors

import "errors"

// Standard error types for internal operations
var (
    ErrNoActiveSession      = errors.New("no active session")
    ErrSessionAlreadyActive = errors.New("session already active")
    ErrConnectionNotFound   = errors.New("connection not found")
    ErrChannelFull         = errors.New("send channel full")
    ErrInvalidMessageType  = errors.New("invalid message type")
    ErrRateLimitExceeded   = errors.New("rate limit exceeded")
)
EOF

# Create development configuration
cat > config/development.yaml << 'EOF'
server:
  host: "localhost"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  
database:
  path: "./db/switchboard.db"
  
logging:
  level: "debug"
  file: "./logs/switchboard.log"
  
websocket:
  check_origin: true
  handshake_timeout: 10s
EOF

# Create Makefile for common tasks
cat > Makefile << 'EOF'
.PHONY: setup build run test clean dev

# Setup development environment
setup:
	go mod tidy
	go mod download
	mkdir -p db logs

# Build the application
build:
	go build -o bin/switchboard cmd/server/main.go

# Run in development mode  
run: build
	./bin/switchboard

# Run all tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f db/switchboard.db*

# Development mode with auto-reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air

# Initialize database
init-db:
	sqlite3 db/switchboard.db < internal/database/migrations.sql

# Load test data
load-fixtures:
	sqlite3 db/switchboard.db < db/fixtures/test_data.sql

# Database shell
db-shell:
	sqlite3 db/switchboard.db

# Format code
fmt:
	go fmt ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Build for production
build-prod:
	CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o bin/switchboard-prod cmd/server/main.go
EOF

# Create basic test client
cat > web/static/index.html << 'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>Switchboard V4 Test Client</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .container { max-width: 800px; margin: 0 auto; }
        .section { margin: 20px 0; padding: 10px; border: 1px solid #ccc; }
        textarea { width: 100%; height: 100px; }
        #messages { height: 300px; overflow-y: auto; border: 1px solid #ddd; padding: 10px; }
        button { padding: 10px 15px; margin: 5px; }
        .message { margin: 5px 0; padding: 5px; background: #f9f9f9; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Switchboard V4 Test Client</h1>
        
        <div class="section">
            <h3>Connection</h3>
            <input type="text" id="userId" placeholder="User ID" value="test_user">
            <select id="role">
                <option value="student">Student</option>
                <option value="instructor">Instructor</option>
            </select>
            <button onclick="connect()">Connect</button>
            <button onclick="disconnect()">Disconnect</button>
            <div id="status">Disconnected</div>
        </div>
        
        <div class="section">
            <h3>Session Control</h3>
            <input type="text" id="sessionName" placeholder="Session Name" value="Test Session">
            <button onclick="startSession()">Start Session</button>
            <button onclick="endSession()">End Session</button>
        </div>
        
        <div class="section">
            <h3>Send Message</h3>
            <select id="messageType">
                <option value="broadcast_to_instructors">Question for Instructors</option>
                <option value="direct_message">Direct Message</option>
                <option value="broadcast_to_students">Announcement to Students</option>
            </select>
            <input type="text" id="toUser" placeholder="To User (for direct messages)">
            <textarea id="messageContent" placeholder="Message content..."></textarea>
            <button onclick="sendMessage()">Send Message</button>
        </div>
        
        <div class="section">
            <h3>Messages</h3>
            <div id="messages"></div>
            <button onclick="clearMessages()">Clear</button>
        </div>
    </div>
    
    <script src="js/client.js"></script>
</body>
</html>
EOF

# Create basic JavaScript client
mkdir -p web/static/js
cat > web/static/js/client.js << 'EOF'
let ws = null;
let userId = '';
let role = '';

function connect() {
    userId = document.getElementById('userId').value;
    role = document.getElementById('role').value;
    
    const wsUrl = `ws://localhost:8080/ws?user_id=${userId}&role=${role}`;
    ws = new WebSocket(wsUrl);
    
    ws.onopen = function() {
        document.getElementById('status').textContent = 'Connected';
        addMessage('system', 'Connected to Switchboard');
    };
    
    ws.onmessage = function(event) {
        const message = JSON.parse(event.data);
        addMessage('received', JSON.stringify(message, null, 2));
    };
    
    ws.onclose = function() {
        document.getElementById('status').textContent = 'Disconnected';
        addMessage('system', 'Disconnected from Switchboard');
    };
    
    ws.onerror = function(error) {
        addMessage('error', 'WebSocket error: ' + error);
    };
}

function disconnect() {
    if (ws) {
        ws.close();
        ws = null;
    }
}

function startSession() {
    const sessionName = document.getElementById('sessionName').value;
    fetch('/api/session/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            name: sessionName,
            instructor_id: userId
        })
    })
    .then(response => response.json())
    .then(data => addMessage('api', 'Session started: ' + JSON.stringify(data)))
    .catch(error => addMessage('error', 'Session start error: ' + error));
}

function endSession() {
    fetch('/api/session/end', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            instructor_id: userId
        })
    })
    .then(response => response.json())
    .then(data => addMessage('api', 'Session ended: ' + JSON.stringify(data)))
    .catch(error => addMessage('error', 'Session end error: ' + error));
}

function sendMessage() {
    if (!ws) {
        addMessage('error', 'Not connected');
        return;
    }
    
    const messageType = document.getElementById('messageType').value;
    const toUser = document.getElementById('toUser').value;
    const content = document.getElementById('messageContent').value;
    
    const message = {
        type: messageType,
        context: 'general',
        content: { text: content }
    };
    
    if (messageType === 'direct_message' && toUser) {
        message.to_user = toUser;
    }
    
    ws.send(JSON.stringify(message));
    addMessage('sent', JSON.stringify(message, null, 2));
}

function addMessage(type, content) {
    const messages = document.getElementById('messages');
    const messageDiv = document.createElement('div');
    messageDiv.className = 'message';
    messageDiv.innerHTML = `<strong>[${type}]</strong> ${content}`;
    messages.appendChild(messageDiv);
    messages.scrollTop = messages.scrollHeight;
}

function clearMessages() {
    document.getElementById('messages').innerHTML = '';
}
EOF

# Create placeholder README
cat > README.md << 'EOF'
# Switchboard V4

Ultra-simplified real-time educational communication system.

## Quick Start

```bash
# Setup environment
make setup
make init-db

# Run development server
make run

# Open test client
open http://localhost:8080
```

See [docs/setup.md](docs/setup.md) for detailed setup instructions.
EOF

echo "Project structure created successfully!"
```

### 4. Install Dependencies

```bash
# Download Go dependencies
go mod tidy

# Create database
make init-db

# Optional: Install development tools
go install github.com/cosmtrek/air@latest  # Auto-reload for dev
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest  # Linter
```

### 5. Verify Installation

```bash
# Check Go installation
go version

# Check SQLite installation  
sqlite3 --version

# Test database creation
make init-db
sqlite3 db/switchboard.db ".tables"

# Build application
make build
```

## Development Workflow

### 1. Daily Development Commands

```bash
# Start development server with auto-reload
make dev

# Run tests during development
make test

# Check code formatting and linting
make fmt
make lint

# View database contents
make db-shell
```

### 2. Testing Setup

```bash
# Run all tests
make test

# Run with coverage report
make test-coverage
open coverage.html

# Load test data for manual testing
make load-fixtures
```

### 3. Database Management

```bash
# Open database shell
make db-shell

# View current schema
sqlite3 db/switchboard.db ".schema"

# Reset database (WARNING: deletes all data)
rm db/switchboard.db*
make init-db
```

## Environment Configuration

### Development Environment (default)
- **Database**: `./db/switchboard.db` (SQLite)
- **Server**: `localhost:8080`
- **Logging**: Debug level to `./logs/switchboard.log`
- **WebSocket**: Accepts all origins

### Production Environment
- **Database**: Configurable via `DATABASE_PATH` env var
- **Server**: Configurable via `HOST`/`PORT` env vars
- **Logging**: Info level to stdout
- **WebSocket**: Strict origin checking

### Environment Variables

```bash
# Database configuration
export DATABASE_PATH="/var/lib/switchboard/switchboard.db"

# Server configuration
export HOST="0.0.0.0"
export PORT="8080"

# Logging configuration  
export LOG_LEVEL="info"
export LOG_FILE="/var/log/switchboard/switchboard.log"
```

## Docker Development (Optional)

### Docker Compose Setup

```yaml
# docker-compose.yml
version: '3.8'
services:
  switchboard:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./db:/app/db
      - ./logs:/app/logs
    environment:
      - LOG_LEVEL=debug
```

### Commands

```bash
# Start with Docker Compose
docker-compose up --build

# Development with volume mounts
docker-compose -f docker-compose.dev.yml up
```

## Next Steps

1. **Implement Core Components** - Start with the interfaces defined in the tech specs
2. **Add Unit Tests** - Test each component as you build it
3. **Integration Testing** - Test WebSocket connections and message flow
4. **Load Testing** - Verify performance under concurrent load
5. **Production Deployment** - Configure for your hosting environment

## Troubleshooting

### Common Issues

**Database locked errors:**
```bash
# Check for existing connections
lsof db/switchboard.db
# Reset WAL files if needed
sqlite3 db/switchboard.db "PRAGMA wal_checkpoint(FULL);"
```

**Port already in use:**
```bash
# Find process using port 8080
lsof -i :8080
# Kill process if needed
kill -9 <PID>
```

**Permission errors:**
```bash
# Ensure directories are writable
chmod 755 db logs
```

### Debug Mode

```bash
# Run with verbose logging
LOG_LEVEL=debug make run

# Enable Go race detector
go run -race cmd/server/main.go
```

## Additional Resources

- [Technical Specification](tech-specs-revised.md) - Complete architecture details
- [API Documentation](api.md) - HTTP and WebSocket API reference
- [Performance Benchmarks](benchmarks.md) - Load testing results
- [Deployment Guide](deployment.md) - Production deployment instructions