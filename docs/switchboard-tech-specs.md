# Switchboard - Design Specification

## 1. Overview

### 1.1 Purpose
Switchboard is a real-time communication component that facilitates structured communication between students and instructors within session-based contexts. Switchboard supports various communication patterns including broadcasts, direct messaging, and analytics data collection.

### 1.2 Design Principles
- **Simplicity**: Minimal architecture focused solely on message routing
- **Session-Centric**: All communication occurs within defined educational sessions
- **Real-Time**: WebSocket-based for immediate message delivery
- **Persistence**: All messages stored for history replay and audit
- **Go Concurrency**: Leverages goroutines and channels for scalable concurrent processing
- **Immutable Sessions**: Sessions cannot be modified after creation for predictable behavior
- **Opinionated Decisions**: Simple, predictable behavior over flexibility

### 1.3 Non-Goals
- User authentication (handled by client applications)
- User authorization beyond session membership validation
- Complex error recovery mechanisms
- Offline message queuing
- Cross-session communication
- Session modification after creation
- Content validation or sanitization

### 1.4 Role Determination

The switchboard determines user roles through WebSocket connection context:

**Connection Process:**
1. Client connects with role declared in URL: `ws://host/ws?user_id=user123&role=instructor&session_id=abc123`
2. Switchboard validates role against session membership:
   - **Students**: Must be in session's `student_ids` list
   - **Instructors**: Universal access to all sessions
3. Role is stored in client connection context and used for all future message routing
4. Authentication and role assignment are handled by upstream components

**Security Model:**
- Switchboard trusts the declared role from authenticated clients
- Role tampering prevention occurs at the authentication layer
- Session membership validation provides access control

## 2. Architecture Overview

### 2.1 High-Level Components

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Client Apps   │───▶│   WebSocket Hub  │───▶│  Session Mgmt   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌──────────────────┐    ┌─────────────────┐
                       │ Message Router   │───▶│   DB Manager    │
                       └──────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌──────────────────┐
                       │   SQLite Store   │
                       └──────────────────┘
```

### 2.2 Core Components

**WebSocket Hub**
- Manages client connections and disconnections
- Handles connection validation and cleanup
- Coordinates message flow between components
- Maintains connection maps for dynamic routing

**Session Manager**
- Loads active sessions at startup
- Manages session lifecycle (create, end)
- Validates session membership
- Enforces session immutability

**Message Router**
- Routes messages based on type and sender role
- Calculates dynamic recipient lists using connection maps
- Applies role-based permissions

**Connection Manager**
- Tracks active connections across sessions
- Handles connection replacement and cleanup
- Provides efficient lookup for message routing

**DB Manager**
- Single-threaded database operations via goroutine
- Handles all message persistence
- Manages session storage and retrieval

## 3. Data Models

### 3.1 Session
```
Session {
  id: string (UUID)
  name: string (1-200 characters)
  created_by: string (instructor_id)
  student_ids: []string (fixed list)
  start_time: timestamp (server-generated)
  end_time: timestamp (null while active)
  status: "active" | "ended"
}
```

### 3.2 Message
```
Message {
  id: string (UUID, server-generated)
  session_id: string
  type: string (message type)
  context: string (default "general", 1-50 chars, client-defined semantics)
  from_user: string
  to_user: string (null for broadcasts)
  content: map[string]interface{} (JSON, max 64KB)
  timestamp: timestamp (server-generated)
}
```

### 3.3 Client
```
Client {
  id: string (user_id, 1-50 chars, alphanumeric + underscore/hyphen)
  role: "student" | "instructor"
  session_id: string
  connection: WebSocket
  send_channel: chan Message
  last_heartbeat: timestamp
  messageCount: int (for rate limiting)
  windowStart: timestamp (for rate limiting)
  cleaned_up: boolean (prevent double cleanup)
}
```

### 3.4 Connection Maps
```
ConnectionManager {
  globalConnections: map[string]*Client           // userID -> Client
  sessionInstructors: map[string]map[string]*Client // sessionID -> instructorID -> Client  
  sessionStudents: map[string]map[string]*Client   // sessionID -> studentID -> Client
}
```

## 4. Communication Types

### 4.1 Message Types

| Type | From | To | Description | Context Examples |
|------|------|----|-----------|-----------------| 
| `instructor_inbox` | Student | All Instructors | Student question/message | `"question"`, `"help_request"`, `"clarification"`, `"technical_issue"` |
| `inbox_response` | Instructor | Specific Student | Response to student message | `"answer"`, `"guidance"`, `"follow_up"` |
| `request` | Instructor | Specific Student | Request for information | `"code"`, `"execution_output"`, `"explanation"`, `"screenshot"` |
| `request_response` | Student | All Instructors | Response to instructor request | `"code_submission"`, `"output_results"`, `"explanation"` |
| `analytics` | Student | All Instructors | Analytics/activity data | `"engagement"`, `"progress"`, `"performance"`, `"errors"` |
| `instructor_broadcast` | Instructor | All Students | Announcement/instruction | `"announcement"`, `"instruction"`, `"emergency"` |

**Note**: Context field provides semantic categorization within each message type. Default context is `"general"` for all types. Clients define context semantics based on their needs.

### 4.2 Channel Availability
All message types are available in every session. Clients choose which communication patterns to use based on their needs. No server-side channel restrictions or configuration required.

## 5. Core Algorithms

### 5.1 Message Routing Algorithm

```
Function RouteMessage(message):
  1. Generate new UUID for message.id (ignore any client-provided ID)
  2. Set message.timestamp = current_server_time
  3. Set message.from_user = sender_client.id
  4. Set message.session_id = sender_client.session_id
  5. Set message.context = provided_context OR "general" (if empty/missing)
  6. Send message to DB persistence channel and WAIT for confirmation
  7. If DB write fails: discard message, log error, return
  8. Determine routing pattern based on message type:
     
     Case "instructor_inbox", "request_response", "analytics":
       recipients = sessionInstructors[message.session_id]
     
     Case "inbox_response", "request":
       recipient = sessionStudents[message.session_id][message.to_user]
       if recipient == null: discard message and log error
     
     Case "instructor_broadcast":
       recipients = sessionStudents[message.session_id]
  
  9. For each recipient in recipients:
       Send message to recipient.send_channel (non-blocking)
```

### 5.2 Session Management Algorithm

```
Function CreateSession(instructor_id, session_data):
  1. Validate session_data:
     - Remove duplicate student_ids
     - Ensure required fields present
  2. Begin database transaction
  3. Generate unique session_id (UUID)
  4. Insert session record to database
  5. Commit transaction
  6. Add session to in-memory session_map
  7. Return session_id

Function EndSession(session_id):
  1. Get all clients in session from connection maps
  2. For each client: call CleanupClient(client)
  3. Update session.end_time in database
  4. Update session.status to "ended"
  5. Remove session from in-memory session_map
```

### 5.3 Client Connection Algorithm

```
Function HandleClientConnection(websocket, user_id, role, session_id):
  1. Validate input parameters (format, length)
  2. Validate session exists and status == "active"
  3. Validate role assignment:
       If role == "student":
         Validate user_id in session.student_ids
       If role == "instructor":
         Allow connection (instructors have universal session access)
  4. If user already connected to this session:
       Send old connection cleanup to Hub via unregister channel
       Wait for Hub to process cleanup
  5. Create new client object with validated role
  6. Send new connection to Hub via register channel
  7. Hub processes registration and updates connection maps atomically
  8. Send all historical messages for session to client
  9. Start read/write goroutines for client
  10. Start heartbeat monitoring

Note: The switchboard trusts the client's declared role since authentication 
is handled by upstream components. Role validation occurs only against 
session membership rules.
```

### 5.4 History Replay Algorithm

```
Function SendHistoryToClient(client):
  1. Query all messages for client.session_id ordered by timestamp
  2. If query fails: 
       Log error
       Send "history_unavailable" message to client
       Continue with live connection
  3. For each message:
       If client.role == "instructor": send message
       If client.role == "student": 
         If message involves client.id (from_user, to_user, or broadcast): send message
  4. Send "history_complete" notification to client
```

### 5.5 Connection Cleanup Algorithm

```
Function CleanupClient(client):
  // Idempotent - safe to call multiple times
  1. If client already cleaned up: return
  2. Mark client as cleaned up
  3. Remove from globalConnections[client.id] (if exists)
  4. Remove from session role maps (if exists)
  5. Close client.send_channel (if not already closed)
  6. Close WebSocket connection
  7. Log disconnection event

Function ScanStaleConnections() (background safety net):
  1. For each connection in globalConnections:
       If current_time - client.last_heartbeat > 120 seconds:
         Log "stale connection detected"
         Call CleanupClient(connection)
```

### 5.6 Message Validation Algorithm

```
Function ValidateMessage(message, sender_client):
  1. Check if new minute window needed:
       If current_time - sender_client.windowStart >= 60 seconds:
         sender_client.messageCount = 0
         sender_client.windowStart = current_time
  
  2. Increment sender_client.messageCount
  
  3. If sender_client.messageCount > 100:
       Discard message, log rate limit violation
       Return false
  
  4. Validate message type is one of the 6 valid types
  5. Validate context field (1-50 characters, alphanumeric + underscore/hyphen)
  6. Validate sender role can send this message type using stored role from connection:
     - Students: instructor_inbox, request_response, analytics
     - Instructors: inbox_response, request, instructor_broadcast
  7. Check message content size <= 64KB
  8. For direct messages: validate to_user exists in session
  9. If any validation fails: discard message and log

Note: Sender role is determined from the stored role in the client's 
connection context, established during WebSocket connection setup.
```

## 6. Concurrency Model

### 6.1 Goroutine Architecture

**Main Hub Goroutine**
- Processes client registration/deregistration
- Routes messages between clients
- Updates connection maps
- Single point of coordination

**DB Manager Goroutine** 
- Handles all database write operations
- Prevents SQLite write contention
- Processes writes via channel

**Per-Client Goroutines**
- Read Pump: Reads messages from WebSocket
- Write Pump: Writes messages to WebSocket
- Manages connection lifecycle and heartbeat

**Health Monitor Goroutine**
- Collects basic system metrics
- Provides status for health endpoint

### 6.2 Channel Communication

```
hub.register: chan *Client
hub.unregister: chan *Client  
hub.broadcast: chan Message
db.write: chan DBOperation
health.metrics: chan HealthMetric
```

### 6.3 Essential Limits

- **Message rate limit**: 100 messages per minute per client (prevents spam)
- **Maximum message size**: 64KB (prevents abuse)
- **User ID length**: 1-50 characters (reasonable identifier constraints)
- **Session name length**: 1-200 characters (UI/UX consideration)

## 7. Database Design

### 7.1 Schema

```sql
-- Sessions table
CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_by TEXT NOT NULL,
  student_ids TEXT NOT NULL, -- JSON array
  start_time DATETIME NOT NULL,
  end_time DATETIME,
  status TEXT NOT NULL DEFAULT 'active',
  CHECK (status IN ('active', 'ended'))
);

-- Messages table
CREATE TABLE messages (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  type TEXT NOT NULL,
  context TEXT NOT NULL DEFAULT 'general',
  from_user TEXT NOT NULL,
  to_user TEXT, -- NULL for broadcasts
  content TEXT NOT NULL, -- JSON, max 64KB
  timestamp DATETIME NOT NULL,
  FOREIGN KEY (session_id) REFERENCES sessions(id),
  CHECK (type IN ('instructor_inbox', 'inbox_response', 'request', 'request_response', 'analytics', 'instructor_broadcast')),
  CHECK (length(context) >= 1 AND length(context) <= 50)
);

-- Indexes for performance
CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_messages_session_time ON messages(session_id, timestamp);
```

### 7.2 Concurrency Strategy
- **Writes**: Single goroutine via channel to prevent contention
- **Reads**: Concurrent access (SQLite handles read concurrency well)
- **Transactions**: Used for atomic session creation/updates
- **Connection pooling**: Single connection with proper locking

**Database Error Recovery Algorithm**:
```
DB Manager Goroutine:
  1. Process write operations from channel
  2. If write fails:
       Log error with details
       Retry operation once after 5 seconds
       If retry fails: log critical error, continue processing
  3. Continue processing subsequent operations normally
```

### 7.3 Startup Recovery
- Load all sessions with status='active' into memory
- Reconcile any inconsistencies (sessions without end_time but server was down)
- Initialize connection maps as empty (clients must reconnect)

## 8. API Endpoints

### 8.1 Session Management

**Create Session**
```
POST /api/sessions
Content-Type: application/json

Request Body:
{
  "name": "Math Class - Chapter 5",
  "student_ids": ["student1", "student2", "student3"]
}

Response: 201 Created
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "active",
  "created_at": "2025-07-23T14:30:00Z"
}

Errors:
400 Bad Request - Invalid input data, duplicate student IDs removed automatically
500 Internal Server Error - Database error
```

**End Session**
```
DELETE /api/sessions/{session_id}

Response: 200 OK
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "ended",
  "ended_at": "2025-07-23T16:30:00Z"
}

Errors:
404 Not Found - Session doesn't exist
400 Bad Request - Session already ended
500 Internal Server Error - Database error

Note: Any instructor can end any session
```

**Get Session Details**
```
GET /api/sessions/{session_id}

Response: 200 OK
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Math Class - Chapter 5",
  "created_by": "instructor1",
  "student_ids": ["student1", "student2", "student3"],
  "status": "active",
  "created_at": "2025-07-23T14:30:00Z",
  "ended_at": null
}

Errors:
404 Not Found - Session doesn't exist
```

**List Active Sessions**
```
GET /api/sessions

Response: 200 OK
{
  "sessions": [
    {
      "session_id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Math Class - Chapter 5",
      "created_by": "instructor1", 
      "status": "active",
      "created_at": "2025-07-23T14:30:00Z",
      "connected_clients": 15
    }
  ],
  "total_count": 1
}
```

### 8.2 Health & Monitoring

**System Health Check**
```
GET /health

Response: 200 OK
{
  "status": "healthy",
  "timestamp": "2025-07-23T16:45:00Z",
  "uptime_seconds": 3600,
  "active_sessions": 5,
  "total_connections": 127,
  "database_status": "connected"
}

Errors:
503 Service Unavailable - Database connection failed or other critical error
```

### 8.3 WebSocket Connection

**WebSocket Endpoint**
```
WebSocket URL: ws://localhost:8080/ws

Query Parameters (Required):
- user_id: string (1-50 chars, alphanumeric + underscore/hyphen)
- role: "student" | "instructor"  
- session_id: string (UUID)

Example:
ws://localhost:8080/ws?user_id=student123&role=student&session_id=550e8400-e29b-41d4-a716-446655440000

Connection Process:
1. Validate query parameters
2. Validate session exists and is active
3. Validate user permissions (students must be in session's student_ids)
4. Close any existing connection for same user_id + session_id
5. Send complete message history for session
6. Begin real-time message routing

Connection Errors:
- 400 Bad Request: Missing/invalid query parameters
- 403 Forbidden: Student not in session's student list
- 404 Not Found: Session doesn't exist or is ended

Heartbeat Protocol:
- Client sends WebSocket ping every 30 seconds  
- Server responds with pong and updates client.last_heartbeat
- Server detects disconnections via WebSocket errors (primary)
- Background scanner cleans stale connections after 120 seconds (safety net)
- Connection cleanup coordinated through Hub goroutine
```

### 8.4 WebSocket Message Format

**Incoming Message (Client to Server)**
```json
{
  "type": "request",
  "context": "code",
  "to_user": "student123",
  "content": {
    "text": "Please share your solution for problem 3",
    "requirements": ["include comments", "show algorithm"]
  }
}

Note: Server generates message ID and timestamp
Context defaults to "general" if not provided
```

**Outgoing Message (Server to Client)**
```json
{
  "id": "msg-uuid",
  "type": "request",
  "context": "code",
  "from_user": "instructor1",
  "to_user": "student123",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "content": {
    "text": "Please share your solution for problem 3",
    "requirements": ["include comments", "show algorithm"]
  },
  "timestamp": "2025-07-23T16:45:30Z"
}
```

**System Messages**
```json
{
  "type": "system",
  "content": {
    "event": "session_ended",
    "message": "Session has been ended by instructor"
  },
  "timestamp": "2025-07-23T16:45:30Z"
}
```

## 9. Error Handling & Validation

### 9.1 Input Validation Rules

**User ID**: 1-50 characters, alphanumeric + underscore/hyphen only
**Session Name**: 1-200 characters, any printable characters
**Student IDs**: Duplicates automatically removed
**Context**: 1-50 characters, alphanumeric + underscore/hyphen, defaults to "general"
**Message Content**: Valid JSON, max 64KB

### 9.2 Connection Error Handling

**Invalid Session**: Close connection immediately with error message
**Unauthorized User**: Close connection with 403 error
**Duplicate Connection**: Close previous connection, accept new one
**WebSocket Timeout**: Automatic cleanup and removal from all maps
**Rate Limit Exceeded**: Drop excess messages, log warning, continue connection

### 9.3 Message Error Handling

**Invalid Message Type**: Discard message, log error
**Non-existent Recipient**: Discard message, log error
**Invalid JSON Content**: Discard message, log parsing error
**Content Too Large**: Discard message, log size error
**Session Ended**: Discard message, close connection

### 9.4 Database Error Handling

**Write Failure**: Log error, retry once, continue operation (message routing proceeds)
**Read Failure**: Return empty history, log error, continue connection
**Connection Loss**: Graceful degradation, retry connection every 30 seconds
**Transaction Failure**: Rollback, return error to client

### 9.5 Session Error Handling

**Session Creation Failure**: Return 500 error to client
**Cleanup Failure**: Log error, continue operation

## 10. Business Rules & Constraints

### 10.1 Session Rules

- **Immutable after creation**: No modifications to student_ids after session creation
- **Instructor privileges**: Any instructor can end any session
- **Manual termination**: Sessions can only be ended manually via API
- **All channels available**: All 6 message types available in every session

### 10.2 Connection Rules

- **Unique connections**: One connection per user_id per session_id
- **Connection replacement**: New connection immediately replaces old one
- **Student validation**: Students must be in session's student_ids list
- **Instructor access**: Instructors can join any active session
- **Immediate cleanup**: Disconnections trigger immediate resource cleanup

### 10.3 Message Rules

- **Server-generated IDs**: All message IDs generated by server (clients cannot provide IDs)
- **Message ordering**: Messages ordered by server timestamp within sessions
- **Rate limiting**: 100 messages per minute per client connection
- **Content limits**: Maximum 64KB per message
- **Role validation**: Message types must match sender's role permissions
- **All types available**: All 6 message types available in every session

### 10.4 Persistence Rules

- **All messages persisted**: No message filtering for storage
- **Complete history**: All historical messages sent to new connections (with role-based filtering)
- **No retention policy**: Messages stored indefinitely (operational concern)
- **Atomic operations**: Session creation/deletion uses database transactions
- **Persist-then-route**: Messages persisted before routing for consistency

## 11. Performance Considerations

### 11.1 Design for Classroom Scale
- **Target**: Typical classroom sizes (20-50 concurrent users per session)
- **Approach**: Measure and optimize based on actual usage patterns
- **Scalability**: Single-server design suitable for individual schools/departments

### 11.2 Resource Management
- **Message size limit**: 64KB (prevents abuse)
- **Rate limiting**: 100 messages per minute per connection (prevents spam)
- **Immediate cleanup**: Prevent resource leaks through prompt disconnection handling
- **Efficient routing**: O(1) lookup using connection maps
- **Single DB writer**: Eliminates write contention

### 11.3 Monitoring & Optimization
- **Track actual metrics**: Use /health endpoint for operational visibility
- **Monitor resource usage**: Adjust configurations based on observed patterns
- **Iterative improvement**: Optimize based on real-world usage data

## 12. Security Considerations

### 12.1 Input Validation & Sanitization

- **Parameter Validation**: Strict format checking for all inputs
- **Content Size Limits**: 64KB maximum message content
- **Rate Limiting**: 100 messages per minute per connection
- **JSON Validation**: Proper parsing with error handling
- **Session Membership**: Strict validation against student_ids list

### 12.2 Resource Protection

- **Message size limits**: 64KB maximum prevents abuse
- **Rate limiting**: 100 messages per minute prevents spam
- **Automatic cleanup**: Prevents resource leaks
- **Session isolation**: Individual failures don't affect other sessions
- **Input validation**: Strict format checking for all inputs

### 12.3 Data Privacy & Audit

- **Message Persistence**: Complete audit trail of all communications
- **Session Isolation**: No cross-session data leakage
- **Content Opacity**: Switchboard treats message content as opaque JSON
- **Role-Based Access**: Students see only relevant messages, instructors see all
- **Logging**: Security events logged for monitoring
- **No Encryption**: Transport-level security handled externally

This design specification provides a complete blueprint for implementing the educational communication switchboard with all business rules, technical constraints, and operational requirements clearly defined.