package websocket

import (
	"log"
	"sync"
)

// Registry manages WebSocket connections with thread-safe operations
// ARCHITECTURAL DISCOVERY: Pure connection management without business logic
// maintains clean separation between connection tracking and connection operations
type Registry struct {
	mu                  sync.RWMutex                          // TECHNICAL DISCOVERY: RWMutex optimizes for read-heavy lookup patterns
	globalConnections   map[string]*Connection                // userID -> Connection for O(1) global lookup
	sessionInstructors  map[string]map[string]*Connection     // sessionID -> userID -> Connection
	sessionStudents     map[string]map[string]*Connection     // sessionID -> userID -> Connection
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
	
	// FUNCTIONAL DISCOVERY: Close existing connection asynchronously to prevent deadlock
	// during registration while ensuring immediate replacement
	if existingConn, exists := r.globalConnections[userID]; exists {
		go func() {
			if err := existingConn.Close(); err != nil {
				log.Printf("Failed to close existing connection: %v", err)
			}
		}() // Close asynchronously to avoid deadlock
	}
	
	// Add to global map for O(1) user lookup
	r.globalConnections[userID] = conn
	
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
	
	var connections []*Connection
	
	// Add instructors
	if instructors, exists := r.sessionInstructors[sessionID]; exists {
		for _, conn := range instructors {
			connections = append(connections, conn)
		}
	}
	
	// Add students
	if students, exists := r.sessionStudents[sessionID]; exists {
		for _, conn := range students {
			connections = append(connections, conn)
		}
	}
	
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