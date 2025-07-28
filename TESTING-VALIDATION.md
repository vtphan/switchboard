
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
