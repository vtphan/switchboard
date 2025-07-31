# Phase 4: WebSocket Infrastructure

**Estimated Time:** 2 days  
**Dependencies:** Phase 1 (Configuration, ErrorTypes), Phase 2 (SessionManager), Phase 3 (MessageProcessor)  
**Provides:** Connection, ConnectionRegistry, ConnectionLifecycle, HeartbeatMonitoring

## Phase Overview

Implements WebSocket connection management with single-writer pattern, heartbeat monitoring, and automatic cleanup. Establishes the real-time communication infrastructure that connects users to the message processing pipeline.

---

## Step 4.1: WebSocket Connection Wrapper (Estimated: 4h)

### EXACT REQUIREMENTS:
Read **exactly lines 200-299** of `/Users/vinhthuyphan/Apps/switchboard/docs/tech-specs.md` for complete connection lifecycle and goroutine patterns. Implement single-writer pattern with exactly 100-message buffered channel and 5-second WriteJSON timeout.

### ARCHITECTURAL VALIDATION:
- Implementation in `internal/websocket/connection.go`
- Single-writer pattern: One goroutine per connection handles all writes
- No imports from: `internal/session`, `internal/message` (avoid circular dependencies)
- Buffered channel sized exactly `ConnectionSendBufferSize` (100 messages)

### FUNCTIONAL VALIDATION:
- WriteJSON() delivers messages in order sent
- Close() stops all goroutines within 1 second  
- Authentication state persists correctly after SetCredentials()
- Heartbeat monitoring detects dead connections within 30 seconds

### INTEGRATION CONTRACTS:
- ConnectionRegistry can call NewConnection() safely from any goroutine
- MessageProcessor can send filtered messages via WriteJSON()
- Connection cleanup triggers registry removal automatically

### MANDATORY INTERFACE:
```go
// Connection represents a WebSocket connection with single-writer pattern
type Connection struct {
    userID      string
    role        string
    conn        *websocket.Conn
    sendCh      chan []byte          // Buffered: ConnectionSendBufferSize (100)
    closeCh     chan struct{}
    lastSeen    atomic.Value         // time.Time
    mu          sync.RWMutex         // Protects userID and role
}

// Connection interface for dependency injection
type ConnectionInterface interface {
    WriteJSON(v interface{}) error
    Close() error
    GetUserID() string
    GetRole() string
    SetCredentials(username, role string) error
    UpdateActivity()
    GetLastSeen() time.Time
}
```

### EXACT IMPLEMENTATION PATTERN:
```go
func NewConnection(conn *websocket.Conn) *Connection {
    c := &Connection{
        conn:    conn,
        sendCh:  make(chan []byte, ConnectionSendBufferSize), // Exactly 100
        closeCh: make(chan struct{}),
    }
    c.lastSeen.Store(time.Now())
    return c
}

func (c *Connection) WriteJSON(v interface{}) error {
    data, err := json.Marshal(v)
    if err != nil {
        return fmt.Errorf("json marshal error: %w", err)
    }
    
    select {
    case c.sendCh <- data:
        return nil
    case <-c.closeCh:
        return errors.New("connection closed")
    default:
        return ErrChannelFull
    }
}

func (c *Connection) SetCredentials(username, role string) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    if username == "" || role == "" {
        return errors.New("username and role required")
    }
    
    if role != "student" && role != "instructor" {
        return errors.New("role must be 'student' or 'instructor'")
    }
    
    c.userID = username
    c.role = role
    c.UpdateActivity()
    
    return nil
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

func (c *Connection) UpdateActivity() {
    c.lastSeen.Store(time.Now())
}

func (c *Connection) GetLastSeen() time.Time {
    return c.lastSeen.Load().(time.Time)
}

func (c *Connection) Close() error {
    select {
    case <-c.closeCh:
        return nil // Already closed
    default:
        close(c.closeCh)
        return c.conn.Close()
    }
}

// Start connection goroutines - called by connection handler
func (c *Connection) Start(ctx context.Context, messageHandler func([]byte, string) error) {
    go c.writeLoop(ctx)
    go c.readLoop(ctx, messageHandler)
}

func (c *Connection) writeLoop(ctx context.Context) {
    ticker := time.NewTicker(HeartbeatInterval) // 30 seconds
    defer ticker.Stop()
    
    for {
        select {
        case data := <-c.sendCh:
            c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
            if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
                return // Write error - connection dead
            }
            c.UpdateActivity()
            
        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
            if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
                return // Ping failed - connection dead
            }
            c.UpdateActivity()
            
        case <-c.closeCh:
            return // Clean shutdown
            
        case <-ctx.Done():
            return // Context cancelled
        }
    }
}

func (c *Connection) readLoop(ctx context.Context, messageHandler func([]byte, string) error) {
    c.conn.SetReadLimit(MaxMessageSize) // 64KB limit
    c.conn.SetPongHandler(func(string) error {
        c.UpdateActivity()
        return nil
    })
    
    for {
        select {
        case <-c.closeCh:
            return // Clean shutdown
        case <-ctx.Done():
            return // Context cancelled
        default:
            messageType, data, err := c.conn.ReadMessage()
            if err != nil {
                return // Read error - connection dead
            }
            
            c.UpdateActivity()
            
            if messageType == websocket.TextMessage {
                userID := c.GetUserID()
                if userID != "" { // Only process if authenticated
                    if err := messageHandler(data, userID); err != nil {
                        // Log error but continue processing
                        log.Printf("Message processing error for user %s: %v", userID, err)
                    }
                }
            }
        }
    }
}
```

### SUCCESS CRITERIA:
- [ ] Single-writer pattern: Only writeLoop() goroutine writes to WebSocket
- [ ] Buffered channel sized exactly 100 messages
- [ ] WriteJSON() returns ErrChannelFull when buffer full
- [ ] Close() stops goroutines within 1 second
- [ ] Authentication persists after SetCredentials()
- [ ] Heartbeat ping/pong every 30 seconds
- [ ] `go test -race` passes on connection tests
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `internal/websocket/connection.go`
- `internal/websocket/connection_test.go`

---

## Step 4.2: Connection Registry & Lifecycle (Estimated: 3h)

### EXACT REQUIREMENTS:
Implement thread-safe connection registry with automatic cleanup of stale connections. Registry must support role-based lookup and concurrent registration/removal operations.

### ARCHITECTURAL VALIDATION:
- Implementation in `internal/websocket/registry.go`
- Thread-safe using sync.RWMutex for read-heavy connection access
- No business logic dependencies - pure connection management
- Implements Recipient interfaces from Phase 3 message routing

### FUNCTIONAL VALIDATION:
- Connection registration: Thread-safe addition of new connections
- Role-based lookup: Efficient retrieval of instructors/students
- Automatic cleanup: Removes stale connections every 30 seconds
- Memory management: Prevents unbounded growth of connection map

### INTEGRATION CONTRACTS:
- WebSocket handlers register/unregister connections
- MessageProcessor routes messages via Recipient interface
- Connection cleanup happens automatically in background

### MANDATORY INTERFACE:
```go
// ConnectionRegistry manages active WebSocket connections
type ConnectionRegistry struct {
    mu          sync.RWMutex
    connections map[string]*Connection  // userID -> Connection
    cleanupTicker *time.Ticker
    stopCh      chan struct{}
}

// Implements RecipientRegistry interface from Phase 3
func (cr *ConnectionRegistry) GetInstructors() []Recipient
func (cr *ConnectionRegistry) GetStudents() []Recipient  
func (cr *ConnectionRegistry) GetUserByID(userID string) (Recipient, error)
func (cr *ConnectionRegistry) GetAllUsers() []Recipient

// Registry management
func (cr *ConnectionRegistry) Register(userID string, conn *Connection) error
func (cr *ConnectionRegistry) Unregister(userID string)
func (cr *ConnectionRegistry) Start()
func (cr *ConnectionRegistry) Stop()
```

### EXACT IMPLEMENTATION PATTERN:
```go
func NewConnectionRegistry() *ConnectionRegistry {
    return &ConnectionRegistry{
        connections:   make(map[string]*Connection),
        cleanupTicker: time.NewTicker(ConnectionCleanupInterval), // 30 seconds
        stopCh:        make(chan struct{}),
    }
}

func (cr *ConnectionRegistry) Register(userID string, conn *Connection) error {
    cr.mu.Lock()
    defer cr.mu.Unlock()
    
    if userID == "" {
        return errors.New("userID required")
    }
    
    // Check if user already connected
    if existingConn, exists := cr.connections[userID]; exists {
        // Close existing connection
        existingConn.Close()
    }
    
    cr.connections[userID] = conn
    return nil
}

func (cr *ConnectionRegistry) Unregister(userID string) {
    cr.mu.Lock()
    defer cr.mu.Unlock()
    
    if conn, exists := cr.connections[userID]; exists {
        conn.Close()
        delete(cr.connections, userID)
    }
}

func (cr *ConnectionRegistry) GetInstructors() []Recipient {
    cr.mu.RLock()
    defer cr.mu.RUnlock()
    
    var instructors []Recipient
    for _, conn := range cr.connections {
        if conn.GetRole() == "instructor" {
            instructors = append(instructors, conn)
        }
    }
    return instructors
}

func (cr *ConnectionRegistry) GetStudents() []Recipient {
    cr.mu.RLock()
    defer cr.mu.RUnlock()
    
    var students []Recipient
    for _, conn := range cr.connections {
        if conn.GetRole() == "student" {
            students = append(students, conn)
        }
    }
    return students
}

func (cr *ConnectionRegistry) GetUserByID(userID string) (Recipient, error) {
    cr.mu.RLock()
    defer cr.mu.RUnlock()
    
    conn, exists := cr.connections[userID]
    if !exists {
        return nil, ErrConnectionNotFound
    }
    return conn, nil
}

func (cr *ConnectionRegistry) GetAllUsers() []Recipient {
    cr.mu.RLock()
    defer cr.mu.RUnlock()
    
    recipients := make([]Recipient, 0, len(cr.connections))
    for _, conn := range cr.connections {
        recipients = append(recipients, conn)
    }
    return recipients
}

func (cr *ConnectionRegistry) Start() {
    go cr.cleanupLoop()
}

func (cr *ConnectionRegistry) cleanupLoop() {
    for {
        select {
        case <-cr.cleanupTicker.C:
            cr.cleanupStaleConnections()
        case <-cr.stopCh:
            return
        }
    }
}

func (cr *ConnectionRegistry) cleanupStaleConnections() {
    cr.mu.Lock()
    defer cr.mu.Unlock()
    
    cutoff := time.Now().Add(-ConnectionTimeout) // 120 seconds
    cleaned := 0
    
    for userID, conn := range cr.connections {
        if conn.GetLastSeen().Before(cutoff) {
            conn.Close()
            delete(cr.connections, userID)
            cleaned++
        }
    }
    
    if cleaned > 0 {
        log.Printf("Cleaned up %d stale connections", cleaned)
    }
}

func (cr *ConnectionRegistry) Stop() {
    close(cr.stopCh)
    cr.cleanupTicker.Stop()
    
    // Close all connections
    cr.mu.Lock()
    defer cr.mu.Unlock()
    
    for userID, conn := range cr.connections {
        conn.Close()
        delete(cr.connections, userID)
    }
}
```

### SUCCESS CRITERIA:
- [ ] Thread-safe registration/unregistration under concurrent load
- [ ] Role-based lookups return correct connection sets
- [ ] Cleanup removes stale connections within 30 seconds
- [ ] Registry prevents memory leaks from dead connections
- [ ] Implements RecipientRegistry interface correctly
- [ ] `go test -race` passes on registry tests
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `internal/websocket/registry.go`
- `internal/websocket/registry_test.go`

---

## Step 4.3: WebSocket Handler & Upgrade (Estimated: 3h)

### EXACT REQUIREMENTS:
Implement WebSocket upgrade handler that integrates connection lifecycle with session state and message processing. Handle authentication, session state delivery, and connection cleanup.

### ARCHITECTURAL VALIDATION:
- Implementation in `internal/websocket/handler.go`
- Uses SessionManager interface from Phase 2 for session state
- Uses MessageProcessor interface from Phase 3 for message handling
- HTTP upgrade logic isolated from connection management

### FUNCTIONAL VALIDATION:
- WebSocket upgrade: Validates query parameters and upgrades connection
- Authentication: Sets user credentials immediately after upgrade
- Session state delivery: Sends current session state or waiting message
- Message history: Delivers complete session history to late joiners
- Connection cleanup: Automatic cleanup when connection ends

### INTEGRATION CONTRACTS:
- HTTP server calls HandleWebSocketUpgrade() for /ws endpoints
- Connection integrates with SessionManager for session state
- Message processing integrates with MessageProcessor from Phase 3
- Connection registry manages connection lifecycle

### MANDATORY INTERFACE:
```go
// WebSocketHandler manages WebSocket connections
type WebSocketHandler struct {
    registry       *ConnectionRegistry
    sessionManager SessionManager
    messageProcessor MessageProcessor
    upgrader       websocket.Upgrader
}

// HandleWebSocketUpgrade upgrades HTTP to WebSocket
func (wsh *WebSocketHandler) HandleWebSocketUpgrade(w http.ResponseWriter, r *http.Request) error
```

### EXACT IMPLEMENTATION PATTERN:
```go
func NewWebSocketHandler(registry *ConnectionRegistry, sessionManager SessionManager, messageProcessor MessageProcessor) *WebSocketHandler {
    return &WebSocketHandler{
        registry:        registry,
        sessionManager:  sessionManager,
        messageProcessor: messageProcessor,
        upgrader: websocket.Upgrader{
            CheckOrigin: func(r *http.Request) bool {
                // In development: allow all origins
                // In production: implement proper origin checking
                return true
            },
            HandshakeTimeout: 10 * time.Second,
        },
    }
}

func (wsh *WebSocketHandler) HandleWebSocketUpgrade(w http.ResponseWriter, r *http.Request) error {
    // Extract query parameters
    userID := r.URL.Query().Get("user_id")
    role := r.URL.Query().Get("role")
    
    if userID == "" || role == "" {
        http.Error(w, "user_id and role query parameters required", http.StatusBadRequest)
        return errors.New("missing required parameters")
    }
    
    if role != "student" && role != "instructor" {
        http.Error(w, "role must be 'student' or 'instructor'", http.StatusBadRequest)
        return errors.New("invalid role")
    }
    
    // Upgrade connection
    wsConn, err := wsh.upgrader.Upgrade(w, r, nil)
    if err != nil {
        return fmt.Errorf("websocket upgrade failed: %w", err)
    }
    
    // Create connection wrapper
    conn := NewConnection(wsConn)
    err = conn.SetCredentials(userID, role)
    if err != nil {
        conn.Close()
        return fmt.Errorf("credential setting failed: %w", err)
    }
    
    // Register connection
    err = wsh.registry.Register(userID, conn)
    if err != nil {
        conn.Close()
        return fmt.Errorf("connection registration failed: %w", err)
    }
    
    // Send session state
    wsh.sendSessionState(conn)
    
    // Send session history if session active
    if session := wsh.sessionManager.GetActiveSession(); session != nil {
        wsh.sendSessionHistory(conn, session)
    }
    
    // Start connection goroutines with message handler
    ctx := context.Background() // TODO: Use proper context from HTTP server
    conn.Start(ctx, wsh.handleMessage)
    
    // Connection cleanup happens automatically when goroutines exit
    go func() {
        // Wait for connection to close (goroutines will exit)
        // This is a simplified approach - in production, use proper synchronization
        time.Sleep(100 * time.Millisecond)
        wsh.registry.Unregister(userID)
    }()
    
    return nil
}

func (wsh *WebSocketHandler) sendSessionState(conn *Connection) {
    session := wsh.sessionManager.GetActiveSession()
    
    if session == nil {
        // No active session - send waiting message
        message := map[string]interface{}{
            "type": "system",
            "content": map[string]interface{}{
                "event":   "waiting_for_session",
                "message": "Connected successfully. Waiting for instructor to start session.",
            },
            "timestamp": time.Now(),
        }
        conn.WriteJSON(message)
    } else {
        // Active session - send session details
        message := map[string]interface{}{
            "type": "system",
            "content": map[string]interface{}{
                "event":        "session_active",
                "session_id":   session.ID,
                "session_name": session.Name,
                "started_by":   session.CreatedBy,
                "start_time":   session.StartTime,
            },
            "timestamp": time.Now(),
        }
        conn.WriteJSON(message)
    }
}

func (wsh *WebSocketHandler) sendSessionHistory(conn *Connection, session *Session) {
    // TODO: Implement role-based message filtering for history
    // This would integrate with Phase 1 DatabaseManager and Phase 3 RoleBasedFilter
    
    // For now, send a placeholder indicating history would be sent
    message := map[string]interface{}{
        "type": "system",
        "content": map[string]interface{}{
            "event":   "history_delivered",
            "message": fmt.Sprintf("Session history for %s would be delivered here", session.Name),
        },
        "timestamp": time.Now(),
    }
    conn.WriteJSON(message)
}

func (wsh *WebSocketHandler) handleMessage(rawData []byte, senderID string) error {
    // Delegate to message processor from Phase 3
    return wsh.messageProcessor.ProcessIncomingMessage(rawData, senderID)
}
```

### SUCCESS CRITERIA:
- [ ] WebSocket upgrade validates query parameters correctly
- [ ] Connection authentication happens immediately after upgrade
- [ ] Session state delivered based on current session status
- [ ] Message processing integrates with Phase 3 MessageProcessor
- [ ] Connection cleanup happens automatically on disconnect
- [ ] Error handling returns appropriate HTTP status codes
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `internal/websocket/handler.go`
- `internal/websocket/handler_test.go`

---

## Step 4.4: Broadcast System (Estimated: 2h)

### EXACT REQUIREMENTS:
Implement message broadcast system that delivers filtered messages to appropriate recipients. Must integrate with Phase 3 message routing and role-based filtering.

### ARCHITECTURAL VALIDATION:
- Implementation in `internal/websocket/broadcast.go`
- Uses RecipientRegistry interface for connection lookup
- Uses RoleBasedFilter from Phase 3 for message filtering
- No message processing logic - pure delivery system

### FUNCTIONAL VALIDATION:
- Message delivery: Sends messages to filtered recipient list
- Role-based filtering: Applies educational privacy rules
- Non-blocking delivery: Failed sends to individual connections don't block others
- Error handling: Logs delivery failures but continues processing

### INTEGRATION CONTRACTS:
- MessageProcessor calls broadcast system after processing messages
- Broadcast system uses connection registry for recipient lookup
- Role-based filter applied before message delivery

### MANDATORY INTERFACE:
```go
// BroadcastSystem handles message delivery to connections
type BroadcastSystem struct {
    registry *ConnectionRegistry
    filter   *RoleBasedFilter
}

// BroadcastMessage sends message to appropriate recipients
func (bs *BroadcastSystem) BroadcastMessage(message *Message, recipients []Recipient) error
```

### EXACT IMPLEMENTATION PATTERN:
```go
func NewBroadcastSystem(registry *ConnectionRegistry, filter *RoleBasedFilter) *BroadcastSystem {
    return &BroadcastSystem{
        registry: registry,
        filter:   filter,
    }
}

func (bs *BroadcastSystem) BroadcastMessage(message *Message, recipients []Recipient) error {
    if len(recipients) == 0 {
        return nil // No recipients - not an error
    }
    
    deliveredCount := 0
    errorCount := 0
    
    for _, recipient := range recipients {
        // Apply role-based filtering
        shouldReceive := bs.filter.ShouldReceiveMessage(message, recipient.GetRole(), recipient.GetUserID())
        if !shouldReceive {
            continue
        }
        
        // Deliver message (non-blocking)
        messageData := map[string]interface{}{
            "id":         message.ID,
            "session_id": message.SessionID,
            "type":       message.Type,
            "context":    message.Context,
            "from_user":  message.FromUser,
            "content":    message.Content,
            "timestamp":  message.Timestamp,
        }
        
        if message.ToUser != nil {
            messageData["to_user"] = *message.ToUser
        }
        
        err := recipient.SendMessage(mustMarshalJSON(messageData))
        if err != nil {
            log.Printf("Failed to deliver message %s to user %s: %v", message.ID, recipient.GetUserID(), err)
            errorCount++
        } else {
            deliveredCount++
        }
    }
    
    log.Printf("Message %s delivered to %d/%d recipients", message.ID, deliveredCount, deliveredCount+errorCount)
    
    if errorCount > 0 && deliveredCount == 0 {
        return fmt.Errorf("failed to deliver message to any recipients")
    }
    
    return nil
}

func mustMarshalJSON(v interface{}) []byte {
    data, err := json.Marshal(v)
    if err != nil {
        log.Printf("JSON marshal error: %v", err)
        return []byte("{\"error\":\"marshal_failed\"}")
    }
    return data
}
```

### SUCCESS CRITERIA:
- [ ] Message delivery applies role-based filtering correctly
- [ ] Failed deliveries to individual connections don't block others
- [ ] Broadcast logging shows delivery success/failure counts
- [ ] JSON marshaling errors handled gracefully
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `internal/websocket/broadcast.go`
- `internal/websocket/broadcast_test.go`

---

## Phase 4 Integration Tests

### Phase 4 Integration Test: WebSocket Connection Lifecycle
```go
// Auto-generate: tests/integration/phase4_websocket_lifecycle_integration_test.go
func TestWebSocketConnection_Lifecycle_Integration(t *testing.T) {
    // Setup test server
    registry := NewConnectionRegistry()
    registry.Start()
    defer registry.Stop()
    
    sessionManager := &MockSessionManager{}
    messageProcessor := &MockMessageProcessor{}
    
    handler := NewWebSocketHandler(registry, sessionManager, messageProcessor)
    
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        handler.HandleWebSocketUpgrade(w, r)
    }))
    defer server.Close()
    
    // Convert HTTP URL to WebSocket URL
    wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?user_id=test_user&role=student"
    
    // Test connection establishment
    conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
    require.NoError(t, err)
    defer conn.Close()
    
    // Verify connection registered
    time.Sleep(100 * time.Millisecond) // Allow connection setup
    recipient, err := registry.GetUserByID("test_user")
    require.NoError(t, err)
    assert.Equal(t, "test_user", recipient.GetUserID())
    assert.Equal(t, "student", recipient.GetRole())
    
    // Test message sending
    testMessage := map[string]interface{}{
        "type":    "broadcast_to_instructors",
        "context": "question",
        "content": map[string]interface{}{"text": "I have a question"},
    }
    
    err = conn.WriteJSON(testMessage)
    require.NoError(t, err)
    
    // Verify message was processed
    time.Sleep(100 * time.Millisecond)
    // MockMessageProcessor should have received the message
    
    // Test heartbeat
    err = conn.WriteMessage(websocket.PongMessage, []byte{})
    require.NoError(t, err)
    
    // Test connection cleanup on close
    conn.Close()
    time.Sleep(200 * time.Millisecond) // Allow cleanup
    
    _, err = registry.GetUserByID("test_user")
    assert.Equal(t, ErrConnectionNotFound, err)
}
```

### Phase 4 Integration Test: Message Broadcasting
```go
// Auto-generate: tests/integration/phase4_message_broadcast_integration_test.go
func TestBroadcastSystem_MessageDelivery_Integration(t *testing.T) {
    // Setup connections
    registry := NewConnectionRegistry()
    filter := &RoleBasedFilter{}
    broadcast := NewBroadcastSystem(registry, filter)
    
    // Create mock connections
    instructorConn := &MockConnection{userID: "instructor1", role: "instructor"}
    student1Conn := &MockConnection{userID: "student1", role: "student"}
    student2Conn := &MockConnection{userID: "student2", role: "student"}
    
    registry.Register("instructor1", instructorConn)
    registry.Register("student1", student1Conn)
    registry.Register("student2", student2Conn)
    
    // Test broadcast to instructors (student question)
    instructorMessage := &Message{
        ID:        "msg-001",
        SessionID: "session-123",
        Type:      MessageTypeBroadcastToInstructors,
        Context:   ContextQuestion,
        FromUser:  "student1",
        Content:   map[string]interface{}{"text": "I have a question"},
        Timestamp: time.Now(),
    }
    
    recipients := registry.GetInstructors()
    err := broadcast.BroadcastMessage(instructorMessage, recipients)
    require.NoError(t, err)
    
    // Verify instructor received message
    assert.Len(t, instructorConn.sentMessages, 1)
    // Verify students did not receive message (privacy)
    assert.Len(t, student1Conn.sentMessages, 0)
    assert.Len(t, student2Conn.sentMessages, 0)
    
    // Test broadcast to students (instructor announcement)
    studentMessage := &Message{
        ID:        "msg-002",
        SessionID: "session-123",
        Type:      MessageTypeBroadcastToStudents,
        Context:   ContextAnnouncement,
        FromUser:  "instructor1",
        Content:   map[string]interface{}{"text": "Class starts in 5 minutes"},
        Timestamp: time.Now(),
    }
    
    allRecipients := registry.GetAllUsers()
    err = broadcast.BroadcastMessage(studentMessage, allRecipients)
    require.NoError(t, err)
    
    // Verify all users received the announcement
    assert.Len(t, instructorConn.sentMessages, 2) // Original + announcement
    assert.Len(t, student1Conn.sentMessages, 1)  // Announcement only
    assert.Len(t, student2Conn.sentMessages, 1)  // Announcement only
    
    // Test direct message
    directMessage := &Message{
        ID:        "msg-003",
        SessionID: "session-123",
        Type:      MessageTypeDirectMessage,
        Context:   ContextResponse,
        FromUser:  "instructor1",
        ToUser:    &[]string{"student2"}[0],
        Content:   map[string]interface{}{"text": "Good work on the assignment"},
        Timestamp: time.Now(),
    }
    
    err = broadcast.BroadcastMessage(directMessage, allRecipients)
    require.NoError(t, err)
    
    // Verify only instructor and student2 received direct message
    assert.Len(t, instructorConn.sentMessages, 3) // All messages
    assert.Len(t, student1Conn.sentMessages, 1)  // Announcement only
    assert.Len(t, student2Conn.sentMessages, 2)  // Announcement + direct message
}
```

### Phase 4 Integration Test: Connection Cleanup
```go
// Auto-generate: tests/integration/phase4_connection_cleanup_integration_test.go
func TestConnectionRegistry_AutoCleanup_Integration(t *testing.T) {
    registry := NewConnectionRegistry()
    registry.Start()
    defer registry.Stop()
    
    // Create connections with different activity times
    activeConn := &MockConnection{
        userID:   "active_user",
        role:     "student",
        lastSeen: time.Now(), // Recently active
    }
    
    staleConn := &MockConnection{
        userID:   "stale_user", 
        role:     "student",
        lastSeen: time.Now().Add(-ConnectionTimeout - time.Minute), // Stale
    }
    
    registry.Register("active_user", activeConn)
    registry.Register("stale_user", staleConn)
    
    // Verify both connections registered
    _, err := registry.GetUserByID("active_user")
    require.NoError(t, err)
    _, err = registry.GetUserByID("stale_user")
    require.NoError(t, err)
    
    // Trigger cleanup by waiting for cleanup interval
    // In test, we can manually trigger cleanup
    registry.cleanupStaleConnections()
    
    // Verify stale connection removed, active connection remains
    _, err = registry.GetUserByID("active_user")
    assert.NoError(t, err)
    
    _, err = registry.GetUserByID("stale_user")
    assert.Equal(t, ErrConnectionNotFound, err)
    
    // Verify stale connection was closed
    assert.True(t, staleConn.closed)
    assert.False(t, activeConn.closed)
}
```

---

## Phase 4 Success Criteria

### ARCHITECTURAL VALIDATION ✓
- [ ] Single-writer pattern enforced: only writeLoop() writes to WebSocket
- [ ] Connection registry uses RWMutex for read-heavy access patterns
- [ ] WebSocket handler separated from connection management logic
- [ ] Broadcast system isolated from message processing logic

### FUNCTIONAL VALIDATION ✓
- [ ] WebSocket connections authenticate immediately after upgrade
- [ ] Message delivery maintains order through buffered channels
- [ ] Heartbeat monitoring detects dead connections within 30 seconds
- [ ] Connection cleanup removes stale connections automatically
- [ ] Role-based filtering preserves educational privacy during broadcast

### TECHNICAL VALIDATION ✓
- [ ] `go test -race` passes on all WebSocket connection tests
- [ ] Test coverage ≥85% statements for all connection management code
- [ ] Connection cleanup prevents memory leaks under load
- [ ] Broadcast system handles delivery failures gracefully

### INTEGRATION READINESS ✓
- [ ] HTTP server can call WebSocket upgrade handler (Phase 5)
- [ ] Connection registry implements RecipientRegistry from Phase 3
- [ ] Message processing integrates with Phase 3 MessageProcessor
- [ ] Session state delivery integrates with Phase 2 SessionManager
- [ ] Complete real-time communication pipeline ready for end-to-end testing

**Phase 4 establishes the real-time WebSocket infrastructure that connects users to Switchboard's message processing pipeline.**