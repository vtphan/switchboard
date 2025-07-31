# Phase 3: Message Processing & Rate Limiting

**Estimated Time:** 2 days  
**Dependencies:** Phase 1 (DatabaseManager, Message model), Phase 2 (SessionManager)  
**Provides:** MessageProcessor, MessageRouter, RoleBasedFilter, RateLimiter

## Phase Overview

Implements the core message processing pipeline with 3-message type routing, role-based filtering for educational privacy, and rate limiting. This phase establishes the message flow that is central to real-time communication.

---

## Step 3.1: Message Processing Pipeline (Estimated: 4h)

### EXACT REQUIREMENTS:
Read **exactly lines 304-345** of `/Users/vinhthuyphan/Apps/switchboard/docs/tech-specs.md` for complete ProcessIncomingMessage algorithm. Implement exactly as specified with session validation, rate limiting, and asynchronous persistence.

### ARCHITECTURAL VALIDATION:
- Implementation in `internal/message/processor.go`
- Uses SessionManager interface from Phase 2 for session state
- Uses DatabaseManager interface from Phase 1 for persistence
- No imports from: `internal/websocket` (avoid circular dependencies)

### FUNCTIONAL VALIDATION:
- Session gating: Messages rejected without active session
- Message validation: Parse JSON, validate required fields
- Rate limiting: Calls rate limiter before processing
- Asynchronous persistence: Database writes don't block real-time delivery

### INTEGRATION CONTRACTS:
- WebSocket handlers will call ProcessIncomingMessage() for all messages
- MessageRouter routes messages by type to appropriate recipients
- RoleBasedFilter ensures educational privacy rules
- DatabaseManager persists messages with batching

### MANDATORY INTERFACE:
```go
// MessageProcessor handles incoming message processing pipeline
type MessageProcessor struct {
    sessionManager SessionManager
    dbManager     DatabaseManager  
    rateLimiter   *RateLimiter
    router        *MessageRouter
}

// ProcessIncomingMessage processes a raw message from WebSocket
func (mp *MessageProcessor) ProcessIncomingMessage(rawData []byte, senderID string) error
```

### EXACT IMPLEMENTATION PATTERN:
```go
func (mp *MessageProcessor) ProcessIncomingMessage(rawData []byte, senderID string) error {
    // Step 1: Atomic session state capture
    session := mp.sessionManager.GetActiveSession()
    if session == nil {
        return ErrNoActiveSession
    }
    
    // Step 2: Parse and validate message
    var message Message
    if err := json.Unmarshal(rawData, &message); err != nil {
        return fmt.Errorf("invalid message format: %w", err)
    }
    
    if err := validateMessage(&message); err != nil {
        return fmt.Errorf("message validation failed: %w", err)
    }
    
    // Step 3: Prepare message with session context
    message.ID = generateMessageID()
    message.Timestamp = time.Now()
    message.FromUser = senderID
    message.SessionID = session.ID
    
    if message.Context == "" {
        message.Context = ContextGeneral
    }
    
    // Step 4: Rate limiting check
    if !mp.rateLimiter.Allow(senderID) {
        return ErrRateLimitExceeded
    }
    
    // Step 5: Route message and determine recipients
    recipients, err := mp.router.GetRecipients(&message)
    if err != nil {
        return fmt.Errorf("message routing failed: %w", err)
    }
    
    // Step 6: Real-time broadcast (immediate delivery)
    mp.broadcastToRecipients(&message, recipients)
    
    // Step 7: Asynchronous persistence
    go func() {
        if err := mp.dbManager.WriteMessage(&message); err != nil {
            log.Printf("Failed to persist message %s: %v", message.ID, err)
        }
    }()
    
    // Step 8: Return success to sender
    return nil
}

func validateMessage(msg *Message) error {
    if msg.Type == "" {
        return errors.New("message type required")
    }
    
    switch msg.Type {
    case MessageTypeBroadcastToInstructors, MessageTypeDirectMessage, MessageTypeBroadcastToStudents:
        // Valid types
    default:
        return ErrInvalidMessageType
    }
    
    if msg.Type == MessageTypeDirectMessage && (msg.ToUser == nil || *msg.ToUser == "") {
        return errors.New("direct message requires to_user field")
    }
    
    if msg.Content == nil {
        return errors.New("message content required")
    }
    
    return nil
}
```

### SUCCESS CRITERIA:
- [ ] Session validation occurs before any processing
- [ ] Message parsing handles malformed JSON gracefully
- [ ] Rate limiting prevents processing when limit exceeded
- [ ] Database persistence is asynchronous (doesn't block)
- [ ] Error handling returns appropriate error types
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `internal/message/processor.go`
- `internal/message/processor_test.go`
- `internal/message/validation.go`
- `internal/message/validation_test.go`

---

## Step 3.2: Message Routing by Type (Estimated: 3h)

### EXACT REQUIREMENTS:
Implement 3-message type routing system exactly as specified. Route `broadcast_to_instructors`, `direct_message`, and `broadcast_to_students` to appropriate recipients based on connection registry.

### ARCHITECTURAL VALIDATION:
- Implementation in `internal/message/router.go` 
- No dependency on WebSocket connection details
- Uses recipient interface to abstract connection management
- Routes based solely on message type and recipient roles

### FUNCTIONAL VALIDATION:
- broadcast_to_instructors: Routes to all instructor connections
- direct_message: Routes to specific user connection (validates ToUser field)
- broadcast_to_students: Routes to all student connections
- Error handling: Returns errors for invalid message types or missing recipients

### INTEGRATION CONTRACTS:
- MessageProcessor calls GetRecipients() for each processed message
- WebSocket connection layer provides recipient interface implementation
- RoleBasedFilter applies filtering after routing determines base recipients

### MANDATORY INTERFACE:
```go
// Recipient represents a message recipient
type Recipient interface {
    GetUserID() string
    GetRole() string
    SendMessage(data []byte) error
}

// RecipientRegistry provides access to connected users
type RecipientRegistry interface {
    GetInstructors() []Recipient
    GetStudents() []Recipient
    GetUserByID(userID string) (Recipient, error)
    GetAllUsers() []Recipient
}

// MessageRouter handles message routing by type
type MessageRouter struct {
    recipients RecipientRegistry
}

// GetRecipients returns recipients for a message based on type
func (mr *MessageRouter) GetRecipients(message *Message) ([]Recipient, error)
```

### EXACT IMPLEMENTATION PATTERN:
```go
func (mr *MessageRouter) GetRecipients(message *Message) ([]Recipient, error) {
    switch message.Type {
    case MessageTypeBroadcastToInstructors:
        // Route to all instructor connections
        return mr.recipients.GetInstructors(), nil
        
    case MessageTypeDirectMessage:
        // Route to specific user connection
        if message.ToUser == nil || *message.ToUser == "" {
            return nil, errors.New("direct message requires to_user field")
        }
        
        recipient, err := mr.recipients.GetUserByID(*message.ToUser)
        if err != nil {
            return nil, fmt.Errorf("recipient not found: %w", err)
        }
        
        return []Recipient{recipient}, nil
        
    case MessageTypeBroadcastToStudents:
        // Route to all student connections
        return mr.recipients.GetStudents(), nil
        
    default:
        return nil, ErrInvalidMessageType
    }
}
```

### SUCCESS CRITERIA:
- [ ] All 3 message types route to correct recipient sets
- [ ] Direct messages validate ToUser field presence
- [ ] Routing errors handled gracefully (missing recipients)
- [ ] No WebSocket-specific dependencies in routing logic
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `internal/message/router.go`
- `internal/message/router_test.go`
- `internal/message/interfaces.go` (Recipient interfaces)

---

## Step 3.3: Role-Based Message Filtering (Estimated: 2h)

### EXACT REQUIREMENTS:
Read **exactly lines 719-749** of `/Users/vinhthuyphan/Apps/switchboard/docs/tech-specs.md` for complete role-based filtering algorithm. Implement exactly as specified to preserve educational privacy.

### ARCHITECTURAL VALIDATION:
- Implementation in `internal/message/filter.go`
- Pure filtering logic with no external dependencies
- Uses Message model and role constants only
- No connection or session dependencies

### FUNCTIONAL VALIDATION:
- Instructors see all messages (complete oversight)
- Students see filtered messages (privacy preservation)
- broadcast_to_instructors: Students cannot see other students' questions
- direct_message: Only participants see the message
- broadcast_to_students: All students see instructor announcements

### INTEGRATION CONTRACTS:
- MessageProcessor applies filtering after routing determines recipients
- WebSocket broadcast system uses filtering before message delivery
- Filtering is stateless and can be applied concurrently

### MANDATORY INTERFACE:
```go
// RoleBasedFilter handles educational privacy filtering
type RoleBasedFilter struct{}

// ShouldReceiveMessage determines if recipient should receive message
func (rbf *RoleBasedFilter) ShouldReceiveMessage(message *Message, recipientRole, recipientUserID string) bool
```

### EXACT IMPLEMENTATION PATTERN:
```go
func (rbf *RoleBasedFilter) ShouldReceiveMessage(message *Message, recipientRole, recipientUserID string) bool {
    // Instructors see all messages for educational oversight
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

### SUCCESS CRITERIA:
- [ ] Instructors receive all messages (complete oversight)
- [ ] Students cannot see other students' questions to instructors
- [ ] Direct messages only visible to participants
- [ ] Student broadcasts visible to all students
- [ ] Filtering logic is stateless and concurrent-safe
- [ ] Coverage ≥85% statements with comprehensive role/type matrix

### FILES TO CREATE:
- `internal/message/filter.go`
- `internal/message/filter_test.go`

---

## Step 3.4: Rate Limiting Implementation (Estimated: 3h)

### EXACT REQUIREMENTS:
Implement rate limiting with 100 messages per minute per user, cleanup interval of 5 minutes, and thread-safe concurrent access. Follow standard token bucket or sliding window algorithm.

### ARCHITECTURAL VALIDATION:
- Implementation in `internal/rate/limiter.go`
- Thread-safe using sync.RWMutex for user rate tracking
- No dependencies on message or session packages
- Configurable limits using constants from Phase 1

### FUNCTIONAL VALIDATION:
- Rate limiting: 100 messages per user per minute (RateLimitMaxMessages)
- Cleanup: Remove stale entries every 5 minutes (RateLimitCleanupInterval)
- Thread safety: Multiple goroutines can check limits concurrently
- Memory management: Automatic cleanup prevents memory leaks

### INTEGRATION CONTRACTS:
- MessageProcessor calls Allow(userID) before processing messages
- Rate limiter is independent and doesn't need message context
- Returns boolean result for immediate accept/reject decision

### MANDATORY INTERFACE:
```go
// RateLimiter provides per-user rate limiting
type RateLimiter struct {
    mu           sync.RWMutex
    userCounts   map[string]*UserRateLimit
    maxMessages  int
    window       time.Duration
    cleanupTicker *time.Ticker
    stopCh       chan struct{}
}

// UserRateLimit tracks messages for a specific user
type UserRateLimit struct {
    messages  []time.Time
    lastSeen  time.Time
}

// Allow checks if user can send a message
func (rl *RateLimiter) Allow(userID string) bool

// Start begins cleanup goroutine
func (rl *RateLimiter) Start()

// Stop ends cleanup goroutine
func (rl *RateLimiter) Stop()
```

### EXACT IMPLEMENTATION PATTERN:
```go
func NewRateLimiter() *RateLimiter {
    rl := &RateLimiter{
        userCounts:  make(map[string]*UserRateLimit),
        maxMessages: RateLimitMaxMessages,  // 100
        window:      RateLimitWindow,       // 1 minute
        cleanupTicker: time.NewTicker(RateLimitCleanupInterval), // 5 minutes
        stopCh:      make(chan struct{}),
    }
    return rl
}

func (rl *RateLimiter) Allow(userID string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    now := time.Now()
    cutoff := now.Add(-rl.window)
    
    // Get or create user rate limit
    userLimit, exists := rl.userCounts[userID]
    if !exists {
        userLimit = &UserRateLimit{
            messages: make([]time.Time, 0),
            lastSeen: now,
        }
        rl.userCounts[userID] = userLimit
    }
    
    // Remove expired messages (sliding window)
    validMessages := userLimit.messages[:0]
    for _, timestamp := range userLimit.messages {
        if timestamp.After(cutoff) {
            validMessages = append(validMessages, timestamp)
        }
    }
    userLimit.messages = validMessages
    userLimit.lastSeen = now
    
    // Check if under limit
    if len(userLimit.messages) >= rl.maxMessages {
        return false
    }
    
    // Add current message
    userLimit.messages = append(userLimit.messages, now)
    return true
}

func (rl *RateLimiter) Start() {
    go rl.cleanupLoop()
}

func (rl *RateLimiter) cleanupLoop() {
    for {
        select {
        case <-rl.cleanupTicker.C:
            rl.cleanup()
        case <-rl.stopCh:
            return
        }
    }
}

func (rl *RateLimiter) cleanup() {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    cutoff := time.Now().Add(-RateLimitCleanupInterval)
    
    for userID, userLimit := range rl.userCounts {
        if userLimit.lastSeen.Before(cutoff) {
            delete(rl.userCounts, userID)
        }
    }
}
```

### SUCCESS CRITERIA:
- [ ] Rate limiting enforces exactly 100 messages per minute per user
- [ ] Sliding window algorithm removes expired message timestamps
- [ ] Cleanup removes inactive users every 5 minutes
- [ ] Thread-safe: `go test -race` passes under concurrent load
- [ ] Memory efficient: No unbounded growth of user tracking
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `internal/rate/limiter.go`
- `internal/rate/limiter_test.go`

---

## Phase 3 Integration Tests

### Phase 3 Integration Test: Message Processing Pipeline
```go
// Auto-generate: tests/integration/phase3_message_processing_integration_test.go
func TestMessageProcessing_EndToEnd_Integration(t *testing.T) {
    // Setup components
    db, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)
    defer db.Close()
    
    // Apply schema
    schema, err := os.ReadFile("../../../internal/database/migrations.sql")
    require.NoError(t, err)
    _, err = db.Exec(string(schema))
    require.NoError(t, err)
    
    dbManager := NewSQLiteDatabaseManager(db)
    err = dbManager.Start()
    require.NoError(t, err)
    defer dbManager.Stop()
    
    sessionManager := &SessionManagerImpl{dbManager: dbManager}
    rateLimiter := NewRateLimiter()
    rateLimiter.Start()
    defer rateLimiter.Stop()
    
    // Create mock recipient registry
    instructors := []Recipient{&MockRecipient{userID: "instructor1", role: "instructor"}}
    students := []Recipient{
        &MockRecipient{userID: "student1", role: "student"},
        &MockRecipient{userID: "student2", role: "student"},
    }
    
    recipientRegistry := &MockRecipientRegistry{
        instructors: instructors,
        students:    students,
    }
    
    router := &MessageRouter{recipients: recipientRegistry}
    processor := &MessageProcessor{
        sessionManager: sessionManager,
        dbManager:      dbManager,
        rateLimiter:    rateLimiter,
        router:         router,
    }
    
    // Create active session
    session := &Session{
        ID:        "test-session-123",
        Name:      "Integration Test Session",
        CreatedBy: "instructor1",
        StartTime: time.Now(),
        Status:    "active",
    }
    err = sessionManager.SetActiveSession(session)
    require.NoError(t, err)
    
    // Test message processing for each type
    testCases := []struct {
        name        string
        messageType string
        fromUser    string
        toUser      *string
        expectError bool
    }{
        {
            name:        "broadcast to instructors",
            messageType: MessageTypeBroadcastToInstructors,
            fromUser:    "student1",
            expectError: false,
        },
        {
            name:        "direct message",
            messageType: MessageTypeDirectMessage,
            fromUser:    "student1",
            toUser:      &[]string{"instructor1"}[0],
            expectError: false,
        },
        {
            name:        "broadcast to students",
            messageType: MessageTypeBroadcastToStudents,
            fromUser:    "instructor1",
            expectError: false,
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            message := map[string]interface{}{
                "type":    tc.messageType,
                "context": "general",
                "content": map[string]interface{}{"text": "Test message"},
            }
            
            if tc.toUser != nil {
                message["to_user"] = *tc.toUser
            }
            
            rawData, err := json.Marshal(message)
            require.NoError(t, err)
            
            err = processor.ProcessIncomingMessage(rawData, tc.fromUser)
            if tc.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
    
    // Verify messages were persisted
    time.Sleep(100 * time.Millisecond) // Allow async persistence
    messages, err := dbManager.GetSessionMessages(session.ID)
    require.NoError(t, err)
    assert.Len(t, messages, 3)
}
```

### Phase 3 Integration Test: Role-Based Filtering
```go
// Auto-generate: tests/integration/phase3_role_filtering_integration_test.go
func TestRoleBasedFiltering_Integration(t *testing.T) {
    filter := &RoleBasedFilter{}
    
    // Test messages
    instructorQuestion := &Message{
        Type:     MessageTypeBroadcastToInstructors,
        FromUser: "student1",
        Content:  map[string]interface{}{"text": "I have a question"},
    }
    
    directMessage := &Message{
        Type:     MessageTypeDirectMessage,
        FromUser: "instructor1",
        ToUser:   &[]string{"student2"}[0],
        Content:  map[string]interface{}{"text": "Good work!"},
    }
    
    studentAnnouncement := &Message{
        Type:     MessageTypeBroadcastToStudents,
        FromUser: "instructor1",
        Content:  map[string]interface{}{"text": "Class starts soon"},
    }
    
    // Test filtering matrix
    testCases := []struct {
        message       *Message
        recipientRole string
        recipientID   string
        shouldReceive bool
        description   string
    }{
        // Instructor oversight - sees everything
        {instructorQuestion, "instructor", "instructor1", true, "instructor sees student question"},
        {directMessage, "instructor", "instructor1", true, "instructor sees direct message"},
        {studentAnnouncement, "instructor", "instructor1", true, "instructor sees announcement"},
        
        // Student privacy - filtered view
        {instructorQuestion, "student", "student1", false, "student1 doesn't see own question to instructors"},
        {instructorQuestion, "student", "student2", false, "student2 doesn't see other student's question"},
        {directMessage, "student", "student2", true, "student2 sees direct message to them"},
        {directMessage, "student", "student1", false, "student1 doesn't see direct message to student2"},
        {studentAnnouncement, "student", "student1", true, "student1 sees announcement"},
        {studentAnnouncement, "student", "student2", true, "student2 sees announcement"},
    }
    
    for _, tc := range testCases {
        t.Run(tc.description, func(t *testing.T) {
            result := filter.ShouldReceiveMessage(tc.message, tc.recipientRole, tc.recipientID)
            assert.Equal(t, tc.shouldReceive, result)
        })
    }
}
```

### Phase 3 Integration Test: Rate Limiting Under Load
```go
// Auto-generate: tests/integration/phase3_rate_limiting_integration_test.go
func TestRateLimiter_ConcurrentLoad_Integration(t *testing.T) {
    rateLimiter := NewRateLimiter()
    rateLimiter.Start()
    defer rateLimiter.Stop()
    
    const numUsers = 10
    const messagesPerUser = 150 // Exceeds limit of 100
    const concurrentGoroutines = 50
    
    var wg sync.WaitGroup
    results := make(chan bool, numUsers*messagesPerUser)
    
    // Simulate concurrent message sending
    for user := 0; user < numUsers; user++ {
        for goroutine := 0; goroutine < concurrentGoroutines; goroutine++ {
            wg.Add(1)
            go func(userID string) {
                defer wg.Done()
                
                for msg := 0; msg < messagesPerUser/concurrentGoroutines; msg++ {
                    allowed := rateLimiter.Allow(userID)
                    results <- allowed
                    
                    // Small delay to spread requests
                    time.Sleep(time.Millisecond)
                }
            }(fmt.Sprintf("user_%d", user))
        }
    }
    
    wg.Wait()
    close(results)
    
    // Count allowed vs rejected messages per user
    userCounts := make(map[string]int)
    totalAllowed := 0
    
    for allowed := range results {
        if allowed {
            totalAllowed++
        }
    }
    
    // Should allow approximately 100 messages per user (some variance due to timing)
    expectedMax := numUsers * 100
    expectedMin := numUsers * 90 // Allow 10% variance for timing
    
    assert.GreaterOrEqual(t, totalAllowed, expectedMin, "Too few messages allowed")
    assert.LessOrEqual(t, totalAllowed, expectedMax+10, "Too many messages allowed") // Small buffer for timing
    
    t.Logf("Rate limiter allowed %d/%d messages for %d users", totalAllowed, numUsers*messagesPerUser, numUsers)
}
```

---

## Phase 3 Success Criteria

### ARCHITECTURAL VALIDATION ✓
- [ ] Message processing uses SessionManager and DatabaseManager interfaces only
- [ ] Message routing is independent of WebSocket connection details
- [ ] Role-based filtering is stateless and concurrent-safe
- [ ] Rate limiting is thread-safe and memory-efficient

### FUNCTIONAL VALIDATION ✓
- [ ] Messages rejected without active session (gating works)
- [ ] All 3 message types route to correct recipients
- [ ] Educational privacy preserved: students don't see other students' questions
- [ ] Rate limiting enforces exactly 100 messages/minute per user
- [ ] Asynchronous persistence doesn't block real-time delivery

### TECHNICAL VALIDATION ✓
- [ ] `go test -race` passes on all message processing tests
- [ ] Test coverage ≥85% statements for all message processing code
- [ ] Rate limiter handles concurrent load without memory leaks
- [ ] Message filtering covers all role/type combinations

### INTEGRATION READINESS ✓
- [ ] WebSocket connections can call ProcessIncomingMessage() (Phase 4)
- [ ] Message broadcast system can use routing and filtering (Phase 4)
- [ ] HTTP API can trigger session state for message gating (Phase 5)
- [ ] Complete message flow ready for end-to-end testing (Phase 5)

**Phase 3 establishes the core message processing pipeline that enables real-time educational communication with privacy preservation.**