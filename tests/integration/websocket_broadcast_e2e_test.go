// End-to-end WebSocket tests for BroadcastSystem integration
// These tests validate complete real-time message flow through WebSocket connections

package integration

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
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
	websocketpkg "switchboard/internal/websocket"
	"switchboard/web"
	"switchboard/web/api"
)

// TestWebSocketBroadcastE2E tests complete end-to-end message broadcasting through WebSocket
func TestWebSocketBroadcastE2E(t *testing.T) {
	t.Run("complete_websocket_broadcast_flow", func(t *testing.T) {
		// Setup complete application stack
		app := setupE2ETestApplication(t)
		defer app.Cleanup()

		// Start session via HTTP API
		sessionReq := map[string]string{
			"name":          "E2E Broadcast Test",
			"instructor_id": "instructor1",
		}
		reqBody, _ := json.Marshal(sessionReq)

		resp, err := http.Post(app.BaseURL+"/api/session/start", "application/json", bytes.NewReader(reqBody))
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}()
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		// Connect WebSocket clients
		instructorConn, instructorMsgs := app.ConnectWebSocket(t, "instructor1", "instructor")
		defer func() {
			if err := instructorConn.Close(); err != nil {
				t.Logf("Failed to close instructor connection: %v", err)
			}
		}()

		student1Conn, student1Msgs := app.ConnectWebSocket(t, "student1", "student")
		defer func() {
			if err := student1Conn.Close(); err != nil {
				t.Logf("Failed to close student1 connection: %v", err)
			}
		}()

		student2Conn, student2Msgs := app.ConnectWebSocket(t, "student2", "student")
		defer func() {
			if err := student2Conn.Close(); err != nil {
				t.Logf("Failed to close student2 connection: %v", err)
			}
		}()

		// Wait for initial connection messages
		time.Sleep(100 * time.Millisecond)

		// Test 1: Student question to instructors
		studentQuestion := map[string]interface{}{
			"type":    "broadcast_to_instructors",
			"context": "question",
			"content": map[string]interface{}{"text": "Can you explain the homework?"},
		}

		err = student1Conn.WriteJSON(studentQuestion)
		require.NoError(t, err)

		// Wait for message processing and broadcasting
		time.Sleep(200 * time.Millisecond)

		// Verify instructor received the question
		instructorMessages := readAllMessages(instructorMsgs)
		studentQuestions := filterMessagesByType(instructorMessages, "broadcast_to_instructors")
		assert.Len(t, studentQuestions, 1, "Instructor should receive student question")

		if len(studentQuestions) > 0 {
			assert.Equal(t, "student1", studentQuestions[0]["from_user"])
			content := studentQuestions[0]["content"].(map[string]interface{})
			assert.Equal(t, "Can you explain the homework?", content["text"])
		}

		// Verify students did not receive the question (privacy)
		student1Messages := readAllMessages(student1Msgs)
		student1Questions := filterMessagesByType(student1Messages, "broadcast_to_instructors")
		assert.Len(t, student1Questions, 0, "Student1 should not see their own question")

		student2Messages := readAllMessages(student2Msgs)
		student2Questions := filterMessagesByType(student2Messages, "broadcast_to_instructors")
		assert.Len(t, student2Questions, 0, "Student2 should not see other student's question")

		// Test 2: Instructor announcement to students
		announcement := map[string]interface{}{
			"type":    "broadcast_to_students",
			"context": "announcement",
			"content": map[string]interface{}{"text": "Quiz next week!"},
		}

		err = instructorConn.WriteJSON(announcement)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		// Verify all users received the announcement
		instructorMessages = readAllMessages(instructorMsgs)
		instructorAnnouncements := filterMessagesByType(instructorMessages, "broadcast_to_students")
		assert.Len(t, instructorAnnouncements, 1, "Instructor should see their own announcement")

		student1Messages = readAllMessages(student1Msgs)
		student1Announcements := filterMessagesByType(student1Messages, "broadcast_to_students")
		assert.Len(t, student1Announcements, 1, "Student1 should receive announcement")

		student2Messages = readAllMessages(student2Msgs)
		student2Announcements := filterMessagesByType(student2Messages, "broadcast_to_students")
		assert.Len(t, student2Announcements, 1, "Student2 should receive announcement")

		// Test 3: Direct message
		directMessage := map[string]interface{}{
			"type":    "direct_message",
			"context": "response",
			"content": map[string]interface{}{"text": "Great question!"},
			"to_user": "student1",
		}

		err = instructorConn.WriteJSON(directMessage)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		// Verify direct message filtering
		instructorMessages = readAllMessages(instructorMsgs)
		instructorDirectMsgs := filterMessagesByType(instructorMessages, "direct_message")
		assert.Len(t, instructorDirectMsgs, 1, "Instructor should see their own direct message")

		student1Messages = readAllMessages(student1Msgs)
		student1DirectMsgs := filterMessagesByType(student1Messages, "direct_message")
		assert.Len(t, student1DirectMsgs, 1, "Student1 should receive direct message")

		student2Messages = readAllMessages(student2Msgs)
		student2DirectMsgs := filterMessagesByType(student2Messages, "direct_message")
		assert.Len(t, student2DirectMsgs, 0, "Student2 should not receive direct message to student1")
	})

	t.Run("concurrent_websocket_broadcast", func(t *testing.T) {
		// Test concurrent message broadcasting
		app := setupE2ETestApplication(t)
		defer app.Cleanup()

		// Start session
		sessionReq := map[string]string{
			"name":          "Concurrent Broadcast Test",
			"instructor_id": "instructor1",
		}
		reqBody, _ := json.Marshal(sessionReq)

		resp, err := http.Post(app.BaseURL+"/api/session/start", "application/json", bytes.NewReader(reqBody))
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}()

		// Connect instructor to receive all messages
		instructorConn, instructorMsgs := app.ConnectWebSocket(t, "instructor1", "instructor")
		defer func() {
			if err := instructorConn.Close(); err != nil {
				t.Logf("Failed to close instructor connection: %v", err)
			}
		}()

		// Connect multiple students
		const numStudents = 5
		var studentConns []*websocket.Conn
		var wg sync.WaitGroup

		for i := 0; i < numStudents; i++ {
			conn, _ := app.ConnectWebSocket(t, fmt.Sprintf("student%d", i), "student")
			studentConns = append(studentConns, conn)
			defer func(c *websocket.Conn) {
				if err := c.Close(); err != nil {
					t.Logf("Failed to close student connection: %v", err)
				}
			}(conn)
		}

		time.Sleep(100 * time.Millisecond) // Allow connections to establish

		// Send concurrent messages from all students
		for i, conn := range studentConns {
			wg.Add(1)
			go func(studentID int, conn *websocket.Conn) {
				defer wg.Done()

				message := map[string]interface{}{
					"type":    "broadcast_to_instructors",
					"context": "question",
					"content": map[string]interface{}{"text": fmt.Sprintf("Question from student %d", studentID)},
				}

				err := conn.WriteJSON(message)
				if err != nil {
					t.Errorf("Failed to send message from student %d: %v", studentID, err)
				}
			}(i, conn)
		}

		wg.Wait()

		// Allow time for all messages to be processed and broadcast
		time.Sleep(500 * time.Millisecond)

		// Verify instructor received all student questions
		instructorMessages := readAllMessages(instructorMsgs)
		studentQuestions := filterMessagesByType(instructorMessages, "broadcast_to_instructors")
		assert.Equal(t, numStudents, len(studentQuestions), "Instructor should receive all student questions")

		// Verify all questions have different senders
		senders := make(map[string]bool)
		for _, msg := range studentQuestions {
			sender := msg["from_user"].(string)
			senders[sender] = true
		}
		assert.Equal(t, numStudents, len(senders), "All student questions should have different senders")
	})

	t.Run("websocket_broadcast_with_connection_drops", func(t *testing.T) {
		// Test that broadcast continues working when connections drop
		app := setupE2ETestApplication(t)
		defer app.Cleanup()

		// Start session
		sessionReq := map[string]string{
			"name":          "Connection Drop Test",
			"instructor_id": "instructor1",
		}
		reqBody, _ := json.Marshal(sessionReq)

		resp, err := http.Post(app.BaseURL+"/api/session/start", "application/json", bytes.NewReader(reqBody))
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}()

		// Connect clients
		instructorConn, instructorMsgs := app.ConnectWebSocket(t, "instructor1", "instructor")
		defer func() {
			if err := instructorConn.Close(); err != nil {
				t.Logf("Failed to close instructor connection: %v", err)
			}
		}()

		student1Conn, student1Msgs := app.ConnectWebSocket(t, "student1", "student")
		student2Conn, student2Msgs := app.ConnectWebSocket(t, "student2", "student")
		defer func() {
			if err := student2Conn.Close(); err != nil {
				t.Logf("Failed to close student2 connection: %v", err)
			}
		}()

		time.Sleep(100 * time.Millisecond)

		// Drop student1 connection
		if err := student1Conn.Close(); err != nil {
			t.Logf("Failed to close student1 connection: %v", err)
		}
		time.Sleep(100 * time.Millisecond) // Allow cleanup

		// Send announcement - should still reach remaining connections
		announcement := map[string]interface{}{
			"type":    "broadcast_to_students",
			"context": "announcement",
			"content": map[string]interface{}{"text": "Test after connection drop"},
		}

		err = instructorConn.WriteJSON(announcement)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)

		// Verify instructor and remaining student received message
		instructorMessages := readAllMessages(instructorMsgs)
		instructorAnnouncements := filterMessagesByType(instructorMessages, "broadcast_to_students")
		assert.Len(t, instructorAnnouncements, 1, "Instructor should receive announcement")

		student2Messages := readAllMessages(student2Msgs)
		student2Announcements := filterMessagesByType(student2Messages, "broadcast_to_students")
		assert.Len(t, student2Announcements, 1, "Student2 should receive announcement")

		// Verify dropped connection didn't prevent broadcast
		student1Messages := readAllMessages(student1Msgs)
		assert.Len(t, student1Messages, 0, "Dropped connection should not receive new messages")
	})
}

// E2ETestApplication represents a complete test application setup
type E2ETestApplication struct {
	Server       *httptest.Server
	BaseURL      string
	dbManager    *database.SQLiteDatabaseManager
	rateLimiter  *rate.RateLimiter
	registry     *websocketpkg.ConnectionRegistry
}

func (app *E2ETestApplication) ConnectWebSocket(t *testing.T, userID, role string) (*websocket.Conn, <-chan map[string]interface{}) {
	wsURL := "ws" + strings.TrimPrefix(app.BaseURL, "http") + fmt.Sprintf("/ws?user_id=%s&role=%s", userID, role)
	
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// Create channel to collect messages
	messages := make(chan map[string]interface{}, 100)

	// Start message reader
	go func() {
		defer close(messages)
		for {
			var message map[string]interface{}
			err := conn.ReadJSON(&message)
			if err != nil {
				return // Connection closed
			}
			
			select {
			case messages <- message:
			case <-time.After(1 * time.Second):
				// Prevent blocking if channel is full
			}
		}
	}()

	return conn, messages
}

func (app *E2ETestApplication) Cleanup() {
	if app.Server != nil {
		app.Server.Close()
	}
	if app.rateLimiter != nil {
		app.rateLimiter.Stop()
	}
	if app.registry != nil {
		app.registry.Stop()
	}
	if app.dbManager != nil {
		if err := app.dbManager.Stop(); err != nil {
			// Use log.Printf since this is not in a test function context
			// and we don't have access to testing.T
			fmt.Printf("Failed to stop database manager: %v\n", err)
		}
	}
}

func setupE2ETestApplication(t *testing.T) *E2ETestApplication {
	// Setup in-memory database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Apply database schema
	schema, err := os.ReadFile("../../internal/database/migrations.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(schema))
	require.NoError(t, err)

	// Create database manager
	// Skip table initialization and pragmas since we already applied migrations.sql
	dbManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
		SkipTableInit: true,
		SkipPragmas:   true,
	})
	require.NoError(t, err)
	require.NoError(t, dbManager.Start())

	// Create session management
	sessionManager := session.NewSessionManager(dbManager)
	sessionLifecycle := session.NewSessionLifecycle(sessionManager, dbManager)

	// Create message processing components
	rateLimiter := rate.NewRateLimiter()
	connectionRegistry := websocketpkg.NewConnectionRegistry(sessionManager)
	connectionRegistry.Start()

	roleBasedFilter := message.NewRoleBasedFilter()
	messageRouter := message.NewMessageRouter(connectionRegistry, roleBasedFilter)

	// Create and wire BroadcastSystem
	filterAdapter := websocketpkg.NewFilterAdapter(roleBasedFilter)
	broadcastSystem := websocketpkg.NewBroadcastSystem(connectionRegistry, filterAdapter)

	messageProcessor := message.NewMessageProcessor(
		sessionManager,
		dbManager,
		rateLimiter,
		messageRouter,
		broadcastSystem,
	)

	// Create WebSocket handler
	websocketHandler := websocketpkg.NewWebSocketHandler(
		connectionRegistry,
		sessionManager,
		messageProcessor,
		dbManager,
	)

	// Create API handlers
	sessionAPIHandler := api.NewSessionAPIHandler(sessionLifecycle)

	// Create HTTP server
	httpServer := web.NewHTTPServer(sessionAPIHandler, websocketHandler)

	// Create test server
	testServer := httptest.NewServer(httpServer.GetHandler())

	return &E2ETestApplication{
		Server:      testServer,
		BaseURL:     testServer.URL,
		dbManager:   dbManager,
		rateLimiter: rateLimiter,
		registry:    connectionRegistry,
	}
}

// Helper functions for message handling

func readAllMessages(msgChan <-chan map[string]interface{}) []map[string]interface{} {
	var messages []map[string]interface{}
	
	// Non-blocking read of all available messages
	for {
		select {
		case msg, ok := <-msgChan:
			if !ok {
				return messages
			}
			messages = append(messages, msg)
		case <-time.After(10 * time.Millisecond):
			// No more messages available
			return messages
		}
	}
}

func filterMessagesByType(messages []map[string]interface{}, messageType string) []map[string]interface{} {
	var filtered []map[string]interface{}
	
	for _, msg := range messages {
		if msgType, ok := msg["type"].(string); ok && msgType == messageType {
			filtered = append(filtered, msg)
		}
	}
	
	return filtered
}