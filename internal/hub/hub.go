package hub

import (
	"context"
	"log"
	"sync"
	"time"

	"switchboard/pkg/types"
	"switchboard/internal/websocket"
	"switchboard/internal/router"
)

// Hub coordinates message routing and connection management
// ARCHITECTURAL DISCOVERY: Central coordination point for all message flow
// maintains clean separation between WebSocket handling and message routing
type Hub struct {
	// Channels for coordination
	// FUNCTIONAL DISCOVERY: Buffered channels prevent blocking during message bursts
	messageChannel    chan *MessageContext // TECHNICAL DISCOVERY: 1000 buffer handles classroom message bursts
	registerChannel   chan *websocket.Connection // 100 buffer for connection lifecycle events
	unregisterChannel chan string // userID - smaller buffer for deregistration events
	shutdownChannel   chan struct{} // Unbuffered for immediate shutdown signaling
	
	// Components
	// ARCHITECTURAL DISCOVERY: Dependency injection enables clean testing with mocks
	registry *websocket.Registry
	router   *router.Router
	
	// State
	// TECHNICAL DISCOVERY: RWMutex allows concurrent reads of running state
	running bool
	mu      sync.RWMutex
}

// MessageContext wraps a message with sender information
// FUNCTIONAL DISCOVERY: Context preservation ensures proper message attribution
// and enables session-scoped routing decisions
type MessageContext struct {
	Message    *types.Message
	SenderID   string
	SessionID  string
	Timestamp  time.Time
}

// NewHub creates a new hub
// ARCHITECTURAL DISCOVERY: Constructor pattern with dependency injection
// enables clean testing and component isolation
func NewHub(registry *websocket.Registry, router *router.Router) *Hub {
	return &Hub{
		// TECHNICAL DISCOVERY: Channel buffer sizes based on classroom scale testing
		messageChannel:    make(chan *MessageContext, 1000), // Buffer for message bursts
		registerChannel:   make(chan *websocket.Connection, 100), // Connection lifecycle events
		unregisterChannel: make(chan string, 100), // Deregistration events
		shutdownChannel:   make(chan struct{}), // Immediate shutdown signaling
		registry:         registry,
		router:          router,
		running:         false,
	}
}

// Start begins hub processing
// FUNCTIONAL DISCOVERY: Single hub goroutine prevents race conditions
// while maintaining high throughput message processing
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
	// ARCHITECTURAL DISCOVERY: Single goroutine coordination prevents race conditions
	go h.run(ctx)
	
	return nil
}

// Stop gracefully shuts down the hub
// TECHNICAL DISCOVERY: Graceful shutdown ensures proper channel cleanup
// and prevents goroutine leaks in production deployment
func (h *Hub) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.running {
		return ErrHubNotRunning
	}
	h.running = false
	
	log.Println("Stopping message hub...")
	
	// TECHNICAL DISCOVERY: Safe channel close using select to prevent panic
	select {
	case <-h.shutdownChannel:
		// Channel already closed
	default:
		close(h.shutdownChannel)
	}
	
	return nil
}

// SendMessage queues a message for routing with context-aware timeout
// CONTEXT FIX: Add timeout to prevent indefinite blocking when channel is full
func (h *Hub) SendMessage(message *types.Message, senderID string) error {
	return h.SendMessageWithContext(context.Background(), message, senderID)
}

// SendMessageWithContext queues a message for routing with context support
// FUNCTIONAL DISCOVERY: Message context extraction ensures proper routing
// even when sender information is not embedded in message payload
func (h *Hub) SendMessageWithContext(ctx context.Context, message *types.Message, senderID string) error {
	h.mu.RLock()
	if !h.running {
		h.mu.RUnlock()
		return ErrHubNotRunning
	}
	h.mu.RUnlock()
	
	// Get sender connection to extract session context
	// ARCHITECTURAL DISCOVERY: Registry lookup provides session context
	// without coupling message structure to connection details
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
	
	// CONTEXT FIX: Context-aware channel send with timeout prevents indefinite blocking
	select {
	case h.messageChannel <- messageCtx:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return ErrMessageChannelFull
	}
}

// RegisterConnection queues a connection for registration with context support
// CONTEXT FIX: Add timeout to prevent indefinite blocking during registration
func (h *Hub) RegisterConnection(conn *websocket.Connection) error {
	return h.RegisterConnectionWithContext(context.Background(), conn)
}

// RegisterConnectionWithContext queues a connection for registration with context support
// FUNCTIONAL DISCOVERY: Asynchronous registration prevents blocking
// WebSocket handler during connection establishment
func (h *Hub) RegisterConnectionWithContext(ctx context.Context, conn *websocket.Connection) error {
	h.mu.RLock()
	if !h.running {
		h.mu.RUnlock()
		return ErrHubNotRunning
	}
	h.mu.RUnlock()
	
	// CONTEXT FIX: Context-aware channel send with timeout
	select {
	case h.registerChannel <- conn:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return ErrRegisterChannelFull
	}
}

// UnregisterConnection queues a connection for deregistration with context support
// CONTEXT FIX: Add timeout to prevent indefinite blocking during deregistration
func (h *Hub) UnregisterConnection(userID string) error {
	return h.UnregisterConnectionWithContext(context.Background(), userID)
}

// UnregisterConnectionWithContext queues a connection for deregistration with context support
// ARCHITECTURAL DISCOVERY: User ID-based deregistration enables cleanup
// even when connection object is no longer available
func (h *Hub) UnregisterConnectionWithContext(ctx context.Context, userID string) error {
	h.mu.RLock()
	if !h.running {
		h.mu.RUnlock()
		return ErrHubNotRunning
	}
	h.mu.RUnlock()
	
	// CONTEXT FIX: Context-aware channel send with timeout
	select {
	case h.unregisterChannel <- userID:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return ErrUnregisterChannelFull
	}
}

// run is the main hub processing loop
// TECHNICAL DISCOVERY: Single select loop handles all coordination
// preventing race conditions while maintaining high throughput
func (h *Hub) run(ctx context.Context) {
	defer log.Println("Hub processing stopped")
	
	for {
		select {
		case messageCtx := <-h.messageChannel:
			// FUNCTIONAL DISCOVERY: Message processing continues despite individual failures
			h.handleMessage(ctx, messageCtx)
			
		case conn := <-h.registerChannel:
			// ARCHITECTURAL DISCOVERY: Registration coordination through hub
			// ensures consistent state between registry and connection tracking
			h.handleRegistration(conn)
			
		case userID := <-h.unregisterChannel:
			// FUNCTIONAL DISCOVERY: Deregistration by user ID enables cleanup
			// even when connection is already closed or corrupted
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
// FUNCTIONAL DISCOVERY: Message context restoration ensures proper routing
// even when message doesn't contain complete sender information
func (h *Hub) handleMessage(ctx context.Context, messageCtx *MessageContext) {
	// Set message metadata from context
	// ARCHITECTURAL DISCOVERY: Message enrichment at hub level
	// keeps message struct clean while ensuring routing context
	messageCtx.Message.FromUser = messageCtx.SenderID
	messageCtx.Message.SessionID = messageCtx.SessionID
	
	// Route the message
	// TECHNICAL DISCOVERY: Router errors logged but don't crash hub
	// ensuring system resilience during partial failures
	if err := h.router.RouteMessage(ctx, messageCtx.Message); err != nil {
		log.Printf("Message routing failed for user %s in session %s: %v", 
			messageCtx.SenderID, messageCtx.SessionID, err)
		
		// Optionally send error response back to sender
		// FUNCTIONAL DISCOVERY: Error feedback improves user experience
		// during message delivery failures
		h.sendErrorToSender(messageCtx.SenderID, err)
	} else {
		log.Printf("Message routed successfully: type=%s from=%s session=%s", 
			messageCtx.Message.Type, messageCtx.SenderID, messageCtx.SessionID)
	}
}

// handleRegistration processes connection registration
// ARCHITECTURAL DISCOVERY: Registration error handling ensures
// failed connections are cleaned up properly
func (h *Hub) handleRegistration(conn *websocket.Connection) {
	// TECHNICAL DISCOVERY: Nil connection check prevents panic
	// during hub processing of invalid registration requests
	if conn == nil {
		log.Printf("Attempted to register nil connection")
		return
	}
	
	if err := h.registry.RegisterConnection(conn); err != nil {
		log.Printf("Connection registration failed for user %s: %v", 
			conn.GetUserID(), err)
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("Failed to close connection after registration failure: %v", closeErr)
		}
	} else {
		log.Printf("Connection registered: user=%s role=%s session=%s", 
			conn.GetUserID(), conn.GetRole(), conn.GetSessionID())
	}
}

// handleDeregistration processes connection deregistration
// FUNCTIONAL DISCOVERY: Connection-based deregistration ensures proper cleanup
// by removing the exact connection that was registered
func (h *Hub) handleDeregistration(userID string) {
	// TECHNICAL DISCOVERY: Get connection first to pass to UnregisterConnection
	// which requires the connection instance for race-free removal
	if conn, exists := h.registry.GetUserConnection(userID); exists {
		h.registry.UnregisterConnection(conn)
		log.Printf("Connection deregistered: user=%s", userID)
	} else {
		log.Printf("Connection already deregistered: user=%s", userID)
	}
}

// sendErrorToSender sends an error message back to the sender
// TECHNICAL DISCOVERY: Error feedback mechanism using system message format
// provides user-friendly error reporting without exposing internal details
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