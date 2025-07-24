# Specification Source Tracking

## Source Document
**File**: `docs/switchboard-tech-specs.md`  

## Overview
This document tracks the mapping between specification requirements and implementation phases, ensuring complete coverage and providing traceability for validation.

## Requirements Coverage Matrix

### Data Models (Lines 94-143)
| Specification Element | Implementation Location | Phase.Step | Status |
|----------------------|------------------------|------------|---------|
| Session struct (96-104) | `pkg/types/types.go` | 1.1 | ⏳ Pending |
| Message struct (108-118) | `pkg/types/types.go` | 1.1 | ⏳ Pending |
| Client struct (122-133) | `pkg/types/types.go` | 1.1 | ⏳ Pending |
| ConnectionManager struct (137-142) | `pkg/types/types.go` | 1.1 | ⏳ Pending |
| Database schema mapping | `migrations/001_initial_schema.sql` | 1.3 | ⏳ Pending |

### Message Types (Lines 146-158)
| Message Type | Routing Pattern | Implementation Location | Phase.Step | Status |
|-------------|----------------|------------------------|------------|---------|
| instructor_inbox | Student → All Instructors | `internal/router/router.go` | 3.1 | ⏳ Pending |
| inbox_response | Instructor → Specific Student | `internal/router/router.go` | 3.1 | ⏳ Pending |
| request | Instructor → Specific Student | `internal/router/router.go` | 3.1 | ⏳ Pending |
| request_response | Student → All Instructors | `internal/router/router.go` | 3.1 | ⏳ Pending |
| analytics | Student → All Instructors | `internal/router/router.go` | 3.1 | ⏳ Pending |
| instructor_broadcast | Instructor → All Students | `internal/router/router.go` | 3.1 | ⏳ Pending |

### Core Algorithms (Lines 164-302)

#### Message Routing Algorithm (Lines 164-190)
| Algorithm Step | Implementation Location | Phase.Step | Status |
|---------------|------------------------|------------|---------|
| Generate UUID for message.id | `internal/router/router.go` RouteMessage() | 3.1 | ⏳ Pending |
| Set server timestamp | `internal/router/router.go` RouteMessage() | 3.1 | ⏳ Pending |
| Set context default to "general" | `internal/router/router.go` RouteMessage() | 3.1 | ⏳ Pending |
| Persist to DB and wait | `internal/router/router.go` RouteMessage() | 3.1 | ⏳ Pending |
| Routing patterns for 6 types | `internal/router/router.go` GetRecipients() | 3.1 | ⏳ Pending |

#### Session Management Algorithm (Lines 192-212)
| Algorithm Step | Implementation Location | Phase.Step | Status |
|---------------|------------------------|------------|---------|
| CreateSession validation | `internal/session/manager.go` CreateSession() | 4.1 | ⏳ Pending |
| Remove duplicate student_ids | `internal/session/manager.go` removeDuplicates() | 4.1 | ⏳ Pending |
| Database transaction | `internal/database/manager.go` CreateSession() | 4.2 | ⏳ Pending |
| EndSession cleanup | `internal/session/manager.go` EndSession() | 4.1 | ⏳ Pending |

#### Client Connection Algorithm (Lines 214-238)
| Algorithm Step | Implementation Location | Phase.Step | Status |
|---------------|------------------------|------------|---------|
| Parameter validation | `internal/websocket/handler.go` HandleWebSocket() | 2.3 | ⏳ Pending |
| Session validation | `internal/websocket/handler.go` HandleWebSocket() | 2.3 | ⏳ Pending |
| Role-based access control | `internal/session/manager.go` ValidateSessionMembership() | 4.1 | ⏳ Pending |
| Connection replacement | `internal/websocket/registry.go` RegisterConnection() | 2.2 | ⏳ Pending |
| History replay | `internal/websocket/handler.go` sendSessionHistory() | 2.3 | ⏳ Pending |

#### History Replay Algorithm (Lines 241-254)
| Algorithm Step | Implementation Location | Phase.Step | Status |
|---------------|------------------------|------------|---------|
| Query messages by session | `internal/database/manager.go` GetSessionHistory() | 4.2 | ⏳ Pending |
| Role-based filtering | `internal/websocket/handler.go` sendSessionHistory() | 2.3 | ⏳ Pending |
| Error handling for history | `internal/websocket/handler.go` sendSessionHistory() | 2.3 | ⏳ Pending |

#### Connection Cleanup Algorithm (Lines 257-274)
| Algorithm Step | Implementation Location | Phase.Step | Status |
|---------------|------------------------|------------|---------|
| Idempotent cleanup | `internal/websocket/connection.go` Close() | 2.1 | ⏳ Pending |
| Registry cleanup | `internal/websocket/registry.go` UnregisterConnection() | 2.2 | ⏳ Pending |
| Stale connection detection | `internal/websocket/handler.go` heartbeat monitoring | 2.3 | ⏳ Pending |

#### Message Validation Algorithm (Lines 276-302)
| Algorithm Step | Implementation Location | Phase.Step | Status |
|---------------|------------------------|------------|---------|
| Rate limiting (100/minute) | `internal/router/rate_limiter.go` Allow() | 3.1 | ⏳ Pending |
| Message type validation | `internal/router/router.go` ValidateMessage() | 3.1 | ⏳ Pending |
| Role permission checking | `internal/router/router.go` canSendMessageType() | 3.1 | ⏳ Pending |
| Content size validation | `pkg/types/validation.go` Message.Validate() | 1.1 | ⏳ Pending |

### Concurrency Model (Lines 304-343)

#### Goroutine Architecture (Lines 306-327)
| Component | Implementation Location | Phase.Step | Status |
|-----------|------------------------|------------|---------|
| Main Hub Goroutine | `internal/hub/hub.go` run() | 3.2 | ⏳ Pending |
| DB Manager Goroutine | `internal/database/manager.go` writeLoop() | 4.2 | ⏳ Pending |
| Per-Client Read/Write Goroutines | `internal/websocket/connection.go` writeLoop() | 2.1 | ⏳ Pending |
| Health Monitor Goroutine | `internal/api/server.go` health monitoring | 5.1 | ⏳ Pending |

#### Channel Communication (Lines 329-336)
| Channel Type | Implementation Location | Phase.Step | Status |
|-------------|------------------------|------------|---------|
| hub.register | `internal/hub/hub.go` registerChannel | 3.2 | ⏳ Pending |
| hub.unregister | `internal/hub/hub.go` unregisterChannel | 3.2 | ⏳ Pending |
| hub.broadcast | `internal/hub/hub.go` messageChannel | 3.2 | ⏳ Pending |
| db.write | `internal/database/manager.go` writeChannel | 4.2 | ⏳ Pending |

#### Essential Limits (Lines 339-343)
| Limit | Value | Implementation Location | Phase.Step | Status |
|-------|-------|------------------------|------------|---------|
| Message rate limit | 100/minute | `internal/router/rate_limiter.go` | 3.1 | ⏳ Pending |
| Maximum message size | 64KB | `pkg/types/validation.go` | 1.1 | ⏳ Pending |
| User ID length | 1-50 chars | `pkg/types/validation.go` | 1.1 | ⏳ Pending |
| Session name length | 1-200 chars | `pkg/types/validation.go` | 1.1 | ⏳ Pending |

### Database Design (Lines 347-403)

#### Schema (Lines 349-380)
| Table/Index | Implementation Location | Phase.Step | Status |
|------------|------------------------|------------|---------|
| sessions table | `migrations/001_initial_schema.sql` | 1.3 | ⏳ Pending |
| messages table | `migrations/001_initial_schema.sql` | 1.3 | ⏳ Pending |
| Performance indexes | `migrations/001_initial_schema.sql` | 1.3 | ⏳ Pending |

#### Concurrency Strategy (Lines 382-398)
| Strategy Element | Implementation Location | Phase.Step | Status |
|-----------------|------------------------|------------|---------|
| Single-writer pattern | `internal/database/manager.go` writeLoop() | 4.2 | ⏳ Pending |
| Retry logic | `internal/database/manager.go` writeLoop() | 4.2 | ⏳ Pending |
| WAL mode configuration | `pkg/database/config.go` SQLite optimizations | 1.3 | ⏳ Pending |

### API Endpoints (Lines 404-543)

#### Session Management (Lines 406-448)
| Endpoint | Implementation Location | Phase.Step | Status |
|----------|------------------------|------------|---------|
| POST /api/sessions | `internal/api/handlers.go` createSession() | 5.1 | ⏳ Pending |
| DELETE /api/sessions/{id} | `internal/api/handlers.go` endSession() | 5.1 | ⏳ Pending |
| GET /api/sessions/{id} | `internal/api/handlers.go` getSession() | 5.1 | ⏳ Pending |
| GET /api/sessions | `internal/api/handlers.go` listSessions() | 5.1 | ⏳ Pending |

#### Health & Monitoring (Lines 489-507)
| Endpoint | Implementation Location | Phase.Step | Status |
|----------|------------------------|------------|---------|
| GET /health | `internal/api/handlers.go` healthCheck() | 5.1 | ⏳ Pending |

#### WebSocket Connection (Lines 509-542)
| Feature | Implementation Location | Phase.Step | Status |
|---------|------------------------|------------|---------|
| WebSocket upgrade | `internal/websocket/handler.go` HandleWebSocket() | 2.3 | ⏳ Pending |
| Query parameter validation | `internal/websocket/handler.go` HandleWebSocket() | 2.3 | ⏳ Pending |
| Heartbeat protocol | `internal/websocket/handler.go` handleConnection() | 2.3 | ⏳ Pending |

#### WebSocket Message Format (Lines 544-589)
| Message Format | Implementation Location | Phase.Step | Status |
|---------------|------------------------|------------|---------|
| Incoming message parsing | `internal/hub/hub.go` handleMessage() | 3.2 | ⏳ Pending |
| Outgoing message format | `internal/router/router.go` RouteMessage() | 3.1 | ⏳ Pending |
| System messages | `internal/websocket/handler.go` error feedback | 2.3 | ⏳ Pending |

### Error Handling & Validation (Lines 591-628)

#### Input Validation Rules (Lines 593-600)
| Validation Rule | Implementation Location | Phase.Step | Status |
|----------------|------------------------|------------|---------|
| User ID format | `pkg/types/validation.go` IsValidUserID() | 1.1 | ⏳ Pending |
| Session name format | `pkg/types/validation.go` Session.Validate() | 1.1 | ⏳ Pending |
| Context format | `pkg/types/validation.go` IsValidContext() | 1.1 | ⏳ Pending |
| Message content size | `pkg/types/validation.go` Message.Validate() | 1.1 | ⏳ Pending |

#### Connection Error Handling (Lines 602-608)
| Error Type | Implementation Location | Phase.Step | Status |
|-----------|------------------------|------------|---------|
| Invalid session | `internal/websocket/handler.go` HandleWebSocket() | 2.3 | ⏳ Pending |
| Unauthorized user | `internal/websocket/handler.go` HandleWebSocket() | 2.3 | ⏳ Pending |
| Rate limit exceeded | `internal/router/router.go` RouteMessage() | 3.1 | ⏳ Pending |

#### Database Error Handling (Lines 618-623)
| Error Scenario | Implementation Location | Phase.Step | Status |
|---------------|------------------------|------------|---------|
| Write failure retry | `internal/database/manager.go` writeLoop() | 4.2 | ⏳ Pending |
| Read failure handling | `internal/database/manager.go` query methods | 4.2 | ⏳ Pending |
| Connection recovery | `internal/database/manager.go` health check | 4.2 | ⏳ Pending |

### Business Rules & Constraints (Lines 629-662)

#### Session Rules (Lines 631-637)
| Rule | Implementation Location | Phase.Step | Status |
|------|------------------------|------------|---------|
| Immutable after creation | `internal/session/manager.go` (no modify methods) | 4.1 | ⏳ Pending |
| Instructor universal access | `internal/session/manager.go` ValidateSessionMembership() | 4.1 | ⏳ Pending |
| Manual termination only | `internal/session/manager.go` EndSession() | 4.1 | ⏳ Pending |

#### Connection Rules (Lines 639-645)
| Rule | Implementation Location | Phase.Step | Status |
|------|------------------------|------------|---------|
| Unique connections | `internal/websocket/registry.go` RegisterConnection() | 2.2 | ⏳ Pending |
| Connection replacement | `internal/websocket/registry.go` RegisterConnection() | 2.2 | ⏳ Pending |
| Student validation | `internal/session/manager.go` ValidateSessionMembership() | 4.1 | ⏳ Pending |

#### Message Rules (Lines 647-654)
| Rule | Implementation Location | Phase.Step | Status |
|------|------------------------|------------|---------|
| Server-generated IDs | `internal/router/router.go` RouteMessage() | 3.1 | ⏳ Pending |
| Server timestamps | `internal/router/router.go` RouteMessage() | 3.1 | ⏳ Pending |
| Role validation | `internal/router/router.go` canSendMessageType() | 3.1 | ⏳ Pending |

#### Persistence Rules (Lines 656-662)
| Rule | Implementation Location | Phase.Step | Status |
|------|------------------------|------------|---------|
| All messages persisted | `internal/router/router.go` RouteMessage() | 3.1 | ⏳ Pending |
| Persist-then-route | `internal/router/router.go` RouteMessage() | 3.1 | ⏳ Pending |
| Complete history sent | `internal/websocket/handler.go` sendSessionHistory() | 2.3 | ⏳ Pending |

### Performance Considerations (Lines 663-681)
| Consideration | Implementation Location | Phase.Step | Status |
|--------------|------------------------|------------|---------|
| Classroom scale design | All components (50 users target) | All | ⏳ Pending |
| Resource limits | Rate limiting, size limits | 3.1, 1.1 | ⏳ Pending |
| Efficient routing | O(1) lookups in registry | 2.2 | ⏳ Pending |
| Health monitoring | API health endpoint | 5.1 | ⏳ Pending |

### Security Considerations (Lines 682-709)
| Security Aspect | Implementation Location | Phase.Step | Status |
|----------------|------------------------|------------|---------|
| Input validation | Entry points (WebSocket, API) | 2.3, 5.1 | ⏳ Pending |
| Rate limiting | Message router | 3.1 | ⏳ Pending |
| Session isolation | Session membership validation | 4.1 | ⏳ Pending |
| Audit trail | Message persistence | 4.2 | ⏳ Pending |

## Coverage Validation Checklist

### Phase 1 Coverage
- [ ] All data models from lines 94-143 implemented
- [ ] All validation rules from lines 593-600 implemented
- [ ] Database schema from lines 349-380 created
- [ ] Interface definitions cover all required operations

### Phase 2 Coverage
- [ ] WebSocket connection process from lines 509-542 implemented
- [ ] Client connection algorithm from lines 214-238 implemented
- [ ] History replay algorithm from lines 241-254 implemented
- [ ] Connection cleanup from lines 257-274 implemented

### Phase 3 Coverage
- [ ] Message routing algorithm from lines 164-190 implemented
- [ ] All 6 message types from lines 146-158 supported
- [ ] Message validation algorithm from lines 276-302 implemented
- [ ] Hub goroutine architecture from lines 306-327 implemented

### Phase 4 Coverage
- [ ] Session management algorithm from lines 192-212 implemented
- [ ] Database concurrency strategy from lines 382-398 implemented
- [ ] Business rules from lines 631-662 enforced
- [ ] Error handling from lines 618-628 implemented

### Phase 5 Coverage
- [ ] All API endpoints from lines 404-507 implemented
- [ ] Health monitoring from lines 677-681 implemented
- [ ] System integration and configuration management
- [ ] Performance targets from lines 663-676 met

## Traceability Matrix Summary

| Specification Section | Lines | Implementation Phases | Coverage Status |
|-----------------------|-------|----------------------|-----------------|
| Data Models | 94-143 | 1.1, 1.3 | ⏳ Pending |
| Message Types | 146-158 | 3.1 | ⏳ Pending |
| Core Algorithms | 164-302 | 2.3, 3.1, 3.2, 4.1, 4.2 | ⏳ Pending |
| Concurrency Model | 304-343 | 2.1, 3.2, 4.2 | ⏳ Pending |
| Database Design | 347-403 | 1.3, 4.2 | ⏳ Pending |
| API Endpoints | 404-543 | 2.3, 5.1 | ⏳ Pending |
| Error Handling | 591-628 | All phases | ⏳ Pending |
| Business Rules | 629-662 | 2.2, 3.1, 4.1 | ⏳ Pending |
| Performance | 663-681 | All phases | ⏳ Pending |
| Security | 682-709 | 2.3, 3.1, 4.1, 5.1 | ⏳ Pending |

**Total Requirements Tracked:** 78 major requirements
**Implementation Coverage:** 100% (all requirements mapped to specific implementation locations)
**Validation Status:** ⏳ All pending implementation

This specification source tracking will be updated as implementation progresses to ensure complete requirement coverage and provide traceability for validation and testing.