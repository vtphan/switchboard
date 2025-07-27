# Implicit Lobby System - Implementation Plan

## Overview

This document outlines the implementation plan for adding an implicit lobby system to Switchboard. The lobby system enables persistent WebSocket connections for all users, providing real-time presence awareness and instant notifications without requiring active sessions.

## Current State Analysis

### Problems Addressed
1. **No real-time notifications**: Students must poll every 30 seconds to discover new sessions
2. **Connection loss on session end**: Clients disconnect when sessions end, requiring reconnection
3. **No presence awareness**: Users don't know who's online before sessions start
4. **Poor user experience**: Delays and disconnections disrupt the learning flow

### Current Architecture Constraints
- WebSocket connections require active `session_id`
- `session_ended` messages trigger full client disconnection
- No mechanism for cross-session communication
- Single connection per user (existing - this is good)

## Solution: Implicit Lobby System

### Core Concept
- **Always-on connections**: Users connect to WebSocket without requiring active sessions
- **Lobby as default state**: All users start in "lobby" when connected
- **Session participation**: Users join sessions while maintaining lobby connection
- **Persistent presence**: Users remain connected across session transitions

### Architecture Overview

```
Connection States:
┌─────────────┐    join session    ┌─────────────┐
│   LOBBY     │ ───────────────► │ IN SESSION  │
│ (default)   │ ◄─────────────── │  (active)   │
└─────────────┘   leave session   └─────────────┘
```

Users maintain WebSocket connection throughout, transitioning between lobby and session states.

## Breaking Changes

### 1. System Message Changes
**BEFORE:**
```json
{"type": "system", "context": "session_ended", "content": {"reason": "..."}}
```

**AFTER:**
```json
{"type": "system", "context": "session_left", "content": {"session_id": "xyz", "reason": "..."}}
{"type": "system", "context": "connection_replaced", "content": {"reason": "..."}}
```

### 2. Client Behavior Changes
- **BEFORE**: `session_ended` → disconnect WebSocket
- **AFTER**: `session_left` → return to lobby state, stay connected

### 3. Connection Semantics
- **BEFORE**: Connection tied to specific session
- **AFTER**: Connection persistent, session participation additive

## Implementation Plan

### Phase 1: Server-Side Foundation

#### 1.1 WebSocket Handler Changes
**File**: `internal/websocket/handler.go`

```go
// Allow connections without session validation
if sessionID != "" && sessionID != "lobby" {
    if err := h.sessionManager.ValidateSessionMembership(sessionID, userID, role); err != nil {
        // Handle session validation errors
    }
}
```

#### 1.2 Registry Enhancements
**File**: `internal/websocket/registry.go`

Add methods for lobby-wide broadcasting:
```go
// GetAllConnections returns all connected users
func (r *Registry) GetAllConnections() []*Connection

// GetLobbyConnections returns users not in any session
func (r *Registry) GetLobbyConnections() []*Connection

// BroadcastToAll sends message to all connected users
func (r *Registry) BroadcastToAll(message interface{})
```

#### 1.3 Connection Event Broadcasting
**File**: `internal/websocket/registry.go`

In `RegisterConnection()`:
```go
// Broadcast user_connected to all users
connectionMsg := map[string]interface{}{
    "type": "system",
    "context": "user_connected", 
    "content": map[string]interface{}{
        "user_id": userID,
        "role": role,
        "timestamp": time.Now(),
    },
}
r.BroadcastToAll(connectionMsg)
```

In `UnregisterConnection()`:
```go
// Broadcast user_disconnected to all users
disconnectionMsg := map[string]interface{}{
    "type": "system",
    "context": "user_disconnected",
    "content": map[string]interface{}{
        "user_id": userID,
        "timestamp": time.Now(),
    },
}
r.BroadcastToAll(disconnectionMsg)
```

#### 1.4 Session Event Broadcasting
**File**: `internal/api/server.go`

In `createSession()`:
```go
// Broadcast session_started to enrolled students
sessionStartedMsg := map[string]interface{}{
    "type": "system",
    "context": "session_started",
    "content": map[string]interface{}{
        "session_id": session.ID,
        "session_name": session.Name,
        "instructor_id": session.CreatedBy,
        "student_ids": session.StudentIDs,
        "timestamp": time.Now(),
    },
}

// Send to enrolled students who are connected
for _, studentID := range session.StudentIDs {
    if conn, exists := s.registry.GetUserConnection(studentID); exists {
        conn.WriteJSON(sessionStartedMsg)
    }
}

// Also send to the instructor
if conn, exists := s.registry.GetUserConnection(session.CreatedBy); exists {
    conn.WriteJSON(sessionStartedMsg)
}
```

In `endSession()` - Replace `session_ended` with `session_left`:
```go
sessionLeftMsg := map[string]interface{}{
    "type": "system",
    "context": "session_left",
    "content": map[string]interface{}{
        "session_id": sessionID,
        "reason": "Session ended by instructor",
        "timestamp": time.Now(),
    },
}
```

#### 1.5 Connection Replacement Messages
**File**: `internal/websocket/registry.go`

Update connection replacement message:
```go
connectionReplacedMsg := &types.Message{
    Type: "system",
    Context: "connection_replaced",
    Content: map[string]interface{}{
        "reason": "New connection established",
        "timestamp": time.Now(),
    },
}
```

### Phase 2: SDK Updates

#### 2.1 Python SDK Changes
**File**: `sdk/python/switchboard_sdk/client.py`

Update system message handling:
```python
async def _handle_system_message(self, message: Message) -> None:
    event = self._extract_event(message)
    
    if event == "session_left":
        # Don't disconnect! Return to lobby state
        self.current_session_id = None
        await self._notify_message_handlers(message)
        logger.info("Returned to lobby state")
        return
        
    elif event == "session_started":
        # Check if we should auto-join this session
        session_data = message.content
        if self._should_join_session(session_data):
            await self._join_session(session_data["session_id"])
        await self._notify_message_handlers(message)
        return
        
    elif event == "connection_replaced":
        # Handle graceful connection replacement
        logger.info("Connection replaced by new instance")
        return
        
    elif event == "user_connected" or event == "user_disconnected":
        # Handle presence updates
        await self._notify_message_handlers(message)
        return
```

Add lobby-specific methods:
```python
def is_in_lobby(self) -> bool:
    """Check if currently in lobby state"""
    return self.connected and self.current_session_id is None

async def join_session(self, session_id: str) -> None:
    """Join a specific session while maintaining connection"""
    # Implementation depends on whether we need to reconnect or just update state
```

#### 2.2 Student SDK Updates
**File**: `sdk/python/switchboard_sdk/student.py`

Update connection flow:
```python
async def connect_to_lobby(self) -> None:
    """Connect to lobby and wait for session notifications"""
    await self.connect("lobby")  # or empty session_id

def on_session_available(self, handler):
    """Register handler for session_started events"""
    self.on_message(MessageType.SYSTEM, self._create_session_started_handler(handler))

async def _create_session_started_handler(self, user_handler):
    async def handler(message):
        if message.context == "session_started":
            session_data = message.content
            if self.user_id in session_data.get("student_ids", []):
                await user_handler(session_data)
    return handler
```

#### 2.3 JavaScript/Teacher Client Updates
**File**: `sdk/hint-master/teacher-client/switchboard-sdk.js`

Update system message handling:
```javascript
handleSystemMessage(message) {
    if (message.content.event === 'session_left') {
        this.currentSessionId = null;
        this.emit('sessionLeft', message.content);
    } else if (message.content.event === 'session_started') {
        this.emit('sessionStarted', message.content);
    } else if (message.content.event === 'user_connected') {
        this.emit('userConnected', message.content);
    } else if (message.content.event === 'user_disconnected') {
        this.emit('userDisconnected', message.content);
    } else if (message.content.event === 'connection_replaced') {
        // Handle gracefully, don't emit error
        console.log('Connection replaced by new instance');
    }
    
    // Always emit generic system event
    this.emit('system', message);
}
```

### Phase 3: Client Application Updates

#### 3.1 Hint Agent Updates
**File**: `sdk/hint-master/hint-agent/hint_agent.py`

Remove polling, use event-driven session joining:
```python
def _setup_event_handlers(self):
    @self.client.on_system_message
    async def handle_system_message(message: Message):
        event = message.context
        
        if event == "session_started":
            session_data = message.content
            if self.config.user_id in session_data.get("student_ids", []):
                await self._join_session(session_data["session_id"])
                
        elif event == "session_left":
            # Return to waiting state but stay connected
            print(f"⏳ {self.config.name} waiting for next session...")

async def start(self):
    # Connect to lobby first
    await self.client.connect_to_lobby()
    print(f"✅ {self.config.name} connected to lobby")
    
    # No more polling - just wait for events
    await self._keep_alive()
```

#### 3.2 Teacher Client Updates
**File**: `sdk/hint-master/teacher-client/app.js`

Add presence management:
```javascript
constructor() {
    // ... existing code ...
    this.connectedUsers = new Map(); // Track online users
}

setupEventHandlers() {
    // ... existing handlers ...
    
    // Add presence handlers
    this.teacher.on('userConnected', (data) => {
        this.connectedUsers.set(data.user_id, data);
        this.updatePresenceDisplay();
    });
    
    this.teacher.on('userDisconnected', (data) => {
        this.connectedUsers.delete(data.user_id);
        this.updatePresenceDisplay();
    });
    
    // Handle session events
    this.teacher.on('sessionStarted', (data) => {
        if (data.instructor_id === this.teacher.userId) {
            this.updateSessionUI(data);
        }
    });
}

updatePresenceDisplay() {
    // Update UI to show who's online
    const onlineCount = this.connectedUsers.size;
    document.getElementById('onlineUsers').textContent = onlineCount;
}
```

### Phase 4: Guidelines and Documentation

#### 4.1 Update Student Guidelines
**File**: `sdk/student-client-guideline.md`

Add sections for:
- Lobby connection workflow
- Session event handling
- Presence awareness features
- Migration from polling to event-driven

#### 4.2 Update Teacher Guidelines  
**File**: `sdk/teacher-client-guideline.md`

Add sections for:
- Presence management
- Real-time session notifications
- Connection state handling

## Migration Strategy

### Backward Compatibility Approach
1. **Gradual rollout**: Support both old and new message formats temporarily
2. **Feature flags**: Enable lobby features progressively
3. **Client detection**: Detect client capabilities and adapt behavior

### Deployment Sequence
1. **Server-side changes**: Deploy lobby support first
2. **SDK updates**: Update Python and JavaScript SDKs
3. **Client updates**: Update hint-agent and teacher client
4. **Guidelines**: Update documentation
5. **Cleanup**: Remove deprecated behavior after migration

## Testing Strategy

### Unit Tests
- Registry broadcasting methods
- Connection state management
- Message routing with lobby connections

### Integration Tests
- End-to-end lobby connection flow
- Session joining/leaving transitions
- Presence event broadcasting

### Load Tests
- Lobby scaling with many connected users
- Broadcast performance with large user base
- Connection replacement under load

## Performance Considerations

### Broadcast Efficiency
- Use connection pooling for broadcasts
- Implement message batching for high-frequency events
- Consider rate limiting for presence updates

### Memory Management
- Track lobby connections efficiently
- Clean up stale presence data
- Monitor connection registry size

## Security Considerations

### Presence Privacy
- Control visibility of user presence
- Implement role-based presence filtering
- Audit presence information exposure

### Connection Validation
- Maintain authentication for lobby connections
- Validate user permissions for session joining
- Prevent unauthorized presence access

## Success Metrics

### Performance Metrics
- Session join latency: < 100ms (vs 30s polling)
- Connection success rate: > 99%
- Broadcast delivery time: < 50ms

### User Experience Metrics
- Reduced connection drops
- Faster session discovery
- Improved real-time responsiveness

## Risk Mitigation

### Technical Risks
- **Connection scaling**: Monitor lobby connection limits
- **Message flooding**: Implement rate limiting
- **State synchronization**: Ensure consistent lobby state

### Migration Risks
- **Breaking changes**: Maintain compatibility layers
- **Performance impact**: Monitor system load during rollout
- **User confusion**: Provide clear migration documentation

## Implementation Timeline

### Week 1: Server Foundation
- WebSocket handler changes
- Registry enhancements
- Basic broadcasting

### Week 2: SDK Updates
- Python SDK lobby support
- JavaScript SDK updates
- Message format changes

### Week 3: Client Applications
- Hint agent updates
- Teacher client enhancements
- Event-driven architecture

### Week 4: Testing and Documentation
- Integration testing
- Performance validation
- Guidelines updates

### Week 5: Deployment and Migration
- Staged rollout
- Monitor and adjust
- Complete migration

## Conclusion

The implicit lobby system transforms Switchboard from a session-only platform to a persistent real-time communication system. This design maintains architectural simplicity while dramatically improving user experience through instant notifications, presence awareness, and persistent connections.

The implementation plan ensures backward compatibility during migration while establishing a foundation for future real-time features.