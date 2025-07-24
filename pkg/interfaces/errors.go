package interfaces

import "errors"

// Common interface errors used across components
var (
	ErrSessionNotFound = errors.New("session not found")
	ErrUnauthorized    = errors.New("unauthorized access")
)