package switchboard

import (
	"encoding/json"
	"time"
)

// Role represents the user's role in the system
type Role string

const (
	RoleStudent    Role = "student"
	RoleInstructor Role = "instructor"
)

// ConnectionState represents the current connection state
type ConnectionState string

const (
	ConnectionStateDisconnected ConnectionState = "disconnected"
	ConnectionStateConnecting   ConnectionState = "connecting"
	ConnectionStateConnected    ConnectionState = "connected"
	ConnectionStateError        ConnectionState = "error"
)

// Session represents session information
type Session struct {
	Active    bool   `json:"active"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	StartedBy string `json:"startedBy,omitempty"`
}

// Message represents a received message from the server
type Message struct {
	ID        string                 `json:"id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	Type      string                 `json:"type"`
	Context   string                 `json:"context,omitempty"`
	FromUser  string                 `json:"from_user,omitempty"`
	ToUser    string                 `json:"to_user,omitempty"`
	Content   map[string]interface{} `json:"content"`
	Timestamp time.Time              `json:"timestamp,omitempty"`
}

// OutgoingMessage represents a message being sent to the server
type OutgoingMessage struct {
	Type    string                 `json:"type"`
	Context string                 `json:"context"`
	ToUser  string                 `json:"to_user,omitempty"`
	Content map[string]interface{} `json:"content"`
}

// MessageContent is a convenient type for building message content
type MessageContent map[string]interface{}

// NewMessageContent creates a new message content with text
func NewMessageContent(text string) MessageContent {
	return MessageContent{"text": text}
}

// WithContext adds context (but it will be extracted and put at message level)
func (mc MessageContent) WithContext(context string) MessageContent {
	mc["context"] = context
	return mc
}

// WithField adds any field to the message content
func (mc MessageContent) WithField(key string, value interface{}) MessageContent {
	mc[key] = value
	return mc
}

// Configuration constants
const (
	MaxMessageSize       = 64 * 1024 // 64KB
	ReconnectBaseDelay   = 1000      // 1 second in milliseconds
	MaxReconnectDelay    = 30000     // 30 seconds in milliseconds
	ConnectionTimeout    = 10000     // 10 seconds in milliseconds
	RateLimitWindow      = 60000     // 1 minute in milliseconds
	RateLimitMaxMessages = 100       // messages per minute
)

// Helper functions for parsing JSON messages
func parseMessage(data []byte) (*Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	return &msg, err
}