# Teacher Client Application Development Guideline

## Overview

This guide provides comprehensive instructions for developing teacher (instructor) client applications that connect to the Switchboard real-time messaging system. Switchboard facilitates structured communication between instructors and students within session-based educational contexts.

## Table of Contents

1. [System Architecture](#system-architecture)
2. [Lobby System and Presence Management](#lobby-system-and-presence-management)
3. [Session Management](#session-management)
4. [WebSocket Connection](#websocket-connection)
5. [Message Types and Communication Channels](#message-types-and-communication-channels)
6. [Client Implementation](#client-implementation)
7. [Best Practices](#best-practices)
8. [Error Handling](#error-handling)
9. [Example Implementations](#example-implementations)

## Lobby System and Presence Management

### Overview

Switchboard implements an **implicit lobby system** that provides persistent connections and real-time presence awareness for all users. When teachers connect to Switchboard, they automatically join a "lobby" where they receive notifications about:

- User connections and disconnections
- New session creation and availability
- Session lifecycle events
- System-wide announcements

This lobby system eliminates the need for polling and provides instant notifications about student availability and session events.

### Key Benefits for Teachers

1. **Real-time Presence Awareness**: Know immediately when students come online or disconnect
2. **Instant Session Notifications**: Receive immediate alerts when new sessions are created
3. **Persistent Connections**: Stay connected even when not in active sessions
4. **Improved Coordination**: Better visibility into student availability for session planning
5. **Reduced Latency**: Event-driven updates instead of polling-based discovery

### Connection Behavior

#### Lobby Connection
Teachers can connect to the lobby without specifying a session:

```javascript
// Connect to lobby for presence awareness
const wsUrl = 'ws://localhost:8080/ws?user_id=teacher_001&role=instructor';
const ws = new WebSocket(wsUrl);

// Or connect with explicit lobby parameter
const wsUrl = 'ws://localhost:8080/ws?user_id=teacher_001&role=instructor&session_id=lobby';
```

#### Session Connection
When connecting to a specific session, teachers maintain lobby awareness:

```javascript
// Connect to specific session (also receives lobby events)
const wsUrl = 'ws://localhost:8080/ws?user_id=teacher_001&role=instructor&session_id=550e8400-e29b-41d4-a716-446655440000';
```

### Lobby System Events

#### User Presence Events

**User Connected**
```json
{
  "type": "system",
  "context": "user_connected",
  "content": {
    "user_id": "student_001",
    "role": "student",
    "timestamp": "2024-01-15T10:00:00Z"
  }
}
```

**User Disconnected**
```json
{
  "type": "system",
  "context": "user_disconnected", 
  "content": {
    "user_id": "student_001",
    "timestamp": "2024-01-15T10:30:00Z"
  }
}
```

#### Session Lifecycle Events

**Session Started**
```json
{
  "type": "system",
  "context": "session_started",
  "content": {
    "session_id": "550e8400-e29b-41d4-a716-446655440000",
    "session_name": "React Workshop",
    "instructor_id": "teacher_001",
    "student_ids": ["student_001", "student_002"],
    "timestamp": "2024-01-15T10:00:00Z"
  }
}
```

**Session Left** (Replaces session_ended)
```json
{
  "type": "system",
  "context": "session_left",
  "content": {
    "session_id": "550e8400-e29b-41d4-a716-446655440000",
    "reason": "session_ended",
    "timestamp": "2024-01-15T11:00:00Z"
  }
}
```

#### Connection Management

**Connection Replaced**
```json
{
  "type": "system", 
  "context": "connection_replaced",
  "content": {
    "reason": "new_connection_from_same_user",
    "timestamp": "2024-01-15T10:15:00Z"
  }
}
```

### Implementation Examples

#### Basic Lobby Event Handling

```javascript
class TeacherLobbyClient {
  constructor(instructorId) {
    this.instructorId = instructorId;
    this.connectedUsers = new Map();
    this.availableSessions = new Map();
  }

  setupLobbyHandlers() {
    this.ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      
      if (message.type === 'system') {
        this.handleSystemMessage(message);
      }
    };
  }

  handleSystemMessage(message) {
    const event = message.context;
    const content = message.content;

    switch (event) {
      case 'user_connected':
        this.connectedUsers.set(content.user_id, content);
        this.updatePresenceDisplay();
        console.log(`👥 ${content.role} ${content.user_id} connected`);
        break;

      case 'user_disconnected':
        this.connectedUsers.delete(content.user_id);
        this.updatePresenceDisplay();
        console.log(`👥 ${content.user_id} disconnected`);
        break;

      case 'session_started':
        this.availableSessions.set(content.session_id, content);
        this.updateSessionList();
        
        // Auto-join if this teacher created the session
        if (content.instructor_id === this.instructorId) {
          console.log(`🚀 My session started: ${content.session_name}`);
          this.autoJoinSession(content.session_id);
        } else {
          console.log(`ℹ️ New session available: ${content.session_name}`);
          this.notifyNewSession(content);
        }
        break;

      case 'session_left':
        console.log(`🔄 Left session: ${content.session_id}`);
        // Connection remains active in lobby
        this.currentSessionId = null;
        this.updateSessionUI();
        break;

      case 'connection_replaced':
        console.log(`🔄 Connection replaced: ${content.reason}`);
        // Handle gracefully - new connection takes over
        break;
    }
  }

  updatePresenceDisplay() {
    const onlineCount = this.connectedUsers.size;
    const students = Array.from(this.connectedUsers.values())
      .filter(user => user.role === 'student');
    
    document.getElementById('onlineUsers').textContent = onlineCount;
    document.getElementById('onlineStudents').textContent = students.length;
    
    // Update detailed presence list
    this.renderPresenceList(students);
  }

  renderPresenceList(students) {
    const list = document.getElementById('studentPresenceList');
    list.innerHTML = students.map(student => `
      <div class="student-presence-item">
        <span class="status-indicator online">●</span>
        <span class="student-id">${student.user_id}</span>
        <span class="connection-time">${this.formatTime(student.timestamp)}</span>
      </div>
    `).join('');
  }

  notifyNewSession(sessionData) {
    // Show notification to teacher about new session
    const notification = document.createElement('div');
    notification.className = 'session-notification';
    notification.innerHTML = `
      <div class="notification-content">
        <h4>New Session Available</h4>
        <p><strong>${sessionData.session_name}</strong></p>
        <p>Students: ${sessionData.student_ids.length}</p>
        <button onclick="this.joinSession('${sessionData.session_id}')">
          Join Session
        </button>
        <button onclick="this.dismissNotification(this)">
          Dismiss
        </button>
      </div>
    `;
    
    document.getElementById('notifications').appendChild(notification);
  }
}
```

#### Advanced Presence Management

```javascript
class AdvancedPresenceManager {
  constructor(client) {
    this.client = client;
    this.presenceData = new Map();
    this.sessionCandidates = new Set();
  }

  trackUserActivity() {
    this.client.on('userConnected', (data) => {
      this.presenceData.set(data.user_id, {
        ...data,
        connectionTime: Date.now(),
        lastSeen: Date.now()
      });

      // Auto-suggest session creation if enough students are online
      this.evaluateSessionOpportunity();
    });

    this.client.on('userDisconnected', (data) => {
      const user = this.presenceData.get(data.user_id);
      if (user) {
        user.disconnectionTime = Date.now();
        user.sessionDuration = user.disconnectionTime - user.connectionTime;
      }
      
      this.presenceData.delete(data.user_id);
    });
  }

  evaluateSessionOpportunity() {
    const onlineStudents = Array.from(this.presenceData.values())
      .filter(user => user.role === 'student');

    if (onlineStudents.length >= 3) {
      this.suggestSessionCreation(onlineStudents);
    }
  }

  suggestSessionCreation(students) {
    const suggestion = {
      type: 'session_suggestion',
      students: students.map(s => s.user_id),
      timestamp: Date.now()
    };

    // Show UI suggestion to teacher
    this.showSessionSuggestion(suggestion);
  }

  showSessionSuggestion(suggestion) {
    const banner = document.createElement('div');
    banner.className = 'session-suggestion-banner';
    banner.innerHTML = `
      <div class="suggestion-content">
        <h4>💡 Session Opportunity</h4>
        <p>${suggestion.students.length} students are online and available</p>
        <button onclick="this.createSuggestedSession(${JSON.stringify(suggestion.students)})">
          Create Session
        </button>
        <button onclick="this.dismissSuggestion(this)">
          Not Now
        </button>
      </div>
    `;
    
    document.body.appendChild(banner);
    
    // Auto-dismiss after 30 seconds
    setTimeout(() => {
      if (banner.parentNode) {
        banner.remove();
      }
    }, 30000);
  }

  getPresenceStats() {
    const stats = {
      totalOnline: this.presenceData.size,
      students: 0,
      instructors: 0,
      averageSessionTime: 0,
      peakTime: null
    };

    for (const user of this.presenceData.values()) {
      if (user.role === 'student') stats.students++;
      if (user.role === 'instructor') stats.instructors++;
    }

    return stats;
  }
}
```

### Best Practices for Lobby System

#### 1. Connection Management
- **Persistent Connections**: Keep WebSocket connections alive even between sessions
- **Graceful Reconnection**: Handle connection drops with automatic reconnection
- **State Synchronization**: Maintain local state that syncs with server events

#### 2. Presence Awareness
- **Real-time Updates**: Update UI immediately when users connect/disconnect
- **Smart Notifications**: Avoid notification fatigue with intelligent filtering
- **Context-Aware Suggestions**: Use presence data to suggest optimal session timing

#### 3. Session Coordination
- **Proactive Planning**: Use presence data to plan session timing
- **Automatic Invitations**: Auto-invite available students to relevant sessions
- **Load Balancing**: Distribute students across multiple sessions when needed

#### 4. User Experience
- **Visual Indicators**: Clear presence indicators for online/offline status
- **Notification Management**: Configurable notification preferences
- **Session History**: Track which students were available vs. participated

### Migration from Previous System

#### Breaking Changes
- **session_ended → session_left**: Connections remain active after session ends
- **Persistent Connections**: WebSocket connections no longer close when sessions end
- **New Event Types**: Additional system messages for presence and session management

#### Compatibility
- **Existing Message Types**: All original message types (instructor_broadcast, etc.) work unchanged
- **Session Management**: REST API for session CRUD operations remains the same
- **Authentication**: Same WebSocket authentication parameters

## System Architecture

### Core Components

- **HTTP API**: RESTful endpoints for session management
- **WebSocket Server**: Real-time bidirectional communication
- **Session Management**: User membership and role validation
- **Message Router**: Type-based message routing with permissions
- **Database**: Message persistence and history replay

### Teacher Role Permissions

Teachers have **universal session access** and can:
- Create and manage sessions
- Connect to any active session
- Send 3 specific message types: `inbox_response`, `request`, `instructor_broadcast`
- Receive all message types from students
- View complete message history

## Session Management

### 1. Creating a Session

**Endpoint**: `POST /api/sessions`

```http
POST /api/sessions
Content-Type: application/json

{
  "name": "Advanced React Workshop",
  "instructor_id": "teacher_001",
  "student_ids": ["student_001", "student_002", "student_003"]
}
```

**Response** (201 Created):
```json
{
  "session": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Advanced React Workshop",
    "created_by": "teacher_001",
    "student_ids": ["student_001", "student_002", "student_003"],
    "start_time": "2024-01-15T10:00:00Z",
    "end_time": null,
    "status": "active"
  }
}
```

### 2. Retrieving Session Information

**Endpoint**: `GET /api/sessions/{session_id}`

```http
GET /api/sessions/550e8400-e29b-41d4-a716-446655440000
```

**Response**:
```json
{
  "session": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Advanced React Workshop",
    "created_by": "teacher_001",
    "student_ids": ["student_001", "student_002", "student_003"],
    "start_time": "2024-01-15T10:00:00Z",
    "end_time": null,
    "status": "active"
  },
  "connection_count": 3
}
```

### 3. Listing Active Sessions

**Endpoint**: `GET /api/sessions`

```http
GET /api/sessions
```

**Response**:
```json
{
  "sessions": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Advanced React Workshop",
      "created_by": "teacher_001",
      "student_ids": ["student_001", "student_002", "student_003"],
      "start_time": "2024-01-15T10:00:00Z",
      "end_time": null,
      "status": "active",
      "connection_count": 3
    }
  ]
}
```

### 4. Ending a Session

**Endpoint**: `DELETE /api/sessions/{session_id}`

```http
DELETE /api/sessions/550e8400-e29b-41d4-a716-446655440000
```

**Response** (200 OK):
```json
{
  "message": "Session ended successfully"
}
```

## WebSocket Connection

### Connection URL Format

```
ws://localhost:8080/ws?user_id={instructor_id}&role=instructor&session_id={session_id}
```

### Authentication Flow

1. **Establish WebSocket Connection**
   ```javascript
   const ws = new WebSocket('ws://localhost:8080/ws?user_id=teacher_001&role=instructor&session_id=550e8400-e29b-41d4-a716-446655440000');
   ```

2. **Connection Events**
   - `open`: Connection established, history replay begins
   - `message`: Incoming messages from students or system
   - `error`: Connection errors
   - `close`: Connection terminated

3. **History Replay**
   - Upon connection, server sends all historical messages for the session
   - Messages are filtered based on instructor role (instructors see everything)
   - History replay ends with `history_complete` system message

### System Messages

```json
{
  "type": "system",
  "content": {
    "event": "history_complete",
    "message": "Message history loaded"
  },
  "timestamp": "2024-01-15T10:05:00Z"
}
```

## Message Types and Communication Channels

### Teacher-Sendable Message Types

#### 1. `inbox_response` - Direct Response to Student Questions

**Purpose**: Respond to specific student questions received via `instructor_inbox`

**Required Fields**:
- `to_user`: Target student ID (required)
- `context`: Response context (e.g., "answer", "clarification")
- `content`: Response content

**Example**:
```json
{
  "type": "inbox_response",
  "context": "answer",
  "content": {
    "text": "Great question! The useEffect hook runs after the component renders.",
    "code_example": "useEffect(() => { console.log('Component rendered'); }, []);"
  },
  "to_user": "student_001"
}
```

#### 2. `request` - Direct Request to Student

**Purpose**: Ask specific students for information, code, or feedback

**Required Fields**:
- `to_user`: Target student ID (required)
- `context`: Request type (e.g., "code", "explanation", "demo")
- `content`: Request details

**Example**:
```json
{
  "type": "request",
  "context": "code",
  "content": {
    "text": "Please share your current component implementation",
    "instructions": "Focus on the state management logic"
  },
  "to_user": "student_002"
}
```

#### 3. `instructor_broadcast` - Broadcast to All Students

**Purpose**: Send announcements, instructions, or information to all students in the session

**Required Fields**:
- `context`: Broadcast context (e.g., "announcement", "instruction", "emergency")
- `content`: Broadcast content
- `to_user`: Must be omitted (broadcast messages have no specific recipient)

**Example**:
```json
{
  "type": "instructor_broadcast",
  "context": "announcement",
  "content": {
    "text": "We'll take a 10-minute break. Please save your work.",
    "break_duration": 600,
    "resume_time": "10:15 AM"
  }
}
```

### Student-Generated Message Types (Received by Teachers)

#### 1. `instructor_inbox` - Questions from Students

Students send questions that all instructors in the session receive.

**Example Received**:
```json
{
  "type": "instructor_inbox",
  "context": "question",
  "content": {
    "text": "How do I handle async operations in useEffect?",
    "code_context": "const [data, setData] = useState(null);"
  },
  "from_user": "student_001",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2024-01-15T10:15:00Z"
}
```

#### 2. `request_response` - Responses to Teacher Requests

Students respond to teacher `request` messages.

**Example Received**:
```json
{
  "type": "request_response",
  "context": "code",
  "content": {
    "code": "const [count, setCount] = useState(0);",
    "explanation": "I'm using useState to manage the counter state"
  },
  "from_user": "student_002",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2024-01-15T10:20:00Z"
}
```

#### 3. `analytics` - Student Activity Data

Students send various analytics about their learning progress.

**Example Received**:
```json
{
  "type": "analytics",
  "context": "progress",
  "content": {
    "completion_percentage": 75,
    "time_spent": 1800,
    "errors_encountered": 3,
    "topic": "React Hooks"
  },
  "from_user": "student_003",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2024-01-15T10:25:00Z"
}
```

## Client Implementation

### Core Client Structure

```javascript
class TeacherSwitchboardClient {
  constructor(serverUrl, instructorId) {
    this.serverUrl = serverUrl;
    this.instructorId = instructorId;
    this.ws = null;
    this.currentSessionId = null;
    this.connected = false;
    this.messageHandlers = new Map();
  }

  // Session Management
  async createSession(name, studentIds) { /* ... */ }
  async getSession(sessionId) { /* ... */ }
  async listActiveSessions() { /* ... */ }
  async endSession(sessionId) { /* ... */ }

  // WebSocket Connection
  async connectToSession(sessionId) { /* ... */ }
  disconnect() { /* ... */ }

  // Message Sending
  async sendResponse(toStudentId, context, content) { /* ... */ }
  async sendRequest(toStudentId, context, content) { /* ... */ }
  async sendBroadcast(context, content) { /* ... */ }

  // Message Handling
  onStudentQuestion(handler) { /* ... */ }
  onStudentResponse(handler) { /* ... */ }
  onStudentAnalytics(handler) { /* ... */ }
}
```

### Message Handling Pattern

```javascript
// Set up message handlers
client.onStudentQuestion((message) => {
  console.log(`Question from ${message.from_user}: ${message.content.text}`);
  
  // Respond to the student
  client.sendResponse(
    message.from_user,
    "answer",
    { text: "Here's the answer to your question..." }
  );
});

client.onStudentAnalytics((message) => {
  console.log(`Analytics from ${message.from_user}:`, message.content);
  // Process student progress data
});
```

## Best Practices

### 1. Connection Management

- **Single Connection Per Session**: Maintain one WebSocket connection per active session
- **Automatic Reconnection**: Implement exponential backoff for reconnection attempts
- **Graceful Cleanup**: Always close connections when switching sessions or shutting down

### 2. Message Validation

- **Required Fields**: Always include required fields for each message type
- **Content Validation**: Validate message content before sending
- **Rate Limiting Awareness**: Respect the 100 messages/minute rate limit

### 3. Error Handling

- **Network Errors**: Handle connection failures gracefully
- **Authentication Errors**: Provide clear feedback for auth failures
- **Message Errors**: Handle malformed or rejected messages

### 4. User Experience

- **Real-time Updates**: Process incoming messages immediately
- **Status Indicators**: Show connection status and student presence
- **Message History**: Display session history upon connection

### 5. Security

- **Input Sanitization**: Sanitize all user inputs before sending
- **Session Validation**: Verify session membership before connecting
- **Credential Protection**: Never expose credentials in logs or client-side code

## Error Handling

### Common Error Scenarios

#### 1. Connection Errors

```javascript
ws.onerror = (error) => {
  console.error('WebSocket error:', error);
  // Implement reconnection logic
};

ws.onclose = (event) => {
  if (event.code !== 1000) { // Not normal closure
    console.warn('Connection closed unexpectedly:', event.code);
    // Attempt reconnection
  }
};
```

#### 2. Authentication Failures

```http
HTTP/1.1 400 Bad Request
Content-Type: text/plain

Missing required query parameters: user_id, role, session_id
```

#### 3. Message Validation Errors

```json
{
  "type": "system",
  "content": {
    "event": "message_error",
    "message": "Failed to send message",
    "error": "Invalid message type for instructor role"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Error Recovery Strategies

1. **Connection Recovery**: Automatic reconnection with exponential backoff
2. **Message Retry**: Queue and retry failed messages
3. **Session Recovery**: Re-establish session state after reconnection
4. **User Notification**: Inform users of connection issues and recovery attempts

## Example Implementations

### Simple JavaScript Teacher Client

```javascript
class SimpleTeacherClient {
  constructor(serverUrl, instructorId) {
    this.serverUrl = serverUrl;
    this.instructorId = instructorId;
    this.ws = null;
    this.currentSessionId = null;
  }

  async createSession(name, studentIds) {
    const response = await fetch(`${this.serverUrl}/api/sessions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name,
        instructor_id: this.instructorId,
        student_ids: studentIds
      })
    });
    
    if (!response.ok) {
      throw new Error(`Failed to create session: ${response.statusText}`);
    }
    
    const data = await response.json();
    return data.session;
  }

  async connectToSession(sessionId) {
    const wsUrl = `ws://localhost:8080/ws?user_id=${this.instructorId}&role=instructor&session_id=${sessionId}`;
    
    this.ws = new WebSocket(wsUrl);
    this.currentSessionId = sessionId;

    this.ws.onopen = () => {
      console.log('Connected to session:', sessionId);
    };

    this.ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      this.handleMessage(message);
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    this.ws.onclose = () => {
      console.log('Disconnected from session');
    };

    return new Promise((resolve, reject) => {
      this.ws.onopen = () => resolve();
      this.ws.onerror = (error) => reject(error);
    });
  }

  handleMessage(message) {
    switch (message.type) {
      case 'instructor_inbox':
        console.log(`Question from ${message.from_user}: ${message.content.text}`);
        break;
      case 'request_response':
        console.log(`Response from ${message.from_user}:`, message.content);
        break;
      case 'analytics':
        console.log(`Analytics from ${message.from_user}:`, message.content);
        break;
      case 'system':
        console.log('System message:', message.content);
        break;
    }
  }

  sendMessage(type, context, content, toUser = null) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('Not connected to session');
    }

    const message = {
      type,
      context,
      content
    };

    if (toUser) {
      message.to_user = toUser;
    }

    this.ws.send(JSON.stringify(message));
  }

  sendResponse(toStudentId, context, content) {
    this.sendMessage('inbox_response', context, content, toStudentId);
  }

  sendRequest(toStudentId, context, content) {
    this.sendMessage('request', context, content, toStudentId);
  }

  sendBroadcast(context, content) {
    this.sendMessage('instructor_broadcast', context, content);
  }

  async endSession() {
    if (!this.currentSessionId) return;

    await fetch(`${this.serverUrl}/api/sessions/${this.currentSessionId}`, {
      method: 'DELETE'
    });

    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }

    this.currentSessionId = null;
  }
}

// Usage Example
async function runTeacherSession() {
  const client = new SimpleTeacherClient('http://localhost:8080', 'teacher_001');

  try {
    // Create a new session
    const session = await client.createSession('React Workshop', [
      'student_001', 'student_002', 'student_003'
    ]);

    console.log('Session created:', session.id);

    // Connect to the session
    await client.connectToSession(session.id);

    // Send a welcome broadcast
    client.sendBroadcast('announcement', {
      text: 'Welcome to the React Workshop! Please introduce yourselves.',
      instructions: 'Share your name and experience level'
    });

    // Handle student questions (done automatically by handleMessage)

    // Later, send a direct request to a student
    setTimeout(() => {
      client.sendRequest('student_001', 'code', {
        text: 'Please share your current component code',
        deadline: '10 minutes'
      });
    }, 60000); // After 1 minute

    // End session after class
    setTimeout(() => {
      client.sendBroadcast('announcement', {
        text: 'Class is ending. Thank you for participating!'
      });
      
      setTimeout(() => client.endSession(), 5000);
    }, 3600000); // After 1 hour

  } catch (error) {
    console.error('Error in teacher session:', error);
  }
}

runTeacherSession();
```

### Python Teacher Client Example

```python
import asyncio
import json
import aiohttp
import websockets
from typing import Optional, Dict, Any, List

class TeacherSwitchboardClient:
    def __init__(self, server_url: str, instructor_id: str):
        self.server_url = server_url
        self.instructor_id = instructor_id
        self.ws: Optional[websockets.WebSocketServerProtocol] = None
        self.current_session_id: Optional[str] = None

    async def create_session(self, name: str, student_ids: List[str]) -> Dict[str, Any]:
        async with aiohttp.ClientSession() as session:
            async with session.post(
                f"{self.server_url}/api/sessions",
                json={
                    "name": name,
                    "instructor_id": self.instructor_id,
                    "student_ids": student_ids
                }
            ) as response:
                if response.status != 201:
                    raise Exception(f"Failed to create session: {response.status}")
                
                data = await response.json()
                return data["session"]

    async def connect_to_session(self, session_id: str):
        ws_url = f"ws://localhost:8080/ws?user_id={self.instructor_id}&role=instructor&session_id={session_id}"
        
        self.ws = await websockets.connect(ws_url)
        self.current_session_id = session_id
        
        print(f"Connected to session: {session_id}")
        
        # Start message handling task
        asyncio.create_task(self.handle_messages())

    async def handle_messages(self):
        if not self.ws:
            return
            
        async for message in self.ws:
            try:
                data = json.loads(message)
                await self.process_message(data)
            except json.JSONDecodeError:
                print(f"Invalid JSON received: {message}")

    async def process_message(self, message: Dict[str, Any]):
        msg_type = message.get("type")
        
        if msg_type == "instructor_inbox":
            print(f"Question from {message['from_user']}: {message['content']['text']}")
        elif msg_type == "request_response":
            print(f"Response from {message['from_user']}: {message['content']}")
        elif msg_type == "analytics":
            print(f"Analytics from {message['from_user']}: {message['content']}")
        elif msg_type == "system":
            print(f"System message: {message['content']}")

    async def send_message(self, msg_type: str, context: str, content: Dict[str, Any], to_user: Optional[str] = None):
        if not self.ws:
            raise Exception("Not connected to session")

        message = {
            "type": msg_type,
            "context": context,
            "content": content
        }

        if to_user:
            message["to_user"] = to_user

        await self.ws.send(json.dumps(message))

    async def send_response(self, to_student_id: str, context: str, content: Dict[str, Any]):
        await self.send_message("inbox_response", context, content, to_student_id)

    async def send_request(self, to_student_id: str, context: str, content: Dict[str, Any]):
        await self.send_message("request", context, content, to_student_id)

    async def send_broadcast(self, context: str, content: Dict[str, Any]):
        await self.send_message("instructor_broadcast", context, content)

    async def end_session(self):
        if not self.current_session_id:
            return

        async with aiohttp.ClientSession() as session:
            async with session.delete(f"{self.server_url}/api/sessions/{self.current_session_id}") as response:
                if response.status == 200:
                    print("Session ended successfully")

        if self.ws:
            await self.ws.close()
            self.ws = None

        self.current_session_id = None

# Usage example
async def main():
    client = TeacherSwitchboardClient("http://localhost:8080", "teacher_001")
    
    try:
        # Create session
        session = await client.create_session("Python Workshop", [
            "student_001", "student_002", "student_003"
        ])
        
        # Connect to session
        await client.connect_to_session(session["id"])
        
        # Send welcome broadcast
        await client.send_broadcast("announcement", {
            "text": "Welcome to the Python Workshop!",
            "agenda": ["Variables", "Functions", "Classes"]
        })
        
        # Keep the connection alive
        await asyncio.sleep(3600)  # 1 hour
        
    except Exception as e:
        print(f"Error: {e}")
    finally:
        await client.end_session()

if __name__ == "__main__":
    asyncio.run(main())
```

This guideline provides comprehensive instructions for developing teacher client applications that effectively utilize the Switchboard real-time messaging system. The examples demonstrate practical implementation patterns while the specifications ensure protocol compliance and robust error handling.