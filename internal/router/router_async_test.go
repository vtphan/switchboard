package router

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"switchboard/internal/websocket"
	"switchboard/pkg/types"
)

// TrackedMockDB tracks database operations for async persistence testing
type TrackedMockDB struct {
	storeMessageCalls      int32
	storeMessageBatchCalls int32
	messages               []*types.Message
	batchMessages          [][]*types.Message
	mu                     sync.Mutex
	shouldFail             bool
	storeDelay             time.Duration
}

func NewTrackedMockDB() *TrackedMockDB {
	return &TrackedMockDB{
		messages:      make([]*types.Message, 0),
		batchMessages: make([][]*types.Message, 0),
	}
}

func (t *TrackedMockDB) StoreMessage(ctx context.Context, message *types.Message) error {
	if t.storeDelay > 0 {
		time.Sleep(t.storeDelay)
	}
	
	atomic.AddInt32(&t.storeMessageCalls, 1)
	
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.shouldFail {
		return assert.AnError
	}
	
	// Deep copy to avoid race conditions
	msgCopy := *message
	t.messages = append(t.messages, &msgCopy)
	return nil
}

func (t *TrackedMockDB) StoreMessageBatch(ctx context.Context, messages []*types.Message) error {
	if t.storeDelay > 0 {
		time.Sleep(t.storeDelay)
	}
	
	atomic.AddInt32(&t.storeMessageBatchCalls, 1)
	
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.shouldFail {
		return assert.AnError
	}
	
	// Deep copy batch to avoid race conditions
	batch := make([]*types.Message, len(messages))
	for i, msg := range messages {
		msgCopy := *msg
		batch[i] = &msgCopy
	}
	t.batchMessages = append(t.batchMessages, batch)
	return nil
}

func (t *TrackedMockDB) GetStoreMessageCalls() int {
	return int(atomic.LoadInt32(&t.storeMessageCalls))
}

func (t *TrackedMockDB) GetStoreMessageBatchCalls() int {
	return int(atomic.LoadInt32(&t.storeMessageBatchCalls))
}

func (t *TrackedMockDB) GetMessages() []*types.Message {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	result := make([]*types.Message, len(t.messages))
	copy(result, t.messages)
	return result
}

func (t *TrackedMockDB) GetBatches() [][]*types.Message {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	result := make([][]*types.Message, len(t.batchMessages))
	copy(result, t.batchMessages)
	return result
}

// Implement remaining DatabaseManager methods
func (t *TrackedMockDB) CreateSession(ctx context.Context, session *types.Session) error { return nil }
func (t *TrackedMockDB) GetSession(ctx context.Context, sessionID string) (*types.Session, error) { 
	return &types.Session{}, nil 
}
func (t *TrackedMockDB) UpdateSession(ctx context.Context, session *types.Session) error { return nil }
func (t *TrackedMockDB) ListActiveSessions(ctx context.Context) ([]*types.Session, error) { 
	return []*types.Session{}, nil 
}
func (t *TrackedMockDB) GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error) { 
	return []*types.Message{}, nil 
}
func (t *TrackedMockDB) HealthCheck(ctx context.Context) error { return nil }
func (t *TrackedMockDB) Close() error { return nil }

// Router Async Persistence Tests

func TestRouter_EnableBatching(t *testing.T) {
	// Test enabling batching with valid database manager
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)

	// EnableBatching should succeed with valid database
	err := router.EnableBatching(25, 50*time.Millisecond)
	assert.NoError(t, err)
	
	// Verify batcher is initialized
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

func TestRouter_EnableBatching_NoDatabaseManager(t *testing.T) {
	// Test enabling batching without database manager
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)

	// EnableBatching should fail without database
	err := router.EnableBatching(25, 50*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database manager required")
	
	// Batcher should not be initialized
	metrics := router.GetBatcherMetrics()
	assert.Equal(t, BatchMetrics{}, metrics)
}

func TestRouter_SetBatchStore(t *testing.T) {
	// Test setting custom batch store
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)
	
	mockStore := NewMockBatchStore()
	router.SetBatchStore(mockStore)
	
	// Verify batcher is initialized with custom store
	metrics := router.GetBatcherMetrics()
	assert.Equal(t, 0, metrics.QueueDepth)
	
	// Clean shutdown
	if router.batcher != nil {
		if err := router.batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}
}

func TestRouter_SetBatchStore_ReplacesExisting(t *testing.T) {
	// Test replacing existing batcher
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)
	
	// First set up batching
	err := router.EnableBatching(10, 50*time.Millisecond)
	require.NoError(t, err)
	
	oldBatcher := router.batcher
	require.NotNil(t, oldBatcher)
	
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

func TestRouter_GetBatcherMetrics_NoBatcher(t *testing.T) {
	// Test getting metrics when no batcher is configured
	registry := websocket.NewRegistry()
	router := NewRouter(registry, nil)

	// Should return empty metrics
	metrics := router.GetBatcherMetrics()
	assert.Equal(t, BatchMetrics{}, metrics)
}

func TestRouter_persistMessageAsync_WithBatcher(t *testing.T) {
	// Test async persistence when batcher is enabled
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)
	
	// Enable batching
	err := router.EnableBatching(2, 100*time.Millisecond)
	require.NoError(t, err)
	defer func() {
		if err := router.batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()
	
	// Create test message
	message := &types.Message{
		ID:        "test-msg-1",
		SessionID: "test-session",
		Type:      types.MessageTypeAnalytics,
		FromUser:  "student1",
		Content:   map[string]interface{}{"data": "test"},
		Timestamp: time.Now(),
	}
	
	// Persist message async
	router.persistMessageAsync(message)
	
	// Wait for batching
	time.Sleep(150 * time.Millisecond)
	
	// Verify message was batched (not stored directly)
	assert.Equal(t, 0, mockDB.GetStoreMessageCalls())
	assert.Equal(t, 1, mockDB.GetStoreMessageBatchCalls())
	
	// Verify message in batch
	batches := mockDB.GetBatches()
	require.Len(t, batches, 1)
	assert.Len(t, batches[0], 1)
	assert.Equal(t, "test-msg-1", batches[0][0].ID)
}

func TestRouter_persistMessageAsync_NoBatcher(t *testing.T) {
	// Test async persistence when no batcher is configured
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)
	
	// Create test message
	message := &types.Message{
		ID:        "test-msg-2",
		SessionID: "test-session",
		Type:      types.MessageTypeAnalytics,
		FromUser:  "student1",
		Content:   map[string]interface{}{"data": "test"},
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
	require.Len(t, messages, 1)
	assert.Equal(t, "test-msg-2", messages[0].ID)
}

func TestRouter_persistMessageAsync_BatcherFallback(t *testing.T) {
	// Test fallback to direct persistence when batcher fails
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)
	
	// Enable batching with very small queue to force failure
	store := NewMockBatchStore()
	store.blockNext = true // This will block the batcher
	router.SetBatchStore(store)
	defer func() {
		if err := router.batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()
	
	// Fill the batcher queue to trigger fallback
	for i := 0; i < 10; i++ {
		message := &types.Message{
			ID:        "test-msg-" + string(rune('0'+i)),
			SessionID: "test-session",
			Type:      types.MessageTypeAnalytics,
			FromUser:  "student1",
			Content:   map[string]interface{}{"index": i},
			Timestamp: time.Now(),
		}
		router.persistMessageAsync(message)
	}
	
	// Wait for async operations
	time.Sleep(100 * time.Millisecond)
	
	// Some messages should have fallen back to direct persistence
	// (exact count depends on timing, but should be > 0)
	assert.Greater(t, mockDB.GetStoreMessageCalls(), 0)
}

func TestRouter_persistMessageDirect(t *testing.T) {
	// Test direct message persistence
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)
	
	// Create test message
	message := &types.Message{
		ID:        "test-msg-direct",
		SessionID: "test-session",
		Type:      types.MessageTypeAnalytics,
		FromUser:  "student1",
		Content:   map[string]interface{}{"data": "direct"},
		Timestamp: time.Now(),
	}
	
	// Persist message directly
	router.persistMessageDirect(message)
	
	// Wait for async goroutine
	time.Sleep(50 * time.Millisecond)
	
	// Verify message was stored
	assert.Equal(t, 1, mockDB.GetStoreMessageCalls())
	messages := mockDB.GetMessages()
	require.Len(t, messages, 1)
	assert.Equal(t, "test-msg-direct", messages[0].ID)
	assert.Equal(t, "direct", messages[0].Content["data"])
}

func TestRouter_persistMessageDirect_NoDatabaseManager(t *testing.T) {
	// Test direct persistence with no database manager
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
	
	// Should not panic or error
	router.persistMessageDirect(message)
	
	// Wait briefly
	time.Sleep(10 * time.Millisecond)
	
	// No assertions needed - just verify no panic
}

func TestRouter_persistMessageDirect_DatabaseError(t *testing.T) {
	// Test direct persistence with database error
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

func TestRouter_databaseBatchStore_Integration(t *testing.T) {
	// Test the database batch store adapter
	mockDB := NewTrackedMockDB()
	store := &databaseBatchStore{dbManager: mockDB}
	
	// Create test messages
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
	require.Len(t, batches, 1)
	assert.Len(t, batches[0], 2)
	assert.Equal(t, "batch-msg-1", batches[0][0].ID)
	assert.Equal(t, "batch-msg-2", batches[0][1].ID)
}

func TestRouter_BatchingMetrics_Integration(t *testing.T) {
	// Test batcher metrics integration
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)
	
	// Enable batching
	err := router.EnableBatching(3, 100*time.Millisecond)
	require.NoError(t, err)
	defer func() {
		if err := router.batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()
	
	// Add messages to build up metrics
	for i := 0; i < 5; i++ {
		message := &types.Message{
			ID:        "metrics-msg-" + string(rune('0'+i)),
			SessionID: "test-session",
			Type:      types.MessageTypeAnalytics,
			FromUser:  "student1",
			Content:   map[string]interface{}{"index": i},
			Timestamp: time.Now(),
		}
		router.persistMessageAsync(message)
	}
	
	// Wait for processing
	time.Sleep(150 * time.Millisecond)
	
	// Check metrics
	metrics := router.GetBatcherMetrics()
	assert.Equal(t, uint64(5), metrics.TotalMessages)
	assert.Greater(t, metrics.BatchCount, uint64(0))
	assert.Equal(t, uint64(0), metrics.DroppedMessages)
	
	// Verify batching occurred
	assert.Greater(t, mockDB.GetStoreMessageBatchCalls(), 0)
}

func TestRouter_ConcurrentAsyncPersistence(t *testing.T) {
	// Test concurrent async persistence operations
	registry := websocket.NewRegistry()
	mockDB := NewTrackedMockDB()
	router := NewRouter(registry, mockDB)
	
	// Enable batching
	err := router.EnableBatching(10, 50*time.Millisecond)
	require.NoError(t, err)
	defer func() {
		if err := router.batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()
	
	// Concurrent message persistence
	const numGoroutines = 10
	const messagesPerGoroutine = 5
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < messagesPerGoroutine; j++ {
				message := &types.Message{
					ID:        "concurrent-msg-" + string(rune('0'+goroutineID)) + "-" + string(rune('0'+j)),
					SessionID: "test-session",
					Type:      types.MessageTypeAnalytics,
					FromUser:  "student" + string(rune('0'+goroutineID)),
					Content:   map[string]interface{}{"goroutine": goroutineID, "message": j},
					Timestamp: time.Now(),
				}
				router.persistMessageAsync(message)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Wait for all batches to flush
	time.Sleep(200 * time.Millisecond)
	
	// Verify all messages were processed
	metrics := router.GetBatcherMetrics()
	assert.Equal(t, uint64(numGoroutines*messagesPerGoroutine), metrics.TotalMessages)
	assert.Equal(t, uint64(0), metrics.DroppedMessages)
	
	// Verify batching was used
	assert.Greater(t, mockDB.GetStoreMessageBatchCalls(), 0)
}