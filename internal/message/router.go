package message

import (
	"fmt"

	"switchboard/internal/database"
	"switchboard/pkg/errors"
)

// MessageRouterImpl implements the MessageRouter interface
// This router implements the 3-message type routing system as specified in tech specs
type MessageRouterImpl struct {
	connectionProvider ConnectionProvider
	roleFilter         *RoleBasedFilter
}

// NewMessageRouter creates a new MessageRouter instance
func NewMessageRouter(connectionProvider ConnectionProvider, roleFilter *RoleBasedFilter) MessageRouter {
	return &MessageRouterImpl{
		connectionProvider: connectionProvider,
		roleFilter:         roleFilter,
	}
}

// GetRecipients determines who should receive a message based on its type
// Implements the 3-type routing system from tech specs:
// 1. broadcast_to_instructors - Only instructors receive the message
// 2. direct_message - Only the specific target user receives the message
// 3. broadcast_to_students - All students and instructors receive the message
func (r *MessageRouterImpl) GetRecipients(msg *database.Message) ([]Recipient, error) {
	if msg == nil {
		return nil, fmt.Errorf("%w: message cannot be nil", errors.ErrInvalidMessageData)
	}

	// Get all connected users
	connectedUsers, err := r.connectionProvider.GetConnectedUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to get connected users: %w", err)
	}

	switch msg.Type {
	case database.MessageTypeBroadcastToInstructors:
		return r.getBroadcastToInstructorsRecipients(connectedUsers), nil

	case database.MessageTypeDirectMessage:
		return r.getDirectMessageRecipients(msg, connectedUsers)

	case database.MessageTypeBroadcastToStudents:
		return r.getBroadcastToStudentsRecipients(connectedUsers), nil

	default:
		return nil, fmt.Errorf("%w: unsupported message type '%s'", errors.ErrInvalidMessageType, msg.Type)
	}
}

// getBroadcastToInstructorsRecipients returns only instructors from connected users
func (r *MessageRouterImpl) getBroadcastToInstructorsRecipients(connectedUsers []Recipient) []Recipient {
	var recipients []Recipient

	for _, user := range connectedUsers {
		if user.GetRole() == "instructor" {
			recipients = append(recipients, user)
		}
	}

	return recipients
}

// getDirectMessageRecipients returns only the target user for private messaging
// Updated privacy rules: instructors no longer have oversight of direct messages
func (r *MessageRouterImpl) getDirectMessageRecipients(msg *database.Message, connectedUsers []Recipient) ([]Recipient, error) {
	if msg.ToUser == nil {
		return nil, fmt.Errorf("%w: direct message must have to_user specified", errors.ErrInvalidMessageData)
	}

	targetUserID := *msg.ToUser
	var recipients []Recipient
	var targetFound bool

	// Find both sender and target user among connected users
	for _, user := range connectedUsers {
		userID := user.GetUserID()
		// Add sender (they should see their own message)
		if userID == msg.FromUser {
			recipients = append(recipients, user)
		}
		// Add target recipient
		if userID == targetUserID {
			recipients = append(recipients, user)
			targetFound = true
		}
	}

	// Validate that target user exists and is connected
	if !targetFound {
		return nil, fmt.Errorf("%w: target user '%s' not found or not connected", errors.ErrInvalidMessageData, targetUserID)
	}

	return recipients, nil
}

// getBroadcastToStudentsRecipients returns only students
// Updated privacy rules: instructors no longer receive broadcast_to_students messages
func (r *MessageRouterImpl) getBroadcastToStudentsRecipients(connectedUsers []Recipient) []Recipient {
	var recipients []Recipient

	for _, user := range connectedUsers {
		// Only students receive broadcast_to_students messages
		if user.GetRole() == "student" {
			recipients = append(recipients, user)
		}
	}

	return recipients
}
