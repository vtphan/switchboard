# Switchboard Student Client SDK Contract

## Overview

This document defines the communication protocol and SDK requirements for student clients connecting to the Switchboard V4 educational communication system. Student clients have restricted visibility to maintain privacy while enabling effective classroom communication.

## Core Capabilities

### 1. Message Types
Students can send and receive:
- `broadcast_to_instructors` - Send questions visible only to instructors
- `direct_message` - Private conversations (only see messages involving them)
- `broadcast_to_students` - Receive announcements from instructors

### 2. Privacy-Filtered Visibility
Students see only:
- Their own messages
- Direct messages sent to or from them
- Broadcasts to all students
- System messages

Students do NOT see:
- Questions from other students
- Direct messages between other users

## Connection Protocol

### WebSocket Connection

**Endpoint**: `ws://[host]:[port]/ws`

**Query Parameters**:
- `user_id` (required): Unique student identifier (1-50 chars, alphanumeric + underscore/hyphen)
- `role` (required): Must be `"student"`

**Example**:
```
ws://localhost:8080/ws?user_id=student123&role=student
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

Followed by filtered session history (only messages the student is allowed to see), ending with:

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

**History Delivery Rules for Students**:
- Messages delivered in chronological order
- Students see only: own messages + direct messages involving them + broadcasts to students
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

## Message Protocol

### Sending Messages

Messages are sent as JSON over the WebSocket connection.

#### 1. Ask Question to Instructors
```json
{
  "type": "broadcast_to_instructors",
  "context": "question",
  "content": {
    "text": "I don't understand how the for loop increments",
    "code_snippet": "for i := 0; i < 10; i++ {",
    "line_number": 25
  }
}
```

#### 2. Direct Message (to instructor or another student)
```json
{
  "type": "direct_message",
  "to_user": "prof_smith",
  "context": "question",
  "content": {
    "text": "Can I submit my assignment late?",
    "private": true
  }
}
```

#### 3. Peer Help Request
```json
{
  "type": "direct_message",
  "to_user": "student456",
  "context": "peer_help",
  "content": {
    "text": "Did you figure out exercise 3?",
    "exercise_number": 3
  }
}
```

### Message Contexts

Valid contexts for student messages:
- `question` - Questions to instructors
- `submission` - Code/answer submissions
- `peer_help` - Peer assistance requests
- `response` - Replies to messages
- `general` - General communication

### Receiving Messages

#### Messages You'll Receive

1. **Your Own Messages** (echoed back with server metadata):
```json
{
  "id": "msg-123",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "broadcast_to_instructors",
  "context": "question",
  "from_user": "student123",
  "to_user": null,
  "content": {
    "text": "Your question here"
  },
  "timestamp": "2025-01-15T14:35:30Z"
}
```

2. **Direct Messages To/From You**:
```json
{
  "id": "msg-456",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "direct_message",
  "context": "response",
  "from_user": "prof_smith",
  "to_user": "student123",
  "content": {
    "text": "Yes, you can submit until midnight"
  },
  "timestamp": "2025-01-15T14:36:00Z"
}
```

3. **Broadcasts to All Students**:
```json
{
  "id": "msg-789",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "broadcast_to_students",
  "context": "announcement",
  "from_user": "prof_smith",
  "to_user": null,
  "content": {
    "text": "Remember to save your work before the break",
    "important": true
  },
  "timestamp": "2025-01-15T14:37:00Z"
}
```

### System Messages

System events are delivered with type `"system"`:

```json
{
  "type": "system",
  "content": {
    "event": "session_started",
    "session_name": "Exercise 3: For Loops",
    "instructor": "prof_smith"
  },
  "timestamp": "2025-01-15T14:30:00Z"
}
```

System events students receive:
- `session_started` - New session began
- `session_ended` - Current session ended
- `waiting_for_session` - Connected but no active session
- `session_active` - Connected to active session

## Error Handling

#### **WebSocket Error Messages**
All errors are sent as JSON messages over the WebSocket connection:

```json
{
  "type": "error",
  "error": "no_active_session",
  "message": "Cannot send messages without an active session",
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
class StudentClient {
  constructor(userId, websocketUrl) {
    this.userId = userId;
    this.role = 'student';
    this.ws = null;
    this.sessionActive = false;
    this.reconnectAttempts = 0;
    this.maxMessageSize = 64 * 1024; // 64KB limit
  }

  connect() {
    const url = `${websocketUrl}?user_id=${this.userId}&role=${this.role}`;
    this.ws = new WebSocket(url);
    
    this.ws.onopen = () => {
      console.log('Connected to class');
      this.reconnectAttempts = 0;
    };
    
    this.ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      this.handleMessage(message);
    };
    
    this.ws.onerror = (error) => {
      console.error('Connection error:', error);
    };
    
    this.ws.onclose = (event) => {
      console.log(`Connection closed: ${event.code} - ${event.reason}`);
      this.sessionActive = false;
      if (event.code === 1001) { // Server shutdown
        this.scheduleReconnect();
      }
    };
  }

  sendDirectMessage(toUser, text, context = 'general') {
    if (!this.sessionActive) {
      throw new Error('No active class session');
    }
    
    this.sendMessage('direct_message', { text }, toUser, context);
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

  handleMessage(message) {
    switch (message.type) {
      case 'broadcast_to_students':
        this.onAnnouncement(message);
        break;
      case 'direct_message':
        this.onDirectMessage(message);
        break;
      case 'broadcast_to_instructors':
        // Your own question echoed back
        this.onOwnQuestion(message);
        break;
      case 'system':
        this.handleSystemMessage(message);
        break;
      case 'error':
        this.onError(message);
        break;
    }
  }

  handleSystemMessage(message) {
    switch (message.content.event) {
      case 'session_started':
        this.sessionActive = true;
        this.onSessionStarted(message.content);
        break;
      case 'session_ended':
        this.sessionActive = false;
        this.onSessionEnded();
        break;
      case 'waiting_for_session':
        this.sessionActive = false;
        this.onWaitingForSession();
        break;
      case 'session_active':
        this.sessionActive = true;
        this.onSessionActive(message.content);
        break;
      case 'history_delivered':
        this.onHistoryComplete();
        break;
    }
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

class StudentClient:
    def __init__(self, user_id: str, websocket_url: str):
        self.user_id = user_id
        self.role = 'student'
        self.websocket_url = websocket_url
        self.ws = None
        self.session_active = False
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
            
            print("Connected to class")
            
            # Handle incoming messages
            async for message_str in self.ws:
                message = json.loads(message_str)
                await self.handle_message(message)
                
        except ConnectionClosed as e:
            print(f"Connection closed: {e.code} - {e.reason}")
            self.session_active = False
            if e.code == 1001:  # Server shutdown
                await self.reconnect()
                
    async def ask_question(self, question_text: str, code_snippet: str = None):
        if not self.session_active:
            raise Exception("No active class session")
            
        content = {'text': question_text}
        if code_snippet:
            content['code_snippet'] = code_snippet
            
        await self.send_message('broadcast_to_instructors', content, context='question')
        
    async def send_direct_message(self, to_user: str, text: str, context: str = 'general'):
        if not self.session_active:
            raise Exception("No active class session")
            
        await self.send_message('direct_message', {'text': text}, to_user, context)
        
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
        
    async def handle_message(self, message: dict):
        msg_type = message.get('type')
        
        if msg_type == 'broadcast_to_students':
            await self.on_announcement(message)
        elif msg_type == 'direct_message':
            await self.on_direct_message(message)
        elif msg_type == 'broadcast_to_instructors':
            # Own question echoed back
            await self.on_own_question(message)
        elif msg_type == 'system':
            await self.handle_system_message(message)
        elif msg_type == 'error':
            await self.on_error(message)
            
    async def handle_system_message(self, message: dict):
        event = message.get('content', {}).get('event')
        
        if event == 'session_started':
            self.session_active = True
            await self.on_session_started(message['content'])
        elif event == 'session_ended':
            self.session_active = False
            await self.on_session_ended()
        elif event == 'waiting_for_session':
            self.session_active = False
            await self.on_waiting_for_session()
        elif event == 'session_active':
            self.session_active = True
            await self.on_session_active(message['content'])
        elif event == 'history_delivered':
            await self.on_history_complete()
```

#### **Go (Gorilla WebSocket)**
```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/url"
    "time"
    
    "github.com/gorilla/websocket"
)

type StudentClient struct {
    userID        string
    role          string
    wsURL         string
    conn          *websocket.Conn
    sessionActive bool
    maxMsgSize    int64
}

func NewStudentClient(userID, wsURL string) *StudentClient {
    return &StudentClient{
        userID:     userID,
        role:       "student",
        wsURL:      wsURL,
        maxMsgSize: 64 * 1024, // 64KB
    }
}

func (sc *StudentClient) Connect() error {
    u, _ := url.Parse(sc.wsURL)
    q := u.Query()
    q.Set("user_id", sc.userID)
    q.Set("role", sc.role)
    u.RawQuery = q.Encode()
    
    conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
    if err != nil {
        return err
    }
    
    sc.conn = conn
    sc.conn.SetReadLimit(sc.maxMsgSize)
    
    // Gorilla WebSocket handles ping/pong automatically
    // Start reading messages
    go sc.readLoop()
    
    log.Println("Connected to class")
    return nil
}

func (sc *StudentClient) readLoop() {
    defer sc.conn.Close()
    
    for {
        _, messageBytes, err := sc.conn.ReadMessage()
        if err != nil {
            if websocket.IsCloseError(err, websocket.CloseGoingAway) {
                log.Println("Server shutdown, attempting reconnect...")
                sc.sessionActive = false
                go sc.reconnect()
            }
            return
        }
        
        var message map[string]interface{}
        json.Unmarshal(messageBytes, &message)
        sc.handleMessage(message)
    }
}

func (sc *StudentClient) AskQuestion(questionText string, codeSnippet *string) error {
    if !sc.sessionActive {
        return fmt.Errorf("no active class session")
    }
    
    content := map[string]interface{}{"text": questionText}
    if codeSnippet != nil {
        content["code_snippet"] = *codeSnippet
    }
    
    return sc.SendMessage("broadcast_to_instructors", content, nil, "question")
}

func (sc *StudentClient) SendDirectMessage(toUser, text, context string) error {
    if !sc.sessionActive {
        return fmt.Errorf("no active class session")
    }
    
    content := map[string]interface{}{"text": text}
    return sc.SendMessage("direct_message", content, &toUser, context)
}

func (sc *StudentClient) SendMessage(msgType string, content map[string]interface{}, 
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
    if int64(len(messageBytes)) > sc.maxMsgSize {
        return fmt.Errorf("message exceeds %d bytes", sc.maxMsgSize)
    }
    
    return sc.conn.WriteMessage(websocket.TextMessage, messageBytes)
}

func (sc *StudentClient) handleMessage(message map[string]interface{}) {
    msgType, _ := message["type"].(string)
    
    switch msgType {
    case "broadcast_to_students":
        sc.onAnnouncement(message)
    case "direct_message":
        sc.onDirectMessage(message)
    case "broadcast_to_instructors":
        // Own question echoed back
        sc.onOwnQuestion(message)
    case "system":
        sc.handleSystemMessage(message)
    case "error":
        sc.onError(message)
    }
}

func (sc *StudentClient) handleSystemMessage(message map[string]interface{}) {
    content, _ := message["content"].(map[string]interface{})
    event, _ := content["event"].(string)
    
    switch event {
    case "session_started":
        sc.sessionActive = true
        sc.onSessionStarted(content)
    case "session_ended":
        sc.sessionActive = false
        sc.onSessionEnded()
    case "waiting_for_session":
        sc.sessionActive = false
        sc.onWaitingForSession()
    case "session_active":
        sc.sessionActive = true
        sc.onSessionActive(content)
    case "history_delivered":
        sc.onHistoryComplete()
    }
}
```

## Implementation Guidelines
```

### 2. Sending Messages to Instructors
```javascript
// Send question using explicit protocol method
sendMessage('broadcast_to_instructors', content, null, 'question') {
  if (!this.sessionActive) {
    throw new Error('No active class session');
  }
  
  const message = {
    type: 'broadcast_to_instructors',
    context: 'question',
    content: content
  };
  
  this.ws.send(JSON.stringify(message));
}
```

### 3. Direct Messaging
```javascript
sendDirectMessage(toUser, text, context = 'general') {
  if (!this.sessionActive) {
    throw new Error('No active class session');
  }
  
  const message = {
    type: 'direct_message',
    to_user: toUser,
    context: context,
    content: { text: text }
  };
  
  this.ws.send(JSON.stringify(message));
}
```

### 4. Message Handling
```javascript
handleMessage(message) {
  switch (message.type) {
    case 'broadcast_to_students':
      this.onAnnouncement(message);
      break;
    case 'direct_message':
      this.onDirectMessage(message);
      break;
    case 'broadcast_to_instructors':
      // This will be your own question echoed back
      this.onOwnQuestion(message);
      break;
    case 'system':
      this.handleSystemMessage(message);
      break;
    case 'error':
      this.onError(message);
      break;
  }
}

handleSystemMessage(message) {
  switch (message.content.event) {
    case 'session_started':
      this.sessionActive = true;
      this.onSessionStarted(message.content);
      break;
    case 'session_ended':
      this.sessionActive = false;
      this.onSessionEnded();
      break;
    case 'waiting_for_session':
      this.sessionActive = false;
      this.onWaitingForSession();
      break;
    case 'session_active':
      this.sessionActive = true;
      this.onSessionActive(message.content);
      break;
  }
}
```

## Best Practices

### 1. User Experience
- Show clear connection status
- Indicate when waiting for class to start
- Display sent questions with pending/sent status
- Notify on instructor responses

### 2. Message Management
- Cache sent messages locally
- Mark messages as delivered when echoed back
- Handle message failures gracefully
- Implement local message queue for offline

### 3. Privacy Awareness
- Don't attempt to access other students' questions
- Respect the privacy model of the system
- Only display messages the server sends to you

### 4. Performance
- Throttle rapid message sending client-side
- Batch UI updates for better performance
- Implement virtual scrolling for message history
- Clear old messages to manage memory

### 5. Error Handling
- Show user-friendly error messages
- Provide clear feedback on rate limiting
- Auto-retry connection on network issues
- Guide users when session isn't active

## Security Considerations

1. **Input Sanitization**: Clean all user inputs before sending
2. **Content Rendering**: Escape HTML when displaying messages
3. **Authentication**: Validate user identity before connecting
4. **TLS**: Use `wss://` protocol in production
5. **Local Storage**: Encrypt any cached messages

## Common Scenarios

### Scenario 1: Joining Late
```javascript
// Student connects after session started
// 1. Receives session_active message
// 2. Receives filtered history (own messages + broadcasts)
// 3. Can immediately start participating
```

### Scenario 2: Asking a Question
```javascript
// 1. Student sends broadcast_to_instructors
// 2. Receives echo of own message
// 3. Only instructors see the question
// 4. Instructor may respond via direct_message
```

### Scenario 3: Class Ending
```javascript
// 1. Instructor ends session
// 2. All students receive session_ended
// 3. New messages are rejected
// 4. Connection remains open (waiting state)
```

## Testing Checklist

- [ ] Connect with valid student credentials
- [ ] Handle waiting for session state
- [ ] Send questions to instructors
- [ ] Receive and display announcements
- [ ] Send/receive direct messages
- [ ] Handle session start/end transitions
- [ ] Respect rate limits
- [ ] Reconnect after network loss
- [ ] Verify privacy filtering works
- [ ] Handle all error types gracefully

## Example Applications

### Minimal Web Client
```html
<!DOCTYPE html>
<html>
<head>
    <title>Student Dashboard</title>
</head>
<body>
    <div id="status">Connecting...</div>
    <div id="messages"></div>
    <input type="text" id="question" placeholder="Ask a question...">
    <button onclick="sendQuestion()">Send to Instructor</button>

    <script>
        const client = new StudentClient('student123', 'ws://localhost:8080/ws');
        
        client.onSessionStarted = (session) => {
            document.getElementById('status').textContent = 
                `Class: ${session.session_name}`;
        };
        
        client.onAnnouncement = (message) => {
            const div = document.createElement('div');
            div.className = 'announcement';
            div.textContent = `📢 ${message.content.text}`;
            document.getElementById('messages').appendChild(div);
        };
        
        function sendQuestion() {
            const input = document.getElementById('question');
            client.sendMessage('broadcast_to_instructors', 
                { text: input.value }, null, 'question');
            input.value = '';
        }
        
        client.connect();
    </script>
</body>
</html>
```

### React Native Mobile Client
```javascript
import { StudentClient } from './switchboard-sdk';

export default function ClassroomScreen({ studentId }) {
  const [messages, setMessages] = useState([]);
  const [connected, setConnected] = useState(false);
  
  useEffect(() => {
    const client = new StudentClient(studentId, WS_URL);
    
    client.onSessionActive = () => setConnected(true);
    client.onAnnouncement = (msg) => {
      setMessages(prev => [...prev, msg]);
    };
    
    client.connect();
    return () => client.disconnect();
  }, [studentId]);
  
  return (
    <View>
      <FlatList data={messages} renderItem={MessageItem} />
      <QuestionInput enabled={connected} />
    </View>
  );
}
```