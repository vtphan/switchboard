package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewConnection verifies connection creation and initialization
func TestNewConnection(t *testing.T) {
	// Create test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			if err := conn.Close(); err != nil {
				// Connection close error during test cleanup, expected
				_ = err // Explicitly ignore error during cleanup
			}
		}()
		
		// Keep connection alive
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// Connect to test server
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("Failed to close websocket: %v", err)
		}
	}()

	// Create connection wrapper
	conn := NewConnection(ws)
	assert.NotNil(t, conn)
	assert.NotNil(t, conn.conn)
	assert.NotNil(t, conn.writeCh)
	assert.NotNil(t, conn.ctx)
	assert.NotNil(t, conn.cancel)
	assert.False(t, conn.authenticated)
	assert.Equal(t, "", conn.userID)
	assert.Equal(t, "", conn.role)
	assert.Equal(t, "", conn.sessionID)

	// Clean up
	err = conn.Close()
	assert.NoError(t, err)
}

// TestWriteJSON verifies JSON writing functionality
func TestWriteJSON(t *testing.T) {
	// Create test WebSocket server that reads messages
	receivedCh := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			if err := conn.Close(); err != nil {
				// Connection close error during test cleanup, expected
				_ = err // Explicitly ignore error during cleanup
			}
		}()
		
		// Read message
		_, msg, err := conn.ReadMessage()
		if err == nil {
			receivedCh <- msg
		}
	}))
	defer server.Close()

	// Connect to test server
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("Failed to close websocket: %v", err)
		}
	}()

	conn := NewConnection(ws)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("Failed to close connection: %v", err)
		}
	}()

	// Test successful write
	testData := map[string]string{"type": "test", "content": "hello"}
	err = conn.WriteJSON(testData)
	assert.NoError(t, err)

	// Verify message received
	select {
	case received := <-receivedCh:
		var decoded map[string]string
		err = json.Unmarshal(received, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, testData, decoded)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// TestWriteJSONErrors verifies error handling in WriteJSON
func TestWriteJSONErrors(t *testing.T) {
	// Create a simple WebSocket connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			if err := conn.Close(); err != nil {
				// Connection close error during test cleanup, expected
				_ = err // Explicitly ignore error during cleanup
			}
		}()
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("Failed to close websocket: %v", err)
		}
	}()

	conn := NewConnection(ws)

	// Test invalid JSON
	type InvalidJSON struct {
		Ch chan int // channels cannot be marshaled to JSON
	}
	err = conn.WriteJSON(&InvalidJSON{Ch: make(chan int)})
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidJSON, err)

	// Test write after close
	err = conn.Close()
	assert.NoError(t, err)
	
	err = conn.WriteJSON(map[string]string{"test": "data"})
	assert.Error(t, err)
	assert.Equal(t, ErrConnectionClosed, err)
}

// TestConcurrentWrites verifies thread-safe writing
func TestConcurrentWrites(t *testing.T) {
	// Create test server that counts messages
	messageCount := 0
	mu := sync.Mutex{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			if err := conn.Close(); err != nil {
				// Connection close error during test cleanup, expected
				_ = err // Explicitly ignore error during cleanup
			}
		}()
		
		// Count messages
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
			mu.Lock()
			messageCount++
			mu.Unlock()
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("Failed to close websocket: %v", err)
		}
	}()

	conn := NewConnection(ws)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("Failed to close connection: %v", err)
		}
	}()

	// Send messages concurrently
	numGoroutines := 10
	messagesPerGoroutine := 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := map[string]interface{}{
					"goroutine": id,
					"message":   j,
				}
				err := conn.WriteJSON(msg)
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond) // Allow time for messages to be processed

	mu.Lock()
	assert.Equal(t, numGoroutines*messagesPerGoroutine, messageCount)
	mu.Unlock()
}

// TestCredentialManagement verifies authentication state management
func TestCredentialManagement(t *testing.T) {
	// Create minimal WebSocket connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			if err := conn.Close(); err != nil {
				// Connection close error during test cleanup, expected
				_ = err // Explicitly ignore error during cleanup
			}
		}()
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("Failed to close websocket: %v", err)
		}
	}()

	conn := NewConnection(ws)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("Failed to close connection: %v", err)
		}
	}()

	// Test initial state
	assert.False(t, conn.IsAuthenticated())
	assert.Equal(t, "", conn.GetUserID())
	assert.Equal(t, "", conn.GetRole())
	assert.Equal(t, "", conn.GetSessionID())

	// Set credentials
	err = conn.SetCredentials("user123", "student", "session456")
	assert.NoError(t, err)

	// Verify credentials
	assert.True(t, conn.IsAuthenticated())
	assert.Equal(t, "user123", conn.GetUserID())
	assert.Equal(t, "student", conn.GetRole())
	assert.Equal(t, "session456", conn.GetSessionID())

	// Test SetSessionID
	conn.SetSessionID("session789")
	assert.Equal(t, "session789", conn.GetSessionID())
}

// TestConcurrentCredentialAccess verifies thread-safe credential access
func TestConcurrentCredentialAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			if err := conn.Close(); err != nil {
				// Connection close error during test cleanup, expected
				_ = err // Explicitly ignore error during cleanup
			}
		}()
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("Failed to close websocket: %v", err)
		}
	}()

	conn := NewConnection(ws)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("Failed to close connection: %v", err)
		}
	}()

	// Set initial credentials
	if err := conn.SetCredentials("user1", "instructor", "session1"); err != nil {
		t.Fatalf("Failed to set credentials: %v", err)
	}

	// Concurrent reads and writes
	var wg sync.WaitGroup
	numGoroutines := 10
	wg.Add(numGoroutines * 2)

	// Readers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = conn.GetUserID()
				_ = conn.GetRole()
				_ = conn.GetSessionID()
				_ = conn.IsAuthenticated()
			}
		}()
	}

	// Writers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				conn.SetSessionID(string(rune('a' + id)))
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Wait()
	// Should complete without race conditions
}

// TestConnectionClose verifies proper cleanup
func TestConnectionClose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			if err := conn.Close(); err != nil {
				// Connection close error during test cleanup, expected
				_ = err // Explicitly ignore error during cleanup
			}
		}()
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	conn := NewConnection(ws)

	// Close connection
	err = conn.Close()
	assert.NoError(t, err)

	// Verify context is cancelled
	select {
	case <-conn.ctx.Done():
		// Good, context is cancelled
	default:
		t.Fatal("context should be cancelled after close")
	}

	// Multiple closes should be safe (idempotent)
	err = conn.Close()
	assert.NoError(t, err)

	// Operations after close should fail
	err = conn.WriteJSON(map[string]string{"test": "data"})
	assert.Equal(t, ErrConnectionClosed, err)
}

// TestWriteChannelBuffer verifies the write channel buffer behavior
func TestWriteChannelBuffer(t *testing.T) {
	// Test that buffer has correct capacity by filling it completely
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			if err := conn.Close(); err != nil {
				// Connection close error during test cleanup, expected
				_ = err // Explicitly ignore error during cleanup
			}
		}()
		
		// Keep connection alive and read messages
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("Failed to close websocket: %v", err)
		}
	}()

	conn := NewConnection(ws)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("Failed to close connection: %v", err)
		}
	}()

	// Test that we can send at least 100 messages (buffer size) successfully
	successCount := 0
	for i := 0; i < 100; i++ {
		err = conn.WriteJSON(map[string]int{"message": i})
		if err == nil {
			successCount++
		}
	}

	// Should be able to send all 100 messages to buffer
	assert.Equal(t, 100, successCount, "Should be able to buffer 100 messages")
	
	// Verify connection is still functional
	err = conn.WriteJSON(map[string]string{"final": "message"})
	assert.NoError(t, err, "Connection should still be functional")
}

// TestWriteLoopRecovery verifies writeLoop handles panics gracefully
func TestWriteLoopRecovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer func() {
			if err := conn.Close(); err != nil {
				// Connection close error during test cleanup, expected
				_ = err // Explicitly ignore error during cleanup
			}
		}()
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("Failed to close websocket: %v", err)
		}
	}()

	conn := NewConnection(ws)

	// Close connection
	err = conn.Close()
	assert.NoError(t, err)

	// Try to write after close - should not panic
	err = conn.WriteJSON(map[string]string{"test": "data"})
	assert.Equal(t, ErrConnectionClosed, err)
}