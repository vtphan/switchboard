// Unit tests for MessageProcessor with BroadcastSystem integration
// These tests focus on the specific integration between MessageProcessor and BroadcastSystem

package unit

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/internal/message"
	"switchboard/internal/rate"
	switchboardErrors "switchboard/pkg/errors"
)

// TestMessageProcessorBroadcastIntegration tests the integration between MessageProcessor and BroadcastSystem
func TestMessageProcessorBroadcastIntegration(t *testing.T) {
	t.Run("successful_message_broadcast", func(t *testing.T) {
		// Setup test components
		sessionManager := &mockSessionManager{
			activeSession: &database.Session{
				ID:        "test-session",
				Name:      "Test Session",
				CreatedBy: "instructor1",
				StartTime: time.Now(),
				Status:    "active",
			},
		}

		dbManager := &mockDatabaseManager{}
		rateLimiter := rate.NewRateLimiter()
		defer rateLimiter.Stop()

		// Create mock recipients
		recipients := []message.Recipient{
			&mockRecipient{userID: "instructor1", role: "instructor"},
			&mockRecipient{userID: "student1", role: "student"},
		}

		mockRouter := &mockMessageRouter{recipients: recipients}
		trackingBroadcastSystem := &trackingBroadcastSystem{}

		processor := message.NewMessageProcessor(
			sessionManager,
			dbManager,
			rateLimiter,
			mockRouter,
			trackingBroadcastSystem,
		)

		// Test message processing
		testMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test question"},
		}

		messageData, _ := json.Marshal(testMessage)
		err := processor.ProcessIncomingMessage(messageData, "student1")

		// Verify processing succeeded
		require.NoError(t, err)

		// Verify message was broadcast
		broadcasts := trackingBroadcastSystem.GetBroadcasts()
		assert.Len(t, broadcasts, 1, "Message should be broadcast once")

		broadcast := broadcasts[0]
		assert.Equal(t, database.MessageTypeBroadcastToInstructors, broadcast.message.Type)
		assert.Equal(t, "student1", broadcast.message.FromUser)
		assert.Equal(t, "test-session", broadcast.message.SessionID)
		assert.Len(t, broadcast.recipients, 2, "Should broadcast to all recipients from router")

		// Verify message was persisted
		time.Sleep(50 * time.Millisecond) // Allow async persistence
		assert.Equal(t, 1, dbManager.GetMessageCount())
	})

	t.Run("broadcast_failure_doesnt_break_processing", func(t *testing.T) {
		// Setup with failing broadcast system
		sessionManager := &mockSessionManager{
			activeSession: &database.Session{
				ID:     "test-session",
				Status: "active",
			},
		}

		dbManager := &mockDatabaseManager{}
		rateLimiter := rate.NewRateLimiter()
		defer rateLimiter.Stop()

		recipients := []message.Recipient{
			&mockRecipient{userID: "instructor1", role: "instructor"},
		}

		mockRouter := &mockMessageRouter{recipients: recipients}
		failingBroadcastSystem := &failingBroadcastSystem{shouldFail: true}

		processor := message.NewMessageProcessor(
			sessionManager,
			dbManager,
			rateLimiter,
			mockRouter,
			failingBroadcastSystem,
		)

		// Process message with failing broadcast
		testMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test message"},
		}

		messageData, _ := json.Marshal(testMessage)
		err := processor.ProcessIncomingMessage(messageData, "student1")

		// Processing should succeed even with broadcast failure
		require.NoError(t, err, "Message processing should continue despite broadcast failure")

		// Verify message was still persisted
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, 1, dbManager.GetMessageCount(), "Message should be persisted despite broadcast failure")
	})

	t.Run("broadcast_called_with_correct_parameters", func(t *testing.T) {
		// Test that broadcast system receives correct message and recipients
		sessionManager := &mockSessionManager{
			activeSession: &database.Session{
				ID:        "session-123",
				Name:      "Parameter Test",
				CreatedBy: "instructor1",
				StartTime: time.Now(),
				Status:    "active",
			},
		}

		dbManager := &mockDatabaseManager{}
		rateLimiter := rate.NewRateLimiter()
		defer rateLimiter.Stop()

		// Create specific recipients for testing
		expectedRecipients := []message.Recipient{
			&mockRecipient{userID: "instructor1", role: "instructor"},
			&mockRecipient{userID: "instructor2", role: "instructor"},
		}

		mockRouter := &mockMessageRouter{recipients: expectedRecipients}
		trackingBroadcastSystem := &trackingBroadcastSystem{}

		processor := message.NewMessageProcessor(
			sessionManager,
			dbManager,
			rateLimiter,
			mockRouter,
			trackingBroadcastSystem,
		)

		// Process direct message to test parameter passing
		directMessage := map[string]interface{}{
			"type":    database.MessageTypeDirectMessage,
			"context": database.ContextResponse,
			"content": map[string]interface{}{"text": "Direct response"},
			"to_user": "student1",
		}

		messageData, _ := json.Marshal(directMessage)
		err := processor.ProcessIncomingMessage(messageData, "instructor1")
		require.NoError(t, err)

		// Verify broadcast parameters
		broadcasts := trackingBroadcastSystem.GetBroadcasts()
		require.Len(t, broadcasts, 1)

		broadcast := broadcasts[0]
		
		// Verify message parameters
		assert.Equal(t, database.MessageTypeDirectMessage, broadcast.message.Type)
		assert.Equal(t, database.ContextResponse, broadcast.message.Context)
		assert.Equal(t, "instructor1", broadcast.message.FromUser)
		assert.Equal(t, "session-123", broadcast.message.SessionID)
		assert.NotEmpty(t, broadcast.message.ID, "Message should have generated ID")
		assert.NotNil(t, broadcast.message.ToUser)
		assert.Equal(t, "student1", *broadcast.message.ToUser)

		// Verify recipients match router output
		assert.Equal(t, expectedRecipients, broadcast.recipients)
	})

	t.Run("no_broadcast_without_active_session", func(t *testing.T) {
		// Test that no broadcast occurs without active session
		sessionManager := &mockSessionManager{activeSession: nil} // No active session

		dbManager := &mockDatabaseManager{}
		rateLimiter := rate.NewRateLimiter()
		defer rateLimiter.Stop()

		mockRouter := &mockMessageRouter{}
		trackingBroadcastSystem := &trackingBroadcastSystem{}

		processor := message.NewMessageProcessor(
			sessionManager,
			dbManager,
			rateLimiter,
			mockRouter,
			trackingBroadcastSystem,
		)

		// Attempt to process message without session
		testMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Should not be broadcast"},
		}

		messageData, _ := json.Marshal(testMessage)
		err := processor.ProcessIncomingMessage(messageData, "student1")

		// Should fail with no active session error
		assert.Equal(t, switchboardErrors.ErrNoActiveSession, err)

		// Verify no broadcast occurred
		broadcasts := trackingBroadcastSystem.GetBroadcasts()
		assert.Len(t, broadcasts, 0, "No broadcast should occur without active session")

		// Verify no message was persisted
		assert.Equal(t, 0, dbManager.GetMessageCount())
	})

	t.Run("broadcast_called_after_routing", func(t *testing.T) {
		// Test that broadcast is called after message routing
		sessionManager := &mockSessionManager{
			activeSession: &database.Session{
				ID:     "routing-test",
				Status: "active",
			},
		}

		dbManager := &mockDatabaseManager{}
		rateLimiter := rate.NewRateLimiter()
		defer rateLimiter.Stop()

		// Create router that tracks when it's called
		trackingRouter := &trackingMessageRouter{
			recipients: []message.Recipient{
				&mockRecipient{userID: "instructor1", role: "instructor"},
			},
		}

		trackingBroadcastSystem := &trackingBroadcastSystem{}

		processor := message.NewMessageProcessor(
			sessionManager,
			dbManager,
			rateLimiter,
			trackingRouter,
			trackingBroadcastSystem,
		)

		// Process message
		testMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Routing test"},
		}

		messageData, _ := json.Marshal(testMessage)
		err := processor.ProcessIncomingMessage(messageData, "student1")
		require.NoError(t, err)

		// Verify router was called before broadcast
		assert.True(t, trackingRouter.WasCalled(), "Router should be called")
		assert.Len(t, trackingBroadcastSystem.GetBroadcasts(), 1, "Broadcast should occur after routing")

		// Verify broadcast received routed recipients
		broadcast := trackingBroadcastSystem.GetBroadcasts()[0]
		assert.Equal(t, trackingRouter.recipients, broadcast.recipients)
	})
}

// Mock implementations for testing

type mockSessionManager struct {
	activeSession *database.Session
}

func (m *mockSessionManager) GetActiveSession() *database.Session {
	return m.activeSession
}

func (m *mockSessionManager) SetActiveSession(session *database.Session) error {
	m.activeSession = session
	return nil
}

func (m *mockSessionManager) ClearActiveSession() error {
	m.activeSession = nil
	return nil
}

func (m *mockSessionManager) HasActiveSession() bool {
	return m.activeSession != nil
}

func (m *mockSessionManager) CheckAndClearActiveSession() (*database.Session, error) {
	session := m.activeSession
	if session == nil {
		return nil, switchboardErrors.ErrNoActiveSession
	}
	m.activeSession = nil
	return session, nil
}

type mockDatabaseManager struct {
	mu       sync.Mutex
	messages []*database.Message
}

func (m *mockDatabaseManager) WriteMessage(msg *database.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockDatabaseManager) GetMessageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *mockDatabaseManager) WriteBatch(msgs []*database.Message) error { return nil }
func (m *mockDatabaseManager) CreateSession(session *database.Session) error { return nil }
func (m *mockDatabaseManager) UpdateSession(session *database.Session) error { return nil }
func (m *mockDatabaseManager) GetActiveSession() (*database.Session, error) { return nil, nil }
func (m *mockDatabaseManager) GetSessionMessages(sessionID string) ([]*database.Message, error) { return nil, nil }
func (m *mockDatabaseManager) Start() error { return nil }
func (m *mockDatabaseManager) Stop() error { return nil }
func (m *mockDatabaseManager) WaitForPendingWrites() error { return nil }

type mockRecipient struct {
	userID string
	role   string
}

func (m *mockRecipient) GetUserID() string { return m.userID }
func (m *mockRecipient) GetRole() string { return m.role }
func (m *mockRecipient) SendMessage(data []byte) error { return nil }

type mockMessageRouter struct {
	recipients []message.Recipient
}

func (m *mockMessageRouter) GetRecipients(msg *database.Message) ([]message.Recipient, error) {
	return m.recipients, nil
}

type trackingMessageRouter struct {
	recipients []message.Recipient
	called     bool
	mu         sync.Mutex
}

func (t *trackingMessageRouter) GetRecipients(msg *database.Message) ([]message.Recipient, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.called = true
	return t.recipients, nil
}

func (t *trackingMessageRouter) WasCalled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.called
}

type broadcastCall struct {
	message    *database.Message
	recipients []message.Recipient
}

type trackingBroadcastSystem struct {
	mu         sync.Mutex
	broadcasts []broadcastCall
}

func (t *trackingBroadcastSystem) BroadcastMessage(message *database.Message, recipients []message.Recipient) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Deep copy message to prevent external modifications
	msgCopy := *message
	if message.ToUser != nil {
		toUserCopy := *message.ToUser
		msgCopy.ToUser = &toUserCopy
	}

	t.broadcasts = append(t.broadcasts, broadcastCall{
		message:    &msgCopy,
		recipients: recipients,
	})
	return nil
}

func (t *trackingBroadcastSystem) GetBroadcasts() []broadcastCall {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Return copy to prevent external modifications
	result := make([]broadcastCall, len(t.broadcasts))
	copy(result, t.broadcasts)
	return result
}

type failingBroadcastSystem struct {
	shouldFail bool
}

func (f *failingBroadcastSystem) BroadcastMessage(message *database.Message, recipients []message.Recipient) error {
	if f.shouldFail {
		return errors.New("broadcast system failure")
	}
	return nil
}