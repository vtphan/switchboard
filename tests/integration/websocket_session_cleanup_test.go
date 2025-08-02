package integration

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/internal/session"
	"switchboard/internal/websocket"
)

// TestConnectionSessionIntegration tests core connection registry integration with sessions
func TestConnectionSessionIntegration(t *testing.T) {
	t.Run("connection_registry_basic_operations", func(t *testing.T) {
		// Setup fresh environment for each test
		dbManager, _, connectionRegistry := setupCleanupTestEnvironment(t)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Test that connection registry can register and retrieve connections
		mockConn := &MockConnection{userID: "test_user", role: "student", lastSeen: time.Now()}
		
		err := connectionRegistry.Register("test_user", mockConn)
		require.NoError(t, err)
		
		// Verify connection is registered
		users, err := connectionRegistry.GetConnectedUsers()
		require.NoError(t, err)
		assert.Len(t, users, 1)
		assert.Equal(t, "test_user", users[0].GetUserID())
		assert.Equal(t, "student", users[0].GetRole())
	})

	t.Run("session_state_integration", func(t *testing.T) {
		// Setup fresh environment for each test
		dbManager, sessionManager, connectionRegistry := setupCleanupTestEnvironment(t)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Test that session manager integrates with connection registry
		
		// Initially no active session
		activeSession := sessionManager.GetActiveSession()
		assert.Nil(t, activeSession)

		// Create and set active session
		session := &database.Session{
			ID:        "test-session-integration",
			Name:      "Integration Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))
		require.NoError(t, dbManager.CreateSession(session))

		// Verify session is active
		activeSession = sessionManager.GetActiveSession()
		require.NotNil(t, activeSession)
		assert.Equal(t, session.ID, activeSession.ID)

		// Register connections during active session
		instructorConn := &MockConnection{userID: "instructor1", role: "instructor", lastSeen: time.Now()}
		studentConn := &MockConnection{userID: "student1", role: "student", lastSeen: time.Now()}
		
		err := connectionRegistry.Register("instructor1", instructorConn)
		require.NoError(t, err)
		err = connectionRegistry.Register("student1", studentConn)
		require.NoError(t, err)
		
		// Verify both connections are registered
		users, err := connectionRegistry.GetConnectedUsers()
		require.NoError(t, err)
		assert.Len(t, users, 2)
	})
}

// setupCleanupTestEnvironment creates simplified test environment for cleanup testing
func setupCleanupTestEnvironment(t *testing.T) (*database.SQLiteDatabaseManager, session.SessionManager, *websocket.ConnectionRegistry) {
	// Setup in-memory database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Apply database schema
	schema, err := os.ReadFile("../../internal/database/migrations.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(schema))
	require.NoError(t, err)

	// Create database manager
	dbManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
		SkipTableInit: true,
		SkipPragmas:   true,
	})
	require.NoError(t, err)
	require.NoError(t, dbManager.Start())

	// Create session manager
	sessionManager := session.NewSessionManager(dbManager)

	// Create connection registry and start cleanup process
	connectionRegistry := websocket.NewConnectionRegistry(sessionManager)
	connectionRegistry.Start()

	return dbManager, sessionManager, connectionRegistry
}

// MockConnection implements the Connection interface for testing
type MockConnection struct {
	userID   string
	role     string
	lastSeen time.Time
}

func (mc *MockConnection) GetUserID() string    { return mc.userID }
func (mc *MockConnection) GetRole() string      { return mc.role }
func (mc *MockConnection) GetLastSeen() time.Time { return mc.lastSeen }
func (mc *MockConnection) SendMessage(data []byte) error { return nil }
func (mc *MockConnection) WriteJSON(v interface{}) error { return nil }
func (mc *MockConnection) SetCredentials(username, role string) error { return nil }
func (mc *MockConnection) UpdateActivity() { mc.lastSeen = time.Now() }
func (mc *MockConnection) SendCloseMessage(reason string) error { return nil }
func (mc *MockConnection) Close() error { return nil }
func (mc *MockConnection) Start(ctx context.Context, messageHandler func([]byte, string) error) {}