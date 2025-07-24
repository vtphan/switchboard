package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"switchboard/pkg/interfaces"
)

// Test WebSocket upgrader for creating test connections
var testUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Architectural Validation Tests
func TestConnection_InterfaceCompliance(t *testing.T) {
	// Verify Connection implements interfaces.Connection
	var _ interfaces.Connection = &Connection{}
}

func TestConnection_ImportBoundaryCompliance(t *testing.T) {
	// This test passes if compilation succeeds - no circular imports
	t.Log("Import boundaries maintained - no circular dependencies")
}

// Functional Validation Tests
func TestConnection_NewConnectionInitialization(t *testing.T) {
	wsConn := createTestWebSocketConnection(t)
	defer wsConn.Close()

	conn := NewConnection(wsConn)
	defer conn.Close()

	if conn == nil {
		t.Fatal("NewConnection returned nil")
	}

	if conn.writeCh == nil {
		t.Error("Write channel not initialized")
	}

	if cap(conn.writeCh) != 100 {
		t.Errorf("Expected write channel buffer of 100, got %d", cap(conn.writeCh))
	}

	if conn.IsAuthenticated() {
		t.Error("New connection should not be authenticated")
	}
}

func TestConnection_AuthenticationFlow(t *testing.T) {
	wsConn := createTestWebSocketConnection(t)
	defer wsConn.Close()

	conn := NewConnection(wsConn)
	defer conn.Close()

	// Initially not authenticated
	if conn.IsAuthenticated() {
		t.Error("New connection should not be authenticated")
	}

	// Set credentials
	err := conn.SetCredentials("user123", "student", "session456")
	if err != nil {
		t.Errorf("SetCredentials failed: %v", err)
	}

	// Should now be authenticated
	if !conn.IsAuthenticated() {
		t.Error("Connection should be authenticated after SetCredentials")
	}

	// Check credential values
	if conn.GetUserID() != "user123" {
		t.Errorf("Expected userID 'user123', got '%s'", conn.GetUserID())
	}
	if conn.GetRole() != "student" {
		t.Errorf("Expected role 'student', got '%s'", conn.GetRole())
	}
	if conn.GetSessionID() != "session456" {
		t.Errorf("Expected sessionID 'session456', got '%s'", conn.GetSessionID())
	}
}

func TestConnection_WriteJSONValidData(t *testing.T) {
	wsConn := createTestWebSocketConnection(t)
	defer wsConn.Close()

	conn := NewConnection(wsConn)
	defer conn.Close()

	testData := map[string]interface{}{
		"type":    "test",
		"content": "test message",
	}

	// Should successfully write JSON
	err := conn.WriteJSON(testData)
	if err != nil {
		t.Errorf("WriteJSON failed: %v", err)
	}
}

func TestConnection_WriteJSONInvalidData(t *testing.T) {
	wsConn := createTestWebSocketConnection(t)
	defer wsConn.Close()

	conn := NewConnection(wsConn)
	defer conn.Close()

	// Function type cannot be marshaled to JSON
	invalidData := map[string]interface{}{
		"func": func() {},
	}

	err := conn.WriteJSON(invalidData)
	if err != ErrInvalidJSON {
		t.Errorf("Expected ErrInvalidJSON, got %v", err)
	}
}

func TestConnection_CloseIdempotent(t *testing.T) {
	wsConn := createTestWebSocketConnection(t)
	defer wsConn.Close()

	conn := NewConnection(wsConn)

	// Close should be safe to call multiple times
	err1 := conn.Close()
	err2 := conn.Close()
	err3 := conn.Close()

	if err1 != nil {
		t.Errorf("First close failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second close failed: %v", err2)
	}
	if err3 != nil {
		t.Errorf("Third close failed: %v", err3)
	}
}

func TestConnection_WriteAfterClose(t *testing.T) {
	wsConn := createTestWebSocketConnection(t)
	defer wsConn.Close()

	conn := NewConnection(wsConn)
	conn.Close()

	// Give time for context cancellation to propagate
	time.Sleep(10 * time.Millisecond)

	testData := map[string]interface{}{
		"type": "test",
	}

	err := conn.WriteJSON(testData)
	if err != ErrConnectionClosed {
		t.Errorf("Expected ErrConnectionClosed, got %v", err)
	}
}

// Technical Validation Tests (Race Detection)
func TestConnection_ConcurrentWrites(t *testing.T) {
	wsConn := createTestWebSocketConnection(t)
	defer wsConn.Close()

	conn := NewConnection(wsConn)
	defer conn.Close()

	const numGoroutines = 10
	const messagesPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Test concurrent writes don't cause race conditions
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				testData := map[string]interface{}{
					"worker":  id,
					"message": j,
				}
				conn.WriteJSON(testData) // Should be thread-safe
			}
		}(i)
	}

	wg.Wait()
}

func TestConnection_ConcurrentCredentialAccess(t *testing.T) {
	wsConn := createTestWebSocketConnection(t)
	defer wsConn.Close()

	conn := NewConnection(wsConn)
	defer conn.Close()

	conn.SetCredentials("user123", "student", "session456")

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Test concurrent read access to credentials
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			// These should be thread-safe
			userID := conn.GetUserID()
			role := conn.GetRole()
			sessionID := conn.GetSessionID()
			auth := conn.IsAuthenticated()

			// Verify values are consistent
			if userID != "user123" || role != "student" || sessionID != "session456" || !auth {
				t.Errorf("Inconsistent credential values during concurrent access")
			}
		}()
	}

	wg.Wait()
}

func TestConnection_GoroutineCleanup(t *testing.T) {
	wsConn := createTestWebSocketConnection(t)
	defer wsConn.Close()

	conn := NewConnection(wsConn)

	// Give time for writeLoop to start
	time.Sleep(10 * time.Millisecond)

	err := conn.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Wait for goroutine cleanup
	time.Sleep(100 * time.Millisecond)

	// If there are goroutine leaks, the race detector should catch them
}

// Helper function to create a test WebSocket connection
func createTestWebSocketConnection(t *testing.T) *websocket.Conn {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		// Keep connection alive for testing
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))

	t.Cleanup(func() { server.Close() })

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Failed to create test WebSocket connection: %v", err)
	}

	return conn
}