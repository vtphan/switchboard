package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Connection implements the interfaces.Connection interface
// ARCHITECTURAL DISCOVERY: WebSocket writes must be serialized to prevent race conditions
// Interface boundary maintained - no business logic in connection wrapper
type Connection struct {
	conn          *websocket.Conn
	writeCh       chan []byte         // FUNCTIONAL DISCOVERY: 100 buffer prevents blocking in classroom scenarios
	userID        string              // Set after authentication
	role          string              // Set after authentication  
	sessionID     string              // Set after authentication
	authenticated bool                // Authentication status
	ctx           context.Context     // For cancellation
	cancel        context.CancelFunc  // For cleanup
	closeOnce     sync.Once           // Ensure single close
	mu            sync.RWMutex        // Protect auth fields
}

// NewConnection creates a new WebSocket connection wrapper
func NewConnection(conn *websocket.Conn) *Connection {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Connection{
		conn:          conn,
		writeCh:       make(chan []byte, 100), // Exactly 100 message buffer
		ctx:           ctx,
		cancel:        cancel,
		authenticated: false,
	}
	
	// Start the single writer goroutine
	go c.writeLoop()
	
	return c
}

// ARCHITECTURAL DISCOVERY: Single writer goroutine pattern eliminates races
func (c *Connection) writeLoop() {
	defer func() {
		// Clean up channel on exit
		for len(c.writeCh) > 0 {
			<-c.writeCh // Drain remaining messages
		}
		close(c.writeCh)
	}()
	
	for {
		select {
		case data, ok := <-c.writeCh:
			if !ok {
				return // Channel closed
			}
			
			// FUNCTIONAL DISCOVERY: 5-second timeout balances responsiveness vs classroom network stability
			if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return // Exit if we can't set deadline
			}
			
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				// Log error but continue processing other messages
				return
			}
			
		case <-c.ctx.Done():
			return
		}
	}
}

// WriteJSON implementation with timeout and error handling
func (c *Connection) WriteJSON(v interface{}) error {
	// Check if connection is closed
	select {
	case <-c.ctx.Done():
		return ErrConnectionClosed
	default:
	}
	
	// Marshal to JSON
	data, err := json.Marshal(v)
	if err != nil {
		return ErrInvalidJSON // FUNCTIONAL: Error wrapping for debugging
	}
	
	// Send to write channel with timeout  
	select {
	case c.writeCh <- data:
		return nil
	case <-time.After(5 * time.Second):
		return ErrWriteTimeout // FUNCTIONAL: Exact timeout as specified
	case <-c.ctx.Done():
		return ErrConnectionClosed
	}
}

// ARCHITECTURAL DISCOVERY: Clean shutdown requires careful goroutine coordination
func (c *Connection) Close() error {
	var err error
	c.closeOnce.Do(func() {
		// Cancel context to stop goroutines
		c.cancel()
		
		// Close WebSocket connection
		if c.conn != nil {
			err = c.conn.Close()
		}
		
		// writeCh will be closed by writeLoop goroutine
	})
	return err
}

// Authentication state management
func (c *Connection) SetCredentials(userID, role, sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.userID = userID
	c.role = role
	c.sessionID = sessionID
	c.authenticated = true
	
	return nil
}

func (c *Connection) IsAuthenticated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.authenticated
}

func (c *Connection) GetUserID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userID
}

func (c *Connection) GetRole() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.role
}

func (c *Connection) GetSessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}