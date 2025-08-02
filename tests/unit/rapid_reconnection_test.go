package unit

import (
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
	"switchboard/internal/database"
	"switchboard/internal/session"
	"switchboard/internal/websocket"
	pkgErrors "switchboard/pkg/errors"
)

// RapidReconnectionMockConnection implements ConnectionInterface for testing rapid reconnections
type RapidReconnectionMockConnection struct {
	userID       string
	role         string
	closed       bool
	closedMu     sync.RWMutex
	closeCh      chan struct{}
	sendCh       chan []byte
	lastActivity time.Time
	activityMu   sync.RWMutex
}

func NewRapidReconnectionMockConnection(userID, role string) *RapidReconnectionMockConnection {
	return &RapidReconnectionMockConnection{
		userID:       userID,
		role:         role,
		closed:       false,
		closeCh:      make(chan struct{}),
		sendCh:       make(chan []byte, 100),
		lastActivity: time.Now(),
	}
}

func (mc *RapidReconnectionMockConnection) WriteJSON(v interface{}) error {
	if mc.IsClosed() {
		return errors.New("connection closed")
	}
	return nil
}

func (mc *RapidReconnectionMockConnection) Close() error {
	mc.closedMu.Lock()
	defer mc.closedMu.Unlock()
	
	if !mc.closed {
		mc.closed = true
		close(mc.closeCh)
		close(mc.sendCh)
	}
	return nil
}

func (mc *RapidReconnectionMockConnection) IsClosed() bool {
	mc.closedMu.RLock()
	defer mc.closedMu.RUnlock()
	return mc.closed
}

func (mc *RapidReconnectionMockConnection) GetUserID() string {
	return mc.userID
}

func (mc *RapidReconnectionMockConnection) GetRole() string {
	return mc.role
}

func (mc *RapidReconnectionMockConnection) SetCredentials(username, role string) error {
	mc.userID = username
	mc.role = role
	return nil
}

func (mc *RapidReconnectionMockConnection) UpdateActivity() {
	mc.activityMu.Lock()
	defer mc.activityMu.Unlock()
	mc.lastActivity = time.Now()
}

func (mc *RapidReconnectionMockConnection) GetLastSeen() time.Time {
	mc.activityMu.RLock()
	defer mc.activityMu.RUnlock()
	return mc.lastActivity
}

func (mc *RapidReconnectionMockConnection) SendMessage(data []byte) error {
	if mc.IsClosed() {
		return errors.New("connection closed")
	}
	
	select {
	case mc.sendCh <- data:
		return nil
	default:
		return pkgErrors.ErrChannelFull
	}
}

func (mc *RapidReconnectionMockConnection) SendCloseMessage(reason string) error {
	return mc.Close()
}

// SimpleMockDatabaseManager implements minimal DatabaseManager interface for testing
type SimpleMockDatabaseManager struct{}

func (m *SimpleMockDatabaseManager) CreateSession(s *database.Session) error { return nil }
func (m *SimpleMockDatabaseManager) UpdateSession(s *database.Session) error { return nil }
func (m *SimpleMockDatabaseManager) GetActiveSession() (*database.Session, error) { return nil, nil }
func (m *SimpleMockDatabaseManager) WriteMessage(msg *database.Message) error { return nil }
func (m *SimpleMockDatabaseManager) WriteBatch(messages []*database.Message) error { return nil }
func (m *SimpleMockDatabaseManager) GetSessionMessages(sessionID string) ([]*database.Message, error) { return nil, nil }
func (m *SimpleMockDatabaseManager) Start() error { return nil }
func (m *SimpleMockDatabaseManager) Stop() error { return nil }
func (m *SimpleMockDatabaseManager) WaitForPendingWrites() error { return nil }

func TestRapidReconnectionTiming(t *testing.T) {
	// Setup test environment
	db := setupRapidReconnectionTestDatabase(t)
	defer db.Close()
	
	sessionManager := session.NewSessionManager(&SimpleMockDatabaseManager{})
	registry := websocket.NewConnectionRegistry(sessionManager)
	
	t.Run("synchronous connection replacement prevents race conditions", func(t *testing.T) {
		userID := "test_user"
		
		// Create first connection
		conn1 := NewRapidReconnectionMockConnection(userID, "student")
		err := registry.Register(userID, conn1)
		require.NoError(t, err)
		
		// Verify first connection is registered
		assert.True(t, registry.IsRegistered(userID))
		user, err := registry.GetUserByID(userID)
		require.NoError(t, err)
		assert.Equal(t, userID, user.GetUserID())
		
		// Create second connection (rapid reconnection)
		conn2 := NewRapidReconnectionMockConnection(userID, "student")
		err = registry.Register(userID, conn2)
		require.NoError(t, err)
		
		// Verify old connection was closed
		assert.True(t, conn1.IsClosed(), "First connection should be closed")
		
		// Verify new connection is active
		assert.False(t, conn2.IsClosed(), "Second connection should be active")
		
		// Verify registry has the new connection
		user, err = registry.GetUserByID(userID)
		require.NoError(t, err)
		assert.Equal(t, userID, user.GetUserID())
		
		// Clean up
		registry.Unregister(userID)
	})
	
	t.Run("rapid successive reconnections handle properly", func(t *testing.T) {
		userID := "rapid_user"
		connectionCount := 10
		connections := make([]*RapidReconnectionMockConnection, connectionCount)
		
		// Create and register connections rapidly in sequence
		for i := 0; i < connectionCount; i++ {
			connections[i] = NewRapidReconnectionMockConnection(userID, "instructor")
			err := registry.Register(userID, connections[i])
			require.NoError(t, err)
		}
		
		// Verify only the last connection is active
		assert.False(t, connections[connectionCount-1].IsClosed(), "Last connection should be active")
		
		// Verify all previous connections were closed
		for i := 0; i < connectionCount-1; i++ {
			assert.True(t, connections[i].IsClosed(), "Connection %d should be closed", i)
		}
		
		// Verify registry has only one connection for the user
		assert.True(t, registry.IsRegistered(userID))
		user, err := registry.GetUserByID(userID)
		require.NoError(t, err)
		assert.Equal(t, userID, user.GetUserID())
		
		// Clean up
		registry.Unregister(userID)
	})
	
	t.Run("concurrent rapid reconnections are thread-safe", func(t *testing.T) {
		userID := "concurrent_user"
		goroutineCount := 50
		
		var wg sync.WaitGroup
		connections := make([]*RapidReconnectionMockConnection, goroutineCount)
		errors := make([]error, goroutineCount)
		
		// Launch concurrent registration attempts
		for i := 0; i < goroutineCount; i++ {
			connections[i] = NewRapidReconnectionMockConnection(userID, "student")
			
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				errors[idx] = registry.Register(userID, connections[idx])
			}(i)
		}
		
		// Wait for all to complete
		wg.Wait()
		
		// Check all registrations succeeded
		for i, err := range errors {
			assert.NoError(t, err, "Registration %d should succeed", i)
		}
		
		// Verify only one connection is active
		activeCount := 0
		var activeConnection *RapidReconnectionMockConnection
		
		for i, conn := range connections {
			if !conn.IsClosed() {
				activeCount++
				activeConnection = connections[i]
			}
		}
		
		assert.Equal(t, 1, activeCount, "Exactly one connection should be active")
		assert.NotNil(t, activeConnection, "One connection should remain active")
		
		// Verify registry state is consistent
		assert.True(t, registry.IsRegistered(userID))
		user, err := registry.GetUserByID(userID)
		require.NoError(t, err)
		assert.Equal(t, userID, user.GetUserID())
		
		// Clean up
		registry.Unregister(userID)
	})
	
	t.Run("heartbeat tracking survives rapid reconnections", func(t *testing.T) {
		userID := "heartbeat_user"
		
		// Register initial connection
		conn1 := NewRapidReconnectionMockConnection(userID, "student")
		err := registry.Register(userID, conn1)
		require.NoError(t, err)
		
		// Update heartbeat
		initialTime := time.Now().Add(-time.Minute)
		registry.SetHeartbeatTimeForTest(userID, initialTime)
		
		// Rapid reconnection
		conn2 := NewRapidReconnectionMockConnection(userID, "student")
		err = registry.Register(userID, conn2)
		require.NoError(t, err)
		
		// Verify heartbeat was reset for new connection
		newHeartbeat := registry.GetHeartbeatTimeForTest(userID)
		assert.True(t, newHeartbeat.After(initialTime), 
			"Heartbeat should be updated for new connection")
		
		// Verify old connection was closed
		assert.True(t, conn1.IsClosed())
		assert.False(t, conn2.IsClosed())
		
		// Clean up
		registry.Unregister(userID)
	})
	
	t.Run("connection replacement preserves role information", func(t *testing.T) {
		userID := "role_user"
		
		// Register as student
		conn1 := NewRapidReconnectionMockConnection(userID, "student")
		err := registry.Register(userID, conn1)
		require.NoError(t, err)
		
		user, err := registry.GetUserByID(userID)
		require.NoError(t, err)
		assert.Equal(t, "student", user.GetRole())
		
		// Reconnect as instructor
		conn2 := NewRapidReconnectionMockConnection(userID, "instructor")
		err = registry.Register(userID, conn2)
		require.NoError(t, err)
		
		// Verify role was updated
		user, err = registry.GetUserByID(userID)
		require.NoError(t, err)
		assert.Equal(t, "instructor", user.GetRole())
		
		// Verify old connection was closed
		assert.True(t, conn1.IsClosed())
		assert.False(t, conn2.IsClosed())
		
		// Clean up
		registry.Unregister(userID)
	})
}

func TestConnectionReplacementResourceCleanup(t *testing.T) {
	// Setup test environment
	db := setupRapidReconnectionTestDatabase(t)
	defer db.Close()
	
	sessionManager := session.NewSessionManager(&SimpleMockDatabaseManager{})
	registry := websocket.NewConnectionRegistry(sessionManager)
	
	t.Run("replaced connections are properly cleaned up", func(t *testing.T) {
		userID := "cleanup_user"
		
		// Track resource cleanup
		var cleanupCount int
		var cleanupMu sync.Mutex
		
		// Create connection that tracks cleanup
		conn1 := NewRapidReconnectionMockConnection(userID, "student")
		
		// Track when close is called
		originalClosed := conn1.closed
		closeDetected := false
		
		// Monitor connection close status periodically
		go func() {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			
			for range ticker.C {
				if conn1.IsClosed() && !originalClosed {
					cleanupMu.Lock()
					if !closeDetected {
						cleanupCount++
						closeDetected = true
					}
					cleanupMu.Unlock()
					return
				}
			}
		}()
		
		// Register first connection
		err := registry.Register(userID, conn1)
		require.NoError(t, err)
		
		// Replace with second connection
		conn2 := NewRapidReconnectionMockConnection(userID, "student")
		err = registry.Register(userID, conn2)
		require.NoError(t, err)
		
		// Wait a bit for cleanup detection
		time.Sleep(50 * time.Millisecond)
		
		// Verify cleanup was detected
		cleanupMu.Lock()
		cleanupCountValue := cleanupCount
		cleanupMu.Unlock()
		
		assert.Equal(t, 1, cleanupCountValue, "Cleanup should be detected once for replaced connection")
		assert.True(t, conn1.IsClosed(), "First connection should be closed")
		assert.False(t, conn2.IsClosed(), "Second connection should be active")
		
		// Clean up
		registry.Unregister(userID)
	})
}

// setupRapidReconnectionTestDatabase creates an in-memory SQLite database for testing
func setupRapidReconnectionTestDatabase(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	return db
}