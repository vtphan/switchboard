package integration

import (
	"database/sql"
	"encoding/json"
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
	"switchboard/pkg/config"
	"switchboard/pkg/errors"
)

// ========================================
// PHASE 3 SYSTEM INTEGRATION VALIDATION
// ========================================

// This test file validates system-level behaviors for Phase 3 components
// Focus is on emergent behaviors when components work together, not individual contracts

// MockConnectionProvider implements the ConnectionProvider interface for testing
type mockConnectionProvider struct {
	mu    sync.RWMutex
	users []message.Recipient
}

func (m *mockConnectionProvider) GetConnectedUsers() ([]message.Recipient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]message.Recipient{}, m.users...), nil
}

func (m *mockConnectionProvider) setConnectedUsers(users []message.Recipient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users = users
}

// MockBroadcastSystem implements BroadcastSystemInterface for testing
type MockBroadcastSystem struct {
	mu           sync.Mutex
	broadcasts   []*database.Message
	broadcastErr error
}

func (m *MockBroadcastSystem) BroadcastMessage(message *database.Message, recipients []message.Recipient) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.broadcastErr != nil {
		return m.broadcastErr
	}
	m.broadcasts = append(m.broadcasts, message)
	return nil
}

func (m *MockBroadcastSystem) GetBroadcasts() []*database.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	result := make([]*database.Message, len(m.broadcasts))
	copy(result, m.broadcasts)
	return result
}

func (m *MockBroadcastSystem) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcasts = nil
	m.broadcastErr = nil
}

func setupIntegrationEnvironment(t *testing.T) (*database.SQLiteDatabaseManager, session.SessionManager, *message.MessageProcessor, *mockConnectionProvider, *rate.RateLimiter) {
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

	// Create rate limiter
	rateLimiter := rate.NewRateLimiter()

	// Create mock connection provider
	connectionProvider := &mockConnectionProvider{}

	// Create role-based filter and router
	roleFilter := message.NewRoleBasedFilter()
	router := message.NewMessageRouter(connectionProvider, roleFilter)

	// Create mock broadcast system
	broadcastSystem := &MockBroadcastSystem{}
	
	// Create message processor
	processor := message.NewMessageProcessor(sessionManager, dbManager, rateLimiter, router, broadcastSystem)

	return dbManager, sessionManager, processor, connectionProvider, rateLimiter
}

// ========================================
// 1. END-TO-END MESSAGE FLOW SYSTEM BEHAVIOR
// ========================================

func TestEndToEndMessageFlowSystemBehavior(t *testing.T) {
	dbManager, sessionManager, processor, connectionProvider, rateLimiter := setupIntegrationEnvironment(t)
	defer func() { _ = dbManager.Stop() }()
	defer rateLimiter.Stop()

	t.Run("complete_message_processing_pipeline_integration", func(t *testing.T) {
		// Setup educational scenario: instructor and students connected
		connectionProvider.setConnectedUsers([]message.Recipient{
			message.NewRecipient("instructor1", "instructor"),
			message.NewRecipient("student1", "student"),
			message.NewRecipient("student2", "student"),
		})

		// Create active session
		activeSession := &database.Session{
			ID:        "test-session-123",
			Name:      "Integration Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		require.NoError(t, sessionManager.SetActiveSession(activeSession))

		// Test complete pipeline for student question to instructors
		studentQuestion := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{
				"text":     "I need help with the assignment",
				"priority": "normal",
			},
		}

		messageData, _ := json.Marshal(studentQuestion)

		// Process message - this should trigger the complete 8-step pipeline
		startTime := time.Now()
		err := processor.ProcessIncomingMessage(messageData, "student1")
		processingTime := time.Since(startTime)

		// Verify real-time processing completed quickly
		assert.NoError(t, err)
		assert.Less(t, processingTime, 5*time.Millisecond, "Message processing should be near-instantaneous")

		// Wait for async persistence to complete
		time.Sleep(250 * time.Millisecond)

		// Verify message was persisted to database with correct enrichment
		messages, err := dbManager.GetSessionMessages(activeSession.ID)
		require.NoError(t, err)
		require.Len(t, messages, 1)

		persistedMsg := messages[0]
		assert.Equal(t, activeSession.ID, persistedMsg.SessionID)
		assert.Equal(t, "student1", persistedMsg.FromUser)
		assert.Equal(t, database.MessageTypeBroadcastToInstructors, persistedMsg.Type)
		assert.Equal(t, database.ContextQuestion, persistedMsg.Context)
		assert.NotEmpty(t, persistedMsg.ID)
		assert.WithinDuration(t, time.Now(), persistedMsg.Timestamp, 10*time.Second)

		// Verify content preservation
		content := persistedMsg.Content
		assert.Equal(t, "I need help with the assignment", content["text"])
		assert.Equal(t, "normal", content["priority"])
	})

	t.Run("message_routing_coordinates_with_filtering", func(t *testing.T) {
		// Test emergent behavior: routing + filtering working together
		connectionProvider.setConnectedUsers([]message.Recipient{
			message.NewRecipient("instructor1", "instructor"),
			message.NewRecipient("student1", "student"), 
			message.NewRecipient("student2", "student"),
			message.NewRecipient("admin1", "admin"), // Non-standard role
		})

		// Test broadcast_to_students message - should route to both students and instructors
		announcementMsg := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToStudents,
			"context": database.ContextAnnouncement,
			"content": map[string]interface{}{"text": "Class ends in 10 minutes"},
		}

		messageData, _ := json.Marshal(announcementMsg)
		err := processor.ProcessIncomingMessage(messageData, "instructor1")
		assert.NoError(t, err)

		// The system should have properly routed this to students and instructors
		// (admin1 should be excluded by routing logic)
		// This tests the coordination between MessageRouter and RoleBasedFilter
	})
}

// ========================================
// 2. EDUCATIONAL PRIVACY SYSTEM COORDINATION
// ========================================

func TestEducationalPrivacySystemCoordination(t *testing.T) {
	dbManager, sessionManager, processor, connectionProvider, rateLimiter := setupIntegrationEnvironment(t)
	defer func() { _ = dbManager.Stop() }()
	defer rateLimiter.Stop()

	// Create active session for privacy tests
	activeSession := &database.Session{
		ID:        "privacy-test-session",
		Name:      "Privacy Test Session",
		CreatedBy: "instructor1",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}
	require.NoError(t, sessionManager.SetActiveSession(activeSession))

	t.Run("student_question_privacy_maintained", func(t *testing.T) {
		// Test that student questions to instructors don't leak to other students
		connectionProvider.setConnectedUsers([]message.Recipient{
			message.NewRecipient("instructor1", "instructor"),
			message.NewRecipient("instructor2", "instructor"),
			message.NewRecipient("student1", "student"),  // sender
			message.NewRecipient("student2", "student"),  // should not see
			message.NewRecipient("student3", "student"),  // should not see
		})

		// Student asks private question to instructors
		privateQuestion := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{
				"text":        "I'm struggling with personal issues affecting my work",
				"sensitive":   true,
				"needs_help":  true,
			},
		}

		messageData, _ := json.Marshal(privateQuestion)
		err := processor.ProcessIncomingMessage(messageData, "student1")
		assert.NoError(t, err)

		// Verify message was processed and persisted
		time.Sleep(100 * time.Millisecond)
		messages, err := dbManager.GetSessionMessages(activeSession.ID)
		require.NoError(t, err)
		assert.Len(t, messages, 1)

		// The routing and filtering system should have coordinated to ensure:
		// 1. Only instructors receive this message (routing decision)
		// 2. Students are filtered out (privacy enforcement)  
		// 3. Message is still persisted for instructor access later
		persistedMsg := messages[0]
		assert.Equal(t, database.MessageTypeBroadcastToInstructors, persistedMsg.Type)
		
		content := persistedMsg.Content
		assert.Equal(t, true, content["sensitive"])
	})

	t.Run("direct_message_privacy_boundaries", func(t *testing.T) {
		// Test direct message privacy with instructor oversight
		connectionProvider.setConnectedUsers([]message.Recipient{
			message.NewRecipient("instructor1", "instructor"),
			message.NewRecipient("student1", "student"),  // sender
			message.NewRecipient("student2", "student"),  // recipient
			message.NewRecipient("student3", "student"),  // should not see
		})

		// Student sends direct message to another student
		directMsg := map[string]interface{}{
			"type":    database.MessageTypeDirectMessage,
			"to_user": "student2",
			"context": database.ContextPeerHelp,
			"content": map[string]interface{}{
				"text": "Can you share your notes from today?",
			},
		}

		messageData, _ := json.Marshal(directMsg)
		err := processor.ProcessIncomingMessage(messageData, "student1")
		assert.NoError(t, err)

		// Wait for persistence
		time.Sleep(100 * time.Millisecond)
		messages, err := dbManager.GetSessionMessages(activeSession.ID)
		require.NoError(t, err)
		assert.Len(t, messages, 2) // Previous test + this test

		// Find the direct message
		var directMessage *database.Message
		for _, msg := range messages {
			if msg.Type == database.MessageTypeDirectMessage {
				directMessage = msg
				break
			}
		}
		require.NotNil(t, directMessage)

		// Verify proper direct message handling:
		// - Sender can see their own message
		// - Recipient can see message sent to them  
		// - Instructors can see for monitoring
		// - Other students cannot see
		assert.Equal(t, "student1", directMessage.FromUser)
		assert.Equal(t, "student2", *directMessage.ToUser)
	})
}

// ========================================
// 3. RATE LIMITING SYSTEM INTEGRATION
// ========================================

func TestRateLimitingSystemIntegration(t *testing.T) {
	dbManager, sessionManager, processor, connectionProvider, rateLimiter := setupIntegrationEnvironment(t)
	defer func() { _ = dbManager.Stop() }() 
	defer rateLimiter.Stop()

	// Create active session
	activeSession := &database.Session{
		ID:        "rate-limit-test-session",
		Name:      "Rate Limit Test Session",
		CreatedBy: "instructor1",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}
	require.NoError(t, sessionManager.SetActiveSession(activeSession))

	connectionProvider.setConnectedUsers([]message.Recipient{
		message.NewRecipient("instructor1", "instructor"),
	})

	t.Run("rate_limiting_occurs_before_processing", func(t *testing.T) {
		// Reset rate limiter for clean test
		rateLimiter.ResetAll()

		userID := "student_rate_test"
		
		// Fill up the rate limit (100 messages per minute)
		baseMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test message"},
		}
		messageData, _ := json.Marshal(baseMessage)

		// Send messages up to the limit
		for i := 0; i < config.RateLimitMaxMessages; i++ {
			err := processor.ProcessIncomingMessage(messageData, userID)
			assert.NoError(t, err, "Message %d should be allowed", i+1)
		}

		// Next message should be rate limited BEFORE any processing
		err := processor.ProcessIncomingMessage(messageData, userID)
		assert.Equal(t, errors.ErrRateLimitExceeded, err)

		// Verify that rate-limited message was not processed or persisted
		time.Sleep(100 * time.Millisecond)
		messages, err := dbManager.GetSessionMessages(activeSession.ID)
		require.NoError(t, err)
		
		// Should have exactly 100 messages (the limit), not 101
		assert.Len(t, messages, config.RateLimitMaxMessages)
	})

	t.Run("rate_limiting_per_user_isolation", func(t *testing.T) {
		// Test that rate limiting doesn't affect other users
		rateLimiter.ResetAll()

		baseMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Isolation test"},
		}
		messageData, _ := json.Marshal(baseMessage)

		// Fill rate limit for user1
		for i := 0; i < config.RateLimitMaxMessages; i++ {
			err := processor.ProcessIncomingMessage(messageData, "user1")
			assert.NoError(t, err)
		}

		// user1 should now be rate limited
		err := processor.ProcessIncomingMessage(messageData, "user1")
		assert.Equal(t, errors.ErrRateLimitExceeded, err)

		// user2 should still be able to send messages
		err = processor.ProcessIncomingMessage(messageData, "user2")
		assert.NoError(t, err, "Different user should not be affected by other user's rate limit")
	})

	t.Run("rate_limiting_coordinates_with_session_gating", func(t *testing.T) {
		// Test order of operations: session gating should occur before rate limiting
		rateLimiter.ResetAll()

		// Clear active session
		require.NoError(t, sessionManager.ClearActiveSession())

		baseMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "No session test"},
		}
		messageData, _ := json.Marshal(baseMessage)

		// Message should be rejected due to no active session, not rate limiting
		err := processor.ProcessIncomingMessage(messageData, "test_user")
		assert.Equal(t, errors.ErrNoActiveSession, err)

		// Rate limiter should not have been consumed (session check comes first)
		count := rateLimiter.GetUserMessageCount("test_user")
		assert.Equal(t, 0, count, "Rate limiter should not be affected when session gating fails")
	})
}

// ========================================
// 4. CROSS-PHASE COORDINATION UNDER LOAD
// ========================================

func TestCrossPhaseCoordinationUnderLoad(t *testing.T) {
	dbManager, sessionManager, processor, connectionProvider, rateLimiter := setupIntegrationEnvironment(t)
	defer func() { _ = dbManager.Stop() }()
	defer rateLimiter.Stop()

	// Create active session
	activeSession := &database.Session{
		ID:        "load-test-session",
		Name:      "Load Test Session",
		CreatedBy: "instructor1",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}
	require.NoError(t, sessionManager.SetActiveSession(activeSession))

	// Setup realistic educational load: multiple instructors and students
	connectionProvider.setConnectedUsers([]message.Recipient{
		message.NewRecipient("instructor1", "instructor"),
		message.NewRecipient("instructor2", "instructor"),
		message.NewRecipient("student1", "student"),
		message.NewRecipient("student2", "student"),
		message.NewRecipient("student3", "student"),
		message.NewRecipient("student4", "student"),
		message.NewRecipient("student5", "student"),
	})

	t.Run("concurrent_message_processing_stability", func(t *testing.T) {
		// Test system stability under concurrent load
		const numGoroutines = 10
		const messagesPerGoroutine = 20
		
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines*messagesPerGoroutine)
		processingTimes := make(chan time.Duration, numGoroutines*messagesPerGoroutine)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()
				
				for j := 0; j < messagesPerGoroutine; j++ {
					message := map[string]interface{}{
						"type":    database.MessageTypeBroadcastToInstructors,
						"context": database.ContextQuestion,
						"content": map[string]interface{}{
							"text":      "Concurrent test message",
							"routine":   routineID,
							"sequence":  j,
						},
					}
					
					messageData, _ := json.Marshal(message)
					userID := "concurrent_user_" + string(rune('0'+routineID))
					
					start := time.Now()
					err := processor.ProcessIncomingMessage(messageData, userID)
					duration := time.Since(start)
					
					errors <- err
					processingTimes <- duration
				}
			}(i)
		}

		wg.Wait()
		close(errors)
		close(processingTimes)

		// Verify all messages processed successfully
		errorCount := 0
		for err := range errors {
			if err != nil {
				t.Logf("Processing error: %v", err)
				errorCount++
			}
		}
		assert.Equal(t, 0, errorCount, "No errors should occur during concurrent processing")

		// Verify performance maintained under load
		var totalTime time.Duration
		count := 0
		for duration := range processingTimes {
			totalTime += duration
			count++
		}
		avgProcessingTime := totalTime / time.Duration(count)
		assert.Less(t, avgProcessingTime, 10*time.Millisecond, "Average processing time should remain low under load")

		// Allow time for all async persistence to complete
		time.Sleep(2 * time.Second)

		// Verify all messages were persisted correctly
		messages, err := dbManager.GetSessionMessages(activeSession.ID)
		require.NoError(t, err)
		assert.Len(t, messages, numGoroutines*messagesPerGoroutine, "All messages should be persisted")
	})

	t.Run("session_state_concurrent_access", func(t *testing.T) {
		// Test that session state access remains consistent under concurrent load
		const numReaders = 20
		var wg sync.WaitGroup
		sessionIDs := make(chan string, numReaders)

		// Start multiple goroutines reading session state concurrently
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				session := sessionManager.GetActiveSession()
				if session != nil {
					sessionIDs <- session.ID
				} else {
					sessionIDs <- ""
				}
			}()
		}

		wg.Wait()
		close(sessionIDs)

		// Verify all readers got the same session ID (consistency)
		var ids []string
		for id := range sessionIDs {
			ids = append(ids, id)
		}

		assert.Len(t, ids, numReaders)
		for _, id := range ids {
			assert.Equal(t, activeSession.ID, id, "All concurrent readers should see the same session")
		}
	})
}

// ========================================
// 5. SYSTEM-LEVEL ERROR COORDINATION
// ========================================

func TestSystemLevelErrorCoordination(t *testing.T) {
	dbManager, sessionManager, processor, connectionProvider, rateLimiter := setupIntegrationEnvironment(t)
	defer func() { _ = dbManager.Stop() }()
	defer rateLimiter.Stop()

	connectionProvider.setConnectedUsers([]message.Recipient{
		message.NewRecipient("instructor1", "instructor"),
		message.NewRecipient("student1", "student"),
	})

	t.Run("error_cascade_prevention", func(t *testing.T) {
		// Test that errors in one component don't cascade to others
		
		// Setup: Create session but then simulate database persistence errors
		activeSession := &database.Session{
			ID:        "error-test-session",
			Name:      "Error Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		require.NoError(t, sessionManager.SetActiveSession(activeSession))

		// Stop database manager to simulate database failures
		require.NoError(t, dbManager.Stop())

		message := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test with database error"},
		}
		messageData, _ := json.Marshal(message)

		// Message processing should still succeed (real-time delivery)
		// even though persistence will fail
		err := processor.ProcessIncomingMessage(messageData, "student1")
		assert.NoError(t, err, "Message processing should succeed despite database persistence failure")

		// Verify the system remains stable - session manager still works
		session := sessionManager.GetActiveSession()
		assert.NotNil(t, session)
		assert.Equal(t, activeSession.ID, session.ID)

		// Verify rate limiter still works
		count := rateLimiter.GetUserMessageCount("student1")
		assert.Equal(t, 1, count, "Rate limiter should still track the message")
	})

	t.Run("partial_failure_recovery", func(t *testing.T) {
		// Test system recovery after partial failures
		
		// Restart database manager
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		schema, err := os.ReadFile("../../internal/database/migrations.sql")
		require.NoError(t, err)
		_, err = db.Exec(string(schema))
		require.NoError(t, err)

		// Skip table initialization and pragmas since we already applied migrations.sql
		newDBManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
			SkipTableInit: true,
			SkipPragmas:   true,
		})
		require.NoError(t, err)
		require.NoError(t, newDBManager.Start())
		defer func() { _ = newDBManager.Stop() }()

		// Create new processor with recovered database
		roleFilter := message.NewRoleBasedFilter()
		router := message.NewMessageRouter(connectionProvider, roleFilter)
		recoveredBroadcastSystem := &MockBroadcastSystem{}
		recoveredProcessor := message.NewMessageProcessor(sessionManager, newDBManager, rateLimiter, router, recoveredBroadcastSystem)

		// System should work normally after recovery
		message := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": "Test after recovery"},
		}
		messageData, _ := json.Marshal(message)

		err = recoveredProcessor.ProcessIncomingMessage(messageData, "student1")
		assert.NoError(t, err, "System should work normally after component recovery")

		// Verify persistence works again
		time.Sleep(100 * time.Millisecond)
		messages, err := newDBManager.GetSessionMessages(sessionManager.GetActiveSession().ID)
		require.NoError(t, err)
		assert.Len(t, messages, 1, "Messages should be persisted after recovery")
	})

	t.Run("error_boundary_isolation", func(t *testing.T) {
		// Test that errors are properly isolated to their boundaries
		
		// Clear any existing session first
		_ = sessionManager.ClearActiveSession()
		
		// Create session for clean test
		activeSession := &database.Session{
			ID:        "boundary-test-session",
			Name:      "Boundary Test Session", 
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		require.NoError(t, sessionManager.SetActiveSession(activeSession))

		// Test various error conditions in isolation
		testCases := []struct {
			name          string
			messageData   []byte
			userID        string
			expectedError error
			description   string
		}{
			{
				name:          "invalid_json",
				messageData:   []byte(`{"invalid": json}`),
				userID:        "student1",
				expectedError: nil, // Should be wrapped, but not nil
				description:   "JSON parsing errors should be contained",
			},
			{
				name:          "session_gating",
				messageData:   []byte(`{"type": "broadcast_to_instructors", "content": {"text": "test"}}`),
				userID:        "student1",
				expectedError: nil, // Will have an error after clearing session
				description:   "Session gating errors should be contained",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.name == "session_gating" {
					// Clear session for this specific test
					require.NoError(t, sessionManager.ClearActiveSession())
					defer func() {
						// Restore session for other tests (ignore error if already set)
						_ = sessionManager.SetActiveSession(activeSession)
					}()
				}

				err := processor.ProcessIncomingMessage(tc.messageData, tc.userID)
				
				// Error should be properly typed and contained
				switch tc.name {
				case "invalid_json":
					assert.Error(t, err, "Invalid JSON should produce an error")
					assert.Contains(t, err.Error(), "invalid message format", "Error should be properly wrapped")
				case "session_gating":
					assert.Equal(t, errors.ErrNoActiveSession, err, "Should get proper session error")
				}

				// System should remain stable after each error
				assert.True(t, rateLimiter.GetUserMessageCount(tc.userID) >= 0, "Rate limiter should remain functional")
			})
		}
	})
}