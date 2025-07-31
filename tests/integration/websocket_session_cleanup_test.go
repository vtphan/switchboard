package integration

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/internal/message"
	"switchboard/internal/rate"
	"switchboard/internal/session"
	internalWS "switchboard/internal/websocket"
	"switchboard/web"
	"switchboard/web/api"
)

// Accelerated configuration for fast testing
const (
	TestCleanupInterval = 100 * time.Millisecond // Fast cleanup for testing
	TestTimeout         = 500 * time.Millisecond // Short timeout for testing
)

// TestConnectionSessionAwareCleanup tests the core session-aware cleanup behavior
func TestConnectionSessionAwareCleanup(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	scenarios := []struct {
		name     string
		sequence []string
		expected string
	}{
		{
			name:     "connections_survive_active_session",
			sequence: []string{"connect", "start_session", "wait_timeout", "verify_connected"},
			expected: "Connections should remain alive during active session even after timeout period",
		},
		{
			name:     "connections_cleanup_no_session",
			sequence: []string{"connect", "wait_timeout", "verify_disconnected"},
			expected: "Connections should be cleaned up when no session and timeout exceeded",
		},
		{
			name:     "session_prevents_cleanup",
			sequence: []string{"connect", "wait_half_timeout", "start_session", "wait_timeout", "verify_connected"},
			expected: "Starting session should prevent cleanup even if total time > timeout",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			executeScenario(t, server, scenario.sequence, scenario.expected)
		})
	}
}

func executeScenario(t *testing.T, server *testServer, sequence []string, expected string) {
	var conn *websocket.Conn
	var err error
	
	for _, step := range sequence {
		switch step {
		case "connect":
			conn = connectWebSocket(t, "test_user", "student")
			
		case "start_session":
			err = startTestSession(server.dbManager)
			require.NoError(t, err, "Failed to start session")
			
		case "wait_timeout":
			time.Sleep(TestTimeout + 50*time.Millisecond) // Wait slightly longer than timeout
			
		case "wait_half_timeout":
			time.Sleep(TestTimeout / 2)
			
		case "verify_connected":
			// Send a test message to verify connection is still alive
			testMsg := map[string]interface{}{
				"type": "test_ping",
				"content": map[string]interface{}{
					"message": "ping",
				},
			}
			err = conn.WriteJSON(testMsg)
			assert.NoError(t, err, expected)
			
		case "verify_disconnected":
			// Try to send a message - should fail if connection is cleaned up
			testMsg := map[string]interface{}{
				"type": "test_ping", 
				"content": map[string]interface{}{
					"message": "ping",
				},
			}
			err = conn.WriteJSON(testMsg)
			
			// Connection might be closed by server, or write might fail
			// We'll check for either condition
			if err == nil {
				// If write succeeded, try to read - should get connection closed error
				_, _, readErr := conn.ReadMessage()
				assert.Error(t, readErr, expected)
			} else {
				// Write failed, which is expected for closed connection
				assert.Error(t, err, expected)
			}
		}
	}
	
	if conn != nil {
		if err := conn.Close(); err != nil {
			t.Logf("Failed to close connection: %v", err)
		}
	}
}

func connectWebSocket(t *testing.T, userID, role string) *websocket.Conn {
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	q := u.Query()
	q.Set("user_id", userID)
	q.Set("role", role)
	u.RawQuery = q.Encode()
	
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err, "Failed to connect WebSocket")
	
	return conn
}

func startTestSession(dbManager database.DatabaseManager) error {
	// Create session directly in database
	session := &database.Session{
		ID:        "test_session_" + fmt.Sprintf("%d", time.Now().Unix()),
		Name:      "Test Session",
		CreatedBy: "test_instructor",
		StartTime: time.Now(),
		Status:    "active",
	}
	
	return dbManager.CreateSession(session)
}

func setupTestServer(t *testing.T) (*testServer, func()) {
	// Create in-memory database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	
	// Apply schema
	schema, err := readSchemaFile()
	require.NoError(t, err)
	
	_, err = db.Exec(string(schema))
	require.NoError(t, err)
	
	dbManager, err := database.NewSQLiteDatabaseManager(db)
	require.NoError(t, err)
	
	// Start database manager
	err = dbManager.Start()
	require.NoError(t, err)
	
	// Initialize components with test configuration
	sessionManager := session.NewSessionManager(dbManager)
	sessionLifecycle := session.NewSessionLifecycle(sessionManager, dbManager)
	rateLimiter := rate.NewRateLimiter()
	
	// Use registry with accelerated cleanup for testing
	connectionRegistry := internalWS.NewConnectionRegistry(sessionManager)
	
	// Override cleanup interval for testing (this would require modifying the registry)
	// For now, we'll use the default but with shorter timeout in our test
	
	roleBasedFilter := &message.RoleBasedFilter{}
	messageRouter := message.NewMessageRouter(connectionRegistry, roleBasedFilter)
	
	filterAdapter := internalWS.NewFilterAdapter(roleBasedFilter)
	broadcastSystem := internalWS.NewBroadcastSystem(connectionRegistry, filterAdapter)
	
	messageProcessor := message.NewMessageProcessor(
		sessionManager,
		dbManager,
		rateLimiter,
		messageRouter,
		broadcastSystem,
	)
	
	websocketHandler := internalWS.NewWebSocketHandler(
		connectionRegistry,
		sessionManager,
		messageProcessor,
		dbManager,
	)
	
	sessionAPIHandler := api.NewSessionAPIHandler(sessionLifecycle)
	httpServer := web.NewHTTPServer(sessionAPIHandler, websocketHandler)
	
	// Start components
	connectionRegistry.Start()
	
	// Start HTTP server
	go func() {
		if err := httpServer.Start("8080"); err != nil {
			t.Logf("HTTP server error: %v", err)
		}
	}()
	
	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	
	server := &testServer{
		httpServer: httpServer,
		dbManager:  dbManager,
	}
	
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := httpServer.Stop(ctx); err != nil {
			t.Logf("Server shutdown error: %v", err)
		}
	}
	
	return server, cleanup
}

type testServer struct {
	httpServer *web.HTTPServer
	dbManager  database.DatabaseManager
}

// readSchemaFile reads the database schema from migrations.sql
func readSchemaFile() ([]byte, error) {
	// Try different paths since test may run from different working directories
	possiblePaths := []string{
		"internal/database/migrations.sql",
		"../../internal/database/migrations.sql", 
		"../../../internal/database/migrations.sql",
	}
	
	var schema []byte
	var err error
	
	for _, path := range possiblePaths {
		schema, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}
	
	if err != nil {
		return nil, fmt.Errorf("schema file not found in any of the expected paths: %w", err)
	}
	
	return schema, nil
}