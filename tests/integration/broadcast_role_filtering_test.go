// Role-based filtering tests for BroadcastSystem integration
// These tests validate that educational privacy rules are properly enforced during broadcasting

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

// TestBroadcastRoleFiltering validates role-based filtering during message broadcasting
func TestBroadcastRoleFiltering(t *testing.T) {
	t.Run("instructor_oversight_filtering", func(t *testing.T) {
		// Test that instructors see all messages for educational oversight
		dbManager, sessionManager, connectionRegistry, _, messageProcessor := setupRoleFilteringEnvironment(t)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create connections: 2 instructors, 3 students
		instructor1 := &FilteringTrackingConnection{userID: "instructor1", role: "instructor"}
		instructor2 := &FilteringTrackingConnection{userID: "instructor2", role: "instructor"}
		student1 := &FilteringTrackingConnection{userID: "student1", role: "student"}
		student2 := &FilteringTrackingConnection{userID: "student2", role: "student"}
		student3 := &FilteringTrackingConnection{userID: "student3", role: "student"}

		// Register all connections
		connections := []*FilteringTrackingConnection{instructor1, instructor2, student1, student2, student3}
		for _, conn := range connections {
			require.NoError(t, connectionRegistry.Register(conn.GetUserID(), conn))
		}

		// Create active session
		session := &database.Session{
			ID:        "oversight-test",
			Name:      "Instructor Oversight Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Send different types of messages and verify instructor oversight

		// 1. Student question to instructors
		studentQuestion := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Question from student1"},
		}
		questionData, _ := json.Marshal(studentQuestion)
		err := messageProcessor.ProcessIncomingMessage(questionData, "student1")
		require.NoError(t, err)

		// 2. Direct message between instructor and student
		directMessage := map[string]interface{}{
			"type":    database.MessageTypeDirectMessage,
			"context": database.ContextResponse,
			"content": map[string]interface{}{"text": "Private feedback"},
			"to_user": "student2",
		}
		directData, _ := json.Marshal(directMessage)
		err = messageProcessor.ProcessIncomingMessage(directData, "instructor1")
		require.NoError(t, err)

		// 3. Instructor announcement to students  
		announcement := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToStudents,
			"context": database.ContextAnnouncement,
			"content": map[string]interface{}{"text": "Class announcement"},
		}
		announcementData, _ := json.Marshal(announcement)
		err = messageProcessor.ProcessIncomingMessage(announcementData, "instructor2")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond) // Allow processing

		// Verify instructor oversight: both instructors should see ALL messages
		instructor1Messages := instructor1.GetDeliveredMessages()
		instructor2Messages := instructor2.GetDeliveredMessages()

		assert.Len(t, instructor1Messages, 3, "Instructor1 should see all messages (oversight)")
		assert.Len(t, instructor2Messages, 3, "Instructor2 should see all messages (oversight)")

		// Verify students see filtered messages
		student1Messages := student1.GetDeliveredMessages()
		student2Messages := student2.GetDeliveredMessages()
		student3Messages := student3.GetDeliveredMessages()

		// Student1 should see: announcement only (not their own question, not direct to student2)
		assert.Len(t, student1Messages, 1, "Student1 should see only announcement")
		assertMessageType(t, student1Messages[0], database.MessageTypeBroadcastToStudents)

		// Student2 should see: direct message to them + announcement
		assert.Len(t, student2Messages, 2, "Student2 should see direct message and announcement")
		messageTypes := getMessageTypes(student2Messages)
		assert.Contains(t, messageTypes, database.MessageTypeDirectMessage)
		assert.Contains(t, messageTypes, database.MessageTypeBroadcastToStudents)

		// Student3 should see: announcement only
		assert.Len(t, student3Messages, 1, "Student3 should see only announcement")
		assertMessageType(t, student3Messages[0], database.MessageTypeBroadcastToStudents)
	})

	t.Run("student_privacy_filtering", func(t *testing.T) {
		// Test that students cannot see other students' questions (privacy protection)
		dbManager, sessionManager, connectionRegistry, _, messageProcessor := setupRoleFilteringEnvironment(t)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create connections: 1 instructor, 3 students
		instructor := &FilteringTrackingConnection{userID: "instructor1", role: "instructor"}
		student1 := &FilteringTrackingConnection{userID: "student1", role: "student"}
		student2 := &FilteringTrackingConnection{userID: "student2", role: "student"}
		student3 := &FilteringTrackingConnection{userID: "student3", role: "student"}

		connections := []*FilteringTrackingConnection{instructor, student1, student2, student3}
		for _, conn := range connections {
			require.NoError(t, connectionRegistry.Register(conn.GetUserID(), conn))
		}

		// Create active session
		session := &database.Session{
			ID:     "privacy-test",
			Status: "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Each student sends a question to instructors
		for i := 1; i <= 3; i++ {
			question := map[string]interface{}{
				"type":    database.MessageTypeBroadcastToInstructors,
				"context": database.ContextQuestion,
				"content": map[string]interface{}{"text": fmt.Sprintf("Question from student%d", i)},
			}
			questionData, _ := json.Marshal(question)
			err := messageProcessor.ProcessIncomingMessage(questionData, fmt.Sprintf("student%d", i))
			require.NoError(t, err)
		}

		time.Sleep(100 * time.Millisecond)

		// Verify privacy protection
		instructorMessages := instructor.GetDeliveredMessages()
		student1Messages := student1.GetDeliveredMessages()
		student2Messages := student2.GetDeliveredMessages()
		student3Messages := student3.GetDeliveredMessages()

		// Instructor should see all 3 questions
		assert.Len(t, instructorMessages, 3, "Instructor should see all student questions")

		// Each student should see NO questions (privacy protection)
		assert.Len(t, student1Messages, 0, "Student1 should not see any questions (privacy)")
		assert.Len(t, student2Messages, 0, "Student2 should not see any questions (privacy)")
		assert.Len(t, student3Messages, 0, "Student3 should not see any questions (privacy)")

		// Verify instructor can see questions from all students
		senders := make(map[string]bool)
		for _, msg := range instructorMessages {
			var msgData map[string]interface{}
			if err := json.Unmarshal(msg, &msgData); err != nil {
				t.Logf("Failed to unmarshal message: %v", err)
				continue
			}
			sender := msgData["from_user"].(string)
			senders[sender] = true
		}
		assert.Len(t, senders, 3, "Instructor should see questions from all 3 students")
		assert.Contains(t, senders, "student1")
		assert.Contains(t, senders, "student2")
		assert.Contains(t, senders, "student3")
	})

	t.Run("direct_message_privacy_filtering", func(t *testing.T) {
		// Test that direct messages are only visible to participants and instructors
		dbManager, sessionManager, connectionRegistry, _, messageProcessor := setupRoleFilteringEnvironment(t)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create connections
		instructor1 := &FilteringTrackingConnection{userID: "instructor1", role: "instructor"}
		instructor2 := &FilteringTrackingConnection{userID: "instructor2", role: "instructor"}
		student1 := &FilteringTrackingConnection{userID: "student1", role: "student"}
		student2 := &FilteringTrackingConnection{userID: "student2", role: "student"}
		student3 := &FilteringTrackingConnection{userID: "student3", role: "student"}

		connections := []*FilteringTrackingConnection{instructor1, instructor2, student1, student2, student3}
		for _, conn := range connections {
			require.NoError(t, connectionRegistry.Register(conn.GetUserID(), conn))
		}

		// Create active session
		session := &database.Session{
			ID:     "direct-message-test",
			Status: "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Send direct message from instructor1 to student2
		directMessage := map[string]interface{}{
			"type":    database.MessageTypeDirectMessage,
			"context": database.ContextResponse,
			"content": map[string]interface{}{"text": "Individual feedback for student2"},
			"to_user": "student2",
		}
		directData, _ := json.Marshal(directMessage)
		err := messageProcessor.ProcessIncomingMessage(directData, "instructor1")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Verify direct message filtering
		instructor1Messages := instructor1.GetDeliveredMessages()
		instructor2Messages := instructor2.GetDeliveredMessages()
		student1Messages := student1.GetDeliveredMessages()
		student2Messages := student2.GetDeliveredMessages()
		student3Messages := student3.GetDeliveredMessages()

		// Both instructors should see the direct message (oversight)
		assert.Len(t, instructor1Messages, 1, "Instructor1 (sender) should see direct message")
		assert.Len(t, instructor2Messages, 1, "Instructor2 should see direct message (oversight)")

		// Only student2 (recipient) should see the direct message
		assert.Len(t, student1Messages, 0, "Student1 should not see direct message to student2")
		assert.Len(t, student2Messages, 1, "Student2 (recipient) should see direct message")
		assert.Len(t, student3Messages, 0, "Student3 should not see direct message to student2")

		// Verify message content is correct
		var student2MsgData map[string]interface{}
		if err := json.Unmarshal(student2Messages[0], &student2MsgData); err != nil {
			t.Fatalf("Failed to unmarshal student2 message: %v", err)
		}
		assert.Equal(t, "instructor1", student2MsgData["from_user"])
		assert.Equal(t, "student2", student2MsgData["to_user"])
		content := student2MsgData["content"].(map[string]interface{})
		assert.Equal(t, "Individual feedback for student2", content["text"])
	})

	t.Run("mixed_message_types_filtering", func(t *testing.T) {
		// Test complex scenario with multiple message types and proper filtering
		dbManager, sessionManager, connectionRegistry, _, messageProcessor := setupRoleFilteringEnvironment(t)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Failed to stop database manager: %v", err)
			}
		}()

		// Create diverse user set
		instructor := &FilteringTrackingConnection{userID: "instructor1", role: "instructor"}
		students := []*FilteringTrackingConnection{
			{userID: "student1", role: "student"},
			{userID: "student2", role: "student"},
			{userID: "student3", role: "student"},
		}

		require.NoError(t, connectionRegistry.Register(instructor.GetUserID(), instructor))
		for _, student := range students {
			require.NoError(t, connectionRegistry.Register(student.GetUserID(), student))
		}

		// Create active session
		session := &database.Session{
			ID:     "mixed-messages-test",
			Status: "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Complex message sequence
		messages := []struct {
			data     map[string]interface{}
			sender   string
			expected map[string]int // role -> expected message count after this message
		}{
			{
				// Student1 asks question
				data: map[string]interface{}{
					"type": database.MessageTypeBroadcastToInstructors,
					"context": database.ContextQuestion,
					"content": map[string]interface{}{"text": "Question 1"},
				},
				sender: "student1",
				expected: map[string]int{
					"instructor": 1, "student1": 0, "student2": 0, "student3": 0,
				},
			},
			{
				// Instructor announces to all
				data: map[string]interface{}{
					"type": database.MessageTypeBroadcastToStudents,
					"context": database.ContextAnnouncement,
					"content": map[string]interface{}{"text": "Announcement 1"},
				},
				sender: "instructor1",
				expected: map[string]int{
					"instructor": 2, "student1": 1, "student2": 1, "student3": 1,
				},
			},
			{
				// Direct message to student2
				data: map[string]interface{}{
					"type": database.MessageTypeDirectMessage,
					"context": database.ContextResponse,
					"content": map[string]interface{}{"text": "Direct to student2"},
					"to_user": "student2",
				},
				sender: "instructor1",
				expected: map[string]int{
					"instructor": 3, "student1": 1, "student2": 2, "student3": 1,
				},
			},
			{
				// Student3 asks question
				data: map[string]interface{}{
					"type": database.MessageTypeBroadcastToInstructors,
					"context": database.ContextQuestion,
					"content": map[string]interface{}{"text": "Question 2"},
				},
				sender: "student3",
				expected: map[string]int{
					"instructor": 4, "student1": 1, "student2": 2, "student3": 1,
				},
			},
		}

		// Send messages and verify filtering at each step
		for i, msg := range messages {
			msgData, _ := json.Marshal(msg.data)
			err := messageProcessor.ProcessIncomingMessage(msgData, msg.sender)
			require.NoError(t, err, "Message %d should process successfully", i+1)

			time.Sleep(50 * time.Millisecond)

			// Verify expected message counts
			assert.Len(t, instructor.GetDeliveredMessages(), msg.expected["instructor"],
				"Instructor message count after message %d", i+1)
			assert.Len(t, students[0].GetDeliveredMessages(), msg.expected["student1"],
				"Student1 message count after message %d", i+1)
			assert.Len(t, students[1].GetDeliveredMessages(), msg.expected["student2"],
				"Student2 message count after message %d", i+1)
			assert.Len(t, students[2].GetDeliveredMessages(), msg.expected["student3"],
				"Student3 message count after message %d", i+1)
		}
	})
}

// setupRoleFilteringEnvironment creates environment for role filtering tests
func setupRoleFilteringEnvironment(t *testing.T) (*database.SQLiteDatabaseManager, session.SessionManager, *websocket.ConnectionRegistry, *websocket.BroadcastSystem, *message.MessageProcessor) {
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

// FilteringTrackingConnection for role filtering tests
type FilteringTrackingConnection struct {
	userID            string
	role              string
	deliveredMessages [][]byte
	mu                sync.RWMutex
	closed            bool
}

func (ftc *FilteringTrackingConnection) GetUserID() string {
	ftc.mu.RLock()
	defer ftc.mu.RUnlock()
	return ftc.userID
}

func (ftc *FilteringTrackingConnection) GetRole() string {
	ftc.mu.RLock()
	defer ftc.mu.RUnlock()
	return ftc.role
}

func (ftc *FilteringTrackingConnection) SendMessage(data []byte) error {
	ftc.mu.Lock()
	defer ftc.mu.Unlock()
	
	if ftc.closed {
		return fmt.Errorf("connection closed")
	}
	
	// Copy data to prevent external modifications
	msgCopy := make([]byte, len(data))
	copy(msgCopy, data)
	ftc.deliveredMessages = append(ftc.deliveredMessages, msgCopy)
	return nil
}

func (ftc *FilteringTrackingConnection) GetDeliveredMessages() [][]byte {
	ftc.mu.RLock()
	defer ftc.mu.RUnlock()
	
	// Return copy to prevent external modifications
	result := make([][]byte, len(ftc.deliveredMessages))
	for i, msg := range ftc.deliveredMessages {
		result[i] = make([]byte, len(msg))
		copy(result[i], msg)
	}
	return result
}

func (ftc *FilteringTrackingConnection) Close() error {
	ftc.mu.Lock()
	defer ftc.mu.Unlock()
	ftc.closed = true
	return nil
}

func (ftc *FilteringTrackingConnection) GetLastSeen() time.Time {
	return time.Now()
}

func (ftc *FilteringTrackingConnection) WriteJSON(v interface{}) error {
	return nil
}

func (ftc *FilteringTrackingConnection) SetCredentials(username, role string) error {
	return nil
}

func (ftc *FilteringTrackingConnection) UpdateActivity() {}

func (ftc *FilteringTrackingConnection) SendCloseMessage(reason string) error {
	return nil
}

// Helper functions for assertions

func assertMessageType(t *testing.T, messageData []byte, expectedType string) {
	var msgData map[string]interface{}
	err := json.Unmarshal(messageData, &msgData)
	require.NoError(t, err)
	
	msgType, ok := msgData["type"].(string)
	require.True(t, ok, "Message should have type field")
	assert.Equal(t, expectedType, msgType)
}

func getMessageTypes(messages [][]byte) []string {
	var types []string
	for _, msgData := range messages {
		var msg map[string]interface{}
		if json.Unmarshal(msgData, &msg) == nil {
			if msgType, ok := msg["type"].(string); ok {
				types = append(types, msgType)
			}
		}
	}
	return types
}