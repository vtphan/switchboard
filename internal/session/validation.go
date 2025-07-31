package session

import (
	"errors"

	"switchboard/internal/database"
)

// ValidateSession checks complete session data integrity.
// This function performs comprehensive validation of all session fields including
// required fields, name constraints, status validation, and time consistency.
func ValidateSession(session *database.Session) error {
	// Handle nil session
	if session == nil {
		return errors.New("session cannot be nil")
	}

	// Validate required fields
	if session.ID == "" {
		return errors.New("session ID required")
	}

	if session.CreatedBy == "" {
		return errors.New("session creator required")
	}

	// Check if StartTime is zero value
	if session.StartTime.IsZero() {
		return errors.New("session start time required")
	}

	// Validate session name using dedicated function
	if err := ValidateSessionName(session.Name); err != nil {
		return err
	}

	// Validate session status using dedicated function
	if err := ValidateSessionStatus(session.Status); err != nil {
		return err
	}

	// Validate time consistency: EndTime must be after StartTime if set
	if session.EndTime != nil && session.EndTime.Before(session.StartTime) {
		return errors.New("end time cannot be before start time")
	}

	return nil
}

// ValidateSessionName checks name constraints (1-200 characters).
// This function validates that session names meet the length requirements
// without performing any trimming operations on the input.
func ValidateSessionName(name string) error {
	// Check length constraints: must be 1-200 characters
	if len(name) == 0 || len(name) > database.MaxSessionNameLength {
		return errors.New("session name must be 1-200 characters")
	}

	return nil
}

// ValidateSessionStatus checks status enum ("active"/"ended").
// This function ensures that session status values match the allowed
// constants with case-sensitive validation.
func ValidateSessionStatus(status string) error {
	// Check against valid status constants
	if status != database.SessionStatusActive && status != database.SessionStatusEnded {
		return errors.New("status must be 'active' or 'ended'")
	}

	return nil
}