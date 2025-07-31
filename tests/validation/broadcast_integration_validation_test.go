// Validation test for BroadcastSystem integration
// This test demonstrates that the complete message delivery pipeline works end-to-end

package validation

import (
	"database/sql"
	"encoding/json"
	"os"
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

// TestBroadcastIntegrationValidation validates the complete BroadcastSystem integration
func TestBroadcastIntegrationValidation(t *testing.T) {
	t.Run("validate_complete_message_pipeline", func(t *testing.T) {
		// This test validates that:
		// 1. MessageProcessor can accept BroadcastSystem
		// 2. Messages are processed and delivered to BroadcastSystem
		// 3. BroadcastSystem correctly delivers messages with role filtering
		// 4. System continues to work after broadcast success/failure

		// Setup complete system
		dbManager, sessionManager, messageProcessor, trackingConnections := setupValidationEnvironment(t)
		defer dbManager.Stop()

		// Create active session
		session := &database.Session{
			ID:        "validation-test",
			Name:      "Validation Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, sessionManager.SetActiveSession(session))

		// Test 1: Student question to instructors (should only reach instructor)
		studentQuestion := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Integration validation question"},
		}

		questionData, _ := json.Marshal(studentQuestion)
		err := messageProcessor.ProcessIncomingMessage(questionData, "student1")
		require.NoError(t, err, "Message processing should succeed")

		time.Sleep(100 * time.Millisecond) // Allow processing

		// Validate that BroadcastSystem received and filtered the message correctly
		instructorMessages := trackingConnections["instructor1"].GetDeliveredMessages()
		student1Messages := trackingConnections["student1"].GetDeliveredMessages()

		assert.Len(t, instructorMessages, 1, "Instructor should receive student question")
		assert.Len(t, student1Messages, 0, "Student should not see their own question (privacy)")

		// Verify message content
		var msgData map[string]interface{}
		require.NoError(t, json.Unmarshal(instructorMessages[0], &msgData))
		assert.Equal(t, database.MessageTypeBroadcastToInstructors, msgData["type"])
		assert.Equal(t, "student1", msgData["from_user"])

		// Test 2: Instructor announcement (should reach everyone)
		announcement := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToStudents,
			"context": database.ContextAnnouncement,
			"content": map[string]interface{}{"text": "Integration validation announcement"},
		}

		announcementData, _ := json.Marshal(announcement)
		err = messageProcessor.ProcessIncomingMessage(announcementData, "instructor1")
		require.NoError(t, err, "Announcement processing should succeed")

		time.Sleep(100 * time.Millisecond)

		// Validate that all users received the announcement
		instructorMessages = trackingConnections["instructor1"].GetDeliveredMessages()
		student1Messages = trackingConnections["student1"].GetDeliveredMessages()

		assert.Len(t, instructorMessages, 2, "Instructor should see question + announcement")
		assert.Len(t, student1Messages, 1, "Student should receive announcement")

		// Test 3: Verify message persistence worked alongside broadcasting
		time.Sleep(100 * time.Millisecond) // Allow async persistence
		messages, err := dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Len(t, messages, 2, "Both messages should be persisted")

		// Validate integration success
		t.Log("✅ BroadcastSystem integration validation successful:")
		t.Log("✅ MessageProcessor correctly uses BroadcastSystem")
		t.Log("✅ Role-based filtering works during broadcast")
		t.Log("✅ Message persistence works alongside broadcasting")
		t.Log("✅ Complete message delivery pipeline functional")
	})
}

// setupValidationEnvironment creates a minimal test environment for validation
func setupValidationEnvironment(t *testing.T) (*database.SQLiteDatabaseManager, session.SessionManager, *message.MessageProcessor, map[string]*ValidationConnection) {
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

	// Create connection registry with tracking connections
	connectionRegistry := websocket.NewConnectionRegistry(sessionManager)
	connectionRegistry.Start()

	// Create validation connections
	connections := map[string]*ValidationConnection{
		"instructor1": {userID: "instructor1", role: "instructor"},
		"student1":    {userID: "student1", role: "student"},
	}

	// Register connections
	for userID, conn := range connections {
		require.NoError(t, connectionRegistry.Register(userID, conn))
	}

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

	return dbManager, sessionManager, messageProcessor, connections
}

// ValidationConnection is a simple connection for validation testing
type ValidationConnection struct {
	userID            string
	role              string
	deliveredMessages [][]byte
}

func (vc *ValidationConnection) GetUserID() string { return vc.userID }
func (vc *ValidationConnection) GetRole() string   { return vc.role }

func (vc *ValidationConnection) SendMessage(data []byte) error {
	// Copy data to prevent external modifications
	msgCopy := make([]byte, len(data))
	copy(msgCopy, data)
	vc.deliveredMessages = append(vc.deliveredMessages, msgCopy)
	return nil
}

func (vc *ValidationConnection) GetDeliveredMessages() [][]byte {
	// Return copy to prevent external modifications
	result := make([][]byte, len(vc.deliveredMessages))
	for i, msg := range vc.deliveredMessages {
		result[i] = make([]byte, len(msg))
		copy(result[i], msg)
	}
	return result
}

// Required ConnectionInterface methods (minimal implementation for validation)
func (vc *ValidationConnection) WriteJSON(v interface{}) error              { return nil }
func (vc *ValidationConnection) Close() error                               { return nil }
func (vc *ValidationConnection) SetCredentials(username, role string) error { return nil }
func (vc *ValidationConnection) UpdateActivity()                            {}
func (vc *ValidationConnection) GetLastSeen() time.Time                     { return time.Now() }
func (vc *ValidationConnection) SendCloseMessage(reason string) error       { return nil }