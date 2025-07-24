package fixtures

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"switchboard/pkg/types"
	"switchboard/pkg/interfaces"
	wsConnection "switchboard/internal/websocket"
)

// TestClient represents a WebSocket client for testing
type TestClient struct {
	UserID    string
	Role      string
	SessionID string
	ServerURL string
	
	conn     interfaces.Connection  // Use production Connection interface
	rawConn  *websocket.Conn       // Keep for reading (like server does)
	messages chan *types.Message
	errors   chan error
	done     chan struct{}
	
	mu       sync.RWMutex
	closed   bool
	connected bool
}

// NewTestClient creates a new WebSocket test client
func NewTestClient(userID, role, sessionID, serverURL string) *TestClient {
	return &TestClient{
		UserID:    userID,
		Role:      role,
		SessionID: sessionID,
		ServerURL: serverURL,
		messages:  make(chan *types.Message, 100), // Buffer for message collection
		errors:    make(chan error, 10),
		done:      make(chan struct{}),
	}
}

// Connect establishes WebSocket connection to the server
func (tc *TestClient) Connect(ctx context.Context) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	
	if tc.connected {
		return fmt.Errorf("client already connected")
	}
	
	// Build WebSocket URL with query parameters
	u, err := url.Parse(tc.ServerURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	
	// Switch to WebSocket scheme
	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	
	u.Path = "/ws"
	query := u.Query()
	query.Set("user_id", tc.UserID)
	query.Set("role", tc.Role)
	query.Set("session_id", tc.SessionID)
	u.RawQuery = query.Encode()
	
	// Establish WebSocket connection
	dialer := websocket.DefaultDialer
	rawConn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	
	// Use production Connection wrapper for thread-safe writes
	tc.conn = wsConnection.NewConnection(rawConn)
	tc.rawConn = rawConn  // Keep for reading (like server does)
	
	// Set credentials to match production pattern
	if err := tc.conn.SetCredentials(tc.UserID, tc.Role, tc.SessionID); err != nil {
		rawConn.Close()
		return fmt.Errorf("failed to set credentials: %w", err)
	}
	
	tc.connected = true
	
	// Start message reading goroutine
	go tc.readLoop()
	
	return nil
}

// readLoop continuously reads messages from the WebSocket connection
func (tc *TestClient) readLoop() {
	defer func() {
		tc.mu.Lock()
		tc.connected = false
		tc.mu.Unlock()
		
		// Close done channel if not already closed
		select {
		case <-tc.done:
			// Already closed
		default:
			close(tc.done)
		}
	}()
	
	for {
		tc.mu.RLock()
		rawConn := tc.rawConn
		closed := tc.closed
		tc.mu.RUnlock()
		
		if closed || rawConn == nil {
			return
		}
		
		// Set read timeout
		rawConn.SetReadDeadline(time.Now().Add(30 * time.Second))
		
		// Read message using the same pattern as server (ReadMessage then Unmarshal)
		messageType, data, err := rawConn.ReadMessage()
		if err != nil {
			tc.mu.RLock()
			stillClosed := tc.closed
			tc.mu.RUnlock()
			
			if !stillClosed {
				select {
				case tc.errors <- fmt.Errorf("read error: %w", err):
				default:
				}
			}
			return
		}
		
		if messageType == websocket.TextMessage {
			var message types.Message
			err := json.Unmarshal(data, &message)
			if err != nil {
				tc.mu.RLock()
				stillClosed := tc.closed
				tc.mu.RUnlock()
				
				if !stillClosed {
					select {
					case tc.errors <- fmt.Errorf("parse error: %w", err):
					default:
					}
				}
				continue
			}
			
			// Send message to channel (non-blocking)
			select {
			case tc.messages <- &message:
			default:
				// Channel full, drop message (shouldn't happen in tests)
				select {
				case tc.errors <- fmt.Errorf("message channel full, dropping message"):
				default:
				}
			}
		}
	}
}

// SendMessage sends a message to the server
func (tc *TestClient) SendMessage(msgType, context string, content map[string]interface{}, toUser string) error {
	tc.mu.RLock()
	conn := tc.conn
	connected := tc.connected
	tc.mu.RUnlock()
	
	if !connected || conn == nil {
		return fmt.Errorf("client not connected")
	}
	
	// Build message (server will set ID, timestamp, from_user, session_id)
	message := map[string]interface{}{
		"type":    msgType,
		"context": context,
		"content": content,
	}
	
	if toUser != "" {
		message["to_user"] = toUser
	}
	
	// Send message using production Connection wrapper (thread-safe)
	// The Connection wrapper handles timeouts and single-writer pattern internally
	err := conn.WriteJSON(message)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	
	return nil
}

// ReceiveMessage waits for a message with timeout
func (tc *TestClient) ReceiveMessage(timeout time.Duration) (*types.Message, error) {
	select {
	case message := <-tc.messages:
		return message, nil
	case err := <-tc.errors:
		return nil, err
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for message")
	case <-tc.done:
		return nil, fmt.Errorf("client disconnected")
	}
}

// ReceiveMessageOfType waits for a message of specific type, skipping system messages
func (tc *TestClient) ReceiveMessageOfType(msgType string, timeout time.Duration) (*types.Message, error) {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		
		select {
		case message := <-tc.messages:
			if message.Type == msgType {
				return message, nil
			}
			// Skip system messages and continue waiting
			if message.Type == "system" {
				continue
			}
			// Return non-system message even if type doesn't match
			return message, nil
		case err := <-tc.errors:
			return nil, err
		case <-time.After(remaining):
			break
		case <-tc.done:
			return nil, fmt.Errorf("client disconnected")
		}
	}
	
	return nil, fmt.Errorf("timeout waiting for message of type %s", msgType)
}

// ReceiveMessages waits for multiple messages with timeout
func (tc *TestClient) ReceiveMessages(count int, timeout time.Duration) ([]*types.Message, error) {
	messages := make([]*types.Message, 0, count)
	deadline := time.Now().Add(timeout)
	
	for len(messages) < count {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return messages, fmt.Errorf("timeout: received %d/%d messages", len(messages), count)
		}
		
		message, err := tc.ReceiveMessage(remaining)
		if err != nil {
			return messages, err
		}
		
		messages = append(messages, message)
	}
	
	return messages, nil
}

// GetReceivedMessages returns all messages received so far
func (tc *TestClient) GetReceivedMessages() []*types.Message {
	messages := []*types.Message{}
	
	// Drain messages channel non-blocking
	for {
		select {
		case msg := <-tc.messages:
			messages = append(messages, msg)
		default:
			return messages
		}
	}
}

// WaitForMessageType waits for a specific message type with timeout
func (tc *TestClient) WaitForMessageType(messageType string, timeout time.Duration) (*types.Message, error) {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		message, err := tc.ReceiveMessage(remaining)
		if err != nil {
			return nil, err
		}
		
		if message.Type == messageType {
			return message, nil
		}
		
		// Message wasn't the type we wanted, put it back (simulate by continuing)
	}
	
	return nil, fmt.Errorf("timeout waiting for message type: %s", messageType)
}

// WaitForMessageFrom waits for a message from a specific user
func (tc *TestClient) WaitForMessageFrom(fromUser string, timeout time.Duration) (*types.Message, error) {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		message, err := tc.ReceiveMessage(remaining)
		if err != nil {
			return nil, err
		}
		
		if message.FromUser == fromUser {
			return message, nil
		}
	}
	
	return nil, fmt.Errorf("timeout waiting for message from: %s", fromUser)
}

// SendPing sends a WebSocket ping to test connection health
func (tc *TestClient) SendPing() error {
	tc.mu.RLock()
	rawConn := tc.rawConn
	connected := tc.connected
	tc.mu.RUnlock()
	
	if !connected || rawConn == nil {
		return fmt.Errorf("client not connected")
	}
	
	rawConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return rawConn.WriteMessage(websocket.PingMessage, []byte{})
}

// Close closes the WebSocket connection and cleans up resources
func (tc *TestClient) Close() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	
	if tc.closed {
		return nil // Already closed
	}
	
	tc.closed = true
	
	// Close the production Connection wrapper (handles cleanup properly)
	if tc.conn != nil {
		tc.conn.Close()
	}
	
	// Also close raw connection as fallback
	if tc.rawConn != nil {
		tc.rawConn.Close()
	}
	
	// Signal done to any waiting goroutines
	select {
	case <-tc.done:
		// Already closed
	default:
		close(tc.done)
	}
	
	return nil
}

// IsConnected returns whether the client is currently connected
func (tc *TestClient) IsConnected() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.connected && !tc.closed
}

// GetErrors returns any errors that occurred during connection
func (tc *TestClient) GetErrors() []error {
	errors := []error{}
	
	// Drain errors channel non-blocking
	for {
		select {
		case err := <-tc.errors:
			errors = append(errors, err)
		default:
			return errors
		}
	}
}

// WaitForConnection waits for the connection to be established
func (tc *TestClient) WaitForConnection(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		if tc.IsConnected() {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	
	return fmt.Errorf("timeout waiting for connection")
}

// DrainMessages clears the message buffer
func (tc *TestClient) DrainMessages() {
	for {
		select {
		case <-tc.messages:
			// Discard message
		default:
			return
		}
	}
}

// GetMessageCount returns the number of buffered messages
func (tc *TestClient) GetMessageCount() int {
	return len(tc.messages)
}

// SendQuickMessage is a convenience method for simple message sending
func (tc *TestClient) SendQuickMessage(msgType, text string) error {
	content := map[string]interface{}{
		"text": text,
	}
	
	return tc.SendMessage(msgType, "general", content, "")
}

// SendDirectMessage sends a message to a specific user
func (tc *TestClient) SendDirectMessage(msgType, text, toUser string) error {
	content := map[string]interface{}{
		"text": text,
	}
	
	return tc.SendMessage(msgType, "general", content, toUser)
}