# Switchboard - V4 Design Specification (Ultra-Simplified Architecture)

## 1. Overview

### 1.1 Purpose
Switchboard V4 is a real-time educational communication system with an **ultra-simplified architecture** based on a **single global session state**. It eliminates all complex session management while preserving essential educational communication patterns through **3 message types** and **4 core behavioral rules**.

### 1.2 Design Principles
- **Single Global Session**: One active session at a time with simple on/off state
- **Pre-Connection Support**: Users can connect before sessions start (waiting state)
- **Message Gating**: No messages accepted without active session
- **Complete History**: Late joiners receive full session history (role-filtered)
- **3-Message Simplicity**: broadcast_to_instructors, direct_message, broadcast_to_students
- **Educational Privacy**: Role-based filtering preserves student privacy
- **Concurrency Safety**: Thread-safe operations with idiomatic Go patterns (RWMutex, atomic operations)

### 1.3 Core Behavioral Rules
1. **No messages without active session** - Clean boundary enforcement
2. **Users can connect before session starts** - Pre-session waiting supported  
3. **Messages broadcast to all connected users** - Role-filtered real-time delivery
4. **Late joiners get complete session history** - Role-filtered historical context

### 1.4 Production-Ready Features
- **Database Retry Logic**: Exponential backoff with dead letter queue prevents message loss
- **Session-Aware Connection Management**: Connections never timeout during active sessions, 25-minute timeout during waiting periods
- **Graceful Shutdown**: Multi-phase shutdown ensures data integrity and clean resource cleanup
- **Heartbeat Monitoring**: WebSocket ping/pong with application-level heartbeat tracking
- **Error Recovery**: Comprehensive error handling for database failures and network issues

## 2. System Configuration

### 2.1 Core Constants
```go
const (
    // Connection Management
    ConnectionSendBufferSize = 100             // Buffered channel per connection
    MaxMessageSize          = 64 * 1024       // 64KB message limit
    HeartbeatInterval       = 30 * time.Second
    InactiveConnectionTimeout = 25 * time.Minute  // Cleanup inactive connections when no session
    ConnectionCleanupInterval = 30 * time.Second
    
    // Database Performance
    DatabaseBatchSize       = 100             // Messages per batch
    DatabaseFlushInterval   = 200 * time.Millisecond
    DatabaseWriteBuffer     = 100             // Write channel buffer
    DatabaseMaxRetries      = 3               // Retry attempts
    DeadLetterQueueSize     = 50              // Failed write buffer
    
    // Rate Limiting
    RateLimitMaxMessages    = 100             // Per user per window
    RateLimitWindow         = time.Minute     // Rate limit window
    RateLimitCleanupInterval = 5 * time.Minute
    
    // Graceful Shutdown Timeouts
    MessageProcessingTimeout = 2 * time.Second
    GoroutineCleanupTimeout = 1 * time.Second
)

// Database Configuration
const SQLiteConfiguration = `
PRAGMA journal_mode = WAL;          -- Enable concurrent reads
PRAGMA synchronous = NORMAL;        -- Balance durability/performance  
PRAGMA cache_size = -64000;         -- 64MB cache
PRAGMA wal_autocheckpoint = 1000;   -- Auto-checkpoint every 1000 pages
`
```

## 3. Connection Lifecycle Management

### 3.1 Session-Aware Connection Cleanup

The system implements **educationally-appropriate connection management** that aligns with classroom usage patterns:

#### **Active Session State**
- **No Connection Timeouts**: During active learning sessions, connections remain alive indefinitely
- **Heartbeat Monitoring**: Only dead connections (failed WebSocket ping/pong) are removed
- **Educational Rationale**: Students should never be disconnected mid-lesson due to inactivity

#### **No Active Session State** 
- **25-Minute Timeout**: Connections inactive for >25 minutes are automatically cleaned up
- **Resource Management**: Prevents memory leaks from abandoned connections between classes
- **Grace Period**: Allows for reasonable breaks, bathroom visits, and transition time

#### **Implementation Details**
```go
// Session-aware cleanup logic
func (cr *ConnectionRegistry) cleanupStaleConnections() {
    if cr.sessionManager.HasActiveSession() {
        // During active session: only remove dead connections
        cr.cleanupDeadConnections()
        return
    }
    
    // During waiting period: apply 25-minute timeout
    cutoff := time.Now().Add(-config.InactiveConnectionTimeout)
    cr.cleanupInactiveConnections(cutoff)
}
```

### 3.2 WebSocket Protocol Ping/Pong Requirements

Switchboard uses **WebSocket protocol-level ping/pong frames** (RFC 6455) for connection health monitoring:

#### **Server Behavior**
- Server sends WebSocket PING control frames every 30 seconds
- Empty payload: `[]byte{}`
- Write deadline: 5 seconds
- Connections that fail to respond with PONG are terminated

#### **Client Requirements - Automatic Support**
All major WebSocket libraries handle ping/pong automatically:

**JavaScript (Browser)**
```javascript
const ws = new WebSocket('ws://localhost:8080/ws?user_id=student1&role=student');
// Browser automatically responds to ping frames with pong frames
// No application code required - handled at protocol level
```

**Python (websockets library)**
```python
import asyncio
import websockets

async with websockets.connect("ws://localhost:8080/ws") as ws:
    # Library automatically handles ping/pong
    # Default: responds to pings, sends keepalive pings every 20s
    async for message in ws:
        print(message)
```

**Go (Gorilla WebSocket)**
```go
conn, _, err := websocket.DefaultDialer.Dial(url, nil)
// Default ping handler automatically responds with pong
// Application must read connection to process control frames

go func() {
    for {
        _, _, err := conn.ReadMessage()
        if err != nil {
            break // Connection closed or error
        }
    }
}()
```

#### **Protocol Specification**
- **Ping Frame**: WebSocket control frame (opcode 0x9)
- **Pong Frame**: WebSocket control frame (opcode 0xA) 
- **Automatic Response**: Client libraries MUST respond to ping with pong
- **Timeout**: Server waits maximum 30 seconds for pong response
- **Connection Termination**: No pong response = dead connection cleanup

### 3.3 Connection State Transitions

```
Client Connect → Registry Registration → Heartbeat Tracking
     ↓
Active Session?
  ├─ YES → Connection Persists (no timeout)
  └─ NO  → 25-minute inactivity timeout
     ↓
Heartbeat Failed OR Timeout Exceeded → Connection Cleanup
```

## 4. Architecture Overview

### 3.1 System State Model

```go
type Switchboard struct {
    // Core state with separate mutexes for optimal performance
    activeSessionMu  sync.RWMutex
    activeSession    *Session
    connectionsMu    sync.RWMutex
    connections      map[string]*Connection
    
    // Unified database management
    dbManager        DatabaseManager
    rateLimiter      *RateLimiter
    
    // Lifecycle management
    ctx              context.Context
    cancel           context.CancelFunc
    wg               sync.WaitGroup
    messageProcessing sync.WaitGroup
}

type Session struct {
    ID        string    `json:"id" db:"id"`
    Name      string    `json:"name" db:"name"`
    CreatedBy string    `json:"created_by" db:"created_by"`
    StartTime time.Time `json:"start_time" db:"start_time"`
    EndTime   *time.Time `json:"end_time,omitempty" db:"end_time"`
    Status    string    `json:"status" db:"status"`
}

type Connection struct {
    UserID      string
    Role        string    // "student" or "instructor"
    WebSocket   *websocket.Conn
    SendChannel chan []byte  // Buffered channel for writes
    LastSeen    time.Time
}
```

### 3.2 Unified Database Management Interface

```go
// Single interface for all database operations
type DatabaseManager interface {
    // Session operations
    CreateSession(session *Session) error
    UpdateSession(session *Session) error
    GetActiveSession() (*Session, error)
    
    // Message operations
    WriteMessage(msg *Message) error
    WriteBatch(msgs []*Message) error
    GetSessionMessages(sessionID string) ([]*Message, error)
    
    // Lifecycle
    Start() error
    Stop() error
}

// Single implementation with internal batching and retry logic
type SQLiteDatabaseManager struct {
    db              *sql.DB
    writeChannel    chan writeRequest
    batcher         *messageBatcher
    deadLetterQueue chan writeRequest
    metrics         *DatabaseMetrics
    stopCh          chan struct{}
}
```

### 3.3 Message Flow Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Connected     │───▶│   Message Gate   │───▶│   Broadcaster   │
│   Users         │    │ (Active Session  │    │ (Role Filtered) │
│   (Waiting)     │    │     Check)       │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                       ┌──────────────────┐    ┌─────────────────┐
                       │ Database Manager │───▶│   SQLite with   │
                       │ (Batched Writes) │    │   WAL + Retry   │
                       └──────────────────┘    └─────────────────┘
```

## 4. Core Algorithms

### 4.1 Standardized Session State Access

**Interface Contract (Keep Exact):**
```go
// Consistent session state interface
type SessionManager interface {
    GetActiveSession() *Session
    SetActiveSession(session *Session) error
    ClearActiveSession() error
    HasActiveSession() bool
}
```

**Implementation Pattern (Pseudocode):**
```
Function GetActiveSession():
  1. Acquire read lock on activeSessionMu
  2. Copy session pointer atomically
  3. Release read lock
  4. Return session pointer (may be nil)

Function SetActiveSession(session):
  1. Acquire write lock on activeSessionMu
  2. If activeSession is not nil: return "session already active" error
  3. Set activeSession = session
  4. Release write lock
  5. Return success

Function ClearActiveSession():
  1. Acquire write lock on activeSessionMu
  2. If activeSession is nil: return "no active session" error
  3. Set activeSession = nil
  4. Release write lock
  5. Return success

Function HasActiveSession():
  1. Acquire read lock on activeSessionMu
  2. Check if activeSession is not nil
  3. Release read lock
  4. Return boolean result
```

### 4.2 Connection Lifecycle Management

```go
func (s *Switchboard) HandleClientConnection(ws *websocket.Conn, userID, role string) error {
    // Input validation
    if err := s.validateConnection(userID, role); err != nil {
        return err
    }
    
    // Create connection
    connection := &Connection{
        UserID:      userID,
        Role:        role,
        WebSocket:   ws,
        SendChannel: make(chan []byte, ConnectionSendBufferSize),
        LastSeen:    time.Now(),
    }
    
    // Atomic connection registration
    s.registerConnection(userID, connection)
    
    // Handle session state consistently
    session := s.GetActiveSession() // Use standardized access
    if session == nil {
        s.sendWaitingMessage(connection)
    } else {
        s.sendActiveSessionMessage(connection, session)
        s.sendSessionHistory(connection, session)
    }
    
    // Start connection goroutines
    s.wg.Add(2)
    go s.connectionReadLoop(connection)
    go s.connectionWriteLoop(connection)
    
    s.broadcastPresenceUpdate(userID, "connected")
    return nil
}

**Connection Cleanup Algorithm:**
```
Function startConnectionCleanup():
  Start background goroutine connectionCleanupLoop()

Function connectionCleanupLoop():
  Create ticker for ConnectionCleanupInterval (30 seconds)
  
  Loop:
    Wait for:
      - Ticker event: call cleanupStaleConnections()
      - Context cancellation: exit loop

Function cleanupStaleConnections():
  1. Lock connectionsMu
  2. Calculate cutoff time = now - ConnectionTimeout
  3. For each connection in connections map:
     - If connection.LastSeen < cutoff:
       - Close WebSocket connection
       - Close SendChannel to signal goroutines
       - Remove from connections map
       - Log cleanup action
  4. Unlock connectionsMu
  5. Log total cleanup count if any removed

Function updateConnectionActivity(userID):
  1. Read-lock connectionsMu
  2. Get connection for userID
  3. Unlock connectionsMu
  4. If connection exists: update LastSeen to current time
```

**Connection Goroutine Patterns:**
```
Function connectionReadLoop(connection):
  Setup cleanup on exit:
    - Remove connection from map
    - Close SendChannel
  
  Loop:
    1. Read message from WebSocket
    2. If read error: log and exit (triggers cleanup)
    3. Update connection activity timestamp
    4. Process incoming message

Function connectionWriteLoop(connection):
  Setup heartbeat ticker (HeartbeatInterval)
  
  Loop:
    Wait for:
      - Message from SendChannel:
        - Write to WebSocket
        - If write error: exit loop
        - Update activity timestamp
      - Heartbeat ticker:
        - Send ping message
        - If ping fails: exit loop
        - Update activity timestamp
      - SendChannel closed:
        - Exit loop cleanly
```
```

### 4.3 Message Processing Algorithm

```
Function ProcessIncomingMessage(rawData, senderID):
  1. Atomic session state capture:
     - session = GetActiveSession() // Use standardized interface
     - If session == nil: return "no_active_session" error
  
  2. Parse and validate message:
     - Parse rawData as JSON into Message struct
     - If parse error: return "invalid_message" error
     - Validate required fields and constraints
  
  3. Prepare message with session context:
     - Generate unique message ID
     - Set current timestamp
     - Set fromUser = senderID
     - Set sessionID = session.ID
     - Set default context if empty
  
  4. Rate limiting check:
     - If rateLimiter.Allow(senderID) == false:
       Return "rate_limit_exceeded" error
  
  5. Route message and determine recipients:
     - Switch on message.Type:
       - "broadcast_to_instructors": get all instructor connections
       - "direct_message": validate toUser, get specific connection
       - "broadcast_to_students": get all student connections
       - Default: return "invalid_message_type" error
  
  6. Real-time broadcast (immediate delivery):
     - For each recipient:
       - Apply role-based filtering
       - Send to recipient's SendChannel (non-blocking)
       - If channel full: log warning, continue
  
  7. Asynchronous persistence:
     - Increment message processing counter
     - Queue message for database write via DatabaseManager
     - Handle persistence errors with logging
  
  8. Return success to sender
```

### 4.4 Session Lifecycle Management

```
Function StartSession(sessionName, instructorID):
  1. Validate input parameters:
     - sessionName: 1-200 characters
     - instructorID: valid format
  
  2. Create new session object:
     - Generate unique session ID
     - Set name, createdBy, startTime
     - Set status = "active"
  
  3. Atomic session creation:
     - Call SetActiveSession(newSession)
     - If session already active: return "session_already_active" error
  
  4. Persist to database:
     - Call dbManager.CreateSession(newSession)
     - If database error:
       - Rollback: call ClearActiveSession()
       - Return "database_error"
  
  5. Notify all connected users:
     - Broadcast system message "session_started" with:
       - session_id, session_name, started_by
       - timestamp
  
  6. Return session details

Function EndSession(instructorID):
  1. Get current session atomically:
     - session = GetActiveSession()
     - If session == nil: return "no_active_session" error
  
  2. Update session state:
     - Set session.status = "ended"
     - Set session.endTime = current time
     - Call ClearActiveSession()
  
  3. Persist session update:
     - Call dbManager.UpdateSession(session)
     - Log any persistence errors (non-blocking)
  
  4. Notify all connected users:
     - Broadcast system message "session_ended" with:
       - session_id, ended_by
       - timestamp
  
  5. Return success
```

### 4.5 Unified Error Handling

**Error Handler Interface (Keep Exact):**
```go
// Standardized error handling interface
type ErrorHandler interface {
    SendWebSocketError(userID, errorType, message string) error
    LogInternalError(operation string, err error)
    FormatClientError(errorType, message string) []byte
}

// Standard Go error types for internal operations
var (
    ErrNoActiveSession      = errors.New("no active session")
    ErrSessionAlreadyActive = errors.New("session already active")
    ErrConnectionNotFound   = errors.New("connection not found")
    ErrChannelFull         = errors.New("send channel full")
    ErrInvalidMessageType  = errors.New("invalid message type")
    ErrRateLimitExceeded   = errors.New("rate limit exceeded")
)
```

**Error Handling Patterns:**
```
Function sendError(userID, errorType, message):
  1. Create error response structure:
     - type: "error"
     - error: errorType
     - message: message
     - timestamp: current time
  
  2. Marshal to JSON format
  
  3. Look up connection:
     - Read-lock connectionsMu
     - Get connection for userID
     - Unlock connectionsMu
  
  4. Send error to client:
     - If connection exists:
       - Try send to SendChannel (non-blocking)
       - If channel full: return "channel full" error
       - If send successful: return success
     - If connection not found: return "connection not found" error

Function logInternalError(operation, error):
  Log error with operation context for debugging
  Do not expose internal details to clients

Function formatClientError(errorType, message):
  Format error as JSON structure suitable for WebSocket clients
  Ensure consistent error format across all client communications
```

## 5. Data Models

### 5.1 Core Data Structures

```go
// Message - 3-type message structure
type Message struct {
    ID        string                 `json:"id" db:"id"`
    SessionID string                 `json:"session_id" db:"session_id"`
    Type      string                 `json:"type" db:"type"`          // 3 types only
    Context   string                 `json:"context" db:"context"`    // Semantic meaning
    FromUser  string                 `json:"from_user" db:"from_user"`
    ToUser    *string                `json:"to_user,omitempty" db:"to_user"`
    Content   map[string]interface{} `json:"content" db:"content"`
    Timestamp time.Time              `json:"timestamp" db:"timestamp"`
}

// Message type constants
const (
    MessageTypeBroadcastToInstructors = "broadcast_to_instructors"
    MessageTypeDirectMessage         = "direct_message"
    MessageTypeBroadcastToStudents   = "broadcast_to_students"
    MessageTypeSystem                = "system"
)

// Context constants for semantic meaning
const (
    ContextQuestion      = "question"
    ContextSubmission    = "submission"
    ContextAnalytics     = "analytics"
    ContextResponse      = "response"
    ContextRequest       = "request"
    ContextPeerHelp      = "peer_help"
    ContextAnnouncement  = "announcement"
    ContextInstruction   = "instruction"
    ContextEmergency     = "emergency"
    ContextGeneral       = "general"
)
```

## 6. Database Design

### 6.1 Optimized Schema

```sql
-- Ultra-simple sessions table
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_by TEXT NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    status TEXT NOT NULL DEFAULT 'active',
    CHECK (status IN ('active', 'ended')),
    CHECK (length(name) >= 1 AND length(name) <= 200)
);

-- 3-type messages table
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    type TEXT NOT NULL,
    context TEXT NOT NULL DEFAULT 'general',
    from_user TEXT NOT NULL,
    to_user TEXT,
    content TEXT NOT NULL,
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    CHECK (type IN ('broadcast_to_instructors', 'direct_message', 'broadcast_to_students')),
    CHECK (length(context) >= 1 AND length(context) <= 50),
    CHECK (length(from_user) >= 1 AND length(from_user) <= 50),
    CHECK (length(content) <= 65536)  -- 64KB limit
);

-- Performance indexes
CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_sessions_start_time ON sessions(start_time DESC);
CREATE INDEX idx_messages_session_time ON messages(session_id, timestamp);
CREATE INDEX idx_messages_type_context ON messages(type, context);
CREATE INDEX idx_messages_to_user ON messages(to_user) WHERE to_user IS NOT NULL;
```

## 7. Database Implementation

### 7.1 SQLite Database Manager Implementation

**Database Manager Structure (Keep Exact):**
```go
type SQLiteDatabaseManager struct {
    db              *sql.DB
    writeChannel    chan writeRequest    // Buffer: DatabaseWriteBuffer
    deadLetterQueue chan writeRequest    // Buffer: DeadLetterQueueSize
    metrics         *DatabaseMetrics
    stopCh          chan struct{}
    wg              sync.WaitGroup
}

type DatabaseMetrics struct {
    SuccessfulWrites    int64
    FailedWrites        int64
    RetriedWrites       int64
    DeadLetterCount     int64
    PermanentLossCount  int64
}
```

**Database Manager Initialization:**
```
Function NewSQLiteDatabaseManager(database):
  1. Apply SQLite configuration:
     - Execute SQLiteConfiguration pragmas
     - Set WAL mode, cache size, synchronous mode
  
  2. Create manager structure:
     - Set database connection
     - Create buffered channels for write operations
     - Initialize metrics tracking
     - Setup stop channel
  
  3. Create internal message batcher
  4. Return configured manager
```

**Write Processing Algorithm:**
```
Function Start():
  Start two background goroutines:
    1. writeLoop() - Process write requests
    2. deadLetterProcessor() - Handle failed writes

Function writeLoop():
  Loop:
    Wait for:
      - Write request from writeChannel:
        - Execute with retry logic (max DatabaseMaxRetries)
        - If all retries fail:
          - Try to queue in deadLetterQueue
          - If dead letter queue full: log permanent loss
        - If successful: increment success metrics
      - Stop signal: exit loop

Function executeWithRetry(request, maxAttempts):
  Set initial backoff = 100ms
  
  For attempt 1 to maxAttempts:
    1. Try to execute request against database
    2. If successful: return success
    3. If error is not retryable: return error
    4. If more attempts remaining:
       - Sleep for backoff duration
       - Double backoff time (exponential backoff)
       - Increment retry metrics
  
  Return "max retries exceeded" error
```

**Message Batching Algorithm:**
```
Function addMessage(message):
  1. Lock batcher mutex
  2. Add message to current batch
  3. If this is first message in batch:
     - Start flush timer (DatabaseFlushInterval)
  4. If batch reaches DatabaseBatchSize:
     - Flush batch immediately
  5. Unlock mutex

Function flushBatch():
  1. If no messages: return
  2. Copy messages to new batch array
  3. Clear internal message buffer
  4. Create batch write request
  5. Try to send to writeChannel:
     - If successful: batch queued
     - If channel full: log dropped batch warning

Function resetTimer():
  Reset flush timer to DatabaseFlushInterval
  When timer expires: call flushBatch()
```

## 8. WebSocket Protocol Specifications

### 8.1 Message Size and Format Constraints

**Message Size Limits**
- **Maximum Message Size**: 64KB (65,536 bytes) per WebSocket frame
- **Encoding**: UTF-8 JSON text messages only
- **Frame Type**: WebSocket text frames (opcode 0x1)
- **Violation Handling**: Messages exceeding 64KB result in connection termination

**Message Structure Requirements**
```json
{
  "type": "broadcast_to_instructors" | "direct_message" | "broadcast_to_students",
  "context": "question" | "response" | "announcement" | ... ,
  "content": { /* arbitrary JSON object */ },
  "to_user": "optional_recipient_id"  // Required for direct_message only
}
```

### 8.2 Connection Close Protocol

**Graceful Shutdown**
- **Close Code**: 1001 (Going Away)
- **Close Reason**: "Server shutting down"
- **Client Behavior**: Attempt reconnection after receiving close frame

**Connection Termination Scenarios**
- Ping timeout (no pong response within 30 seconds)
- Message size violation (exceeds 64KB)
- Authentication failure during connection establishment
- Rate limit violations (excessive message sending)

### 8.3 Error Message Format

**WebSocket Error Messages**
All errors are sent as JSON messages over the WebSocket connection:

```json
{
  "type": "error",
  "error": "rate_limit_exceeded",
  "message": "Maximum 100 messages per minute exceeded",
  "timestamp": "2025-01-15T14:37:00Z"
}
```

**Standard Error Codes**
- `no_active_session` - Message sent without active session
- `rate_limit_exceeded` - User exceeded 100 messages/minute
- `invalid_message` - Malformed JSON or missing required fields
- `invalid_message_type` - Unknown message type
- `invalid_recipient` - Direct message to non-existent user
- `message_too_large` - Message exceeds 64KB limit

### 8.4 Session History Delivery Protocol

**Late Joiner Message Sequence**
1. **Connection State**: Session active/waiting message
2. **Historical Messages**: All session messages (role-filtered), chronological order
3. **History Complete**: System message indicating end of history

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

**Message Filtering Rules**
- **Students**: See own messages + direct messages involving them + broadcasts to students
- **Instructors**: See all messages without filtering

## 9. HTTP API Specifications

### 9.1 Session Management API

**Start Session**
```http
POST /api/session/start
Content-Type: application/json

{
  "name": "Exercise 3: For Loops",
  "instructor_id": "prof_smith"
}

Response: 201 Created
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

**End Session**
```http
POST /api/session/end
Content-Type: application/json

{
  "instructor_id": "prof_smith"
}

Response: 200 OK
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "ended",  
  "ended_at": "2025-01-15T16:30:00Z"
}
```

### 8.2 WebSocket Connection

**Connection URL**
```
ws://localhost:8080/ws?user_id={userID}&role={role}

Parameters:
- user_id: string (1-50 chars, alphanumeric + underscore/hyphen) [Required]
- role: "student" | "instructor" [Required]
```

**Connection Response Messages**

*No Active Session:*
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

*Active Session (with history):*
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

## 9. Role-Based Message Filtering

```go
func shouldReceiveMessage(message *Message, recipientRole, recipientUserID string) bool {
    // Instructors see all messages
    if recipientRole == "instructor" {
        return true
    }
    
    // Students see filtered messages for privacy
    if recipientRole == "student" {
        switch message.Type {
        case MessageTypeBroadcastToInstructors:
            // Students don't see questions from other students
            return false
            
        case MessageTypeDirectMessage:
            // Students see only messages involving them
            return (message.FromUser == recipientUserID || 
                   (message.ToUser != nil && *message.ToUser == recipientUserID))
            
        case MessageTypeBroadcastToStudents:
            // Students see all broadcasts to students
            return true
            
        default:
            return false
        }
    }
    
    return false
}
```

## 10. Graceful Shutdown Implementation

```
Function Shutdown(shutdownContext):
  Log "Starting graceful shutdown"
  
  Phase 1 - Stop accepting new connections:
    Cancel main system context
    This stops new WebSocket upgrades and connection accepts
  
  Phase 2 - Wait for in-flight messages (with timeout):
    Create message completion channel
    Start goroutine to wait for messageProcessing.Wait()
    
    Wait for first of:
      - All messages processed: continue to next phase
      - MessageProcessingTimeout exceeded: force continue
      - Shutdown context cancelled: force continue
  
  Phase 3 - Close all WebSocket connections:
    Lock connectionsMu
    For each connection:
      - Send WebSocket close message ("Server shutting down")
      - Close WebSocket connection
      - Close SendChannel to signal goroutines
      - Log connection closure
    Clear connections map
    Unlock connectionsMu
  
  Phase 4 - Stop database manager:
    Call dbManager.Stop()
    This flushes pending writes and stops background goroutines
    Log any stop errors (non-blocking)
  
  Phase 5 - Wait for all goroutines (with timeout):
    Create goroutine completion channel
    Start goroutine to wait for main WaitGroup
    
    Wait for first of:
      - All goroutines finished: shutdown complete
      - GoroutineCleanupTimeout exceeded: force exit
      - Shutdown context cancelled: force exit
  
  Log "Graceful shutdown completed"
  Return success

Function RunWithGracefulShutdown():
  Setup signal handling for SIGINT/SIGTERM
  Start server in background goroutine
  
  Wait for shutdown signal
  Create shutdown context with 5-second timeout
  Call Shutdown(shutdownContext)
  
  Return shutdown result
```

## 11. Performance Characteristics

### 11.1 Measured Performance
- **Message routing**: 83.992µs average latency
- **Database batching**: 500+ messages/second capability  
- **Memory usage**: 2.6MB peak for 53 concurrent connections
- **Connection stability**: Zero errors under load testing

### 11.2 Resource Requirements

**Memory:**
- Base system: ~10MB
- Per connection: ~2KB (WebSocket + buffers)
- Message batching: ~100KB per batch
- 50 concurrent users: ~15MB total memory usage

**Database:**
- SQLite adequate for single-classroom deployment
- ~1KB per message storage requirement
- Indexes require ~20% additional storage

### 11.3 Scaling Characteristics

**Target Performance:**
- 50 concurrent students + instructors
- 100+ messages/minute sustained rate
- <100µs message routing latency
- >99% message delivery reliability

## 12. Security and Privacy

### 12.1 Security Model
- **Authentication**: WebSocket connections authenticated at application layer
- **Message Security**: Content treated as opaque JSON with 64KB size limits
- **Rate Limiting**: 100 messages/minute per user prevents spam
- **Transport Security**: WebSocket connections over TLS in production

### 12.2 Educational Privacy Preservation
- **Instructors**: See all messages for educational oversight
- **Students**: See only messages involving them directly
- **Audit Trail**: All messages persisted with complete metadata
- **Immutable Record**: No message deletion or modification

This revised specification eliminates architectural redundancies while maintaining all essential technical content in a clearer, more consistent presentation.