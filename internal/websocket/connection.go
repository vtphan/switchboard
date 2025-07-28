package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Connection implements the interfaces.Connection interface
// Pure channel-based design - no mutex contention on writes
type Connection struct {
	conn          *websocket.Conn
	writeCh       chan []byte         // 100 buffer prevents blocking in classroom scenarios
	userID        string              // Set after authentication
	role          string              // Set after authentication  
	sessionID     string              // Set after authentication
	authenticated bool                // Authentication status
	ctx           context.Context     // For cancellation
	cancel        context.CancelFunc  // For cleanup
	closeOnce     sync.Once           // Ensure single close
	mu            sync.RWMutex        // Protect auth fields only
	writeClosed   int32               // Atomic flag for write channel status
}

// NewConnection creates a new WebSocket connection wrapper
func NewConnection(conn *websocket.Conn) *Connection {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Connection{
		conn:          conn,
		writeCh:       make(chan []byte, 500), // Increased buffer for high-frequency scenarios
		ctx:           ctx,
		cancel:        cancel,
		authenticated: false,
	}
	
	// Start the single writer goroutine
	go c.writeLoop()
	
	return c
}

// Single writer goroutine pattern with pure channel coordination
func (c *Connection) writeLoop() {
	defer func() {
		// Set atomic flag before closing to prevent panics
		atomic.StoreInt32(&c.writeClosed, 1)
		
		// Clean up channel on exit - no mutex needed
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

// WriteJSON implementation with pure channel-based concurrency
func (c *Connection) WriteJSON(v interface{}) error {
	// Check if connection is closed first
	select {
	case <-c.ctx.Done():
		return ErrConnectionClosed
	default:
	}
	
	// Marshal to JSON
	data, err := json.Marshal(v)
	if err != nil {
		return ErrInvalidJSON
	}
	
	// Check atomic flag to avoid panic on closed channel
	if atomic.LoadInt32(&c.writeClosed) == 1 {
		return ErrConnectionClosed
	}
	
	select {
	case c.writeCh <- data:
		return nil
	case <-c.ctx.Done():
		return ErrConnectionClosed
	default:
		// Non-blocking write: if channel is full, drop message and return error
		// This prevents broadcasts from hanging on slow/stuck connections
		return ErrWriteTimeout
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

// SetSessionID updates the session ID for auto-transition
// PHASE 4 DISCOVERY: Enables registry to update connection state during transitions
func (c *Connection) SetSessionID(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionID = sessionID
}