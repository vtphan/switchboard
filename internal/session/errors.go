package session

import "errors"

// Session management error types - exactly as specified in Phase 4.1
var (
	ErrInvalidSessionName  = errors.New("session name must be 1-200 characters")
	ErrInvalidCreatedBy    = errors.New("created_by must be valid user ID")
	ErrEmptyStudentList    = errors.New("student list cannot be empty")
	ErrInvalidStudentID    = errors.New("invalid student ID format")
	ErrSessionNotFound     = errors.New("session not found")
	ErrSessionEnded        = errors.New("session has ended")
	ErrSessionAlreadyEnded = errors.New("session is already ended")
	ErrUnauthorized        = errors.New("user not authorized for this session")
	ErrInvalidRole         = errors.New("invalid role: must be 'student' or 'instructor'")
)