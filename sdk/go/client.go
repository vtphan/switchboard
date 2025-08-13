package switchboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// outgoingMessage represents a message to be sent via WebSocket
type outgoingMessage struct {
	messageType int
	data        []byte
}

// Config contains configuration for the Switchboard client
type Config struct {
	UserID string // Required: unique user identifier
	Role   Role   // Required: "student" or "instructor"

	// Optional configuration
	WsURL   string // WebSocket URL (default: "ws://localhost:8080/ws")
	APIURL  string // API URL (auto-derived from WsURL if not set)
	
	// 4 message type hooks (all optional)
	OnBroadcastToInstructors func(*Message) // Student questions
	OnBroadcastToStudents    func(*Message) // Instructor announcements
	OnDirectMessage          func(*Message) // Private messages
	OnSystem                 func(*Message) // System messages
	
	// 2 state change hooks (optional)
	OnConnectionChange func(ConnectionState, error) // Connection state changes
	OnSessionChange    func(*Session)               // Session state changes
	
	// Optional advanced configuration
	MaxReconnectAttempts int           // Default: 10
	Logger              *log.Logger   // Default: log.Default()
}

// Client is the main Switchboard client implementing the V2 simplified API
type Client struct {
	config            Config
	conn              *websocket.Conn
	connMu            sync.RWMutex
	connectionState   ConnectionState
	sessionActive     bool
	currentSession    *Session
	messageQueue      []string
	queueMu           sync.Mutex
	reconnectAttempts int
	rateLimitTimes    []time.Time
	rateLimitMu       sync.Mutex
	stopChan          chan struct{}
	doneChan          chan struct{}
	outgoingChan      chan outgoingMessage
	writerDone        chan struct{}
	once              sync.Once
	httpClient        *http.Client
	logger            *log.Logger
}

// NewClient creates a new Switchboard client with V2 simplified API
func NewClient(config Config) (*Client, error) {
	// Validate required fields
	if config.UserID == "" {
		return nil, fmt.Errorf("userID required")
	}
	if config.Role != RoleStudent && config.Role != RoleInstructor {
		return nil, fmt.Errorf("role must be 'student' or 'instructor'")
	}

	// Set defaults
	if config.WsURL == "" {
		config.WsURL = "ws://localhost:8080/ws"
	}
	if config.APIURL == "" {
		config.APIURL = strings.Replace(strings.Replace(config.WsURL, "ws:", "http:", 1), "/ws", "/api", 1)
	}
	if config.MaxReconnectAttempts == 0 {
		config.MaxReconnectAttempts = 10
	}
	if config.Logger == nil {
		config.Logger = log.Default()
	}

	return &Client{
		config:          config,
		connectionState: ConnectionStateDisconnected,
		stopChan:        make(chan struct{}),
		doneChan:        make(chan struct{}),
		outgoingChan:    make(chan outgoingMessage, 100), // Buffered channel for outgoing messages
		writerDone:      make(chan struct{}),
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		logger:          config.Logger,
	}, nil
}

// Connect establishes a WebSocket connection
func (c *Client) Connect() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.connectionState == ConnectionStateConnected {
		return nil
	}
	if c.connectionState == ConnectionStateConnecting {
		return fmt.Errorf("connection already in progress")
	}

	c.setConnectionState(ConnectionStateConnecting, nil)

	// Build WebSocket URL
	u, err := url.Parse(c.config.WsURL)
	if err != nil {
		c.setConnectionState(ConnectionStateError, err)
		return fmt.Errorf("invalid WebSocket URL: %w", err)
	}

	query := u.Query()
	query.Set("user_id", c.config.UserID)
	query.Set("role", string(c.config.Role))
	u.RawQuery = query.Encode()

	// Connect with timeout
	dialer := websocket.Dialer{HandshakeTimeout: time.Duration(ConnectionTimeout) * time.Millisecond}
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		c.setConnectionState(ConnectionStateError, err)
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.conn = conn
	c.conn.SetReadLimit(MaxMessageSize)
	c.setConnectionState(ConnectionStateConnected, nil)
	c.reconnectAttempts = 0

	// Start goroutines
	go c.readLoop()
	go c.pingLoop()
	go c.writeLoop()

	// Flush queued messages
	c.flushMessageQueue()

	return nil
}

// Disconnect closes the WebSocket connection
func (c *Client) Disconnect() {
	c.once.Do(func() {
		close(c.stopChan)
	})

	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		// Send close message via channel with short timeout
		select {
		case c.outgoingChan <- outgoingMessage{websocket.CloseMessage, 
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Client disconnect")}:
		case <-time.After(1 * time.Second):
			// Timeout sending close message, proceed anyway
		}
		
		c.conn.Close()
		c.conn = nil
	}

	c.setConnectionState(ConnectionStateDisconnected, nil)
	c.reconnectAttempts = 0

	// Wait for both readLoop and writeLoop to finish
	<-c.doneChan
	<-c.writerDone
}

// BroadcastToInstructors sends a message to all instructors (students only)
func (c *Client) BroadcastToInstructors(content interface{}) error {
	return c.send("broadcast_to_instructors", content, "")
}

// BroadcastToStudents sends a message to all students (instructors only)  
func (c *Client) BroadcastToStudents(content interface{}) error {
	return c.send("broadcast_to_students", content, "")
}

// DirectMessage sends a private message to a specific user
func (c *Client) DirectMessage(toUser string, content interface{}) error {
	return c.send("direct_message", content, toUser)
}

// Session Management (instructors only)

// StartSession starts a new session
func (c *Client) StartSession(sessionName string) error {
	if c.config.Role != RoleInstructor {
		return fmt.Errorf("only instructors can start sessions")
	}

	req := map[string]interface{}{
		"name":          sessionName,
		"instructor_id": c.config.UserID,
	}

	_, err := c.makeAPIRequest("POST", "/session/start", req)
	return err
}

// EndSession ends the current session
func (c *Client) EndSession() error {
	if c.config.Role != RoleInstructor {
		return fmt.Errorf("only instructors can end sessions")
	}

	req := map[string]interface{}{
		"instructor_id": c.config.UserID,
	}

	_, err := c.makeAPIRequest("POST", "/session/end", req)
	return err
}

// Utility methods

// IsConnected returns true if connected
func (c *Client) IsConnected() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connectionState == ConnectionStateConnected
}

// IsSessionActive returns true if there's an active session
func (c *Client) IsSessionActive() bool {
	return c.sessionActive
}

// GetCurrentSession returns the current session info
func (c *Client) GetCurrentSession() *Session {
	return c.currentSession
}

// GetConnectionStatus returns the current connection state
func (c *Client) GetConnectionStatus() ConnectionState {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connectionState
}

// Private methods

func (c *Client) send(msgType string, content interface{}, toUser string) error {
	// Convert content to MessageContent if it's a string
	var msgContent MessageContent
	switch v := content.(type) {
	case string:
		msgContent = NewMessageContent(v)
	case MessageContent:
		msgContent = v
	case map[string]interface{}:
		msgContent = MessageContent(v)
	default:
		return fmt.Errorf("content must be string, MessageContent, or map[string]interface{}")
	}

	// Extract context from content (critical fix for V2)
	context := "general"
	if ctx, ok := msgContent["context"].(string); ok {
		context = ctx
		delete(msgContent, "context") // Remove to prevent double-nesting
	}

	// Validate session unless system context
	if !c.sessionActive && context != "system" {
		return fmt.Errorf("no active session")
	}

	// Check rate limit
	if !c.checkRateLimit() {
		return fmt.Errorf("rate limit exceeded: maximum %d messages per minute", RateLimitMaxMessages)
	}

	// Build message with correct protocol structure
	msg := OutgoingMessage{
		Type:    msgType,
		Context: context,
		Content: msgContent,
	}

	if toUser != "" {
		msg.ToUser = toUser
	}

	// Serialize and validate size
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	if len(data) > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(data), MaxMessageSize)
	}

	// Send via channel for serialized writes
	c.connMu.RLock()
	state := c.connectionState
	c.connMu.RUnlock()

	if state == ConnectionStateConnected {
		// Try to send via channel with timeout
		select {
		case c.outgoingChan <- outgoingMessage{websocket.TextMessage, data}:
			c.trackRateLimit()
			return nil
		case <-time.After(5 * time.Second):
			// Channel is full or blocked, queue for later
			c.queueMessage(string(data))
			return fmt.Errorf("send timeout, message queued")
		}
	}

	c.queueMessage(string(data))
	return nil
}

func (c *Client) setConnectionState(state ConnectionState, err error) {
	c.connectionState = state
	if c.config.OnConnectionChange != nil {
		c.config.OnConnectionChange(state, err)
	}
}

func (c *Client) readLoop() {
	defer close(c.doneChan)

	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		c.connMu.RLock()
		conn := c.conn
		c.connMu.RUnlock()

		if conn == nil {
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.setConnectionState(ConnectionStateDisconnected, err)
				c.scheduleReconnect()
				return
			}
			continue
		}

		c.handleMessage(data)
	}
}

func (c *Client) pingLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.connMu.RLock()
			state := c.connectionState
			c.connMu.RUnlock()

			if state == ConnectionStateConnected {
				// Send ping via channel
				select {
				case c.outgoingChan <- outgoingMessage{websocket.PingMessage, nil}:
					// Ping sent successfully
				case <-time.After(1 * time.Second):
					// Channel is blocked, skip this ping
					c.logger.Printf("Skipped ping due to channel timeout")
				}
			}
		}
	}
}

func (c *Client) writeLoop() {
	defer close(c.writerDone)
	
	for {
		select {
		case <-c.stopChan:
			return
		case msg := <-c.outgoingChan:
			c.connMu.RLock()
			conn := c.conn
			state := c.connectionState
			c.connMu.RUnlock()
			
			if state == ConnectionStateConnected && conn != nil {
				if err := conn.WriteMessage(msg.messageType, msg.data); err != nil {
					c.logger.Printf("Failed to write message: %v", err)
					// Connection likely broken, will be handled by readLoop
				}
			}
			// If not connected, message is discarded (it was already queued)
		}
	}
}

func (c *Client) handleMessage(data []byte) {
	msg, err := parseMessage(data)
	if err != nil {
		c.logger.Printf("Failed to parse message: %v", err)
		return
	}

	// Handle system messages internally first
	if msg.Type == "system" {
		c.handleSystemMessage(msg)
	}

	// Route to appropriate handler
	switch msg.Type {
	case "broadcast_to_instructors":
		if c.config.OnBroadcastToInstructors != nil {
			c.config.OnBroadcastToInstructors(msg)
		}
	case "broadcast_to_students":
		if c.config.OnBroadcastToStudents != nil {
			c.config.OnBroadcastToStudents(msg)
		}
	case "direct_message":
		if c.config.OnDirectMessage != nil {
			c.config.OnDirectMessage(msg)
		}
	case "system":
		if c.config.OnSystem != nil {
			c.config.OnSystem(msg)
		}
	}
}

func (c *Client) handleSystemMessage(msg *Message) {
	event, _ := msg.Content["event"].(string)

	switch event {
	case "session_started", "session_active":
		c.sessionActive = true
		c.currentSession = &Session{
			Active:    true,
			ID:        getString(msg.Content, "session_id"),
			Name:      getString(msg.Content, "session_name"),
			StartedBy: getString(msg.Content, "started_by"),
		}
	case "session_ended", "waiting_for_session":
		c.sessionActive = false
		c.currentSession = &Session{Active: false}
	}

	if c.config.OnSessionChange != nil {
		c.config.OnSessionChange(c.currentSession)
	}
}

func (c *Client) scheduleReconnect() {
	if c.reconnectAttempts >= c.config.MaxReconnectAttempts {
		c.logger.Printf("Max reconnection attempts reached")
		return
	}

	delay := time.Duration(ReconnectBaseDelay * (1 << c.reconnectAttempts)) * time.Millisecond
	if delay > time.Duration(MaxReconnectDelay) * time.Millisecond {
		delay = time.Duration(MaxReconnectDelay) * time.Millisecond
	}

	time.AfterFunc(delay, func() {
		c.reconnectAttempts++
		c.Connect()
	})
}

func (c *Client) queueMessage(msgStr string) {
	c.queueMu.Lock()
	defer c.queueMu.Unlock()
	c.messageQueue = append(c.messageQueue, msgStr)
}

func (c *Client) flushMessageQueue() {
	c.queueMu.Lock()
	queue := c.messageQueue
	c.messageQueue = nil
	c.queueMu.Unlock()

	for _, msgStr := range queue {
		c.connMu.RLock()
		state := c.connectionState
		c.connMu.RUnlock()

		if state == ConnectionStateConnected {
			// Send via channel with timeout
			select {
			case c.outgoingChan <- outgoingMessage{websocket.TextMessage, []byte(msgStr)}:
				c.trackRateLimit()
			case <-time.After(1 * time.Second):
				// Channel is blocked, skip this message
				c.logger.Printf("Skipped queued message due to channel timeout")
			}
		}
	}
}

func (c *Client) checkRateLimit() bool {
	c.rateLimitMu.Lock()
	defer c.rateLimitMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Duration(RateLimitWindow) * time.Millisecond)

	var validTimes []time.Time
	for _, t := range c.rateLimitTimes {
		if t.After(cutoff) {
			validTimes = append(validTimes, t)
		}
	}
	c.rateLimitTimes = validTimes

	return len(c.rateLimitTimes) < RateLimitMaxMessages
}

func (c *Client) trackRateLimit() {
	c.rateLimitMu.Lock()
	defer c.rateLimitMu.Unlock()
	c.rateLimitTimes = append(c.rateLimitTimes, time.Now())
}

func (c *Client) makeAPIRequest(method, endpoint string, reqBody interface{}) (map[string]interface{}, error) {
	url := c.config.APIURL + endpoint

	var reqBytes []byte
	var err error

	if reqBody != nil {
		reqBytes, err = json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errorResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorResp)
		if msg, ok := errorResp["message"].(string); ok {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, msg)
		}
		return nil, fmt.Errorf("API error: %d %s", resp.StatusCode, resp.Status)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response, nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}