# Switchboard JavaScript SDK V2

A dramatically simplified JavaScript SDK for Switchboard V4 with clean, intuitive API design.

## 🎯 Key Improvements in V2

- **6 hooks instead of 20+** - Only what you need: 4 message types + 2 state changes
- **Simple object-based messaging** - No more builder patterns
- **Fixed protocol compliance** - Context field properly separated (no double-nesting)
- **Hidden complexity** - Auto-reconnection, rate limiting, and queuing built-in
- **Smart defaults** - Works with minimal configuration

## 🚀 Quick Start

```javascript
import SwitchboardClient from './src/client-v2.js';

// Create client with only the hooks you need
const client = new SwitchboardClient({
  userId: 'alice',
  role: 'student',
  
  // Only 4 message type hooks
  onBroadcastToStudents: (msg) => console.log('Announcement:', msg.content.text),
  onDirectMessage: (msg) => console.log('Direct:', msg.content.text),
  
  // 2 state change hooks
  onConnectionChange: (state) => console.log('Connection:', state),
  onSessionChange: (session) => console.log('Session:', session.active ? session.name : 'inactive')
});

// Connect and send messages
await client.connect();

// Simple object-based message sending
client.broadcastToInstructors({
  text: 'I have a question about homework',
  context: 'question',
  urgent: true
});
```

## 📚 Complete API Reference

### Constructor

```javascript
new SwitchboardClient({
  userId: string,              // Required: unique user identifier
  role: 'student'|'instructor', // Required: user role
  wsUrl?: string,              // Optional: WebSocket URL (default: ws://localhost:8080/ws)
  apiUrl?: string,             // Optional: API URL (auto-derived from wsUrl)
  
  // 4 optional message type hooks
  onBroadcastToInstructors?: (message) => void,
  onBroadcastToStudents?: (message) => void,
  onDirectMessage?: (message) => void,
  onSystem?: (message) => void,
  
  // 2 optional state change hooks  
  onConnectionChange?: (state, error?) => void,
  onSessionChange?: (session) => void,
  
  // Optional configuration
  maxReconnectAttempts?: number // Default: 10
})
```

### Connection Management

```javascript
await client.connect()     // Connect to server
client.disconnect()        // Disconnect from server
client.isConnected()       // Check connection status
```

### Message Sending

```javascript
// Send to instructors (students only)
client.broadcastToInstructors({
  text: 'Question text',
  context: 'question',      // Separate context field (no double-nesting)
  urgent: true,             // Any additional properties
  tags: ['homework', 'math']
})

// Send to students (instructors only)
client.broadcastToStudents({
  text: 'Announcement text',
  context: 'announcement',
  important: true
})

// Send direct message
client.directMessage('user123', {
  text: 'Private message',
  context: 'response'
})

// String messages automatically converted
client.broadcastToStudents('Simple string message')
// Becomes: { text: 'Simple string message', context: 'general' }
```

### Session Management (Instructors)

```javascript
// Start session
await client.startSession('Math Class Session')

// End session  
await client.endSession()

// Check session status
client.isSessionActive()     // Returns boolean
client.getCurrentSession()   // Returns session object or null
```

## 🔧 Protocol Compliance

V2 fixes the critical context field double-nesting bug:

```javascript
// ✅ V2 Correct Protocol Structure
{
  "type": "broadcast_to_instructors",
  "context": "question",           // Context as separate field
  "content": {                     // Clean content without context
    "text": "Question text",
    "urgent": true
  }
}

// ❌ V1 Incorrect Structure (fixed in V2)
{
  "type": "broadcast_to_instructors", 
  "context": "question",
  "content": {
    "text": "Question text",
    "context": "question",         // Double-nested! Breaks server
    "urgent": true
  }
}
```

## 📊 Hook Consolidation

### V1: 20+ Hooks
- Connection: onConnecting, onConnected, onDisconnected, onReconnecting, onConnectionError
- Messages: onMessage, onBroadcastToInstructors, onBroadcastToStudents, onDirectMessage  
- Session: onSessionStarted, onSessionEnded, onSessionActive, onWaitingForSession, onHistoryDelivered
- Errors: onError, onRateLimited, onNoActiveSession, onMessageTooLarge
- System: onSystemMessage

### V2: 6 Hooks
```javascript
// 4 message type hooks (direct protocol mapping)
onBroadcastToInstructors(message)
onBroadcastToStudents(message)
onDirectMessage(message)
onSystem(message)

// 2 state hooks (consolidated)
onConnectionChange(state, error?)  // 'disconnected'|'connecting'|'connected'|'error'
onSessionChange(session)           // { active: bool, id?, name?, startedBy? }
```

## 🧪 Testing

### Run Basic Tests
```bash
node test-basic.js
```

### Run Integration Tests (requires server)
```bash
# Start server in project root
make run

# Run integration tests
node test-integration.js
```

### Run Example Apps
```bash
# Start server
make run

# Open in browser
open examples/student/index-v2.html
open examples/teacher/index-v2.html
```

## 📁 Files Structure

```
sdk/javascript/
├── src/
│   ├── client.js          # Original V1 client
│   └── client-v2.js       # New V2 client ⭐
├── tests/
│   ├── client-v2.test.js  # Comprehensive test suite
│   └── setup.js           # Test configuration
├── examples/
│   ├── student/
│   │   ├── index-v2.html  # V2 student demo
│   │   └── student-app-v2.js
│   └── teacher/
│       ├── index-v2.html  # V2 teacher demo
│       └── teacher-app-v2.js
├── test-basic.js          # Basic functionality test
├── test-integration.js    # Server integration test
└── README-V2.md          # This file
```

## 🔄 Migration from V1

### Before (V1)
```javascript
const client = new SwitchboardClient({
  userId: 'alice',
  role: 'student',
  hooks: {
    onConnecting: () => updateUI('connecting'),
    onConnected: () => updateUI('connected'), 
    onDisconnected: () => updateUI('disconnected'),
    onBroadcastToStudents: (msg) => handleMsg(msg),
    onSessionStarted: (s) => showSession(s),
    onSessionEnded: () => hideSession(),
    onError: (e) => showError(e),
    // ... many more hooks
  }
});

// Complex message building
client.broadcast_to_instructors('helpRequest')
  .withText(questionInput.value)
  .withContext('question')
  .withUrgency('high')
  .send();
```

### After (V2)
```javascript
const client = new SwitchboardClient({
  userId: 'alice',
  role: 'student',
  onBroadcastToStudents: (msg) => handleMsg(msg),
  onConnectionChange: (state) => updateUI(state),
  onSessionChange: (session) => session.active ? showSession(session) : hideSession()
});

// Direct message sending
client.broadcastToInstructors({
  text: questionInput.value,
  context: 'question',
  urgency: 'high'
});
```

## ✅ Success Criteria

All implementation goals achieved:

- ✅ Reduced from 20+ hooks to exactly 6 hooks
- ✅ Fixed critical context field double-nesting bug
- ✅ Simple object-based message sending API
- ✅ Hidden complexity (auto-reconnection, rate limiting, queuing)
- ✅ Protocol compliance with server database schema
- ✅ Backwards compatibility support ready
- ✅ Comprehensive test coverage
- ✅ Production-ready example applications

## 🎉 Ready for Production

The V2 SDK is fully implemented and tested. It provides a clean, intuitive API that maps directly to the Switchboard protocol while hiding complexity and ensuring protocol compliance.

Start using V2 today by importing `client-v2.js` instead of `client.js`!