package websocket

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"switchboard/internal/database"
	"switchboard/internal/message"
)

// SessionBroadcaster implements the SystemBroadcaster interface
// It acts as an adapter between SessionLifecycle and BroadcastSystem
type SessionBroadcaster struct {
	broadcastSystem    *BroadcastSystem
	connectionProvider ConnectionProvider
}

// ConnectionProvider interface for getting connected users
type ConnectionProvider interface {
	GetConnectedUsers() ([]message.Recipient, error)
}

// NewSessionBroadcaster creates a new SessionBroadcaster
func NewSessionBroadcaster(broadcastSystem *BroadcastSystem, connectionProvider ConnectionProvider) *SessionBroadcaster {
	return &SessionBroadcaster{
		broadcastSystem:    broadcastSystem,
		connectionProvider: connectionProvider,
	}
}

// BroadcastSessionStarted implements the SystemBroadcaster interface
func (sb *SessionBroadcaster) BroadcastSessionStarted(sessionID, sessionName, startedBy string, startTime time.Time) error {
	// Get all connected users
	recipients, err := sb.connectionProvider.GetConnectedUsers()
	if err != nil {
		return err
	}

	// Create system message with session_started event
	systemMessage := &database.Message{
		ID:        generateMessageID(),
		Type:      "system",
		FromUser:  "system",
		SessionID: sessionID,
		Context:   "system",
		Content: map[string]interface{}{
			"event":        "session_started",
			"session_id":   sessionID,
			"session_name": sessionName,
			"started_by":   startedBy,
		},
		Timestamp: startTime,
	}

	// Broadcast to all connected users
	return sb.broadcastSystem.BroadcastMessage(systemMessage, recipients)
}

// BroadcastSessionEnded implements the SystemBroadcaster interface
func (sb *SessionBroadcaster) BroadcastSessionEnded(sessionID, endedBy string, endTime time.Time) error {
	// Get all connected users
	recipients, err := sb.connectionProvider.GetConnectedUsers()
	if err != nil {
		return err
	}

	// Create system message with session_ended event
	systemMessage := &database.Message{
		ID:        generateMessageID(),
		Type:      "system",
		FromUser:  "system",
		SessionID: sessionID,
		Context:   "system",
		Content: map[string]interface{}{
			"event":      "session_ended",
			"session_id": sessionID,
			"ended_by":   endedBy,
		},
		Timestamp: endTime,
	}

	// Broadcast to all connected users
	return sb.broadcastSystem.BroadcastMessage(systemMessage, recipients)
}

// generateMessageID generates a unique ID for system messages
func generateMessageID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}