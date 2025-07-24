package types

import (
	"time"
)

// ARCHITECTURAL DISCOVERY: Message type constants defined exactly as specified
// to ensure compatibility with all routing logic across the system
const (
	MessageTypeInstructorInbox     = "instructor_inbox"
	MessageTypeInboxResponse       = "inbox_response"
	MessageTypeRequest             = "request"
	MessageTypeRequestResponse     = "request_response"
	MessageTypeAnalytics           = "analytics"
	MessageTypeInstructorBroadcast = "instructor_broadcast"
)

// Session represents an educational session
// FUNCTIONAL DISCOVERY: Session is immutable after creation except for end_time and status
// This prevents race conditions and simplifies session validation caching
type Session struct {
	ID         string    `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	CreatedBy  string    `json:"created_by" db:"created_by"`
	StudentIDs []string  `json:"student_ids" db:"student_ids"`
	StartTime  time.Time `json:"start_time" db:"start_time"`
	EndTime    *time.Time `json:"end_time,omitempty" db:"end_time"`
	Status     string    `json:"status" db:"status"`
}

// Message represents a communication message
// ARCHITECTURAL DISCOVERY: Content as map[string]interface{} allows flexible
// message payloads while maintaining JSON compatibility for WebSocket transport
type Message struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id"`
	Type      string                 `json:"type"`
	Context   string                 `json:"context"`
	FromUser  string                 `json:"from_user"`
	ToUser    *string                `json:"to_user,omitempty"`
	Content   map[string]interface{} `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
}

// Client represents a connected WebSocket client
// FUNCTIONAL DISCOVERY: SendChannel must be buffered to prevent blocking
// during message broadcasts in classroom scenarios with 20-50 students
type Client struct {
	ID            string          `json:"id"`
	Role          string          `json:"role"`
	SessionID     string          `json:"session_id"`
	SendChannel   chan Message    `json:"-"` // TECHNICAL DISCOVERY: json:"-" prevents channel serialization
	LastHeartbeat time.Time       `json:"last_heartbeat"`
	MessageCount  int             `json:"message_count"`
	WindowStart   time.Time       `json:"window_start"`
	CleanedUp     bool            `json:"cleaned_up"`
}

// ConnectionManager manages client connections
// ARCHITECTURAL DISCOVERY: Three-level mapping structure enables O(1) lookups
// for both global user lookup and session-specific instructor/student lists
type ConnectionManager struct {
	GlobalConnections   map[string]*Client            `json:"global_connections"`
	SessionInstructors  map[string]map[string]*Client `json:"session_instructors"`
	SessionStudents     map[string]map[string]*Client `json:"session_students"`
}