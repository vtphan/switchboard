package unit

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/internal/websocket"
	"switchboard/pkg/config"
)

// MockSessionManager implements session.SessionManager for testing
type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) StartSession(name, instructorID string) (*database.Session, error) {
	args := m.Called(name, instructorID)
	return args.Get(0).(*database.Session), args.Error(1)
}

func (m *MockSessionManager) EndSession(instructorID string) (*database.Session, error) {
	args := m.Called(instructorID)
	return args.Get(0).(*database.Session), args.Error(1)
}

func (m *MockSessionManager) GetActiveSession() *database.Session {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*database.Session)
}

func (m *MockSessionManager) HasActiveSession() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockSessionManager) SetActiveSession(session *database.Session) error {
	args := m.Called(session)
	return args.Error(0)
}

func (m *MockSessionManager) ClearActiveSession() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSessionManager) CheckAndClearActiveSession() (*database.Session, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.Session), args.Error(1)
}

// MockConnection implements websocket.ConnectionInterface for testing
type MockConnection struct {
	mock.Mock
	userID   string
	role     string
	lastSeen time.Time
}

func (m *MockConnection) WriteJSON(v interface{}) error {
	args := m.Called(v)
	return args.Error(0)
}

func (m *MockConnection) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConnection) GetUserID() string {
	return m.userID
}

func (m *MockConnection) GetRole() string {
	return m.role
}

func (m *MockConnection) SetCredentials(userID, role string) error {
	args := m.Called(userID, role)
	m.userID = userID
	m.role = role
	return args.Error(0)
}

func (m *MockConnection) UpdateActivity() {
	m.lastSeen = time.Now()
	m.Called()
}

func (m *MockConnection) GetLastSeen() time.Time {
	return m.lastSeen
}

func (m *MockConnection) SendMessage(data []byte) error {
	args := m.Called(data)
	return args.Error(0)
}

func (m *MockConnection) SendCloseMessage(reason string) error {
	args := m.Called(reason)
	return args.Error(0)
}

func (m *MockConnection) SetRegistry(registry websocket.RegistryNotifier) {
	m.Called(registry)
}

// Test session-aware cleanup behavior
func TestConnectionRegistry_CleanupStaleConnections_SessionAware(t *testing.T) {
	tests := []struct {
		name          string
		sessionActive bool
		connectionAge time.Duration
		expectCleanup bool
		description   string
	}{
		{
			name:          "active_session_no_cleanup_old_connection",
			sessionActive: true,
			connectionAge: 30 * time.Minute, // Older than 25min timeout
			expectCleanup: false,
			description:   "During active session, old connections should not be cleaned up",
		},
		{
			name:          "inactive_session_cleanup_old_connection",
			sessionActive: false,
			connectionAge: 30 * time.Minute, // Older than 25min timeout
			expectCleanup: true,
			description:   "During inactive session, old connections should be cleaned up",
		},
		{
			name:          "inactive_session_keep_recent_connection",
			sessionActive: false,
			connectionAge: 20 * time.Minute, // Newer than 25min timeout
			expectCleanup: false,
			description:   "During inactive session, recent connections should be kept",
		},
		{
			name:          "active_session_keep_recent_connection",
			sessionActive: true,
			connectionAge: 5 * time.Minute, // Recent connection
			expectCleanup: false,
			description:   "During active session, recent connections should be kept",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock session manager
			mockSessionMgr := &MockSessionManager{}
			mockSessionMgr.On("HasActiveSession").Return(tt.sessionActive)

			// Create registry
			registry := websocket.NewConnectionRegistry(mockSessionMgr)

			// Create mock connection
			mockConn := &MockConnection{
				userID:   "test_user",
				role:     "student",
				lastSeen: time.Now().Add(-tt.connectionAge),
			}
			mockConn.On("Close").Return(nil)

			// Register connection
			err := registry.Register("test_user", mockConn)
			require.NoError(t, err)

			// Set heartbeat to simulate connection age
			registry.UpdateHeartbeat("test_user")
			// Manually adjust heartbeat time to simulate age
			registry.SetHeartbeatTimeForTest("test_user", time.Now().Add(-tt.connectionAge))

			// Count connections before cleanup
			initialCount := len(registry.GetAllUsers())
			assert.Equal(t, 1, initialCount, "Should have 1 connection initially")

			// Perform cleanup
			registry.CleanupStaleConnectionsForTest()

			// Count connections after cleanup
			finalCount := len(registry.GetAllUsers())

			if tt.expectCleanup {
				assert.Equal(t, 0, finalCount, tt.description)
				mockConn.AssertCalled(t, "Close")
			} else {
				assert.Equal(t, 1, finalCount, tt.description)
				mockConn.AssertNotCalled(t, "Close")
			}

			mockSessionMgr.AssertExpectations(t)
		})
	}
}

// Test heartbeat integration with cleanup logic
func TestConnectionRegistry_UpdateHeartbeat_Integration(t *testing.T) {
	mockSessionMgr := &MockSessionManager{}
	mockSessionMgr.On("HasActiveSession").Return(false) // No active session

	registry := websocket.NewConnectionRegistry(mockSessionMgr)

	// Create mock connection
	mockConn := &MockConnection{
		userID: "test_user",
		role:   "student",
	}
	mockConn.On("Close").Return(nil)

	// Register connection
	err := registry.Register("test_user", mockConn)
	require.NoError(t, err)

	// Set initial old heartbeat
	oldTime := time.Now().Add(-30 * time.Minute) // Older than timeout
	registry.SetHeartbeatTimeForTest("test_user", oldTime)

	// Verify connection would be cleaned up with old heartbeat
	registry.CleanupStaleConnectionsForTest()
	assert.Equal(t, 0, len(registry.GetAllUsers()), "Connection should be cleaned up with old heartbeat")

	// Re-register connection
	err = registry.Register("test_user", mockConn)
	require.NoError(t, err)

	// Update heartbeat to recent time
	registry.UpdateHeartbeat("test_user")

	// Verify connection is not cleaned up with recent heartbeat
	registry.CleanupStaleConnectionsForTest()
	assert.Equal(t, 1, len(registry.GetAllUsers()), "Connection should not be cleaned up with recent heartbeat")
}

// Test concurrent heartbeat updates and cleanup operations
func TestConnectionRegistry_HeartbeatThreadSafety(t *testing.T) {
	mockSessionMgr := &MockSessionManager{}
	mockSessionMgr.On("HasActiveSession").Return(false)

	registry := websocket.NewConnectionRegistry(mockSessionMgr)

	// Register multiple connections
	connections := make([]*MockConnection, 10)
	for i := 0; i < 10; i++ {
		conn := &MockConnection{
			userID: fmt.Sprintf("user_%d", i),
			role:   "student",
		}
		conn.On("Close").Return(nil)
		connections[i] = conn

		err := registry.Register(conn.userID, conn)
		require.NoError(t, err)
	}

	// Concurrent heartbeat updates and cleanup operations
	var wg sync.WaitGroup

	// Start heartbeat updaters
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				userID := fmt.Sprintf("user_%d", j%10)
				registry.UpdateHeartbeat(userID)
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	// Start cleanup operations
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				registry.CleanupStaleConnectionsForTest()
				time.Sleep(2 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// Verify no race conditions occurred and connections still exist
	// (since heartbeats are being updated, connections should remain)
	finalCount := len(registry.GetAllUsers())
	assert.Greater(t, finalCount, 0, "Some connections should remain after concurrent operations")
}

// Test heartbeat behavior during connection registration and unregistration
func TestConnectionRegistry_HeartbeatLifecycle(t *testing.T) {
	mockSessionMgr := &MockSessionManager{}
	mockSessionMgr.On("HasActiveSession").Return(false)

	registry := websocket.NewConnectionRegistry(mockSessionMgr)

	// Test heartbeat creation during registration
	mockConn := &MockConnection{
		userID: "test_user",
		role:   "student",
	}
	mockConn.On("Close").Return(nil) // Add missing mock expectation

	err := registry.Register("test_user", mockConn)
	require.NoError(t, err)

	// Verify heartbeat was created
	heartbeatTime := registry.GetHeartbeatTimeForTest("test_user")
	assert.False(t, heartbeatTime.IsZero(), "Heartbeat should be created during registration")
	assert.WithinDuration(t, time.Now(), heartbeatTime, time.Second, "Heartbeat should be recent")

	// Test heartbeat cleanup during unregistration
	registry.Unregister("test_user")

	// Verify heartbeat was cleaned up
	heartbeatTime = registry.GetHeartbeatTimeForTest("test_user")
	assert.True(t, heartbeatTime.IsZero(), "Heartbeat should be cleaned up during unregistration")
}

// Test timeout configuration alignment
func TestConnectionRegistry_TimeoutConfiguration(t *testing.T) {
	// Verify that cleanup uses the correct timeout constant
	mockSessionMgr := &MockSessionManager{}
	mockSessionMgr.On("HasActiveSession").Return(false)

	registry := websocket.NewConnectionRegistry(mockSessionMgr)

	mockConn := &MockConnection{
		userID: "test_user",
		role:   "student",
	}
	mockConn.On("Close").Return(nil)

	err := registry.Register("test_user", mockConn)
	require.NoError(t, err)

	// Test connection at exactly timeout boundary
	timeoutBoundary := time.Now().Add(-config.InactiveConnectionTimeout)
	registry.SetHeartbeatTimeForTest("test_user", timeoutBoundary.Add(-time.Second)) // Just over timeout

	registry.CleanupStaleConnectionsForTest()
	assert.Equal(t, 0, len(registry.GetAllUsers()), "Connection should be cleaned up just over timeout")

	// Re-register and test just under timeout
	err = registry.Register("test_user", mockConn)
	require.NoError(t, err)

	registry.SetHeartbeatTimeForTest("test_user", timeoutBoundary.Add(time.Second)) // Just under timeout

	registry.CleanupStaleConnectionsForTest()
	assert.Equal(t, 1, len(registry.GetAllUsers()), "Connection should not be cleaned up just under timeout")
}