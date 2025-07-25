# Student Client Application Development Guideline

## Overview

This guide provides comprehensive instructions for developing student client applications that connect to the Switchboard real-time messaging system. Students participate in instructor-managed sessions and can communicate with teachers through structured message channels with role-based permissions.

## Table of Contents

1. [System Architecture](#system-architecture)
2. [Student Role Overview](#student-role-overview)
3. [Session Discovery](#session-discovery)
4. [WebSocket Connection](#websocket-connection)
5. [Message Types and Communication Channels](#message-types-and-communication-channels)
6. [Client Implementation](#client-implementation)
7. [Best Practices](#best-practices)
8. [Error Handling](#error-handling)
9. [Example Implementations](#example-implementations)

## System Architecture

### Core Components

- **HTTP API**: RESTful endpoints for session discovery
- **WebSocket Server**: Real-time bidirectual communication
- **Session Management**: User membership validation
- **Message Router**: Type-based message routing with permissions
- **Database**: Message persistence and history replay

### Student Role Permissions

Students have **restricted session access** and can:
- Connect only to sessions where they are explicitly listed in `student_ids`
- Send 3 specific message types: `instructor_inbox`, `request_response`, `analytics`
- Receive messages from instructors: `inbox_response`, `request`, `instructor_broadcast`
- View filtered message history (only messages relevant to them)

### Permission Restrictions

- **Cannot create or manage sessions** - sessions are instructor-managed
- **Cannot send instructor-only message types** (`inbox_response`, `request`, `instructor_broadcast`)
- **Cannot access sessions they're not enrolled in**
- **Cannot see all session messages** - history is filtered based on relevance

## Student Role Overview

### Authentication Context

Students connect with:
- **user_id**: Their unique student identifier
- **role**: Always `"student"`
- **session_id**: The session they're enrolled in

### Message Visibility Rules

Students receive messages if:
1. **From instructor to them specifically** (`inbox_response`, `request` with `to_user` matching their ID)
2. **Broadcast messages from instructors** (`instructor_broadcast` - no `to_user` field)
3. **Their own sent messages** (for confirmation/history)

Students do NOT see:
- Messages between instructors and other students
- Direct instructor-to-instructor communications
- Analytics from other students

## Session Discovery

Since students cannot create sessions, they need to discover available sessions they're enrolled in.

### Finding Your Sessions

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

### Session Enrollment Check

Students should filter sessions where their `user_id` appears in the `student_ids` array:

```javascript
const mySessionsFilter = (sessions, myUserId) => {
  return sessions.filter(session => 
    session.student_ids.includes(myUserId) && 
    session.status === 'active'
  );
};
```

### Getting Session Details

**Endpoint**: `GET /api/sessions/{session_id}`

```http
GET /api/sessions/550e8400-e29b-41d4-a716-446655440000
```

Use this to verify enrollment and get current session information before connecting.

## WebSocket Connection

### Connection URL Format

```
ws://localhost:8080/ws?user_id={student_id}&role=student&session_id={session_id}
```

### Authentication Flow

1. **Verify Session Enrollment** (recommended pre-check)
   ```javascript
   const session = await fetch(`/api/sessions/${sessionId}`).then(r => r.json());
   if (!session.session.student_ids.includes(myUserId)) {
     throw new Error('Not enrolled in this session');
   }
   ```

2. **Establish WebSocket Connection**
   ```javascript
   const ws = new WebSocket(`ws://localhost:8080/ws?user_id=student_001&role=student&session_id=${sessionId}`);
   ```

3. **Connection Events**
   - `open`: Connection established, filtered history replay begins
   - `message`: Incoming messages from instructors or system
   - `error`: Connection errors (often due to enrollment issues)
   - `close`: Connection terminated

4. **Filtered History Replay**
   - Server sends only messages relevant to this student
   - Includes: instructor broadcasts, direct messages to/from this student
   - Excludes: other students' private conversations with instructors
   - Ends with `history_complete` system message

### Authentication Errors

Common enrollment-related errors:
```http
HTTP/1.1 403 Forbidden
Content-Type: text/plain

Student not enrolled in session
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

  // Session Discovery
  async findAvailableSessions() { /* ... */ }
  async getSessionInfo(sessionId) { /* ... */ }
  async checkEnrollment(sessionId) { /* ... */ }

  // WebSocket Connection
  async connectToSession(sessionId) { /* ... */ }
  disconnect() { /* ... */ }

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

  async findAvailableSessions() {
    const response = await fetch(`${this.serverUrl}/api/sessions`);
    if (!response.ok) {
      throw new Error(`Failed to fetch sessions: ${response.statusText}`);
    }
    
    const data = await response.json();
    
    // Filter sessions where this student is enrolled
    return data.sessions.filter(session => 
      session.student_ids.includes(this.studentId) && 
      session.status === 'active'
    );
  }

  async checkEnrollment(sessionId) {
    const response = await fetch(`${this.serverUrl}/api/sessions/${sessionId}`);
    if (!response.ok) {
      return false;
    }
    
    const data = await response.json();
    return data.session.student_ids.includes(this.studentId);
  }

  async connectToSession(sessionId) {
    // Verify enrollment first
    const isEnrolled = await this.checkEnrollment(sessionId);
    if (!isEnrolled) {
      throw new Error('Not enrolled in this session');
    }

    const wsUrl = `ws://localhost:8080/ws?user_id=${this.studentId}&role=student&session_id=${sessionId}`;
    
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

    this.ws.onclose = (event) => {
      console.log('Disconnected from session');
      if (event.code !== 1000) {
        console.warn('Unexpected disconnection:', event.code);
      }
    };

    return new Promise((resolve, reject) => {
      this.ws.onopen = () => resolve();
      this.ws.onerror = (error) => reject(error);
    });
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
    const event = message.content.event;
    
    switch (event) {
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
    // Find available sessions
    const sessions = await client.findAvailableSessions();
    console.log('Available sessions:', sessions);

    if (sessions.length === 0) {
      console.log('No active sessions found');
      return;
    }

    // Connect to the first available session
    const session = sessions[0];
    await client.connectToSession(session.id);

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

    async def find_available_sessions(self) -> List[Dict[str, Any]]:
        async with aiohttp.ClientSession() as session:
            async with session.get(f"{self.server_url}/api/sessions") as response:
                if response.status != 200:
                    raise Exception(f"Failed to fetch sessions: {response.status}")
                
                data = await response.json()
                
                # Filter sessions where this student is enrolled
                available_sessions = [
                    s for s in data["sessions"] 
                    if self.student_id in s["student_ids"] and s["status"] == "active"
                ]
                
                return available_sessions

    async def check_enrollment(self, session_id: str) -> bool:
        async with aiohttp.ClientSession() as session:
            async with session.get(f"{self.server_url}/api/sessions/{session_id}") as response:
                if response.status != 200:
                    return False
                
                data = await response.json()
                return self.student_id in data["session"]["student_ids"]

    async def connect_to_session(self, session_id: str):
        # Verify enrollment first
        is_enrolled = await self.check_enrollment(session_id)
        if not is_enrolled:
            raise Exception("Not enrolled in this session")

        ws_url = f"ws://localhost:8080/ws?user_id={self.student_id}&role=student&session_id={session_id}"
        
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
        event = message['content']['event']
        
        if event == 'history_complete':
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

# Usage example with interactive features
async def interactive_student_session():
    client = StudentSwitchboardClient("http://localhost:8080", "student_001")
    
    try:
        # Find available sessions
        sessions = await client.find_available_sessions()
        
        if not sessions:
            print("No active sessions found")
            return
        
        print("Available sessions:")
        for i, session in enumerate(sessions):
            print(f"{i + 1}. {session['name']} (ID: {session['id']})")
        
        # Let user choose session (simplified - in real app, use GUI)
        session_choice = 0  # Use first session for demo
        chosen_session = sessions[session_choice]
        
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

        client.set_message_handler('instructor_request', handle_request)
        client.set_message_handler('instructor_broadcast', handle_broadcast)
        
        # Connect to session
        await client.connect_to_session(chosen_session['id'])
        
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