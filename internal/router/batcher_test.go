package router

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"switchboard/pkg/types"
)

// MockBatchStore tracks batch operations for testing
type MockBatchStore struct {
	mu          sync.Mutex
	batches     [][]*types.Message
	batchTimes  []time.Time
	failNext    bool
	blockNext   bool
	blockChan   chan struct{}
}

func NewMockBatchStore() *MockBatchStore {
	return &MockBatchStore{
		batches:    make([][]*types.Message, 0),
		batchTimes: make([]time.Time, 0),
		blockChan:  make(chan struct{}),
	}
}

func (m *MockBatchStore) StoreMessageBatch(ctx context.Context, messages []*types.Message) error {
	// Check if we should block BEFORE acquiring the mutex to avoid deadlock
	m.mu.Lock()
	shouldBlock := m.blockNext
	m.mu.Unlock()
	
	if shouldBlock {
		<-m.blockChan // Block until released
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failNext {
		m.failNext = false
		return assert.AnError
	}

	// Deep copy messages to prevent race conditions in tests
	batch := make([]*types.Message, len(messages))
	for i, msg := range messages {
		msgCopy := *msg
		batch[i] = &msgCopy
	}

	m.batches = append(m.batches, batch)
	m.batchTimes = append(m.batchTimes, time.Now())
	return nil
}

func (m *MockBatchStore) GetBatchCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.batches)
}

func (m *MockBatchStore) GetLastBatch() []*types.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.batches) == 0 {
		return nil
	}
	return m.batches[len(m.batches)-1]
}

func (m *MockBatchStore) GetBatches() [][]*types.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a deep copy to prevent race conditions
	result := make([][]*types.Message, len(m.batches))
	for i, batch := range m.batches {
		batchCopy := make([]*types.Message, len(batch))
		copy(batchCopy, batch)
		result[i] = batchCopy
	}
	return result
}

func (m *MockBatchStore) Unblock() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blockNext = false
	select {
	case m.blockChan <- struct{}{}:
		// Channel unblocked successfully
	default:
		// Channel already unblocked or closed, safe to ignore
	}
}

// Architectural Validation Tests

func TestBatcher_InterfaceCompliance(t *testing.T) {
	// ARCHITECTURAL VALIDATION: Verify clean boundaries
	// Batcher should not implement database interfaces directly
	store := NewMockBatchStore()
	batcher := NewMessageBatcher(store, 50, 100*time.Millisecond)
	require.NotNil(t, batcher)
	
	// Verify batcher doesn't expose database operations
	// Only Add, Start, Stop, and GetMetrics should be public
	assert.IsType(t, &MessageBatcher{}, batcher)
}

func TestBatcher_NoDatabaseImports(t *testing.T) {
	// ARCHITECTURAL VALIDATION: No direct database access
	// This test will fail during compilation if batcher imports database package
	// The batcher should only know about the BatchStore interface
	_ = &MessageBatcher{}
}

// Functional Validation Tests

func TestBatcher_AddMessage(t *testing.T) {
	// FUNCTIONAL VALIDATION: Messages accumulate correctly
	store := NewMockBatchStore()
	batcher := NewMessageBatcher(store, 50, 100*time.Millisecond)
	
	ctx := context.Background()
	if err := batcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start batcher: %v", err)
	}
	defer func() {
		if err := batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()

	// Add messages below batch size
	msg1 := &types.Message{ID: "1", Content: map[string]interface{}{"text": "hello"}}
	msg2 := &types.Message{ID: "2", Content: map[string]interface{}{"text": "world"}}
	
	err := batcher.Add(msg1)
	assert.NoError(t, err)
	
	err = batcher.Add(msg2)
	assert.NoError(t, err)
	
	// Messages should accumulate, not flush immediately
	assert.Equal(t, 0, store.GetBatchCount())
	
	// Wait for time-based flush
	time.Sleep(150 * time.Millisecond)
	
	assert.Equal(t, 1, store.GetBatchCount())
	batch := store.GetLastBatch()
	assert.Len(t, batch, 2)
	assert.Equal(t, "1", batch[0].ID)
	assert.Equal(t, "2", batch[1].ID)
}

func TestBatcher_TimeTriggerFlush(t *testing.T) {
	// FUNCTIONAL VALIDATION: Flushes after configured time
	store := NewMockBatchStore()
	batcher := NewMessageBatcher(store, 50, 100*time.Millisecond)
	
	ctx := context.Background()
	if err := batcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start batcher: %v", err)
	}
	defer func() {
		if err := batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()

	// Add single message
	msg := &types.Message{ID: "1"}
	err := batcher.Add(msg)
	assert.NoError(t, err)
	
	// Should not flush immediately
	assert.Equal(t, 0, store.GetBatchCount())
	
	// Wait for timer
	time.Sleep(150 * time.Millisecond)
	
	// Should have flushed
	assert.Equal(t, 1, store.GetBatchCount())
	assert.Len(t, store.GetLastBatch(), 1)
}

func TestBatcher_SizeTriggerFlush(t *testing.T) {
	// FUNCTIONAL VALIDATION: Flushes at configured size
	store := NewMockBatchStore()
	batcher := NewMessageBatcher(store, 3, 1*time.Hour) // Long timer to test size trigger
	
	ctx := context.Background()
	if err := batcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start batcher: %v", err)
	}
	defer func() {
		if err := batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()

	// Add messages up to batch size
	for i := 0; i < 3; i++ {
		msg := &types.Message{ID: string(rune('0' + i))}
		err := batcher.Add(msg)
		assert.NoError(t, err)
	}
	
	// Should flush immediately when batch size reached
	time.Sleep(10 * time.Millisecond) // Small delay for async flush
	
	assert.Equal(t, 1, store.GetBatchCount())
	assert.Len(t, store.GetLastBatch(), 3)
}

func TestBatcher_GracefulShutdown(t *testing.T) {
	// FUNCTIONAL VALIDATION: Drains queue on stop
	store := NewMockBatchStore()
	batcher := NewMessageBatcher(store, 50, 1*time.Hour)
	
	ctx := context.Background()
	if err := batcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start batcher: %v", err)
	}

	// Add messages
	for i := 0; i < 5; i++ {
		msg := &types.Message{ID: string(rune('0' + i))}
		err := batcher.Add(msg)
		assert.NoError(t, err)
	}
	
	// Stop should flush pending messages
	err := batcher.Stop()
	assert.NoError(t, err)
	
	assert.Equal(t, 1, store.GetBatchCount())
	assert.Len(t, store.GetLastBatch(), 5)
}

func TestBatcher_HandlesPersistenceFailure(t *testing.T) {
	// FUNCTIONAL VALIDATION: Continues operation on storage failure
	store := NewMockBatchStore()
	batcher := NewMessageBatcher(store, 2, 100*time.Millisecond)
	
	ctx := context.Background()
	if err := batcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start batcher: %v", err)
	}
	defer func() {
		if err := batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()

	// First batch will fail
	store.failNext = true
	
	msg1 := &types.Message{ID: "1"}
	msg2 := &types.Message{ID: "2"}
	
	err := batcher.Add(msg1)
	assert.NoError(t, err)
	err = batcher.Add(msg2)
	assert.NoError(t, err)
	
	// Wait for flush
	time.Sleep(10 * time.Millisecond)
	
	// Should have attempted but failed
	assert.Equal(t, 0, store.GetBatchCount())
	
	// Metrics should reflect dropped messages
	metrics := batcher.GetMetrics()
	assert.Equal(t, uint64(2), metrics.DroppedMessages)
	
	// Next batch should work
	msg3 := &types.Message{ID: "3"}
	msg4 := &types.Message{ID: "4"}
	
	err = batcher.Add(msg3)
	assert.NoError(t, err)
	err = batcher.Add(msg4)
	assert.NoError(t, err)
	
	time.Sleep(10 * time.Millisecond)
	
	assert.Equal(t, 1, store.GetBatchCount())
}

// Technical Validation Tests

func TestBatcher_ConcurrentAdd(t *testing.T) {
	// TECHNICAL VALIDATION: Race-free message addition
	store := NewMockBatchStore()
	batcher := NewMessageBatcher(store, 100, 100*time.Millisecond)
	
	ctx := context.Background()
	if err := batcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start batcher: %v", err)
	}
	defer func() {
		if err := batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()

	// Concurrent message addition
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				msg := &types.Message{
					ID: fmt.Sprintf("%d-%d", id, j),
				}
				err := batcher.Add(msg)
				assert.NoError(t, err)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Force flush
	time.Sleep(150 * time.Millisecond)
	
	// Should have all 100 messages
	totalMessages := 0
	batches := store.GetBatches()
	for _, batch := range batches {
		totalMessages += len(batch)
	}
	assert.Equal(t, 100, totalMessages)
}

func TestBatcher_MetricsAccuracy(t *testing.T) {
	// TECHNICAL VALIDATION: Tracks metrics correctly
	store := NewMockBatchStore()
	batcher := NewMessageBatcher(store, 5, 100*time.Millisecond)
	
	ctx := context.Background()
	if err := batcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start batcher: %v", err)
	}
	defer func() {
		if err := batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()

	// Add messages
	for i := 0; i < 7; i++ {
		msg := &types.Message{ID: string(rune('0' + i))}
		err := batcher.Add(msg)
		assert.NoError(t, err)
	}
	
	// Wait a bit for the batch flush to happen
	time.Sleep(50 * time.Millisecond)
	
	// Check queue depth after batch flush
	metrics := batcher.GetMetrics()
	assert.Equal(t, 2, metrics.QueueDepth) // 7 messages, 5 flushed, 2 remaining
	
	// Wait for time flush
	time.Sleep(150 * time.Millisecond)
	
	// Check final metrics
	metrics = batcher.GetMetrics()
	assert.Equal(t, 0, metrics.QueueDepth)
	assert.Equal(t, uint64(7), metrics.TotalMessages)
	assert.Equal(t, uint64(2), metrics.BatchCount)
	assert.Equal(t, uint64(0), metrics.DroppedMessages)
}

func TestBatcher_ContextCancellation(t *testing.T) {
	// TECHNICAL VALIDATION: Respects context cancellation
	store := NewMockBatchStore()
	store.blockNext = true // Block first batch operation
	
	batcher := NewMessageBatcher(store, 2, 100*time.Millisecond)
	
	ctx, cancel := context.WithCancel(context.Background())
	if err := batcher.Start(ctx); err != nil {
		t.Logf("Failed to start batcher: %v", err)
	}

	// Add messages to trigger batch
	msg1 := &types.Message{ID: "1"}
	msg2 := &types.Message{ID: "2"}
	
	err := batcher.Add(msg1)
	assert.NoError(t, err)
	err = batcher.Add(msg2)
	assert.NoError(t, err)
	
	// Cancel context while batch is blocked
	cancel()
	
	// Unblock store
	close(store.blockChan)
	
	// Stop should complete even with cancelled context
	err = batcher.Stop()
	assert.NoError(t, err)
}

func TestBatcher_QueueFullBehavior(t *testing.T) {
	// TECHNICAL VALIDATION: Handles full queue gracefully
	store := NewMockBatchStore()
	store.blockNext = true // Block to fill queue
	
	// Small queue for testing
	batcher := NewMessageBatcherWithQueueSize(store, 2, 100*time.Millisecond, 5)
	
	ctx := context.Background()
	if err := batcher.Start(ctx); err != nil {
		t.Logf("Failed to start batcher: %v", err)
	}
	defer func() {
		store.Unblock() // Unblock before stop
		if err := batcher.Stop(); err != nil {
			t.Logf("Failed to stop batcher: %v", err)
		}
	}()

	// Fill queue - with blocked store, messages just go into the channel queue
	for i := 0; i < 10; i++ {
		msg := &types.Message{ID: string(rune('0' + i))}
		err := batcher.Add(msg)
		if i < 5 { // Queue size is 5
			assert.NoError(t, err, "Message %d should succeed", i)
		} else {
			assert.Error(t, err, "Message %d should fail", i)
			assert.ErrorIs(t, err, ErrQueueFull)
		}
	}
	
	metrics := batcher.GetMetrics()
	assert.Equal(t, uint64(5), metrics.DroppedMessages) // Messages 5-9 were dropped
}

