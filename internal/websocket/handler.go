package websocket

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"switchboard/internal/database"
	"switchboard/internal/message"
	"switchboard/internal/session"
)

// WebSocketHandler manages WebSocket connections with integration to session management
// and message processing. Implements the exact pattern from tech specs lines 419-598.
type WebSocketHandler struct {
	registry        *ConnectionRegistry      // Step 4.2 connection registry
	sessionManager  session.SessionManager  // Phase 2 session management
	messageProcessor message.MessageProcessorInterface // Phase 3 message processing interface
	dbManager       database.DatabaseManager // Database access for session history
	upgrader        websocket.Upgrader       // WebSocket connection upgrader
}

// NewWebSocketHandler creates a new WebSocket handler with all required dependencies.
// Following the exact initialization pattern from tech specs lines 459-473.
func NewWebSocketHandler(
	registry *ConnectionRegistry,
	sessionManager session.SessionManager,
	messageProcessor message.MessageProcessorInterface,
	dbManager database.DatabaseManager,
) *WebSocketHandler {
	return &WebSocketHandler{
		registry:        registry,
		sessionManager:  sessionManager,
		messageProcessor: messageProcessor,
		dbManager:       dbManager,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// In development: allow all origins
				// In production: implement proper origin checking
				return true
			},
			HandshakeTimeout: 10 * time.Second,
		},
	}
}

// HandleWebSocketUpgrade upgrades HTTP to WebSocket following the exact algorithm
// from tech specs lines 475-531. Handles authentication, session state delivery,
// and connection lifecycle management.
func (wsh *WebSocketHandler) HandleWebSocketUpgrade(w http.ResponseWriter, r *http.Request) error {
	// Extract query parameters - following tech specs lines 476-488
	userID := r.URL.Query().Get("user_id")
	role := r.URL.Query().Get("role")
	
	log.Printf("WEBSOCKET UPGRADE: user_id=%s, role=%s", userID, role)
	
	if userID == "" || role == "" {
		log.Printf("WEBSOCKET ERROR: Missing parameters - user_id='%s', role='%s'", userID, role)
		http.Error(w, "user_id and role query parameters required", http.StatusBadRequest)
		return errors.New("missing required parameters")
	}
	
	if role != "student" && role != "instructor" {
		log.Printf("WEBSOCKET ERROR: Invalid role '%s'", role)
		http.Error(w, "role must be 'student' or 'instructor'", http.StatusBadRequest)
		return errors.New("invalid role")
	}
	
	// Upgrade connection - following tech specs lines 490-494
	log.Printf("WEBSOCKET UPGRADING: Connection for %s", userID)
	wsConn, err := wsh.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WEBSOCKET UPGRADE FAILED: %v", err)
		return fmt.Errorf("websocket upgrade failed: %w", err)
	}
	log.Printf("WEBSOCKET UPGRADED: Successfully upgraded connection for %s", userID)
	
	// Create connection wrapper - following tech specs lines 496-502
	conn := NewConnection(wsConn)
	conn.SetRegistry(wsh.registry) // Set registry for heartbeat notifications
	err = conn.SetCredentials(userID, role)
	if err != nil {
		log.Printf("WEBSOCKET CREDENTIALS FAILED: %v", err)
		_ = conn.Close()
		return fmt.Errorf("credential setting failed: %w", err)
	}
	log.Printf("WEBSOCKET CREDENTIALS SET: %s as %s", userID, role)
	
	// Register connection - following tech specs lines 504-509
	err = wsh.registry.Register(userID, conn)
	if err != nil {
		log.Printf("WEBSOCKET REGISTRATION FAILED: %v", err)
		_ = conn.Close()
		return fmt.Errorf("connection registration failed: %w", err)
	}
	log.Printf("WEBSOCKET REGISTERED: %s in registry", userID)
	
	// Send session state - following tech specs lines 511-512
	log.Printf("WEBSOCKET SENDING SESSION STATE: to %s", userID)
	wsh.sendSessionState(conn)
	
	// Send session history if session active - following tech specs lines 514-517
	if activeSession := wsh.sessionManager.GetActiveSession(); activeSession != nil {
		log.Printf("WEBSOCKET SENDING HISTORY: session %s to %s", activeSession.ID, userID)
		wsh.sendSessionHistory(conn, activeSession)
	} else {
		log.Printf("WEBSOCKET NO HISTORY: no active session for %s", userID)
	}
	
	// Start connection goroutines with message handler - following tech specs lines 519-521
	ctx := context.Background() // TODO: Use proper context from HTTP server
	log.Printf("WEBSOCKET STARTING: Connection loops for %s", userID)
	conn.Start(ctx, wsh.handleMessage)
	
	// Connection cleanup handled by registry's session-aware cleanup mechanism
	// No per-connection cleanup goroutine needed
	
	return nil
}

// sendSessionState sends the current session state to a newly connected client.
// Following the exact message format from tech specs lines 534-562.
func (wsh *WebSocketHandler) sendSessionState(conn *Connection) {
	activeSession := wsh.sessionManager.GetActiveSession()
	
	if activeSession == nil {
		// No active session - send waiting message (lines 537-547)
		message := map[string]interface{}{
			"type": "system",
			"content": map[string]interface{}{
				"event":   "waiting_for_session",
				"message": "Connected successfully. Waiting for instructor to start session.",
			},
			"timestamp": time.Now(),
		}
		if err := conn.WriteJSON(message); err != nil {
			log.Printf("Failed to send waiting message to user %s: %v", conn.GetUserID(), err)
		}
	} else {
		// Active session - send session details (lines 548-562)
		message := map[string]interface{}{
			"type": "system",
			"content": map[string]interface{}{
				"event":        "session_active",
				"session_id":   activeSession.ID,
				"session_name": activeSession.Name,
				"started_by":   activeSession.CreatedBy,
				"start_time":   activeSession.StartTime,
			},
			"timestamp": time.Now(),
		}
		if err := conn.WriteJSON(message); err != nil {
			log.Printf("Failed to send session state to user %s: %v", conn.GetUserID(), err)
		}
	}
}

// sendSessionHistory sends the complete session message history to late joiners.
// Following tech specs lines 565-578. Retrieves messages from database and applies
// role-based filtering to deliver appropriate history to the connecting user.
func (wsh *WebSocketHandler) sendSessionHistory(conn *Connection, activeSession *database.Session) {
	// Retrieve all session messages from database
	messages, err := wsh.dbManager.GetSessionMessages(activeSession.ID)
	if err != nil {
		log.Printf("Failed to retrieve session messages for user %s: %v", conn.GetUserID(), err)
		return
	}

	// Create role-based filter for message filtering
	filter := message.NewRoleBasedFilter()

	// Filter and send each message based on user's role
	for _, msg := range messages {
		// Check if this user should receive this message based on their role
		if filter.ShouldReceiveMessage(msg, conn) {
			// Convert database message to client message format
			clientMessage := map[string]interface{}{
				"id":        msg.ID,
				"type":      msg.Type,
				"context":   msg.Context,
				"from_user": msg.FromUser,
				"content":   msg.Content,
				"timestamp": msg.Timestamp,
			}
			
			// Add to_user field for direct messages
			if msg.ToUser != nil {
				clientMessage["to_user"] = *msg.ToUser
			}

			// Send the filtered message to the late joiner
			if err := conn.WriteJSON(clientMessage); err != nil {
				log.Printf("Failed to send history message %s to user %s: %v", msg.ID, conn.GetUserID(), err)
				// Continue sending other messages even if one fails
			}
		}
	}

	// Send completion notification
	completionMessage := map[string]interface{}{
		"type": "system",
		"content": map[string]interface{}{
			"event":   "history_delivered",
			"message": fmt.Sprintf("Session history for %s delivered", activeSession.Name),
		},
		"timestamp": time.Now(),
	}
	if err := conn.WriteJSON(completionMessage); err != nil {
		log.Printf("Failed to send history completion message to user %s: %v", conn.GetUserID(), err)
	}
}

// handleMessage delegates incoming WebSocket messages to the Phase 3 MessageProcessor.
// Following the exact delegation pattern from tech specs lines 581-583.
func (wsh *WebSocketHandler) handleMessage(rawData []byte, senderID string) error {
	// Delegate to message processor from Phase 3
	return wsh.messageProcessor.ProcessIncomingMessage(rawData, senderID)
}