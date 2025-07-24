# Dependency Graph and Integration Map

## Overview
This document provides a visual representation of component dependencies and integration points across all phases, ensuring proper implementation order and preventing circular dependencies.

## Dependency Hierarchy

```
Phase 1: Foundation Layer
├── pkg/types (Step 1.1)          ← Base types, no dependencies
├── pkg/interfaces (Step 1.2)     ← Depends on: pkg/types
└── pkg/database (Step 1.3)       ← Depends on: pkg/types

Phase 2: WebSocket Infrastructure  
├── internal/websocket/connection (Step 2.1)  ← Depends on: pkg/interfaces, pkg/types
├── internal/websocket/registry (Step 2.2)    ← Depends on: internal/websocket/connection, pkg/types
└── internal/websocket/handler (Step 2.3)     ← Depends on: internal/websocket/connection, internal/websocket/registry, pkg/interfaces

Phase 3: Message Routing System
├── internal/router (Step 3.1)     ← Depends on: pkg/interfaces, pkg/types, internal/websocket/registry
└── internal/hub (Step 3.2)        ← Depends on: internal/websocket, internal/router, pkg/types

Phase 4: Session Management System  
├── internal/session (Step 4.1)    ← Depends on: pkg/interfaces, pkg/types
└── internal/database (Step 4.2)   ← Depends on: pkg/interfaces, pkg/types, pkg/database

Phase 5: API Layer and System Integration
├── internal/api (Step 5.1)        ← Depends on: pkg/interfaces, pkg/types
└── cmd/switchboard (Step 5.2)     ← Depends on: ALL previous components
```

## Critical Integration Points

### 1. Interface Contracts (Phase 1 → All Phases)
```
pkg/interfaces/connection.go → internal/websocket/connection.go
pkg/interfaces/session.go → internal/session/manager.go  
pkg/interfaces/router.go → internal/router/router.go
pkg/interfaces/database.go → internal/database/manager.go
```

**Validation Points:**
- Interface method signatures match implementations exactly
- Error types defined in interfaces match implementation usage
- Context handling consistent across all interface methods

### 2. WebSocket Integration Chain (Phase 2)
```
Connection Wrapper (2.1) → Registry (2.2) → Handler (2.3)
                   ↓            ↓             ↓
             WriteJSON()   RegisterConn()  Authentication
             Close()       GetConnections() History Replay
             GetUserID()   UnregisterConn() Heartbeat
```

**Critical Dependencies:**
- Registry must use Connection interface, not concrete type
- Handler must coordinate registration through Registry
- Authentication flow must complete before registration

### 3. Message Flow Integration (Phase 2 → Phase 3)
```
WebSocket Handler (2.3) → Hub (3.2) → Router (3.1) → Registry (2.2)
        ↓                    ↓           ↓              ↓
   Parse JSON            Queue Msg    Route Logic   Find Recipients
   Extract Sender        Coordinate   Validate      Connection Lookup
   Send to Hub          Error Handle  Rate Limit    WriteJSON()
```

**Critical Flow:**
1. Handler receives WebSocket message
2. Handler sends to Hub via channel
3. Hub forwards to Router for processing  
4. Router uses Registry to find recipients
5. Router calls WriteJSON() on connections

### 4. Session Management Integration (Phase 4)
```
SessionManager (4.1) ↔ DatabaseManager (4.2)
        ↓                      ↓
   Cache Active            Persist Changes
   Validate Access         Transaction Support
   Coordinate Lifecycle    Error Handling
        ↑                      ↑
WebSocket Handler (2.3) ← Validation Required
```

**Critical Operations:**
- Session validation must use in-memory cache for performance
- Database operations must use single-writer pattern
- Session termination must coordinate with connection cleanup

### 5. Complete System Integration (Phase 5)
```
HTTP API (5.1) → SessionManager (4.1) → DatabaseManager (4.2)
     ↓                ↓                       ↓
JSON Serialization  Session Operations   Persistence Layer
Error Handling      Access Validation    Transaction Support
Status Codes        Cache Management     Health Checks
     ↑                ↑                       ↑
Main App (5.2) ← Component Coordination ← All Dependencies
```

## Component Initialization Order

### Startup Sequence (Must Follow Exactly)
```
1. DatabaseManager      ← Must be first (validates schema)
2. SessionManager       ← Loads active sessions from database  
3. WebSocket Registry   ← Connection tracking (no dependencies)
4. Message Router       ← Needs Registry and DatabaseManager
5. Message Hub          ← Needs Registry and Router
6. WebSocket Handler    ← Needs Registry, SessionManager, DatabaseManager
7. HTTP API Server      ← Needs SessionManager, DatabaseManager, Registry
8. Main HTTP Server     ← Coordinates WebSocket and API endpoints
```

### Shutdown Sequence (Reverse Order)
```
1. HTTP Server          ← Stop accepting new requests
2. WebSocket Handler    ← Close all connections
3. Message Hub          ← Stop message processing
4. Message Router       ← Clean up routing state
5. WebSocket Registry   ← Clear connection maps
6. SessionManager       ← Save any pending state
7. DatabaseManager      ← Close database connections (LAST)
```

## Interface Satisfaction Matrix

| Interface | Implementation | Dependencies | Status |
|-----------|----------------|-------------|---------|
| `Connection` | `internal/websocket/connection.go` | pkg/types | ✅ Step 2.1 |
| `SessionManager` | `internal/session/manager.go` | pkg/interfaces, pkg/types | ✅ Step 4.1 |
| `MessageRouter` | `internal/router/router.go` | pkg/interfaces, pkg/types, internal/websocket | ✅ Step 3.1 |
| `DatabaseManager` | `internal/database/manager.go` | pkg/interfaces, pkg/types, pkg/database | ✅ Step 4.2 |

## Critical Validation Checkpoints

### Phase 1 Completion Validation
- [ ] `go mod graph | grep cycle` returns empty (no circular dependencies)
- [ ] All type validation methods work correctly
- [ ] Interface method signatures complete and consistent
- [ ] Database schema matches type definitions exactly

### Phase 2 Completion Validation  
- [ ] WebSocket connections implement Connection interface exactly
- [ ] Registry operations are thread-safe under concurrent access
- [ ] Authentication flow complete before connection registration
- [ ] Heartbeat monitoring prevents stale connections

### Phase 3 Completion Validation
- [ ] All 6 message types route to correct recipients
- [ ] Rate limiting enforced (100 messages/minute per client)
- [ ] Persist-then-route pattern prevents message loss
- [ ] Hub coordinates message flow without blocking

### Phase 4 Completion Validation
- [ ] Session validation uses in-memory cache for speed
- [ ] Database operations use single-writer pattern
- [ ] Session immutability enforced (no modifications after creation)
- [ ] Error handling provides clear feedback for API layer

### Phase 5 Completion Validation
- [ ] All API endpoints match specification exactly
- [ ] Health checks validate all system components
- [ ] Graceful shutdown completes within 30 seconds
- [ ] Application ready for production deployment

## Circular Dependency Prevention Rules

### Allowed Import Patterns
```
✅ pkg/types ← ANY (foundation types)
✅ pkg/interfaces ← internal/* (interface implementations)
✅ pkg/database ← internal/database (configuration usage)
✅ internal/websocket ← internal/hub, internal/api (websocket usage)
✅ internal/router ← internal/hub (router usage)
✅ internal/session ← internal/api (session management)
✅ internal/database ← internal/session (database usage)
```

### Forbidden Import Patterns
```
❌ pkg/types → internal/* (foundation cannot depend on implementations)
❌ pkg/interfaces → internal/* (interfaces cannot depend on implementations)
❌ internal/websocket → internal/router (would create circular dependency)
❌ internal/router → internal/session (would create circular dependency)
❌ internal/session → internal/websocket (would create circular dependency)
❌ internal/database → internal/session (would create circular dependency)
```

### Dependency Validation Commands
```bash
# Check for circular dependencies (run after each phase)
go mod graph | grep -E "cycle|circular"

# Verify import restrictions
go list -deps ./... | grep -E "internal/websocket.*internal/router"
go list -deps ./... | grep -E "internal/router.*internal/session"  
go list -deps ./... | grep -E "internal/session.*internal/websocket"

# All should return empty results
```

## Performance Dependencies

### Critical Path Analysis
```
WebSocket Message → Hub Queue → Router Process → Database Persist → Recipient Delivery
     <1ms              <1ms         <10ms          <50ms           <1ms
```

**Performance Requirements:**
- Registry lookups: O(1) using maps (Step 2.2)
- Session validation: <1ms using cache (Step 4.1)  
- Message routing: <10ms for broadcasts (Step 3.1)
- Database writes: <50ms using single-writer (Step 4.2)
- API responses: <100ms for session operations (Step 5.1)

### Memory Dependencies
```
Active Sessions Cache (Step 4.1) ← 10 sessions × 1KB = 10KB
Connection Registry (Step 2.2) ← 50 connections × 5KB = 250KB
Message Rate Limiting (Step 3.1) ← 50 clients × 100B = 5KB
Database Connection Pool (Step 4.2) ← 10 connections × 50KB = 500KB

Total Estimated Memory: ~1MB for classroom scale (50 concurrent users)
```

This dependency graph ensures proper implementation order and validates that the layer-by-layer architecture maintains clean boundaries while providing all required functionality.