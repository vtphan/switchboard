package types

import (
	"encoding/json"
	"regexp"
)

// FUNCTIONAL DISCOVERY: Regex compiled once at package initialization
// for better performance in high-frequency validation scenarios
var (
	userIDRegex  = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	contextRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// Validate ensures the session meets all requirements
// ARCHITECTURAL DISCOVERY: Validation at type level ensures consistency
// across all components without duplicating validation logic
func (s *Session) Validate() error {
	if len(s.Name) < 1 || len(s.Name) > 200 {
		return ErrInvalidSessionName
	}
	if len(s.StudentIDs) == 0 {
		return ErrEmptyStudentList
	}
	if !IsValidUserID(s.CreatedBy) {
		return ErrInvalidCreatedBy
	}
	return nil
}

// Validate ensures the message meets all requirements
// FUNCTIONAL DISCOVERY: Context defaulting happens during validation
// to ensure consistent behavior across all message paths
func (m *Message) Validate() error {
	if !IsValidMessageType(m.Type) {
		return ErrInvalidMessageType
	}
	
	// FUNCTIONAL DISCOVERY: Default context applied here ensures
	// all messages have valid context even if client omits it
	if m.Context == "" {
		m.Context = "general"
	}
	
	if !IsValidContext(m.Context) {
		return ErrInvalidContext
	}
	
	// TECHNICAL DISCOVERY: Content size check requires marshaling
	// which adds overhead but ensures accurate byte count
	contentBytes, err := json.Marshal(m.Content)
	if err != nil {
		return ErrInvalidContent
	}
	if len(contentBytes) > 65536 { // 64KB = 65536 bytes
		return ErrContentTooLarge
	}
	
	return nil
}

// IsValidUserID checks if a user ID meets format requirements
// FUNCTIONAL DISCOVERY: 1-50 character limit prevents database issues
// and ensures reasonable display in UI components
func IsValidUserID(userID string) bool {
	if len(userID) < 1 || len(userID) > 50 {
		return false
	}
	return userIDRegex.MatchString(userID)
}

// IsValidMessageType checks if the message type is one of the allowed types
// ARCHITECTURAL DISCOVERY: Explicit validation prevents undefined message
// types from entering the routing system
func IsValidMessageType(msgType string) bool {
	switch msgType {
	case MessageTypeInstructorInbox,
		MessageTypeInboxResponse,
		MessageTypeRequest,
		MessageTypeRequestResponse,
		MessageTypeAnalytics,
		MessageTypeInstructorBroadcast:
		return true
	default:
		return false
	}
}

// IsValidContext checks if the context string meets requirements
// FUNCTIONAL DISCOVERY: Context validation ensures compatibility with
// client-defined semantic categorization systems
func IsValidContext(context string) bool {
	if len(context) < 1 || len(context) > 50 {
		return false
	}
	return contextRegex.MatchString(context)
}