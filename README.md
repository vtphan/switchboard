# Switchboard - Real-Time Classroom Messaging System

A real-time messaging system designed for educational environments, enabling seamless communication between instructors and students through WebSocket connections.

## Architecture Overview

```
                    ┌─────────────────────────────────────────────────────────────┐
                    │                    Switchboard Server                        │
                    │                                                             │
    ┌───────────────┤  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
    │               │  │   Session   │  │   Message   │  │     Database        │ │
    │  WebSocket    │  │  Management │  │   Router    │  │   (SQLite)          │ │
    │   Handler     │  │             │  │    Hub      │  │                     │ │
    │               │  └─────────────┘  └─────────────┘  └─────────────────────┘ │
    └───────────────┤                                                           │
                    │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
                    │  │    REST     │  │ Connection  │  │   Rate Limiting     │ │
                    │  │     API     │  │  Registry   │  │   & Validation      │ │
                    │  │             │  │             │  │                     │ │
                    │  └─────────────┘  └─────────────┘  └─────────────────────┘ │
                    └─────────────────────────────────────────────────────────────┘
                                            │
                           ┌────────────────┼────────────────┐
                           │                │                │
                    ┌──────▼──────┐         │         ┌──────▼──────┐
                    │             │         │         │             │
              ┌─────┤ Instructor  │         │         │   Student   │─────┐
              │     │   Client    │         │         │   Client    │     │
              │     │             │         │         │             │     │
              │     │ - Web UI    │         │         │ - AI Expert │     │
              │     │ - Session   │         │         │ - Hint Gen  │     │
              │     │   Mgmt      │         │         │ - Analytics │     │
              │     │ - Problem   │         │         │ - Response  │     │
              │     │   Config    │         │         │   Tracking  │     │
              │     └─────────────┘         │         └─────────────┘     │
              │                             │                             │
              │     ┌─────────────┐         │         ┌─────────────┐     │
              │     │ Instructor  │         │         │   Student   │     │
              └─────┤   Client    │         │         │   Client    │─────┘
                    │     #2      │         │         │     #2      │
                    └─────────────┘         │         └─────────────┘
                                            │
                                     ┌──────▼──────┐
                                     │             │
                                     │   Student   │
                                     │   Client    │
                                     │     #N      │
                                     │             │
                                     └─────────────┘

Message Flow Examples:
├─ instructor_broadcast: Instructor → Switchboard → All Students
├─ instructor_inbox:     Student → Switchboard → All Instructors  
├─ request:              Instructor → Switchboard → Specific Student
├─ request_response:     Student → Switchboard → All Instructors
├─ analytics:            Student → Switchboard → All Instructors
└─ inbox_response:       Instructor → Switchboard → Specific Student

Connection Details:
├─ WebSocket: ws://localhost:8080/ws?user_id=<id>&role=<role>&session_id=<id>
├─ Authentication: Query parameter based (user_id, role, session_id)
├─ Heartbeat: 30s ping/pong, 120s stale cleanup
└─ Rate Limiting: 100 messages/minute per client
```

Switchboard is built with a layered architecture following a 5-phase development approach:

1. **Foundation Layer** (`pkg/`) - Core data structures and interfaces
2. **WebSocket Infrastructure** (`internal/websocket/`) - Connection management and messaging
3. **Message Routing System** (`internal/router/`, `internal/hub/`) - Message processing and delivery
4. **Session Management** (`internal/session/`, `internal/database/`) - Session lifecycle and persistence
5. **API Layer** (`internal/api/`, `cmd/switchboard/`) - REST and WebSocket endpoints

### Message Types

The system supports 6 message types with specific routing patterns:

| Message Type | Sender | Recipients | Key Requirement |
|-------------|--------|------------|-----------------|
| `instructor_inbox` | Student | All session instructors | No to_user field |
| `inbox_response` | Instructor | Specific student | Requires to_user |
| `request` | Instructor | Specific student | Requires to_user |
| `request_response` | Student | All session instructors | No to_user field |
| `analytics` | Student | All session instructors | No to_user field |
| `instructor_broadcast` | Instructor | All session students | No to_user field |

## Quick Start

### Prerequisites

- Go 1.21 or later
- SQLite3
- Make (optional, but recommended)

### Building and Running

```bash
# Build the application
make build

# Run in development mode
make dev

# Run with production settings
make run
```

### Manual Build

```bash
# Build
go build -o switchboard cmd/switchboard/main.go

# Run
./switchboard
```

## Testing

### Full Test Suite

```bash
# Run all tests
make test

# Run tests with race detection (critical for WebSocket code)
make test-race

# Generate test coverage report (target: 85%+ for critical code)
make coverage

# Run performance benchmarks
make benchmark

# Run load tests for real-time components
make load-test
```

### Individual Test Packages

```bash
# Test specific packages
go test ./pkg/types -v
go test ./internal/websocket -v
go test ./internal/router -v
go test ./internal/session -v
go test ./internal/database -v
```

### Core Workflow Scenario Tests

The `tests/scenarios/` directory contains comprehensive classroom simulation tests. These tests validate realistic interaction patterns:

#### Run Individual Scenario Tests

**Complete Q&A Session Test**
```bash
# Simulates instructor question broadcast and student responses
go test ./tests/scenarios -run TestCompleteQASession -v
```

**Code Review Session Test**
```bash
# Simulates code request, submission, and feedback cycle
go test ./tests/scenarios -run TestCodeReviewSession -v
```

**Real-Time Analytics Test**
```bash
# Simulates student analytics collection and instructor monitoring
go test ./tests/scenarios -run TestRealTimeAnalytics -v
```

**Multi-Context Communication Test**
```bash
# Simulates all message types with various contexts simultaneously
go test ./tests/scenarios -run TestMultiContextCommunication -v
```

#### Edge Cases and Advanced Testing

**Edge Case Validation Tests**
```bash
# Test invalid message handling and error cases
go test ./tests/scenarios -run TestEdgeCases -v

# Test boundary conditions and limits
go test ./tests/scenarios -run TestBoundaryConditions -v

# Test malformed input handling  
go test ./tests/scenarios -run TestInputValidation -v
```

**Advanced Scenario Tests** *(Future Implementation)*
```bash
# These tests are planned for future development
go test ./tests/scenarios -run TestAdvancedScenarios -v
go test ./tests/scenarios -run TestConcurrentMultiSession -v
go test ./tests/scenarios -run TestInstructorCollaboration -v
go test ./tests/scenarios -run TestConnectionReplacement -v
go test ./tests/scenarios -run TestLargeClassSession -v
```

**Foundation Layer Unit Tests**
```bash
# Test core data structures
go test ./pkg/types -run TestMessageValidation -v
go test ./pkg/types -run TestSessionStructure -v

# Test interface contracts
go test ./pkg/interfaces -v

# Test database configuration
go test ./pkg/database -v
```

### Integration Tests

```bash
# Test component interactions
go test ./tests/integration -v

# Test message flow end-to-end
go test ./tests/integration -run TestMessageFlow -v
```

### Load and Stress Testing

The load testing suite validates system performance under realistic classroom conditions. **Note: Load tests are automatically skipped in short mode.**

#### Run All Load Tests
```bash
# Run comprehensive load test suite (30+ minute execution time)
make load-test

# Or run directly with timeout
go test ./tests/scenarios -run "TestClassroomScaleLoad|TestMessageBurstHandling|TestConcurrentSessionsLoad|TestConnectionStabilityStress" -timeout=30m -v
```

#### Individual Load Tests

**Classroom Scale Load Test** (5-minute duration)
```bash
# Simulates realistic classroom: 30 students + 3 instructors for 5 minutes
# Validates: <50ms message routing, >99% delivery success, 1000+ msg/sec throughput
go test ./tests/scenarios -run TestClassroomScaleLoad -timeout=10m -v
```

**Message Burst Handling Test** (2-minute duration)
```bash
# Tests system resilience under message bursts and rate limiting
# Simulates: All 30 students responding within 10 seconds + instructor broadcasts
go test ./tests/scenarios -run TestMessageBurstHandling -timeout=5m -v
```

**Concurrent Sessions Load Test** (3-minute duration)
```bash
# Tests multiple classroom sessions simultaneously with cross-session isolation
# Simulates: 3 concurrent sessions with 10 students each
go test ./tests/scenarios -run TestConcurrentSessionsLoad -timeout=10m -v
```

**Connection Stability Stress Test** (4-minute duration)
```bash
# Tests connection resilience with random disconnections and network simulation
# Simulates: Random disconnection/reconnection patterns + network latency
go test ./tests/scenarios -run TestConnectionStabilityStress -timeout=10m -v
```

#### Performance Benchmarks
```bash
# Benchmark message processing throughput
go test ./tests/scenarios -bench=BenchmarkMessageThroughput -benchtime=5s

# Benchmark connection establishment performance
go test ./tests/scenarios -bench=BenchmarkLoadTestConnectionSetup -benchtime=5s

# Run all benchmarks
make benchmark
```

### Testing Specific Scenarios

```bash
# Test specific message types
go test ./tests/scenarios -run TestContextFieldHandling -v

# Test WebSocket connection handling
go test ./tests/integration -run TestWebSocketIntegration -v

# Test session management
go test ./tests/integration -run TestSessionManagement -v
```

## Validation and Quality Assurance

### Code Quality Checks

```bash
# Run all validation checks (must pass before commit)
make validate

# Individual validation steps
make lint                 # Static analysis (golangci-lint)
make security            # Security analysis (gosec)
make vulnerability       # Check for vulnerabilities (govulncheck)
```

### Resource Leak Detection

```bash
# Check for memory leaks
make leak-test

# Check for goroutine leaks (critical for WebSocket handlers)
make goroutine-test
```

### Database Operations

```bash
# Apply database migrations
make migrate-up

# Rollback database (destructive)
make migrate-down
```

## Development Guidelines

### Performance Targets

- Session validation: <1ms (using in-memory cache)
- Message routing: <10ms for broadcasts
- Database writes: <50ms (single-writer pattern)
- WebSocket message throughput: 1000+ messages/second per connection
- Memory usage: ~1MB for 50 concurrent users

### Critical Patterns

**Single-Writer Pattern (MANDATORY)**:
- WebSocket connections: One writeLoop goroutine per connection
- Database operations: One writeLoop goroutine for all DB writes
- Prevents race conditions and data corruption

**Channel Communication**:
- Hub coordinates via channels with appropriate buffer sizes
- Use select with context for graceful shutdown

**Resource Management**:
- Connection cleanup: Close() must be idempotent
- Goroutine cleanup: No leaks after shutdown
- Channel cleanup: Proper closure signaling

### Test Execution Options

#### Common Testing Patterns

```bash
# Run a single test with verbose output  
go test -run TestSpecificFunction ./path/to/package -v

# Run tests with race detection (always use for concurrent code)
go test -race ./internal/websocket -run TestConcurrentWrites

# Run tests with custom timeout
go test ./tests/scenarios -timeout=5m -v

# Run tests multiple times to catch race conditions
go test -count=10 ./internal/hub -run TestMessageRouting

# Skip long-running tests (load tests automatically skip)
go test -short ./tests/scenarios -v

# Run with detailed coverage
go test -cover -coverprofile=coverage.out ./internal/router -v
```

#### Test Filtering and Selection

```bash
# Run multiple specific tests by pattern
go test ./tests/scenarios -run "TestComplete.*|TestCode.*" -v

# Run all tests except load tests (using short mode)
go test -short ./... -v

# Run only foundation layer tests
go test ./pkg/... -v

# Run only internal component tests
go test ./internal/... -v

# Run tests for specific message types
go test ./tests/scenarios -run TestContextFieldHandling -v
```

#### Debugging and Troubleshooting

```bash
# Run tests with maximum verbosity and detailed output
go test -v -x ./tests/scenarios -run TestCompleteQASession

# Debug test failures with detailed error information
go test -v -failfast ./tests/scenarios -run TestMessageBurstHandling

# Profile memory usage during tests
go test -memprofile=mem.prof ./tests/scenarios -run TestClassroomScaleLoad
go tool pprof mem.prof

# Profile CPU usage during tests  
go test -cpuprofile=cpu.prof ./tests/scenarios -run TestConnectionStabilityStress
go tool pprof cpu.prof

# Run tests with trace for goroutine analysis
go test -trace=trace.out ./internal/websocket -run TestConcurrentWrites
go tool trace trace.out
```

#### Continuous Integration Testing

```bash
# Full validation pipeline (recommended for CI)
make validate

# Quick validation for development
go test -short ./... && make lint

# Performance regression testing
go test -bench=. -benchmem ./... > current_bench.txt
# Compare with: benchcmp baseline.txt current_bench.txt
```

### Test Environment Setup

#### Required Test Dependencies

```bash
# Install development tools (if not already installed)
make install-tools

# Verify test environment
go version  # Requires Go 1.21+
sqlite3 -version  # For database tests
```

#### Test Database Setup

Tests automatically create temporary databases, but for manual testing:

```bash
# Setup test database
make migrate-up

# Clean test artifacts
make clean
```

#### Best Practices for Test Execution

**Development Workflow:**
```bash
# Quick validation during development (skips load tests)
go test -short ./... -v

# Test specific components you're working on
go test ./internal/router -v -race

# Full validation before commit
make validate
```

**Load Test Execution:**
```bash
# Ensure sufficient system resources for load tests
# Recommended: 8GB+ RAM, 4+ CPU cores

# Run load tests individually for debugging
go test ./tests/scenarios -run TestClassroomScaleLoad -v -timeout=10m

# Monitor system resources during load tests
# Use: htop, Activity Monitor, or similar tools
```

**Troubleshooting Test Failures:**

1. **WebSocket Connection Issues:**
   ```bash
   # Check for port conflicts
   netstat -an | grep LISTEN
   
   # Run with verbose WebSocket logging
   go test ./tests/scenarios -run TestCompleteQASession -v
   ```

2. **Database Conflicts:**
   ```bash
   # Clean up test databases
   rm -f /tmp/switchboard_test_*.db
   
   # Check database locks
   lsof switchboard.db
   ```

3. **Race Condition Detection:**
   ```bash
   # Always run concurrent tests with race detection
   go test -race ./internal/websocket -v
   
   # Run multiple times to catch intermittent races
   go test -count=20 -race ./internal/hub -run TestMessageRouting
   ```

4. **Performance Issues:**
   ```bash
   # Profile slow tests
   go test -cpuprofile=cpu.prof ./tests/scenarios -run TestSlowTest
   go tool pprof cpu.prof
   ```

## API Endpoints

### WebSocket Connection
```
ws://localhost:8080/ws?user_id=<id>&role=<instructor|student>&session_id=<session_id>
```

### REST Endpoints
```
GET  /health                    # Health check
POST /sessions                  # Create session
GET  /sessions/{id}            # Get session info
POST /sessions/{id}/end        # End session
```

## Configuration

Environment variables and configuration options:

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

## Project Structure

```
switchboard/
├── cmd/switchboard/           # Application entry point
├── internal/                  # Private application code
│   ├── api/                  # REST API handlers
│   ├── app/                  # Application setup and coordination
│   ├── config/               # Configuration management
│   ├── database/             # Database operations
│   ├── hub/                  # Message hub coordination
│   ├── router/               # Message routing logic
│   ├── session/              # Session management
│   └── websocket/            # WebSocket handling
├── pkg/                      # Public library code
│   ├── database/             # Database configuration
│   ├── interfaces/           # Interface definitions
│   └── types/                # Core data structures
├── tests/                    # Test suites
│   ├── fixtures/             # Test infrastructure (ScenarioRunner, TestClient, etc.)
│   ├── integration/          # Integration tests
│   └── scenarios/            # Classroom scenario tests & load testing
├── migrations/               # Database migrations
└── planning/                 # Project documentation
```

## Contributing

1. Ensure all tests pass: `make test`
2. Run validation checks: `make validate`
3. Test for race conditions: `make test-race`
4. Maintain test coverage: `make coverage`

For detailed architectural information, see `CLAUDE.md` and the `planning/` directory.

## License

[License information to be added]