# /create-phases [spec-file]

Build implementation plan for layer-by-layer architecture with integrated architectural and functional validation.

## What This Command Does
Breaks down specs into phases (architectural layers) and steps (focused implementations), each with architectural validation (design integrity) and functional validation (behavior correctness). Sets up discovery tracking.

## Usage
```bash
/create-phases                        # Uses technical-specs-v4.md (default)
/create-phases requirements.md        # Uses custom spec file
/create-phases specs/auth-spec.md     # Uses specific path
```

## Prerequisites
- Specification file exists (default: `technical-specs-v4.md`)
- Default coverage: 85% statements, 80% branches for critical code

## Process

### 1. Extract From Specs
Identify from specification file:
- System architecture (becomes phases with architectural validation)
- Component boundaries (prevent circular dependencies)
- Functional requirements (behavior correctness validation)
- Integration points (critical for step ordering)
- Concurrency requirements (Go-specific patterns)

### 2. Design Phases (Architectural Layers)
```markdown
Phase 1: Foundation              ← Types, interfaces, database
├── Step 1.1: Core types        ← Message, User, Channel structs
├── Step 1.2: Interface defs    ← Prevent circular dependencies
└── Step 1.3: Database schema   ← Persistence operations

Phase 2: WebSocket Infrastructure ← Connection handling
├── Step 2.1: Connection wrapper ← Single-writer pattern
├── Step 2.2: Connection registry ← Thread-safe registry
└── Step 2.3: Basic auth handler ← Authentication only

NOT a phase: "User Registration" (business feature, not architecture)
Good phases: Foundation, WebSocket, Channel System, Message Routing, Session Management
```

### 3. Integrated Validation Strategy Template

Each step includes three validation types:

#### **ARCHITECTURAL VALIDATION** (BLOCKING - Design Integrity):
- **Dependencies**: No circular imports, correct dependency direction
- **Boundaries**: Clean component separation, interface compliance
- **Integration**: Contracts between components well-defined
- **Patterns**: Go concurrency patterns followed correctly

#### **FUNCTIONAL VALIDATION** (BLOCKING - Behavior Correctness):
- **Requirements**: All specified behaviors implemented
- **Contracts**: Integration points work as specified
- **Error Handling**: All error cases handled correctly
- **Business Logic**: Domain rules implemented accurately

#### **TECHNICAL VALIDATION** (WARNING - Quality Assurance):
- **Coverage**: Test coverage meets targets (85%+ critical code)
- **Race Detection**: Concurrent code safe under load
- **Performance**: Response times within acceptable limits
- **Code Quality**: Static analysis clean, documentation adequate

### 4. Self-Contained Step Template
```markdown
## Step 2.1: Connection Wrapper (Estimated: 2h)

### EXACT REQUIREMENTS (do not exceed scope):
- Single-writer pattern: One goroutine per connection reads from writeCh
- Buffered channel size: exactly 100 messages
- Timeout: exactly 5 seconds for WriteJSON operations
- Context cancellation: Clean shutdown when context.Done() 
- Interface compliance: Must implement Connection interface exactly

### ARCHITECTURAL VALIDATION REQUIREMENTS:
**Dependencies** (BLOCKING):
- Only import: pkg/interfaces, standard library, websocket package
- No imports from: internal/channel, internal/session (circular risk)
- Verify: `go mod graph | grep -E "cycle|circular"` returns empty

**Boundaries** (BLOCKING):
- No business logic in connection wrapper (auth decisions, message routing)
- Clean separation: connection handling vs message processing
- Interface compliance: Exact match to Connection interface

**Integration Contracts** (BLOCKING):
- Registry can call NewConnection() safely from any goroutine
- Auth handler can call SetCredentials() after token validation
- Message router can call WriteJSON() concurrently without corruption

### FUNCTIONAL VALIDATION REQUIREMENTS:
**Core Behaviors** (BLOCKING):
- WriteJSON() delivers messages in order sent
- Close() stops all goroutines and releases resources within 1 second
- Context cancellation immediately stops write operations
- Authentication state (username, role) persists correctly after setting

**Error Handling** (BLOCKING):
- Timeout after exactly 5 seconds returns ErrWriteTimeout
- Invalid JSON returns ErrInvalidJSON with context
- Closed connection writes return ErrConnectionClosed
- Multiple Close() calls safe (idempotent)

**Integration Contracts** (BLOCKING):
- GetUsername()/GetRole() return correct values after SetCredentials()
- IsAuthenticated() reflects actual authentication state
- WriteJSON() from 10 concurrent goroutines works without data corruption

### TECHNICAL VALIDATION REQUIREMENTS:
**Race Detection** (WARNING):
- `go test -race` passes on all tests
- No goroutine leaks after connection close
- Channel operations don't block indefinitely

**Performance** (WARNING):
- Handle 1000+ messages/second per connection
- Memory usage <5KB per connection
- Connection setup/teardown <100ms

### MANDATORY INTERFACE (implement exactly):
```go
type Connection interface {
    WriteJSON(v interface{}) error    // Must use channel, not direct websocket write
    Close() error                     // Must cleanup goroutines and close writeCh
    GetUsername() string              // Return authenticated username
    GetRole() string                  // Return "student" or "teacher"
    IsAuthenticated() bool            // Return authentication status
    SetCredentials(username, role string) error // Set after auth validation
}
```

### CRITICAL PATTERNS (must follow):
```go
type Connection struct {
    conn     *websocket.Conn
    writeCh  chan []byte              // Buffer size: 100
    username string                   // Set after authentication
    role     string                   // Set after authentication
    ctx      context.Context          // For cancellation
    cancel   context.CancelFunc       // For cleanup
}

// REQUIRED: Single writer goroutine pattern
func (c *Connection) writeLoop() {
    for {
        select {
        case data := <-c.writeCh:
            c.conn.WriteMessage(websocket.TextMessage, data)
        case <-c.ctx.Done():
            return
        }
    }
}
```

### ERROR TYPES (define exactly these):
- ErrConnectionClosed = errors.New("connection closed")
- ErrWriteTimeout = errors.New("write timeout after 5 seconds")  
- ErrInvalidJSON = errors.New("invalid JSON data")

### SUCCESS CRITERIA:
**Architectural** (BLOCKING):
- [ ] No circular dependencies: `go mod graph | grep cycle` empty
- [ ] Clean imports: Only allowed packages imported
- [ ] Interface compliance: Implements Connection interface exactly
- [ ] Boundary separation: No business logic in connection layer

**Functional** (BLOCKING):
- [ ] WriteJSON delivers messages correctly and in order
- [ ] Close() cleanup complete within 1 second
- [ ] Authentication state management works correctly
- [ ] All error cases return correct error types

**Technical** (WARNING):
- [ ] `go test -race ./internal/websocket` passes
- [ ] Coverage ≥85% statements
- [ ] No goroutine leaks (test with 30-second timeout)
- [ ] Performance targets met (1000+ msg/sec)

### INTEGRATION CONTRACTS:
**What Step 2.2 (Registry) expects:**
- NewConnection(conn *websocket.Conn) *Connection constructor
- Connection.WriteJSON() thread-safe for concurrent calls
- Connection.Close() can be called multiple times safely
- GetUsername()/GetRole() return values set during authentication

**What Step 2.3 (Auth) will do:**
- Call SetCredentials(username, role string) after token validation
- Call WriteJSON() to send authentication response
- Call Close() on authentication failure

### READ ONLY THESE REFERENCES:
- technical-specs-v4.md lines 264-269 (Go concurrency patterns)
- technical-specs-v4.md lines 1014-1018 (WebSocket write races pitfall)
- pkg/interfaces/connection.go (interface definition from Step 1.2)

### IGNORE EVERYTHING ELSE
Do not read other sections of technical specs. Do not implement features not listed above.

### FILES TO CREATE:
- internal/websocket/connection.go (implementation)
- internal/websocket/connection_test.go (all test cases including architectural/functional validation)
- internal/websocket/errors.go (error type definitions)
```

### 5. Self-Contained Phase File Structure

Each phase file must contain complete step specifications with integrated validation:

```
planning/
├── phase-1.md            # Foundation: 3 self-contained step specs
├── phase-2.md            # WebSocket: 3 self-contained step specs  
├── phase-3.md            # Channel: 3 self-contained step specs
├── phase-4.md            # Routing: 3 self-contained step specs
├── phase-5.md            # Session: 3 self-contained step specs
├── dependency-graph.md   # Visual dependency map
├── code-inventory.md     # Function tracking with validation status
├── discoveries.md        # Implementation insights
└── spec-source.md        # Source tracking

Each phase-N.md contains:
## Phase N: [Name] 

### Step N.1: [Title]
- EXACT REQUIREMENTS (complete list)
- ARCHITECTURAL VALIDATION REQUIREMENTS (blocking criteria)
- FUNCTIONAL VALIDATION REQUIREMENTS (behavior verification)
- TECHNICAL VALIDATION REQUIREMENTS (quality gates)
- MANDATORY INTERFACE (full Go code)  
- CRITICAL PATTERNS (required code patterns)
- ERROR TYPES (exact definitions)
- SUCCESS CRITERIA (measurable checkboxes for all validation types)
- INTEGRATION CONTRACTS (what other steps expect)
- READ ONLY THESE REFERENCES (specific line numbers)
- IGNORE EVERYTHING ELSE (explicit scope limits)
- FILES TO CREATE (exact file paths)

### Step N.2: [Title]
[Complete self-contained specification with validation requirements]

### Step N.3: [Title]
[Complete self-contained specification with validation requirements]
```

### 6. For Phase 2+ (Dependency Management)
When creating additional phases:
```markdown
## Before Creating New Phases
1. Read planning/discoveries.md for architectural and functional insights
2. Review validation failures from Phase 1
3. Apply learnings to validation requirements:
   - Adjust interface designs based on implementation experience
   - Include discovered integration points
   - Add validation steps for patterns that emerged
   
Example adjustments:
- WebSocket tasks: +25% time (race condition testing complex)
- Add architectural validation steps (prevent circular deps)
- Include functional behavior verification upfront
- Add cleanup verification tasks based on discovered patterns
```

## Key Rules for Integrated Validation
- **Architectural Validation** = Design integrity (dependencies, boundaries, contracts)
- **Functional Validation** = Behavior correctness (requirements, error handling, integration)
- **Technical Validation** = Quality assurance (coverage, performance, race detection)
- **Phase** = Architectural layer with complete step specifications including validation
- **Step** = Self-contained implementation guide with all validation requirements
- **Interface definitions** = Full Go code in phase file, not external references  
- **Pattern requirements** = Show exact code structure with validation implications
- **Scope limitations** = Explicit "IGNORE EVERYTHING ELSE" to prevent feature creep
- **Integration contracts** = What this step provides/expects with validation criteria

## Anti-Patterns to Avoid
- ❌ "Read technical-specs-v4.md for requirements" (too vague, no validation focus)
- ❌ "Implement authentication as needed" (undefined scope, no validation criteria)
- ❌ "Follow Go best practices" (subjective, no specific validation)
- ❌ "See interface definition elsewhere" (forces context switching)
- ❌ "Make sure it works" (no specific functional validation criteria)

## Required Patterns  
- ✅ "Implement exactly this interface: [full Go code]"
- ✅ "Use this pattern: [complete code example]"
- ✅ "Read only lines 264-269 of technical-specs-v4.md"
- ✅ "IGNORE everything about persistence in this step"
- ✅ "Architectural validation: No imports from internal/channel"
- ✅ "Functional validation: WriteJSON must deliver messages in order"
- ✅ "Technical validation: Must pass race detector"