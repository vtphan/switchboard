package router

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"switchboard/internal/websocket"
	"switchboard/pkg/types"
)

// Test basic router functionality to improve coverage
func TestRouter_BasicFunctionality(t *testing.T) {
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)

	// Test router initialization
	assert.NotNil(t, router)
	assert.NotNil(t, router.registry)
	assert.NotNil(t, router.dbManager)
	assert.NotNil(t, router.rateLimiter)
}

func TestRouter_MessageTypeValidation(t *testing.T) {
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)

	// Test all valid message types
	validTypes := []string{
		types.MessageTypeInstructorInbox,
		types.MessageTypeInboxResponse,
		types.MessageTypeRequest,
		types.MessageTypeRequestResponse,
		types.MessageTypeAnalytics,
		types.MessageTypeInstructorBroadcast,
	}

	for _, msgType := range validTypes {
		assert.True(t, router.isValidMessageType(msgType), "Message type %s should be valid", msgType)
	}

	// Test invalid message types
	assert.False(t, router.isValidMessageType("invalid_type"))
	assert.False(t, router.isValidMessageType(""))
	assert.False(t, router.isValidMessageType("chat"))
}

func TestRouter_RolePermissions(t *testing.T) {
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)

	// Test student permissions
	assert.True(t, router.canSendMessageType("student", types.MessageTypeInstructorInbox))
	assert.True(t, router.canSendMessageType("student", types.MessageTypeRequestResponse))
	assert.True(t, router.canSendMessageType("student", types.MessageTypeAnalytics))
	assert.False(t, router.canSendMessageType("student", types.MessageTypeInboxResponse))
	assert.False(t, router.canSendMessageType("student", types.MessageTypeRequest))
	assert.False(t, router.canSendMessageType("student", types.MessageTypeInstructorBroadcast))

	// Test instructor permissions  
	assert.False(t, router.canSendMessageType("instructor", types.MessageTypeInstructorInbox))
	assert.False(t, router.canSendMessageType("instructor", types.MessageTypeRequestResponse))
	assert.False(t, router.canSendMessageType("instructor", types.MessageTypeAnalytics))
	assert.True(t, router.canSendMessageType("instructor", types.MessageTypeInboxResponse))
	assert.True(t, router.canSendMessageType("instructor", types.MessageTypeRequest))
	assert.True(t, router.canSendMessageType("instructor", types.MessageTypeInstructorBroadcast))

	// Test invalid roles
	assert.False(t, router.canSendMessageType("admin", types.MessageTypeInstructorInbox))
	assert.False(t, router.canSendMessageType("", types.MessageTypeAnalytics))
	assert.False(t, router.canSendMessageType("unknown", types.MessageTypeRequest))
}

func TestRouter_RateLimiter(t *testing.T) {
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)

	// Test that rate limiter is initialized
	assert.NotNil(t, router.rateLimiter)

	// Test rate limiter allows initial requests
	assert.True(t, router.rateLimiter.Allow("user1"))
	assert.True(t, router.rateLimiter.Allow("user2"))

	// Test rate limiter with token bucket (global limiting, not per-user)
	// BEHAVIOR CHANGE: Token bucket provides global rate limiting vs per-user sliding window
	allowedCount := 0
	for i := 0; i < 50; i++ {
		if router.rateLimiter.Allow("user1") {
			allowedCount++
		}
		if router.rateLimiter.Allow("user2") {
			allowedCount++
		}
	}
	
	// Should allow up to remaining capacity (100 - 2 initial = 98)
	assert.LessOrEqual(t, allowedCount, 98, "Should respect global token bucket capacity")
	assert.Greater(t, allowedCount, 50, "Should allow substantial number of messages")
}

func TestRouter_DirectAsyncPersistence(t *testing.T) {
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)

	// Test direct async persistence
	message := &types.Message{
		ID:        "test-msg",
		SessionID: "test-session",
		Type:      types.MessageTypeAnalytics,
		FromUser:  "student1",
		Content:   map[string]interface{}{"data": "test"},
		Timestamp: time.Now(),
	}

	// Call persistMessageDirect
	router.persistMessageDirect(message)

	// Wait for async operation
	time.Sleep(50 * time.Millisecond)

	// Verify persistence was called
	assert.Equal(t, 1, mockDB.GetStoreMessageCalls())
}

func TestRouter_BatchingConfiguration(t *testing.T) {
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)

	// Test enabling batching
	err := router.EnableBatching(10, 100*time.Millisecond)
	assert.NoError(t, err)

	// Test batcher metrics
	metrics := router.GetBatcherMetrics()
	assert.Equal(t, 0, metrics.QueueDepth)
	assert.Equal(t, uint64(0), metrics.TotalMessages)

	// Clean shutdown
	if router.batcher != nil {
		if err := router.batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}
}

func TestRouter_BatchingWithNilDatabase(t *testing.T) {
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)

	// Test enabling batching without database should fail
	err := router.EnableBatching(10, 100*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database manager required")

	// Test getting metrics without batcher
	metrics := router.GetBatcherMetrics()
	assert.Equal(t, BatchMetrics{}, metrics)
}

func TestRouter_SetBatchStoreConfig(t *testing.T) {
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)

	// Test setting custom batch store
	mockStore := NewMockBatchStore()
	router.SetBatchStore(mockStore)

	// Test batcher metrics after setting store
	metrics := router.GetBatcherMetrics()
	assert.Equal(t, 0, metrics.QueueDepth)

	// Clean shutdown
	if router.batcher != nil {
		if err := router.batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}
}

func TestRouter_BatchStoreReplacement(t *testing.T) {
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)

	// First set up batching
	err := router.EnableBatching(10, 50*time.Millisecond)
	assert.NoError(t, err)

	oldBatcher := router.batcher
	assert.NotNil(t, oldBatcher)

	// Replace with new batch store
	mockStore := NewMockBatchStore()
	router.SetBatchStore(mockStore)

	// Should have new batcher
	assert.NotEqual(t, oldBatcher, router.batcher)

	// Clean shutdown
	if router.batcher != nil {
		if err := router.batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}
}

func TestRouter_DatabaseBatchStoreAdapter(t *testing.T) {
	mockDB := NewTrackedMockDB()
	store := &databaseBatchStore{dbManager: mockDB}

	// Test batch store adapter
	messages := []*types.Message{
		{
			ID:        "batch-msg-1",
			SessionID: "test-session",
			Type:      types.MessageTypeAnalytics,
			FromUser:  "student1",
			Content:   map[string]interface{}{"index": 1},
			Timestamp: time.Now(),
		},
		{
			ID:        "batch-msg-2", 
			SessionID: "test-session",
			Type:      types.MessageTypeAnalytics,
			FromUser:  "student2",
			Content:   map[string]interface{}{"index": 2},
			Timestamp: time.Now(),
		},
	}

	// Store batch
	err := store.StoreMessageBatch(context.Background(), messages)
	assert.NoError(t, err)

	// Verify batch was stored
	assert.Equal(t, 1, mockDB.GetStoreMessageBatchCalls())
	batches := mockDB.GetBatches()
	assert.Len(t, batches, 1)
	assert.Len(t, batches[0], 2)
	assert.Equal(t, "batch-msg-1", batches[0][0].ID)
	assert.Equal(t, "batch-msg-2", batches[0][1].ID)
}

func TestRouter_AsyncPersistenceWithBatcher(t *testing.T) {
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)

	// Enable batching
	err := router.EnableBatching(2, 100*time.Millisecond)
	assert.NoError(t, err)
	defer func() {
		if err := router.batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()

	// Create test message
	message := &types.Message{
		ID:        "test-msg-batched",
		SessionID: "test-session",
		Type:      types.MessageTypeAnalytics,
		FromUser:  "student1",
		Content:   map[string]interface{}{"data": "batched"},
		Timestamp: time.Now(),
	}

	// Persist message async (should use batcher)
	router.persistMessageAsync(message)

	// Wait for batching
	time.Sleep(150 * time.Millisecond)

	// Verify message was batched (not stored directly)
	assert.Equal(t, 0, mockDB.GetStoreMessageCalls())
	assert.Equal(t, 1, mockDB.GetStoreMessageBatchCalls())

	// Verify message in batch
	batches := mockDB.GetBatches()
	assert.Len(t, batches, 1)
	assert.Len(t, batches[0], 1)
	assert.Equal(t, "test-msg-batched", batches[0][0].ID)
}

func TestRouter_AsyncPersistenceWithoutBatcher(t *testing.T) {
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)

	// Don't enable batching

	// Create test message
	message := &types.Message{
		ID:        "test-msg-direct",
		SessionID: "test-session",
		Type:      types.MessageTypeAnalytics,
		FromUser:  "student1",
		Content:   map[string]interface{}{"data": "direct"},
		Timestamp: time.Now(),
	}

	// Persist message async (should use direct persistence)
	router.persistMessageAsync(message)

	// Wait for async goroutine
	time.Sleep(50 * time.Millisecond)

	// Verify message was stored directly
	assert.Equal(t, 1, mockDB.GetStoreMessageCalls())
	assert.Equal(t, 0, mockDB.GetStoreMessageBatchCalls())

	// Verify message content
	messages := mockDB.GetMessages()
	assert.Len(t, messages, 1)
	assert.Equal(t, "test-msg-direct", messages[0].ID)
}

func TestRouter_PersistenceWithNilDatabase(t *testing.T) {
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)

	// Create test message
	message := &types.Message{
		ID:        "test-msg-no-db",
		SessionID: "test-session",
		Type:      types.MessageTypeAnalytics,
		FromUser:  "student1",
		Content:   map[string]interface{}{"data": "test"},
		Timestamp: time.Now(),
	}

	// Should not panic or error with nil database
	router.persistMessageDirect(message)
	router.persistMessageAsync(message)

	// Wait briefly
	time.Sleep(10 * time.Millisecond)

	// No assertions needed - just verify no panic
}

func TestRouter_PersistenceWithDatabaseError(t *testing.T) {
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	mockDB.shouldFail = true
	router := NewRouter(registry, mockDB)

	// Create test message
	message := &types.Message{
		ID:        "test-msg-error",
		SessionID: "test-session",
		Type:      types.MessageTypeAnalytics,
		FromUser:  "student1",
		Content:   map[string]interface{}{"data": "error"},
		Timestamp: time.Now(),
	}

	// Should not panic despite database error
	router.persistMessageDirect(message)

	// Wait for async goroutine
	time.Sleep(50 * time.Millisecond)

	// Verify database was called despite error
	assert.Equal(t, 1, mockDB.GetStoreMessageCalls())
}

func TestRouter_ConcurrentAsyncPersistenceLoad(t *testing.T) {
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)

	// Enable batching
	err := router.EnableBatching(10, 50*time.Millisecond)
	assert.NoError(t, err)
	defer func() {
		if err := router.batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()

	// Concurrent message persistence
	const numMessages = 20
	for i := 0; i < numMessages; i++ {
		go func(index int) {
			message := &types.Message{
				ID:        "concurrent-msg-" + string(rune('0'+(index%10))),
				SessionID: "test-session",
				Type:      types.MessageTypeAnalytics,
				FromUser:  "student1",
				Content:   map[string]interface{}{"index": index},
				Timestamp: time.Now(),
			}
			router.persistMessageAsync(message)
		}(i)
	}

	// Wait for all batches to flush
	time.Sleep(200 * time.Millisecond)

	// Verify batching was used
	metrics := router.GetBatcherMetrics()
	assert.Equal(t, uint64(numMessages), metrics.TotalMessages)
	assert.Equal(t, uint64(0), metrics.DroppedMessages)
	assert.Greater(t, mockDB.GetStoreMessageBatchCalls(), 0)
}