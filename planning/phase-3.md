# Phase 3: Message Routing System

## Overview
Implements the core message routing logic that handles all 6 message types, applies role-based permissions, and coordinates message persistence with delivery. Builds on WebSocket infrastructure to provide reliable message distribution.

## Step 3.1: Message Router Implementation (Estimated: 3h)

### EXACT REQUIREMENTS (do not exceed scope):
- Route exactly 6 message types with specific recipient patterns from specs
- Role-based message validation: students can send 3 types, instructors can send 3 types
- Persist-then-route pattern: messages must be persisted before delivery
- Rate limiting: 100 messages per minute per client connection
- Recipient calculation using connection registry for dynamic routing
- Context field handling: default to "general" if empty/missing

### ARCHITECTURAL VALIDATION REQUIREMENTS:
**Dependencies** (BLOCKING):
- Only import: pkg/interfaces, pkg/types, internal/websocket, standard library
- No imports from: internal/session (session management is separate concern)
- Verify: `go mod graph | grep -E "cycle|circular"` returns empty

**Boundaries** (BLOCKING):
- Pure message routing logic - no session management, connection handling, or database operations
- Clean separation: message routing vs persistence vs connection management
- Interface compliance: exact implementation of MessageRouter interface

**Integration Contracts** (BLOCKING):
- Uses Registry from Step 2.2 for recipient lookup
- Uses DatabaseManager interface for message persistence
- Integrates with WebSocket Handler from Step 2.3 for message forwarding

### FUNCTIONAL VALIDATION REQUIREMENTS:
**Core Behaviors** (BLOCKING):
- RouteMessage() handles all 6 message types with correct recipient patterns
- Role validation: students limited to instructor_inbox, request_response, analytics
- Role validation: instructors limited to inbox_response, request, instructor_broadcast
- Rate limiting: exactly 100 messages per minute per client, resets every minute
- Message ID generation: server generates UUID, ignores any client-provided ID
- Context defaulting: empty/missing context becomes "general"

**Error Handling** (BLOCKING):
- Invalid message type returns ErrInvalidMessageType
- Role permission violations return ErrUnauthorizedMessageType
- Rate limit exceeded returns ErrRateLimitExceeded and drops message
- Non-existent recipient returns ErrRecipientNotFound
- Database persistence failure stops routing and returns error

**Integration Contracts** (BLOCKING):
- Database persistence completes before message delivery begins
- Registry provides current connection state for recipient calculation
- WebSocket connections receive messages via WriteJSON() calls

### TECHNICAL VALIDATION REQUIREMENTS:
**Performance** (WARNING):
- Message routing completes in <10ms for broadcast messages
- Rate limiting calculation efficient (O(1) per client)
- Recipient lookup efficient using registry maps

**Concurrency** (WARNING):
- Thread-safe rate limiting across concurrent messages from same client
- Safe concurrent access to connection registry
- No race conditions in persist-then-route sequence

### MANDATORY INTERFACE (implement exactly):
```go
package router

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/switchboard/pkg/interfaces"
    "github.com/switchboard/pkg/types"
    "github.com/switchboard/internal/websocket"
)

// Router implements the MessageRouter interface
type Router struct {
    registry    *websocket.Registry
    dbManager   interfaces.DatabaseManager
    rateLimiter *RateLimiter
}

// NewRouter creates a new message router
func NewRouter(registry *websocket.Registry, dbManager interfaces.DatabaseManager) *Router {
    return &Router{
        registry:    registry,
        dbManager:   dbManager,
        rateLimiter: NewRateLimiter(),
    }
}

// RouteMessage routes a message to appropriate recipients
func (r *Router) RouteMessage(ctx context.Context, message *types.Message) error {
    // Generate server-side message ID (ignore any client-provided ID)
    message.ID = uuid.New().String()
    message.Timestamp = time.Now()
    
    // Set default context if empty
    if message.Context == "" {
        message.Context = "general"
    }
    
    // Validate message content and sender permissions
    sender, err := r.registry.GetUserConnection(message.FromUser)
    if err != nil {
        return ErrSenderNotConnected
    }
    
    if err := r.ValidateMessage(message, sender); err != nil {
        return err
    }
    
    // Check rate limit
    if !r.rateLimiter.Allow(message.FromUser) {
        return ErrRateLimitExceeded
    }
    
    // Persist message first (persist-then-route pattern)
    if err := r.dbManager.StoreMessage(ctx, message); err != nil {
        return fmt.Errorf("failed to persist message: %w", err)
    }
    
    // Get recipients based on message type
    recipients, err := r.GetRecipients(message)
    if err != nil {
        return err
    }
    
    // Deliver to all recipients
    for _, recipient := range recipients {
        if err := recipient.WriteJSON(message); err != nil {
            // Log error but continue delivery to other recipients
            log.Printf("Failed to deliver message to %s: %v", recipient.GetUserID(), err)
        }
    }
    
    return nil
}
```

### CRITICAL ROUTING PATTERNS (must follow):
```go
// GetRecipients determines recipients based on message type
func (r *Router) GetRecipients(message *types.Message) ([]*websocket.Connection, error) {
    sessionID := message.SessionID
    
    switch message.Type {
    case types.MessageTypeInstructorInbox, types.MessageTypeRequestResponse, types.MessageTypeAnalytics:
        // Route to all instructors in session
        return r.registry.GetSessionInstructors(sessionID), nil
        
    case types.MessageTypeInboxResponse, types.MessageTypeRequest:
        // Route to specific student
        if message.ToUser == nil {
            return nil, ErrMissingRecipient
        }
        
        recipient, exists := r.registry.GetUserConnection(*message.ToUser)
        if !exists {
            return nil, ErrRecipientNotFound
        }
        
        // Verify recipient is in the same session
        if recipient.GetSessionID() != sessionID {
            return nil, ErrRecipientNotInSession
        }
        
        return []*websocket.Connection{recipient}, nil
        
    case types.MessageTypeInstructorBroadcast:
        // Route to all students in session
        return r.registry.GetSessionStudents(sessionID), nil
        
    default:
        return nil, ErrInvalidMessageType
    }
}

// ValidateMessage validates message content and sender permissions
func (r *Router) ValidateMessage(message *types.Message, sender *websocket.Connection) error {
    // Verify sender is in the message's session
    if sender.GetSessionID() != message.SessionID {
        return ErrSenderNotInSession
    }
    
    // Validate message type exists
    if !r.isValidMessageType(message.Type) {
        return ErrInvalidMessageType
    }
    
    // Validate role permissions
    senderRole := sender.GetRole()
    if !r.canSendMessageType(senderRole, message.Type) {
        return ErrUnauthorizedMessageType
    }
    
    // Validate context field
    if message.Context != "" && !types.IsValidContext(message.Context) {
        return ErrInvalidContext
    }
    
    // Validate content size
    if err := message.Validate(); err != nil {
        return err
    }
    
    return nil
}

// Role-based message type permissions
func (r *Router) canSendMessageType(role, messageType string) bool {
    switch role {
    case "student":
        return messageType == types.MessageTypeInstructorInbox ||
               messageType == types.MessageTypeRequestResponse ||
               messageType == types.MessageTypeAnalytics
    case "instructor":
        return messageType == types.MessageTypeInboxResponse ||
               messageType == types.MessageTypeRequest ||
               messageType == types.MessageTypeInstructorBroadcast
    default:
        return false
    }
}

func (r *Router) isValidMessageType(messageType string) bool {
    validTypes := []string{
        types.MessageTypeInstructorInbox,
        types.MessageTypeInboxResponse,
        types.MessageTypeRequest,
        types.MessageTypeRequestResponse,
        types.MessageTypeAnalytics,
        types.MessageTypeInstructorBroadcast,
    }
    
    for _, validType := range validTypes {
        if messageType == validType {
            return true
        }
    }
    return false
}
```

### RATE LIMITING IMPLEMENTATION (implement exactly):
```go
// RateLimiter implements per-client rate limiting
type RateLimiter struct {
    mu      sync.RWMutex
    clients map[string]*ClientLimit
}

// ClientLimit tracks rate limiting for a single client
type ClientLimit struct {
    messageCount int
    windowStart  time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
    return &RateLimiter{
        clients: make(map[string]*ClientLimit),
    }
}

// Allow checks if client can send a message (100 per minute limit)
func (rl *RateLimiter) Allow(userID string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    now := time.Now()
    
    limit, exists := rl.clients[userID]
    if !exists {
        rl.clients[userID] = &ClientLimit{
            messageCount: 1,
            windowStart:  now,
        }
        return true
    }
    
    // Check if new minute window needed
    if now.Sub(limit.windowStart) >= time.Minute {
        limit.messageCount = 1
        limit.windowStart = now
        return true
    }
    
    // Check rate limit (100 messages per minute)
    if limit.messageCount >= 100 {
        return false
    }
    
    limit.messageCount++
    return true
}

// Cleanup removes old client entries (call periodically)
func (rl *RateLimiter) Cleanup() {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    now := time.Now()
    for userID, limit := range rl.clients {
        if now.Sub(limit.windowStart) > 5*time.Minute {
            delete(rl.clients, userID)
        }
    }
}
```

### ERROR TYPES (define exactly these):
```go
var (
    ErrInvalidMessageType      = errors.New("invalid message type")
    ErrUnauthorizedMessageType = errors.New("user not authorized to send this message type")
    ErrRateLimitExceeded      = errors.New("rate limit exceeded: 100 messages per minute")
    ErrSenderNotConnected     = errors.New("sender not connected")
    ErrSenderNotInSession     = errors.New("sender not in message session")
    ErrRecipientNotFound      = errors.New("recipient not found")
    ErrRecipientNotInSession  = errors.New("recipient not in same session")
    ErrMissingRecipient       = errors.New("direct message missing recipient")
    ErrInvalidContext         = errors.New("invalid context field")
)
```

### SUCCESS CRITERIA:
**Architectural** (BLOCKING):
- [ ] No circular dependencies: clean integration with WebSocket and database layers
- [ ] Interface compliance: implements MessageRouter interface exactly
- [ ] Boundary separation: pure routing logic, no connection or session management
- [ ] Clean integration: uses Registry and DatabaseManager through interfaces

**Functional** (BLOCKING):
- [ ] All 6 message types route to correct recipients
- [ ] Role-based permissions enforced correctly (3 types each for students/instructors)
- [ ] Rate limiting works exactly (100 messages per minute, resets every minute)
- [ ] Persist-then-route pattern ensures messages saved before delivery
- [ ] Context field defaults to "general" when empty

**Technical** (WARNING):
- [ ] Message routing completes in <10ms for broadcasts
- [ ] Rate limiting thread-safe under concurrent access
- [ ] No race conditions in persist-then-route sequence
- [ ] Efficient recipient lookup using registry

### INTEGRATION CONTRACTS:
**What Step 3.2 (Hub Integration) expects:**
- RouteMessage() method ready for integration with WebSocket Handler
- Error types for proper error handling in hub
- Thread-safe operation for concurrent message processing

**What WebSocket Registry provides:**
- GetSessionInstructors(sessionID) for instructor_inbox, request_response, analytics
- GetSessionStudents(sessionID) for instructor_broadcast
- GetUserConnection(userID) for inbox_response, request messages

**What DatabaseManager provides:**
- StoreMessage(ctx, message) for persistence before routing
- Error handling for database failures

### READ ONLY THESE REFERENCES:
- switchboard-tech-specs.md lines 164-190 (message routing algorithm)
- switchboard-tech-specs.md lines 146-158 (message types and routing patterns)
- switchboard-tech-specs.md lines 276-302 (message validation algorithm)
- switchboard-tech-specs.md lines 339-343 (rate limiting: 100 messages per minute)

### IGNORE EVERYTHING ELSE
Do not read sections about session management, WebSocket connection handling, or database implementation. Focus only on message routing logic.

### FILES TO CREATE:
- internal/router/router.go (message router implementation)
- internal/router/rate_limiter.go (rate limiting implementation)
- internal/router/errors.go (error definitions)
- internal/router/router_test.go (comprehensive routing tests for all 6 message types)

---

## Step 3.2: Hub Integration (Estimated: 2h)

### EXACT REQUIREMENTS (do not exceed scope):
- Central hub goroutine that coordinates message routing with WebSocket connections
- Integration point between WebSocket Handler message reading and Router message processing
- Channel-based communication for message processing and connection lifecycle events
- Error handling and logging for message routing failures
- Clean integration with existing Connection, Registry, and Router components

### ARCHITECTURAL VALIDATION REQUIREMENTS:
**Dependencies** (BLOCKING):
- Only import: internal/websocket, internal/router, pkg/interfaces, pkg/types, standard library
- No imports from: internal/session, internal/database (use interfaces)
- Verify: `go mod graph | grep -E "cycle|circular"` returns empty

**Boundaries** (BLOCKING):
- Coordination layer only - no business logic, just message flow orchestration
- Clean separation: hub coordination vs message routing vs connection management
- Channel-based communication pattern following Go concurrency best practices

**Integration Contracts** (BLOCKING):
- Receives messages from WebSocket Handler read pumps
- Forwards messages to Router for processing
- Coordinates connection lifecycle with Registry
- Provides single point of message flow coordination

### FUNCTIONAL VALIDATION REQUIREMENTS:
**Core Behaviors** (BLOCKING):
- Hub goroutine processes messages from channel in order received
- Message routing errors logged but don't crash hub
- Connection registration/deregistration coordinated through hub
- Graceful shutdown with proper channel cleanup
- Message forwarding preserves sender context correctly

**Error Handling** (BLOCKING):
- Router errors logged with message context for debugging
- Invalid messages discarded with appropriate logging
- Connection errors don't affect other connections
- Hub continues processing after individual message failures

**Integration Contracts** (BLOCKING):
- WebSocket Handler sends messages to hub via channel
- Hub forwards messages to Router.RouteMessage()
- Registry updated atomically for connection changes
- Database operations coordinated through Router

### TECHNICAL VALIDATION REQUIREMENTS:
**Performance** (WARNING):
- Hub processes messages without blocking (<1ms per message)
- Channel buffering prevents backpressure during message bursts
- Memory usage reasonable for classroom-scale message volume

**Concurrency** (WARNING):
- Single hub goroutine prevents race conditions
- Channel operations don't block indefinitely
- Proper cleanup on hub shutdown

### MANDATORY IMPLEMENTATION (implement exactly):
```go
package hub

import (
    "context"
    "log"
    "sync"
    
    "github.com/switchboard/pkg/types"
    "github.com/switchboard/internal/websocket"
    "github.com/switchboard/internal/router"
)

// Hub coordinates message routing and connection management
type Hub struct {
    // Channels for coordination
    messageChannel    chan *MessageContext
    registerChannel   chan *websocket.Connection
    unregisterChannel chan string // userID
    shutdownChannel   chan struct{}
    
    // Components
    registry *websocket.Registry
    router   *router.Router
    
    // State
    running bool
    mu      sync.RWMutex
}

// MessageContext wraps a message with sender information
type MessageContext struct {
    Message    *types.Message
    SenderID   string
    SessionID  string
    Timestamp  time.Time
}

// NewHub creates a new hub
func NewHub(registry *websocket.Registry, router *router.Router) *Hub {
    return &Hub{
        messageChannel:    make(chan *MessageContext, 1000), // Buffer for message bursts
        registerChannel:   make(chan *websocket.Connection, 100),
        unregisterChannel: make(chan string, 100),
        shutdownChannel:   make(chan struct{}),
        registry:         registry,
        router:          router,
        running:         false,
    }
}

// Start begins hub processing
func (h *Hub) Start(ctx context.Context) error {
    h.mu.Lock()
    if h.running {
        h.mu.Unlock()
        return ErrHubAlreadyRunning
    }
    h.running = true
    h.mu.Unlock()
    
    log.Println("Starting message hub...")
    
    // Start the main hub goroutine
    go h.run(ctx)
    
    return nil
}

// Stop gracefully shuts down the hub
func (h *Hub) Stop() error {
    h.mu.Lock()
    if !h.running {
        h.mu.Unlock()
        return ErrHubNotRunning
    }
    h.running = false
    h.mu.Unlock()
    
    log.Println("Stopping message hub...")
    
    close(h.shutdownChannel)
    return nil
}

// SendMessage queues a message for routing
func (h *Hub) SendMessage(message *types.Message, senderID string) error {
    h.mu.RLock()
    if !h.running {
        h.mu.RUnlock()
        return ErrHubNotRunning
    }
    h.mu.RUnlock()
    
    // Get sender connection to extract session context
    sender, exists := h.registry.GetUserConnection(senderID)
    if !exists {
        return ErrSenderNotConnected
    }
    
    messageCtx := &MessageContext{
        Message:   message,
        SenderID:  senderID,
        SessionID: sender.GetSessionID(),
        Timestamp: time.Now(),
    }
    
    select {
    case h.messageChannel <- messageCtx:
        return nil
    default:
        return ErrMessageChannelFull
    }
}

// RegisterConnection queues a connection for registration
func (h *Hub) RegisterConnection(conn *websocket.Connection) error {
    h.mu.RLock()
    if !h.running {
        h.mu.RUnlock()
        return ErrHubNotRunning
    }
    h.mu.RUnlock()
    
    select {
    case h.registerChannel <- conn:
        return nil
    default:
        return ErrRegisterChannelFull
    }
}

// UnregisterConnection queues a connection for deregistration
func (h *Hub) UnregisterConnection(userID string) error {
    h.mu.RLock()
    if !h.running {
        h.mu.RUnlock()
        return ErrHubNotRunning
    }
    h.mu.RUnlock()
    
    select {
    case h.unregisterChannel <- userID:
        return nil
    default:
        return ErrUnregisterChannelFull
    }
}
```

### CRITICAL HUB LOOP (must follow):
```go
// run is the main hub processing loop
func (h *Hub) run(ctx context.Context) {
    defer log.Println("Hub processing stopped")
    
    for {
        select {
        case messageCtx := <-h.messageChannel:
            h.handleMessage(ctx, messageCtx)
            
        case conn := <-h.registerChannel:
            h.handleRegistration(conn)
            
        case userID := <-h.unregisterChannel:
            h.handleDeregistration(userID)
            
        case <-h.shutdownChannel:
            log.Println("Hub shutdown requested")
            return
            
        case <-ctx.Done():
            log.Println("Hub context cancelled")
            return
        }
    }
}

// handleMessage processes a message through the router
func (h *Hub) handleMessage(ctx context.Context, messageCtx *MessageContext) {
    // Set message metadata from context
    messageCtx.Message.FromUser = messageCtx.SenderID
    messageCtx.Message.SessionID = messageCtx.SessionID
    
    // Route the message
    if err := h.router.RouteMessage(ctx, messageCtx.Message); err != nil {
        log.Printf("Message routing failed for user %s in session %s: %v", 
            messageCtx.SenderID, messageCtx.SessionID, err)
        
        // Optionally send error response back to sender
        h.sendErrorToSender(messageCtx.SenderID, err)
    } else {
        log.Printf("Message routed successfully: type=%s from=%s session=%s", 
            messageCtx.Message.Type, messageCtx.SenderID, messageCtx.SessionID)
    }
}

// handleRegistration processes connection registration
func (h *Hub) handleRegistration(conn *websocket.Connection) {
    if err := h.registry.RegisterConnection(conn); err != nil {
        log.Printf("Connection registration failed for user %s: %v", 
            conn.GetUserID(), err)
        conn.Close()
    } else {
        log.Printf("Connection registered: user=%s role=%s session=%s", 
            conn.GetUserID(), conn.GetRole(), conn.GetSessionID())
    }
}

// handleDeregistration processes connection deregistration
func (h *Hub) handleDeregistration(userID string) {
    h.registry.UnregisterConnection(userID)
    log.Printf("Connection deregistered: user=%s", userID)
}

// sendErrorToSender sends an error message back to the sender
func (h *Hub) sendErrorToSender(senderID string, routingErr error) {
    sender, exists := h.registry.GetUserConnection(senderID)
    if !exists {
        return // Sender already disconnected
    }
    
    errorMsg := map[string]interface{}{
        "type": "system",
        "content": map[string]interface{}{
            "event": "message_error",
            "message": "Message could not be delivered",
            "error": routingErr.Error(),
        },
        "timestamp": time.Now(),
    }
    
    if err := sender.WriteJSON(errorMsg); err != nil {
        log.Printf("Failed to send error message to %s: %v", senderID, err)
    }
}
```

### WEBSOCKET HANDLER INTEGRATION (modify Step 2.3):
```go
// Add to WebSocket Handler in handleConnection method
func (h *Handler) handleConnection(conn *Connection, hub *Hub) {
    // ... existing code ...
    
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
            // Parse incoming message
            var message types.Message
            if err := json.Unmarshal(data, &message); err != nil {
                log.Printf("Invalid JSON from %s: %v", conn.GetUserID(), err)
                continue
            }
            
            // Send to hub for routing
            if err := hub.SendMessage(&message, conn.GetUserID()); err != nil {
                log.Printf("Failed to queue message from %s: %v", conn.GetUserID(), err)
            }
        }
    }
}
```

### ERROR TYPES (define exactly these):
```go
var (
    ErrHubAlreadyRunning     = errors.New("hub is already running")
    ErrHubNotRunning         = errors.New("hub is not running")
    ErrSenderNotConnected    = errors.New("sender not connected")
    ErrMessageChannelFull    = errors.New("message channel is full")
    ErrRegisterChannelFull   = errors.New("register channel is full")
    ErrUnregisterChannelFull = errors.New("unregister channel is full")
)
```

### SUCCESS CRITERIA:
**Architectural** (BLOCKING):
- [ ] No circular dependencies: clean integration layer
- [ ] Channel-based coordination following Go concurrency patterns
- [ ] Single hub goroutine prevents race conditions
- [ ] Clean integration with WebSocket Handler and Router

**Functional** (BLOCKING):
- [ ] Messages processed in order received
- [ ] Connection registration/deregistration coordinated properly
- [ ] Error handling doesn't crash hub processing
- [ ] Graceful shutdown with proper cleanup

**Technical** (WARNING):
- [ ] Hub processes messages without blocking (<1ms each)
- [ ] Channel buffering prevents backpressure during bursts
- [ ] Memory usage reasonable for classroom scale
- [ ] No goroutine leaks on shutdown

### INTEGRATION CONTRACTS:
**What WebSocket Handler provides:**
- Parsed messages sent to hub via SendMessage()
- Connection lifecycle events via RegisterConnection()/UnregisterConnection()
- Sender context (userID) for message processing

**What Router provides:**
- RouteMessage() implementation for message processing
- Error types for proper error handling
- Thread-safe operation for concurrent message routing

### READ ONLY THESE REFERENCES:
- switchboard-tech-specs.md lines 306-327 (goroutine architecture and channels)
- Step 2.3 WebSocket Handler implementation for integration points
- Step 3.1 Router interface for message processing

### IGNORE EVERYTHING ELSE
Do not read sections about database operations or session management. Focus only on message flow coordination.

### FILES TO CREATE:
- internal/hub/hub.go (hub implementation)
- internal/hub/errors.go (error definitions)
- internal/hub/hub_test.go (hub coordination tests)
- Modify internal/websocket/handler.go (add hub integration)