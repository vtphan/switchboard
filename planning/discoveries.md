# Implementation Discoveries and Insights

## Overview
This document captures architectural and functional insights discovered during the planning process that will guide implementation decisions and help avoid common pitfalls.

## Phase 1 Implementation Discoveries

### Phase 1, Step 1.1 (Core Types)
Date: 2025-01-24

#### Architectural Discoveries
- **Pure data structures with validation**: Types package contains only data structures and validation methods, no business logic or external dependencies beyond standard library
- **Message type constants**: Defined exactly as specified to ensure compatibility with all routing logic across the system
- **Three-level connection mapping**: ConnectionManager structure enables O(1) lookups for both global user lookup and session-specific instructor/student lists
- **Interface compatibility**: All structs designed to support the interfaces that will be defined in Step 1.2

#### Functional Discoveries
- **Session immutability**: Session is immutable after creation except for end_time and status, preventing race conditions and simplifying session validation caching
- **Content flexibility**: Message Content as map[string]interface{} allows flexible message payloads while maintaining JSON compatibility for WebSocket transport
- **Context defaulting**: Empty context defaults to "general" during validation to ensure consistent behavior across all message paths
- **User ID validation**: 1-50 character limit prevents database issues and ensures reasonable display in UI components

#### Technical Discoveries
- **Channel serialization**: json:"-" tag prevents SendChannel serialization in Client struct
- **Regex performance**: Regular expressions compiled once at package initialization for better performance in high-frequency validation scenarios
- **Content size validation**: Requires JSON marshaling which adds overhead but ensures accurate byte count (64KB limit)
- **Validation performance**: All validation methods complete in <1ms for typical inputs (verified through testing)

#### Validation Impact
- Architectural validation passed: No circular dependencies, clean imports (standard library only)
- Functional validation passed: All 6 message types defined, validation enforces all spec requirements
- Technical validation passed: 96.4% test coverage, no race conditions, clean static analysis

#### Estimation
- Planned: 1.5 hours
- Actual: 1 hour
- Reason: Clear specifications and straightforward implementation made it faster than expected

### Phase 2, Step 1 (Connection Wrapper)
Date: 2025-01-24

#### Architectural Discoveries
- **Single-writer pattern with channels eliminates WebSocket race conditions**: Direct gorilla/websocket writes from multiple goroutines cause data corruption. Channel-based single-writer pattern with 100-message buffer prevents blocking while ensuring write ordering.
- **Context cancellation propagation essential for clean component boundaries**: Using context.WithCancel() enables proper goroutine coordination and resource cleanup when connections close.
- **Interface compliance testing catches integration issues early**: Compile-time interface verification ensures implementations match contracts exactly.
- **Clean separation between connection handling and business logic**: Connection wrapper focuses purely on WebSocket communication without message routing or session logic.

#### Functional Discoveries  
- **5-second write timeout adequate but needs monitoring**: Timeout handles classroom network conditions but may need configuration for different deployment environments.
- **JSON marshaling errors need proper error wrapping**: Returning ErrInvalidJSON with clear semantics helps debugging invalid message content.
- **Idempotent Close() implementation using sync.Once**: Multiple Close() calls are safe and expected in concurrent cleanup scenarios.
- **Authentication state separation from connection establishment**: SetCredentials() after WebSocket upgrade allows proper credential validation flow.

#### Technical Discoveries
- **100-message buffered channel prevents blocking in classroom scenarios**: Testing shows this size handles typical message bursts without timeouts while limiting memory usage.
- **RWMutex for authentication fields balances performance with safety**: Read-heavy credential access patterns benefit from reader-writer locks vs exclusive mutexes.
- **WriteLoop goroutine cleanup requires careful channel handling**: Draining remaining messages before channel close prevents resource leaks during shutdown.
- **Race detector essential for concurrent code validation**: Go race detector catches subtle synchronization issues that unit tests might miss.

#### Validation Impact
- **Architectural validation passed**: No circular dependencies, clean import boundaries (only gorilla/websocket, standard library), proper interface implementation
- **Functional validation passed**: WriteJSON delivers messages correctly, Close() completes cleanup, authentication state managed properly, all error types work correctly  
- **Technical validation passed**: 94.0% test coverage (exceeds 85% target), no race conditions detected, no goroutine leaks, idempotent operations verified

#### Estimation
- Planned: 2.5 hours
- Actual: 2 hours  
- Reason: Comprehensive test suite and validation took less time than expected due to clear implementation requirements

### Phase 2, Step 2 (Connection Registry)
Date: 2025-01-24

#### Architectural Discoveries
- **RWMutex optimizes read-heavy lookup patterns**: Registry operations are primarily lookups during message routing, making RWMutex more efficient than regular mutex for concurrent access patterns.
- **Three-map structure enables O(1) lookups**: Global user map + session-role maps provide both direct user access and efficient recipient calculation for different message types.
- **Asynchronous connection replacement prevents deadlocks**: Closing old connections in goroutines during RegisterConnection() avoids potential deadlocks while ensuring immediate replacement.
- **Empty map cleanup prevents memory leaks**: Removing empty session maps when last connection leaves prevents unbounded memory growth over time.

#### Functional Discoveries  
- **Idempotent unregistration simplifies cleanup**: UnregisterConnection() safely handles non-existent connections, making it safe to call from multiple cleanup paths without error checking.
- **Role-based connection maps enable efficient message routing**: Separate instructor/student maps allow message router to directly access recipients without filtering by role.
- **Connection replacement coordinates with authentication flow**: New connections immediately replace old ones in all maps, ensuring consistent state during user reconnection.
- **Statistics provide operational insight**: GetStats() offers visibility into registry state for monitoring without exposing internal structure.

#### Technical Discoveries
- **Concurrent map operations require careful coordination**: All map updates must be atomic within single lock acquisition to prevent inconsistent state during concurrent operations.
- **Session map initialization must be lazy**: Creating session maps only when first connection joins prevents memory waste for unused sessions.
- **Map iteration order is undefined**: Converting map values to slices ensures consistent iteration order for connection lists.
- **Memory efficiency through map cleanup**: Removing empty maps immediately prevents memory leaks in long-running registry instances.

#### Validation Impact
- **Architectural validation passed**: No circular dependencies, clean import boundaries (only sync), proper registry pattern implementation without business logic
- **Functional validation passed**: RegisterConnection() updates all maps atomically, UnregisterConnection() removes completely and idempotently, lookup methods consistent during concurrent updates  
- **Technical validation passed**: 92.9% test coverage (exceeds 85% target), no race conditions under concurrent access, O(1) lookup performance maintained, no deadlocks

#### Estimation
- Planned: 2 hours
- Actual: 1.5 hours  
- Reason: Well-defined interface from Step 1.2 and patterns from Step 2.1 made implementation smoother than expected

### Phase 2, Step 3 (WebSocket Handler and Authentication)
Date: 2025-01-24

#### Architectural Discoveries
- **Multi-stage validation prevents resource waste**: Query parameters → session membership → WebSocket upgrade → authentication → registration flow ensures invalid requests fail fast without consuming WebSocket resources.
- **Asynchronous history replay prevents blocking**: Background goroutine for history delivery allows connection setup to complete immediately while ensuring message history is delivered.
- **Interface-based dependency injection enables testing**: Handler constructor accepts interfaces rather than concrete types, facilitating comprehensive testing with mock implementations.
- **Separate goroutines for connection lifecycle management**: Independent goroutines for heartbeat monitoring and message reading prevent blocking between different connection concerns.

#### Functional Discoveries  
- **Role-based message filtering at delivery time**: Server-side filtering during history replay ensures students only see relevant messages while instructors have full classroom visibility.
- **WebSocket upgrade after validation optimizes error handling**: HTTP error responses for validation failures before WebSocket upgrade provides better client error handling than WebSocket close codes.
- **30-second ping interval with 60-second read deadline**: Balanced heartbeat timing provides reliable connection health monitoring for classroom environments without excessive network overhead.
- **Integration point prepared for Phase 3**: Message reading loop logs incoming messages and provides clear integration point for message router without tight coupling.

#### Technical Discoveries
- **httptest.NewRecorder() limitations with WebSocket upgrade**: Mock testing requires actual WebSocket servers for upgrade testing, leading to separation of HTTP validation tests vs WebSocket integration tests.
- **Gorilla WebSocket upgrader configuration**: CheckOrigin function and HandshakeTimeout settings critical for production deployment security and reliability.
- **Connection cleanup coordination**: Deferred cleanup in handleConnection() ensures registry deregistration and resource cleanup even during panic or unexpected exit.
- **Context propagation for graceful shutdown**: Connection context from Step 2.1 enables clean coordination between WebSocket handler and connection management.

#### Validation Impact
- **Architectural validation passed**: No circular dependencies, clean import boundaries (only interfaces, gorilla/websocket, standard library), proper separation of concerns between WebSocket handling and business logic
- **Functional validation passed**: Query parameter validation works correctly (400 for missing/invalid), role-based session validation enforced (403/404/500), connection registration and history replay working properly
- **Technical validation passed**: 87.5% test coverage (exceeds 85% target), no race conditions detected, WebSocket upgrade and connection lifecycle working correctly, concurrent request handling validated

#### Estimation
- Planned: 2.5 hours
- Actual: 2.25 hours  
- Reason: Comprehensive testing with mock interfaces and WebSocket integration took slightly less time than expected due to reusable patterns from previous steps

### Phase 2 Integration Bug Fix (Connection Replacement Race Condition)
Date: 2025-07-24

#### Problem Discovery
- **Critical race condition in connection replacement**: When users reconnected with the same userID, the second connection would register successfully but then be immediately removed from the registry
- **Root cause identified**: Old connection cleanup was calling `UnregisterConnection(userID)` which removed the NEW connection instead of the old one
- **Race sequence**: New connection registers → Old connection cleanup executes → Registry becomes empty despite successful registration
- **Test failure pattern**: `TestPhase2_ConnectionReplacementFlow` showed registry stats of 0 connections after second connection establishment

#### Architectural Discovery
- **Connection instance identity critical**: Registry operations must distinguish between specific connection instances, not just userIDs
- **Cleanup coordination complexity**: Asynchronous connection replacement requires careful coordination between registration and cleanup operations
- **Interface evolution necessity**: Registry API needed modification to prevent race conditions without breaking existing contracts
- **Deferred cleanup timing**: Connection cleanup goroutines can outlive the connections they're meant to clean up

#### Functional Discovery
- **Pointer comparison for identity**: Using connection instance pointers (registeredConn != conn) provides reliable identity checking during cleanup
- **Idempotent cleanup with instance awareness**: Only removing connections that match the registered instance prevents accidental removal of newer connections
- **Race-free connection replacement**: New connections can immediately replace old ones without risk of cleanup interference
- **Debug logging effectiveness**: Strategic logging revealed the race condition timing and execution order

#### Technical Discovery
- **Go pointer comparison reliability**: Connection instances maintain stable pointer identity throughout their lifecycle, enabling safe comparison
- **Test signature evolution**: Registry tests needed updates to pass connection instances instead of userID strings
- **Race detector validation**: `go test -race` essential for detecting subtle concurrency issues in WebSocket connection management
- **Mock test adaptation**: Concurrent registry tests required modification to track actual connection instances for realistic unregistration

#### Implementation Solution
**Before (Race Condition)**:
```go
func (r *Registry) UnregisterConnection(userID string) {
    // Removes whatever connection is currently registered for userID
    // PROBLEM: Old connection cleanup removes new connection
}
```

**After (Race-Free)**:
```go
func (r *Registry) UnregisterConnection(conn *Connection) {
    // Only removes if this specific connection instance is registered
    if registeredConn != conn {
        return // Different connection now registered, don't remove it
    }
}
```

#### Validation Impact
- **Architectural validation maintained**: No new circular dependencies, clean API evolution, preserved interface contracts
- **Functional validation restored**: Connection replacement now works correctly, registry maintains exactly one connection after replacement
- **Technical validation improved**: All 35 WebSocket tests pass, race detection clean, no performance regression

#### Performance Impact
- **Memory efficiency preserved**: Connection instance tracking adds minimal overhead (single pointer comparison)
- **Lookup performance maintained**: O(1) registry operations unchanged, no additional data structures needed
- **Cleanup latency unchanged**: Connection cleanup timing unaffected, only safety improved

#### Integration Lessons
- **Race condition detection methodology**: Debug logging at key coordination points reveals timing issues effectively
- **Test-driven debugging value**: Integration tests caught the race condition that unit tests missed
- **Connection lifecycle complexity**: WebSocket connection management requires careful coordination between multiple goroutines
- **API evolution strategy**: Changing method signatures requires systematic update of all callers and tests

#### Estimation
- Planned: N/A (unplanned bug fix)
- Actual: 2 hours
- Breakdown: 45 minutes investigation, 30 minutes fix implementation, 45 minutes testing and cleanup
- Reason: Race condition required systematic debugging to identify timing issue, but fix was straightforward once root cause understood

### Phase 1, Step 1.2 (Interface Definitions)
Date: 2025-01-24

#### Architectural Discoveries
- **Pure interface abstraction**: Interfaces contain no implementation details, ensuring clean boundaries between components and preventing circular dependencies
- **Context-first design pattern**: All database and session operations use context.Context as first parameter for proper cancellation and timeout handling
- **Single responsibility principle**: Each interface focuses on one concern - Connection (WebSocket management), SessionManager (session lifecycle), MessageRouter (routing logic), DatabaseManager (persistence)
- **Import boundary compliance**: Only imports from pkg/types and standard library, maintaining clean architectural boundaries

#### Functional Discoveries
- **Thread-safety documentation**: Connection interface explicitly documents thread-safety requirements, ensuring all implementations use proper patterns
- **Role-based validation abstraction**: Connection interface provides role and authentication state access needed for message routing permission checks
- **Efficient pointer usage**: Return *types.Session and []*types.Session for memory efficiency with classroom-scale data
- **Separate validation step**: Connection.SetCredentials() enables WebSocket upgrade before credential validation, improving connection establishment flow

#### Technical Discoveries
- **Interface compliance testing**: Mock implementations verify all interface methods exist and have correct signatures at compile time
- **Type safety enforcement**: Strong typing prevents runtime errors by ensuring correct parameter and return types across component boundaries
- **Integration contract clarity**: Interface documentation specifies exact integration points and expected behavior for implementation phases
- **Memory efficiency**: Pointer-based returns reduce memory allocation for frequently accessed objects

#### Validation Impact
- Architectural validation passed: No circular dependencies, clean import boundaries, single responsibility maintained
- Functional validation passed: All required operations covered, proper error handling signatures, integration contracts clear
- Technical validation passed: All interfaces compile correctly, mock implementations work, comprehensive documentation

#### Estimation
- Planned: 1 hour
- Actual: 45 minutes
- Reason: Interface definitions are straightforward, comprehensive planning made implementation smooth

### Phase 1, Step 1.3 (Database Schema and Configuration)
Date: 2025-01-24

#### Architectural Discoveries
- **WAL mode for concurrency**: SQLite Write-Ahead Logging mode enables concurrent reads while maintaining single-writer pattern required by DatabaseManager
- **Schema-code alignment**: Database schema exactly matches pkg/types struct definitions to ensure seamless JSON marshaling and ORM operations
- **Migration system separation**: Separate migration manager and schema validator enable different operational concerns (deployment vs validation)
- **Configuration externalization**: Database configuration struct enables environment-specific settings without code changes

#### Functional Discoveries
- **Constraint enforcement at database level**: Check constraints for message types and field lengths prevent invalid data at persistence layer
- **Foreign key cascading**: ON DELETE CASCADE for messages ensures referential integrity when sessions are terminated
- **Index optimization for query patterns**: Compound indexes designed for specific access patterns (session history, message routing, user lookups)
- **JSON storage for flexibility**: Student IDs and message content stored as JSON text enables flexible data structures without schema changes

#### Technical Discoveries
- **SQLite optimization pragmas**: 64MB cache, WAL mode, and memory temp storage provide optimal performance for classroom scale (20-50 users)
- **Migration transaction safety**: Each migration runs in transaction with rollback on failure ensures atomic schema evolution
- **Schema validation depth**: Multi-level validation (tables, columns, indexes, constraints) provides comprehensive deployment verification
- **File-based migrations**: SQL files enable version control integration and collaborative database evolution

#### Validation Impact
- Architectural validation passed: No circular dependencies, clean separation of concerns, configuration vs operations separated
- Functional validation passed: Schema supports all required operations, constraints enforce business rules, indexes enable performance targets
- Technical validation passed: 65.5% test coverage, migrations work correctly, SQLite optimizations apply successfully

#### Estimation
- Planned: 1.5 hours
- Actual: 1.25 hours
- Reason: Schema definition was straightforward but migration system required more careful transaction handling than expected

## Key Architectural Insights

### 1. Single-Writer Pattern Critical for SQLite
**Discovery:** SQLite write contention is a major bottleneck for concurrent applications.
**Implication:** Database Manager must use exactly one goroutine for all write operations.
**Implementation Impact:**
- Step 4.2: Channel-based write coordination is mandatory, not optional
- Retry logic needed for write failures (exactly once after 5 seconds)
- Read operations can be concurrent but writes must be serialized

### 2. WebSocket Race Conditions in Message Delivery
**Discovery:** Direct WebSocket writes from multiple goroutines cause data corruption.
**Implication:** Connection wrapper must implement single-writer pattern.
**Implementation Impact:**
- Step 2.1: Buffered channel (exactly 100 messages) with dedicated writeLoop goroutine
- WriteJSON() sends to channel, writeLoop handles actual WebSocket writes
- Context cancellation must cleanly stop writeLoop to prevent goroutine leaks

### 3. Session Validation Performance Requirements
**Discovery:** WebSocket authentication calls ValidateSessionMembership() for every connection.
**Implication:** Session Manager must use in-memory cache for sub-millisecond validation.
**Implementation Impact:**
- Step 4.1: LoadActiveSessions() at startup is mandatory for performance
- Cache-first lookup pattern: check memory before database
- Session termination must update cache immediately to prevent stale access

### 4. Message Routing Complexity with 6 Types
**Discovery:** Each message type has different recipient calculation logic.
**Implication:** Router must handle 3 distinct routing patterns efficiently.
**Implementation Impact:**
- Step 3.1: GetRecipients() method needs switch statement for message types
- instructor_inbox, request_response, analytics → all session instructors
- inbox_response, request → specific student (require to_user field)
- instructor_broadcast → all session students

### 5. Rate Limiting State Management
**Discovery:** 100 messages/minute per client requires sliding window implementation.
**Implication:** Rate limiter needs per-client state with periodic cleanup.
**Implementation Impact:**
- Step 3.1: ClientLimit struct tracks messageCount and windowStart per user
- Thread-safe map operations with mutex protection
- Cleanup() method needed to prevent memory leaks from disconnected clients

## Critical Integration Patterns

### 1. Persist-Then-Route Message Flow
**Discovery:** Message delivery without persistence creates audit gaps.
**Implication:** Database write must complete before recipient delivery begins.
**Implementation Impact:**
- Step 3.1: RouteMessage() calls StoreMessage() first, then delivers to recipients
- Database write failure stops routing entirely (message not delivered)
- Message ID generated server-side to ensure database consistency

### 2. Connection Replacement Coordination
**Discovery:** Multiple connections per user require careful cleanup to prevent resource leaks.
**Implication:** Registry registration must close old connections atomically.
**Implementation Impact:**
- Step 2.2: RegisterConnection() closes existing connection before adding new one
- Asynchronous close to prevent deadlocks in registration flow
- Cleanup coordination through Hub to maintain connection map consistency

### 3. History Replay Role-Based Filtering
**Discovery:** Students should only see relevant messages, instructors see everything.
**Implication:** History replay must filter by role at delivery time.
**Implementation Impact:**
- Step 2.3: sendSessionHistory() applies role-based filtering per message
- Instructors: all messages sent
- Students: messages involving them (from_user, to_user, or broadcasts)
- "history_complete" system message sent after filtering complete

### 4. Hub Channel Buffering Requirements
**Discovery:** Message bursts during classroom activities can overwhelm processing.
**Implication:** Hub channels need appropriate buffering to prevent blocking.
**Implementation Impact:**
- Step 3.2: Message channel buffered to 1000 messages for burst handling
- Registration/deregistration channels smaller (100) for lifecycle events
- Non-blocking sends with error handling for channel full scenarios

## Go Concurrency Patterns Identified

### 1. Context Cancellation for Graceful Shutdown
**Pattern:** All long-running goroutines must respect context cancellation.
**Critical Locations:**
- Connection writeLoop (Step 2.1): Stop writing on context.Done()
- Database writeLoop (Step 4.2): Stop processing writes on shutdown
- Hub processing loop (Step 3.2): Coordinate shutdown across all channels
- HTTP server (Step 5.2): Graceful shutdown with request completion

### 2. Mutex Protection for Shared State
**Pattern:** Connection registry and rate limiter need thread-safe access.
**Critical Locations:**
- Registry maps (Step 2.2): RWMutex for connection lookups during message routing
- Rate limiter client state (Step 3.1): Mutex for messageCount updates
- Session cache (Step 4.1): RWMutex for session validation during WebSocket auth
- Connection authentication state (Step 2.1): RWMutex for credential access

### 3. Channel Communication for Coordination
**Pattern:** Hub coordinates message flow and connection lifecycle via channels.
**Implementation Requirements:**
- Buffered channels to prevent blocking during high activity
- Select statements with timeout for non-blocking operations
- Channel closure for graceful shutdown signaling
- Error channels for write operation coordination

### 4. Resource Cleanup Patterns
**Pattern:** Prevent goroutine and connection leaks through proper cleanup.
**Critical Requirements:**
- Connection.Close() must be idempotent (safe to call multiple times)
- Registry cleanup removes from all maps atomically
- Database manager waits for write goroutine before closing connection
- HTTP server shutdown waits for request completion with timeout

## Database Schema Optimization Insights

### 1. Index Strategy for Message History
**Discovery:** Session history queries are frequent and time-sensitive.
**Optimization:** Compound index on (session_id, timestamp) for efficient ordering.
**Impact:** Step 1.3 schema includes idx_messages_session_time for <100ms history retrieval.

### 2. JSON Storage for Flexible Content
**Discovery:** Message content varies significantly across use cases.
**Decision:** Store content as JSON text rather than structured columns.
**Impact:** 64KB limit enforced at application layer, not database constraints.

### 3. SQLite WAL Mode for Concurrency
**Discovery:** Default SQLite journal mode blocks readers during writes.
**Optimization:** WAL (Write-Ahead Logging) mode allows concurrent reads.
**Impact:** Step 1.3 includes WAL pragma in optimization setup.

## Error Handling Strategy Insights

### 1. Error Type Hierarchy
**Pattern:** Each layer defines specific error types for proper handling by callers.
**Implementation:**
- Database errors: connectivity, constraint violations, query failures
- Session errors: not found, unauthorized, validation failures
- WebSocket errors: connection closed, write timeout, invalid JSON
- Router errors: rate limit, invalid recipient, permission denied

### 2. Error Context Preservation
**Pattern:** Wrap errors with context at each layer boundary.
**Implementation:**
- Database layer: "failed to store message: connection timeout"
- Session layer: "session validation failed: session not found"
- WebSocket layer: "connection setup failed: invalid user ID format"

### 3. Client Error Feedback
**Pattern:** Send system messages to clients for recoverable errors.
**Implementation:**
- Rate limit exceeded: notify client with current limit
- Message routing failed: explain why delivery failed
- Session ended: inform client that session is no longer active

## Performance Optimization Discoveries

### 1. Connection Pool Configuration
**Discovery:** SQLite connection pool size affects concurrent read performance.
**Optimization:** 10 connections max (SQLite recommended limit for classroom scale).
**Impact:** Step 1.3 database config includes MaxConnections: 10 setting.

### 2. Memory Usage for Classroom Scale
**Estimation:** 50 concurrent users = ~1MB total memory usage.
**Breakdown:**
- Active session cache: 10 sessions × 1KB = 10KB
- Connection registry: 50 connections × 5KB = 250KB  
- Rate limiter state: 50 clients × 100B = 5KB
- Database connection pool: 10 connections × 50KB = 500KB

### 3. Message Throughput Targets
**Requirements:** Handle typical classroom message patterns.
**Targets:**
- 100 messages/minute per client (rate limit)
- 1000+ messages/second per WebSocket connection (burst handling)
- <10ms message routing for broadcasts
- <1ms session validation using cache

## Security Consideration Insights

### 1. Input Validation Boundaries
**Pattern:** Validate at system entry points, trust internal data.
**Implementation:**
- WebSocket handler validates query parameters and message JSON
- API endpoints validate request bodies and path parameters
- Internal components assume validated data from entry points

### 2. Role-Based Access Control
**Pattern:** Verify permissions at operation time, not just connection time.
**Implementation:**
- Message router checks sender role for each message type
- Session manager validates role during membership checks
- Database queries include role-based filtering where appropriate

### 3. Resource Exhaustion Protection
**Pattern:** Prevent abuse through rate limiting and size constraints.
**Implementation:**
- 100 messages/minute per client prevents spam
- 64KB message content limit prevents memory abuse
- Connection replacement prevents connection accumulation
- Channel buffering prevents memory growth during bursts

## Testing Strategy Insights

### 1. Race Condition Testing
**Pattern:** Use Go race detector on all concurrent code.
**Critical Areas:**
- Connection registry operations during message routing
- Rate limiter updates during concurrent message processing
- Session cache access during WebSocket authentication
- Database write coordination under load

### 2. Integration Test Scenarios
**Pattern:** Test complete message flows across component boundaries.
**Key Scenarios:**
- WebSocket message → Hub → Router → Database → Recipient delivery
- Session creation → Cache update → WebSocket authentication
- Connection replacement → Registry update → Old connection cleanup
- System shutdown → Component cleanup → Resource deallocation

### 3. Load Testing Parameters
**Pattern:** Validate performance under classroom-scale load.
**Test Conditions:**
- 50 concurrent WebSocket connections
- Message burst scenarios (entire class sending simultaneously)
- Session creation/termination during active connections
- Database operations under concurrent access

## Deployment Considerations

### 1. Configuration Management
**Pattern:** Environment-specific settings without code changes.
**Implementation:**
- Default configuration suitable for development
- Environment variable overrides for production
- Configuration validation prevents startup with invalid settings

### 2. Health Check Requirements
**Pattern:** Validate all critical components for load balancer integration.
**Implementation:**
- Database connectivity check with timeout
- Session manager cache validation
- Connection registry statistics
- Component lifecycle status

### 3. Graceful Shutdown Requirements
**Pattern:** Clean resource cleanup for container orchestration.
**Implementation:**
- Signal handling for SIGTERM/SIGINT
- HTTP server graceful shutdown with request completion
- WebSocket connection cleanup with proper close frames
- Database connection cleanup with pending write completion

These discoveries will guide implementation decisions and help avoid common pitfalls during development. They represent lessons learned from the architectural design process and should be referenced during each phase implementation.