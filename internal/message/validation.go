package message

import (
	"fmt"
	"strings"

	"switchboard/internal/database"
	"switchboard/pkg/errors"
)

// validateMessage validates a message according to the business rules specified in tech specs
// This function validates JSON-parsed message structure and field constraints
func validateMessage(msg *database.Message) error {
	if msg == nil {
		return errors.ErrInvalidMessageData
	}

	// Check required fields
	if strings.TrimSpace(msg.Type) == "" {
		return fmt.Errorf("%w: message type is required", errors.ErrInvalidMessageData)
	}

	// Note: FromUser is not validated here because it's set by the processor
	// based on the authenticated user, not provided by the client

	// Validate message type enum
	if !isValidMessageType(msg.Type) {
		return fmt.Errorf("%w: invalid message type '%s'", errors.ErrInvalidMessageType, msg.Type)
	}

	// Validate context if provided (will be set to default if empty)
	if msg.Context != "" && !isValidContext(msg.Context) {
		return fmt.Errorf("%w: invalid context '%s'", errors.ErrInvalidMessageData, msg.Context)
	}

	// For direct messages, ToUser must be specified
	if msg.Type == database.MessageTypeDirectMessage {
		if msg.ToUser == nil || strings.TrimSpace(*msg.ToUser) == "" {
			return fmt.Errorf("%w: to_user is required for direct messages", errors.ErrInvalidMessageData)
		}
	}

	// For broadcast messages, ToUser should not be specified
	if (msg.Type == database.MessageTypeBroadcastToInstructors || msg.Type == database.MessageTypeBroadcastToStudents) && msg.ToUser != nil {
		return fmt.Errorf("%w: to_user should not be specified for broadcast messages", errors.ErrInvalidMessageData)
	}

	// Content validation is handled by Message.Validate() method which checks size constraints
	// We just ensure it's not malformed JSON (already parsed at this point)

	return nil
}

// Helper functions for validation
func isValidMessageType(msgType string) bool {
	switch msgType {
	case database.MessageTypeBroadcastToInstructors, database.MessageTypeDirectMessage, database.MessageTypeBroadcastToStudents:
		return true
	default:
		return false
	}
}

func isValidContext(context string) bool {
	switch context {
	case database.ContextQuestion, database.ContextSubmission, database.ContextAnalytics,
		database.ContextResponse, database.ContextRequest, database.ContextPeerHelp,
		database.ContextAnnouncement, database.ContextInstruction, database.ContextEmergency,
		database.ContextGeneral:
		return true
	default:
		return false
	}
}
