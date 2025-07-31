package websocket

import (
	"errors"
	"log"
	"sync"
	"time"

	"switchboard/internal/message"
	"switchboard/internal/session"
	"switchboard/pkg/config"
	pkgErrors "switchboard/pkg/errors"
)

// ConnectionRegistry manages active WebSocket connections with thread-safe operations
// and automatic cleanup of stale connections. Implements the RecipientRegistry interface
// from Phase 3 message routing system.
type ConnectionRegistry struct {
	mu             sync.RWMutex                     // RWMutex for read-heavy access patterns
	connections    map[string]ConnectionInterface   // userID -> Connection mapping
	heartbeats     map[string]time.Time             // userID -> last heartbeat time for inactive session cleanup
	heartbeatMu    sync.RWMutex                     // Separate mutex for heartbeat tracking
	sessionManager session.SessionManager           // Session manager to check for active sessions
	cleanupTicker  *time.Ticker                     // Periodic cleanup ticker (30 seconds)
	stopCh         chan struct{}                    // Channel for stopping cleanup goroutine
}

// NewConnectionRegistry creates a new connection registry with proper initialization.
// The registry is configured with a 30-second cleanup ticker and is ready for use,
// but the cleanup goroutine must be started manually with Start().
func NewConnectionRegistry(sessionManager session.SessionManager) *ConnectionRegistry {
	return &ConnectionRegistry{
		connections:    make(map[string]ConnectionInterface),
		heartbeats:     make(map[string]time.Time),
		sessionManager: sessionManager,
		cleanupTicker:  time.NewTicker(config.ConnectionCleanupInterval), // 30 seconds
		stopCh:         make(chan struct{}),
	}
}

// Register adds a new connection to the registry in a thread-safe manner.
// If a connection with the same userID already exists, the old connection
// is closed and replaced with the new one. This prevents duplicate connections
// for the same user and ensures proper resource cleanup.
func (cr *ConnectionRegistry) Register(userID string, conn ConnectionInterface) error {
	if userID == "" {
		return errors.New("userID required")
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Check if user already connected - close existing connection
	if existingConn, exists := cr.connections[userID]; exists {
		// Use CloseWithCode to notify client it was replaced
		if closer, ok := existingConn.(*Connection); ok {
			_ = closer.CloseWithCode(4001, "replaced by newer connection")
		} else {
			_ = existingConn.Close()
		}
	}

	// Store new connection
	cr.connections[userID] = conn
	
	// Initialize heartbeat tracking
	cr.heartbeatMu.Lock()
	cr.heartbeats[userID] = time.Now()
	cr.heartbeatMu.Unlock()
	
	return nil
}

// Unregister removes a connection from the registry and closes it.
// This method is idempotent - calling it multiple times with the same
// userID will not cause errors. The connection is properly closed to
// prevent resource leaks.
func (cr *ConnectionRegistry) Unregister(userID string) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if conn, exists := cr.connections[userID]; exists {
		_ = conn.Close()
		delete(cr.connections, userID)
		
		// Clean up heartbeat tracking
		cr.heartbeatMu.Lock()
		delete(cr.heartbeats, userID)
		cr.heartbeatMu.Unlock()
	}
}

// GetInstructors returns all connected users with "instructor" role.
// This method uses RLock for efficient concurrent access and returns
// a slice of Recipient interfaces for compatibility with the message
// routing system from Phase 3.
func (cr *ConnectionRegistry) GetInstructors() []message.Recipient {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	var instructors []message.Recipient
	for _, conn := range cr.connections {
		if conn.GetRole() == "instructor" {
			instructors = append(instructors, conn)
		}
	}
	return instructors
}

// GetStudents returns all connected users with "student" role.
// This method uses RLock for efficient concurrent access and returns
// a slice of Recipient interfaces for compatibility with the message
// routing system from Phase 3.
func (cr *ConnectionRegistry) GetStudents() []message.Recipient {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	var students []message.Recipient
	for _, conn := range cr.connections {
		if conn.GetRole() == "student" {
			students = append(students, conn)
		}
	}
	return students
}

// GetUserByID retrieves a specific user's connection by their userID.
// Returns ErrConnectionNotFound if the user is not connected. This method
// uses RLock for efficient concurrent access.
func (cr *ConnectionRegistry) GetUserByID(userID string) (message.Recipient, error) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	conn, exists := cr.connections[userID]
	if !exists {
		return nil, pkgErrors.ErrConnectionNotFound
	}
	return conn, nil
}

// GetAllUsers returns all connected users regardless of role.
// This method uses RLock for efficient concurrent access and returns
// a slice of Recipient interfaces for compatibility with the message
// routing system from Phase 3. The returned slice is pre-allocated
// for optimal memory usage.
func (cr *ConnectionRegistry) GetAllUsers() []message.Recipient {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	recipients := make([]message.Recipient, 0, len(cr.connections))
	for _, conn := range cr.connections {
		recipients = append(recipients, conn)
	}
	return recipients
}

// IsRegistered checks if a user is currently registered in the connection registry.
// Returns true if the user has an active connection, false otherwise.
// This method uses RLock for efficient concurrent access.
func (cr *ConnectionRegistry) IsRegistered(userID string) bool {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	
	_, exists := cr.connections[userID]
	return exists
}

// GetConnectedUsers implements the ConnectionProvider interface from Phase 3.
// This enables the registry to work with the message routing system.
// It returns the same result as GetAllUsers() but through the interface contract.
func (cr *ConnectionRegistry) GetConnectedUsers() ([]message.Recipient, error) {
	return cr.GetAllUsers(), nil
}

// UpdateHeartbeat updates the last heartbeat time for a user.
// This is called by connections when they receive heartbeat/activity updates.
func (cr *ConnectionRegistry) UpdateHeartbeat(userID string) {
	cr.heartbeatMu.Lock()
	defer cr.heartbeatMu.Unlock()
	
	cr.heartbeats[userID] = time.Now()
}

// Start launches the background cleanup goroutine that removes stale connections
// every 30 seconds. This goroutine runs until Stop() is called.
func (cr *ConnectionRegistry) Start() {
	go cr.cleanupLoop()
}

// cleanupLoop is the main loop for the background cleanup goroutine.
// It listens for cleanup ticker events and stop signals, calling
// cleanupStaleConnections() periodically to remove inactive connections.
func (cr *ConnectionRegistry) cleanupLoop() {
	for {
		select {
		case <-cr.cleanupTicker.C:
			cr.cleanupStaleConnections()
		case <-cr.stopCh:
			return
		}
	}
}

// cleanupStaleConnections removes connections based on session-aware timeout rules:
// - During active session: No timeouts (connections stay alive)
// - During no active session: 25-minute timeout for inactive connections
func (cr *ConnectionRegistry) cleanupStaleConnections() {
	// Check if there's an active session
	if cr.sessionManager.HasActiveSession() {
		// During active session: No connection timeouts
		return
	}
	
	// No active session: Apply 25-minute timeout
	cr.mu.Lock()
	defer cr.mu.Unlock()
	
	cr.heartbeatMu.RLock()
	defer cr.heartbeatMu.RUnlock()
	
	cutoff := time.Now().Add(-config.InactiveConnectionTimeout) // 25 minutes ago
	cleaned := 0

	for userID, conn := range cr.connections {
		// Check heartbeat time instead of connection's last seen
		if lastHeartbeat, exists := cr.heartbeats[userID]; exists {
			if lastHeartbeat.Before(cutoff) {
				_ = conn.Close()
				delete(cr.connections, userID)
				// Note: heartbeat cleanup happens in Unregister
				cleaned++
			}
		} else {
			// No heartbeat record - remove immediately
			_ = conn.Close()
			delete(cr.connections, userID)
			cleaned++
		}
	}

	if cleaned > 0 {
		log.Printf("ConnectionRegistry: Cleaned up %d inactive connections (no active session)", cleaned)
	}
}

// Stop performs graceful shutdown of the connection registry.
// It stops the cleanup goroutine, sends close messages to all connections,
// closes all active connections, and clears the connections map. 
// This method is idempotent and can be called multiple times safely.
func (cr *ConnectionRegistry) Stop() {
	log.Println("ConnectionRegistry: Starting graceful shutdown...")
	
	// Try to close stop channel - if already closed, this will panic
	// but we recover and continue with cleanup
	defer func() {
		if r := recover(); r != nil {
			// Stop channel already closed, continue with cleanup
			_ = r // Use the recovered value to avoid SA9003
		}
	}()
	
	close(cr.stopCh)
	cr.cleanupTicker.Stop()

	// Send close messages and close all connections
	cr.mu.Lock()
	defer cr.mu.Unlock()

	closeCount := len(cr.connections)
	for userID, conn := range cr.connections {
		// Send graceful close message to client
		if err := conn.SendCloseMessage("Server shutting down"); err != nil {
			log.Printf("ConnectionRegistry: Failed to send close message to user %s: %v", userID, err)
		}
		
		// Close the connection
		if err := conn.Close(); err != nil {
			log.Printf("ConnectionRegistry: Failed to close connection for user %s: %v", userID, err)
		}
		
		delete(cr.connections, userID)
	}
	
	// Clean up heartbeat tracking
	cr.heartbeatMu.Lock()
	cr.heartbeats = make(map[string]time.Time)
	cr.heartbeatMu.Unlock()
	
	log.Printf("ConnectionRegistry: Gracefully closed %d connections", closeCount)
}

// Test helper methods - only included in non-production builds
// These methods expose internal state for testing purposes

// CleanupStaleConnectionsForTest exposes the cleanup logic for testing
func (cr *ConnectionRegistry) CleanupStaleConnectionsForTest() {
	cr.cleanupStaleConnections()
}

// SetHeartbeatTimeForTest allows tests to manually set heartbeat times
func (cr *ConnectionRegistry) SetHeartbeatTimeForTest(userID string, t time.Time) {
	cr.heartbeatMu.Lock()
	defer cr.heartbeatMu.Unlock()
	cr.heartbeats[userID] = t
}

// GetHeartbeatTimeForTest allows tests to read heartbeat times
func (cr *ConnectionRegistry) GetHeartbeatTimeForTest(userID string) time.Time {
	cr.heartbeatMu.RLock()
	defer cr.heartbeatMu.RUnlock()
	return cr.heartbeats[userID]
}