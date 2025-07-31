package errors

import "errors"

// Standard error types for internal operations
var (
    ErrNoActiveSession      = errors.New("no active session")
    ErrSessionAlreadyActive = errors.New("session already active")
    ErrConnectionNotFound   = errors.New("connection not found")
    ErrChannelFull         = errors.New("send channel full")
    ErrInvalidMessageType  = errors.New("invalid message type")
    ErrRateLimitExceeded   = errors.New("rate limit exceeded")
    
    // Validation errors
    ErrInvalidSessionData   = errors.New("invalid session data")
    ErrInvalidMessageData   = errors.New("invalid message data")
    ErrContentTooLarge     = errors.New("content too large")
)