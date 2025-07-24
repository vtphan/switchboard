package websocket

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"switchboard/pkg/interfaces"
	"switchboard/pkg/types"
)

// WebSocket upgrader with production-ready settings
// ARCHITECTURAL DISCOVERY: Separate upgrader configuration enables reuse
// and consistent WebSocket settings across different handler instances
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// FUNCTIONAL DISCOVERY: Allow all origins for development
		// Production deployments should implement stricter origin checking
		return true
	},
	HandshakeTimeout: 10 * time.Second,
}

// Handler manages WebSocket connections and authentication
// ARCHITECTURAL DISCOVERY: Clean separation of WebSocket handling from business logic
// integrates with Registry for connection management and interfaces for external dependencies
type Handler struct {
	registry       *Registry                    // Connection tracking and lookup
	sessionManager interfaces.SessionManager   // Session validation and management
	dbManager      interfaces.DatabaseManager  // Message history and persistence
}

// NewHandler creates a new WebSocket handler with dependency injection
// FUNCTIONAL DISCOVERY: Constructor pattern enables proper dependency management
// and facilitates testing with mock implementations
func NewHandler(registry *Registry, sessionManager interfaces.SessionManager, dbManager interfaces.DatabaseManager) *Handler {
	return &Handler{
		registry:       registry,
		sessionManager: sessionManager,
		dbManager:      dbManager,
	}
}

// HandleWebSocket handles WebSocket connection requests with comprehensive validation
// ARCHITECTURAL DISCOVERY: Multi-stage validation (parameters -> session -> WebSocket -> auth -> registration)
// ensures proper error handling and prevents invalid connections from consuming resources
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract and validate query parameters
	userID := r.URL.Query().Get("user_id")
	role := r.URL.Query().Get("role")
	sessionID := r.URL.Query().Get("session_id")
	
	if userID == "" || role == "" || sessionID == "" {
		http.Error(w, "Missing required query parameters: user_id, role, session_id", http.StatusBadRequest)
		return
	}
	
	// Validate user ID format using types package validation
	// FUNCTIONAL DISCOVERY: Reuse validation logic from types package
	// ensures consistent validation rules across all components
	if !types.IsValidUserID(userID) {
		http.Error(w, "Invalid user_id format", http.StatusBadRequest)
		return
	}
	
	// Validate role
	if role != "student" && role != "instructor" {
		http.Error(w, "Invalid role: must be 'student' or 'instructor'", http.StatusBadRequest)
		return
	}
	
	// Validate session membership using session manager
	// ARCHITECTURAL DISCOVERY: Delegate session validation to SessionManager interface
	// enables different validation strategies (cache-first, database-only, etc.)
	if err := h.sessionManager.ValidateSessionMembership(sessionID, userID, role); err != nil {
		switch err {
		case interfaces.ErrSessionNotFound:
			http.Error(w, "Session not found or ended", http.StatusNotFound)
		case interfaces.ErrUnauthorized:
			http.Error(w, "Not authorized to join this session", http.StatusForbidden)
		default:
			http.Error(w, "Session validation failed", http.StatusInternalServerError)
		}
		return
	}
	
	// Upgrade to WebSocket
	// FUNCTIONAL DISCOVERY: WebSocket upgrade after validation prevents resource waste
	// on invalid requests while providing proper HTTP error responses
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	
	// Create connection wrapper with single-writer pattern from Step 2.1
	wsConn := NewConnection(conn)
	
	// Set credentials after successful validation
	// TECHNICAL DISCOVERY: Authentication state set immediately after validation
	// prevents race conditions between connection registration and credential access
	if err := wsConn.SetCredentials(userID, role, sessionID); err != nil {
		log.Printf("Failed to set credentials: %v", err)
		_ = wsConn.Close()
		return
	}
	
	// Register connection with registry from Step 2.2
	// FUNCTIONAL DISCOVERY: Registration after authentication ensures only valid
	// connections are tracked and available for message routing
	if err := h.registry.RegisterConnection(wsConn); err != nil {
		log.Printf("Failed to register connection: %v", err)
		_ = wsConn.Close()
		return
	}
	
	// Send session history in background
	// ARCHITECTURAL DISCOVERY: Asynchronous history replay prevents blocking
	// connection setup while ensuring message history is delivered
	go h.sendSessionHistory(wsConn)
	
	// Start connection monitoring and message handling
	// TECHNICAL DISCOVERY: Separate goroutine for connection lifecycle management
	// enables clean resource cleanup and heartbeat monitoring
	go h.handleConnection(wsConn)
}

// sendSessionHistory sends all historical messages to new connection with role-based filtering
// FUNCTIONAL DISCOVERY: Role-based message filtering at delivery time ensures students
// only see relevant messages while instructors have full visibility
func (h *Handler) sendSessionHistory(conn *Connection) {
	sessionID := conn.GetSessionID()
	userID := conn.GetUserID()
	role := conn.GetRole()
	
	ctx := context.Background()
	messages, err := h.dbManager.GetSessionHistory(ctx, sessionID)
	if err != nil {
		log.Printf("Failed to get session history: %v", err)
		// Send error message to client
		errorMsg := map[string]interface{}{
			"type": "system",
			"content": map[string]interface{}{
				"event":   "history_unavailable",
				"message": "Unable to load message history",
			},
			"timestamp": time.Now(),
		}
		if err := conn.WriteJSON(errorMsg); err != nil {
			log.Printf("Failed to send auth error message: %v", err)
		}
		return
	}
	
	// Filter messages based on role for security and relevance
	// FUNCTIONAL DISCOVERY: Server-side filtering prevents sensitive message exposure
	// and reduces bandwidth for student connections with large message histories
	for _, message := range messages {
		shouldSend := false
		
		switch role {
		case "instructor":
			// Instructors see all messages for classroom management
			shouldSend = true
		case "student":
			// Students see messages involving them or broadcasts
			if message.FromUser == userID || 
			   (message.ToUser != nil && *message.ToUser == userID) ||
			   message.ToUser == nil { // Broadcast message
				shouldSend = true
			}
		}
		
		if shouldSend {
			if err := conn.WriteJSON(message); err != nil {
				log.Printf("Failed to send history message: %v", err)
				return
			}
		}
	}
	
	// Send history complete notification for client synchronization
	// TECHNICAL DISCOVERY: Explicit completion signal enables client-side loading states
	// and prevents confusion about history replay status
	completeMsg := map[string]interface{}{
		"type": "system",
		"content": map[string]interface{}{
			"event":   "history_complete",
			"message": "Message history loaded",
		},
		"timestamp": time.Now(),
	}
	if err := conn.WriteJSON(completeMsg); err != nil {
		log.Printf("Failed to send auth complete message: %v", err)
	}
}

// handleConnection manages the connection lifecycle with heartbeat monitoring
// ARCHITECTURAL DISCOVERY: Single goroutine per connection handles both heartbeat
// and message reading to prevent goroutine proliferation and resource leaks
func (h *Handler) handleConnection(conn *Connection) {
	defer func() {
		// Clean up connection from registry and close resources
		// FUNCTIONAL DISCOVERY: Deferred cleanup ensures resources are released
		// even if connection handling panics or exits unexpectedly
		h.registry.UnregisterConnection(conn)
		_ = conn.Close()
	}()
	
	// Set up ping/pong heartbeat monitoring
	// TECHNICAL DISCOVERY: 60-second read deadline with 30-second ping interval
	// provides reliable connection health monitoring for classroom environments
	if err := conn.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		log.Printf("Failed to set read deadline: %v", err)
		return
	}
	conn.conn.SetPongHandler(func(string) error {
		if err := conn.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			log.Printf("Failed to set read deadline in pong handler: %v", err)
			return err
		}
		return nil
	})
	
	// Start ping ticker for heartbeat monitoring
	// FUNCTIONAL DISCOVERY: Separate ticker goroutine enables consistent heartbeat
	// timing independent of message processing or client responsiveness
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	go func() {
		for {
			select {
			case <-ticker.C:
				// Send ping message with write timeout
				if err := conn.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
					return
				}
			case <-conn.ctx.Done():
				return
			}
		}
	}()
	
	// Read pump - handle incoming messages
	// ARCHITECTURAL DISCOVERY: Message reading loop processes client messages
	// and prepares integration point for Phase 3 message routing
	for {
		messageType, data, err := conn.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		
		if messageType == websocket.TextMessage {
			// INTEGRATION POINT: Forward message to message router (Phase 3)
			// FUNCTIONAL DISCOVERY: Message forwarding will be implemented in Phase 3
			// Current logging provides visibility into message flow for debugging
			log.Printf("Received message from %s: %s", conn.GetUserID(), string(data))
		}
	}
}