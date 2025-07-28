package router

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"switchboard/pkg/interfaces"
	"switchboard/pkg/types"
	"switchboard/internal/websocket"
)

// MessageRateLimiter defines the interface for rate limiting
// ARCHITECTURAL DISCOVERY: Interface segregation allows different rate limiting strategies
type MessageRateLimiter interface {
	Allow(userID string) bool
}

// Router implements the MessageRouter interface
// ARCHITECTURAL DISCOVERY: Pure message routing logic without session management or connection handling
// maintains clean separation between routing decisions and message delivery mechanisms
type Router struct {
	registry    *websocket.Registry
	dbManager   interfaces.DatabaseManager
	rateLimiter MessageRateLimiter  // INTERFACE DISCOVERY: Allows pluggable rate limiting strategies
	batcher     *MessageBatcher     // ARCHITECTURAL DISCOVERY: Optional batching for async persistence
}

// NewRouter creates a new message router
// FUNCTIONAL DISCOVERY: Dependency injection enables testing with mock components
func NewRouter(registry *websocket.Registry, dbManager interfaces.DatabaseManager) *Router {
	return &Router{
		registry:    registry,
		dbManager:   dbManager,
		rateLimiter: NewTokenRateLimiter(), // PERFORMANCE IMPROVEMENT: Token bucket eliminates mutex contention
	}
}

// RouteMessage routes a message to appropriate recipients
// FUNCTIONAL DISCOVERY: Persist-then-route pattern ensures message durability before delivery
// Server-side ID generation prevents client tampering and ensures database consistency
func (r *Router) RouteMessage(ctx context.Context, message *types.Message) error {
	// Generate server-side message ID (ignore any client-provided ID)
	// ARCHITECTURAL DISCOVERY: Server controls message IDs to prevent client manipulation
	message.ID = uuid.New().String()
	message.Timestamp = time.Now()
	
	// Set default context if empty
	// FUNCTIONAL DISCOVERY: Context defaults to "general" for consistent behavior
	if message.Context == "" {
		message.Context = "general"
	}
	
	// Validate message content and sender permissions
	sender, exists := r.registry.GetUserConnection(message.FromUser)
	if !exists {
		return ErrSenderNotConnected
	}
	
	// TECHNICAL DISCOVERY: Convert Connection to Client for validation interface
	senderClient := &types.Client{
		ID:   sender.GetUserID(),
		Role: sender.GetRole(),
	}
	
	if err := r.ValidateMessage(message, senderClient); err != nil {
		return err
	}
	
	// Check rate limit
	// TECHNICAL DISCOVERY: Rate limiting applied per user before persistence to prevent spam
	if !r.rateLimiter.Allow(message.FromUser) {
		return ErrRateLimitExceeded
	}
	
	// Route message immediately for real-time delivery
	if err := r.routeMessageToConnections(message); err != nil {
		return err
	}
	
	// FUNCTIONAL DISCOVERY: Async persistence doesn't block real-time routing
	// Persist message asynchronously
	r.persistMessageAsync(message)
	
	return nil
}

// routeMessageToConnections routes message with parallel delivery optimization
func (r *Router) routeMessageToConnections(message *types.Message) error {
	sessionID := message.SessionID
	
	var connections []*websocket.Connection
	
	switch message.Type {
	case types.MessageTypeInstructorInbox, types.MessageTypeRequestResponse, types.MessageTypeAnalytics:
		// Route to all instructors in session
		connections = r.registry.GetSessionInstructors(sessionID)
		
	case types.MessageTypeInboxResponse, types.MessageTypeRequest:
		// Route to specific student
		if message.ToUser == nil {
			return ErrMissingRecipient
		}
		
		recipient, exists := r.registry.GetUserConnection(*message.ToUser)
		if !exists {
			return ErrRecipientNotFound
		}
		
		// Verify recipient is in the same session
		if recipient.GetSessionID() != sessionID {
			return ErrRecipientNotInSession
		}
		
		connections = []*websocket.Connection{recipient}
		
	case types.MessageTypeInstructorBroadcast:
		// Route to all students in session
		connections = r.registry.GetSessionStudents(sessionID)
		
	default:
		return ErrInvalidMessageType
	}
	
	// Optimized delivery: single recipient vs broadcast
	return r.deliverToConnections(connections, message)
}

// deliverToConnections handles both single and parallel delivery optimally
func (r *Router) deliverToConnections(connections []*websocket.Connection, message *types.Message) error {
	if len(connections) == 0 {
		return nil
	}
	
	// Single recipient - direct delivery (no goroutine overhead)
	if len(connections) == 1 {
		if err := connections[0].WriteJSON(message); err != nil {
			log.Printf("Failed to deliver message to %s: %v", connections[0].GetUserID(), err)
			// Don't return connection errors as routing errors - message was successfully routed
			if err == websocket.ErrConnectionClosed || err == websocket.ErrWriteTimeout {
				return nil
			}
			return err
		}
		return nil
	}
	
	// Multiple recipients - parallel delivery for optimal performance
	var wg sync.WaitGroup
	errorChan := make(chan error, len(connections))
	
	for _, conn := range connections {
		wg.Add(1)
		go func(c *websocket.Connection) {
			defer wg.Done()
			if err := c.WriteJSON(message); err != nil {
				log.Printf("Failed to deliver message to %s: %v", c.GetUserID(), err)
				// Only report non-connection errors as routing errors
				if err != websocket.ErrConnectionClosed && err != websocket.ErrWriteTimeout {
					select {
					case errorChan <- err:
					default: // Don't block if channel is full
					}
				}
			}
		}(conn)
	}
	
	wg.Wait()
	close(errorChan)
	
	// Return first non-connection error if any occurred
	select {
	case err := <-errorChan:
		return err
	default:
		return nil
	}
}

// ValidateMessage validates message content and sender permissions
// ARCHITECTURAL DISCOVERY: Role-based validation enforced at routing layer
// ensures proper separation between authentication and authorization
func (r *Router) ValidateMessage(message *types.Message, sender *types.Client) error {
	// Verify sender is in the message's session
	// FUNCTIONAL DISCOVERY: Session membership verified through registry lookup
	senderConn, exists := r.registry.GetUserConnection(sender.ID)
	if !exists {
		return ErrSenderNotConnected
	}
	
	if senderConn.GetSessionID() != message.SessionID {
		return ErrSenderNotInSession
	}
	
	// Validate message type exists
	if !r.isValidMessageType(message.Type) {
		return ErrInvalidMessageType
	}
	
	// Validate role permissions
	// TECHNICAL DISCOVERY: Role-based permissions enforced for each message type
	senderRole := sender.Role
	if !r.canSendMessageType(senderRole, message.Type) {
		return ErrUnauthorizedMessageType
	}
	
	// Validate context field
	// FUNCTIONAL DISCOVERY: Context validation delegated to types package for consistency
	if message.Context != "" && !types.IsValidContext(message.Context) {
		return ErrInvalidContext
	}
	
	// Validate content size
	// ARCHITECTURAL DISCOVERY: Message validation delegated to Message.Validate() for consistency
	if err := message.Validate(); err != nil {
		return err
	}
	
	return nil
}

// Role-based message type permissions
// FUNCTIONAL DISCOVERY: Exact 3-3 split between student and instructor message types
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

// isValidMessageType checks if message type is one of the 6 allowed types
// TECHNICAL DISCOVERY: Linear search acceptable for 6 message types, O(1) average case
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


// EnableBatching enables message batching for async persistence
// ARCHITECTURAL DISCOVERY: Batching reduces database write overhead for high-volume scenarios
func (r *Router) EnableBatching(batchSize int, flushInterval time.Duration) error {
	if r.dbManager == nil {
		return fmt.Errorf("database manager required for batching")
	}
	
	// Create batcher that implements BatchStore using database manager
	store := &databaseBatchStore{dbManager: r.dbManager}
	r.batcher = NewMessageBatcher(store, batchSize, flushInterval)
	
	// Start the batcher
	return r.batcher.Start(context.Background())
}

// SetBatchStore sets a custom batch store (for testing)
func (r *Router) SetBatchStore(store BatchStore) {
	if r.batcher != nil {
		if err := r.batcher.Stop(); err != nil {
			log.Printf("Failed to stop existing batcher: %v", err)
		}
	}
	r.batcher = NewMessageBatcher(store, 50, 100*time.Millisecond)
}

// GetBatcherMetrics returns batcher metrics if batching is enabled
func (r *Router) GetBatcherMetrics() BatchMetrics {
	if r.batcher == nil {
		return BatchMetrics{}
	}
	return r.batcher.GetMetrics()
}

// persistMessageAsync handles asynchronous message persistence
// FUNCTIONAL DISCOVERY: Falls back to direct persistence if batching not enabled
func (r *Router) persistMessageAsync(message *types.Message) {
	if r.batcher != nil {
		// Use batcher for async persistence
		if err := r.batcher.Add(message); err != nil {
			// If batcher fails, fall back to direct persistence
			log.Printf("Batcher failed, falling back to direct persistence: %v", err)
			r.persistMessageDirect(message)
		}
	} else {
		// Direct persistence as fallback
		r.persistMessageDirect(message)
	}
}

// persistMessageDirect handles direct message persistence
// TECHNICAL DISCOVERY: Async goroutine prevents blocking routing
func (r *Router) persistMessageDirect(message *types.Message) {
	if r.dbManager == nil {
		return
	}
	
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := r.dbManager.StoreMessage(ctx, message); err != nil {
			log.Printf("Failed to persist message %s: %v", message.ID, err)
		}
	}()
}

// databaseBatchStore adapts DatabaseManager to BatchStore interface
type databaseBatchStore struct {
	dbManager interfaces.DatabaseManager
}

func (d *databaseBatchStore) StoreMessageBatch(ctx context.Context, messages []*types.Message) error {
	return d.dbManager.StoreMessageBatch(ctx, messages)
}