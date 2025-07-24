# Phase 2: WebSocket Infrastructure Layer

## Overview
Implements the WebSocket connection handling system with proper concurrency patterns, authentication, and connection management. Builds on Phase 1 foundations to provide reliable real-time communication infrastructure.

## Step 2.1: Connection Wrapper (Estimated: 2.5h)

### EXACT REQUIREMENTS (do not exceed scope):
- Single-writer pattern: One goroutine per connection reads from writeCh
- Buffered channel size: exactly 100 messages
- Timeout: exactly 5 seconds for WriteJSON operations  
- Context cancellation: Clean shutdown when context.Done()
- Interface compliance: Must implement Connection interface exactly
- Authentication state management: Store user credentials after validation
- Proper cleanup: No goroutine leaks, channel cleanup on close

### ARCHITECTURAL VALIDATION REQUIREMENTS:
**Dependencies** (BLOCKING):
- Only import: pkg/interfaces, pkg/types, standard library, github.com/gorilla/websocket
- No imports from: internal/session, internal/router (circular risk)
- Verify: `go mod graph | grep -E "cycle|circular"` returns empty

**Boundaries** (BLOCKING):
- No business logic in connection wrapper (auth decisions, message routing)
- Clean separation: connection handling vs message processing
- Interface compliance: Exact match to Connection interface from Step 1.2

**Integration Contracts** (BLOCKING):
- Registry can call NewConnection() safely from any goroutine
- Auth handler can call SetCredentials() after token validation
- Message router can call WriteJSON() concurrently without corruption

### FUNCTIONAL VALIDATION REQUIREMENTS:
**Core Behaviors** (BLOCKING):
- WriteJSON() delivers messages in order sent
- Close() stops all goroutines and releases resources within 1 second
- Context cancellation immediately stops write operations
- Authentication state (userID, role, sessionID) persists correctly after setting

**Error Handling** (BLOCKING):
- Timeout after exactly 5 seconds returns ErrWriteTimeout
- Invalid JSON returns ErrInvalidJSON with context
- Closed connection writes return ErrConnectionClosed
- Multiple Close() calls safe (idempotent)

**Integration Contracts** (BLOCKING):
- GetUserID()/GetRole()/GetSessionID() return correct values after SetCredentials()
- IsAuthenticated() reflects actual authentication state
- WriteJSON() from 10 concurrent goroutines works without data corruption

### TECHNICAL VALIDATION REQUIREMENTS:
**Race Detection** (WARNING):
- `go test -race` passes on all tests
- No goroutine leaks after connection close
- Channel operations don't block indefinitely

**Performance** (WARNING):
- Handle 1000+ messages/second per connection
- Memory usage <5KB per connection
- Connection setup/teardown <100ms

### MANDATORY INTERFACE (implement exactly):
```go
package websocket

import (
    "context"
    "sync"
    "time"
    "github.com/gorilla/websocket"
    "github.com/switchboard/pkg/interfaces"
)

// Connection implements the interfaces.Connection interface
type Connection struct {
    conn         *websocket.Conn
    writeCh      chan []byte         // Buffer size: 100
    userID       string              // Set after authentication
    role         string              // Set after authentication  
    sessionID    string              // Set after authentication
    authenticated bool               // Authentication status
    ctx          context.Context     // For cancellation
    cancel       context.CancelFunc  // For cleanup
    closeOnce    sync.Once           // Ensure single close
    mu           sync.RWMutex        // Protect auth fields
}

// NewConnection creates a new WebSocket connection wrapper
func NewConnection(conn *websocket.Conn) *Connection {
    ctx, cancel := context.WithCancel(context.Background())
    c := &Connection{
        conn:      conn,
        writeCh:   make(chan []byte, 100),  // Exactly 100 message buffer
        ctx:       ctx,
        cancel:    cancel,
        authenticated: false,
    }
    
    // Start the single writer goroutine
    go c.writeLoop()
    
    return c
}
```

### CRITICAL PATTERNS (must follow):
```go
// REQUIRED: Single writer goroutine pattern
func (c *Connection) writeLoop() {
    defer close(c.writeCh)
    
    for {
        select {
        case data, ok := <-c.writeCh:
            if !ok {
                return // Channel closed
            }
            
            // Set write deadline for exactly 5 seconds
            c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
            
            if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
                // Log error but continue processing
                return
            }
            
        case <-c.ctx.Done():
            return
        }
    }
}

// WriteJSON implementation with timeout and error handling
func (c *Connection) WriteJSON(v interface{}) error {
    // Check if connection is closed
    select {
    case <-c.ctx.Done():
        return ErrConnectionClosed
    default:
    }
    
    // Marshal to JSON
    data, err := json.Marshal(v)
    if err != nil {
        return ErrInvalidJSON
    }
    
    // Send to write channel with timeout
    select {
    case c.writeCh <- data:
        return nil
    case <-time.After(5 * time.Second):
        return ErrWriteTimeout
    case <-c.ctx.Done():
        return ErrConnectionClosed
    }
}

// Close implementation - idempotent cleanup
func (c *Connection) Close() error {
    var err error
    c.closeOnce.Do(func() {
        // Cancel context to stop goroutines
        c.cancel()
        
        // Close WebSocket connection
        c.conn.Close()
        
        // writeCh will be closed by writeLoop goroutine
    })
    return err
}

// Authentication state management
func (c *Connection) SetCredentials(userID, role, sessionID string) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.userID = userID
    c.role = role
    c.sessionID = sessionID
    c.authenticated = true
    
    return nil
}

func (c *Connection) IsAuthenticated() bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.authenticated
}

func (c *Connection) GetUserID() string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.userID
}

func (c *Connection) GetRole() string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.role
}

func (c *Connection) GetSessionID() string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.sessionID
}
```

### ERROR TYPES (define exactly these):
```go
var (
    ErrConnectionClosed = errors.New("connection closed")
    ErrWriteTimeout     = errors.New("write timeout after 5 seconds")
    ErrInvalidJSON      = errors.New("invalid JSON data")
)
```

### SUCCESS CRITERIA:
**Architectural** (BLOCKING):
- [ ] No circular dependencies: `go mod graph | grep cycle` empty
- [ ] Clean imports: Only allowed packages imported
- [ ] Interface compliance: Implements Connection interface exactly
- [ ] Boundary separation: No business logic in connection layer

**Functional** (BLOCKING):
- [ ] WriteJSON delivers messages correctly and in order
- [ ] Close() cleanup complete within 1 second
- [ ] Authentication state management works correctly
- [ ] All error cases return correct error types

**Technical** (WARNING):
- [ ] `go test -race ./internal/websocket` passes
- [ ] Coverage ≥85% statements
- [ ] No goroutine leaks (test with 30-second timeout)
- [ ] Performance targets met (1000+ msg/sec)

### INTEGRATION CONTRACTS:
**What Step 2.2 (Registry) expects:**
- NewConnection(conn *websocket.Conn) *Connection constructor
- Connection.WriteJSON() thread-safe for concurrent calls
- Connection.Close() can be called multiple times safely
- GetUserID()/GetRole()/GetSessionID() return values set during authentication

**What Step 2.3 (Auth Handler) will do:**
- Call SetCredentials(userID, role, sessionID string) after token validation
- Call WriteJSON() to send authentication response
- Call Close() on authentication failure

### READ ONLY THESE REFERENCES:
- switchboard-tech-specs.md lines 304-327 (Go concurrency patterns)
- switchboard-tech-specs.md lines 528-542 (WebSocket connection process)
- pkg/interfaces/connection.go (interface definition from Step 1.2)

### IGNORE EVERYTHING ELSE
Do not read other sections of technical specs. Do not implement features not listed above.

### FILES TO CREATE:
- internal/websocket/connection.go (implementation)
- internal/websocket/connection_test.go (all test cases including architectural/functional validation)
- internal/websocket/errors.go (error type definitions)

---

## Step 2.2: Connection Registry (Estimated: 2h)

### EXACT REQUIREMENTS (do not exceed scope):
- Thread-safe connection registry using ConnectionManager from pkg/types
- Atomic registration/deregistration operations
- Connection replacement: new connection immediately replaces old one for same user+session
- Efficient lookup: O(1) access to connections by user ID, session ID, or role
- Proper cleanup: remove from all maps when connection closes
- No business logic: pure connection tracking and lookup

### ARCHITECTURAL VALIDATION REQUIREMENTS:
**Dependencies** (BLOCKING):
- Only import: pkg/types, pkg/interfaces, internal/websocket, standard library (sync)
- No imports from: internal/session, internal/router (prevent coupling)
- Verify: `go mod graph | grep -E "cycle|circular"` returns empty

**Boundaries** (BLOCKING):
- Pure connection management - no session validation, message routing, auth decisions
- Clean separation: connection tracking vs connection operations
- Registry pattern: centralized connection lookup without business logic

**Integration Contracts** (BLOCKING):
- Hub can call RegisterConnection()/UnregisterConnection() safely from main goroutine
- Message router can call GetSessionConnections() for efficient recipient lookup
- Session manager can call GetConnections(sessionID) to clean up on session end

### FUNCTIONAL VALIDATION REQUIREMENTS:
**Core Behaviors** (BLOCKING):
- RegisterConnection() adds to all appropriate maps (global, session-role specific)
- UnregisterConnection() removes from all maps atomically
- Connection replacement: old connection automatically cleaned up when new one registered
- GetSessionConnections() returns all connections for a session
- GetUserConnection() returns current connection for a user globally

**Error Handling** (BLOCKING):
- Registration of nil connection returns ErrNilConnection
- Unregistration of non-existent connection is idempotent (no error)
- Concurrent access protected by mutex without deadlocks
- Map operations safe under high concurrency

**Integration Contracts** (BLOCKING):
- Registry updates immediately visible to all callers
- Connection replacement coordinated with connection cleanup
- Lookup methods return consistent views during concurrent updates

### TECHNICAL VALIDATION REQUIREMENTS:
**Race Detection** (WARNING):
- `go test -race` passes on all registry operations
- No race conditions under concurrent register/unregister operations
- Map access properly synchronized

**Performance** (WARNING):
- O(1) lookup performance for GetUserConnection()
- Registration/unregistration complete in <1ms
- Memory efficient for classroom scale (50 connections)

### MANDATORY INTERFACE (implement exactly):
```go
package websocket

import (
    "sync"
    "github.com/switchboard/pkg/types"
    "github.com/switchboard/pkg/interfaces"
)

// Registry manages WebSocket connections
type Registry struct {
    mu                  sync.RWMutex
    globalConnections   map[string]*Connection            // userID -> Connection
    sessionInstructors  map[string]map[string]*Connection // sessionID -> userID -> Connection
    sessionStudents     map[string]map[string]*Connection // sessionID -> userID -> Connection
}

// NewRegistry creates a new connection registry
func NewRegistry() *Registry {
    return &Registry{
        globalConnections:  make(map[string]*Connection),
        sessionInstructors: make(map[string]map[string]*Connection),
        sessionStudents:    make(map[string]map[string]*Connection),
    }
}

// RegisterConnection adds a connection to all appropriate maps
func (r *Registry) RegisterConnection(conn *Connection) error {
    if conn == nil {
        return ErrNilConnection
    }
    
    if !conn.IsAuthenticated() {
        return ErrConnectionNotAuthenticated
    }
    
    userID := conn.GetUserID()
    role := conn.GetRole()
    sessionID := conn.GetSessionID()
    
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // Close any existing connection for this user
    if existingConn, exists := r.globalConnections[userID]; exists {
        go existingConn.Close() // Close asynchronously to avoid deadlock
    }
    
    // Add to global map
    r.globalConnections[userID] = conn
    
    // Add to appropriate session-role map
    if role == "instructor" {
        if r.sessionInstructors[sessionID] == nil {
            r.sessionInstructors[sessionID] = make(map[string]*Connection)
        }
        r.sessionInstructors[sessionID][userID] = conn
    } else if role == "student" {
        if r.sessionStudents[sessionID] == nil {
            r.sessionStudents[sessionID] = make(map[string]*Connection)
        }
        r.sessionStudents[sessionID][userID] = conn
    }
    
    return nil
}

// UnregisterConnection removes a connection from all maps
func (r *Registry) UnregisterConnection(userID string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    conn, exists := r.globalConnections[userID]
    if !exists {
        return // Idempotent - no error if connection doesn't exist
    }
    
    role := conn.GetRole()
    sessionID := conn.GetSessionID()
    
    // Remove from global map
    delete(r.globalConnections, userID)
    
    // Remove from session-role map
    if role == "instructor" {
        if instructors, exists := r.sessionInstructors[sessionID]; exists {
            delete(instructors, userID)
            if len(instructors) == 0 {
                delete(r.sessionInstructors, sessionID)
            }
        }
    } else if role == "student" {
        if students, exists := r.sessionStudents[sessionID]; exists {
            delete(students, userID)
            if len(students) == 0 {
                delete(r.sessionStudents, sessionID)
            }
        }
    }
}
```

### CRITICAL LOOKUP PATTERNS (must follow):
```go
// GetUserConnection returns the current connection for a user
func (r *Registry) GetUserConnection(userID string) (*Connection, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    conn, exists := r.globalConnections[userID]
    return conn, exists
}

// GetSessionConnections returns all connections in a session
func (r *Registry) GetSessionConnections(sessionID string) []*Connection {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    var connections []*Connection
    
    // Add instructors
    if instructors, exists := r.sessionInstructors[sessionID]; exists {
        for _, conn := range instructors {
            connections = append(connections, conn)
        }
    }
    
    // Add students
    if students, exists := r.sessionStudents[sessionID]; exists {
        for _, conn := range students {
            connections = append(connections, conn)
        }
    }
    
    return connections
}

// GetSessionInstructors returns instructor connections for a session
func (r *Registry) GetSessionInstructors(sessionID string) []*Connection {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    var connections []*Connection
    if instructors, exists := r.sessionInstructors[sessionID]; exists {
        for _, conn := range instructors {
            connections = append(connections, conn)
        }
    }
    
    return connections
}

// GetSessionStudents returns student connections for a session
func (r *Registry) GetSessionStudents(sessionID string) []*Connection {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    var connections []*Connection
    if students, exists := r.sessionStudents[sessionID]; exists {
        for _, conn := range students {
            connections = append(connections, conn)
        }
    }
    
    return connections
}

// GetStats returns registry statistics
func (r *Registry) GetStats() map[string]int {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    return map[string]int{
        "total_connections": len(r.globalConnections),
        "active_sessions":   len(r.sessionInstructors) + len(r.sessionStudents),
    }
}
```

### ERROR TYPES (define exactly these):
```go
var (
    ErrNilConnection              = errors.New("connection cannot be nil")
    ErrConnectionNotAuthenticated = errors.New("connection must be authenticated before registration")
)
```

### SUCCESS CRITERIA:
**Architectural** (BLOCKING):
- [ ] No circular dependencies: clean imports without coupling
- [ ] Boundary separation: pure connection tracking, no business logic
- [ ] Thread safety: all operations protected by mutex
- [ ] Registry pattern: centralized lookup without side effects

**Functional** (BLOCKING):
- [ ] RegisterConnection() updates all maps atomically
- [ ] UnregisterConnection() removes from all maps completely  
- [ ] Connection replacement works correctly (old connection closed)
- [ ] Lookup methods return consistent results during concurrent updates

**Technical** (WARNING):
- [ ] `go test -race ./internal/websocket` passes
- [ ] O(1) lookup performance maintained
- [ ] Memory usage efficient for classroom scale
- [ ] No deadlocks under concurrent access

### INTEGRATION CONTRACTS:
**What Step 2.3 (WebSocket Handler) expects:**
- RegisterConnection() accepts authenticated connections
- UnregisterConnection() handles cleanup on disconnect
- Registry provides immediate consistency for connection state

**What Step 3.1 (Message Router) will use:**
- GetSessionInstructors()/GetSessionStudents() for efficient recipient lookup
- GetUserConnection() for direct message routing
- Thread-safe concurrent access during message routing

### READ ONLY THESE REFERENCES:
- switchboard-tech-specs.md lines 136-143 (ConnectionManager data structure)
- switchboard-tech-specs.md lines 224-227 (connection replacement logic)
- pkg/types/types.go (ConnectionManager struct from Step 1.1)

### IGNORE EVERYTHING ELSE
Do not read sections about message routing, session management, or authentication logic. Focus only on connection tracking.

### FILES TO CREATE:
- internal/websocket/registry.go (connection registry implementation)
- internal/websocket/registry_test.go (comprehensive registry tests including race conditions)

---

## Step 2.3: WebSocket Handler and Authentication (Estimated: 2.5h)

### EXACT REQUIREMENTS (do not exceed scope):
- HTTP upgrade handler for WebSocket connections
- Query parameter validation: user_id, role, session_id (all required)
- Role validation: students must be in session's student_ids, instructors have universal access
- Connection authentication flow: validate → authenticate → register → send history
- Heartbeat handling: WebSocket ping/pong every 30 seconds, cleanup stale connections
- Error responses: proper HTTP status codes for different failure types
- Integration with Connection and Registry from previous steps

### ARCHITECTURAL VALIDATION REQUIREMENTS:
**Dependencies** (BLOCKING):
- Only import: internal/websocket, pkg/interfaces, pkg/types, net/http, github.com/gorilla/websocket
- No imports from: internal/router (prevents circular dependency)
- Verify: `go mod graph | grep -E "cycle|circular"` returns empty

**Boundaries** (BLOCKING):
- WebSocket handling and authentication only - no message routing
- Clean separation: connection setup vs message processing
- Authentication against session membership only - no user auth

**Integration Contracts** (BLOCKING):
- Uses SessionManager interface for session validation
- Creates Connection instances from Step 2.1
- Registers connections with Registry from Step 2.2
- Sends history via DatabaseManager interface

### FUNCTIONAL VALIDATION REQUIREMENTS:
**Core Behaviors** (BLOCKING):
- WebSocket upgrade only succeeds with valid query parameters
- Student role validation: must be in session's student_ids list
- Instructor role validation: universal access to all active sessions
- Connection replacement: new connection replaces old one for same user+session
- History replay: send all session messages with role-based filtering
- Heartbeat monitoring: cleanup stale connections after 120 seconds

**Error Handling** (BLOCKING):
- 400 Bad Request: missing/invalid query parameters
- 403 Forbidden: student not in session's student list
- 404 Not Found: session doesn't exist or is ended
- WebSocket close on any authentication failure
- Proper cleanup on connection errors

**Integration Contracts** (BLOCKING):
- SessionManager.ValidateSessionMembership() called for authorization
- Connection.SetCredentials() called after successful validation
- Registry.RegisterConnection() called after authentication
- DatabaseManager.GetSessionHistory() called for history replay

### TECHNICAL VALIDATION REQUIREMENTS:
**Performance** (WARNING):
- WebSocket upgrade completes in <100ms
- History replay efficient for sessions with 1000+ messages
- Heartbeat overhead minimal (<1% CPU)

**Security** (WARNING):
- Query parameter validation prevents injection attacks
- Session membership validation enforced strictly
- No sensitive data logged in error messages

### MANDATORY HANDLER (implement exactly):
```go
package websocket

import (
    "context"
    "log"
    "net/http"
    "time"
    
    "github.com/gorilla/websocket"
    "github.com/switchboard/pkg/interfaces"
    "github.com/switchboard/pkg/types"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        // Allow all origins - production should be more restrictive
        return true
    },
    HandshakeTimeout: 10 * time.Second,
}

// Handler manages WebSocket connections
type Handler struct {
    registry       *Registry
    sessionManager interfaces.SessionManager
    dbManager      interfaces.DatabaseManager
}

// NewHandler creates a new WebSocket handler
func NewHandler(registry *Registry, sessionManager interfaces.SessionManager, dbManager interfaces.DatabaseManager) *Handler {
    return &Handler{
        registry:       registry,
        sessionManager: sessionManager,
        dbManager:      dbManager,
    }
}

// HandleWebSocket handles WebSocket connection requests
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    // Extract and validate query parameters
    userID := r.URL.Query().Get("user_id")
    role := r.URL.Query().Get("role")
    sessionID := r.URL.Query().Get("session_id")
    
    if userID == "" || role == "" || sessionID == "" {
        http.Error(w, "Missing required query parameters: user_id, role, session_id", http.StatusBadRequest)
        return
    }
    
    // Validate user ID format
    if !types.IsValidUserID(userID) {
        http.Error(w, "Invalid user_id format", http.StatusBadRequest)
        return
    }
    
    // Validate role
    if role != "student" && role != "instructor" {
        http.Error(w, "Invalid role: must be 'student' or 'instructor'", http.StatusBadRequest)
        return
    }
    
    // Validate session membership
    ctx := context.Background()
    if err := h.sessionManager.ValidateSessionMembership(sessionID, userID, role); err != nil {
        if err == interfaces.ErrSessionNotFound {
            http.Error(w, "Session not found or ended", http.StatusNotFound)
        } else if err == interfaces.ErrUnauthorized {
            http.Error(w, "Not authorized to join this session", http.StatusForbidden)
        } else {
            http.Error(w, "Session validation failed", http.StatusInternalServerError)
        }
        return
    }
    
    // Upgrade to WebSocket
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("WebSocket upgrade failed: %v", err)
        return
    }
    
    // Create connection wrapper
    wsConn := NewConnection(conn)
    
    // Set credentials after successful validation
    if err := wsConn.SetCredentials(userID, role, sessionID); err != nil {
        log.Printf("Failed to set credentials: %v", err)
        wsConn.Close()
        return
    }
    
    // Register connection (this will replace any existing connection)
    if err := h.registry.RegisterConnection(wsConn); err != nil {
        log.Printf("Failed to register connection: %v", err)
        wsConn.Close()
        return
    }
    
    // Send session history
    go h.sendSessionHistory(wsConn)
    
    // Start connection monitoring
    go h.handleConnection(wsConn)
}
```

### CRITICAL PATTERNS (must follow):
```go
// sendSessionHistory sends all historical messages to new connection
func (h *Handler) sendSessionHistory(conn *Connection) {
    sessionID := conn.GetSessionID()
    userID := conn.GetUserID()
    role := conn.GetRole()
    
    ctx := context.Background()
    messages, err := h.dbManager.GetSessionHistory(ctx, sessionID)
    if err != nil {
        log.Printf("Failed to get session history: %v", err)
        // Send error message to client
        errorMsg := map[string]interface{}{
            "type": "system",
            "content": map[string]interface{}{
                "event": "history_unavailable",
                "message": "Unable to load message history",
            },
            "timestamp": time.Now(),
        }
        conn.WriteJSON(errorMsg)
        return
    }
    
    // Filter messages based on role
    for _, message := range messages {
        shouldSend := false
        
        if role == "instructor" {
            // Instructors see all messages
            shouldSend = true
        } else if role == "student" {
            // Students see messages involving them or broadcasts
            if message.FromUser == userID || 
               (message.ToUser != nil && *message.ToUser == userID) ||
               message.ToUser == nil { // Broadcast message
                shouldSend = true
            }
        }
        
        if shouldSend {
            if err := conn.WriteJSON(message); err != nil {
                log.Printf("Failed to send history message: %v", err)
                return
            }
        }
    }
    
    // Send history complete notification
    completeMsg := map[string]interface{}{
        "type": "system",
        "content": map[string]interface{}{
            "event": "history_complete",
            "message": "Message history loaded",
        },
        "timestamp": time.Now(),
    }
    conn.WriteJSON(completeMsg)
}

// handleConnection manages the connection lifecycle
func (h *Handler) handleConnection(conn *Connection) {
    defer func() {
        h.registry.UnregisterConnection(conn.GetUserID())
        conn.Close()
    }()
    
    // Set up ping/pong heartbeat
    conn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
    conn.conn.SetPongHandler(func(string) error {
        conn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        return nil
    })
    
    // Start ping ticker
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    go func() {
        for {
            select {
            case <-ticker.C:
                if err := conn.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
                    return
                }
            case <-conn.ctx.Done():
                return
            }
        }
    }()
    
    // Read pump - handle incoming messages
    for {
        messageType, data, err := conn.conn.ReadMessage()
        if err != nil {
            if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
                log.Printf("WebSocket error: %v", err)
            }
            break
        }
        
        if messageType == websocket.TextMessage {
            // Forward message to message router
            // This will be implemented in Phase 3
            log.Printf("Received message from %s: %s", conn.GetUserID(), string(data))
        }
    }
}
```

### ERROR HANDLING (implement exactly):
```go
// Define WebSocket-specific errors
var (
    ErrInvalidParameters = errors.New("invalid connection parameters")
    ErrSessionValidation = errors.New("session validation failed")
    ErrConnectionSetup   = errors.New("connection setup failed")
)

// HTTP error responses with appropriate status codes
func (h *Handler) sendHTTPError(w http.ResponseWriter, message string, statusCode int) {
    w.Header().Set("Content-Type", "text/plain")
    w.WriteHeader(statusCode)
    w.Write([]byte(message))
}
```

### SUCCESS CRITERIA:
**Architectural** (BLOCKING):
- [ ] No circular dependencies: clean integration with other components
- [ ] Boundary separation: WebSocket handling only, no message routing
- [ ] Interface usage: proper integration with SessionManager and DatabaseManager
- [ ] Connection lifecycle: proper setup, authentication, registration, cleanup

**Functional** (BLOCKING):
- [ ] Query parameter validation works correctly for all cases
- [ ] Role-based session membership validation enforced
- [ ] Connection replacement handles duplicate connections properly
- [ ] History replay filtered correctly by role
- [ ] Heartbeat monitoring prevents stale connections

**Technical** (WARNING):
- [ ] WebSocket upgrade completes quickly (<100ms)
- [ ] History replay efficient for large sessions
- [ ] No connection leaks on errors
- [ ] Proper cleanup on disconnection

### INTEGRATION CONTRACTS:
**What Step 3.1 (Message Router) expects:**
- Authenticated connections registered in Registry
- Message forwarding integration point in handleConnection()
- Clean connection state management

**What SessionManager interface provides:**
- ValidateSessionMembership(sessionID, userID, role string) error
- Session existence and membership validation

**What DatabaseManager interface provides:**
- GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error)

### READ ONLY THESE REFERENCES:
- switchboard-tech-specs.md lines 509-542 (WebSocket endpoint and connection process)
- switchboard-tech-specs.md lines 214-238 (client connection algorithm)  
- switchboard-tech-specs.md lines 241-254 (history replay algorithm)
- pkg/interfaces/session.go and pkg/interfaces/database.go (interface definitions)

### IGNORE EVERYTHING ELSE
Do not read sections about message routing or session management implementation. Focus only on WebSocket connection handling.

### FILES TO CREATE:
- internal/websocket/handler.go (WebSocket handler implementation)
- internal/websocket/handler_test.go (comprehensive handler tests including error cases)
- internal/websocket/heartbeat.go (heartbeat monitoring implementation)