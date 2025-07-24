package interfaces

import (
	"context"
	"switchboard/pkg/types"
)

// MessageRouter handles message routing between clients
// ARCHITECTURAL DISCOVERY: Routing logic abstracted from message delivery
// enables different routing strategies and simplifies testing with mocks
type MessageRouter interface {
	// RouteMessage routes a message to appropriate recipients
	// FUNCTIONAL DISCOVERY: Context enables timeout and cancellation during
	// database persistence and recipient delivery phases
	RouteMessage(ctx context.Context, message *types.Message) error

	// GetRecipients determines recipients for a message
	// ARCHITECTURAL DISCOVERY: Recipient calculation separated from delivery
	// enables testing routing logic without actual message delivery
	// FUNCTIONAL DISCOVERY: Returns Client slice for efficient iteration
	// during bulk message delivery operations
	GetRecipients(message *types.Message) ([]*types.Client, error)

	// ValidateMessage validates message content and permissions
	// FUNCTIONAL DISCOVERY: Sender context required for role-based validation
	// and rate limiting checks before message processing
	ValidateMessage(message *types.Message, sender *types.Client) error
}