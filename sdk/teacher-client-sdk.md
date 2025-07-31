# Switchboard Teacher Client SDK Contract

## Overview

This document defines the communication protocol and SDK requirements for teacher/instructor clients connecting to the Switchboard V4 educational communication system. Teacher clients have elevated privileges including session management capabilities and visibility of all messages.

## Core Capabilities

### 1. Session Management (Teacher-Only)
- Start new sessions
- End active sessions
- View session history

### 2. Message Types
Teachers can send and receive all three message types:
- `broadcast_to_instructors` - See all student questions
- `direct_message` - Private conversations with any user
- `broadcast_to_students` - Announcements to all students

### 3. Full Visibility
Teachers see all messages in the system regardless of type or recipient.

## Connection Protocol

### WebSocket Connection

**Endpoint**: `ws://[host]:[port]/ws`

**Query Parameters**:
- `user_id` (required): Unique teacher identifier (1-50 chars, alphanumeric + underscore/hyphen)
- `role` (required): Must be `"instructor"`

**Example**:
```
ws://localhost:8080/ws?user_id=prof_smith&role=instructor
```

### Connection States

#### 1. Waiting for Session
When connecting with no active session:
```json
{
  "type": "system",
  "content": {
    "event": "waiting_for_session",
    "message": "Connected successfully. Waiting for instructor to start session."
  },
  "timestamp": "2025-01-15T14:25:00Z"
}
```

#### 2. Active Session
When connecting to an active session:
```json
{
  "type": "system",
  "content": {
    "event": "session_active",
    "session_id": "550e8400-e29b-41d4-a716-446655440000",
    "session_name": "Exercise 3: For Loops",
    "started_by": "prof_smith",
    "start_time": "2025-01-15T14:30:00Z"
  },
  "timestamp": "2025-01-15T14:35:00Z"
}
```

Followed by session history messages (all messages from the current session), ending with:

```json
{
  "type": "system",
  "content": {
    "event": "history_delivered",
    "message": "Session history for Exercise 3: For Loops delivered"
  },
  "timestamp": "2025-01-15T14:35:00Z"
}
```

**History Delivery Rules**:
- Messages delivered in chronological order
- Teachers receive ALL session messages (no filtering)
- History delivery completes before any new real-time messages
- The `history_delivered` event signals completion

### WebSocket Protocol Requirements

#### **Ping/Pong Heartbeat (Automatic)**
Switchboard uses WebSocket protocol-level ping/pong frames (RFC 6455):
- **Server sends**: PING control frames every 30 seconds (empty payload)
- **Client response**: PONG control frames (handled automatically by libraries)
- **Timeout**: Connections not responding within 30 seconds are terminated
- **Implementation**: All major WebSocket libraries handle this automatically

**Verification**: Connect and remain idle for 60+ seconds - connection should stay active.

#### **Message Size Limits**
- **Maximum**: 64KB (65,536 bytes) per message
- **Encoding**: UTF-8 JSON only
- **Violation**: Connection terminated immediately
- **Recommendation**: Validate message size client-side before sending

#### **Connection Close Protocol**
- **Server shutdown**: Close code 1001 (Going Away) with reason "Server shutting down"
- **Client action**: Implement reconnection logic with exponential backoff
- **Other terminations**: Ping timeout, message size violation, rate limiting

#### **Connection Timeout Behavior**
- **Active session**: No timeout - connections persist indefinitely
- **No session**: 25-minute inactivity timeout
- **Cleanup**: Server checks every 30 seconds for stale connections

## Session Management API

### Start Session

**Endpoint**: `POST /api/session/start`

**Request**:
```json
{
  "name": "Exercise 3: For Loops",
  "instructor_id": "prof_smith"
}
```

**Response** (201 Created):
```json
{
  "session": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Exercise 3: For Loops",
    "created_by": "prof_smith",
    "status": "active",
    "start_time": "2025-01-15T14:30:00Z"
  }
}
```

**Error Cases**:
- 400: Invalid session name (must be 1-200 characters)
- 409: Session already active

### End Session

**Endpoint**: `POST /api/session/end`

**Request**:
```json
{
  "instructor_id": "prof_smith"
}
```

**Response** (200 OK):
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "ended",
  "ended_at": "2025-01-15T16:30:00Z"
}
```

**Error Cases**:
- 404: No active session

### Get Active Session

**Endpoint**: `GET /api/session/active`

**Response** (200 OK):
```json
{
  "session": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Exercise 3: For Loops",
    "created_by": "prof_smith",
    "status": "active",
    "start_time": "2025-01-15T14:30:00Z"
  }
}
```

**Response** (204 No Content): No active session

## Message Protocol

### Sending Messages

Messages are sent as JSON over the WebSocket connection.

#### 1. Student Question Broadcast (Teacher sees these from students)
```json
{
  "type": "broadcast_to_instructors",
  "context": "question",
  "content": {
    "text": "I'm confused about the loop condition",
    "code_snippet": "for i := 0; i < len(arr); i++",
    "line_number": 42
  }
}
```

#### 2. Direct Message
```json
{
  "type": "direct_message",
  "to_user": "student123",
  "context": "response",
  "content": {
    "text": "Let me help you with that loop condition...",
    "reference_message_id": "msg-456"
  }
}
```

#### 3. Broadcast to All Students
```json
{
  "type": "broadcast_to_students",
  "context": "announcement",
  "content": {
    "text": "We'll take a 10-minute break",
    "important": true
  }
}
```

### Message Contexts

Valid contexts for semantic categorization:
- `question` - Student questions
- `response` - Replies to questions
- `announcement` - General announcements
- `instruction` - Teaching instructions
- `emergency` - Urgent notifications
- `submission` - Code/answer submissions
- `analytics` - Performance data
- `request` - Action requests
- `peer_help` - Peer assistance
- `general` - General communication

### Receiving Messages

All messages received follow this structure:

```json
{
  "id": "msg-789",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "broadcast_to_instructors",
  "context": "question",
  "from_user": "student456",
  "to_user": null,
  "content": {
    "text": "Question content here"
  },
  "timestamp": "2025-01-15T14:35:30Z"
}
```

### System Messages

System events are delivered with type `"system"`:

```json
{
  "type": "system",
  "content": {
    "event": "user_connected",
    "user_id": "student789",
    "role": "student"
  },
  "timestamp": "2025-01-15T14:36:00Z"
}
```

System events include:
- `session_started`
- `session_ended`
- `user_connected`
- `user_disconnected`
- `waiting_for_session`
- `session_active`

## Error Handling

#### **WebSocket Error Messages**
All errors are sent as JSON messages over the WebSocket connection:

```json
{
  "type": "error",
  "error": "rate_limit_exceeded",
  "message": "Maximum 100 messages per minute exceeded", 
  "timestamp": "2025-01-15T14:37:00Z"
}
```

**Complete Error Code Reference**
- `no_active_session` - Message sent without active session
- `rate_limit_exceeded` - Exceeded 100 messages/minute (rolling window)
- `invalid_message` - Malformed JSON or missing required fields
- `invalid_message_type` - Unknown message type
- `invalid_recipient` - Direct message to non-existent user
- `message_too_large` - Message exceeds 64KB limit

#### **Connection Termination Errors**
These errors result in immediate connection closure:
- Message size > 64KB
- Repeated rate limit violations
- Ping timeout (no pong response)
- Authentication failure

### HTTP API Errors

Standard HTTP status codes with JSON error bodies:

```json
{
  "error": "session_already_active",
  "message": "A session is already in progress"
}
```

## Rate Limiting

- **Limit**: 100 messages per minute per user
- **Window**: Rolling 1-minute window
- **Response**: Error message when exceeded
- **Reset**: Automatic after window passes

## Protocol Implementation Requirements

### **Critical WebSocket Features**

Your WebSocket library MUST support:

1. **Automatic Ping/Pong**: Library automatically responds to ping frames
2. **Close Frame Handling**: Handle close code 1001 (Going Away) gracefully  
3. **Text Frame Support**: Send/receive JSON messages as text frames
4. **Message Size Validation**: Support 64KB message limit

### **Library-Specific Implementation Examples**

#### **JavaScript (Browser)**
```javascript
class TeacherClient {
  constructor(userId, websocketUrl) {
    this.userId = userId;
    this.role = 'instructor';
    this.ws = null;
    this.reconnectAttempts = 0;
    this.maxMessageSize = 64 * 1024; // 64KB limit
  }

  connect() {
    const url = `${websocketUrl}?user_id=${this.userId}&role=${this.role}`;
    this.ws = new WebSocket(url);
    
    this.ws.onopen = () => {
      console.log('Connected to Switchboard');
      this.reconnectAttempts = 0;
    };
    
    this.ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      this.handleMessage(message);
    };
    
    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
    
    this.ws.onclose = (event) => {
      console.log(`Connection closed: ${event.code} - ${event.reason}`);
      if (event.code === 1001) { // Server shutdown
        this.scheduleReconnect();
      }
    };
  }

  sendMessage(type, content, toUser = null, context = 'general') {
    if (this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('Not connected');
    }
    
    const message = { type, context, content };
    if (toUser) message.to_user = toUser;
    
    const messageStr = JSON.stringify(message);
    if (messageStr.length > this.maxMessageSize) {
      throw new Error(`Message exceeds ${this.maxMessageSize} bytes`);
    }
    
    this.ws.send(messageStr);
  }

  scheduleReconnect() {
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
    setTimeout(() => {
      this.reconnectAttempts++;
      this.connect();
    }, delay);
  }
}
```

#### **Python (asyncio websockets)**
```python
import asyncio
import json
import websockets
from websockets.exceptions import ConnectionClosed

class TeacherClient:
    def __init__(self, user_id: str, websocket_url: str):
        self.user_id = user_id
        self.role = 'instructor'
        self.websocket_url = websocket_url
        self.ws = None
        self.max_message_size = 64 * 1024  # 64KB limit
        
    async def connect(self):
        url = f"{self.websocket_url}?user_id={self.user_id}&role={self.role}"
        
        try:
            # websockets library handles ping/pong automatically
            self.ws = await websockets.connect(
                url,
                ping_interval=20,  # Send keepalive pings
                ping_timeout=20,   # Wait for pong response
                max_size=self.max_message_size  # Message size limit
            )
            
            print("Connected to Switchboard")
            
            # Handle incoming messages
            async for message_str in self.ws:
                message = json.loads(message_str)
                await self.handle_message(message)
                
        except ConnectionClosed as e:
            print(f"Connection closed: {e.code} - {e.reason}")
            if e.code == 1001:  # Server shutdown
                await self.reconnect()
                
    async def send_message(self, msg_type: str, content: dict, 
                          to_user: str = None, context: str = 'general'):
        if not self.ws or self.ws.closed:
            raise Exception("Not connected")
            
        message = {'type': msg_type, 'context': context, 'content': content}
        if to_user:
            message['to_user'] = to_user
            
        message_str = json.dumps(message)
        if len(message_str.encode('utf-8')) > self.max_message_size:
            raise Exception(f"Message exceeds {self.max_message_size} bytes")
            
        await self.ws.send(message_str)
```

#### **Go (Gorilla WebSocket)**
```go
package main

import (
    "encoding/json"
    "log"
    "net/url"
    "time"
    
    "github.com/gorilla/websocket"
)

type TeacherClient struct {
    userID    string
    role      string
    wsURL     string
    conn      *websocket.Conn
    maxMsgSize int64
}

func NewTeacherClient(userID, wsURL string) *TeacherClient {
    return &TeacherClient{
        userID:     userID,
        role:       "instructor",
        wsURL:      wsURL,
        maxMsgSize: 64 * 1024, // 64KB
    }
}

func (tc *TeacherClient) Connect() error {
    u, _ := url.Parse(tc.wsURL)
    q := u.Query()
    q.Set("user_id", tc.userID)
    q.Set("role", tc.role)
    u.RawQuery = q.Encode()
    
    conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
    if err != nil {
        return err
    }
    
    tc.conn = conn
    tc.conn.SetReadLimit(tc.maxMsgSize)
    
    // Gorilla WebSocket handles ping/pong automatically
    // Start reading messages
    go tc.readLoop()
    
    return nil
}

func (tc *TeacherClient) readLoop() {
    defer tc.conn.Close()
    
    for {
        _, messageBytes, err := tc.conn.ReadMessage()
        if err != nil {
            if websocket.IsCloseError(err, websocket.CloseGoingAway) {
                log.Println("Server shutdown, attempting reconnect...")
                go tc.reconnect()
            }
            return
        }
        
        var message map[string]interface{}
        json.Unmarshal(messageBytes, &message)
        tc.handleMessage(message)
    }
}

func (tc *TeacherClient) SendMessage(msgType string, content map[string]interface{}, 
                                   toUser *string, context string) error {
    message := map[string]interface{}{
        "type":    msgType,
        "context": context,
        "content": content,
    }
    
    if toUser != nil {
        message["to_user"] = *toUser
    }
    
    messageBytes, _ := json.Marshal(message)
    if int64(len(messageBytes)) > tc.maxMsgSize {
        return fmt.Errorf("message exceeds %d bytes", tc.maxMsgSize)
    }
    
    return tc.conn.WriteMessage(websocket.TextMessage, messageBytes)
}
```

## Implementation Guidelines
```

### 2. Message Sending
```javascript
sendMessage(type, content, toUser = null, context = 'general') {
  if (this.ws.readyState !== WebSocket.OPEN) {
    throw new Error('Not connected');
  }
  
  const message = {
    type: type,
    context: context,
    content: content
  };
  
  if (toUser) {
    message.to_user = toUser;
  }
  
  this.ws.send(JSON.stringify(message));
}
```

### 3. Session Management
```javascript
async startSession(sessionName) {
  const response = await fetch('/api/session/start', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: sessionName,
      instructor_id: this.userId
    })
  });
  
  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.message);
  }
  
  return await response.json();
}
```

### 4. Message Handling
```javascript
handleMessage(message) {
  switch (message.type) {
    case 'broadcast_to_instructors':
      this.onStudentQuestion(message);
      break;
    case 'direct_message':
      this.onDirectMessage(message);
      break;
    case 'broadcast_to_students':
      this.onBroadcast(message);
      break;
    case 'system':
      this.onSystemMessage(message);
      break;
    case 'error':
      this.onError(message);
      break;
  }
}
```

## Best Practices

### 1. Connection Resilience
- Implement automatic reconnection with exponential backoff
- Queue messages during brief disconnections
- Sync state after reconnection

### 2. Message Validation
- Validate message structure before sending
- Handle malformed messages gracefully
- Implement client-side rate limiting

### 3. Session State
- Cache active session information
- Update UI based on session state changes
- Handle session transitions smoothly

### 4. Error Recovery
- Display user-friendly error messages
- Implement retry logic for failed operations
- Log errors for debugging

### 5. Performance
- Batch UI updates for multiple messages
- Implement message pagination for history
- Use efficient data structures for message storage

## Security Considerations

1. **Authentication**: Implement proper authentication before establishing WebSocket connection
2. **Input Validation**: Sanitize all user inputs before sending
3. **TLS**: Use `wss://` protocol in production
4. **Token Management**: If using tokens, refresh before expiration
5. **Content Security**: Escape message content when rendering to prevent XSS

## Testing Checklist

- [ ] Connection establishment with valid credentials
- [ ] Graceful handling of connection failures
- [ ] Message sending for all three types
- [ ] Session start/end operations
- [ ] Rate limit compliance
- [ ] Reconnection after network interruption
- [ ] Proper cleanup on disconnect
- [ ] Error message handling
- [ ] System message processing
- [ ] Message history retrieval