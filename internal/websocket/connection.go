package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"switchboard/pkg/config"
	pkgErrors "switchboard/pkg/errors"
)

// WebSocketConn interface abstracts the gorilla websocket connection for testing
type WebSocketConn interface {
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
	SetWriteDeadline(t time.Time) error
	SetReadLimit(limit int64)
	SetPongHandler(h func(string) error)
}

// RegistryNotifier defines the interface for notifying registry of heartbeat updates
type RegistryNotifier interface {
	UpdateHeartbeat(userID string)
}

// Connection represents a WebSocket connection with single-writer pattern
type Connection struct {
	userID   string
	role     string
	conn     WebSocketConn   // Interface for testing
	sendCh   chan []byte     // Buffered: ConnectionSendBufferSize (100)
	closeCh  chan struct{}
	lastSeen atomic.Value    // time.Time
	mu       sync.RWMutex    // Protects userID and role
	registry RegistryNotifier // For notifying registry of heartbeat updates
}

// ConnectionInterface defines the interface for WebSocket connections
type ConnectionInterface interface {
	WriteJSON(v interface{}) error
	Close() error
	GetUserID() string
	GetRole() string
	SetCredentials(username, role string) error
	UpdateActivity()
	GetLastSeen() time.Time
	SendMessage(data []byte) error // Extended interface for message delivery
	SendCloseMessage(reason string) error // Send close frame with reason
}

// NewConnection creates a new WebSocket connection wrapper with single-writer pattern
func NewConnection(conn WebSocketConn) *Connection {
	c := &Connection{
		conn:    conn,
		sendCh:  make(chan []byte, config.ConnectionSendBufferSize), // Exactly 100
		closeCh: make(chan struct{}),
	}
	c.lastSeen.Store(time.Now())
	return c
}

// SetRegistry sets the registry notifier for heartbeat updates
func (c *Connection) SetRegistry(registry RegistryNotifier) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.registry = registry
}

// WriteJSON marshals the given object to JSON and sends it via the send channel
// Uses non-blocking send to prevent blocking on slow connections
func (c *Connection) WriteJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}

	select {
	case c.sendCh <- data:
		return nil
	case <-c.closeCh:
		return errors.New("connection closed")
	default:
		return pkgErrors.ErrChannelFull
	}
}

// SendMessage implements the Recipient interface for message delivery
// This allows Connection to be used directly by the message routing system
func (c *Connection) SendMessage(data []byte) error {
	select {
	case c.sendCh <- data:
		return nil
	case <-c.closeCh:
		return errors.New("connection closed")
	default:
		return pkgErrors.ErrChannelFull
	}
}

// SendCloseMessage sends a WebSocket close frame with the specified reason
// This is used during graceful shutdown to inform clients of server shutdown
func (c *Connection) SendCloseMessage(reason string) error {
	select {
	case <-c.closeCh:
		return errors.New("connection already closed")
	default:
		// Use channel to maintain single-writer pattern
		closeMessage := websocket.FormatCloseMessage(websocket.CloseGoingAway, reason)
		select {
		case c.sendCh <- closeMessage:
			return nil
		case <-time.After(1 * time.Second):
			// Channel blocked or closed - graceful degradation
			return nil
		}
	}
}

// SetCredentials validates and sets user credentials with thread safety
func (c *Connection) SetCredentials(username, role string) error {
	if username == "" || role == "" {
		return errors.New("username and role required")
	}

	if role != "student" && role != "instructor" {
		return errors.New("role must be 'student' or 'instructor'")
	}

	c.mu.Lock()
	c.userID = username
	c.role = role
	c.mu.Unlock()
	
	// Call UpdateActivity outside the lock to avoid deadlock
	c.UpdateActivity()

	return nil
}

// GetUserID returns the user ID with thread-safe read access
func (c *Connection) GetUserID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userID
}

// GetRole returns the user role with thread-safe read access
func (c *Connection) GetRole() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.role
}

// UpdateActivity updates the last seen timestamp atomically and notifies registry
func (c *Connection) UpdateActivity() {
	c.lastSeen.Store(time.Now())
	
	// Notify registry of heartbeat update if registry is set
	c.mu.RLock()
	registry := c.registry
	userID := c.userID
	c.mu.RUnlock()
	
	if registry != nil && userID != "" {
		registry.UpdateHeartbeat(userID)
	}
}

// GetLastSeen returns the last activity timestamp atomically
func (c *Connection) GetLastSeen() time.Time {
	return c.lastSeen.Load().(time.Time)
}

// CloseWithCode sends a close frame with specific code and reason before closing
func (c *Connection) CloseWithCode(code int, reason string) error {
	// Send close message through the write channel to maintain single-writer pattern
	closeMessage := websocket.FormatCloseMessage(code, reason)
	select {
	case c.sendCh <- closeMessage:
		// Give time for close message to be sent
		time.Sleep(100 * time.Millisecond)
	default:
		// Channel full or closed, proceed anyway
	}
	return c.Close()
}

// Close performs idempotent cleanup of connection resources
func (c *Connection) Close() error {
	select {
	case <-c.closeCh:
		return nil // Already closed
	default:
		close(c.closeCh)
		return c.conn.Close()
	}
}

// Start launches the writeLoop and readLoop goroutines for the connection
// Called by connection handler after successful authentication
func (c *Connection) Start(ctx context.Context, messageHandler func([]byte, string) error) {
	go c.writeLoop(ctx)
	go c.readLoop(ctx, messageHandler)
}

// writeLoop handles all WebSocket writes (single-writer pattern)
// Processes messages from sendCh and sends heartbeat pings
func (c *Connection) writeLoop(ctx context.Context) {
	ticker := time.NewTicker(config.HeartbeatInterval) // 30 seconds
	defer ticker.Stop()

	for {
		select {
		case data := <-c.sendCh:
			_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			
			// Check if this is a close message (raw bytes from FormatCloseMessage)
			if len(data) >= 2 && data[0] == 0x03 && data[1] == 0xe9 { // CloseGoingAway code
				if err := c.conn.WriteMessage(websocket.CloseMessage, data); err != nil {
					log.Printf("Close message write error for user %s: %v", c.GetUserID(), err)
				}
				return // Close message sent - terminate writeLoop
			}
			
			// Regular message
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("Write error for user %s: %v", c.GetUserID(), err)
				return // Write error - connection dead
			}
			c.UpdateActivity()

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Printf("Ping failed for user %s: %v", c.GetUserID(), err)
				return // Ping failed - connection dead
			}
			c.UpdateActivity()

		case <-c.closeCh:
			return // Clean shutdown

		case <-ctx.Done():
			return // Context cancelled
		}
	}
}

// readLoop processes incoming WebSocket messages and updates activity
// Handles pong responses for heartbeat monitoring
func (c *Connection) readLoop(ctx context.Context, messageHandler func([]byte, string) error) {
	c.conn.SetReadLimit(int64(config.MaxMessageSize)) // 64KB limit
	c.conn.SetPongHandler(func(string) error {
		c.UpdateActivity()
		return nil
	})

	for {
		select {
		case <-c.closeCh:
			return // Clean shutdown
		case <-ctx.Done():
			return // Context cancelled
		default:
			messageType, data, err := c.conn.ReadMessage()
			if err != nil {
				log.Printf("Read error for user %s: %v", c.GetUserID(), err)
				return // Read error - connection dead
			}

			c.UpdateActivity()

			if messageType == websocket.TextMessage {
				userID := c.GetUserID()
				log.Printf("WEBSOCKET RECEIVED: Message from %s: %s", userID, string(data))
				if userID != "" { // Only process if authenticated
					log.Printf("WEBSOCKET CALLING: messageHandler for user %s", userID)
					if err := messageHandler(data, userID); err != nil {
						// Log error but continue processing
						log.Printf("Message processing error for user %s: %v", userID, err)
					} else {
						log.Printf("WEBSOCKET SUCCESS: messageHandler completed for user %s", userID)
					}
				} else {
					log.Printf("WEBSOCKET SKIPPED: User not authenticated")
				}
			}
		}
	}
}