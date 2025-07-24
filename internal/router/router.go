package router

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"switchboard/pkg/interfaces"
	"switchboard/pkg/types"
	"switchboard/internal/websocket"
)

// Router implements the MessageRouter interface
// ARCHITECTURAL DISCOVERY: Pure message routing logic without session management or connection handling
// maintains clean separation between routing decisions and message delivery mechanisms
type Router struct {
	registry    *websocket.Registry
	dbManager   interfaces.DatabaseManager
	rateLimiter *RateLimiter
}

// NewRouter creates a new message router
// FUNCTIONAL DISCOVERY: Dependency injection enables testing with mock components
func NewRouter(registry *websocket.Registry, dbManager interfaces.DatabaseManager) *Router {
	return &Router{
		registry:    registry,
		dbManager:   dbManager,
		rateLimiter: NewRateLimiter(),
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
	
	// Persist message first (persist-then-route pattern)
	// ARCHITECTURAL DISCOVERY: Database persistence must complete before routing to prevent audit gaps
	if r.dbManager != nil {
		if err := r.dbManager.StoreMessage(ctx, message); err != nil {
			return fmt.Errorf("failed to persist message: %w", err)
		}
	}
	
	// Get recipients based on message type
	recipients, err := r.GetRecipients(message)
	if err != nil {
		return err
	}
	
	// Deliver to all recipients
	// FUNCTIONAL DISCOVERY: Continue delivery to other recipients even if one fails
	// TECHNICAL DISCOVERY: Need to get actual connections for message delivery
	for _, recipientClient := range recipients {
		if conn, exists := r.registry.GetUserConnection(recipientClient.ID); exists {
			if err := conn.WriteJSON(message); err != nil {
				// Log error but continue delivery to other recipients
				log.Printf("Failed to deliver message to %s: %v", recipientClient.ID, err)
			}
		}
	}
	
	return nil
}

// GetRecipients determines recipients based on message type
// FUNCTIONAL DISCOVERY: Three distinct routing patterns based on message type and role relationships
// ARCHITECTURAL DISCOVERY: Interface requires Client slice, conversion from Connection slice needed
func (r *Router) GetRecipients(message *types.Message) ([]*types.Client, error) {
	sessionID := message.SessionID
	
	switch message.Type {
	case types.MessageTypeInstructorInbox, types.MessageTypeRequestResponse, types.MessageTypeAnalytics:
		// Route to all instructors in session
		// ARCHITECTURAL DISCOVERY: Broadcast pattern for messages that all instructors should see
		connections := r.registry.GetSessionInstructors(sessionID)
		return r.convertConnectionsToClients(connections), nil
		
	case types.MessageTypeInboxResponse, types.MessageTypeRequest:
		// Route to specific student
		// FUNCTIONAL DISCOVERY: Direct messaging requires recipient validation within session
		if message.ToUser == nil {
			return nil, ErrMissingRecipient
		}
		
		recipient, exists := r.registry.GetUserConnection(*message.ToUser)
		if !exists {
			return nil, ErrRecipientNotFound
		}
		
		// Verify recipient is in the same session
		// ARCHITECTURAL DISCOVERY: Session boundaries enforced at routing level
		if recipient.GetSessionID() != sessionID {
			return nil, ErrRecipientNotInSession
		}
		
		return r.convertConnectionsToClients([]*websocket.Connection{recipient}), nil
		
	case types.MessageTypeInstructorBroadcast:
		// Route to all students in session
		// FUNCTIONAL DISCOVERY: Instructor broadcast pattern for classroom announcements
		connections := r.registry.GetSessionStudents(sessionID)
		return r.convertConnectionsToClients(connections), nil
		
	default:
		return nil, ErrInvalidMessageType
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

// convertConnectionsToClients converts websocket connections to client representations
// ARCHITECTURAL DISCOVERY: Interface abstraction requires type conversion for clean boundaries
func (r *Router) convertConnectionsToClients(connections []*websocket.Connection) []*types.Client {
	clients := make([]*types.Client, len(connections))
	for i, conn := range connections {
		clients[i] = &types.Client{
			ID:   conn.GetUserID(),
			Role: conn.GetRole(),
		}
	}
	return clients
}