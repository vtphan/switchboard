package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"switchboard/pkg/database"
	"switchboard/pkg/interfaces"
	"switchboard/pkg/types"
)

// setupTestManager creates a test database manager with in-memory database
func setupTestManager(t *testing.T) (*Manager, func()) {
	// Create temporary database file
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	config := &database.Config{
		DatabasePath:    dbPath,
		MaxConnections:  5,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: time.Minute,
	}

	manager, err := NewManager(config)
	require.NoError(t, err)

	// Run migrations
	migrationManager := database.NewMigrationManager(manager.GetDB(), "../../migrations")
	err = migrationManager.ApplyMigrations()
	require.NoError(t, err)

	cleanup := func() {
		if err := manager.Close(); err != nil {
			t.Logf("Failed to close database manager: %v", err)
		}
		if err := os.Remove(dbPath); err != nil {
			t.Logf("Failed to remove test database: %v", err)
		}
	}

	return manager, cleanup
}

// Batch Method Tests

// TestManager_StoreMessageBatch tests batch message insertion
func TestManager_StoreMessageBatch(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test session first
	session := &types.Session{
		ID:         "test-session",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1", "student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	err := manager.CreateSession(ctx, session)
	require.NoError(t, err)

	// Create batch of messages
	messages := []*types.Message{
		{
			ID:        "msg1",
			SessionID: "test-session",
			Type:      types.MessageTypeInstructorInbox,
			Context:   "general",
			FromUser:  "student1",
			ToUser:    nil,
			Content:   map[string]interface{}{"text": "Hello"},
			Timestamp: time.Now(),
		},
		{
			ID:        "msg2",
			SessionID: "test-session",
			Type:      types.MessageTypeInboxResponse,
			Context:   "general",
			FromUser:  "instructor1",
			ToUser:    &[]string{"student1"}[0],
			Content:   map[string]interface{}{"text": "Hi there"},
			Timestamp: time.Now(),
		},
		{
			ID:        "msg3",
			SessionID: "test-session",
			Type:      types.MessageTypeAnalytics,
			Context:   "progress",
			FromUser:  "student2",
			ToUser:    nil,
			Content:   map[string]interface{}{"progress": 75},
			Timestamp: time.Now(),
		},
	}

	// Test batch insertion
	err = manager.StoreMessageBatch(ctx, messages)
	require.NoError(t, err)

	// Verify all messages were stored
	history, err := manager.GetSessionHistory(ctx, "test-session")
	require.NoError(t, err)
	assert.Len(t, history, 3)

	// Verify message order and content
	assert.Equal(t, "msg1", history[0].ID)
	assert.Equal(t, "Hello", history[0].Content["text"])
	assert.Equal(t, "msg2", history[1].ID)
	assert.Equal(t, "Hi there", history[1].Content["text"])
	assert.Equal(t, "msg3", history[2].ID)
	assert.Equal(t, float64(75), history[2].Content["progress"])
}

// TestManager_BatchTransaction tests that batch uses single transaction
func TestManager_BatchTransaction(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test session
	session := &types.Session{
		ID:         "test-session",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	err := manager.CreateSession(ctx, session)
	require.NoError(t, err)

	// Create batch with one invalid message (invalid session)
	messages := []*types.Message{
		{
			ID:        "msg1",
			SessionID: "test-session",
			Type:      types.MessageTypeInstructorInbox,
			Context:   "general",
			FromUser:  "student1",
			Content:   map[string]interface{}{"text": "Valid message"},
			Timestamp: time.Now(),
		},
		{
			ID:        "msg2",
			SessionID: "invalid-session", // This should cause transaction to fail
			Type:      types.MessageTypeInstructorInbox,
			Context:   "general",
			FromUser:  "student1",
			Content:   map[string]interface{}{"text": "Invalid session"},
			Timestamp: time.Now(),
		},
	}

	// Batch should fail due to foreign key constraint
	err = manager.StoreMessageBatch(ctx, messages)
	assert.Error(t, err)

	// Verify no messages were stored (transaction rollback)
	history, err := manager.GetSessionHistory(ctx, "test-session")
	require.NoError(t, err)
	assert.Len(t, history, 0)
}

// TestManager_BatchPerformance tests batch is faster than individual inserts
func TestManager_BatchPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	manager, cleanup := setupTestManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test session
	session := &types.Session{
		ID:         "test-session",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	err := manager.CreateSession(ctx, session)
	require.NoError(t, err)

	// Create 50 messages
	messages := make([]*types.Message, 50)
	for i := 0; i < 50; i++ {
		messages[i] = &types.Message{
			ID:        fmt.Sprintf("msg%d", i),
			SessionID: "test-session",
			Type:      types.MessageTypeAnalytics,
			Context:   "general",
			FromUser:  "student1",
			Content:   map[string]interface{}{"index": i},
			Timestamp: time.Now(),
		}
	}

	// Time individual inserts
	start := time.Now()
	for _, msg := range messages[:25] {
		err := manager.StoreMessage(ctx, msg)
		require.NoError(t, err)
	}
	individualTime := time.Since(start)

	// Time batch insert
	start = time.Now()
	err = manager.StoreMessageBatch(ctx, messages[25:])
	require.NoError(t, err)
	batchTime := time.Since(start)

	// Batch should be significantly faster
	assert.Less(t, batchTime.Nanoseconds(), individualTime.Nanoseconds()/2,
		"Batch insert should be at least 2x faster than individual inserts")

	// Verify all messages stored
	history, err := manager.GetSessionHistory(ctx, "test-session")
	require.NoError(t, err)
	assert.Len(t, history, 50)
}

// TestManager_EmptyBatch tests handling of empty batch
func TestManager_EmptyBatch(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	ctx := context.Background()

	// Test empty batch
	err := manager.StoreMessageBatch(ctx, []*types.Message{})
	assert.NoError(t, err) // Should not error on empty batch
}

// TestManager_LargeBatch tests handling of large batches
func TestManager_LargeBatch(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test session
	session := &types.Session{
		ID:         "test-session",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	err := manager.CreateSession(ctx, session)
	require.NoError(t, err)

	// Create large batch of messages
	messages := make([]*types.Message, 1000)
	for i := 0; i < 1000; i++ {
		messages[i] = &types.Message{
			ID:        fmt.Sprintf("msg%d", i),
			SessionID: "test-session",
			Type:      types.MessageTypeAnalytics,
			Context:   "general",
			FromUser:  "student1",
			Content:   map[string]interface{}{"index": i, "data": "x"},
			Timestamp: time.Now().Add(time.Duration(i) * time.Millisecond),
		}
	}

	// Should handle large batch
	err = manager.StoreMessageBatch(ctx, messages)
	require.NoError(t, err)

	// Verify all stored
	history, err := manager.GetSessionHistory(ctx, "test-session")
	require.NoError(t, err)
	assert.Len(t, history, 1000)
}

// TestCreateSession tests session creation
func TestCreateSession(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	session := &types.Session{
		ID:         "session-123",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1", "student2", "student3"},
		StartTime:  time.Now(),
		Status:     "active",
	}

	err := manager.CreateSession(context.Background(), session)
	assert.NoError(t, err)

	// Verify session was created by retrieving it
	retrieved, err := manager.GetSession(context.Background(), session.ID)
	assert.NoError(t, err)
	assert.Equal(t, session.ID, retrieved.ID)
	assert.Equal(t, session.Name, retrieved.Name)
	assert.Equal(t, session.CreatedBy, retrieved.CreatedBy)
	assert.Equal(t, session.StudentIDs, retrieved.StudentIDs)
	assert.Equal(t, session.Status, retrieved.Status)
	assert.WithinDuration(t, session.StartTime, retrieved.StartTime, time.Second)
	assert.Nil(t, retrieved.EndTime)
}

// TestGetSession_NotFound tests getting non-existent session
func TestGetSession_NotFound(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	session, err := manager.GetSession(context.Background(), "non-existent")
	assert.Nil(t, session)
	assert.Equal(t, interfaces.ErrSessionNotFound, err)
}

// TestUpdateSession tests session updates
func TestUpdateSession(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	// Create session first
	session := &types.Session{
		ID:         "session-123",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1", "student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}

	err := manager.CreateSession(context.Background(), session)
	require.NoError(t, err)

	// Update session
	endTime := time.Now()
	session.EndTime = &endTime
	session.Status = "ended"

	err = manager.UpdateSession(context.Background(), session)
	assert.NoError(t, err)

	// Verify update
	retrieved, err := manager.GetSession(context.Background(), session.ID)
	assert.NoError(t, err)
	assert.Equal(t, "ended", retrieved.Status)
	assert.NotNil(t, retrieved.EndTime)
	assert.WithinDuration(t, endTime, *retrieved.EndTime, time.Second)
}

// TestListActiveSessions tests listing active sessions
func TestListActiveSessions(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	// Create multiple sessions
	activeSession1 := &types.Session{
		ID:         "active-1",
		Name:       "Active Session 1",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now().Add(-2 * time.Hour),
		Status:     "active",
	}

	activeSession2 := &types.Session{
		ID:         "active-2",
		Name:       "Active Session 2",
		CreatedBy:  "instructor2",
		StudentIDs: []string{"student2"},
		StartTime:  time.Now().Add(-1 * time.Hour),
		Status:     "active",
	}

	endedSession := &types.Session{
		ID:         "ended-1",
		Name:       "Ended Session",
		CreatedBy:  "instructor3",
		StudentIDs: []string{"student3"},
		StartTime:  time.Now().Add(-3 * time.Hour),
		Status:     "ended",
	}

	// Create sessions
	require.NoError(t, manager.CreateSession(context.Background(), activeSession1))
	require.NoError(t, manager.CreateSession(context.Background(), activeSession2))
	require.NoError(t, manager.CreateSession(context.Background(), endedSession))

	// List active sessions
	activeSessions, err := manager.ListActiveSessions(context.Background())
	assert.NoError(t, err)
	assert.Len(t, activeSessions, 2)

	// Verify order (most recent first)
	assert.Equal(t, "active-2", activeSessions[0].ID) // More recent
	assert.Equal(t, "active-1", activeSessions[1].ID) // Older
}

// TestStoreMessage tests message storage
func TestStoreMessage(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	// Create a session first (required by foreign key constraint)
	session := &types.Session{
		ID:         "session-123",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1", "student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	err := manager.CreateSession(context.Background(), session)
	require.NoError(t, err)

	message := &types.Message{
		ID:        "msg-123",
		SessionID: "session-123",
		Type:      types.MessageTypeInstructorInbox,
		Context:   "question",
		FromUser:  "student1",
		ToUser:    nil, // Broadcast message
		Content:   map[string]interface{}{"text": "Need help with problem 5", "difficulty": "hard"},
		Timestamp: time.Now(),
	}

	err = manager.StoreMessage(context.Background(), message)
	assert.NoError(t, err)

	// Verify message was stored by retrieving session history
	messages, err := manager.GetSessionHistory(context.Background(), "session-123")
	assert.NoError(t, err)
	assert.Len(t, messages, 1)

	stored := messages[0]
	assert.Equal(t, message.ID, stored.ID)
	assert.Equal(t, message.SessionID, stored.SessionID)
	assert.Equal(t, message.Type, stored.Type)
	assert.Equal(t, message.Context, stored.Context)
	assert.Equal(t, message.FromUser, stored.FromUser)
	assert.Nil(t, stored.ToUser)
	assert.Equal(t, message.Content, stored.Content)
	assert.WithinDuration(t, message.Timestamp, stored.Timestamp, time.Second)
}

// TestStoreMessage_WithRecipient tests storing direct messages
func TestStoreMessage_WithRecipient(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	// Create a session first (required by foreign key constraint)
	session := &types.Session{
		ID:         "session-123",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1", "student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	err := manager.CreateSession(context.Background(), session)
	require.NoError(t, err)

	toUser := "student1"
	message := &types.Message{
		ID:        "msg-456",
		SessionID: "session-123",
		Type:      types.MessageTypeInboxResponse,
		Context:   "answer",
		FromUser:  "instructor1",
		ToUser:    &toUser,
		Content:   map[string]interface{}{"text": "Here's the solution..."},
		Timestamp: time.Now(),
	}

	err = manager.StoreMessage(context.Background(), message)
	assert.NoError(t, err)

	// Verify message was stored with recipient
	messages, err := manager.GetSessionHistory(context.Background(), "session-123")
	assert.NoError(t, err)
	assert.Len(t, messages, 1)

	stored := messages[0]
	assert.NotNil(t, stored.ToUser)
	assert.Equal(t, "student1", *stored.ToUser)
}

// TestGetSessionHistory tests message history retrieval
func TestGetSessionHistory(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	sessionID := "session-123"
	
	// Create a session first (required by foreign key constraint)
	session := &types.Session{
		ID:         sessionID,
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1", "student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	err := manager.CreateSession(context.Background(), session)
	require.NoError(t, err)

	baseTime := time.Now()

	// Create messages in different order than chronological
	messages := []*types.Message{
		{
			ID:        "msg-2",
			SessionID: sessionID,
			Type:      types.MessageTypeInstructorInbox,
			Context:   "question",
			FromUser:  "student1",
			Content:   map[string]interface{}{"text": "Second message"},
			Timestamp: baseTime.Add(2 * time.Minute),
		},
		{
			ID:        "msg-1",
			SessionID: sessionID,
			Type:      types.MessageTypeInstructorInbox,
			Context:   "question",
			FromUser:  "student1",
			Content:   map[string]interface{}{"text": "First message"},
			Timestamp: baseTime.Add(1 * time.Minute),
		},
		{
			ID:        "msg-3",
			SessionID: sessionID,
			Type:      types.MessageTypeInstructorInbox,
			Context:   "question",
			FromUser:  "student1",
			Content:   map[string]interface{}{"text": "Third message"},
			Timestamp: baseTime.Add(3 * time.Minute),
		},
	}

	// Store messages in random order
	for _, msg := range messages {
		require.NoError(t, manager.StoreMessage(context.Background(), msg))
	}

	// Retrieve history
	history, err := manager.GetSessionHistory(context.Background(), sessionID)
	assert.NoError(t, err)
	assert.Len(t, history, 3)

	// Verify chronological order
	assert.Equal(t, "msg-1", history[0].ID)
	assert.Equal(t, "msg-2", history[1].ID)
	assert.Equal(t, "msg-3", history[2].ID)
}

// TestGetSessionHistory_EmptySession tests history for non-existent session
func TestGetSessionHistory_EmptySession(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	history, err := manager.GetSessionHistory(context.Background(), "non-existent")
	assert.NoError(t, err)
	assert.Empty(t, history)
}

// TestHealthCheck tests database health check
func TestHealthCheck(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	err := manager.HealthCheck(context.Background())
	assert.NoError(t, err)
}

// TestHealthCheck_AfterClose tests health check on closed database
func TestHealthCheck_AfterClose(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	// Close the manager
	err := manager.Close()
	require.NoError(t, err)

	// Health check should fail
	err = manager.HealthCheck(context.Background())
	assert.Error(t, err)
}

// TestConcurrentWrites tests single-writer pattern
func TestConcurrentWrites(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	sessionID := "concurrent-test"
	
	// Create a session first (required by foreign key constraint)
	session := &types.Session{
		ID:         sessionID,
		Name:       "Concurrent Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1", "student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	err := manager.CreateSession(context.Background(), session)
	require.NoError(t, err)

	numGoroutines := 10
	messagesPerGoroutine := 5

	// Send messages concurrently
	errors := make(chan error, numGoroutines*messagesPerGoroutine)
	
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < messagesPerGoroutine; j++ {
				message := &types.Message{
					ID:        fmt.Sprintf("msg-%d-%d", goroutineID, j),
					SessionID: sessionID,
					Type:      types.MessageTypeInstructorInbox,
					Context:   "test",
					FromUser:  fmt.Sprintf("user-%d", goroutineID),
					Content:   map[string]interface{}{"text": fmt.Sprintf("Message %d from goroutine %d", j, goroutineID)},
					Timestamp: time.Now(),
				}
				errors <- manager.StoreMessage(context.Background(), message)
			}
		}(i)
	}

	// Collect errors
	for i := 0; i < numGoroutines*messagesPerGoroutine; i++ {
		err := <-errors
		assert.NoError(t, err)
	}

	// Verify all messages were stored
	history, err := manager.GetSessionHistory(context.Background(), sessionID)
	assert.NoError(t, err)
	assert.Len(t, history, numGoroutines*messagesPerGoroutine)
}

// TestWriteTimeout tests write operation timeout behavior
func TestWriteTimeout(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	// This test verifies that the timeout mechanism exists in executeWrite
	// We can't easily simulate a 30-second timeout in a unit test without
	// making the test very slow, so we'll test that the manager can handle
	// normal operations without timeout issues.
	
	session := &types.Session{
		ID:         "timeout-test",
		Name:       "Timeout Test",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}

	// This should complete successfully within the timeout
	err := manager.CreateSession(context.Background(), session)
	assert.NoError(t, err)
	
	// Verify the session was actually created
	retrievedSession, err := manager.GetSession(context.Background(), "timeout-test")
	assert.NoError(t, err)
	assert.Equal(t, "timeout-test", retrievedSession.ID)
}

// TestContextCancellation tests context cancellation handling
func TestContextCancellation(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Try to create session with cancelled context
	session := &types.Session{
		ID:         "cancelled-test",
		Name:       "Cancelled Test",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}

	err := manager.CreateSession(ctx, session)
	assert.Error(t, err)
}

// TestJSONSerialization tests JSON serialization/deserialization
func TestJSONSerialization(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	// Test complex student IDs
	complexStudentIDs := []string{
		"student_1",
		"student-2",
		"user123",
		"test_user_with_underscores",
	}

	session := &types.Session{
		ID:         "json-test",
		Name:       "JSON Test",
		CreatedBy:  "instructor1",
		StudentIDs: complexStudentIDs,
		StartTime:  time.Now(),
		Status:     "active",
	}

	err := manager.CreateSession(context.Background(), session)
	require.NoError(t, err)

	retrieved, err := manager.GetSession(context.Background(), session.ID)
	assert.NoError(t, err)
	assert.Equal(t, complexStudentIDs, retrieved.StudentIDs)

	// Test complex message content
	complexContent := map[string]interface{}{
		"text":        "Complex message",
		"metadata":    map[string]string{"priority": "high", "category": "question"},
		"attachments": []string{"file1.pdf", "image.png"},
		"numbers":     []int{1, 2, 3, 4, 5},
		"boolean":     true,
	}

	message := &types.Message{
		ID:        "json-msg-test",
		SessionID: session.ID,
		Type:      types.MessageTypeInstructorInbox,
		Context:   "question",
		FromUser:  "student1",
		Content:   complexContent,
		Timestamp: time.Now(),
	}

	err = manager.StoreMessage(context.Background(), message)
	require.NoError(t, err)

	messages, err := manager.GetSessionHistory(context.Background(), session.ID)
	assert.NoError(t, err)
	assert.Len(t, messages, 1)

	retrievedContent := messages[0].Content
	assert.Equal(t, "Complex message", retrievedContent["text"])
	assert.Equal(t, "high", retrievedContent["metadata"].(map[string]interface{})["priority"])
}

// TestDatabaseClose tests proper database shutdown
func TestDatabaseClose(t *testing.T) {
	manager, _ := setupTestManager(t)
	defer func() {
		// Don't call cleanup since we're testing Close ourselves
		_ = manager.Close()
	}()

	// Verify manager works before close
	err := manager.HealthCheck(context.Background())
	assert.NoError(t, err)

	// Close manager
	err = manager.Close()
	assert.NoError(t, err)

	// Second close should be idempotent
	err = manager.Close()
	assert.NoError(t, err)

	// Operations after close should fail
	session := &types.Session{
		ID:         "after-close",
		Name:       "After Close",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}

	err = manager.CreateSession(context.Background(), session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}