package websocket

import "errors"

// Connection-related errors
var (
	ErrConnectionClosed = errors.New("connection closed")
	ErrWriteTimeout     = errors.New("write timeout after 5 seconds")
	ErrInvalidJSON      = errors.New("invalid JSON data")
)

// Registry-related errors
var (
	ErrNilConnection              = errors.New("connection cannot be nil")
	ErrConnectionNotAuthenticated = errors.New("connection must be authenticated before registration")
)

// Handler-related errors
var (
	ErrInvalidParameters = errors.New("invalid connection parameters")
	ErrSessionValidation = errors.New("session validation failed")
	ErrConnectionSetup   = errors.New("connection setup failed")
)