package message

import "switchboard/internal/database"

// Recipient represents a user who should receive a message
// This interface abstracts the recipient concept for routing logic
type Recipient interface {
	GetUserID() string
	GetRole() string
	SendMessage(data []byte) error // Added for message delivery
}

// RecipientImpl is a concrete implementation of Recipient
type RecipientImpl struct {
	UserID string
	Role   string
}

// GetUserID returns the user ID of the recipient
func (r *RecipientImpl) GetUserID() string {
	return r.UserID
}

// GetRole returns the role of the recipient
func (r *RecipientImpl) GetRole() string {
	return r.Role
}

// SendMessage implements the Recipient interface (placeholder implementation)
func (r *RecipientImpl) SendMessage(data []byte) error {
	// This is a placeholder implementation for the concrete RecipientImpl
	// In practice, this would not be used for actual message delivery
	// Real message delivery would use Connection instances
	return nil
}

// NewRecipient creates a new Recipient with the given user ID and role
func NewRecipient(userID, role string) Recipient {
	return &RecipientImpl{
		UserID: userID,
		Role:   role,
	}
}

// MessageRouter interface defines routing behavior for messages
type MessageRouter interface {
	GetRecipients(msg *database.Message) ([]Recipient, error)
}

// ConnectionProvider interface abstracts access to connected users
// This interface allows the router to get information about connected users
// without directly depending on websocket layer
type ConnectionProvider interface {
	GetConnectedUsers() ([]Recipient, error)
}

// MessageProcessorInterface defines the message processing pipeline
// This enables dependency injection and testing with mock implementations
type MessageProcessorInterface interface {
	ProcessIncomingMessage(rawData []byte, senderID string) error
}

// BroadcastSystemInterface defines the message broadcasting interface
// This abstracts the broadcast system for dependency injection and testing
type BroadcastSystemInterface interface {
	BroadcastMessage(message *database.Message, recipients []Recipient) error
}
