# /implement-step {phase}.{step}

Implement a specific step using strict TDD with integrated architectural and functional validation.

## What This Command Does
Implements a single step using RED-GREEN-COVER cycle with architectural validation (design integrity), functional validation (behavior correctness), and technical validation (quality assurance). Prevents common Go concurrency mistakes and captures discoveries.

## Usage
```bash
/implement-step 1.1    # Foundation types
/implement-step 2.2    # Connection registry  
/implement-step 3.3    # Channel integration
```

## Required Context (read in order)
1. docs/technical-specs-v4.md → Architecture and component boundaries
2. planning/phase-{N}.md → Step definitions with validation requirements
3. planning/discoveries.md → Insights from previous steps
4. planning/code-inventory.md → Existing functions and coverage

## Pre-Implementation Validation Check

Before starting any step after 1.1:

### Architectural Validation Check:
```bash
# Verify dependency compliance
go mod graph | grep -E "cycle|circular"  # Must be empty
go list -f '{{.ImportPath}}: {{.Imports}}' ./pkg/interfaces/  # Check boundaries

# Verify previous step integration
go build ./...  # All packages compile cleanly
```

### Functional Validation Check:
```markdown
## Previous Step Dependencies
From planning/phase-{N}.md:
- Step X.Y functional requirements verified: [check specific behaviors work]
- Interface contracts fulfilled: [verify actual behavior, not just compilation]
- Integration points tested: [confirm actual data flow, not just interfaces]

Adjustments for this step based on discoveries:
- [Any architectural insights that affect current step]
- [Any functional patterns that need replication]
```

## Integrated TDD Process

### 1. RED - Write Tests That Fail (Architectural + Functional)

**CRITICAL**: Tests must fail initially to prove they're testing real behavior:

```bash
$ go test ./internal/websocket -v
=== RUN TestConnection_ArchitecturalCompliance
--- FAIL: TestConnection_ArchitecturalCompliance (0.00s)
    connection_test.go:15: undefined: Connection
=== RUN TestConnection_WriteJSONBehavior  
--- FAIL: TestConnection_WriteJSONBehavior (0.00s)
    connection_test.go:25: undefined: Connection
=== RUN TestConnection_WriteRace
--- FAIL: TestConnection_WriteRace (0.00s)
    connection_test.go:35: undefined: Connection
FAIL    github.com/space-hub/internal/websocket [build failed]
```

**Test Requirements (Three Validation Types):**

**Architectural Validation Tests:**
- Interface compliance verification (compilation + runtime type assertions)
- Import boundary enforcement (no forbidden packages imported)
- Dependency direction validation (no circular references)
- Component responsibility boundaries (no business logic in infrastructure)

**Functional Validation Tests:**
- Core behavior verification (does WriteJSON actually work correctly?)
- Error handling completeness (all specified error cases)
- Integration contract fulfillment (actual data flow between components)
- Business requirement implementation (domain rules correctly enforced)

**Technical Validation Tests:**
- Concurrent access patterns (race detection enabled)
- Resource management (goroutine/connection cleanup)
- Performance characteristics (response times, throughput)
- Coverage and code quality verification

### 2. GREEN - Implement with Validation-Aware Discovery Comments

```go
// ARCHITECTURAL DISCOVERY: WebSocket writes must be serialized to prevent race conditions
// Interface boundary maintained - no business logic in connection wrapper
type Connection struct {
    conn     *websocket.Conn
    writeCh  chan []byte        // FUNCTIONAL DISCOVERY: 100 buffer prevents blocking in classroom scenarios
    username string
    role     string
    ctx      context.Context
    cancel   context.CancelFunc
}

// FUNCTIONAL DISCOVERY: 5-second timeout balances responsiveness vs classroom network stability
func (c *Connection) WriteJSON(v interface{}) error {
    data, err := json.Marshal(v)
    if err != nil {
        return fmt.Errorf("marshal error: %w", err)  // FUNCTIONAL: Error wrapping for debugging
    }
    
    select {
    case c.writeCh <- data:
        return nil
    case <-c.ctx.Done():
        return ErrConnectionClosed
    case <-time.After(5 * time.Second):
        return ErrWriteTimeout  // FUNCTIONAL: Exact timeout as specified
    }
}

// ARCHITECTURAL DISCOVERY: Clean shutdown requires careful goroutine coordination
func (c *Connection) Close() error {
    c.cancel()  // Signal writeLoop to stop
    // FUNCTIONAL DISCOVERY: Close channel to prevent further writes
    select {
    case <-c.writeCh:  // Drain any remaining messages
    default:
    }
    close(c.writeCh)
    return c.conn.Close()
}
```

**Implementation Rules with Validation Focus:**
- **Architectural**: Maintain clean boundaries, no forbidden imports, follow interface contracts
- **Functional**: Implement all specified behaviors correctly, handle all error cases
- **Technical**: Include DISCOVERY comments for decisions, use context for cancellation
- **Quality**: No TODO comments, no hardcoded values, handle all error paths

### 3. COVER - Verify with Integrated Validation

```bash
# MANDATORY: Comprehensive validation check
$ go test ./internal/websocket -race -cover -v

# Must show:
=== RUN TestConnection_ArchitecturalCompliance
--- PASS: TestConnection_ArchitecturalCompliance (0.01s)
=== RUN TestConnection_WriteJSONBehavior
--- PASS: TestConnection_WriteJSONBehavior (0.03s)
=== RUN TestConnection_WriteRace  
--- PASS: TestConnection_WriteRace (0.05s)
=== RUN TestConnection_ErrorHandling
--- PASS: TestConnection_ErrorHandling (0.02s)
PASS
coverage: 87.3% of statements

# Architectural validation
$ go mod graph | grep -E "cycle|circular"
# (should be empty)

# Functional integration validation  
$ go test ./... -short  # Verify integration points work
```

**Coverage Targets by Component Type:**
- **Critical Components** (WebSocket, Auth, Database): ≥90% (architectural + functional + technical validation)
- **Standard Components** (Message routing, Business logic): ≥85% (functional + technical validation) 
- **Utility Components** (Helpers, Config): ≥75% (functional validation)

## Update Tracking Files with Validation Status

### Code Inventory (after each function)
```markdown
## internal/websocket package
- `Connection.WriteJSON(v interface{}) error` - Single-writer pattern - Step 2.1 - 87.3% - [A✅F✅T✅]
- `Connection.Close() error` - Graceful shutdown - Step 2.1 - 91.2% - [A✅F✅T✅]
- `NewConnection(conn *websocket.Conn) *Connection` - Constructor - Step 2.1 - 100% - [A✅F✅T✅]

Legend: [A=Architectural, F=Functional, T=Technical] - ✅=Pass, ⚠️=Warning, ❌=Fail
```

### Discoveries (capture insights with validation context)
```markdown
## Phase 2, Step 1 (Connection Wrapper)
Date: 2024-01-16

### Architectural Discoveries
- WebSocket write races occur even with mutexes if multiple goroutines call WriteMessage directly
- Single-writer pattern with channels eliminates race conditions entirely
- Context cancellation propagation essential for clean component boundaries
- Interface compliance testing catches integration issues early

### Functional Discoveries  
- 5-second write timeout adequate for classroom network conditions but needs configuration
- JSON marshaling errors need wrapping for debugging context
- Connection replacement common in WebSocket apps - needs explicit handling
- Error type consistency critical for proper error propagation

### Technical Discoveries
- Buffered channel size of 100 prevents blocking in normal classroom scenarios
- Tests need artificial delays to trigger race conditions reliably
- sync.WaitGroup essential for testing goroutine cleanup
- Memory usage: ~2KB per connection (mostly channel buffer)

### Validation Impact
- Architectural validation caught circular dependency risk early
- Functional validation revealed timeout configuration need
- Technical validation showed race conditions in initial implementation

### Estimation
- Planned: 2 hours
- Actual: 2.5 hours (+25%)
- Reason: Architectural validation and race condition testing more complex than expected
```

## Required Output Format

```markdown
### Completed: Step X.Y - [Description]

#### Pre-Implementation Validation:
**Architectural**: ✅ No circular deps, clean boundaries
**Functional**: ✅ Previous step contracts verified
**Dependencies**: ✅ All previous steps complete and validated

#### Tests Written (RED):
```go
// Architectural validation test
func TestConnection_InterfaceCompliance(t *testing.T) {
    var _ Connection = &connection{}  // Compile-time check
}

// Functional validation test  
func TestConnection_WriteJSONBehavior(t *testing.T) {
    // Test actual message delivery behavior
}

// Technical validation test
func TestConnection_WriteRace(t *testing.T) {
    // Concurrent access safety
}
```

Test failures (proving tests are real):
```bash
=== RUN TestConnection_WriteJSONBehavior
--- FAIL: TestConnection_WriteJSONBehavior (0.00s)
    connection_test.go:25: undefined: Connection
```

#### Implementation (GREEN):
```go  
// Implementation with DISCOVERY comments and validation awareness
type Connection struct {
    // ARCHITECTURAL: Clean separation, no business logic
    // FUNCTIONAL: Delivers messages in order as specified
    // TECHNICAL: Single-writer pattern prevents races
}
```

#### Validation Results (COVER):
**Architectural Validation**: ✅ PASS
- No circular dependencies detected
- Clean import boundaries maintained  
- Interface compliance verified
- Component responsibilities properly separated

**Functional Validation**: ✅ PASS  
- WriteJSON delivers messages correctly and in order
- Close() cleanup completes within specified time
- Authentication state management works as specified
- All error cases return correct error types

**Technical Validation**: ✅ PASS
```bash
go test ./internal/websocket -race -cover
PASS
coverage: 87.3% of statements (target: 85%) ✅
No race conditions detected ✅
No goroutine leaks ✅
```

#### Files Changed:
- Created: internal/websocket/connection.go (95 lines)
- Created: internal/websocket/connection_test.go (156 lines with all validation types)
- Updated: planning/code-inventory.md (added 3 functions with validation status)
- Updated: planning/discoveries.md (added component patterns: concurrency, error handling, performance)

#### Component Discoveries This Step:
**Technical Patterns**: Single-writer pattern, context cancellation, buffered channels
**Error Handling**: Operation context wrapping, package-level error variables  
**Performance**: 2KB memory per connection, 1000+ msg/sec throughput
**Implementation**: Race testing complexity, timeout verification importance

#### Component Readiness:
**Component Validation**: ✅ PASS
- Interface implementation correct and complete
- Component behaviors work correctly in isolation  
- Race-free concurrent access within component
- Resource cleanup complete for single component

**Integration Readiness**: ✅ PASS
- Provides Connection interface as specified
- Error types defined for phase-level error handling
- Performance characteristics documented (1000+ msg/sec per connection)
- Ready for integration with registry and auth components

#### Ready for Phase Integration:
- Component complete and validated
- Interface contracts established
- Ready for Step 2.2: Connection Registry
- **Note**: Phase-level integration validation occurs after all Phase 2 steps complete
```

## Quality Gates (All Must Pass)

### Pre-Implementation:
- [ ] **Architectural**: Previous step boundaries clean, no circular dependencies
- [ ] **Functional**: Previous step behaviors verified, integration points tested
- [ ] **Technical**: Test cases defined for all validation types

### During Implementation:
- [ ] **Architectural**: Import boundaries maintained, interface compliance verified
- [ ] **Functional**: Required behaviors implemented correctly, error cases handled
- [ ] **Technical**: Tests fail before code written (RED verified), race detector clean

### Post-Implementation:
- [ ] **Architectural**: No circular dependencies, clean component boundaries
- [ ] **Functional**: Component behaviors work correctly, interface contracts met
- [ ] **Technical**: Coverage meets target, no race conditions in component

### Common Failure Patterns to Avoid:
1. **Architectural**: Circular imports, business logic in wrong component, interface violations
2. **Functional**: Skipping error cases, not testing actual component behavior, missing requirements
3. **Technical**: Skipping RED phase, race conditions in component, goroutine leaks

If any quality gate fails, STOP and fix before proceeding to next step.