package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Message type constants
const (
	MessageTypeBroadcastToInstructors = "broadcast_to_instructors"
	MessageTypeDirectMessage          = "direct_message"
	MessageTypeBroadcastToStudents    = "broadcast_to_students"
	MessageTypeSystem                 = "system"
)

// Context constants
const (
	ContextQuestion     = "question"
	ContextSubmission   = "submission"
	ContextAnalytics    = "analytics"
	ContextResponse     = "response"
	ContextRequest      = "request"
	ContextPeerHelp     = "peer_help"
	ContextAnnouncement = "announcement"
	ContextInstruction  = "instruction"
	ContextEmergency    = "emergency"
	ContextGeneral      = "general"
)

// Session status constants
const (
	SessionStatusActive = "active"
	SessionStatusEnded  = "ended"
)

// Validation constraints
const (
	MaxSessionNameLength = 200
	MaxMessageContentSize = 64 * 1024 // 64KB in bytes
)

// Session represents a learning session with exactly the fields specified in tech specs (lines 91-97)
type Session struct {
	ID        string     `json:"id" db:"id"`
	Name      string     `json:"name" db:"name"`
	CreatedBy string     `json:"created_by" db:"created_by"`
	StartTime time.Time  `json:"start_time" db:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty" db:"end_time"`
	Status    string     `json:"status" db:"status"`
}

// Message represents a message with exactly the fields specified in tech specs (lines 458-490)
type Message struct {
	ID        string                 `json:"id" db:"id"`
	SessionID string                 `json:"session_id" db:"session_id"`
	Type      string                 `json:"type" db:"type"`
	Context   string                 `json:"context" db:"context"`
	FromUser  string                 `json:"from_user" db:"from_user"`
	ToUser    *string                `json:"to_user,omitempty" db:"to_user"`
	Content   map[string]interface{} `json:"content" db:"content"`
	Timestamp time.Time              `json:"timestamp" db:"timestamp"`
}

// NewSession creates a new Session with validation
func NewSession(id, name, createdBy string) (*Session, error) {
	session := &Session{
		ID:        id,
		Name:      name,
		CreatedBy: createdBy,
		StartTime: time.Now().UTC(),
		Status:    SessionStatusActive,
	}
	
	if err := session.Validate(); err != nil {
		return nil, err
	}
	
	return session, nil
}

// NewMessage creates a new Message with validation
func NewMessage(id, sessionID, msgType, context, fromUser string, content map[string]interface{}) (*Message, error) {
	message := &Message{
		ID:        id,
		SessionID: sessionID,
		Type:      msgType,
		Context:   context,
		FromUser:  fromUser,
		Content:   content,
		Timestamp: time.Now().UTC(),
	}
	
	if err := message.Validate(); err != nil {
		return nil, err
	}
	
	return message, nil
}

// NewDirectMessage creates a new direct message with ToUser set
func NewDirectMessage(id, sessionID, context, fromUser, toUser string, content map[string]interface{}) (*Message, error) {
	message := &Message{
		ID:        id,
		SessionID: sessionID,
		Type:      MessageTypeDirectMessage,
		Context:   context,
		FromUser:  fromUser,
		ToUser:    &toUser,
		Content:   content,
		Timestamp: time.Now().UTC(),
	}
	
	if err := message.Validate(); err != nil {
		return nil, err
	}
	
	return message, nil
}

// Validate validates the Session struct according to constraints
func (s *Session) Validate() error {
	// Check required fields
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("session ID is required")
	}
	
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("session name is required")
	}
	
	if strings.TrimSpace(s.CreatedBy) == "" {
		return errors.New("session created_by is required")
	}
	
	// Check name length (1-200 characters)
	if len(s.Name) > MaxSessionNameLength {
		return fmt.Errorf("session name too long: %d characters (max %d)", len(s.Name), MaxSessionNameLength)
	}
	
	// Check status enum
	if !isValidSessionStatus(s.Status) {
		return fmt.Errorf("invalid session status: %s", s.Status)
	}
	
	// Check that EndTime is after StartTime if set
	if s.EndTime != nil && s.EndTime.Before(s.StartTime) {
		return errors.New("session end time cannot be before start time")
	}
	
	return nil
}

// Validate validates the Message struct according to constraints
func (m *Message) Validate() error {
	// Check required fields
	if strings.TrimSpace(m.ID) == "" {
		return errors.New("message ID is required")
	}
	
	if strings.TrimSpace(m.SessionID) == "" {
		return errors.New("message session_id is required")
	}
	
	if strings.TrimSpace(m.FromUser) == "" {
		return errors.New("message from_user is required")
	}
	
	// Check message type enum
	if !isValidMessageType(m.Type) {
		return fmt.Errorf("invalid message type: %s", m.Type)
	}
	
	// Check context enum
	if !isValidContext(m.Context) {
		return fmt.Errorf("invalid message context: %s", m.Context)
	}
	
	// Check content size (serialize to JSON to get actual byte size)
	if m.Content != nil {
		contentBytes, err := json.Marshal(m.Content)
		if err != nil {
			return fmt.Errorf("invalid message content: %w", err)
		}
		if len(contentBytes) > MaxMessageContentSize {
			return fmt.Errorf("message content too large: %d bytes (max %d)", len(contentBytes), MaxMessageContentSize)
		}
	}
	
	return nil
}

// End ends the session by setting EndTime to now and status to ended
func (s *Session) End() error {
	now := time.Now().UTC()
	s.EndTime = &now
	s.Status = SessionStatusEnded
	return s.Validate()
}

// IsActive returns true if the session is currently active
func (s *Session) IsActive() bool {
	return s.Status == SessionStatusActive
}

// IsDirectMessage returns true if this is a direct message
func (m *Message) IsDirectMessage() bool {
	return m.Type == MessageTypeDirectMessage
}

// IsBroadcast returns true if this is a broadcast message (to instructors or students)
func (m *Message) IsBroadcast() bool {
	return m.Type == MessageTypeBroadcastToInstructors || m.Type == MessageTypeBroadcastToStudents
}

// Helper functions for validation

func isValidSessionStatus(status string) bool {
	switch status {
	case SessionStatusActive, SessionStatusEnded:
		return true
	default:
		return false
	}
}

func isValidMessageType(msgType string) bool {
	switch msgType {
	case MessageTypeBroadcastToInstructors, MessageTypeDirectMessage, MessageTypeBroadcastToStudents, MessageTypeSystem:
		return true
	default:
		return false
	}
}

func isValidContext(context string) bool {
	switch context {
	case ContextQuestion, ContextSubmission, ContextAnalytics, ContextResponse, ContextRequest,
		 ContextPeerHelp, ContextAnnouncement, ContextInstruction, ContextEmergency, ContextGeneral:
		return true
	default:
		return false
	}
}