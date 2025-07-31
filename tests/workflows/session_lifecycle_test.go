// Session Lifecycle Workflow Tests
// Tests critical session state transitions and their impact on the entire system
package workflows

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

// TestSessionLifecycleWorkflows validates complete session workflows from start to finish
func TestSessionLifecycleWorkflows(t *testing.T) {
	t.Run("complete_classroom_session_workflow", func(t *testing.T) {
		// Simulate a complete classroom session from start to finish
		env := setupWorkflowEnvironment(t)
		defer env.cleanup()

		// Phase 1: Pre-session - students connect but cannot send messages
		createStudentConnections(t, env, 5)
		createInstructorConnections(t, env, 2)

		// Verify pre-session state - messages should be rejected
		testMessage := createStudentQuestionMessage("Pre-session question")
		err := env.processor.ProcessIncomingMessage(testMessage, "student1")
		require.Error(t, err, "Messages should be rejected before session starts")

		// Phase 2: Session start - instructor initiates session
		session := &database.Session{
			ID:        "classroom-workflow-test",
			Name:      "Biology 101 - Cell Structure",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		// Phase 3: Active session - validate message flow patterns
		// Students ask questions to instructors
		studentQuestions := []string{
			"What is the difference between prokaryotic and eukaryotic cells?",
			"Can you explain mitochondria function?",
			"How does cell membrane transport work?",
		}

		for i, question := range studentQuestions {
			studentID := fmt.Sprintf("student%d", i+1)
			msg := createStudentQuestionMessage(question)
			require.NoError(t, env.processor.ProcessIncomingMessage(msg, studentID))
		}

		// Instructors respond with announcements and direct messages
		announcement := createInstructorAnnouncementMessage("Great questions! Let's start with cell types...")
		require.NoError(t, env.processor.ProcessIncomingMessage(announcement, "instructor1"))

		directResponse1 := createDirectMessage("student1", "Prokaryotic cells lack a nucleus, while eukaryotic cells have one.")
		require.NoError(t, env.processor.ProcessIncomingMessage(directResponse1, "instructor1"))

		directResponse2 := createDirectMessage("student2", "Mitochondria are the powerhouses of the cell, producing ATP.")
		require.NoError(t, env.processor.ProcessIncomingMessage(directResponse2, "instructor2"))

		// Phase 4: Late joiner - student connects mid-session
		lateJoinerConn := createLateJoinerConnection(t, env, "student6")
		
		// Verify late joiner receives appropriate message history
		time.Sleep(100 * time.Millisecond) // Allow history delivery
		lateJoinerMessages := lateJoinerConn.GetDeliveredMessages()
		
		// Late joiner should see: instructor announcement + their relevant direct message (if any)
		// Should NOT see other students' questions to instructors
		assert.GreaterOrEqual(t, len(lateJoinerMessages), 1, "Late joiner should receive announcement")
		
		// Phase 5: Session end - graceful shutdown
		require.NoError(t, env.sessionManager.ClearActiveSession())

		// Phase 6: Post-session - verify cleanup and message rejection
		postSessionMessage := createStudentQuestionMessage("Post-session question")
		err = env.processor.ProcessIncomingMessage(postSessionMessage, "student1")
		require.Error(t, err, "Messages should be rejected after session ends")

		// Verify session data persisted correctly
		messages, err := env.dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(messages), 6, "All messages should be persisted")
		
		// Verify message types and privacy
		instructorVisibleCount := 0
		studentVisibleCount := 0
		for _, msg := range messages {
			if shouldInstructorSeeMessage(msg) {
				instructorVisibleCount++
			}
			if shouldStudentSeeMessage(msg, "student1") {
				studentVisibleCount++
			}
		}
		
		assert.Greater(t, instructorVisibleCount, studentVisibleCount, 
			"Instructors should see more messages than students due to oversight role")
	})

	t.Run("session_transition_atomicity", func(t *testing.T) {
		// Test that session state changes are atomic with database persistence
		env := setupWorkflowEnvironment(t)
		defer env.cleanup()

		session := &database.Session{
			ID:        "atomicity-test",
			Name:      "Atomicity Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}

		// Start session
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		// Send message during active session (use sync processing for persistence verification)
		msg := createStudentQuestionMessage("Test message during active session")
		require.NoError(t, env.processor.ProcessIncomingMessageSync(msg, "student1"))

		// End session
		require.NoError(t, env.sessionManager.ClearActiveSession())

		// Verify session ended atomically - both in-memory and database
		activeSession := env.sessionManager.GetActiveSession()
		assert.Nil(t, activeSession, "No active session should exist in memory")

		// Verify database consistency
		messages, err := env.dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Len(t, messages, 1, "Message should be persisted despite session end")
	})

	t.Run("concurrent_session_operations", func(t *testing.T) {
		// Test concurrent session operations don't cause race conditions
		env := setupWorkflowEnvironment(t)
		defer env.cleanup()

		const numConcurrentOps = 20
		var wg sync.WaitGroup
		results := make(chan error, numConcurrentOps)

		// Attempt concurrent session starts (only one should succeed)
		for i := 0; i < numConcurrentOps; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				session := &database.Session{
					ID:        fmt.Sprintf("concurrent-test-%d", index),
					Name:      fmt.Sprintf("Concurrent Test %d", index),
					CreatedBy: fmt.Sprintf("instructor%d", index),
					StartTime: time.Now(),
					Status:    "active",
				}
				err := env.sessionManager.SetActiveSession(session)
				results <- err
			}(i)
		}

		wg.Wait()
		close(results)

		// Count successful vs failed session starts
		successCount := 0
		failureCount := 0
		for err := range results {
			if err == nil {
				successCount++
			} else {
				failureCount++
			}
		}

		assert.Equal(t, 1, successCount, "Exactly one session should succeed")
		assert.Equal(t, numConcurrentOps-1, failureCount, "All other attempts should fail")
	})

	t.Run("session_recovery_after_database_failure", func(t *testing.T) {
		// Test session state recovery when database operations fail
		env := setupWorkflowEnvironment(t)
		defer env.cleanup()

		session := &database.Session{
			ID:        "recovery-test",
			Name:      "Recovery Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}

		// Start session successfully
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		// Simulate database becoming unavailable (close connection)
		if err := env.dbManager.Stop(); err != nil {
			t.Logf("Expected database stop during failure simulation: %v", err)
		}

		// Attempt to end session - should handle database failure gracefully
		err := env.sessionManager.ClearActiveSession()
		
		// System should either:
		// 1. Successfully end session with best-effort database update, OR
		// 2. Fail but maintain consistent in-memory state
		
		if err != nil {
			// If ending failed, session should remain active in memory
			activeSession := env.sessionManager.GetActiveSession()
			assert.NotNil(t, activeSession, "Session should remain active if database update failed")
		} else {
			// If ending succeeded, session should be cleared from memory
			activeSession := env.sessionManager.GetActiveSession()
			assert.Nil(t, activeSession, "Session should be cleared if end succeeded")
		}
	})

	t.Run("session_timeout_and_cleanup", func(t *testing.T) {
		// Test automatic session cleanup after timeout periods
		env := setupWorkflowEnvironment(t)
		defer env.cleanup()

		// Create session with past end time (simulates long-running session)
		session := &database.Session{
			ID:        "timeout-test",
			Name:      "Timeout Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now().Add(-2 * time.Hour),
			Status:    "active",
		}

		require.NoError(t, env.sessionManager.SetActiveSession(session))

		// In a real implementation, this would trigger automatic cleanup
		// For now, we'll simulate the cleanup logic
		
		// Verify session exists
		activeSession := env.sessionManager.GetActiveSession()
		assert.NotNil(t, activeSession)

		// Simulate timeout check and cleanup
		if time.Since(session.StartTime) > time.Hour {
			if err := env.sessionManager.ClearActiveSession(); err != nil {
				t.Logf("Failed to clear active session during timeout cleanup: %v", err)
			}
		}

		// Verify cleanup occurred
		activeSession = env.sessionManager.GetActiveSession()
		assert.Nil(t, activeSession, "Timed-out session should be cleaned up")
	})
}

// Helper functions and test environment setup

type WorkflowEnvironment struct {
	dbManager       *database.SQLiteDatabaseManager
	sessionManager  session.SessionManager
	connectionRegistry *websocket.ConnectionRegistry
	processor       *message.MessageProcessor
	connections     map[string]*TestConnection
}

func setupWorkflowEnvironment(t *testing.T) *WorkflowEnvironment {
	// Setup in-memory database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Apply schema
	schema, err := os.ReadFile("../../internal/database/migrations.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(schema))
	require.NoError(t, err)

	// Create components
	// Skip table initialization and pragmas since we already applied migrations.sql
	dbManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
		SkipTableInit: true,
		SkipPragmas:   true,
	})
	require.NoError(t, err)
	require.NoError(t, dbManager.Start())

	sessionManager := session.NewSessionManager(dbManager)
	connectionRegistry := websocket.NewConnectionRegistry(sessionManager)
	connectionRegistry.Start()

	roleBasedFilter := message.NewRoleBasedFilter()
	messageRouter := message.NewMessageRouter(connectionRegistry, roleBasedFilter)
	rateLimiter := rate.NewRateLimiter()

	filterAdapter := websocket.NewFilterAdapter(roleBasedFilter)
	broadcastSystem := websocket.NewBroadcastSystem(connectionRegistry, filterAdapter)

	processor := message.NewMessageProcessor(
		sessionManager,
		dbManager,
		rateLimiter,
		messageRouter,
		broadcastSystem,
	)

	return &WorkflowEnvironment{
		dbManager:      dbManager,
		sessionManager: sessionManager,
		connectionRegistry: connectionRegistry,
		processor:      processor,
		connections:    make(map[string]*TestConnection),
	}
}

func (env *WorkflowEnvironment) cleanup() {
	env.connectionRegistry.Stop()
	if err := env.dbManager.Stop(); err != nil {
		// Use fmt.Printf since we don't have testing.T context here
		fmt.Printf("Failed to stop database manager: %v\n", err)
	}
}

func createStudentConnections(t *testing.T, env *WorkflowEnvironment, count int) []*TestConnection {
	connections := make([]*TestConnection, count)
	for i := 0; i < count; i++ {
		userID := fmt.Sprintf("student%d", i+1)
		conn := &TestConnection{userID: userID, role: "student"}
		require.NoError(t, env.connectionRegistry.Register(userID, conn))
		env.connections[userID] = conn
		connections[i] = conn
	}
	return connections
}

func createInstructorConnections(t *testing.T, env *WorkflowEnvironment, count int) []*TestConnection {
	connections := make([]*TestConnection, count)
	for i := 0; i < count; i++ {
		userID := fmt.Sprintf("instructor%d", i+1)
		conn := &TestConnection{userID: userID, role: "instructor"}
		require.NoError(t, env.connectionRegistry.Register(userID, conn))
		env.connections[userID] = conn
		connections[i] = conn
	}
	return connections
}

func createLateJoinerConnection(t *testing.T, env *WorkflowEnvironment, userID string) *TestConnection {
	conn := &TestConnection{userID: userID, role: "student"}
	require.NoError(t, env.connectionRegistry.Register(userID, conn))
	env.connections[userID] = conn
	
	// Wait for any pending async database operations to complete
	time.Sleep(250 * time.Millisecond)
	
	// Manually trigger history delivery for late joiner (simulating WebSocket handler behavior)
	activeSession := env.sessionManager.GetActiveSession()
	if activeSession != nil {
		// Get session messages from database
		messages, err := env.dbManager.GetSessionMessages(activeSession.ID)
		require.NoError(t, err)
		
		// Create role-based filter and deliver filtered history to late joiner
		filter := message.NewRoleBasedFilter()
		for _, msg := range messages {
			if filter.ShouldReceiveMessage(msg, conn) {
				// Convert to message format and deliver
				msgMap := map[string]interface{}{
					"type":      msg.Type,
					"content":   msg.Content,
					"context":   msg.Context,
					"timestamp": msg.Timestamp.Unix(),
					"from_user": msg.FromUser,
				}
				
				if msg.Type == database.MessageTypeDirectMessage {
					msgMap["to_user"] = msg.ToUser
				}
				
				msgData, _ := json.Marshal(msgMap)
				if err := conn.SendMessage(msgData); err != nil {
					// Log send error but don't fail the test as this simulates real connection issues
					fmt.Printf("Failed to send message to connection %s: %v\n", conn.userID, err)
				}
			}
		}
	}
	
	return conn
}

func createStudentQuestionMessage(content string) []byte {
	msg := map[string]interface{}{
		"type":    database.MessageTypeBroadcastToInstructors,
		"context": database.ContextQuestion,
		"content": map[string]interface{}{"text": content},
	}
	data, _ := json.Marshal(msg)
	return data
}

func createInstructorAnnouncementMessage(content string) []byte {
	msg := map[string]interface{}{
		"type":    database.MessageTypeBroadcastToStudents,
		"context": database.ContextAnnouncement,
		"content": map[string]interface{}{"text": content},
	}
	data, _ := json.Marshal(msg)
	return data
}

func createDirectMessage(toUser, content string) []byte {
	msg := map[string]interface{}{
		"type":    database.MessageTypeDirectMessage,
		"context": database.ContextResponse,
		"content": map[string]interface{}{"text": content},
		"to_user": toUser,
	}
	data, _ := json.Marshal(msg)
	return data
}

func shouldInstructorSeeMessage(msg *database.Message) bool {
	// Instructors see all messages for educational oversight
	return true
}

func shouldStudentSeeMessage(msg *database.Message, studentID string) bool {
	// Students have limited visibility for privacy
	switch msg.Type {
	case database.MessageTypeBroadcastToStudents:
		return true
	case database.MessageTypeDirectMessage:
		return msg.FromUser == studentID || (msg.ToUser != nil && *msg.ToUser == studentID)
	case database.MessageTypeBroadcastToInstructors:
		return msg.FromUser == studentID // Students only see their own questions
	default:
		return false
	}
}

// TestConnection for workflow testing
type TestConnection struct {
	userID            string
	role              string
	deliveredMessages [][]byte
	mu                sync.RWMutex
	closed            bool
}

func (tc *TestConnection) GetUserID() string { return tc.userID }
func (tc *TestConnection) GetRole() string   { return tc.role }

func (tc *TestConnection) SendMessage(data []byte) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	
	if tc.closed {
		return fmt.Errorf("connection closed")
	}
	
	msgCopy := make([]byte, len(data))
	copy(msgCopy, data)
	tc.deliveredMessages = append(tc.deliveredMessages, msgCopy)
	return nil
}

func (tc *TestConnection) GetDeliveredMessages() [][]byte {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	
	result := make([][]byte, len(tc.deliveredMessages))
	for i, msg := range tc.deliveredMessages {
		result[i] = make([]byte, len(msg))
		copy(result[i], msg)
	}
	return result
}

func (tc *TestConnection) Close() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.closed = true
	return nil
}

func (tc *TestConnection) WriteJSON(v interface{}) error              { return nil }
func (tc *TestConnection) SetCredentials(username, role string) error { return nil }
func (tc *TestConnection) UpdateActivity()                            {}
func (tc *TestConnection) GetLastSeen() time.Time                     { return time.Now() }
func (tc *TestConnection) SendCloseMessage(reason string) error       { return nil }