package websocket

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"switchboard/internal/database"
	"switchboard/internal/message"
	"switchboard/pkg/config"
	pkgErrors "switchboard/pkg/errors"
)

// Mock Connection for testing registry
type mockConnection struct {
	userID   string
	role     string
	lastSeen time.Time
	closed   bool
	mu       sync.RWMutex
}

func newMockConnection(userID, role string) *mockConnection {
	return &mockConnection{
		userID:   userID,
		role:     role,
		lastSeen: time.Now(),
		closed:   false,
	}
}

func (m *mockConnection) GetUserID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.userID
}

func (m *mockConnection) GetRole() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.role
}

func (m *mockConnection) GetLastSeen() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastSeen
}

func (m *mockConnection) SetLastSeen(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSeen = t
}

func (m *mockConnection) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockConnection) IsClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.closed
}

func (m *mockConnection) SendMessage(data []byte) error {
	if m.IsClosed() {
		return pkgErrors.ErrConnectionNotFound
	}
	return nil
}

// Additional methods to implement ConnectionInterface
func (m *mockConnection) WriteJSON(v interface{}) error {
	if m.IsClosed() {
		return pkgErrors.ErrConnectionNotFound
	}
	return nil
}

func (m *mockConnection) SetCredentials(username, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userID = username
	m.role = role
	return nil
}

func (m *mockConnection) UpdateActivity() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastSeen = time.Now()
}

func (m *mockConnection) SendCloseMessage(reason string) error {
	if m.IsClosed() {
		return pkgErrors.ErrConnectionNotFound
	}
	// Mock implementation - just mark as closed
	return m.Close()
}

// Core Registry Structure Tests

// Test 1: NewConnectionRegistry Creates Valid Instance
func TestNewConnectionRegistry_CreatesValidInstance(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.connections)
	assert.NotNil(t, registry.heartbeats)
	assert.NotNil(t, registry.cleanupTicker)
	assert.NotNil(t, registry.stopCh)
	assert.Equal(t, 0, len(registry.connections))
	assert.Equal(t, 0, len(registry.heartbeats))
}

// Test 2: Registry Struct Fields
func TestConnectionRegistry_StructFields(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	
	// Verify RWMutex is available
	assert.NotNil(t, &registry.mu)
	
	// Verify connections map initialized
	assert.NotNil(t, registry.connections)
	assert.Equal(t, 0, len(registry.connections))
	
	// Verify heartbeats map initialized
	assert.NotNil(t, registry.heartbeats)
	assert.Equal(t, 0, len(registry.heartbeats))
	
	// Verify cleanup ticker configured correctly
	// Note: We can't directly check interval, but verify ticker exists
	assert.NotNil(t, registry.cleanupTicker)
	
	// Verify stop channel
	assert.NotNil(t, registry.stopCh)
}

// Registration Tests

// Test 3: Register Connection Success
func TestConnectionRegistry_Register_Success(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	conn := newMockConnection("user1", "student")
	
	err := registry.Register("user1", conn)
	assert.NoError(t, err)
	
	// Verify connection is stored
	registry.mu.RLock()
	storedConn, exists := registry.connections["user1"]
	registry.mu.RUnlock()
	
	assert.True(t, exists)
	assert.Equal(t, conn, storedConn)
}

// Test 4: Register Connection Validation
func TestConnectionRegistry_Register_Validation(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	conn := newMockConnection("user1", "student")
	
	// Test empty userID
	err := registry.Register("", conn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "userID required")
}

// Test 5: Register Replaces Existing Connection
func TestConnectionRegistry_Register_ReplacesExisting(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	// Register first connection
	conn1 := newMockConnection("user1", "student")
	err := registry.Register("user1", conn1)
	require.NoError(t, err)
	
	// Register second connection with same userID
	conn2 := newMockConnection("user1", "instructor")
	err = registry.Register("user1", conn2)
	require.NoError(t, err)
	
	// Verify first connection was closed
	assert.True(t, conn1.IsClosed())
	
	// Verify second connection is active
	registry.mu.RLock()
	storedConn := registry.connections["user1"]
	registry.mu.RUnlock()
	
	assert.Equal(t, conn2, storedConn)
	assert.False(t, conn2.IsClosed())
}

// Test 6: Unregister Connection
func TestConnectionRegistry_Unregister(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	conn := newMockConnection("user1", "student")
	err := registry.Register("user1", conn)
	require.NoError(t, err)
	
	// Unregister connection
	registry.Unregister("user1")
	
	// Verify connection was closed and removed
	assert.True(t, conn.IsClosed())
	
	registry.mu.RLock()
	_, exists := registry.connections["user1"]
	registry.mu.RUnlock()
	
	assert.False(t, exists)
}

// Test 7: Unregister Non-existent Connection
func TestConnectionRegistry_Unregister_NonExistent(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	// Should not panic or error
	registry.Unregister("nonexistent")
	
	// Verify registry is still empty
	registry.mu.RLock()
	count := len(registry.connections)
	registry.mu.RUnlock()
	
	assert.Equal(t, 0, count)
}

// Role-Based Lookup Tests

// Test 8: GetInstructors
func TestConnectionRegistry_GetInstructors(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	// Register mixed roles
	instructor1 := newMockConnection("instructor1", "instructor")
	instructor2 := newMockConnection("instructor2", "instructor")
	student1 := newMockConnection("student1", "student")
	
	_ = registry.Register("instructor1", instructor1)
	_ = registry.Register("instructor2", instructor2)
	_ = registry.Register("student1", student1)
	
	// Get instructors
	instructors := registry.GetInstructors()
	
	assert.Len(t, instructors, 2)
	
	// Verify all returned are instructors
	for _, recipient := range instructors {
		assert.Equal(t, "instructor", recipient.GetRole())
	}
	
	// Verify specific instructors returned
	userIDs := make([]string, len(instructors))
	for i, recipient := range instructors {
		userIDs[i] = recipient.GetUserID()
	}
	assert.Contains(t, userIDs, "instructor1")
	assert.Contains(t, userIDs, "instructor2")
}

// Test 9: GetStudents
func TestConnectionRegistry_GetStudents(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	// Register mixed roles
	instructor1 := newMockConnection("instructor1", "instructor")
	student1 := newMockConnection("student1", "student")
	student2 := newMockConnection("student2", "student")
	
	_ = registry.Register("instructor1", instructor1)
	_ = registry.Register("student1", student1)
	_ = registry.Register("student2", student2)
	
	// Get students
	students := registry.GetStudents()
	
	assert.Len(t, students, 2)
	
	// Verify all returned are students
	for _, recipient := range students {
		assert.Equal(t, "student", recipient.GetRole())
	}
	
	// Verify specific students returned
	userIDs := make([]string, len(students))
	for i, recipient := range students {
		userIDs[i] = recipient.GetUserID()
	}
	assert.Contains(t, userIDs, "student1")
	assert.Contains(t, userIDs, "student2")
}

// Test 10: GetUserByID Success
func TestConnectionRegistry_GetUserByID_Success(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	conn := newMockConnection("user1", "student")
	_ = registry.Register("user1", conn)
	
	recipient, err := registry.GetUserByID("user1")
	assert.NoError(t, err)
	assert.NotNil(t, recipient)
	assert.Equal(t, "user1", recipient.GetUserID())
	assert.Equal(t, "student", recipient.GetRole())
}

// Test 11: GetUserByID Not Found
func TestConnectionRegistry_GetUserByID_NotFound(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	recipient, err := registry.GetUserByID("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, pkgErrors.ErrConnectionNotFound, err)
	assert.Nil(t, recipient)
}

// Test 12: GetAllUsers
func TestConnectionRegistry_GetAllUsers(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	// Register multiple users
	conn1 := newMockConnection("user1", "student")
	conn2 := newMockConnection("user2", "instructor")
	conn3 := newMockConnection("user3", "student")
	
	_ = registry.Register("user1", conn1)
	_ = registry.Register("user2", conn2)
	_ = registry.Register("user3", conn3)
	
	// Get all users
	allUsers := registry.GetAllUsers()
	
	assert.Len(t, allUsers, 3)
	
	// Verify all users present
	userIDs := make([]string, len(allUsers))
	for i, recipient := range allUsers {
		userIDs[i] = recipient.GetUserID()
	}
	assert.Contains(t, userIDs, "user1")
	assert.Contains(t, userIDs, "user2")
	assert.Contains(t, userIDs, "user3")
}

// Thread Safety Tests

// Test 13: Concurrent Registration
func TestConnectionRegistry_ConcurrentRegistration(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	var wg sync.WaitGroup
	numGoroutines := 10
	connectionsPerGoroutine := 5
	
	// Concurrent registration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < connectionsPerGoroutine; j++ {
				userID := "user_" + string(rune(goroutineID)) + "_" + string(rune(j))
				conn := newMockConnection(userID, "student")
				err := registry.Register(userID, conn)
				assert.NoError(t, err)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify all connections registered
	registry.mu.RLock()
	count := len(registry.connections)
	registry.mu.RUnlock()
	
	assert.Equal(t, numGoroutines*connectionsPerGoroutine, count)
}

// Test 14: Concurrent Read Operations
func TestConnectionRegistry_ConcurrentReads(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	// Setup test data
	for i := 0; i < 10; i++ {
		role := "student"
		if i%3 == 0 {
			role = "instructor"
		}
		userID := "user" + string(rune(i))
		conn := newMockConnection(userID, role)
		_ = registry.Register(userID, conn)
	}
	
	var wg sync.WaitGroup
	numReaders := 20
	
	// Concurrent read operations
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = registry.GetInstructors()
				_ = registry.GetStudents()
				_ = registry.GetAllUsers()
				_, _ = registry.GetUserByID("user1")
			}
		}()
	}
	
	wg.Wait()
	
	// Verify registry still functional
	allUsers := registry.GetAllUsers()
	assert.Len(t, allUsers, 10)
}

// Test 15: Mixed Concurrent Operations
func TestConnectionRegistry_MixedConcurrentOperations(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	var wg sync.WaitGroup
	
	// Concurrent writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				userID := "writer_" + string(rune(id)) + "_" + string(rune(j))
				conn := newMockConnection(userID, "student")
				_ = registry.Register(userID, conn)
				
				if j%2 == 0 {
					registry.Unregister(userID)
				}
			}
		}(i)
	}
	
	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = registry.GetAllUsers()
				_ = registry.GetInstructors()
			}
		}()
	}
	
	wg.Wait()
	
	// Registry should still be functional
	assert.NotNil(t, registry.GetAllUsers())
}

// Cleanup Tests

// Test 16: Cleanup Removes Stale Connections When No Active Session
func TestConnectionRegistry_CleanupStaleConnections(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	// Ensure no active session for cleanup to work
	assert.False(t, sessionManager.HasActiveSession())
	
	// Register connections with different heartbeat times
	oldConn := newMockConnection("old_user", "student")
	recentConn := newMockConnection("recent_user", "instructor")
	
	_ = registry.Register("old_user", oldConn)
	_ = registry.Register("recent_user", recentConn)
	
	// Set old heartbeat time (beyond InactiveConnectionTimeout)
	registry.heartbeatMu.Lock()
	registry.heartbeats["old_user"] = time.Now().Add(-config.InactiveConnectionTimeout - time.Minute)
	registry.heartbeats["recent_user"] = time.Now() // Recent heartbeat
	registry.heartbeatMu.Unlock()
	
	// Trigger cleanup manually
	registry.cleanupStaleConnections()
	
	// Verify stale connection removed
	registry.mu.RLock()
	_, oldExists := registry.connections["old_user"]
	_, recentExists := registry.connections["recent_user"]
	registry.mu.RUnlock()
	
	assert.False(t, oldExists)
	assert.True(t, recentExists)
	assert.True(t, oldConn.IsClosed())
	assert.False(t, recentConn.IsClosed())
}

// Test 17: Cleanup Skips When Active Session
func TestConnectionRegistry_CleanupSkipsWhenActiveSession(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	// Set active session
	activeSession := &database.Session{
		ID:   "test_session",
		Name: "Test Session",
	}
	_ = sessionManager.SetActiveSession(activeSession)
	assert.True(t, sessionManager.HasActiveSession())
	
	// Register connection with old heartbeat
	conn := newMockConnection("test_user", "student")
	_ = registry.Register("test_user", conn)
	
	// Set very old heartbeat time (way beyond timeout)
	registry.heartbeatMu.Lock()
	registry.heartbeats["test_user"] = time.Now().Add(-config.InactiveConnectionTimeout - time.Hour)
	registry.heartbeatMu.Unlock()
	
	// Trigger cleanup manually - should skip due to active session
	registry.cleanupStaleConnections()
	
	// Verify connection still exists (not cleaned up)
	registry.mu.RLock()
	_, exists := registry.connections["test_user"]
	registry.mu.RUnlock()
	
	assert.True(t, exists)
	assert.False(t, conn.IsClosed())
}

// Test 18: Cleanup Handles Empty Registry
func TestConnectionRegistry_CleanupEmpty(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	// Ensure no active session
	assert.False(t, sessionManager.HasActiveSession())
	
	// Should not panic on empty registry
	registry.cleanupStaleConnections()
	
	// Verify registry remains empty
	registry.mu.RLock()
	connCount := len(registry.connections)
	registry.mu.RUnlock()
	
	registry.heartbeatMu.RLock()
	heartbeatCount := len(registry.heartbeats)
	registry.heartbeatMu.RUnlock()
	
	assert.Equal(t, 0, connCount)
	assert.Equal(t, 0, heartbeatCount)
}

// Lifecycle Tests

// Test 19: Start Launches Cleanup Goroutine
func TestConnectionRegistry_Start(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	
	// Start should launch cleanup goroutine
	registry.Start()
	
	// Give goroutine time to start
	time.Sleep(10 * time.Millisecond)
	
	// Should be able to stop cleanly
	registry.Stop()
}

// Test 20: Stop Closes All Connections
func TestConnectionRegistry_Stop_ClosesAllConnections(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	
	// Register several connections
	conn1 := newMockConnection("user1", "student")
	conn2 := newMockConnection("user2", "instructor")
	conn3 := newMockConnection("user3", "student")
	
	_ = registry.Register("user1", conn1)
	_ = registry.Register("user2", conn2)
	_ = registry.Register("user3", conn3)
	
	// Stop registry
	registry.Stop()
	
	// Verify all connections closed
	assert.True(t, conn1.IsClosed())
	assert.True(t, conn2.IsClosed())
	assert.True(t, conn3.IsClosed())
	
	// Verify registry emptied
	registry.mu.RLock()
	count := len(registry.connections)
	registry.mu.RUnlock()
	
	assert.Equal(t, 0, count)
}

// Test 21: Stop is Idempotent
func TestConnectionRegistry_Stop_Idempotent(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	
	// Multiple stops should not panic
	registry.Stop()
	registry.Stop()
	registry.Stop()
}

// Test 22: Stop Prevents Memory Leaks
func TestConnectionRegistry_Stop_PreventsMemoryLeaks(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	registry.Start()
	
	// Register connection
	conn := newMockConnection("user1", "student")
	_ = registry.Register("user1", conn)
	
	// Stop should clean everything
	registry.Stop()
	
	// Verify ticker stopped (indirectly by ensuring no panic on multiple stops)
	registry.Stop()
	
	// Verify stop channel closed (indirectly by checking connection cleanup)
	assert.True(t, conn.IsClosed())
}

// Phase 3 Integration Tests

// Test 23: Implements ConnectionProvider Interface
func TestConnectionRegistry_ImplementsConnectionProvider(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	// Verify registry implements ConnectionProvider interface
	var provider message.ConnectionProvider = registry
	assert.NotNil(t, provider)
}

// Test 24: GetConnectedUsers Implementation
func TestConnectionRegistry_GetConnectedUsers(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	// Register test connections
	conn1 := newMockConnection("user1", "student")
	conn2 := newMockConnection("user2", "instructor")
	
	_ = registry.Register("user1", conn1)
	_ = registry.Register("user2", conn2)
	
	// Get connected users via ConnectionProvider interface
	var provider message.ConnectionProvider = registry
	users, err := provider.GetConnectedUsers()
	
	assert.NoError(t, err)
	assert.Len(t, users, 2)
	
	// Verify users returned
	userIDs := make([]string, len(users))
	for i, user := range users {
		userIDs[i] = user.GetUserID()
	}
	assert.Contains(t, userIDs, "user1")
	assert.Contains(t, userIDs, "user2")
}

// Test 25: Connection as Recipient Interface
func TestConnectionRegistry_ConnectionAsRecipient(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	conn := newMockConnection("user1", "student")
	_ = registry.Register("user1", conn)
	
	// Get connection as Recipient
	recipient, err := registry.GetUserByID("user1")
	require.NoError(t, err)
	
	// Verify Recipient interface methods
	assert.Equal(t, "user1", recipient.GetUserID())
	assert.Equal(t, "student", recipient.GetRole())
	
	// Verify SendMessage works
	testData := []byte("test message")
	err = recipient.SendMessage(testData)
	assert.NoError(t, err)
}

// Test 26: UpdateHeartbeat Functionality
func TestConnectionRegistry_UpdateHeartbeat(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	defer registry.Stop()
	
	conn := newMockConnection("user1", "student")
	_ = registry.Register("user1", conn)
	
	// Get initial heartbeat time
	registry.heartbeatMu.RLock()
	initialTime := registry.heartbeats["user1"]
	registry.heartbeatMu.RUnlock()
	
	// Wait briefly and update heartbeat
	time.Sleep(10 * time.Millisecond)
	registry.UpdateHeartbeat("user1")
	
	// Verify heartbeat was updated
	registry.heartbeatMu.RLock()
	updatedTime := registry.heartbeats["user1"]
	registry.heartbeatMu.RUnlock()
	
	assert.True(t, updatedTime.After(initialTime))
}