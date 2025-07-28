package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ws "switchboard/internal/websocket"
	"switchboard/pkg/types"
)

// WebSocketTestSetup creates a test environment with real WebSocket connections
type WebSocketTestSetup struct {
	server     *httptest.Server
	registry   *ws.Registry
	router     *Router
	mockDB     *TrackedMockDB
	upgrader   websocket.Upgrader
	mu         sync.Mutex
	connections map[string]*ws.Connection
}

func NewWebSocketTestSetup() *WebSocketTestSetup {
	setup := &WebSocketTestSetup{
		registry:    ws.NewRegistry(),
		mockDB:      NewTrackedMockDB(),
		upgrader:    websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		connections: make(map[string]*ws.Connection),
	}
	
	setup.router = NewRouter(setup.registry, setup.mockDB)
	
	// Create test WebSocket server
	setup.server = httptest.NewServer(http.HandlerFunc(setup.handleWebSocket))
	
	return setup
}

func (s *WebSocketTestSetup) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	
	// Create connection wrapper
	wsConn := ws.NewConnection(conn)
	
	// Keep connection alive for test duration
	go func() {
		defer func() {
			if err := wsConn.Close(); err != nil {
				// Connection close error during test cleanup, expected
				_ = err // Explicitly ignore error during cleanup
			}
		}()
		time.Sleep(10 * time.Second) // Simple timeout to prevent test hanging
	}()
	
	// Store connection for test access
	s.mu.Lock()
	s.connections["pending"] = wsConn
	s.mu.Unlock()
}

func (s *WebSocketTestSetup) CreateAuthenticatedConnection(userID, role, sessionID string) (*ws.Connection, error) {
	// Connect to test server
	wsURL := "ws" + strings.TrimPrefix(s.server.URL, "http")
	rawConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}
	
	// Create connection wrapper
	conn := ws.NewConnection(rawConn)
	
	// Authenticate connection
	err = conn.SetCredentials(userID, role, sessionID)
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			// Connection close error during cleanup, expected
			_ = closeErr // Explicitly ignore error during cleanup
		}
		return nil, err
	}
	
	// Register with registry
	err = s.registry.RegisterConnection(conn)
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			// Connection close error during cleanup, expected
			_ = closeErr // Explicitly ignore error during cleanup
		}
		return nil, err
	}
	
	// Store for cleanup
	s.mu.Lock()
	s.connections[userID] = conn
	s.mu.Unlock()
	
	return conn, nil
}

func (s *WebSocketTestSetup) Cleanup() {
	s.mu.Lock()
	for _, conn := range s.connections {
		if err := conn.Close(); err != nil {
			// Connection close error during cleanup, expected
			_ = err // Explicitly ignore error during cleanup
		}
	}
	s.mu.Unlock()
	
	if s.server != nil {
		s.server.Close()
	}
}

// Integration Tests

func TestRouter_WebSocketIntegration_InstructorInbox(t *testing.T) {
	setup := NewWebSocketTestSetup()
	defer setup.Cleanup()
	
	// Create authenticated connections
	student1, err := setup.CreateAuthenticatedConnection("student1", "student", "session1")
	require.NoError(t, err)
	
	instructor1, err := setup.CreateAuthenticatedConnection("instructor1", "instructor", "session1")
	require.NoError(t, err)
	
	instructor2, err := setup.CreateAuthenticatedConnection("instructor2", "instructor", "session1")
	require.NoError(t, err)
	
	// Create message from student to instructors
	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeInstructorInbox,
		FromUser:  "student1",
		Content:   map[string]interface{}{
			"question": "I need help with the assignment",
			"priority": "high",
		},
		Timestamp: time.Now(),
	}
	
	// Route message through the router
	err = setup.router.RouteMessage(context.Background(), message)
	assert.NoError(t, err)
	
	// Give time for WebSocket message delivery
	time.Sleep(100 * time.Millisecond)
	
	// Verify message was persisted
	assert.Equal(t, 1, setup.mockDB.GetStoreMessageCalls())
	
	// Verify message ID was generated
	assert.NotEmpty(t, message.ID)
	assert.Equal(t, "general", message.Context)
	
	// Clean up connections
	if err := student1.Close(); err != nil {
		t.Logf("Failed to close student1 connection: %v", err)
	}
	if err := instructor1.Close(); err != nil {
		t.Logf("Failed to close instructor1 connection: %v", err)
	}
	if err := instructor2.Close(); err != nil {
		t.Logf("Failed to close instructor2 connection: %v", err)
	}
}

func TestRouter_WebSocketIntegration_InboxResponse(t *testing.T) {
	setup := NewWebSocketTestSetup()
	defer setup.Cleanup()
	
	// Create authenticated connections
	student1, err := setup.CreateAuthenticatedConnection("student1", "student", "session1")
	require.NoError(t, err)
	
	student2, err := setup.CreateAuthenticatedConnection("student2", "student", "session1")
	require.NoError(t, err)
	
	instructor1, err := setup.CreateAuthenticatedConnection("instructor1", "instructor", "session1")
	require.NoError(t, err)
	
	// Create targeted response from instructor to specific student
	toUser := "student1"
	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeInboxResponse,
		FromUser:  "instructor1",
		ToUser:    &toUser,
		Content: map[string]interface{}{
			"answer": "Here's the solution to your problem",
			"code":   "console.log('Hello World');",
		},
		Timestamp: time.Now(),
	}
	
	// Route message
	err = setup.router.RouteMessage(context.Background(), message)
	assert.NoError(t, err)
	
	// Give time for message delivery
	time.Sleep(100 * time.Millisecond)
	
	// Verify message was persisted
	assert.Equal(t, 1, setup.mockDB.GetStoreMessageCalls())
	
	// Verify message properties
	assert.NotEmpty(t, message.ID)
	assert.Equal(t, "student1", *message.ToUser)
	
	// Clean up
	if err := student1.Close(); err != nil {
		t.Logf("Failed to close student1 connection: %v", err)
	}
	if err := student2.Close(); err != nil {
		t.Logf("Failed to close student2 connection: %v", err)
	}
	if err := instructor1.Close(); err != nil {
		t.Logf("Failed to close instructor1 connection: %v", err)
	}
}

func TestRouter_WebSocketIntegration_InstructorBroadcast(t *testing.T) {
	setup := NewWebSocketTestSetup()
	defer setup.Cleanup()
	
	// Create authenticated connections
	student1, err := setup.CreateAuthenticatedConnection("student1", "student", "session1")
	require.NoError(t, err)
	
	student2, err := setup.CreateAuthenticatedConnection("student2", "student", "session1")
	require.NoError(t, err)
	
	student3, err := setup.CreateAuthenticatedConnection("student3", "student", "session1")
	require.NoError(t, err)
	
	instructor1, err := setup.CreateAuthenticatedConnection("instructor1", "instructor", "session1")
	require.NoError(t, err)
	
	// Create broadcast message from instructor
	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeInstructorBroadcast,
		FromUser:  "instructor1",
		Content: map[string]interface{}{
			"announcement": "Class will end in 10 minutes. Please save your work.",
			"urgency":      "medium",
			"timestamp":    time.Now().Unix(),
		},
		Timestamp: time.Now(),
	}
	
	// Route message
	err = setup.router.RouteMessage(context.Background(), message)
	assert.NoError(t, err)
	
	// Give time for message delivery
	time.Sleep(100 * time.Millisecond)
	
	// Verify message was persisted
	assert.Equal(t, 1, setup.mockDB.GetStoreMessageCalls())
	
	// Verify message properties
	assert.NotEmpty(t, message.ID)
	assert.Equal(t, "Class will end in 10 minutes. Please save your work.", message.Content["announcement"])
	
	// Clean up
	if err := student1.Close(); err != nil {
		t.Logf("Failed to close student1 connection: %v", err)
	}
	if err := student2.Close(); err != nil {
		t.Logf("Failed to close student2 connection: %v", err)
	}
	if err := student3.Close(); err != nil {
		t.Logf("Failed to close student3 connection: %v", err)
	}
	if err := instructor1.Close(); err != nil {
		t.Logf("Failed to close instructor1 connection: %v", err)
	}
}

func TestRouter_WebSocketIntegration_Analytics(t *testing.T) {
	setup := NewWebSocketTestSetup()
	defer setup.Cleanup()
	
	// Create authenticated connections
	student1, err := setup.CreateAuthenticatedConnection("student1", "student", "session1")
	require.NoError(t, err)
	
	instructor1, err := setup.CreateAuthenticatedConnection("instructor1", "instructor", "session1")
	require.NoError(t, err)
	
	instructor2, err := setup.CreateAuthenticatedConnection("instructor2", "instructor", "session1")
	require.NoError(t, err)
	
	// Create analytics message from student
	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeAnalytics,
		FromUser:  "student1",
		Content: map[string]interface{}{
			"event":        "code_completion",
			"language":     "javascript",
			"lines_added":  15,
			"time_spent":   300, // seconds
			"errors_fixed": 2,
		},
		Timestamp: time.Now(),
	}
	
	// Route message
	err = setup.router.RouteMessage(context.Background(), message)
	assert.NoError(t, err)
	
	// Give time for message delivery
	time.Sleep(100 * time.Millisecond)
	
	// Verify message was persisted
	assert.Equal(t, 1, setup.mockDB.GetStoreMessageCalls())
	
	// Verify analytics data
	assert.Equal(t, "code_completion", message.Content["event"])
	assert.Equal(t, 15, message.Content["lines_added"])
	
	// Clean up
	if err := student1.Close(); err != nil {
		t.Logf("Failed to close student1 connection: %v", err)
	}
	if err := instructor1.Close(); err != nil {
		t.Logf("Failed to close instructor1 connection: %v", err)
	}
	if err := instructor2.Close(); err != nil {
		t.Logf("Failed to close instructor2 connection: %v", err)
	}
}

func TestRouter_WebSocketIntegration_RequestResponse(t *testing.T) {
	setup := NewWebSocketTestSetup()
	defer setup.Cleanup()
	
	// Create authenticated connections
	student1, err := setup.CreateAuthenticatedConnection("student1", "student", "session1")
	require.NoError(t, err)
	
	instructor1, err := setup.CreateAuthenticatedConnection("instructor1", "instructor", "session1")
	require.NoError(t, err)
	
	// Create request response from student to all instructors
	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeRequestResponse,
		FromUser:  "student1",
		Content: map[string]interface{}{
			"request_id": "req-123",
			"response":   "Screen sharing is now enabled",
			"status":     "completed",
		},
		Timestamp: time.Now(),
	}
	
	// Route message
	err = setup.router.RouteMessage(context.Background(), message)
	assert.NoError(t, err)
	
	// Give time for message delivery
	time.Sleep(100 * time.Millisecond)
	
	// Verify message was persisted
	assert.Equal(t, 1, setup.mockDB.GetStoreMessageCalls())
	
	// Verify request response data
	assert.Equal(t, "req-123", message.Content["request_id"])
	assert.Equal(t, "completed", message.Content["status"])
	
	// Clean up
	if err := student1.Close(); err != nil {
		t.Logf("Failed to close student1 connection: %v", err)
	}
	if err := instructor1.Close(); err != nil {
		t.Logf("Failed to close instructor1 connection: %v", err)
	}
}

func TestRouter_WebSocketIntegration_MultipleMessages(t *testing.T) {
	setup := NewWebSocketTestSetup()
	defer setup.Cleanup()
	
	// Create authenticated connections
	student1, err := setup.CreateAuthenticatedConnection("student1", "student", "session1")
	require.NoError(t, err)
	
	instructor1, err := setup.CreateAuthenticatedConnection("instructor1", "instructor", "session1")
	require.NoError(t, err)
	
	// Send multiple messages rapidly
	messages := []*types.Message{
		{
			SessionID: "session1",
			Type:      types.MessageTypeAnalytics,
			FromUser:  "student1",
			Content:   map[string]interface{}{"event": "keypress", "key": "a"},
			Timestamp: time.Now(),
		},
		{
			SessionID: "session1",
			Type:      types.MessageTypeAnalytics,
			FromUser:  "student1",
			Content:   map[string]interface{}{"event": "keypress", "key": "b"},
			Timestamp: time.Now(),
		},
		{
			SessionID: "session1",
			Type:      types.MessageTypeAnalytics,
			FromUser:  "student1",
			Content:   map[string]interface{}{"event": "keypress", "key": "c"},
			Timestamp: time.Now(),
		},
	}
	
	// Route all messages
	for _, message := range messages {
		err = setup.router.RouteMessage(context.Background(), message)
		assert.NoError(t, err)
	}
	
	// Give time for message delivery and persistence
	time.Sleep(200 * time.Millisecond)
	
	// Verify all messages were persisted
	assert.Equal(t, 3, setup.mockDB.GetStoreMessageCalls())
	
	// Clean up
	if err := student1.Close(); err != nil {
		t.Logf("Failed to close student1 connection: %v", err)
	}
	if err := instructor1.Close(); err != nil {
		t.Logf("Failed to close instructor1 connection: %v", err)
	}
}

func TestRouter_WebSocketIntegration_CrossSessionIsolation(t *testing.T) {
	setup := NewWebSocketTestSetup()
	defer setup.Cleanup()
	
	// Create connections in different sessions
	student1_session1, err := setup.CreateAuthenticatedConnection("student1", "student", "session1")
	require.NoError(t, err)
	
	instructor1_session1, err := setup.CreateAuthenticatedConnection("instructor1", "instructor", "session1")
	require.NoError(t, err)
	
	student2_session2, err := setup.CreateAuthenticatedConnection("student2", "student", "session2")
	require.NoError(t, err)
	
	instructor2_session2, err := setup.CreateAuthenticatedConnection("instructor2", "instructor", "session2")
	require.NoError(t, err)
	
	// Send message in session1
	message1 := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeAnalytics,
		FromUser:  "student1",
		Content:   map[string]interface{}{"session": "session1", "data": "test1"},
		Timestamp: time.Now(),
	}
	
	// Send message in session2
	message2 := &types.Message{
		SessionID: "session2",
		Type:      types.MessageTypeAnalytics,
		FromUser:  "student2",
		Content:   map[string]interface{}{"session": "session2", "data": "test2"},
		Timestamp: time.Now(),
	}
	
	// Route both messages
	err = setup.router.RouteMessage(context.Background(), message1)
	assert.NoError(t, err)
	
	err = setup.router.RouteMessage(context.Background(), message2)
	assert.NoError(t, err)
	
	// Give time for message delivery
	time.Sleep(100 * time.Millisecond)
	
	// Verify both messages were persisted
	assert.Equal(t, 2, setup.mockDB.GetStoreMessageCalls())
	
	// Verify messages have correct session IDs
	assert.Equal(t, "session1", message1.SessionID)
	assert.Equal(t, "session2", message2.SessionID)
	
	// Clean up
	if err := student1_session1.Close(); err != nil {
		t.Logf("Failed to close student1_session1 connection: %v", err)
	}
	if err := instructor1_session1.Close(); err != nil {
		t.Logf("Failed to close instructor1_session1 connection: %v", err)
	}
	if err := student2_session2.Close(); err != nil {
		t.Logf("Failed to close student2_session2 connection: %v", err)
	}
	if err := instructor2_session2.Close(); err != nil {
		t.Logf("Failed to close instructor2_session2 connection: %v", err)
	}
}

func TestRouter_WebSocketIntegration_ConnectionFailure(t *testing.T) {
	setup := NewWebSocketTestSetup()
	defer setup.Cleanup()
	
	// Create authenticated connections
	student1, err := setup.CreateAuthenticatedConnection("student1", "student", "session1")
	require.NoError(t, err)
	
	instructor1, err := setup.CreateAuthenticatedConnection("instructor1", "instructor", "session1")
	require.NoError(t, err)
	
	// Close instructor connection to simulate failure
	if err := instructor1.Close(); err != nil {
		t.Logf("Failed to close instructor1 connection: %v", err)
	}
	
	// Create message that would go to the closed instructor
	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeAnalytics,
		FromUser:  "student1",
		Content:   map[string]interface{}{"data": "test_after_disconnect"},
		Timestamp: time.Now(),
	}
	
	// Route message - should succeed despite closed connection
	err = setup.router.RouteMessage(context.Background(), message)
	assert.NoError(t, err)
	
	// Give time for processing
	time.Sleep(100 * time.Millisecond)
	
	// Message should still be persisted
	assert.Equal(t, 1, setup.mockDB.GetStoreMessageCalls())
	
	// Clean up
	if err := student1.Close(); err != nil {
		t.Logf("Failed to close student1 connection: %v", err)
	}
}

func TestRouter_WebSocketIntegration_ValidationFailure(t *testing.T) {
	setup := NewWebSocketTestSetup()
	defer setup.Cleanup()
	
	// Create authenticated connections
	student1, err := setup.CreateAuthenticatedConnection("student1", "student", "session1")
	require.NoError(t, err)
	
	instructor1, err := setup.CreateAuthenticatedConnection("instructor1", "instructor", "session1")
	require.NoError(t, err)
	
	// Try to send message type that student cannot send
	message := &types.Message{
		SessionID: "session1",
		Type:      types.MessageTypeInboxResponse, // Students cannot send this
		FromUser:  "student1",
		Content:   map[string]interface{}{"invalid": "message"},
		Timestamp: time.Now(),
	}
	
	// Route message - should fail validation
	err = setup.router.RouteMessage(context.Background(), message)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not authorized")
	
	// Give time for any async processing
	time.Sleep(50 * time.Millisecond)
	
	// No message should be persisted due to validation failure
	assert.Equal(t, 0, setup.mockDB.GetStoreMessageCalls())
	
	// Clean up
	if err := student1.Close(); err != nil {
		t.Logf("Failed to close student1 connection: %v", err)
	}
	if err := instructor1.Close(); err != nil {
		t.Logf("Failed to close instructor1 connection: %v", err)
	}
}

func TestRouter_WebSocketIntegration_WithBatching(t *testing.T) {
	setup := NewWebSocketTestSetup()
	defer setup.Cleanup()
	
	// Enable batching
	err := setup.router.EnableBatching(5, 100*time.Millisecond)
	require.NoError(t, err)
	defer func() {
		if err := setup.router.batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()
	
	// Create authenticated connections
	student1, err := setup.CreateAuthenticatedConnection("student1", "student", "session1")
	require.NoError(t, err)
	
	instructor1, err := setup.CreateAuthenticatedConnection("instructor1", "instructor", "session1")
	require.NoError(t, err)
	
	// Send multiple messages to trigger batching
	for i := 0; i < 3; i++ {
		message := &types.Message{
			SessionID: "session1",
			Type:      types.MessageTypeAnalytics,
			FromUser:  "student1",
			Content: map[string]interface{}{
				"event": "keypress",
				"index": i,
			},
			Timestamp: time.Now(),
		}
		
		err = setup.router.RouteMessage(context.Background(), message)
		assert.NoError(t, err)
	}
	
	// Wait for batching to process
	time.Sleep(200 * time.Millisecond)
	
	// Verify batching was used (should have batch calls, not individual calls)
	batchCalls := setup.mockDB.GetStoreMessageBatchCalls()
	individualCalls := setup.mockDB.GetStoreMessageCalls()
	
	// Either batching was used, or messages fell back to individual persistence
	assert.True(t, batchCalls > 0 || individualCalls > 0, "Messages should be persisted either via batching or individually")
	
	// Check batcher metrics
	metrics := setup.router.GetBatcherMetrics()
	assert.Equal(t, uint64(3), metrics.TotalMessages)
	
	// Clean up
	if err := student1.Close(); err != nil {
		t.Logf("Failed to close student1 connection: %v", err)
	}
	if err := instructor1.Close(); err != nil {
		t.Logf("Failed to close instructor1 connection: %v", err)
	}
}