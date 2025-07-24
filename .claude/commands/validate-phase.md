# /validate-phase {phase}

Comprehensive validation of complete architectural phase integration before proceeding to next phase.

## What This Command Does
Validates that all steps in a phase work together correctly, fulfill phase-level contracts, and are ready for integration with the next architectural layer. Focuses on integration workflows, cumulative behavior, and phase boundary contracts.

## Usage
```bash
/validate-phase 1    # Validate Foundation phase (types, interfaces, database)
/validate-phase 2    # Validate WebSocket Infrastructure phase (connection, registry, auth)
/validate-phase 3    # Validate Business Logic phase (services, events, jobs)
```

## Required Context
1. docs/technical-specs-v4.md → Phase requirements and integration points
2. planning/phase-{N}.md → Phase definition and step specifications
3. planning/discoveries.md → Implementation insights from all steps in phase
4. All step implementations → Complete phase codebase

## Phase-Level Validation Process

### 1. Architectural Integration Validation (BLOCKING)

Verify the phase forms a coherent architectural layer:

```markdown
## Phase 2: WebSocket Infrastructure Integration

### Cross-Step Architectural Coherence:
✅ All steps follow consistent architectural patterns
✅ No circular dependencies within phase: `go mod graph | grep -E "cycle|circular"`
✅ Clean phase boundaries: No inappropriate imports between steps
✅ Interface contracts between steps well-defined and consistent
❌ VIOLATION: Step 2.2 (Registry) imports Step 2.3 (Auth) directly - should use interfaces

### Phase Boundary Validation:
✅ Phase provides clean interface to next phase (Phase 3: Business Logic)
✅ Phase dependencies on previous phases appropriate (Foundation types only)
✅ No leakage of phase implementation details to external phases
✅ Phase abstraction level appropriate for architectural layer

### Dependency Architecture:
✅ Phase 2 correctly depends only on Phase 1 (Foundation)
✅ Phase 2 provides expected interfaces for Phase 3 consumption
✅ Internal phase dependencies follow correct direction (Connection ← Registry ← Auth)
❌ CONCERN: Auth component tightly coupled to Registry - reduces testability
```

### 2. Functional Integration Validation (BLOCKING)

Verify complete workflows work across all steps in the phase:

```markdown
## End-to-End Functional Workflows

### Primary Phase Workflow: User Connection Flow
Test: Complete user connection → authentication → message sending workflow

**Workflow Steps:**
1. WebSocket connection establishment (Step 2.1: Connection)
2. User authentication with token (Step 2.3: Auth) 
3. Connection registration in registry (Step 2.2: Registry)
4. Message sending through authenticated connection
5. Clean connection cleanup on disconnect

**Test Results:**
✅ User can establish WebSocket connection successfully
✅ Authentication flow works with valid tokens
✅ Registry correctly tracks authenticated connections
✅ Messages flow through complete stack without corruption
❌ FAILURE: Connection cleanup not propagating to registry correctly
❌ FAILURE: Auth failure doesn't trigger proper connection cleanup

### Error Propagation Validation:
✅ Network errors from Connection bubble up through Registry correctly
✅ Auth failures result in proper connection termination
❌ MISSING: Timeout errors in Connection not handled by Registry
✅ Invalid tokens properly rejected with clear error messages

### Resource Coordination:
✅ All components coordinate resource cleanup on disconnect
❌ ISSUE: Registry can leak connection references if Auth fails during setup
✅ Memory usage stable across complete connection lifecycle
✅ No goroutine leaks when multiple components interact
```

### 3. Performance Integration Validation (WARNING)

Verify phase performance characteristics under integrated load:

```bash
# Phase-level performance testing
go test ./internal/websocket/... -bench=BenchmarkPhase -benchmem

# Expected comprehensive performance profile:
BenchmarkPhase_CompleteUserFlow-8           100    12.5ms/op    2.1KB/op    15 allocs/op
BenchmarkPhase_ConcurrentConnections-8       50    45.2ms/op    8.3KB/op    42 allocs/op
BenchmarkPhase_MessageThroughput-8         1000     1.2ms/op    0.8KB/op     5 allocs/op
```

```markdown
## Phase Performance Integration

### Cumulative Performance Characteristics:
✅ Complete user connection flow: 12.5ms average (target: <20ms)
✅ 50 concurrent user connections: 45.2ms setup (target: <60ms)
✅ Message throughput: 1000+ messages/second through complete stack
⚠️ CONCERN: Memory allocation per operation higher than individual components suggest

### Resource Usage Patterns:
✅ Memory usage per complete user session: 5.2KB (acceptable for classroom scale)
✅ CPU utilization under 50 concurrent users: <15% (excellent)
✅ Goroutine count stable under connection churn
⚠️ CONCERN: File descriptor usage higher than expected (investigation needed)

### Phase Scalability:
✅ Phase handles classroom scale (50 concurrent users) comfortably
✅ Performance degrades gracefully under overload
✅ Resource cleanup prevents accumulation under sustained load
⚠️ Room for optimization in connection setup path
```

### 4. Phase Boundary Contract Validation (BLOCKING)

Verify the phase fulfills its contracts to other phases:

```markdown
## Phase Contract Fulfillment

### What Phase 2 Promises to Phase 3 (Business Logic):
**Interface Contract:**
- Authenticated WebSocket connection management
- Message delivery with ordering guarantees  
- Connection state tracking and cleanup
- Role-based access control information

**Performance Contract:**
- <20ms connection establishment
- 1000+ messages/second throughput
- <100ms connection cleanup
- Support for 50+ concurrent classroom users

**Error Contract:**
- Well-defined error types for business logic handling
- Graceful degradation under overload
- Clear error propagation from network issues

### Contract Validation Results:
✅ Interface contracts met: All expected interfaces implemented and tested
✅ Performance contracts met: All targets achieved with margin
❌ PARTIAL: Error contract incomplete - some error cases not properly categorized
✅ Documentation complete: Phase behavior well-documented for Phase 3 consumption

### Integration Test with Phase 1 (Foundation):
✅ Uses Foundation types correctly (User, Message, Session)
✅ Database integration works through Foundation interfaces
✅ No bypassing of Foundation abstractions
✅ Type safety maintained across phase boundary
```

## Critical Issues Requiring Resolution

### ARCHITECTURAL BLOCKING ISSUES:
```markdown
PHASE-ARCH-1: Direct Import Between Steps
Location: internal/websocket/registry.go:12
Issue: Registry directly imports Auth package instead of using interface
Impact: Tight coupling prevents independent testing and violates phase architecture
Fix: Create AuthProvider interface in Foundation, use dependency injection
Estimate: 2 hours

PHASE-ARCH-2: Inconsistent Error Handling Patterns
Location: Steps 2.1, 2.2, 2.3 use different error wrapping approaches  
Issue: No consistent error propagation strategy across phase
Impact: Difficult error handling for Phase 3, inconsistent debugging experience
Fix: Standardize error types and wrapping patterns across phase
Estimate: 1.5 hours
```

### FUNCTIONAL BLOCKING ISSUES:
```markdown
PHASE-FUNC-1: Connection Cleanup Propagation Failure
Location: Registry doesn't receive cleanup notifications from Connection
Issue: Registry maintains stale connection references after network failures
Impact: Memory leaks, incorrect connection counts, routing to dead connections
Fix: Implement cleanup notification mechanism between Connection and Registry
Estimate: 3 hours

PHASE-FUNC-2: Auth Failure Recovery Incomplete
Location: Auth failure during connection setup leaves Registry in inconsistent state
Issue: Partial connection state not cleaned up properly
Impact: Resource leaks, security concerns (unauthenticated connections tracked)
Fix: Implement transaction-like setup with proper rollback
Estimate: 2 hours
```

### PERFORMANCE WARNING ISSUES:
```markdown
PHASE-PERF-1: Higher Than Expected Resource Usage
Location: Phase-level memory usage 40% higher than component sum
Issue: Unknown resource overhead in component interactions
Impact: Reduced scalability headroom
Fix: Profile component interactions, optimize integration overhead
Estimate: 4 hours (investigation + optimization)
```

## Phase Readiness Assessment

```markdown
## Phase 2 Integration Status: BLOCKED

### Architectural Readiness: ⚠️ NEEDS FIXES
- Phase coherence: Good (consistent patterns across steps)
- Boundary contracts: Good (clean interfaces to other phases)  
- Internal coupling: Poor (direct imports between steps)
- **BLOCKING**: 2 architectural issues must be resolved

### Functional Readiness: ❌ BLOCKED  
- Primary workflows: Partial (main flow works, error flows broken)
- Error propagation: Incomplete (some cases not handled)
- Resource management: Broken (cleanup propagation issues)
- **BLOCKING**: 2 functional issues must be resolved

### Performance Readiness: ⚠️ ACCEPTABLE
- Throughput targets: Met (1000+ msg/sec)
- Latency targets: Met (<20ms connection setup)
- Resource usage: Higher than expected but acceptable
- **WARNING**: 1 performance optimization recommended

### Phase Transition Readiness: BLOCKED
❌ Cannot proceed to Phase 3 until blocking issues resolved
❌ Error handling patterns must be consistent for Phase 3 integration
❌ Resource cleanup must work reliably for Phase 3 dependency

### Estimated Fix Time: 8.5 hours
- Critical architectural fixes: 3.5 hours
- Critical functional fixes: 5 hours  
- Optional performance optimization: 4 hours (can defer)

### Files Updated:
- Updated: planning/code-inventory.md (phase integration status)
- Updated: planning/discoveries.md (integration patterns, anti-patterns, system behavior insights)
- Created: planning/phase-2-integration-issues.md (detailed issue tracking)

### Recommended Action:
□ Fix blocking issues and re-validate phase
□ Proceed to Phase 3 with warnings (NOT RECOMMENDED - functional issues too severe)
□ Redesign phase architecture (only if fix estimates too high)

**DECISION REQUIRED**: Fix blocking issues before Phase 3 implementation
```

## Phase Validation Success Criteria

### Ready to Proceed When:
```bash
# All phase integration tests pass
go test ./internal/websocket/... -tags=integration -race

# No architectural violations
go mod graph | grep -E "cycle|circular"  # Empty
grep -r "internal/websocket" ./internal/websocket/*/  # No cross-step imports

# Performance benchmarks meet targets  
go test ./internal/websocket/... -bench=BenchmarkPhase -benchmem  # All targets met

# Documentation complete
ls planning/phase-2-integration.md  # Integration patterns documented
```

Expected output:
```bash
✅ All integration workflows pass
✅ No architectural violations detected
✅ Performance targets met with margin
✅ Error handling consistent and complete
✅ Resource cleanup reliable under all scenarios
✅ Phase boundary contracts fulfilled
✅ Ready for Phase 3 implementation
```

## Discovery Updates for System Evolution

Update `planning/discoveries.md` with **integration and architectural patterns**:

```markdown
## Integration Patterns

### ❌ Anti-Patterns That Failed
- Direct imports between components in same phase: Creates tight coupling, prevents testing
- Assumption-based resource cleanup: Leads to leaks and inconsistent state  
- Component-only performance testing: Misses significant integration overhead

### ✅ Integration Patterns That Work
- Interface-based component communication: Enables testing and loose coupling
- Explicit cleanup notification mechanisms: Prevents resource leaks
- Transaction-like setup with rollback: Handles multi-component setup failures

## System Behavior Insights

### Resource Coordination
- Phase-level memory usage: +40% overhead compared to component sum
- Resource cleanup: Requires explicit coordination, doesn't happen automatically
- Connection lifecycle: Needs cleanup propagation between Registry and Connection

### Error Propagation Patterns
- Consistent error types across phase components: Essential for proper handling
- Error context preservation: Must be maintained across component boundaries
- Failure recovery: Requires explicit rollback mechanisms for multi-component operations

### Performance Integration Characteristics
- End-to-end latency: 12.5ms for complete user connection flow
- Concurrent user scaling: 50 users achievable with current architecture
- Resource optimization: Component interaction overhead significant factor

## Architectural Lessons

### Phase Validation Insights
- Integration testing reveals assumptions component testing misses
- Phase-level workflows show coordination issues not visible in isolation
- System behavior emerges at integration level, not predictable from components

### Next Phase Implications
- Design explicit coordination mechanisms upfront
- Plan integration testing early in phase development
- Budget extra time for integration complexity beyond component sum
```

This phase-level validation approach ensures architectural layers are truly ready for integration and catches the critical issues that component-level validation misses.