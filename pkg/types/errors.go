package types

import "errors"

// ARCHITECTURAL DISCOVERY: Specific error types enable proper error handling
// and user-friendly error messages throughout the system
var (
	ErrInvalidUserID       = errors.New("user ID must be 1-50 characters, alphanumeric + underscore/hyphen only")
	ErrInvalidSessionName  = errors.New("session name must be 1-200 characters")
	ErrEmptyStudentList    = errors.New("student list cannot be empty")
	ErrInvalidCreatedBy    = errors.New("created_by must be valid user ID")
	ErrInvalidMessageType  = errors.New("invalid message type")
	ErrInvalidContext      = errors.New("context must be 1-50 characters, alphanumeric + underscore/hyphen")
	ErrInvalidContent      = errors.New("invalid JSON content")
	ErrContentTooLarge     = errors.New("message content exceeds 64KB limit")
)