# Student Client Application Development Guideline

## Overview

This guide provides comprehensive instructions for developing student client applications that connect to the Switchboard real-time messaging system. Students participate in instructor-managed sessions and can communicate with teachers through structured message channels with role-based permissions.

## Table of Contents

1. [System Architecture](#system-architecture)
2. [Student Role Overview](#student-role-overview)
3. [Lobby System](#lobby-system)
4. [Session Discovery](#session-discovery)
5. [WebSocket Connection](#websocket-connection)
6. [Message Types and Communication Channels](#message-types-and-communication-channels)
7. [Client Implementation](#client-implementation)
8. [Best Practices](#best-practices)
9. [Error Handling](#error-handling)
10. [Example Implementations](#example-implementations)

## System Architecture

### Core Components

- **HTTP API**: RESTful endpoints for session discovery
- **WebSocket Server**: Real-time bidirectual communication
- **Session Management**: User membership validation
- **Message Router**: Type-based message routing with permissions
- **Database**: Message persistence and history replay

### Student Role Permissions

Students have **automatic session assignment** and can:
- Connect to Switchboard without specifying sessions (server auto-assigns)
- Send 3 specific message types: `instructor_inbox`, `request_response`, `analytics`
- Receive messages from instructors: `inbox_response`, `request`, `instructor_broadcast`
- View filtered message history (only messages relevant to them)

### Permission Restrictions

- **Cannot create or manage sessions** - sessions are instructor-managed
- **Cannot send instructor-only message types** (`inbox_response`, `request`, `instructor_broadcast`)
- **Cannot manually choose sessions** - server automatically assigns based on enrollment
- **Cannot see all session messages** - history is filtered based on relevance

### Auto-Assignment Architecture

When students connect, the server automatically:
1. **Checks active session enrollment** - if student is in `student_ids` of active session → assigns to session
2. **Places in lobby otherwise** - if no active session or not enrolled → assigns to lobby  
3. **Handles transitions** - automatically moves students between lobby and sessions

## Student Role Overview

### Authentication Context

Students connect with:
- **user_id**: Their unique student identifier
- **role**: Always `"student"`
- **No session_id**: Server automatically determines assignment

### Message Visibility Rules

Students receive messages if:
1. **From instructor to them specifically** (`inbox_response`, `request` with `to_user` matching their ID)
2. **Broadcast messages from instructors** (`instructor_broadcast` - no `to_user` field)
3. **Their own sent messages** (for confirmation/history)

Students do NOT see:
- Messages between instructors and other students
- Direct instructor-to-instructor communications
- Analytics from other students

## Lobby System

### Real-Time Connection Management

Switchboard uses an **implicit lobby system** that provides persistent WebSocket connections and real-time notifications:

#### Key Features

- **Always-On Connections**: Students can connect without requiring an active session
- **Instant Session Notifications**: Receive immediate alerts when enrolled sessions start
- **Presence Awareness**: See who's online before sessions begin
- **Persistent State**: Connections remain active across session transitions

#### Connection States

```
┌─────────────┐    join session    ┌─────────────┐
│   LOBBY     │ ───────────────► │ IN SESSION  │
│ (connected) │ ◄─────────────── │  (active)   │
└─────────────┘   leave session   └─────────────┘
```

#### Lobby Benefits for Students

1. **No Polling Delays**: Instant notification when sessions become available
2. **Seamless Transitions**: Move between sessions without reconnection
3. **Enhanced UX**: Know when instructors and peers are online
4. **Automatic Joining**: SDK can auto-join sessions when they start

#### System Messages in Lobby

Students receive real-time system messages:

```javascript
// User presence updates
{"type": "system", "context": "user_connected", "content": {"user_id": "instructor1", "role": "instructor"}}
{"type": "system", "context": "user_disconnected", "content": {"user_id": "student2"}}

// Session lifecycle events  
{"type": "system", "context": "session_started", "content": {"session_id": "xyz", "student_ids": [...]}}
{"type": "system", "context": "session_left", "content": {"session_id": "xyz", "reason": "ended"}}

// Connection management
{"type": "system", "context": "connection_replaced", "content": {"reason": "New connection established"}}
```

#### Using the Lobby System

**Python SDK Example:**
```python
# Connect to lobby first
await student.connect_to_lobby()

# Register for session notifications
@student.on_session_available
async def handle_new_session(session_data):
    print(f"New session available: {session_data['session_name']}")
    # SDK auto-joins if student is enrolled

# Handle presence updates
@student.on_system_message
async def handle_presence(message):
    if message.context == "user_connected":
        print(f"User online: {message.content['user_id']}")
```

**Connection Flow:**
1. **Start in Lobby**: `await student.connect_to_lobby()`
2. **Receive Session Notifications**: Auto-join when sessions start
3. **Participate in Session**: Normal message exchange
4. **Return to Lobby**: Stay connected when session ends

## Server Auto-Assignment

Students don't discover or choose sessions. Instead, the server handles assignment automatically based on enrollment and session state.

### How Auto-Assignment Works

1. **Student connects without session_id**: `ws://localhost:8080/ws?user_id=student_001&role=student`
2. **Server checks active session**: If there's currently an active session
3. **Enrollment verification**: Server checks if student is in `student_ids` array
4. **Assignment decision**:
   - **Enrolled in active session** → Assigns student to that session
   - **Not enrolled or no active session** → Assigns student to lobby

### Single Session Enforcement

The Switchboard server enforces **single session architecture**:
- Only one session can be active at any time
- When teachers create a new session, it becomes the active session
- All enrolled students are automatically moved to the new session **without disconnection**
- When sessions end, students return to lobby **maintaining their WebSocket connection**
- **Seamless transitions**: Students experience no connection interruptions during session changes

### Assignment Examples

**Scenario 1: Student enrolled in active session**
```
Student connects → Server finds active session "React Workshop" → 
Student in student_ids → Assigned to session
```

**Scenario 2: Student not enrolled**
```  
Student connects → Server finds active session "React Workshop" →
Student NOT in student_ids → Assigned to lobby
```

**Scenario 3: No active session**
```
Student connects → No active sessions → Assigned to lobby
```

## WebSocket Connection

### Connection URL Format

```
ws://localhost:8080/ws?user_id={student_id}&role=student
```

**Note**: No `session_id` parameter - server handles assignment automatically.

### Connection Flow

1. **Establish WebSocket Connection**
   ```javascript
   const ws = new WebSocket('ws://localhost:8080/ws?user_id=student_001&role=student');
   ```

2. **Server Auto-Assignment**
   - Server determines session assignment based on enrollment
   - Student is placed in active session (if enrolled) or lobby

3. **Connection Events**
   - `open`: Connection established, auto-assignment complete
   - `message`: Incoming messages from instructors, system notifications
   - `error`: Connection errors (network issues, server problems)
   - `close`: Connection terminated

4. **History Replay** 
   - Server sends filtered message history for assigned location (session or lobby)
   - Includes: instructor broadcasts, direct messages to/from this student
   - Excludes: other students' private conversations with instructors  
   - Ends with `history_complete` system message

### System Assignment Notifications

Students receive system messages about their assignment:

**Assigned to Session**:
```json
{
  "type": "system", 
  "context": "session_started",
  "content": {
    "session_id": "550e8400-e29b-41d4-a716-446655440000",
    "session_name": "React Workshop",
    "instructor_id": "teacher_001",
    "student_ids": ["student_001", "student_002"]
  }
}
```

**Moved to Lobby**:
```json
{
  "type": "system",
  "context": "session_left", 
  "content": {
    "session_id": "550e8400-e29b-41d4-a716-446655440000",
    "reason": "Session ended by instructor"
  }
}
```

**Auto-Transitioned to New Session**:
```json
{
  "type": "system",
  "context": "session_transition",
  "content": {
    "session_id": "new-session-uuid",
    "session_name": "Advanced React Concepts",
    "reason": "Auto-transitioned from lobby to session"
  }
}
```

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

### Student-Sendable Message Types

#### 1. `instructor_inbox` - Questions to All Instructors

**Purpose**: Ask questions that all instructors in the session can see and respond to

**Required Fields**:
- `context`: Question context (e.g., "question", "help", "clarification")
- `content`: Question content
- `to_user`: Must be omitted (questions go to all instructors)

**Example**:
```json
{
  "type": "instructor_inbox",
  "context": "question",
  "content": {
    "text": "How do I handle async operations in useEffect?",
    "code_context": "const [data, setData] = useState(null);",
    "urgency": "medium"
  }
}
```

#### 2. `request_response` - Responses to Instructor Requests

**Purpose**: Respond to specific requests from instructors (received via `request` messages)

**Required Fields**:
- `context`: Response context (should match the original request context)
- `content`: Response content
- `to_user`: Must be omitted (responses go to all instructors)

**Example**:
```json
{
  "type": "request_response",
  "context": "code",
  "content": {
    "code": "const [count, setCount] = useState(0);\nconst increment = () => setCount(count + 1);",
    "explanation": "I'm using useState to manage the counter and a function to increment it",
    "questions": "Is this the best approach for updating state?"
  }
}
```

#### 3. `analytics` - Learning Progress and Activity Data

**Purpose**: Send various analytics about learning progress, engagement, and performance

**Required Fields**:
- `context`: Analytics type (e.g., "progress", "engagement", "performance", "error")
- `content`: Analytics data
- `to_user`: Must be omitted (analytics go to all instructors)

**Analytics Examples**:

**Progress Analytics**:
```json
{
  "type": "analytics",
  "context": "progress",
  "content": {
    "completion_percentage": 75,
    "time_spent_minutes": 30,
    "exercises_completed": 8,
    "exercises_total": 10,
    "current_topic": "React Hooks"
  }
}
```

**Engagement Analytics**:
```json
{
  "type": "analytics",
  "context": "engagement",
  "content": {
    "attention_level": "high",
    "confusion_level": "low",
    "participation_score": 85,
    "last_interaction": "2024-01-15T10:25:00Z"
  }
}
```

**Error Analytics**:
```json
{
  "type": "analytics",
  "context": "error",
  "content": {
    "error_type": "syntax_error",
    "error_message": "Unexpected token '}'",
    "code_context": "const component = () => { return <div>Hello</div>; }",
    "attempted_fixes": 3,
    "time_stuck_minutes": 5
  }
}
```

**Performance Analytics**:
```json
{
  "type": "analytics",
  "context": "performance",
  "content": {
    "typing_speed_wpm": 45,
    "code_completion_time_seconds": 120,
    "tests_passed": 8,
    "tests_failed": 2,
    "score": 80
  }
}
```

### Instructor-Generated Message Types (Received by Students)

#### 1. `inbox_response` - Direct Responses from Instructors

Instructors respond to your `instructor_inbox` questions with direct answers.

**Example Received**:
```json
{
  "type": "inbox_response",
  "context": "answer",
  "content": {
    "text": "Great question! For async operations in useEffect, you should use an async function inside the effect.",
    "code_example": "useEffect(() => { const fetchData = async () => { const result = await api.getData(); setData(result); }; fetchData(); }, []);",
    "additional_resources": ["https://reactjs.org/docs/hooks-effect.html"]
  },
  "from_user": "teacher_001",
  "to_user": "student_001",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2024-01-15T10:18:00Z"
}
```

#### 2. `request` - Direct Requests from Instructors

Instructors ask you for specific information, code, or demonstrations.

**Example Received**:
```json
{
  "type": "request",
  "context": "code",
  "content": {
    "text": "Please share your current component implementation",
    "instructions": "Focus on the state management logic",
    "deadline": "10 minutes",
    "specific_requirements": ["Include useState hooks", "Add comments explaining your logic"]
  },
  "from_user": "teacher_001",
  "to_user": "student_001",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2024-01-15T10:20:00Z"
}
```

#### 3. `instructor_broadcast` - Announcements to All Students

Instructors send information to all students in the session.

**Example Received**:
```json
{
  "type": "instructor_broadcast",
  "context": "announcement",
  "content": {
    "text": "We'll take a 10-minute break. Please save your work.",
    "break_duration": 600,
    "resume_time": "10:15 AM",
    "instructions": "When we return, we'll start with React Router"
  },
  "from_user": "teacher_001",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2024-01-15T10:05:00Z"
}
```

## Client Implementation

### Core Client Structure

```javascript
class StudentSwitchboardClient {
  constructor(serverUrl, studentId) {
    this.serverUrl = serverUrl;
    this.studentId = studentId;
    this.ws = null;
    this.currentSessionId = null;
    this.connected = false;
    this.messageHandlers = new Map();
  }

  // Connection (Auto-assignment)
  async connect() { /* Connect without session_id - server assigns */ }
  disconnect() { /* Disconnect from current assignment */ }
  getCurrentLocation() { /* Check if in session or lobby */ }

  // Message Sending
  async askQuestion(context, content) { /* ... */ }
  async respondToRequest(context, content) { /* ... */ }
  async sendAnalytics(context, content) { /* ... */ }

  // Message Handling
  onInstructorResponse(handler) { /* ... */ }
  onInstructorRequest(handler) { /* ... */ }
  onInstructorBroadcast(handler) { /* ... */ }
  onSystemMessage(handler) { /* ... */ }
}
```

### Message Handling Pattern

```javascript
// Set up message handlers
client.onInstructorResponse((message) => {
  console.log(`Answer from ${message.from_user}: ${message.content.text}`);
  // Display the response in your UI
});

client.onInstructorRequest((message) => {
  console.log(`Request from ${message.from_user}: ${message.content.text}`);
  // Prompt user to respond
  showRequestModal(message);
});

client.onInstructorBroadcast((message) => {
  console.log(`Announcement: ${message.content.text}`);
  // Show notification or update UI
  displayAnnouncement(message.content);
});
```

## Best Practices

### 1. Session Management

- **Enrollment Verification**: Always verify enrollment before attempting connection
- **Session Discovery**: Regularly check for new sessions you've been added to
- **Graceful Handling**: Handle enrollment errors gracefully with clear user feedback

### 2. Communication Patterns

- **Clear Questions**: When using `instructor_inbox`, provide context and specific details
- **Timely Responses**: Respond to instructor `request` messages promptly
- **Relevant Analytics**: Send meaningful analytics that help instructors understand your progress

### 3. User Experience

- **Real-time Feedback**: Show immediate feedback when messages are sent
- **Connection Status**: Display clear connection status indicators
- **Message History**: Show conversation history with instructors
- **Notification System**: Alert users to new instructor messages

### 4. Analytics Best Practices

- **Meaningful Data**: Send analytics that provide valuable insights
- **Regular Updates**: Send progress updates periodically
- **Error Reporting**: Report errors and difficulties to help instructors assist
- **Privacy Awareness**: Be mindful of what information you're sharing

### 5. Performance Considerations

- **Rate Limiting**: Respect the 100 messages/minute limit
- **Efficient Updates**: Batch analytics when possible
- **Connection Management**: Maintain stable connections without unnecessary reconnections

## Error Handling

### Common Error Scenarios

#### 1. Enrollment Errors

```http
HTTP/1.1 403 Forbidden
Content-Type: text/plain

Student not enrolled in session
```

**Handling Strategy**:
```javascript
try {
  await client.connectToSession(sessionId);
} catch (error) {
  if (error.message.includes('not enrolled')) {
    showError('You are not enrolled in this session. Please contact your instructor.');
  }
}
```

#### 2. Session Not Found

```http
HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "error": "Not Found",
  "code": 404,
  "message": "Session not found"
}
```

#### 3. Message Permission Errors

```json
{
  "type": "system",
  "content": {
    "event": "message_error",
    "message": "Failed to send message",
    "error": "Invalid message type for student role"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

#### 4. Connection Errors

```javascript
ws.onerror = (error) => {
  console.error('WebSocket error:', error);
  // Show user-friendly error message
  showConnectionError();
};

ws.onclose = (event) => {
  if (event.code === 4003) { // Custom: enrollment revoked
    showError('You have been removed from this session');
  } else if (event.code !== 1000) {
    // Attempt reconnection
    attemptReconnection();
  }
};
```

### Error Recovery Strategies

1. **Enrollment Verification**: Re-check enrollment if connection fails
2. **Session Refresh**: Refresh session list if session not found
3. **Connection Recovery**: Automatic reconnection with exponential backoff
4. **Message Retry**: Queue and retry failed messages
5. **User Notification**: Clear error messages and recovery instructions

## Example Implementations

### Simple JavaScript Student Client

```javascript
class SimpleStudentClient {
  constructor(serverUrl, studentId) {
    this.serverUrl = serverUrl;
    this.studentId = studentId;
    this.ws = null;
    this.currentSessionId = null;
  }

  async connect() {
    // Connect without session_id - server will auto-assign
    const wsUrl = `ws://localhost:8080/ws?user_id=${this.studentId}&role=student`;
    
    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      console.log('Connected! Server will auto-assign to session or lobby.');
    };

    this.ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      this.handleMessage(message);
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    this.ws.onclose = (event) => {
      console.log('Disconnected');
      if (event.code !== 1000) {
        console.warn('Unexpected disconnection:', event.code);
      }
    };

    return new Promise((resolve, reject) => {
      this.ws.onopen = () => resolve();
      this.ws.onerror = (error) => reject(error);
    });
  }

  getCurrentLocation() {
    return {
      connected: this.ws && this.ws.readyState === WebSocket.OPEN,
      currentSession: this.currentSessionId,
      location: this.currentSessionId === 'lobby' ? 'lobby' : `session:${this.currentSessionId}`
    };
  }

  handleMessage(message) {
    switch (message.type) {
      case 'inbox_response':
        console.log(`Response from ${message.from_user}: ${message.content.text}`);
        this.displayInstructorResponse(message);
        break;
      case 'request':
        console.log(`Request from ${message.from_user}: ${message.content.text}`);
        this.showInstructorRequest(message);
        break;
      case 'instructor_broadcast':
        console.log(`Announcement: ${message.content.text}`);
        this.displayAnnouncement(message);
        break;
      case 'system':
        console.log('System message:', message.content);
        this.handleSystemMessage(message);
        break;
    }
  }

  sendMessage(type, context, content) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('Not connected to session');
    }

    const message = {
      type,
      context,
      content
    };

    this.ws.send(JSON.stringify(message));
  }

  askQuestion(context, content) {
    this.sendMessage('instructor_inbox', context, content);
  }

  respondToRequest(context, content) {
    this.sendMessage('request_response', context, content);
  }

  sendAnalytics(context, content) {
    this.sendMessage('analytics', context, content);
  }

  // UI helper methods
  displayInstructorResponse(message) {
    const responseElement = document.createElement('div');
    responseElement.className = 'instructor-response';
    responseElement.innerHTML = `
      <strong>${message.from_user}:</strong>
      <p>${message.content.text}</p>
      ${message.content.code_example ? `<pre><code>${message.content.code_example}</code></pre>` : ''}
    `;
    document.getElementById('messages').appendChild(responseElement);
  }

  showInstructorRequest(message) {
    const modal = document.getElementById('request-modal');
    document.getElementById('request-text').textContent = message.content.text;
    document.getElementById('request-instructions').textContent = message.content.instructions || '';
    
    // Set up response handler
    document.getElementById('submit-response').onclick = () => {
      const response = document.getElementById('response-input').value;
      this.respondToRequest(message.context, { text: response });
      modal.style.display = 'none';
    };
    
    modal.style.display = 'block';
  }

  displayAnnouncement(message) {
    const notification = document.createElement('div');
    notification.className = 'announcement';
    notification.innerHTML = `
      <strong>📢 Announcement:</strong>
      <p>${message.content.text}</p>
    `;
    document.getElementById('announcements').appendChild(notification);
    
    // Show notification popup
    this.showNotification(message.content.text);
  }

  showNotification(text) {
    if (Notification.permission === 'granted') {
      new Notification('Class Update', { body: text });
    }
  }

  handleSystemMessage(message) {
    const event = message.context || message.content?.event;
    
    switch (event) {
      case 'session_started':
        // Check if we're assigned to this session
        if (message.content?.student_ids?.includes(this.studentId)) {
          this.currentSessionId = message.content.session_id;
          console.log(`Assigned to session: ${message.content.session_name}`);
          this.onSessionAssigned?.(message.content);
        }
        break;
      case 'session_left':
        console.log('Moved back to lobby - session ended');
        this.currentSessionId = 'lobby';
        this.onMovedToLobby?.(message.content);
        break;
      case 'history_complete':
        console.log('Message history loaded');
        this.onHistoryLoaded();
        break;
      case 'message_error':
        console.error('Message error:', message.content.error);
        this.showError(message.content.message);
        break;
    }
  }

  onHistoryLoaded() {
    // Enable UI elements after history is loaded
    document.getElementById('message-input').disabled = false;
    document.getElementById('send-button').disabled = false;
  }

  showError(message) {
    const errorElement = document.createElement('div');
    errorElement.className = 'error-message';
    errorElement.textContent = message;
    document.getElementById('errors').appendChild(errorElement);
    
    // Auto-remove after 5 seconds
    setTimeout(() => errorElement.remove(), 5000);
  }

  disconnect() {
    if (this.ws) {
      this.ws.close(1000, 'User disconnected');
      this.ws = null;
    }
    this.currentSessionId = null;
  }
}

// Usage Example  
async function runStudentSession() {
  const client = new SimpleStudentClient('http://localhost:8080', 'student_001');

  try {
    // Set up event handlers for auto-assignment
    client.onSessionAssigned = (sessionData) => {
      console.log(`🎓 Automatically assigned to: ${sessionData.session_name}`);
      // Update UI to show session mode
    };

    client.onMovedToLobby = (reason) => {
      console.log('📚 Back in lobby - waiting for next session');
      // Update UI to show lobby mode
    };

    // Connect - server will auto-assign to session or lobby
    await client.connect();

    // Send initial analytics
    client.sendAnalytics('engagement', {
      joined_at: new Date().toISOString(),
      device_type: 'desktop',
      browser: navigator.userAgent
    });

    // Example: Ask a question after 30 seconds
    setTimeout(() => {
      client.askQuestion('question', {
        text: 'Could you explain the concept we just covered?',
        topic: 'React Hooks',
        urgency: 'medium'
      });
    }, 30000);

    // Send progress analytics every 5 minutes
    setInterval(() => {
      client.sendAnalytics('progress', {
        time_in_session_minutes: Math.floor((Date.now() - startTime) / 60000),
        attention_level: 'high',
        questions_asked: questionCount,
        responses_given: responseCount
      });
    }, 300000); // 5 minutes

  } catch (error) {
    console.error('Error in student session:', error);
  }
}

// Initialize when page loads
document.addEventListener('DOMContentLoaded', runStudentSession);
```

### Python Student Client Example

```python
import asyncio
import json
import aiohttp
import websockets
from typing import Optional, Dict, Any, List
from datetime import datetime

class StudentSwitchboardClient:
    def __init__(self, server_url: str, student_id: str):
        self.server_url = server_url
        self.student_id = student_id
        self.ws: Optional[websockets.WebSocketServerProtocol] = None
        self.current_session_id: Optional[str] = None
        self.message_handlers = {}

    async def connect(self):
        """Connect to Switchboard - server will auto-assign to session or lobby"""
        # Connect without session_id - let server auto-assign
        ws_url = f"ws://localhost:8080/ws?user_id={self.student_id}&role=student"
        
        self.ws = await websockets.connect(ws_url)
        self.current_session_id = None  # Will be set by server assignment
        
        print("Connected! Server will auto-assign to session or lobby.")
        
        # Start message handling task
        asyncio.create_task(self.handle_messages())

    def get_current_location(self) -> Dict[str, Any]:
        """Get current session/lobby status"""
        return {
            "connected": self.ws is not None,
            "current_session": self.current_session_id,
            "location": "lobby" if self.current_session_id == "lobby" else f"session:{self.current_session_id}"
        }

    async def handle_messages(self):
        if not self.ws:
            return
            
        async for message in self.ws:
            try:
                data = json.loads(message)
                await self.process_message(data)
            except json.JSONDecodeError:
                print(f"Invalid JSON received: {message}")
            except Exception as e:
                print(f"Error processing message: {e}")

    async def process_message(self, message: Dict[str, Any]):
        msg_type = message.get("type")
        
        if msg_type == "inbox_response":
            await self.handle_instructor_response(message)
        elif msg_type == "request":
            await self.handle_instructor_request(message)
        elif msg_type == "instructor_broadcast":
            await self.handle_instructor_broadcast(message)
        elif msg_type == "system":
            await self.handle_system_message(message)

    async def handle_instructor_response(self, message: Dict[str, Any]):
        print(f"Response from {message['from_user']}: {message['content']['text']}")
        
        # Call custom handler if set
        if 'instructor_response' in self.message_handlers:
            await self.message_handlers['instructor_response'](message)

    async def handle_instructor_request(self, message: Dict[str, Any]):
        print(f"Request from {message['from_user']}: {message['content']['text']}")
        
        # Call custom handler if set
        if 'instructor_request' in self.message_handlers:
            await self.message_handlers['instructor_request'](message)

    async def handle_instructor_broadcast(self, message: Dict[str, Any]):
        print(f"Announcement: {message['content']['text']}")
        
        # Call custom handler if set
        if 'instructor_broadcast' in self.message_handlers:
            await self.message_handlers['instructor_broadcast'](message)

    async def handle_system_message(self, message: Dict[str, Any]):
        event = message.get('context') or message.get('content', {}).get('event')
        
        if event == 'session_started':
            # Check if we're assigned to this session
            content = message.get('content', {})
            if self.student_id in content.get('student_ids', []):
                self.current_session_id = content.get('session_id')
                session_name = content.get('session_name', 'Unknown')
                print(f"🎓 Assigned to session: {session_name}")
                
                # Call custom handler if set
                if 'session_assigned' in self.message_handlers:
                    await self.message_handlers['session_assigned'](content)
                    
        elif event == 'session_left':
            print("📚 Moved back to lobby - session ended")
            self.current_session_id = 'lobby'
            
            # Call custom handler if set  
            if 'moved_to_lobby' in self.message_handlers:
                await self.message_handlers['moved_to_lobby'](message.get('content', {}))
                
        elif event == 'history_complete':
            print("Message history loaded")
        elif event == 'message_error':
            print(f"Message error: {message['content']['error']}")

    async def send_message(self, msg_type: str, context: str, content: Dict[str, Any]):
        if not self.ws:
            raise Exception("Not connected to session")

        message = {
            "type": msg_type,
            "context": context,
            "content": content
        }

        await self.ws.send(json.dumps(message))

    async def ask_question(self, context: str, content: Dict[str, Any]):
        await self.send_message("instructor_inbox", context, content)

    async def respond_to_request(self, context: str, content: Dict[str, Any]):
        await self.send_message("request_response", context, content)

    async def send_analytics(self, context: str, content: Dict[str, Any]):
        await self.send_message("analytics", context, content)

    def set_message_handler(self, message_type: str, handler):
        """Set custom message handlers"""
        self.message_handlers[message_type] = handler

    async def disconnect(self):
        if self.ws:
            await self.ws.close()
            self.ws = None
        self.current_session_id = None

# Usage example with auto-assignment
async def interactive_student_session():
    client = StudentSwitchboardClient("http://localhost:8080", "student_001")
    
    try:
        # Set up custom message handlers
        async def handle_request(message):
            print(f"\n🔔 REQUEST from {message['from_user']}:")
            print(f"   {message['content']['text']}")
            if 'instructions' in message['content']:
                print(f"   Instructions: {message['content']['instructions']}")
            
            # In a real app, you'd prompt the user for a response
            # For demo, send an automatic response
            await asyncio.sleep(2)  # Simulate thinking time
            
            response_content = {
                "text": "Here is my response to your request",
                "additional_info": "I hope this helps!"
            }
            
            await client.respond_to_request(message['context'], response_content)
            print("   ✅ Response sent!")

        async def handle_broadcast(message):
            print(f"\n📢 ANNOUNCEMENT: {message['content']['text']}")
            
            # Send engagement analytics when receiving broadcasts
            await client.send_analytics("engagement", {
                "event": "announcement_received",
                "timestamp": datetime.now().isoformat(),
                "attention_level": "high"
            })

        async def handle_session_assigned(session_data):
            session_name = session_data.get('session_name', 'Unknown')
            print(f"\n🎓 AUTO-ASSIGNED to session: {session_name}")
            
            # Send session join analytics
            await client.send_analytics("engagement", {
                "event": "session_joined",
                "timestamp": datetime.now().isoformat(),
                "session_name": session_name
            })

        async def handle_moved_to_lobby(reason_data):
            print(f"\n📚 MOVED TO LOBBY - {reason_data.get('reason', 'session ended')}")

        client.set_message_handler('instructor_request', handle_request)
        client.set_message_handler('instructor_broadcast', handle_broadcast)  
        client.set_message_handler('session_assigned', handle_session_assigned)
        client.set_message_handler('moved_to_lobby', handle_moved_to_lobby)
        
        # Connect - server will auto-assign to session or lobby
        await client.connect()
        
        # Send initial analytics
        await client.send_analytics("engagement", {
            "event": "session_joined",
            "timestamp": datetime.now().isoformat(),
            "device_info": "Python client",
            "session_name": chosen_session['name']
        })
        
        # Simulate student activity
        print("\n🎓 Connected to session! Simulating student activity...")
        
        # Ask a question after 10 seconds
        await asyncio.sleep(10)
        await client.ask_question("question", {
            "text": "Could you explain the homework assignment in more detail?",
            "topic": "Assignment clarification",
            "urgency": "medium"
        })
        print("   ❓ Question sent to instructors")
        
        # Send progress analytics every 30 seconds
        progress_task = asyncio.create_task(send_periodic_analytics(client))
        
        # Keep the connection alive for demo
        await asyncio.sleep(300)  # 5 minutes
        
        progress_task.cancel()
        
    except Exception as e:
        print(f"Error: {e}")
    finally:
        await client.disconnect()

async def send_periodic_analytics(client):
    """Send progress analytics every 30 seconds"""
    start_time = datetime.now()
    question_count = 1
    
    while True:
        await asyncio.sleep(30)
        
        session_duration = (datetime.now() - start_time).total_seconds() / 60
        
        await client.send_analytics("progress", {
            "session_duration_minutes": round(session_duration, 1),
            "questions_asked": question_count,
            "engagement_level": "active",
            "timestamp": datetime.now().isoformat()
        })
        
        print(f"   📊 Progress analytics sent (session duration: {session_duration:.1f} min)")

if __name__ == "__main__":
    asyncio.run(interactive_student_session())
```

### Advanced Features Example

```javascript
// Advanced student client with UI integration
class AdvancedStudentClient extends SimpleStudentClient {
  constructor(serverUrl, studentId, uiCallbacks = {}) {
    super(serverUrl, studentId);
    this.ui = uiCallbacks;
    this.analyticsQueue = [];
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
  }

  // Enhanced connection with auto-reconnect
  async connectToSession(sessionId) {
    try {
      await super.connectToSession(sessionId);
      this.reconnectAttempts = 0;
      
      // Set up automatic reconnection
      this.ws.onclose = (event) => {
        if (event.code !== 1000 && this.reconnectAttempts < this.maxReconnectAttempts) {
          this.attemptReconnection();
        }
      };
      
    } catch (error) {
      this.ui.onError?.(error.message);
      throw error;
    }
  }

  async attemptReconnection() {
    this.reconnectAttempts++;
    const delay = Math.pow(2, this.reconnectAttempts) * 1000; // Exponential backoff
    
    this.ui.onReconnecting?.(this.reconnectAttempts, delay);
    
    setTimeout(async () => {
      try {
        await this.connectToSession(this.currentSessionId);
        this.ui.onReconnected?.();
      } catch (error) {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
          this.attemptReconnection();
        } else {
          this.ui.onReconnectionFailed?.();
        }
      }
    }, delay);
  }

  // Enhanced message handling with UI callbacks
  handleMessage(message) {
    super.handleMessage(message);
    
    // Trigger UI updates
    this.ui.onMessage?.(message);
  }

  // Batch analytics sending
  queueAnalytics(context, content) {
    this.analyticsQueue.push({ context, content, timestamp: Date.now() });
    
    // Send batched analytics every 10 seconds
    if (!this.analyticsBatchTimer) {
      this.analyticsBatchTimer = setInterval(() => {
        this.flushAnalytics();
      }, 10000);
    }
  }

  flushAnalytics() {
    if (this.analyticsQueue.length === 0) return;
    
    const batch = [...this.analyticsQueue];
    this.analyticsQueue = [];
    
    this.sendAnalytics('batch', {
      events: batch,
      batch_size: batch.length
    });
  }

  // Typing indicator for questions
  startTyping(messageType = 'instructor_inbox') {
    this.sendAnalytics('activity', {
      event: 'typing_start',
      message_type: messageType,
      timestamp: Date.now()
    });
  }

  stopTyping() {
    this.sendAnalytics('activity', {
      event: 'typing_stop',
      timestamp: Date.now()
    });
  }

  // Smart question asking with context
  async askSmartQuestion(text, options = {}) {
    const questionData = {
      text,
      timestamp: new Date().toISOString(),
      ...options
    };

    // Add automatic context detection
    if (options.includeCodeContext && window.currentCode) {
      questionData.code_context = window.currentCode;
    }

    // Add urgency based on keywords
    if (!options.urgency) {
      const urgentKeywords = ['error', 'broken', 'stuck', 'help', 'urgent'];
      const isUrgent = urgentKeywords.some(keyword => 
        text.toLowerCase().includes(keyword)
      );
      questionData.urgency = isUrgent ? 'high' : 'medium';
    }

    await this.askQuestion(options.context || 'question', questionData);
    
    // Track question analytics
    this.queueAnalytics('engagement', {
      event: 'question_asked',
      question_length: text.length,
      urgency: questionData.urgency,
      has_code_context: !!questionData.code_context
    });
  }

  // Cleanup
  disconnect() {
    if (this.analyticsBatchTimer) {
      clearInterval(this.analyticsBatchTimer);
      this.flushAnalytics(); // Send remaining analytics
    }
    
    super.disconnect();
  }
}

// Usage with UI integration
const uiCallbacks = {
  onMessage: (message) => {
    updateMessageList(message);
    playNotificationSound();
  },
  onError: (error) => {
    showErrorBanner(error);
  },
  onReconnecting: (attempt, delay) => {
    showReconnectingIndicator(attempt, delay);
  },
  onReconnected: () => {
    hideReconnectingIndicator();
    showSuccessMessage('Reconnected to session');
  },
  onReconnectionFailed: () => {
    showErrorMessage('Could not reconnect. Please refresh the page.');
  }
};

const client = new AdvancedStudentClient('http://localhost:8080', 'student_001', uiCallbacks);
```

This comprehensive student client guideline provides all the necessary information for developing robust student applications that integrate seamlessly with the Switchboard system, including proper error handling, analytics, and user experience considerations.