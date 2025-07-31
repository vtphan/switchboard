// Integration tests for BroadcastSystem integration with MessageProcessor
// These tests validate the complete message delivery pipeline from WebSocket to recipients

package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/internal/message"
	"switchboard/internal/rate"
	"switchboard/internal/session"
	"switchboard/internal/websocket"
)

// TestBroadcastSystemIntegration validates the complete message delivery pipeline
func TestBroadcastSystemIntegration(t *testing.T) {
	t.Run("complete_message_delivery_pipeline", func(t *testing.T) {
		// Setup complete system integration
		dbManager, sessionManager, connectionRegistry, _, messageProcessor := setupBroadcastIntegrationEnvironment(t)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create mock WebSocket connections that track delivered messages
		instructorConn := &TrackingConnection{userID: "instructor1", role: "instructor"}
		student1Conn := &TrackingConnection{userID: "student1", role: "student"}
		student2Conn := &TrackingConnection{userID: "student2", role: "student"}

		// Register connections in the registry
		require.NoError(t, connectionRegistry.Register("instructor1", instructorConn))
		require.NoError(t, connectionRegistry.Register("student1", student1Conn))
		require.NoError(t, connectionRegistry.Register("student2", student2Conn))

		// Create active session
		session := &database.Session{
			ID:        "test-session-123",
			Name:      "Broadcast Integration Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Test 1: Student question to instructors (should only reach instructor)
		studentQuestion := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "I have a question about the assignment"},
		}

		questionData, _ := json.Marshal(studentQuestion)
		err := messageProcessor.ProcessIncomingMessage(questionData, "student1")
		require.NoError(t, err)

		// Allow time for async processing
		time.Sleep(100 * time.Millisecond)

		// Verify message reached instructor only
		assert.Len(t, instructorConn.GetDeliveredMessages(), 1, "Instructor should receive student question")
		assert.Len(t, student1Conn.GetDeliveredMessages(), 0, "Student sender should not see their own question")
		assert.Len(t, student2Conn.GetDeliveredMessages(), 0, "Other students should not see question (privacy)")

		// Verify message content
		instructorMsg := instructorConn.GetDeliveredMessages()[0]
		var msgData map[string]interface{}
		require.NoError(t, json.Unmarshal(instructorMsg, &msgData))
		assert.Equal(t, database.MessageTypeBroadcastToInstructors, msgData["type"])
		assert.Equal(t, "student1", msgData["from_user"])

		// Test 2: Instructor announcement to students (should reach all students and instructor)
		instructorConn.Reset()
		student1Conn.Reset()
		student2Conn.Reset()

		announcement := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToStudents,
			"context": database.ContextAnnouncement,
			"content": map[string]interface{}{"text": "Class will start in 5 minutes"},
		}

		announcementData, _ := json.Marshal(announcement)
		err = messageProcessor.ProcessIncomingMessage(announcementData, "instructor1")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Verify announcement reached all users
		assert.Len(t, instructorConn.GetDeliveredMessages(), 1, "Instructor should see their own announcement")
		assert.Len(t, student1Conn.GetDeliveredMessages(), 1, "Student1 should receive announcement")
		assert.Len(t, student2Conn.GetDeliveredMessages(), 1, "Student2 should receive announcement")

		// Test 3: Direct message (should only reach sender, recipient, and instructor)
		instructorConn.Reset()
		student1Conn.Reset()
		student2Conn.Reset()

		directMessage := map[string]interface{}{
			"type":    database.MessageTypeDirectMessage,
			"context": database.ContextResponse,
			"content": map[string]interface{}{"text": "Good work on your presentation!"},
			"to_user": "student2",
		}

		directData, _ := json.Marshal(directMessage)
		err = messageProcessor.ProcessIncomingMessage(directData, "instructor1")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Verify direct message filtering
		assert.Len(t, instructorConn.GetDeliveredMessages(), 1, "Instructor should see their own direct message")
		assert.Len(t, student1Conn.GetDeliveredMessages(), 0, "Student1 should not see direct message to student2")
		assert.Len(t, student2Conn.GetDeliveredMessages(), 1, "Student2 should receive direct message to them")

		// Verify message was persisted to database
		time.Sleep(200 * time.Millisecond) // Allow async persistence
		messages, err := dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Len(t, messages, 3, "All 3 messages should be persisted")
	})

	t.Run("broadcast_system_error_handling", func(t *testing.T) {
		// Test that broadcast failures don't break message processing
		dbManager, sessionManager, connectionRegistry, _, _ := setupBroadcastIntegrationEnvironment(t)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create a failing broadcast system
		failingBroadcastSystem := &FailingBroadcastSystem{shouldFail: true}

		// Create message router
		roleBasedFilter := message.NewRoleBasedFilter()
		messageRouter := message.NewMessageRouter(connectionRegistry, roleBasedFilter)
		rateLimiter := rate.NewRateLimiter()
		defer func() {
			rateLimiter.Stop()
		}()

		// Create processor with failing broadcast system
		messageProcessor := message.NewMessageProcessor(
			sessionManager,
			dbManager,
			rateLimiter,
			messageRouter,
			failingBroadcastSystem,
		)

		// Create active session
		session := &database.Session{
			ID:        "test-session-456",
			Name:      "Error Handling Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Process message even with failing broadcast system
		testMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test message"},
		}

		messageData, _ := json.Marshal(testMessage)
		err := messageProcessor.ProcessIncomingMessage(messageData, "student1")

		// Message processing should succeed even if broadcast fails
		require.NoError(t, err, "Message processing should continue even with broadcast failure")

		// Verify message was still persisted despite broadcast failure
		time.Sleep(100 * time.Millisecond)
		messages, err := dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Len(t, messages, 1, "Message should be persisted even with broadcast failure")
	})

	t.Run("concurrent_message_processing_with_broadcast", func(t *testing.T) {
		// Test concurrent message processing with broadcast system
		dbManager, sessionManager, connectionRegistry, _, messageProcessor := setupBroadcastIntegrationEnvironment(t)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create tracking connections
		instructorConn := &TrackingConnection{userID: "instructor1", role: "instructor"}
		require.NoError(t, connectionRegistry.Register("instructor1", instructorConn))

		// Create active session
		session := &database.Session{
			ID:        "concurrent-test-789",
			Name:      "Concurrent Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Send multiple messages concurrently
		const numMessages = 10
		var wg sync.WaitGroup
		errors := make(chan error, numMessages)

		for i := 0; i < numMessages; i++ {
			wg.Add(1)
			go func(msgNum int) {
				defer wg.Done()
				
				testMessage := map[string]interface{}{
					"type":    database.MessageTypeBroadcastToInstructors,
					"context": database.ContextQuestion,
					"content": map[string]interface{}{"text": fmt.Sprintf("Concurrent message %d", msgNum)},
				}

				messageData, _ := json.Marshal(testMessage)
				if err := messageProcessor.ProcessIncomingMessage(messageData, fmt.Sprintf("student%d", msgNum)); err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for any errors
		for err := range errors {
			t.Errorf("Concurrent message processing error: %v", err)
		}

		// Allow time for all async operations
		time.Sleep(300 * time.Millisecond)

		// Verify all messages were broadcast
		deliveredMessages := instructorConn.GetDeliveredMessages()
		assert.Equal(t, numMessages, len(deliveredMessages), "All concurrent messages should be broadcast")

		// Verify all messages were persisted
		messages, err := dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Equal(t, numMessages, len(messages), "All concurrent messages should be persisted")
	})
}

// setupBroadcastIntegrationEnvironment creates a complete integration environment
func setupBroadcastIntegrationEnvironment(t *testing.T) (*database.SQLiteDatabaseManager, session.SessionManager, *websocket.ConnectionRegistry, *websocket.BroadcastSystem, *message.MessageProcessor) {
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

	// Create session manager
	sessionManager := session.NewSessionManager(dbManager)

	// Create connection registry
	connectionRegistry := websocket.NewConnectionRegistry(sessionManager)
	connectionRegistry.Start()

	// Create broadcast system with role-based filter
	roleBasedFilter := message.NewRoleBasedFilter()
	filterAdapter := websocket.NewFilterAdapter(roleBasedFilter)
	broadcastSystem := websocket.NewBroadcastSystem(connectionRegistry, filterAdapter)

	// Create message router and rate limiter
	messageRouter := message.NewMessageRouter(connectionRegistry, roleBasedFilter)
	rateLimiter := rate.NewRateLimiter()

	// Create message processor with broadcast system
	messageProcessor := message.NewMessageProcessor(
		sessionManager,
		dbManager,
		rateLimiter,
		messageRouter,
		broadcastSystem,
	)

	return dbManager, sessionManager, connectionRegistry, broadcastSystem, messageProcessor
}

// TrackingConnection implements the Connection interface and tracks delivered messages
type TrackingConnection struct {
	userID           string
	role             string
	deliveredMessages [][]byte
	mu               sync.RWMutex
	closed           bool
}

func (tc *TrackingConnection) GetUserID() string {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.userID
}

func (tc *TrackingConnection) GetRole() string {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.role
}

func (tc *TrackingConnection) SendMessage(data []byte) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	
	if tc.closed {
		return errors.New("connection closed")
	}
	
	// Copy data to prevent external modifications
	msgCopy := make([]byte, len(data))
	copy(msgCopy, data)
	tc.deliveredMessages = append(tc.deliveredMessages, msgCopy)
	return nil
}

func (tc *TrackingConnection) GetDeliveredMessages() [][]byte {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	
	// Return copy to prevent external modifications
	result := make([][]byte, len(tc.deliveredMessages))
	for i, msg := range tc.deliveredMessages {
		result[i] = make([]byte, len(msg))
		copy(result[i], msg)
	}
	return result
}

func (tc *TrackingConnection) Reset() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.deliveredMessages = nil
}

func (tc *TrackingConnection) Close() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.closed = true
	return nil
}

// Additional required methods for Connection interface
func (tc *TrackingConnection) WriteJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return tc.SendMessage(data)
}

func (tc *TrackingConnection) SetCredentials(username, role string) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.userID = username
	tc.role = role
	return nil
}

func (tc *TrackingConnection) UpdateActivity() {}

func (tc *TrackingConnection) GetLastSeen() time.Time {
	return time.Now()
}

func (tc *TrackingConnection) SendCloseMessage(reason string) error {
	return nil
}

func (tc *TrackingConnection) Start(ctx context.Context, messageHandler func([]byte, string) error) {}

// FailingBroadcastSystem for error testing
type FailingBroadcastSystem struct {
	shouldFail bool
}

func (fbs *FailingBroadcastSystem) BroadcastMessage(message *database.Message, recipients []message.Recipient) error {
	if fbs.shouldFail {
		return errors.New("broadcast system failure")
	}
	return nil
}