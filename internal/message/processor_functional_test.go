package message

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"switchboard/internal/database"
	"switchboard/internal/rate"
	"switchboard/pkg/errors"
)

// ========================================
// FUNCTIONAL VALIDATION TESTS (BLOCKING)
// ========================================

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
	m.activeSession = nil
	if session == nil {
		return nil, errors.ErrNoActiveSession
	}
	return session, nil
}

type mockDatabaseManager struct {
	mu       sync.Mutex
	messages []*database.Message
	writeErr error
}

func (m *mockDatabaseManager) WriteMessage(msg *database.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.writeErr != nil {
		return m.writeErr
	}
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockDatabaseManager) WriteBatch(msgs []*database.Message) error {
	for _, msg := range msgs {
		if err := m.WriteMessage(msg); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockDatabaseManager) CreateSession(session *database.Session) error { return nil }
func (m *mockDatabaseManager) UpdateSession(session *database.Session) error { return nil }
func (m *mockDatabaseManager) GetActiveSession() (*database.Session, error)  { return nil, nil }
func (m *mockDatabaseManager) GetSessionMessages(sessionID string) ([]*database.Message, error) {
	return nil, nil
}
func (m *mockDatabaseManager) Start() error { return nil }
func (m *mockDatabaseManager) Stop() error  { return nil }
func (m *mockDatabaseManager) WaitForPendingWrites() error { return nil }

func (m *mockDatabaseManager) GetMessages() []*database.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Return a copy to prevent external modifications
	result := make([]*database.Message, len(m.messages))
	copy(result, m.messages)
	return result
}

func (m *mockDatabaseManager) GetMessageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *mockDatabaseManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

func (m *mockDatabaseManager) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
	m.writeErr = nil
}

type processorMockConnectionProvider struct {
	users []Recipient
}

func (m *processorMockConnectionProvider) GetConnectedUsers() ([]Recipient, error) {
	return m.users, nil
}

type mockBroadcastSystem struct {
	mu           sync.Mutex
	broadcasts   []*database.Message
	broadcastErr error
}

func (m *mockBroadcastSystem) BroadcastMessage(message *database.Message, recipients []Recipient) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.broadcastErr != nil {
		return m.broadcastErr
	}
	m.broadcasts = append(m.broadcasts, message)
	return nil
}

func (m *mockBroadcastSystem) GetBroadcasts() []*database.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	result := make([]*database.Message, len(m.broadcasts))
	copy(result, m.broadcasts)
	return result
}

func (m *mockBroadcastSystem) GetBroadcastCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.broadcasts)
}

func (m *mockBroadcastSystem) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcasts = nil
	m.broadcastErr = nil
}

func TestMessageProcessor_SessionGating(t *testing.T) {
	// Setup
	sessionManager := &mockSessionManager{}
	dbManager := &mockDatabaseManager{}
	rateLimiter := rate.NewRateLimiter()
	defer rateLimiter.Stop()

	connectionProvider := &processorMockConnectionProvider{
		users: []Recipient{
			NewRecipient("instructor1", "instructor"),
		},
	}
	roleFilter := NewRoleBasedFilter()
	router := NewMessageRouter(connectionProvider, roleFilter)

	broadcastSystem := &mockBroadcastSystem{}
	processor := NewMessageProcessor(sessionManager, dbManager, rateLimiter, router, broadcastSystem)

	t.Run("reject_message_without_active_session", func(t *testing.T) {
		// No active session
		sessionManager.activeSession = nil

		message := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test message"},
		}

		messageData, _ := json.Marshal(message)

		err := processor.ProcessIncomingMessage(messageData, "student1")

		if err != errors.ErrNoActiveSession {
			t.Errorf("Expected ErrNoActiveSession, got: %v", err)
		}
	})

	t.Run("accept_message_with_active_session", func(t *testing.T) {
		// Set active session
		activeSession := &database.Session{
			ID:        "session123",
			Name:      "Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		sessionManager.activeSession = activeSession

		message := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test message"},
		}

		messageData, _ := json.Marshal(message)

		err := processor.ProcessIncomingMessage(messageData, "student1")

		if err != nil {
			t.Errorf("Expected no error with active session, got: %v", err)
		}
	})
}

func TestMessageProcessor_MessageValidation(t *testing.T) {
	// Setup with active session
	sessionManager := &mockSessionManager{
		activeSession: &database.Session{
			ID:        "session123",
			Name:      "Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		},
	}
	dbManager := &mockDatabaseManager{}
	rateLimiter := rate.NewRateLimiter()
	defer rateLimiter.Stop()

	connectionProvider := &processorMockConnectionProvider{
		users: []Recipient{
			NewRecipient("instructor1", "instructor"),
		},
	}
	roleFilter := NewRoleBasedFilter()
	router := NewMessageRouter(connectionProvider, roleFilter)

	broadcastSystem := &mockBroadcastSystem{}
	processor := NewMessageProcessor(sessionManager, dbManager, rateLimiter, router, broadcastSystem)

	t.Run("reject_invalid_json", func(t *testing.T) {
		invalidJSON := []byte(`{"type": "broadcast_to_instructors", "invalid_json"}`)

		err := processor.ProcessIncomingMessage(invalidJSON, "student1")

		if err == nil {
			t.Error("Expected error for invalid JSON")
		}

		if !contains(err.Error(), "invalid message format") {
			t.Errorf("Expected JSON parse error, got: %v", err)
		}
	})

	t.Run("reject_message_without_type", func(t *testing.T) {
		message := map[string]interface{}{
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test message"},
		}

		messageData, _ := json.Marshal(message)

		err := processor.ProcessIncomingMessage(messageData, "student1")

		if err == nil {
			t.Error("Expected error for message without type")
		}

		if !contains(err.Error(), "message validation failed") {
			t.Errorf("Expected validation error, got: %v", err)
		}
	})

	t.Run("reject_invalid_message_type", func(t *testing.T) {
		message := map[string]interface{}{
			"type":    "invalid_type",
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test message"},
		}

		messageData, _ := json.Marshal(message)

		err := processor.ProcessIncomingMessage(messageData, "student1")

		if err == nil {
			t.Error("Expected error for invalid message type")
		}

		if !contains(err.Error(), "message validation failed") {
			t.Errorf("Expected validation error, got: %v", err)
		}
	})

	t.Run("accept_valid_message", func(t *testing.T) {
		message := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Valid test message"},
		}

		messageData, _ := json.Marshal(message)

		err := processor.ProcessIncomingMessage(messageData, "student1")

		if err != nil {
			t.Errorf("Expected no error for valid message, got: %v", err)
		}
	})
}

func TestMessageProcessor_RateLimiting(t *testing.T) {
	// Setup with active session
	sessionManager := &mockSessionManager{
		activeSession: &database.Session{
			ID:        "session123",
			Name:      "Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		},
	}
	dbManager := &mockDatabaseManager{}
	rateLimiter := rate.NewRateLimiter()
	defer rateLimiter.Stop()

	connectionProvider := &processorMockConnectionProvider{
		users: []Recipient{
			NewRecipient("instructor1", "instructor"),
		},
	}
	roleFilter := NewRoleBasedFilter()
	router := NewMessageRouter(connectionProvider, roleFilter)

	broadcastSystem := &mockBroadcastSystem{}
	processor := NewMessageProcessor(sessionManager, dbManager, rateLimiter, router, broadcastSystem)

	userID := "student1"

	t.Run("allow_messages_within_rate_limit", func(t *testing.T) {
		// Reset rate limiter for this user
		rateLimiter.Reset(userID)

		message := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test message"},
		}

		messageData, _ := json.Marshal(message)

		// Send several messages within the limit
		for i := 0; i < 5; i++ {
			err := processor.ProcessIncomingMessage(messageData, userID)
			if err != nil {
				t.Errorf("Expected message %d to be allowed, got error: %v", i+1, err)
			}
		}
	})

	t.Run("reject_messages_over_rate_limit", func(t *testing.T) {
		// Reset and fill up the rate limit
		rateLimiter.Reset(userID)

		message := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test message"},
		}

		messageData, _ := json.Marshal(message)

		// Fill up to the limit (100 messages)
		for i := 0; i < 100; i++ {
			_ = processor.ProcessIncomingMessage(messageData, userID)
		}

		// Next message should be rate limited
		err := processor.ProcessIncomingMessage(messageData, userID)

		if err != errors.ErrRateLimitExceeded {
			t.Errorf("Expected ErrRateLimitExceeded, got: %v", err)
		}
	})
}

func TestMessageProcessor_AsynchronousPersistence(t *testing.T) {
	// Setup with active session
	sessionManager := &mockSessionManager{
		activeSession: &database.Session{
			ID:        "session123",
			Name:      "Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		},
	}
	dbManager := &mockDatabaseManager{}
	rateLimiter := rate.NewRateLimiter()
	defer rateLimiter.Stop()

	connectionProvider := &processorMockConnectionProvider{
		users: []Recipient{
			NewRecipient("instructor1", "instructor"),
		},
	}
	roleFilter := NewRoleBasedFilter()
	router := NewMessageRouter(connectionProvider, roleFilter)

	broadcastSystem := &mockBroadcastSystem{}
	processor := NewMessageProcessor(sessionManager, dbManager, rateLimiter, router, broadcastSystem)

	t.Run("message_persisted_asynchronously", func(t *testing.T) {
		message := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test message for persistence"},
		}

		messageData, _ := json.Marshal(message)

		// Process message
		err := processor.ProcessIncomingMessage(messageData, "student1")

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		// Give some time for async persistence
		time.Sleep(100 * time.Millisecond)

		// Verify message was persisted
		messages := dbManager.GetMessages()
		if len(messages) != 1 {
			t.Errorf("Expected 1 persisted message, got %d", len(messages))
		}

		if len(messages) > 0 {
			persistedMsg := messages[0]
			if persistedMsg.FromUser != "student1" {
				t.Errorf("Expected FromUser to be 'student1', got '%s'", persistedMsg.FromUser)
			}
			if persistedMsg.SessionID != "session123" {
				t.Errorf("Expected SessionID to be 'session123', got '%s'", persistedMsg.SessionID)
			}
		}
	})

	t.Run("processing_continues_despite_persistence_error", func(t *testing.T) {
		// Setup database to return error
		dbManager.writeErr = errors.ErrInvalidMessageData
		dbManager.Reset() // Reset messages

		message := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test message"},
		}

		messageData, _ := json.Marshal(message)

		// Process message - should succeed despite persistence error
		err := processor.ProcessIncomingMessage(messageData, "student1")

		if err != nil {
			t.Errorf("Expected no error despite persistence failure, got: %v", err)
		}

		// Give some time for async persistence attempt
		time.Sleep(100 * time.Millisecond)

		// Verify no message was persisted due to error
		messageCount := dbManager.GetMessageCount()
		if messageCount != 0 {
			t.Errorf("Expected 0 persisted messages due to error, got %d", messageCount)
		}

		// Reset error for future tests
		dbManager.writeErr = nil
	})
}

func TestMessageProcessor_MessageContextHandling(t *testing.T) {
	// Setup with active session
	sessionManager := &mockSessionManager{
		activeSession: &database.Session{
			ID:        "session123",
			Name:      "Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		},
	}
	dbManager := &mockDatabaseManager{}
	rateLimiter := rate.NewRateLimiter()
	defer rateLimiter.Stop()

	connectionProvider := &processorMockConnectionProvider{
		users: []Recipient{
			NewRecipient("instructor1", "instructor"),
		},
	}
	roleFilter := NewRoleBasedFilter()
	router := NewMessageRouter(connectionProvider, roleFilter)

	broadcastSystem := &mockBroadcastSystem{}
	processor := NewMessageProcessor(sessionManager, dbManager, rateLimiter, router, broadcastSystem)

	t.Run("default_context_set_when_empty", func(t *testing.T) {
		message := map[string]interface{}{
			"type": database.MessageTypeBroadcastToInstructors,
			// No context specified
			"content": map[string]interface{}{"text": "Test message"},
		}

		messageData, _ := json.Marshal(message)

		err := processor.ProcessIncomingMessage(messageData, "student1")

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		// Give time for async persistence
		time.Sleep(100 * time.Millisecond)

		// Verify default context was set
		messages := dbManager.GetMessages()
		if len(messages) > 0 {
			persistedMsg := messages[len(messages)-1]
			if persistedMsg.Context != database.ContextGeneral {
				t.Errorf("Expected context to be '%s', got '%s'", database.ContextGeneral, persistedMsg.Context)
			}
		}
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr) && s[1:len(substr)+1] == substr ||
			findInString(s, substr))))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
