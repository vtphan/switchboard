// Error handling tests for BroadcastSystem integration
// These tests validate that the system gracefully handles various error conditions

package integration

import (
	"database/sql"
	"encoding/json"
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

// TestBroadcastErrorHandling validates error handling in broadcast system integration
func TestBroadcastErrorHandling(t *testing.T) {
	t.Run("broadcast_failure_continues_processing", func(t *testing.T) {
		// Test that message processing continues even when broadcast fails
		dbManager, sessionManager, _, _, messageProcessor := setupErrorHandlingEnvironment(t, true)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create session
		session := &database.Session{
			ID:        "error-test-1",
			Name:      "Broadcast Failure Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Process message with failing broadcast system
		testMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Message despite broadcast failure"},
		}

		messageData, _ := json.Marshal(testMessage)
		err := messageProcessor.ProcessIncomingMessage(messageData, "student1")

		// Processing should succeed despite broadcast failure
		require.NoError(t, err, "Message processing should continue despite broadcast failure")

		// Verify message was persisted despite broadcast failure
		time.Sleep(100 * time.Millisecond) // Allow async persistence
		messages, err := dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Len(t, messages, 1, "Message should be persisted despite broadcast failure")
		assert.Equal(t, "student1", messages[0].FromUser)
	})

	t.Run("partial_broadcast_failure", func(t *testing.T) {
		// Test handling when some recipients fail to receive messages
		dbManager, sessionManager, connectionRegistry, _, messageProcessor := setupErrorHandlingEnvironment(t, false)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create mixed connections: some working, some failing
		workingConn := &ReliableConnection{userID: "instructor1", role: "instructor"}
		failingConn := &FailingConnection{userID: "instructor2", role: "instructor", shouldFail: true}
		anotherWorkingConn := &ReliableConnection{userID: "student1", role: "student"}

		require.NoError(t, connectionRegistry.Register("instructor1", workingConn))
		require.NoError(t, connectionRegistry.Register("instructor2", failingConn))
		require.NoError(t, connectionRegistry.Register("student1", anotherWorkingConn))

		// Create session
		session := &database.Session{
			ID:     "partial-failure-test",
			Status: "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Send message to instructors (mixed success/failure)
		testMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Partial broadcast test"},
		}

		messageData, _ := json.Marshal(testMessage)
		err := messageProcessor.ProcessIncomingMessage(messageData, "student1")

		// Processing should succeed even with partial broadcast failure
		require.NoError(t, err, "Processing should succeed with partial broadcast failure")

		time.Sleep(100 * time.Millisecond)

		// Verify working connections received the message
		workingMessages := workingConn.GetDeliveredMessages()
		assert.Len(t, workingMessages, 1, "Working connection should receive message")

		// Verify failing connection did not receive message
		failingMessages := failingConn.GetDeliveredMessages()
		assert.Len(t, failingMessages, 0, "Failing connection should not receive message")

		// Verify message was still persisted
		messages, err := dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Len(t, messages, 1, "Message should be persisted despite partial broadcast failure")
	})

	t.Run("broadcast_timeout_handling", func(t *testing.T) {
		// Test handling of slow/timeout connections during broadcast
		dbManager, sessionManager, connectionRegistry, _, messageProcessor := setupErrorHandlingEnvironment(t, false)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create connections with different response times
		fastConn := &ReliableConnection{userID: "instructor1", role: "instructor"}
		slowConn := &SlowConnection{userID: "instructor2", role: "instructor", delay: 50 * time.Millisecond}
		normalConn := &ReliableConnection{userID: "student1", role: "student"}

		require.NoError(t, connectionRegistry.Register("instructor1", fastConn))
		require.NoError(t, connectionRegistry.Register("instructor2", slowConn))
		require.NoError(t, connectionRegistry.Register("student1", normalConn))

		// Create session
		session := &database.Session{
			ID:     "timeout-test",
			Status: "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Measure broadcast time
		start := time.Now()

		testMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Timeout test message"},
		}

		messageData, _ := json.Marshal(testMessage)
		err := messageProcessor.ProcessIncomingMessage(messageData, "student1")
		require.NoError(t, err)

		processingTime := time.Since(start)

		// Processing should not be significantly delayed by slow connections
		assert.Less(t, processingTime, 200*time.Millisecond, "Processing should not be blocked by slow connections")

		time.Sleep(200 * time.Millisecond) // Allow all deliveries to complete

		// Verify all connections eventually received the message
		assert.Len(t, fastConn.GetDeliveredMessages(), 1, "Fast connection should receive message")
		assert.Len(t, slowConn.GetDeliveredMessages(), 1, "Slow connection should eventually receive message")
	})

	t.Run("database_failure_with_broadcast_success", func(t *testing.T) {
		// Test scenario where broadcast succeeds but database persistence fails
		dbManager, sessionManager, connectionRegistry, _, messageProcessor := setupErrorHandlingEnvironment(t, false)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create working connection
		workingConn := &ReliableConnection{userID: "instructor1", role: "instructor"}
		require.NoError(t, connectionRegistry.Register("instructor1", workingConn))

		// Create session
		session := &database.Session{
			ID:     "db-failure-test",
			Status: "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Configure database to fail writes (simulate database error)
		// Note: This would need to be enhanced in real implementation to force DB failures

		testMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "DB failure test"},
		}

		messageData, _ := json.Marshal(testMessage)
		err := messageProcessor.ProcessIncomingMessage(messageData, "student1")

		// Processing should succeed (broadcast works even if persistence fails)
		require.NoError(t, err, "Processing should succeed despite database issues")

		// Verify broadcast succeeded
		workingMessages := workingConn.GetDeliveredMessages()
		assert.Len(t, workingMessages, 1, "Broadcast should succeed despite database failure")
	})

	t.Run("connection_registry_error_handling", func(t *testing.T) {
		// Test handling of connection registry errors during recipient lookup
		dbManager, sessionManager, connectionRegistry, _, messageProcessor := setupErrorHandlingEnvironment(t, false)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create session
		session := &database.Session{
			ID:     "registry-error-test",
			Status: "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Stop connection registry to simulate registry failure
		connectionRegistry.Stop()

		testMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Registry error test"},
		}

		messageData, _ := json.Marshal(testMessage)
		err := messageProcessor.ProcessIncomingMessage(messageData, "student1")

		// Processing should handle registry errors gracefully
		require.NoError(t, err, "Processing should handle registry errors gracefully")

		time.Sleep(100 * time.Millisecond)

		// Verify message was still persisted
		messages, err := dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Len(t, messages, 1, "Message should be persisted despite registry errors")
	})

	t.Run("concurrent_error_scenarios", func(t *testing.T) {
		// Test error handling under concurrent load
		dbManager, sessionManager, connectionRegistry, _, messageProcessor := setupErrorHandlingEnvironment(t, false)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create mixed connection types
		connections := []websocket.ConnectionInterface{
			&ReliableConnection{userID: "instructor1", role: "instructor"},
			&FailingConnection{userID: "instructor2", role: "instructor", shouldFail: true},
			&SlowConnection{userID: "student1", role: "student", delay: 30 * time.Millisecond},
			&ReliableConnection{userID: "student2", role: "student"},
		}

		for _, conn := range connections {
			require.NoError(t, connectionRegistry.Register(conn.GetUserID(), conn))
		}

		// Create session
		session := &database.Session{
			ID:     "concurrent-error-test",
			Status: "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Send concurrent messages with various error conditions
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
					"content": map[string]interface{}{"text": fmt.Sprintf("Concurrent error test %d", msgNum)},
				}

				messageData, _ := json.Marshal(testMessage)
				if err := messageProcessor.ProcessIncomingMessage(messageData, fmt.Sprintf("student%d", msgNum)); err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check that no processing errors occurred despite connection failures
		for err := range errors {
			t.Errorf("Unexpected processing error during concurrent scenario: %v", err)
		}

		time.Sleep(300 * time.Millisecond) // Allow all processing to complete

		// Verify messages were persisted despite various failures
		messages, err := dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Equal(t, numMessages, len(messages), "All messages should be persisted despite connection failures")
	})

	t.Run("malformed_message_broadcast_handling", func(t *testing.T) {
		// Test that broadcast system handles malformed message data gracefully
		dbManager, sessionManager, connectionRegistry, _, messageProcessor := setupErrorHandlingEnvironment(t, false)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create working connection
		workingConn := &ReliableConnection{userID: "instructor1", role: "instructor"}
		require.NoError(t, connectionRegistry.Register("instructor1", workingConn))

		// Create session
		session := &database.Session{
			ID:     "malformed-test",
			Status: "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Send malformed JSON that passes initial validation but may cause broadcast issues
		malformedMessage := `{"type":"broadcast_to_instructors","context":"question","content":null,"from_user":"student1"}`

		err := messageProcessor.ProcessIncomingMessage([]byte(malformedMessage), "student1")

		// Processing should handle malformed data gracefully
		require.NoError(t, err, "Processing should handle malformed message data")

		time.Sleep(100 * time.Millisecond)

		// System should attempt to broadcast despite malformed content
		// (BroadcastSystem should handle JSON marshaling errors gracefully)
		workingMessages := workingConn.GetDeliveredMessages()
		assert.Len(t, workingMessages, 1, "System should attempt broadcast despite malformed content")
	})
}

// setupErrorHandlingEnvironment creates environment for error handling tests
func setupErrorHandlingEnvironment(t *testing.T, failBroadcast bool) (*database.SQLiteDatabaseManager, session.SessionManager, *websocket.ConnectionRegistry, message.BroadcastSystemInterface, *message.MessageProcessor) {
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

	// Create message router
	roleBasedFilter := message.NewRoleBasedFilter()
	messageRouter := message.NewMessageRouter(connectionRegistry, roleBasedFilter)
	rateLimiter := rate.NewRateLimiter()

	// Create broadcast system (failing or working based on test needs)
	var broadcastSystem message.BroadcastSystemInterface
	if failBroadcast {
		broadcastSystem = &ErrorBroadcastSystem{shouldFail: true}
	} else {
		filterAdapter := websocket.NewFilterAdapter(roleBasedFilter)
		broadcastSystem = websocket.NewBroadcastSystem(connectionRegistry, filterAdapter)
	}

	// Create message processor
	messageProcessor := message.NewMessageProcessor(
		sessionManager,
		dbManager,
		rateLimiter,
		messageRouter,
		broadcastSystem,
	)

	return dbManager, sessionManager, connectionRegistry, broadcastSystem, messageProcessor
}

// Mock connection implementations for error testing

// ReliableConnection always succeeds
type ReliableConnection struct {
	userID            string
	role              string
	deliveredMessages [][]byte
	mu                sync.RWMutex
}

func (rc *ReliableConnection) GetUserID() string { return rc.userID }
func (rc *ReliableConnection) GetRole() string   { return rc.role }

func (rc *ReliableConnection) SendMessage(data []byte) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	
	msgCopy := make([]byte, len(data))
	copy(msgCopy, data)
	rc.deliveredMessages = append(rc.deliveredMessages, msgCopy)
	return nil
}

func (rc *ReliableConnection) GetDeliveredMessages() [][]byte {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	
	result := make([][]byte, len(rc.deliveredMessages))
	for i, msg := range rc.deliveredMessages {
		result[i] = make([]byte, len(msg))
		copy(result[i], msg)
	}
	return result
}

func (rc *ReliableConnection) WriteJSON(v interface{}) error              { return nil }
func (rc *ReliableConnection) Close() error                               { return nil }
func (rc *ReliableConnection) SetCredentials(username, role string) error { return nil }
func (rc *ReliableConnection) UpdateActivity()                            {}
func (rc *ReliableConnection) GetLastSeen() time.Time                     { return time.Now() }
func (rc *ReliableConnection) SendCloseMessage(reason string) error       { return nil }

// FailingConnection always fails message delivery
type FailingConnection struct {
	userID            string
	role              string
	shouldFail        bool
	deliveredMessages [][]byte
	mu                sync.RWMutex
}

func (fc *FailingConnection) GetUserID() string { return fc.userID }
func (fc *FailingConnection) GetRole() string   { return fc.role }

func (fc *FailingConnection) SendMessage(data []byte) error {
	if fc.shouldFail {
		return fmt.Errorf("connection failure simulation")
	}
	
	fc.mu.Lock()
	defer fc.mu.Unlock()
	
	msgCopy := make([]byte, len(data))
	copy(msgCopy, data)
	fc.deliveredMessages = append(fc.deliveredMessages, msgCopy)
	return nil
}

func (fc *FailingConnection) GetDeliveredMessages() [][]byte {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	
	result := make([][]byte, len(fc.deliveredMessages))
	for i, msg := range fc.deliveredMessages {
		result[i] = make([]byte, len(msg))
		copy(result[i], msg)
	}
	return result
}

func (fc *FailingConnection) WriteJSON(v interface{}) error              { return nil }
func (fc *FailingConnection) Close() error                               { return nil }
func (fc *FailingConnection) SetCredentials(username, role string) error { return nil }
func (fc *FailingConnection) UpdateActivity()                            {}
func (fc *FailingConnection) GetLastSeen() time.Time                     { return time.Now() }
func (fc *FailingConnection) SendCloseMessage(reason string) error       { return nil }

// SlowConnection introduces delay in message delivery
type SlowConnection struct {
	userID            string
	role              string
	delay             time.Duration
	deliveredMessages [][]byte
	mu                sync.RWMutex
}

func (sc *SlowConnection) GetUserID() string { return sc.userID }
func (sc *SlowConnection) GetRole() string   { return sc.role }

func (sc *SlowConnection) SendMessage(data []byte) error {
	// Simulate slow delivery
	time.Sleep(sc.delay)
	
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	msgCopy := make([]byte, len(data))
	copy(msgCopy, data)
	sc.deliveredMessages = append(sc.deliveredMessages, msgCopy)
	return nil
}

func (sc *SlowConnection) GetDeliveredMessages() [][]byte {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	
	result := make([][]byte, len(sc.deliveredMessages))
	for i, msg := range sc.deliveredMessages {
		result[i] = make([]byte, len(msg))
		copy(result[i], msg)
	}
	return result
}

func (sc *SlowConnection) WriteJSON(v interface{}) error              { return nil }
func (sc *SlowConnection) Close() error                               { return nil }
func (sc *SlowConnection) SetCredentials(username, role string) error { return nil }
func (sc *SlowConnection) UpdateActivity()                            {}
func (sc *SlowConnection) GetLastSeen() time.Time                     { return time.Now() }
func (sc *SlowConnection) SendCloseMessage(reason string) error       { return nil }

// ErrorBroadcastSystem that always fails
type ErrorBroadcastSystem struct {
	shouldFail bool
}

func (ebs *ErrorBroadcastSystem) BroadcastMessage(message *database.Message, recipients []message.Recipient) error {
	if ebs.shouldFail {
		return fmt.Errorf("broadcast system failure simulation")
	}
	return nil
}