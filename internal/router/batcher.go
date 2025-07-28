package router

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"switchboard/pkg/types"
)

// BatchStore interface for persisting message batches
type BatchStore interface {
	StoreMessageBatch(ctx context.Context, messages []*types.Message) error
}

// MessageBatcher accumulates messages and flushes them in batches
// ARCHITECTURAL DISCOVERY: Decouples message routing from persistence latency
// by buffering messages and writing them in efficient batches
type MessageBatcher struct {
	store         BatchStore
	batchSize     int
	flushInterval time.Duration
	queueSize     int
	
	// Message accumulation
	messages  []*types.Message
	mu        sync.Mutex
	
	// Control channels
	messageCh chan *types.Message // FUNCTIONAL DISCOVERY: Buffered channel prevents blocking during bursts
	flushCh   chan struct{}       // Triggers immediate flush
	stopCh    chan struct{}       // Shutdown signal
	doneCh    chan struct{}       // Shutdown complete
	
	// Queue tracking
	queuedMessages atomic.Int64    // Total messages in system (channel + processing)
	
	// Metrics
	totalMessages   atomic.Uint64
	batchCount      atomic.Uint64
	droppedMessages atomic.Uint64
	
	// State
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// BatchMetrics contains batcher performance metrics
type BatchMetrics struct {
	QueueDepth      int
	TotalMessages   uint64
	BatchCount      uint64
	DroppedMessages uint64
}

// Error definitions
var (
	ErrQueueFull       = errors.New("message queue is full")
	ErrBatcherStopped  = errors.New("batcher is stopped")
	ErrBatcherNotStarted = errors.New("batcher not started")
)

// NewMessageBatcher creates a new message batcher with default queue size
func NewMessageBatcher(store BatchStore, batchSize int, flushInterval time.Duration) *MessageBatcher {
	return NewMessageBatcherWithQueueSize(store, batchSize, flushInterval, 1000)
}

// NewMessageBatcherWithQueueSize creates a new message batcher with specified queue size
// TECHNICAL DISCOVERY: Queue size should be at least 2x batch size to handle bursts
func NewMessageBatcherWithQueueSize(store BatchStore, batchSize int, flushInterval time.Duration, queueSize int) *MessageBatcher {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &MessageBatcher{
		store:         store,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		queueSize:     queueSize,
		messages:      make([]*types.Message, 0, batchSize),
		messageCh:     make(chan *types.Message, queueSize),
		flushCh:       make(chan struct{}, 1), // Buffered to prevent blocking
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins processing messages
// FUNCTIONAL DISCOVERY: Single goroutine processes all batches to maintain order
func (b *MessageBatcher) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return errors.New("batcher already running")
	}
	b.running = true
	b.mu.Unlock()
	
	// Update context if provided
	if ctx != nil {
		b.ctx = ctx
	}
	
	go b.processingLoop()
	
	return nil
}

// Stop gracefully shuts down the batcher
// FUNCTIONAL DISCOVERY: Final flush ensures no messages are lost during shutdown
func (b *MessageBatcher) Stop() error {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return ErrBatcherNotStarted
	}
	b.running = false
	b.mu.Unlock()
	
	// Signal shutdown
	close(b.stopCh)
	
	// Wait for processing to complete
	select {
	case <-b.doneCh:
		// Final flush of any remaining messages
		b.mu.Lock()
		if len(b.messages) > 0 {
			b.flushBatch()
		}
		b.mu.Unlock()
		
	case <-time.After(5 * time.Second):
		// TECHNICAL DISCOVERY: Timeout prevents hanging on shutdown
		return errors.New("batcher stop timeout")
	}
	
	// Cancel context
	b.cancel()
	
	return nil
}

// Add adds a message to the batch
// FUNCTIONAL DISCOVERY: Non-blocking add with queue full detection
func (b *MessageBatcher) Add(msg *types.Message) error {
	if !b.running {
		return ErrBatcherStopped
	}
	
	b.totalMessages.Add(1)
	
	// Check queue capacity before attempting to add
	if b.queuedMessages.Load() >= int64(b.queueSize) {
		b.droppedMessages.Add(1)
		return ErrQueueFull
	}
	
	select {
	case b.messageCh <- msg:
		b.queuedMessages.Add(1)
		return nil
	default:
		// Channel is full, should not happen with proper queue tracking
		b.droppedMessages.Add(1)
		return ErrQueueFull
	}
}

// GetMetrics returns current batcher metrics
func (b *MessageBatcher) GetMetrics() BatchMetrics {
	b.mu.Lock()
	inMemoryBatch := len(b.messages)
	b.mu.Unlock()
	
	return BatchMetrics{
		QueueDepth:      int(b.queuedMessages.Load()) + inMemoryBatch,
		TotalMessages:   b.totalMessages.Load(),
		BatchCount:      b.batchCount.Load(),
		DroppedMessages: b.droppedMessages.Load(),
	}
}

// processingLoop is the main batch processing goroutine
// ARCHITECTURAL DISCOVERY: Single loop handles both time and size triggers
func (b *MessageBatcher) processingLoop() {
	defer close(b.doneCh)
	
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case msg := <-b.messageCh:
			b.queuedMessages.Add(-1) // Decrement queue counter when message is processed
			b.mu.Lock()
			b.messages = append(b.messages, msg)
			
			// FUNCTIONAL DISCOVERY: Size trigger takes precedence over time trigger
			if len(b.messages) >= b.batchSize {
				b.flushBatch()
			}
			b.mu.Unlock()
			
		case <-ticker.C:
			// TECHNICAL DISCOVERY: Time-based flush for low-traffic periods
			b.mu.Lock()
			if len(b.messages) > 0 {
				b.flushBatch()
			}
			b.mu.Unlock()
			
		case <-b.flushCh:
			// Manual flush request
			b.mu.Lock()
			if len(b.messages) > 0 {
				b.flushBatch()
			}
			b.mu.Unlock()
			
		case <-b.stopCh:
			// Drain remaining messages
			for {
				select {
				case msg := <-b.messageCh:
					b.queuedMessages.Add(-1) // Decrement queue counter when message is processed
					b.mu.Lock()
					b.messages = append(b.messages, msg)
					b.mu.Unlock()
				default:
					return
				}
			}
			
		case <-b.ctx.Done():
			return
		}
	}
}

// flushBatch writes the current batch to storage
// TECHNICAL DISCOVERY: Must be called with mutex held
func (b *MessageBatcher) flushBatch() {
	if len(b.messages) == 0 {
		return
	}
	
	// Copy messages for persistence
	batch := make([]*types.Message, len(b.messages))
	copy(batch, b.messages)
	
	// Clear the buffer
	b.messages = b.messages[:0]
	
	// Release lock during persistence
	b.mu.Unlock()
	
	// FUNCTIONAL DISCOVERY: Context timeout prevents hanging on slow DB
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	
	// Persist batch
	if err := b.store.StoreMessageBatch(ctx, batch); err != nil {
		// TECHNICAL DISCOVERY: Log but don't fail - messages are already delivered
		log.Printf("Failed to persist batch of %d messages: %v", len(batch), err)
		b.droppedMessages.Add(uint64(len(batch)))
	} else {
		b.batchCount.Add(1)
	}
	
	// Re-acquire lock
	b.mu.Lock()
}