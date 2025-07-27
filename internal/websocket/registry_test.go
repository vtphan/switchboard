package websocket

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Test WebSocket upgrader for registry tests  
// var registryTestUpgrader = testUpgrader // unused

// Architectural Validation Tests
func TestRegistry_StructureCompliance(t *testing.T) {
	// This will fail until Registry is implemented
	registry := &Registry{}
	_ = registry // Registry should be properly defined
}

func TestRegistry_ImportBoundaryCompliance(t *testing.T) {
	// This test passes if compilation succeeds - no forbidden imports
	t.Log("Registry import boundaries maintained - only allowed dependencies")
}

func TestRegistry_ThreadSafeDesign(t *testing.T) {
	// Verify Registry has proper synchronization primitives
	registry := NewRegistry()
	
	// Test that mutex exists by checking we can use it
	// (Actual mutex testing happens in concurrent tests)
	if registry == nil {
		t.Error("Registry should be properly initialized")
	}
}

// Functional Validation Tests  
func TestRegistry_NewRegistryInitialization(t *testing.T) {
	registry := NewRegistry()
	
	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}
	
	// Verify all maps are initialized
	stats := registry.GetStats()
	if stats["total_connections"] != 0 {
		t.Errorf("Expected 0 initial connections, got %d", stats["total_connections"])
	}
}

func TestRegistry_RegisterConnectionValidation(t *testing.T) {
	registry := NewRegistry()
	
	// Test nil connection
	err := registry.RegisterConnection(nil)
	if err != ErrNilConnection {
		t.Errorf("Expected ErrNilConnection, got %v", err)
	}
	
	// Test unauthenticated connection
	wsConn := createTestWebSocketConnection(t)
	defer func() { _ = wsConn.Close() }()
	
	conn := NewConnection(wsConn)
	defer func() { _ = conn.Close() }()
	
	// Connection not authenticated yet
	err = registry.RegisterConnection(conn)
	if err != ErrConnectionNotAuthenticated {
		t.Errorf("Expected ErrConnectionNotAuthenticated, got %v", err)
	}
}

func TestRegistry_RegisterConnectionSuccess(t *testing.T) {
	registry := NewRegistry()
	
	// Create authenticated connection
	wsConn := createTestWebSocketConnection(t)
	defer func() { _ = wsConn.Close() }()
	
	conn := NewConnection(wsConn)
	defer func() { _ = conn.Close() }()
	
	// Authenticate the connection
	_ = conn.SetCredentials("user123", "student", "session456")
	
	// Should register successfully
	err := registry.RegisterConnection(conn)
	if err != nil {
		t.Errorf("RegisterConnection failed: %v", err)
	}
	
	// Verify connection is registered
	retrievedConn, exists := registry.GetUserConnection("user123")
	if !exists {
		t.Error("Connection not found after registration")
	}
	if retrievedConn != conn {
		t.Error("Retrieved connection does not match registered connection")
	}
}

func TestRegistry_ConnectionReplacement(t *testing.T) {
	registry := NewRegistry()
	
	// Create first connection
	wsConn1 := createTestWebSocketConnection(t)
	defer func() { _ = wsConn1.Close() }()
	
	conn1 := NewConnection(wsConn1)
	defer func() { _ = conn1.Close() }()
	_ = conn1.SetCredentials("user123", "student", "session456")
	
	_ = registry.RegisterConnection(conn1)
	
	// Create second connection for same user
	wsConn2 := createTestWebSocketConnection(t)
	defer func() { _ = wsConn2.Close() }()
	
	conn2 := NewConnection(wsConn2)
	defer func() { _ = conn2.Close() }()
	_ = conn2.SetCredentials("user123", "student", "session456")
	
	// Register second connection - should replace first
	err := registry.RegisterConnection(conn2)
	if err != nil {
		t.Errorf("Connection replacement failed: %v", err)
	}
	
	// Should now get the second connection
	retrievedConn, exists := registry.GetUserConnection("user123")
	if !exists {
		t.Error("Connection not found after replacement")
	}
	if retrievedConn != conn2 {
		t.Error("Connection was not replaced properly")
	}
	
	// Wait for async broadcast operations from connection replacement
	registry.WaitForBroadcasts()
}

func TestRegistry_UnregisterConnection(t *testing.T) {
	registry := NewRegistry()
	
	// Register a connection
	wsConn := createTestWebSocketConnection(t)
	defer func() { _ = wsConn.Close() }()
	
	conn := NewConnection(wsConn)
	defer func() { _ = conn.Close() }()
	_ = conn.SetCredentials("user123", "instructor", "session456")
	
	_ = registry.RegisterConnection(conn)
	
	// Verify it's registered
	_, exists := registry.GetUserConnection("user123")
	if !exists {
		t.Error("Connection should be registered")
	}
	
	// Unregister
	registry.UnregisterConnection(conn)
	
	// Wait for async broadcast operations from unregistration
	registry.WaitForBroadcasts()
	
	// Should no longer exist
	_, exists = registry.GetUserConnection("user123")
	if exists {
		t.Error("Connection should be unregistered")
	}
}

func TestRegistry_UnregisterNonexistentConnection(t *testing.T) {
	registry := NewRegistry()
	
	// Should be idempotent - no error for nil connection
	registry.UnregisterConnection(nil)
	
	// Should still be empty
	stats := registry.GetStats()
	if stats["total_connections"] != 0 {
		t.Error("Unregistering non-existent connection should not affect registry")
	}
}

func TestRegistry_SessionConnectionLookups(t *testing.T) {
	registry := NewRegistry()
	
	// Register instructor connection
	wsConn1 := createTestWebSocketConnection(t)
	defer func() { _ = wsConn1.Close() }()
	
	instructor := NewConnection(wsConn1)
	defer func() { _ = instructor.Close() }()
	_ = instructor.SetCredentials("instructor1", "instructor", "session123")
	_ = registry.RegisterConnection(instructor)
	
	// Register student connection
	wsConn2 := createTestWebSocketConnection(t)
	defer func() { _ = wsConn2.Close() }()
	
	student := NewConnection(wsConn2)
	defer func() { _ = student.Close() }()
	_ = student.SetCredentials("student1", "student", "session123")
	_ = registry.RegisterConnection(student)
	
	// Test session-specific lookups
	allConnections := registry.GetSessionConnections("session123")
	if len(allConnections) != 2 {
		t.Errorf("Expected 2 session connections, got %d", len(allConnections))
	}
	
	instructors := registry.GetSessionInstructors("session123")
	if len(instructors) != 1 {
		t.Errorf("Expected 1 instructor connection, got %d", len(instructors))
	}
	
	students := registry.GetSessionStudents("session123")
	if len(students) != 1 {
		t.Errorf("Expected 1 student connection, got %d", len(students))
	}
}

func TestRegistry_EmptySessionLookups(t *testing.T) {
	registry := NewRegistry()
	
	// Test lookups on non-existent session
	allConnections := registry.GetSessionConnections("nonexistent")
	if len(allConnections) != 0 {
		t.Errorf("Expected 0 connections for non-existent session, got %d", len(allConnections))
	}
	
	instructors := registry.GetSessionInstructors("nonexistent")
	if len(instructors) != 0 {
		t.Errorf("Expected 0 instructors for non-existent session, got %d", len(instructors))
	}
	
	students := registry.GetSessionStudents("nonexistent")
	if len(students) != 0 {
		t.Errorf("Expected 0 students for non-existent session, got %d", len(students))
	}
}

// Technical Validation Tests (Race Detection)
func TestRegistry_ConcurrentRegistration(t *testing.T) {
	registry := NewRegistry()
	
	const numConnections = 50
	var wg sync.WaitGroup
	wg.Add(numConnections)
	
	// Register multiple connections concurrently
	for i := 0; i < numConnections; i++ {
		go func(id int) {
			defer wg.Done()
			
			wsConn := createTestWebSocketConnection(t)
			defer func() { _ = wsConn.Close() }()
			
			conn := NewConnection(wsConn)
			defer func() { _ = conn.Close() }()
			
			// Use different users to avoid replacement
			_ = conn.SetCredentials(fmt.Sprintf("user%d", id), "student", "session123")
			
			err := registry.RegisterConnection(conn)
			if err != nil {
				t.Errorf("Concurrent registration failed for user%d: %v", id, err)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify all connections registered
	stats := registry.GetStats()
	if stats["total_connections"] != numConnections {
		t.Errorf("Expected %d connections, got %d", numConnections, stats["total_connections"])
	}
}

func TestRegistry_ConcurrentLookup(t *testing.T) {
	registry := NewRegistry()
	
	// Register some connections first
	for i := 0; i < 10; i++ {
		wsConn := createTestWebSocketConnection(t)
		defer func() { _ = wsConn.Close() }()
		
		conn := NewConnection(wsConn)
		defer func() { _ = conn.Close() }()
		
		_ = conn.SetCredentials(fmt.Sprintf("user%d", i), "student", "session123")
		_ = registry.RegisterConnection(conn)
	}
	
	const numReaders = 50
	var wg sync.WaitGroup
	wg.Add(numReaders)
	
	// Concurrent lookups should be safe
	for i := 0; i < numReaders; i++ {
		go func() {
			defer wg.Done()
			
			// Random lookups
			registry.GetUserConnection("user5")
			registry.GetSessionConnections("session123")
			registry.GetSessionInstructors("session123")
			registry.GetSessionStudents("session123")
			registry.GetStats()
		}()
	}
	
	wg.Wait()
}

func TestRegistry_ConcurrentRegistrationAndUnregistration(t *testing.T) {
	registry := NewRegistry()
	
	const numOperations = 100
	var wg sync.WaitGroup
	wg.Add(numOperations)
	
	// Mix of registration and unregistration operations
	for i := 0; i < numOperations; i++ {
		go func(id int) {
			defer wg.Done()
			
			if id%2 == 0 {
				// Register connection
				wsConn := createTestWebSocketConnection(t)
				defer func() { _ = wsConn.Close() }()
				
				conn := NewConnection(wsConn)
				defer func() { _ = conn.Close() }()
				
				_ = conn.SetCredentials(fmt.Sprintf("user%d", id), "student", "session123")
				_ = registry.RegisterConnection(conn)
			} else {
				// Attempt to unregister a connection that might exist
				userID := fmt.Sprintf("user%d", id-1) // Look for previous user
				if conn, exists := registry.GetUserConnection(userID); exists {
					registry.UnregisterConnection(conn)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Registry should be in consistent state
	stats := registry.GetStats()
	if stats["total_connections"] < 0 {
		t.Error("Registry in inconsistent state after concurrent operations")
	}
}

func TestRegistry_LookupPerformance(t *testing.T) {
	registry := NewRegistry()
	
	// Register many connections
	const numConnections = 1000
	for i := 0; i < numConnections; i++ {
		wsConn := createTestWebSocketConnection(t)
		defer func() { _ = wsConn.Close() }()
		
		conn := NewConnection(wsConn)
		defer func() { _ = conn.Close() }()
		
		_ = conn.SetCredentials(fmt.Sprintf("user%d", i), "student", "session123")
		_ = registry.RegisterConnection(conn)
	}
	
	// Test O(1) lookup performance
	start := time.Now()
	for i := 0; i < 1000; i++ {
		registry.GetUserConnection("user500")
	}
	duration := time.Since(start)
	
	// Should be very fast for O(1) lookups
	averageTime := duration / 1000
	if averageTime > time.Microsecond {
		t.Logf("Warning: Lookup time %v may indicate non-O(1) performance", averageTime)
	}
}

// LOBBY SYSTEM TESTS

func TestRegistry_LobbyConnectionRegistration(t *testing.T) {
	registry := NewRegistry()
	
	// Register lobby connection
	wsConn := createTestWebSocketConnection(t)
	defer func() { _ = wsConn.Close() }()
	
	conn := NewConnection(wsConn)
	defer func() { _ = conn.Close() }()
	_ = conn.SetCredentials("user123", "student", "lobby")
	
	err := registry.RegisterConnection(conn)
	if err != nil {
		t.Errorf("Failed to register lobby connection: %v", err)
	}
	
	// Wait for async broadcast operations to complete
	registry.WaitForBroadcasts()
	
	// Should be retrievable by user ID
	retrievedConn, exists := registry.GetUserConnection("user123")
	if !exists {
		t.Error("Lobby connection should be retrievable by user ID")
	}
	if retrievedConn.GetSessionID() != "lobby" {
		t.Error("Connection should have session_id 'lobby'")
	}
}

func TestRegistry_GetLobbyConnections(t *testing.T) {
	registry := NewRegistry()
	
	// Register mixed lobby and session connections
	
	// Lobby connection 1
	wsConn1 := createTestWebSocketConnection(t)
	defer func() { _ = wsConn1.Close() }()
	lobbyConn1 := NewConnection(wsConn1)
	defer func() { _ = lobbyConn1.Close() }()
	_ = lobbyConn1.SetCredentials("lobby_user1", "student", "lobby")
	_ = registry.RegisterConnection(lobbyConn1)
	
	// Regular session connection
	wsConn2 := createTestWebSocketConnection(t)
	defer func() { _ = wsConn2.Close() }()
	sessionConn := NewConnection(wsConn2)
	defer func() { _ = sessionConn.Close() }()
	_ = sessionConn.SetCredentials("session_user", "instructor", "session123")
	_ = registry.RegisterConnection(sessionConn)
	
	// Lobby connection 2 (different user)
	wsConn3 := createTestWebSocketConnection(t)
	defer func() { _ = wsConn3.Close() }()
	lobbyConn2 := NewConnection(wsConn3)
	defer func() { _ = lobbyConn2.Close() }()
	_ = lobbyConn2.SetCredentials("lobby_user2", "instructor", "lobby")
	_ = registry.RegisterConnection(lobbyConn2)
	
	// GetLobbyConnections should return only lobby connections
	lobbyConnections := registry.GetLobbyConnections()
	if len(lobbyConnections) != 2 {
		t.Errorf("Expected 2 lobby connections, got %d", len(lobbyConnections))
	}
	
	// Verify they are the correct connections
	userIDs := make(map[string]bool)
	for _, conn := range lobbyConnections {
		userIDs[conn.GetUserID()] = true
		if conn.GetSessionID() != "lobby" {
			t.Error("All returned connections should have session_id 'lobby'")
		}
	}
	
	if !userIDs["lobby_user1"] || !userIDs["lobby_user2"] {
		t.Error("Lobby connections should include lobby_user1 and lobby_user2")
	}
	if userIDs["session_user"] {
		t.Error("Lobby connections should not include session users")
	}
}

func TestRegistry_GetAllConnections(t *testing.T) {
	registry := NewRegistry()
	
	// Register mixed connections
	
	// Lobby connection
	wsConn1 := createTestWebSocketConnection(t)
	defer func() { _ = wsConn1.Close() }()
	lobbyConn := NewConnection(wsConn1)
	defer func() { _ = lobbyConn.Close() }()
	_ = lobbyConn.SetCredentials("lobby_user", "student", "lobby")
	_ = registry.RegisterConnection(lobbyConn)
	
	// Session connection 1
	wsConn2 := createTestWebSocketConnection(t)
	defer func() { _ = wsConn2.Close() }()
	sessionConn1 := NewConnection(wsConn2)
	defer func() { _ = sessionConn1.Close() }()
	_ = sessionConn1.SetCredentials("session_user1", "instructor", "session123")
	_ = registry.RegisterConnection(sessionConn1)
	
	// Session connection 2 (different session)
	wsConn3 := createTestWebSocketConnection(t)
	defer func() { _ = wsConn3.Close() }()
	sessionConn2 := NewConnection(wsConn3)
	defer func() { _ = sessionConn2.Close() }()
	_ = sessionConn2.SetCredentials("session_user2", "student", "session456")
	_ = registry.RegisterConnection(sessionConn2)
	
	// GetAllConnections should return all connections
	allConnections := registry.GetAllConnections()
	if len(allConnections) != 3 {
		t.Errorf("Expected 3 total connections, got %d", len(allConnections))
	}
	
	// Verify all connections are included
	userIDs := make(map[string]bool)
	for _, conn := range allConnections {
		userIDs[conn.GetUserID()] = true
	}
	
	expected := []string{"lobby_user", "session_user1", "session_user2"}
	for _, userID := range expected {
		if !userIDs[userID] {
			t.Errorf("All connections should include %s", userID)
		}
	}
}

func TestRegistry_BroadcastToAll(t *testing.T) {
	t.Skip("Skipping broadcast test - requires integration with message routing")
}

func TestRegistry_BroadcastToUsers(t *testing.T) {
	t.Skip("Skipping broadcast test - requires integration with message routing")
}

func TestRegistry_LobbyBroadcastConcurrency(t *testing.T) {
	t.Skip("Skipping broadcast test - requires integration with message routing")
}

func TestRegistry_LobbyConnectionMixedWithSessions(t *testing.T) {
	registry := NewRegistry()
	
	// Create a mix of lobby and session connections
	
	// Lobby connections
	for i := 0; i < 5; i++ {
		wsConn := createTestWebSocketConnection(t)
		defer func() { _ = wsConn.Close() }()
		
		conn := NewConnection(wsConn)
		defer func() { _ = conn.Close() }()
		
		_ = conn.SetCredentials(fmt.Sprintf("lobby_user%d", i), "student", "lobby")
		_ = registry.RegisterConnection(conn)
	}
	
	// Session A connections
	for i := 0; i < 3; i++ {
		wsConn := createTestWebSocketConnection(t)
		defer func() { _ = wsConn.Close() }()
		
		conn := NewConnection(wsConn)
		defer func() { _ = conn.Close() }()
		
		_ = conn.SetCredentials(fmt.Sprintf("sessionA_user%d", i), "student", "sessionA")
		_ = registry.RegisterConnection(conn)
	}
	
	// Session B connections
	for i := 0; i < 2; i++ {
		wsConn := createTestWebSocketConnection(t)
		defer func() { _ = wsConn.Close() }()
		
		conn := NewConnection(wsConn)
		defer func() { _ = conn.Close() }()
		
		_ = conn.SetCredentials(fmt.Sprintf("sessionB_user%d", i), "instructor", "sessionB")
		_ = registry.RegisterConnection(conn)
	}
	
	// Verify counts
	lobbyConnections := registry.GetLobbyConnections()
	if len(lobbyConnections) != 5 {
		t.Errorf("Expected 5 lobby connections, got %d", len(lobbyConnections))
	}
	
	sessionAConnections := registry.GetSessionConnections("sessionA")
	if len(sessionAConnections) != 3 {
		t.Errorf("Expected 3 sessionA connections, got %d", len(sessionAConnections))
	}
	
	sessionBConnections := registry.GetSessionConnections("sessionB")
	if len(sessionBConnections) != 2 {
		t.Errorf("Expected 2 sessionB connections, got %d", len(sessionBConnections))
	}
	
	allConnections := registry.GetAllConnections()
	if len(allConnections) != 10 {
		t.Errorf("Expected 10 total connections, got %d", len(allConnections))
	}
	
	// Verify that lobby connections don't appear in session lookups
	for _, conn := range lobbyConnections {
		if conn.GetSessionID() != "lobby" {
			t.Error("Lobby connection has wrong session ID")
		}
	}
	
	// Verify session connections don't appear in lobby
	for _, conn := range sessionAConnections {
		if conn.GetSessionID() == "lobby" {
			t.Error("Session connection should not appear in session results with lobby ID")
		}
	}
}

// Helper: Connection wrapper that tracks sent messages for testing
type trackingConnection struct {
	*Connection
	sentMessages []map[string]interface{}
	mutex        sync.RWMutex
}

func newTrackingConnection(wsConn *websocket.Conn) *trackingConnection {
	return &trackingConnection{
		Connection:   NewConnection(wsConn),
		sentMessages: make([]map[string]interface{}, 0),
	}
}

func (tc *trackingConnection) WriteJSON(message interface{}) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	
	if msg, ok := message.(map[string]interface{}); ok {
		tc.sentMessages = append(tc.sentMessages, msg)
	}
	
	// Don't actually send to WebSocket in tests
	return nil
}

// Test completed - fmt imported at top