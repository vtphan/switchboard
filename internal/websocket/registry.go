package websocket

import (
	"log"
	"sync"
	"time"
)

// Registry manages WebSocket connections with thread-safe operations
// ARCHITECTURAL DISCOVERY: Pure connection management without business logic
// maintains clean separation between connection tracking and connection operations
type Registry struct {
	mu                 sync.RWMutex                      // TECHNICAL DISCOVERY: RWMutex optimizes for read-heavy lookup patterns
	globalConnections  map[string]*Connection            // userID -> Connection for O(1) global lookup
	sessionInstructors map[string]map[string]*Connection // sessionID -> userID -> Connection
	sessionStudents    map[string]map[string]*Connection // sessionID -> userID -> Connection
	broadcastWG        sync.WaitGroup                    // For testing: wait for async broadcasts to complete
	batchMode          bool                              // Suppress presence_update broadcasts during batch registration
}

// NewRegistry creates a new connection registry
// FUNCTIONAL DISCOVERY: Initialize all maps to prevent nil pointer access during concurrent operations
func NewRegistry() *Registry {
	return &Registry{
		globalConnections:  make(map[string]*Connection),
		sessionInstructors: make(map[string]map[string]*Connection),
		sessionStudents:    make(map[string]map[string]*Connection),
	}
}

// RegisterConnection adds a connection to all appropriate maps atomically
// ARCHITECTURAL DISCOVERY: Connection replacement pattern coordinates with cleanup
// to prevent resource leaks while maintaining immediate registration
func (r *Registry) RegisterConnection(conn *Connection) error {
	if conn == nil {
		return ErrNilConnection
	}

	if !conn.IsAuthenticated() {
		return ErrConnectionNotAuthenticated
	}

	userID := conn.GetUserID()
	role := conn.GetRole()
	sessionID := conn.GetSessionID()

	r.mu.Lock()
	defer r.mu.Unlock()

	// RACE CONDITION FIX: Atomic connection replacement prevents race between
	// existence check and replacement notification
	var existingConn *Connection
	if oldConn, exists := r.globalConnections[userID]; exists {
		existingConn = oldConn
	}

	// Atomically replace connection first to prevent race conditions
	r.globalConnections[userID] = conn

	// Close old connection after atomic replacement
	if existingConn != nil {
		log.Printf("Replacing existing connection for user %s", userID)
		// Close synchronously to ensure cleanup completion before proceeding
		_ = existingConn.Close()
		// Note: UnregisterConnection will be called by the connection's cleanup handler
	}

	// Connection already added to global map above (line 59)

	// Add to appropriate session-role map for efficient recipient lookup
	switch role {
	case "instructor":
		if r.sessionInstructors[sessionID] == nil {
			r.sessionInstructors[sessionID] = make(map[string]*Connection)
		}
		r.sessionInstructors[sessionID][userID] = conn
	case "student":
		if r.sessionStudents[sessionID] == nil {
			r.sessionStudents[sessionID] = make(map[string]*Connection)
		}
		r.sessionStudents[sessionID][userID] = conn
	}

	// LOBBY SYSTEM: Send simplified presence update to all users (unless in batch mode)
	if !r.batchMode {
		r.broadcastWG.Add(1)
		go func() {
			defer r.broadcastWG.Done()

			// Broadcast unified presence_update event to all existing instructors
			presenceUpdateMsg := map[string]interface{}{
				"type": "system",
				"content": map[string]interface{}{
					"event":      "presence_update",
					"user_id":    userID,
					"role":       role,
					"session_id": sessionID,
				},
				"timestamp": time.Now(),
			}
			r.BroadcastToInstructors(sessionID, presenceUpdateMsg)
			log.Printf("Broadcast presence_update event for user %s joining session %s to instructors", userID, sessionID)

			// 2. Send current user list to the new connection (so they know who's already online)
			if sessionID == "lobby" {
				r.mu.RLock()
				var currentUsers []map[string]interface{}
				for existingUserID, existingConn := range r.globalConnections {
					if existingUserID != userID && existingConn.GetSessionID() == "lobby" {
						currentUsers = append(currentUsers, map[string]interface{}{
							"user_id":    existingUserID,
							"role":       existingConn.GetRole(),
							"session_id": existingConn.GetSessionID(),
						})
					}
				}
				r.mu.RUnlock()

				if len(currentUsers) > 0 {
					presenceMsg := map[string]interface{}{
						"type":    "system",
						"context": "lobby_users",
						"content": map[string]interface{}{
							"users":     currentUsers,
							"timestamp": time.Now(),
						},
					}
					if err := conn.WriteJSON(presenceMsg); err != nil {
						log.Printf("Failed to send lobby users to new connection %s: %v", userID, err)
					} else {
						log.Printf("Sent lobby users list to new user %s (%d users)", userID, len(currentUsers))
					}
				}
			}
		}()
	}

	return nil
}

// UnregisterConnection removes a specific connection from all maps atomically
// FUNCTIONAL DISCOVERY: Idempotent operation safe for concurrent unregistration
// RACE CONDITION FIX: Only removes the connection if it matches the one currently registered
func (r *Registry) UnregisterConnection(conn *Connection) {
	if conn == nil {
		return
	}

	userID := conn.GetUserID()
	r.mu.Lock()
	defer r.mu.Unlock()

	registeredConn, exists := r.globalConnections[userID]
	if !exists {
		return // Idempotent - no error if connection doesn't exist
	}

	// Only unregister if this is the same connection instance that's registered
	// This prevents old connections from unregistering newer connections during cleanup
	if registeredConn != conn {
		return // Different connection is now registered, don't remove it
	}

	role := conn.GetRole()
	sessionID := conn.GetSessionID()

	// Remove from global map
	delete(r.globalConnections, userID)

	// Remove from session-role map and clean up empty session maps
	// TECHNICAL DISCOVERY: Clean up empty maps to prevent memory leaks
	switch role {
	case "instructor":
		if instructors, exists := r.sessionInstructors[sessionID]; exists {
			delete(instructors, userID)
			if len(instructors) == 0 {
				delete(r.sessionInstructors, sessionID)
			}
		}
	case "student":
		if students, exists := r.sessionStudents[sessionID]; exists {
			delete(students, userID)
			if len(students) == 0 {
				delete(r.sessionStudents, sessionID)
			}
		}
	}

	// LOBBY SYSTEM: Broadcast simplified presence update when user disconnects
	r.broadcastWG.Add(1)
	go func() {
		defer r.broadcastWG.Done()
		presenceUpdateMsg := map[string]interface{}{
			"type": "system",
			"content": map[string]interface{}{
				"event":      "presence_update",
				"user_id":    userID,
				"role":       registeredConn.GetRole(),
				"session_id": nil, // null indicates disconnection
			},
			"timestamp": time.Now(),
		}
		r.BroadcastToInstructors(registeredConn.GetSessionID(), presenceUpdateMsg)
		log.Printf("Broadcast presence_update event for user %s disconnection to instructors", userID)
	}()
}

// GetUserConnection returns the current connection for a user with O(1) lookup
// ARCHITECTURAL DISCOVERY: Read-heavy access pattern benefits from RWMutex
// allowing concurrent reads without blocking during message routing
func (r *Registry) GetUserConnection(userID string) (*Connection, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conn, exists := r.globalConnections[userID]
	return conn, exists
}

// GetSessionConnections returns all connections in a session for broadcasting
// FUNCTIONAL DISCOVERY: Combines instructors and students into single slice
// for efficient iteration during session-wide message delivery
func (r *Registry) GetSessionConnections(sessionID string) []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	log.Printf("DEBUG: GetSessionConnections called for sessionID: %s", sessionID)

	var connections []*Connection

	// Add instructors
	if instructors, exists := r.sessionInstructors[sessionID]; exists {
		log.Printf("DEBUG: Found %d instructors in session %s", len(instructors), sessionID)
		for userID, conn := range instructors {
			log.Printf("DEBUG: Adding instructor connection - userID: %s, role: %s", userID, conn.GetRole())
			connections = append(connections, conn)
		}
	} else {
		log.Printf("DEBUG: No instructors found for session %s", sessionID)
	}

	// Add students
	if students, exists := r.sessionStudents[sessionID]; exists {
		log.Printf("DEBUG: Found %d students in session %s", len(students), sessionID)
		for userID, conn := range students {
			log.Printf("DEBUG: Adding student connection - userID: %s, role: %s", userID, conn.GetRole())
			connections = append(connections, conn)
		}
	} else {
		log.Printf("DEBUG: No students found for session %s", sessionID)
	}

	log.Printf("DEBUG: GetSessionConnections returning %d total connections for session %s", len(connections), sessionID)
	return connections
}

// GetSessionInstructors returns instructor connections for a session
// FUNCTIONAL DISCOVERY: Role-specific lookup enables efficient message routing
// for instructor-only message types (inbox_response, request)
func (r *Registry) GetSessionInstructors(sessionID string) []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var connections []*Connection
	if instructors, exists := r.sessionInstructors[sessionID]; exists {
		for _, conn := range instructors {
			connections = append(connections, conn)
		}
	}

	return connections
}

// GetSessionStudents returns student connections for a session
// FUNCTIONAL DISCOVERY: Student-specific lookup enables efficient broadcasting
// for instructor_broadcast message type targeting all session students
func (r *Registry) GetSessionStudents(sessionID string) []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var connections []*Connection
	if students, exists := r.sessionStudents[sessionID]; exists {
		for _, conn := range students {
			connections = append(connections, conn)
		}
	}

	return connections
}

// GetStats returns registry statistics for monitoring and debugging
// TECHNICAL DISCOVERY: Separate session count calculation for instructors and students
// provides insight into registry state without exposing internal structure
func (r *Registry) GetStats() map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Calculate unique sessions across instructor and student maps
	uniqueSessions := make(map[string]bool)
	for sessionID := range r.sessionInstructors {
		uniqueSessions[sessionID] = true
	}
	for sessionID := range r.sessionStudents {
		uniqueSessions[sessionID] = true
	}

	return map[string]int{
		"total_connections": len(r.globalConnections),
		"active_sessions":   len(uniqueSessions),
	}
}

// LOBBY SYSTEM: Methods for lobby-wide broadcasting and presence management

// GetAllConnections returns all connected users
// FUNCTIONAL DISCOVERY: Enables system-wide broadcasts for lobby functionality
func (r *Registry) GetAllConnections() []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var connections []*Connection
	for _, conn := range r.globalConnections {
		connections = append(connections, conn)
	}

	return connections
}

// GetLobbyConnections returns users not in any active session
// FUNCTIONAL DISCOVERY: Returns users in lobby state (sessionID == "lobby")
func (r *Registry) GetLobbyConnections() []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var connections []*Connection
	for _, conn := range r.globalConnections {
		if conn.GetSessionID() == "lobby" {
			connections = append(connections, conn)
		}
	}

	return connections
}

// BroadcastToAll sends message to all connected users
// ARCHITECTURAL DISCOVERY: System-wide broadcasting for presence and session events
func (r *Registry) BroadcastToAll(message interface{}) {
	r.mu.RLock()
	connections := make([]*Connection, 0, len(r.globalConnections))
	for _, conn := range r.globalConnections {
		connections = append(connections, conn)
	}
	r.mu.RUnlock()

	// Send to all connections
	successCount := 0
	var failedConnections []*Connection
	for _, conn := range connections {
		if err := conn.WriteJSON(message); err != nil {
			log.Printf("Failed to broadcast message to user %s: %v", conn.GetUserID(), err)
			// If write failed due to timeout or closed connection, mark for cleanup
			if err == ErrWriteTimeout || err == ErrConnectionClosed {
				failedConnections = append(failedConnections, conn)
			}
		} else {
			successCount++
		}
	}

	// Clean up failed connections to prevent future hangs
	for _, conn := range failedConnections {
		go func(c *Connection) {
			log.Printf("Cleaning up stuck connection for user %s", c.GetUserID())
			r.UnregisterConnection(c)
			c.Close()
		}(conn)
	}

	log.Printf("Broadcast message sent to %d/%d connected users", successCount, len(connections))
}

// BroadcastToUsers sends message to specific users by ID
// FUNCTIONAL DISCOVERY: Targeted broadcasting for session-specific events
func (r *Registry) BroadcastToUsers(userIDs []string, message interface{}) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	successCount := 0
	for _, userID := range userIDs {
		if conn, exists := r.globalConnections[userID]; exists {
			if err := conn.WriteJSON(message); err != nil {
				log.Printf("Failed to send message to user %s: %v", userID, err)
			} else {
				successCount++
			}
		}
	}

	log.Printf("Targeted message sent to %d/%d specified users", successCount, len(userIDs))
}

// BroadcastToInstructors sends message to all instructors in a session
// PERFORMANCE: Reduces broadcast load by targeting only instructors who need presence info
func (r *Registry) BroadcastToInstructors(sessionID string, message interface{}) {
	r.mu.RLock()
	instructors := make([]*Connection, 0)
	if sessionInstructors, exists := r.sessionInstructors[sessionID]; exists {
		for _, conn := range sessionInstructors {
			instructors = append(instructors, conn)
		}
	}
	r.mu.RUnlock()

	successCount := 0
	var failedConnections []*Connection
	for _, conn := range instructors {
		if err := conn.WriteJSON(message); err != nil {
			log.Printf("Failed to broadcast message to instructor %s: %v", conn.GetUserID(), err)
			if err == ErrWriteTimeout || err == ErrConnectionClosed {
				failedConnections = append(failedConnections, conn)
			}
		} else {
			successCount++
		}
	}

	// Clean up failed connections
	for _, conn := range failedConnections {
		go func(c *Connection) {
			log.Printf("Cleaning up stuck instructor connection for user %s", c.GetUserID())
			r.UnregisterConnection(c)
			c.Close()
		}(conn)
	}

	log.Printf("Broadcast message sent to %d/%d instructors in session %s", successCount, len(instructors), sessionID)
}

// WaitForBroadcasts waits for all async broadcast operations to complete
// TESTING SUPPORT: Use this in tests to ensure broadcasts finish before verification
func (r *Registry) WaitForBroadcasts() {
	r.broadcastWG.Wait()
}

// SetBatchMode enables/disables batch registration mode
// When enabled, presence_update broadcasts are suppressed during registration
func (r *Registry) SetBatchMode(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.batchMode = enabled
}

// SendBatchPresenceUpdate sends presence_update to instructors for all registered users in their sessions
// Should be called after batch registration is complete
func (r *Registry) SendBatchPresenceUpdate() {
	r.mu.RLock()
	// Group users by session
	sessionUsers := make(map[string][]map[string]interface{})
	for userID, conn := range r.globalConnections {
		sessionID := conn.GetSessionID()
		if sessionUsers[sessionID] == nil {
			sessionUsers[sessionID] = make([]map[string]interface{}, 0)
		}
		sessionUsers[sessionID] = append(sessionUsers[sessionID], map[string]interface{}{
			"user_id":    userID,
			"role":       conn.GetRole(),
			"session_id": sessionID,
		})
	}
	r.mu.RUnlock()

	totalUsers := 0
	// Send batch presence update to instructors in each session
	for sessionID, users := range sessionUsers {
		if len(users) > 0 {
			batchPresenceMsg := map[string]interface{}{
				"type": "system",
				"content": map[string]interface{}{
					"event": "batch_presence_update",
					"users": users,
				},
				"timestamp": time.Now(),
			}
			r.BroadcastToInstructors(sessionID, batchPresenceMsg)
			totalUsers += len(users)
		}
	}
	log.Printf("Sent batch presence update for %d users to instructors", totalUsers)
}

// TransitionUserToSession moves a user from lobby to a specific session
// PHASE 4 DISCOVERY: Atomic operation to prevent connection state inconsistencies
func (r *Registry) TransitionUserToSession(userID, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find user in global connections
	conn, exists := r.globalConnections[userID]
	if !exists {
		return nil // User not connected, no error
	}

	role := conn.GetRole()
	currentSessionID := conn.GetSessionID()

	// Only transition from lobby
	if currentSessionID != "lobby" {
		return nil // User not in lobby, no transition needed
	}

	// Remove from lobby
	switch role {
	case "student":
		if lobbyStudents, exists := r.sessionStudents["lobby"]; exists {
			delete(lobbyStudents, userID)
		}
	case "instructor":
		if lobbyInstructors, exists := r.sessionInstructors["lobby"]; exists {
			delete(lobbyInstructors, userID)
		}
	}

	// Add to target session
	switch role {
	case "student":
		if r.sessionStudents[sessionID] == nil {
			r.sessionStudents[sessionID] = make(map[string]*Connection)
		}
		r.sessionStudents[sessionID][userID] = conn
	case "instructor":
		if r.sessionInstructors[sessionID] == nil {
			r.sessionInstructors[sessionID] = make(map[string]*Connection)
		}
		r.sessionInstructors[sessionID][userID] = conn
	}

	// Update connection's session ID
	conn.SetSessionID(sessionID)

	log.Printf("DEBUG: Transitioned user %s (%s) from lobby to session %s", userID, role, sessionID)

	return nil
}

// TransitionUserToLobby moves a user from a session back to lobby
// FUNCTIONAL DISCOVERY: Atomic operation for session cleanup when sessions end
func (r *Registry) TransitionUserToLobby(userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find user in global connections
	conn, exists := r.globalConnections[userID]
	if !exists {
		return nil // User not connected, no error
	}

	role := conn.GetRole()
	currentSessionID := conn.GetSessionID()

	// Skip if already in lobby
	if currentSessionID == "lobby" {
		return nil
	}

	// Remove from current session
	switch role {
	case "student":
		if sessionStudents, exists := r.sessionStudents[currentSessionID]; exists {
			delete(sessionStudents, userID)
		}
	case "instructor":
		if sessionInstructors, exists := r.sessionInstructors[currentSessionID]; exists {
			delete(sessionInstructors, userID)
		}
	}

	// Add to lobby
	switch role {
	case "student":
		if r.sessionStudents["lobby"] == nil {
			r.sessionStudents["lobby"] = make(map[string]*Connection)
		}
		r.sessionStudents["lobby"][userID] = conn
	case "instructor":
		if r.sessionInstructors["lobby"] == nil {
			r.sessionInstructors["lobby"] = make(map[string]*Connection)
		}
		r.sessionInstructors["lobby"][userID] = conn
	}

	// Update connection's session ID
	conn.SetSessionID("lobby")

	log.Printf("🏛️ DEBUG: Transitioned user %s (%s) from session %s to lobby", userID, role, currentSessionID)
	return nil
}
