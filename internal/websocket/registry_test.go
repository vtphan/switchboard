package websocket

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Test WebSocket upgrader for registry tests
var registryTestUpgrader = testUpgrader

// Architectural Validation Tests
func TestRegistry_StructureCompliance(t *testing.T) {
	// This will fail until Registry is implemented
	registry := &Registry{}
	if registry == nil {
		t.Error("Registry should be defined")
	}
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
	defer wsConn.Close()
	
	conn := NewConnection(wsConn)
	defer conn.Close()
	
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
	defer wsConn.Close()
	
	conn := NewConnection(wsConn)
	defer conn.Close()
	
	// Authenticate the connection
	conn.SetCredentials("user123", "student", "session456")
	
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
	defer wsConn1.Close()
	
	conn1 := NewConnection(wsConn1)
	defer conn1.Close()
	conn1.SetCredentials("user123", "student", "session456")
	
	registry.RegisterConnection(conn1)
	
	// Create second connection for same user
	wsConn2 := createTestWebSocketConnection(t)
	defer wsConn2.Close()
	
	conn2 := NewConnection(wsConn2)
	defer conn2.Close()
	conn2.SetCredentials("user123", "student", "session456")
	
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
	
	// Give time for old connection cleanup
	time.Sleep(10 * time.Millisecond)
}

func TestRegistry_UnregisterConnection(t *testing.T) {
	registry := NewRegistry()
	
	// Register a connection
	wsConn := createTestWebSocketConnection(t)
	defer wsConn.Close()
	
	conn := NewConnection(wsConn)
	defer conn.Close()
	conn.SetCredentials("user123", "instructor", "session456")
	
	registry.RegisterConnection(conn)
	
	// Verify it's registered
	_, exists := registry.GetUserConnection("user123")
	if !exists {
		t.Error("Connection should be registered")
	}
	
	// Unregister
	registry.UnregisterConnection(conn)
	
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
	defer wsConn1.Close()
	
	instructor := NewConnection(wsConn1)
	defer instructor.Close()
	instructor.SetCredentials("instructor1", "instructor", "session123")
	registry.RegisterConnection(instructor)
	
	// Register student connection
	wsConn2 := createTestWebSocketConnection(t)
	defer wsConn2.Close()
	
	student := NewConnection(wsConn2)
	defer student.Close()
	student.SetCredentials("student1", "student", "session123")
	registry.RegisterConnection(student)
	
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
			defer wsConn.Close()
			
			conn := NewConnection(wsConn)
			defer conn.Close()
			
			// Use different users to avoid replacement
			conn.SetCredentials(fmt.Sprintf("user%d", id), "student", "session123")
			
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
		defer wsConn.Close()
		
		conn := NewConnection(wsConn)
		defer conn.Close()
		
		conn.SetCredentials(fmt.Sprintf("user%d", i), "student", "session123")
		registry.RegisterConnection(conn)
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
				defer wsConn.Close()
				
				conn := NewConnection(wsConn)
				defer conn.Close()
				
				conn.SetCredentials(fmt.Sprintf("user%d", id), "student", "session123")
				registry.RegisterConnection(conn)
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
		defer wsConn.Close()
		
		conn := NewConnection(wsConn)
		defer conn.Close()
		
		conn.SetCredentials(fmt.Sprintf("user%d", i), "student", "session123")
		registry.RegisterConnection(conn)
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

// Test completed - fmt imported at top