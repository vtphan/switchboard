# Single Active Session Architecture

## Overview

This document describes the simplified single active session architecture for Switchboard, replacing the complex multi-session approach with a design that better matches real classroom usage patterns.

## Design Principles

### Core Principle: One Active Session
Only one session can have `status='active'` at any time, reflecting real classroom usage where there's typically one active lesson occurring.

### Key Benefits
- **Simplified Architecture**: Eliminates multi-session complexity and race conditions
- **Realistic Use Case**: Matches actual classroom where only one lesson occurs at a time
- **Safety First**: Explicit teacher control prevents accidental session termination
- **Trivial Auto-Assignment**: Simple check against single session vs complex multi-session lookup
- **Clear Student UX**: Binary choice (in session or lobby) vs confusing multi-session scenarios

## Architecture Changes

### Database Layer Simplification

#### Before (Multi-Session)
```go
// Complex queries across multiple sessions
func (m *Manager) FindSessionsForStudent(studentID string) ([]*Session, error)
func (m *Manager) ValidateSessionConflicts(studentIDs []string) error
```

#### After (Single Session)
```go
// Simple single session operations
func (m *Manager) GetActiveSession(ctx context.Context) (*Session, error)
func (m *Manager) HasActiveSession(ctx context.Context) (bool, error)
```

### Session Management Business Rules

#### Session Creation Validation
1. **Check for existing active session**
2. **If exists**: Return HTTP 409 Conflict with helpful error message
3. **If not**: Proceed with normal session creation
4. **Auto-transition**: Move lobby students who belong to new session

#### Teacher Workflow
```
1. Check GET /api/sessions (see if active session exists)
2. If active session exists: DELETE /api/sessions/{id} to end it
3. POST /api/sessions to create new session
4. Lobby students automatically transition to new session
```

### Student Auto-Assignment Logic

#### Connection Flow
```go
if sessionID == "" {  // Student connects without specifying session
    activeSession := GetActiveSession()
    if activeSession != nil && studentInSession(userID, activeSession) {
        sessionID = activeSession.ID  // Auto-assign to the session
    } else {
        sessionID = "lobby"           // Send to lobby
    }
}
```

#### Use Cases Solved
1. **Student connects before session**: Goes to lobby, auto-transitions when session created
2. **Student connects after session created**: Automatically assigned to their session
3. **Student not in session**: Always goes to lobby
4. **Session ends**: All students return to lobby

## Implementation Details

### Database Schema
No schema changes needed - business logic enforces single session rule.

```sql
-- Current schema supports single session enforcement
-- through application logic validation
```

### Error Handling

#### Session Creation Conflict (HTTP 409)
```json
{
  "error": "Active session exists",
  "message": "Cannot create new session. Active session 'Math Class' must be ended first.",
  "active_session": {
    "id": "session-123",
    "name": "Math Class",
    "created_at": "2025-01-27T10:00:00Z"
  }
}
```

### Registry Integration

#### Auto-Transition Logic
```go
func autoTransitionLobbyUsers(session *Session) {
    lobbyConnections := registry.GetLobbyConnections()
    
    for _, conn := range lobbyConnections {
        userID := conn.GetUserID()
        role := conn.GetRole()
        
        if role == "instructor" {
            // Instructors have universal access to active sessions
            registry.TransitionUserToSession(userID, session.ID)
        } else if role == "student" && isStudentInSession(userID, session) {
            // Auto-transition enrolled students
            registry.TransitionUserToSession(userID, session.ID)
        }
    }
}
```

**Consistent Auto-Assignment and Auto-Transition:**
- **New connections without session_id**: Auto-assigned to active session (instructors get universal access, students must be enrolled)
- **Existing lobby connections**: Auto-transitioned to new session (same logic as auto-assignment)

## Comparison with Current Multi-Session Plan

| Aspect | Multi-Session Plan | Single-Session Plan |
|--------|-------------------|-------------------|
| **Complexity** | High - track multiple sessions | Low - one session operations |
| **Teacher UX** | Manage multiple concurrent sessions | Clear end-then-create workflow |
| **Student UX** | Potential confusion with multiple sessions | Clear binary choice |
| **Auto-Assignment** | Complex lookup across sessions | Trivial check against one session |
| **Race Conditions** | Possible conflicts between sessions | Eliminated by single session rule |
| **Database Queries** | Complex multi-session queries | Simple single session lookups |
| **Error Handling** | Complex conflict resolution | Simple validation errors |
| **Real-World Mapping** | Academic concept | Actual classroom usage |

## Performance Characteristics

### Database Performance
- **Session Lookup**: O(1) vs O(n) for multi-session
- **Student Assignment**: Single query vs multiple session queries
- **Memory Usage**: Constant vs linear with session count

### Connection Performance
- **Auto-Assignment Latency**: < 10ms (simple boolean check)
- **Lobby Transition**: < 50ms for 50 students (batch operation)
- **Registry Lookups**: O(1) for single session vs O(n) for multi-session

## Migration Strategy

### From Current Multi-Session Code
1. **Add validation in CreateSession**: Check for existing active sessions
2. **Simplify session queries**: Replace multi-session lookups with single session
3. **Update auto-assignment**: Use simple active session check
4. **Add auto-transition**: Move lobby students on session creation

### Backward Compatibility
- API remains the same (just adds validation)
- WebSocket protocol unchanged
- Database schema unchanged
- Only business logic changes

## Testing Strategy

### Unit Tests (TDD Approach)
```go
// Architectural validation
func TestSingleSessionEnforcement_ArchitecturalCompliance(t *testing.T)

// Functional validation  
func TestCreateSession_RejectsWhenActiveExists(t *testing.T)
func TestAutoAssignment_StudentToActiveSession(t *testing.T)
func TestAutoTransition_LobbyToSession(t *testing.T)

// Technical validation
func TestSingleSessionEnforcement_Concurrency(t *testing.T)
func TestAutoAssignment_Performance(t *testing.T)
```

### Integration Tests
```go
func TestRealisticWorkflow_TeacherCreateSession(t *testing.T) {
    // 1. Teacher tries to create when session exists -> 409
    // 2. Teacher ends existing session -> 200
    // 3. Teacher creates new session -> 201 + auto-transition
}

func TestRealisticWorkflow_StudentAutoAssignment(t *testing.T) {
    // 1. Student connects before session -> lobby
    // 2. Session created -> auto-transition
    // 3. Another student connects -> auto-assigned
}
```

## Success Metrics

### Teacher Experience
- **Clear Error Messages**: 409 responses guide proper workflow
- **Simplified Workflow**: End-then-create is intuitive
- **No Accidental Conflicts**: Single session prevents confusion

### Student Experience  
- **Instant Assignment**: < 10ms auto-assignment for 95% of connections
- **Seamless Transitions**: Lobby to session without reconnection
- **Clear State**: Always know if in session or lobby

### System Reliability
- **No Race Conditions**: Single session eliminates conflicts
- **Predictable Behavior**: Clear rules for all scenarios
- **High Performance**: Simple operations scale well

## Implementation Phases

### Phase 1: Database Layer (Week 1)
- Add single session validation methods
- Implement business rule enforcement
- Comprehensive unit testing

### Phase 2: API Layer (Week 1)
- Session creation conflict detection
- Helpful error responses
- Integration testing

### Phase 3: WebSocket Auto-Assignment (Week 2)
- Student auto-assignment logic
- Performance optimization
- Load testing

### Phase 4: Auto-Transition (Week 2)
- Registry integration
- Lobby student transitions
- End-to-end testing

### Phase 5: Documentation & Migration (Week 3)
- Update SDK guidelines
- Migration documentation
- Performance validation

## Conclusion

The single active session architecture dramatically simplifies Switchboard while solving all core user experience issues. By matching real classroom usage patterns and providing explicit teacher control, this approach delivers a robust, performant, and intuitive system that eliminates the complexity of managing multiple concurrent sessions.

This design maintains backward compatibility while providing a clear migration path from the current multi-session approach to a simpler, more reliable architecture.