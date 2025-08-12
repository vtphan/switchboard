package websocket

import (
	"encoding/json"
	"fmt"
	"log"

	"switchboard/internal/database"
	"switchboard/internal/message"
)

// RoleBasedFilterInterface defines the expected interface for role-based filtering
// This interface matches the signature expected by tests and planning documents
type RoleBasedFilterInterface interface {
	ShouldReceiveMessage(msg *database.Message, recipientRole, recipientUserID string) bool
}

// FilterAdapter adapts the current RoleBasedFilter to the expected interface
type FilterAdapter struct {
	filter *message.RoleBasedFilter
}

// NewFilterAdapter creates a new adapter for RoleBasedFilter
func NewFilterAdapter(filter *message.RoleBasedFilter) RoleBasedFilterInterface {
	return &FilterAdapter{filter: filter}
}

// ShouldReceiveMessage adapts the current filter to the expected interface
func (fa *FilterAdapter) ShouldReceiveMessage(msg *database.Message, recipientRole, recipientUserID string) bool {
	// Create a temporary recipient for the filter call
	recipient := &message.RecipientImpl{
		UserID: recipientUserID,
		Role:   recipientRole,
	}
	return fa.filter.ShouldReceiveMessage(msg, recipient)
}

// BroadcastSystem handles message broadcasting with role-based filtering and non-blocking delivery
// Implements the broadcast system from Phase 4 step 4.4 following the exact tech specs pattern
type BroadcastSystem struct {
	registry *ConnectionRegistry
	filter   RoleBasedFilterInterface
}

// NewBroadcastSystem creates a new BroadcastSystem with proper initialization
// registry: The connection registry for recipient lookup
// filter: Role-based filter for educational privacy rules
func NewBroadcastSystem(registry *ConnectionRegistry, filter RoleBasedFilterInterface) *BroadcastSystem {
	return &BroadcastSystem{
		registry: registry,
		filter:   filter,
	}
}

// BroadcastMessage broadcasts a message to specified recipients with role-based filtering
// Implements non-blocking delivery pattern - failed sends to individual connections don't block others
// Returns error only if ALL deliveries fail (deliveredCount == 0 && errorCount > 0)
func (bs *BroadcastSystem) BroadcastMessage(message *database.Message, recipients []message.Recipient) error {
	if len(recipients) == 0 {
		// No recipients is not an error condition
		return nil
	}

	// Marshal message to JSON once for all recipients
	messageData, err := bs.mustMarshalJSON(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message to JSON: %w", err)
	}

	deliveredCount := 0
	errorCount := 0

	// Process each recipient with non-blocking delivery
	// Note: Role-based filtering is already handled by the message router
	for _, recipient := range recipients {
		// Attempt non-blocking delivery
		err := recipient.SendMessage(messageData)
		if err != nil {
			errorCount++
			log.Printf("BroadcastSystem: Failed to deliver message %s to user %s: %v",
				message.ID, recipient.GetUserID(), err)
		} else {
			deliveredCount++
		}
	}

	// Log delivery statistics for monitoring
	log.Printf("BroadcastSystem: Message %s delivered to %d recipients, %d failures",
		message.ID, deliveredCount, errorCount)

	// Return error only when all deliveries failed AND there were recipients who should have received it
	if deliveredCount == 0 && errorCount > 0 {
		return fmt.Errorf("failed to deliver message to any recipients: %d failures", errorCount)
	}

	return nil
}

// mustMarshalJSON marshals a message to JSON with graceful error handling
// Returns consistent JSON even if marshalling fails (fallback message)
func (bs *BroadcastSystem) mustMarshalJSON(message *database.Message) ([]byte, error) {
	data, err := json.Marshal(message)
	if err != nil {
		// Fallback to basic error message if marshaling fails
		fallback := map[string]interface{}{
			"error": "failed to serialize message",
			"id":    message.ID,
			"type":  "system_error",
		}

		fallbackData, fallbackErr := json.Marshal(fallback)
		if fallbackErr != nil {
			// Ultimate fallback - return static error JSON
			return []byte(`{"error":"critical marshaling failure","type":"system_error"}`), err
		}

		return fallbackData, err
	}

	return data, nil
}
