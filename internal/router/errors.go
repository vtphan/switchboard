package router

import "errors"

// Router-specific error types as defined in Phase 3.1 specifications
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