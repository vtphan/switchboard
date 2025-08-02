# JavaScript SDK Redesign Plan

## Executive Summary

This plan outlines a redesign of the Switchboard JavaScript SDK to create a cleaner, more intuitive API. The core principle: **simplify from 20+ hooks to just 6** (4 message types + 2 state changes) while maintaining full functionality.

## Core Design Principles

1. **Direct mapping to protocol** - 4 hooks match the 4 message types exactly
2. **Clear lifecycle** - Create client first, then connect (not inside connect)
3. **Hidden complexity** - All WebSocket details handled internally
4. **Optional everything** - All hooks are optional with smart defaults
5. **Symmetric API** - What you send is what others receive

## Current Problems

1. **20+ hooks** overwhelm developers
2. **Client creation inside connect()** is conceptually unclear
3. **Complex message builder** pattern for simple sends
4. **Too many connection/session hooks** that could be consolidated

## Proposed Architecture

### Core Client Class

```javascript
class SwitchboardClient {
  constructor(config) {
    this.userId = config.userId;
    this.role = config.role;
    this.wsUrl = config.wsUrl || 'ws://localhost:8080/ws';
    
    // 4 core message hooks (all optional)
    this.messageHandlers = {
      broadcast_to_instructors: config.onBroadcastToInstructors,
      broadcast_to_students: config.onBroadcastToStudents,
      direct_message: config.onDirectMessage,
      system: config.onSystem
    };
    
    // 2 state change hooks (optional with defaults)
    this.onConnectionChange = config.onConnectionChange || this._defaultConnectionHandler;
    this.onSessionChange = config.onSessionChange || this._defaultSessionHandler;
    
    // Internal state
    this.connection = null;
    this.connectionState = 'disconnected';
    this.session = null;
    this.messageQueue = [];
    this.reconnectAttempts = 0;
  }
  
  // Connection management
  async connect() { /* ... */ }
  disconnect() { /* ... */ }
  
  // Message sending - simple object API
  broadcastToInstructors(content) { /* ... */ }
  broadcastToStudents(content) { /* ... */ }
  directMessage(toUser, content) { /* ... */ }
  
  // Session management
  async startSession(name) { /* ... */ }
  async endSession() { /* ... */ }
  // getActiveSession() removed - single-session architecture uses internal state
}
```

### Supported Usage Patterns

The SDK must support these clean, intuitive patterns:

```javascript
// 1. Create client upfront
const client = new SwitchboardClient({
  userId: 'alice',
  role: 'student',
  onBroadcastToStudents: (msg) => displayAnnouncement(msg),
  onSessionChange: (session) => updateUI(session)
});

// 2. Connect with error handling
document.getElementById('connectBtn').onclick = async () => {
  try {
    await client.connect();
  } catch (error) {
    alert('Failed to connect: ' + error.message);
  }
};

// 3. Simple message sending - just pass an object
document.getElementById('askBtn').onclick = () => {
  client.broadcastToInstructors({
    text: questionInput.value,
    context: 'question'
  });
};

// 4. Session management for instructors
startClassBtn.onclick = async () => {
  await client.startSession('Math 101');
};

// 5. Send with any properties
announceBtn.onclick = () => {
  client.broadcastToStudents({
    text: announcementInput.value,
    context: 'announcement',
    important: importantCheckbox.checked
  });
};
```

### Hook Consolidation

**Current SDK: 20+ hooks**
- Connection: onConnecting, onConnected, onDisconnected, onReconnecting, onConnectionError
- Messages: onMessage, onBroadcastToInstructors, onBroadcastToStudents, onDirectMessage
- Session: onSessionStarted, onSessionEnded, onSessionActive, onWaitingForSession, onHistoryDelivered
- Errors: onError, onRateLimited, onNoActiveSession, onMessageTooLarge
- System: onSystemMessage

**New SDK: 6 hooks**
```javascript
// 4 message type hooks (matching database model)
onBroadcastToInstructors(message)  // type: 'broadcast_to_instructors'
onBroadcastToStudents(message)     // type: 'broadcast_to_students'
onDirectMessage(message)           // type: 'direct_message'
onSystem(message)                  // type: 'system'

// 2 state hooks
onConnectionChange(state, error?)   // states: 'disconnected', 'connecting', 'connected', 'error'
onSessionChange(session)           // session: { active: bool, id?, name?, startedBy? }
```

### State Change Details

```javascript
// Connection states
onConnectionChange(state, error) {
  // state: 'disconnected' | 'connecting' | 'connected' | 'error'
  // error: only provided when state === 'error'
}

// Session changes
onSessionChange(session) {
  // session when active: { active: true, id: string, name: string, startedBy: string }
  // session when inactive: { active: false }
}
```

## Implementation Details

### Critical Fix: Context Field Handling

**Problem**: The original plan double-nested the `context` field, violating the server's database schema.

**Server Expected Format** (from `internal/database/models.go`):
```javascript
{
  "type": "broadcast_to_instructors",
  "context": "question",           // Separate field
  "content": { "text": "Hello" }  // Pure data object
}
```

**Fixed Implementation**: Context is extracted from content and placed as a separate field to match the database schema exactly.

### Phase 1: Core Structure

#### 1.1 New Client Implementation
```javascript
// File: src/client-v2.js

class SwitchboardClient {
  constructor(config) {
    // Validation
    if (!config.userId) throw new Error('userId required');
    if (!['student', 'instructor'].includes(config.role)) {
      throw new Error('role must be "student" or "instructor"');
    }
    
    // Configuration
    this.userId = config.userId;
    this.role = config.role;
    this.wsUrl = config.wsUrl || 'ws://localhost:8080/ws';
    this.apiUrl = config.apiUrl || this.wsUrl.replace('ws:', 'http:').replace('/ws', '/api');
    
    // Message handlers
    this.messageHandlers = {
      broadcast_to_instructors: config.onBroadcastToInstructors || null,
      broadcast_to_students: config.onBroadcastToStudents || null,
      direct_message: config.onDirectMessage || null,
      system: config.onSystem || null
    };
    
    // State handlers
    this.onConnectionChange = config.onConnectionChange || this._defaultConnectionHandler;
    this.onSessionChange = config.onSessionChange || this._defaultSessionHandler;
    
    // Internal state (from current implementation)
    this.ws = null;
    this.connectionState = 'disconnected';
    this.sessionActive = false;
    this.currentSession = null;
    this.messageQueue = [];
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = config.maxReconnectAttempts || 10;
    
    // Protocol constants (from current implementation)
    this.maxMessageSize = 64 * 1024;
    this.reconnectBaseDelay = 1000;
    this.maxReconnectDelay = 30000;
    
    // Rate limiting (from current implementation)
    this.messagesSent = [];
    this.rateLimitWindow = 60000;
    this.rateLimitMax = 100;
  }
  
  _defaultConnectionHandler(state, error) {
    if (state === 'error' && error) {
      console.error('Switchboard connection error:', error);
    }
  }
  
  _defaultSessionHandler(session) {
    // No-op by default
  }
}
```

#### 1.2 Connection Management
```javascript
async connect() {
  if (this.connectionState === 'connected') return;
  if (this.connectionState === 'connecting') return;
  
  this._setConnectionState('connecting');
  
  try {
    const url = `${this.wsUrl}?user_id=${encodeURIComponent(this.userId)}&role=${this.role}`;
    this.ws = new WebSocket(url);
    
    await new Promise((resolve, reject) => {
      const timeout = setTimeout(() => reject(new Error('Connection timeout')), 10000);
      
      this.ws.onopen = () => {
        clearTimeout(timeout);
        this._onWebSocketOpen();
        resolve();
      };
      
      this.ws.onerror = (error) => {
        clearTimeout(timeout);
        reject(error);
      };
    });
    
    this._setupWebSocketHandlers();
    
  } catch (error) {
    this._setConnectionState('error', error);
    throw error;
  }
}

disconnect() {
  if (this.ws) {
    this.ws.close(1000, 'Client disconnect');
    this.ws = null;
  }
  this._setConnectionState('disconnected');
  this.reconnectAttempts = 0;
}

_setConnectionState(state, error = null) {
  this.connectionState = state;
  this.onConnectionChange(state, error);
}
```

#### 1.3 Message Routing
```javascript
_setupWebSocketHandlers() {
  this.ws.onmessage = (event) => {
    try {
      const message = JSON.parse(event.data);
      this._routeMessage(message);
    } catch (error) {
      console.error('Failed to parse message:', error);
    }
  };
  
  this.ws.onclose = (event) => {
    this._setConnectionState('disconnected');
    if (event.code !== 1000 && this.reconnectAttempts < this.maxReconnectAttempts) {
      this._scheduleReconnect();
    }
  };
  
  this.ws.onerror = (error) => {
    console.error('WebSocket error:', error);
  };
}

_routeMessage(message) {
  // Handle system messages internally first
  if (message.type === 'system') {
    this._handleSystemMessage(message);
  }
  
  // Route to appropriate handler
  const handler = this.messageHandlers[message.type];
  if (handler) {
    handler(message);
  }
}

_handleSystemMessage(message) {
  const event = message.content?.event;
  
  switch (event) {
    case 'session_started':
      this._setSession({
        active: true,
        id: message.content.session_id,
        name: message.content.session_name,
        startedBy: message.content.started_by || message.content.instructor
      });
      break;
      
    case 'session_ended':
      this._setSession({ active: false });
      break;
      
    case 'session_active':
      this._setSession({
        active: true,
        id: message.content.session_id,
        name: message.content.session_name,
        startedBy: message.content.started_by
      });
      break;
      
    case 'waiting_for_session':
      this._setSession({ active: false });
      break;
  }
  
  // Still pass to handler if provided
  if (this.messageHandlers.system) {
    this.messageHandlers.system(message);
  }
}

_setSession(session) {
  this.sessionActive = session.active;
  this.currentSession = session.active ? session : null;
  this.onSessionChange(session);
}
```

#### 1.4 Message Sending (Critical for Usage Pattern)
```javascript
// Simple object-based sending that supports the usage patterns
broadcastToInstructors(content) {
  return this._send('broadcast_to_instructors', content);
}

broadcastToStudents(content) {
  return this._send('broadcast_to_students', content);
}

directMessage(toUser, content) {
  return this._send('direct_message', content, toUser);
}

_send(type, content, toUser = null) {
  // Support both string and object content
  if (typeof content === 'string') {
    content = { text: content };
  }
  
  // Extract context from content (don't double-nest)
  const context = content.context || 'general';
  
  // Create clean content object without metadata
  const cleanContent = { ...content };
  delete cleanContent.context;  // Remove context from content to avoid double-nesting
  
  // Validate session (unless it's a system context)
  if (!this.sessionActive && context !== 'system') {
    throw new Error('No active session');
  }
  
  // Check rate limit
  if (!this._checkRateLimit()) {
    throw new Error('Rate limit exceeded');
  }
  
  // Build message with correct protocol structure
  const message = {
    type,
    context,           // Context as separate field (database schema requirement)
    content: cleanContent  // Clean content without metadata duplication
  };
  
  if (toUser) {
    message.to_user = toUser;
  }
  
  // Size check
  const messageStr = JSON.stringify(message);
  if (messageStr.length > this.maxMessageSize) {
    throw new Error('Message too large');
  }
  
  // Send or queue
  if (this.connectionState === 'connected' && this.ws.readyState === WebSocket.OPEN) {
    this.ws.send(messageStr);
    this._trackRateLimit();
  } else {
    this.messageQueue.push(messageStr);
  }
  
  return message;
}

_checkRateLimit() {
  const now = Date.now();
  const cutoff = now - this.rateLimitWindow;
  this.messagesSent = this.messagesSent.filter(time => time > cutoff);
  return this.messagesSent.length < this.rateLimitMax;
}

_trackRateLimit() {
  this.messagesSent.push(Date.now());
}
```

### Phase 2: Session Management

```javascript
// Keep existing session management for instructors
async startSession(sessionName) {
  const response = await fetch(`${this.apiUrl}/session/start`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: sessionName,
      instructor_id: this.userId
    })
  });
  
  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.message || `HTTP ${response.status}`);
  }
  
  return await response.json();
}

async endSession() {
  const response = await fetch(`${this.apiUrl}/session/end`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      instructor_id: this.userId
    })
  });
  
  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.message || `HTTP ${response.status}`);
  }
  
  return await response.json();
}

// getActiveSession() method removed - not needed in single-session architecture
// Session state is maintained internally via WebSocket system messages:
// - this.sessionActive (boolean) - true when session is active  
// - this.currentSession (object) - current session details or null
// - onSessionChange hook - called when session state changes
//
// Usage: if (client.sessionActive) { console.log(client.currentSession.name); }
```

### Phase 3: Auto-reconnection and Queue Management

```javascript
_scheduleReconnect() {
  const delay = Math.min(
    this.reconnectBaseDelay * Math.pow(2, this.reconnectAttempts),
    this.maxReconnectDelay
  );
  
  setTimeout(() => {
    this.reconnectAttempts++;
    this.connect().catch(error => {
      console.error('Reconnection failed:', error);
    });
  }, delay);
}

_onWebSocketOpen() {
  this._setConnectionState('connected');
  this.reconnectAttempts = 0;
  this._flushMessageQueue();
}

_flushMessageQueue() {
  while (this.messageQueue.length > 0 && this.ws.readyState === WebSocket.OPEN) {
    const messageStr = this.messageQueue.shift();
    this.ws.send(messageStr);
    this._trackRateLimit();
  }
}
```

## Example Usage Comparison

### Before (Current SDK)
```javascript
// Complex setup with many hooks
const client = new SwitchboardClient({
  userId: 'alice',
  role: 'student',
  wsUrl: 'ws://localhost:8080/ws',
  hooks: {
    onConnecting: () => updateUI('connecting'),
    onConnected: () => updateUI('connected'),
    onDisconnected: () => updateUI('disconnected'),
    onBroadcastToStudents: (msg) => handleAnnouncement(msg),
    onDirectMessage: (msg) => handleDM(msg),
    onSessionStarted: (s) => showSession(s),
    onSessionEnded: () => hideSession(),
    onError: (e) => showError(e),
    // ... many more
  }
});

// Message sending with builder pattern
client.broadcast_to_instructors('helpRequest')
  .withText(questionInput.value)
  .withContext('question')
  .send();
```

### After (New SDK)
```javascript
// Simple setup with only needed hooks
const client = new SwitchboardClient({
  userId: 'alice',
  role: 'student',
  onBroadcastToStudents: (msg) => handleAnnouncement(msg),
  onConnectionChange: (state) => updateUI(state)
});

// Direct message sending
client.broadcastToInstructors({
  text: questionInput.value,
  context: 'question'
});
```

## Testing Strategy

### Unit Tests
- Connection state transitions
- Message routing to correct handlers
- Rate limiting enforcement
- Session state management
- Error handling
- Message queuing and flushing

### Integration Tests
- Full connection lifecycle
- Message send/receive flow
- Session start/end flow
- Reconnection behavior
- Queue flushing on reconnect

### Usage Pattern Tests
```javascript
describe('Usage Patterns', () => {
  it('should support simple message sending', () => {
    const client = new SwitchboardClient({
      userId: 'test',
      role: 'student'
    });
    
    // Should not throw
    client.broadcastToInstructors({
      text: 'Hello',
      context: 'question'
    });
  });
  
  it('should support arbitrary content properties with correct structure', () => {
    const client = new SwitchboardClient({
      userId: 'test',
      role: 'instructor'
    });
    
    const message = client.broadcastToStudents({
      text: 'Announcement',
      context: 'announcement',
      important: true,
      customField: 'value',
      tags: ['urgent', 'exam']
    });
    
    // Verify correct protocol structure (context separate from content)
    expect(message.type).toBe('broadcast_to_students');
    expect(message.context).toBe('announcement');
    expect(message.content.text).toBe('Announcement');
    expect(message.content.important).toBe(true);
    expect(message.content.customField).toBe('value');
    expect(message.content.tags).toEqual(['urgent', 'exam']);
    // Context should NOT be in content (avoid double-nesting)
    expect(message.content.context).toBeUndefined();
  });
});
```

## Critical Fixes Applied

### ✅ **Context Field Double-Nesting Bug (CRITICAL)**
- **Problem**: Original plan passed entire content object including context, causing server protocol violations
- **Fix**: Extract context from content and place as separate field, removing from content object
- **Impact**: Messages now comply with database schema and server validation

### ✅ **Removed getActiveSession() Method**
- **Problem**: Method assumes multi-session architecture, but server uses single global session
- **Fix**: Completely removed method - session state maintained internally via WebSocket
- **Impact**: Aligns with single-session architecture, removes unnecessary API surface

### ✅ **Enhanced Testing**
- **Added**: Protocol compliance verification in tests
- **Added**: Explicit context field separation validation
- **Added**: Content object purity checks

## Implementation Timeline

### Week 1: Core Client Implementation

**Day 1-2: Project Setup**
- [ ] Create `src/client-v2.js` file
- [ ] Set up test infrastructure with Jest
- [ ] Create `tests/client-v2.test.js` with basic test structure
- [ ] Set up development environment for testing against local server

**Day 3-4: Constructor and State Management**
- [ ] Implement SwitchboardClient constructor with validation
- [ ] Add internal state properties (connection, session, queues)
- [ ] Implement default handlers for connection and session changes
- [ ] Write tests for constructor validation and defaults

**Day 5: Connection Management**
- [ ] Implement `connect()` method with WebSocket creation
- [ ] Implement `disconnect()` method with cleanup
- [ ] Add connection state transitions
- [ ] Implement `_setConnectionState()` with hook calls
- [ ] Write tests for connection lifecycle

### Week 2: Message Handling and Sending

**Day 6-7: Message Routing**
- [ ] Implement `_setupWebSocketHandlers()` for all WebSocket events
- [ ] Implement `_routeMessage()` to dispatch to correct handlers
- [ ] Implement `_handleSystemMessage()` for session state updates
- [ ] Write tests for message routing logic

**Day 8-9: Message Sending**
- [ ] Implement `broadcastToInstructors()` method
- [ ] Implement `broadcastToStudents()` method
- [ ] Implement `directMessage()` method
- [ ] Implement `_send()` with validation and queuing
- [ ] Write tests for all sending methods

**Day 10: Advanced Features**
- [ ] Implement auto-reconnection with exponential backoff
- [ ] Implement message queue and `_flushMessageQueue()`
- [ ] Implement rate limiting (`_checkRateLimit()`, `_trackRateLimit()`)
- [ ] Write tests for reconnection and queuing

### Week 3: Session Management and Integration

**Day 11-12: Session Management**
- [ ] Implement `startSession()` with API call
- [ ] Implement `endSession()` with API call
- [ ] Implement `_setSession()` for internal state updates (getActiveSession removed)
- [ ] Write tests for session management

**Day 13-14: Integration Testing**
- [ ] Create `tests/integration/` directory
- [ ] Write end-to-end tests with mock WebSocket server
- [ ] Test complete message flow (send → receive)
- [ ] Test session lifecycle with state changes
- [ ] Test error scenarios and recovery

**Day 15: Examples and Documentation**
- [ ] Update `examples/student/student-app-v2.js` with new SDK
- [ ] Update `examples/teacher/teacher-app-v2.js` with new SDK
- [ ] Create migration guide showing before/after code
- [ ] Update TypeScript definitions in `switchboard-client.d.ts`

### Week 4: Compatibility and Polish

**Day 16-17: Compatibility Layer**
- [ ] Create `createLegacyCompatibleClient()` wrapper function
- [ ] Map old hooks to new 6-hook system
- [ ] Write tests to ensure compatibility wrapper works
- [ ] Add deprecation warnings to old client

**Day 18-19: Performance and Polish**
- [ ] Run performance benchmarks comparing old vs new
- [ ] Optimize hot paths identified in profiling
- [ ] Add debug logging with configurable levels
- [ ] Write stress tests for high message volume

**Day 20: Release Preparation**
- [ ] Update package.json with new entry point
- [ ] Create CHANGELOG.md with breaking changes
- [ ] Update README.md with new usage examples
- [ ] Final test run of all test suites
- [ ] Tag release candidate

## Success Criteria (Test Suites)

### Unit Test Suite (`tests/client-v2.test.js`)
```javascript
describe('SwitchboardClient Core', () => {
  // Constructor tests
  ✓ should require userId
  ✓ should validate role as student or instructor
  ✓ should set default wsUrl
  ✓ should accept all 6 hooks
  ✓ should use default handlers when hooks not provided
  
  // Connection tests
  ✓ should transition through connection states correctly
  ✓ should call onConnectionChange with correct states
  ✓ should handle connection timeout
  ✓ should prevent multiple simultaneous connections
  ✓ should clean up on disconnect
  
  // Message routing tests
  ✓ should route broadcast_to_instructors to correct handler
  ✓ should route broadcast_to_students to correct handler
  ✓ should route direct_message to correct handler
  ✓ should route system messages to handler and update state
  ✓ should handle missing handlers gracefully
  
  // Message sending tests
  ✓ should send objects directly without builder pattern
  ✓ should support string content (convert to {text: string})
  ✓ should enforce rate limiting
  ✓ should queue messages when disconnected
  ✓ should validate message size
  ✓ should require active session for non-system messages
  
  // Session state tests
  ✓ should update session state from system messages
  ✓ should call onSessionChange when session changes
  ✓ should track active session correctly
});
```

### Integration Test Suite (`tests/integration/client-v2.integration.test.js`)
```javascript
describe('SwitchboardClient Integration', () => {
  // Full lifecycle tests
  ✓ should connect, send message, and receive response
  ✓ should handle session start → active → end lifecycle
  ✓ should reconnect automatically after unexpected disconnect
  ✓ should flush message queue after reconnection
  ✓ should receive and handle message history
  
  // Error handling tests
  ✓ should handle rate limit errors gracefully
  ✓ should handle no active session errors
  ✓ should handle message too large errors
  ✓ should recover from network failures
});
```

### Usage Pattern Tests (`tests/usage-patterns.test.js`)
```javascript
describe('Usage Pattern Support', () => {
  ✓ should support simple object-based message sending
  ✓ should support arbitrary content properties
  ✓ should support async/await for connect()
  ✓ should support async/await for session methods
  ✓ should work with minimal configuration
  ✓ should support the documented usage examples exactly
});
```

### Performance Test Suite (`tests/performance/client-v2.perf.test.js`)
```javascript
describe('Performance Benchmarks', () => {
  ✓ should handle 100 messages/second throughput
  ✓ should maintain <10ms message routing latency
  ✓ should use <50MB memory with 1000 stored messages
  ✓ should reconnect within 2 seconds
  ✓ should have no memory leaks over 10,000 messages
});
```

All test suites must pass before considering the implementation complete.

## Summary

This redesign maintains all current functionality while dramatically simplifying the API. The 6-hook design (4 message types + 2 state changes) provides a clean mental model that maps directly to the Switchboard protocol. The simple object-based message sending supports the intuitive usage patterns developers expect, while all complexity is hidden internally.