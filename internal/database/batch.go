package database

import (
	"fmt"
	"log"
	"sync"
	"time"

	"switchboard/pkg/config"
)

// MessageBatcher handles batching of messages for efficient database writes
type MessageBatcher struct {
	batch        []*Message
	batchMu      sync.Mutex
	flushTimer   *time.Timer
	batchHandler func([]*Message)
	stopCh       chan struct{}
	wg           sync.WaitGroup
	started      bool
	startedMu    sync.Mutex
}

// NewMessageBatcher creates a new message batcher
func NewMessageBatcher() (*MessageBatcher, error) {
	return &MessageBatcher{
		batch:  make([]*Message, 0, config.DatabaseBatchSize),
		stopCh: make(chan struct{}),
	}, nil
}

// Start initializes the batcher and starts background processing
func (b *MessageBatcher) Start() error {
	b.startedMu.Lock()
	defer b.startedMu.Unlock()

	if b.started {
		return nil // Already started (idempotent)
	}

	b.started = true
	log.Printf("MessageBatcher started with batch size %d and flush interval %v", 
		config.DatabaseBatchSize, config.DatabaseFlushInterval)
	
	return nil
}

// Stop gracefully shuts down the batcher, flushing any pending messages
func (b *MessageBatcher) Stop() error {
	b.startedMu.Lock()
	defer b.startedMu.Unlock()

	if !b.started {
		return nil // Already stopped (idempotent)
	}

	b.started = false

	// Flush any pending messages before stopping
	b.batchMu.Lock()
	if len(b.batch) > 0 && b.batchHandler != nil {
		b.batchHandler(b.batch)
		b.batch = b.batch[:0] // Clear the batch
	}
	if b.flushTimer != nil {
		b.flushTimer.Stop()
		b.flushTimer = nil
	}
	b.batchMu.Unlock()

	// Signal shutdown and wait for goroutines
	close(b.stopCh)
	b.wg.Wait()

	log.Printf("MessageBatcher stopped gracefully")
	return nil
}

// AddMessage adds a message to the current batch
func (b *MessageBatcher) AddMessage(msg *Message) error {
	return b.addMessage(msg)
}

// addMessage is the internal implementation for adding messages to batch
func (b *MessageBatcher) addMessage(msg *Message) error {
	if msg == nil {
		return nil
	}

	b.batchMu.Lock()
	defer b.batchMu.Unlock()

	// Add message to batch
	b.batch = append(b.batch, msg)

	// Check if we need to flush due to size limit
	if len(b.batch) >= config.DatabaseBatchSize {
		b.flushBatch()
		return nil
	}

	// Start or reset the flush timer
	if b.flushTimer != nil {
		b.flushTimer.Stop()
	}
	b.flushTimer = time.AfterFunc(config.DatabaseFlushInterval, func() {
		b.batchMu.Lock()
		defer b.batchMu.Unlock()
		if len(b.batch) > 0 {
			b.flushBatch()
		}
	})

	return nil
}

// SetBatchHandler sets the function to call when a batch is ready to be processed
func (b *MessageBatcher) SetBatchHandler(handler func([]*Message)) {
	b.batchMu.Lock()
	defer b.batchMu.Unlock()
	b.batchHandler = handler
}

// GetBatchSize returns the configured batch size
func (b *MessageBatcher) GetBatchSize() int {
	return config.DatabaseBatchSize
}

// GetFlushInterval returns the configured flush interval
func (b *MessageBatcher) GetFlushInterval() time.Duration {
	return config.DatabaseFlushInterval
}

// flushBatch sends the current batch to the handler and clears it
// Must be called with batchMu locked
func (b *MessageBatcher) flushBatch() {
	if len(b.batch) == 0 || b.batchHandler == nil {
		return
	}

	// Stop the timer if it's running
	if b.flushTimer != nil {
		b.flushTimer.Stop()
		b.flushTimer = nil
	}

	// Create a copy of the batch to send to the handler
	batchCopy := make([]*Message, len(b.batch))
	copy(batchCopy, b.batch)

	// Clear the current batch
	b.batch = b.batch[:0]

	// Send batch to handler (this may block, but we're already in a mutex)
	b.batchHandler(batchCopy)
}

// BatchingDatabaseManager wraps a DatabaseManager with automatic message batching
type BatchingDatabaseManager struct {
	underlying DatabaseManager
	batcher    *MessageBatcher
}

// NewBatchingDatabaseManager creates a new batching database manager
func NewBatchingDatabaseManager(underlying DatabaseManager) (*BatchingDatabaseManager, error) {
	if underlying == nil {
		return nil, fmt.Errorf("underlying database manager cannot be nil")
	}

	batcher, err := NewMessageBatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create message batcher: %w", err)
	}

	manager := &BatchingDatabaseManager{
		underlying: underlying,
		batcher:    batcher,
	}

	// Set up the batch handler to write batches to the underlying database
	batcher.SetBatchHandler(func(messages []*Message) {
		if err := underlying.WriteBatch(messages); err != nil {
			log.Printf("Failed to write message batch: %v", err)
		}
	})

	return manager, nil
}

// Start starts both the underlying database manager and the batcher
func (b *BatchingDatabaseManager) Start() error {
	if err := b.underlying.Start(); err != nil {
		return err
	}
	
	if err := b.batcher.Start(); err != nil {
		_ = b.underlying.Stop() // Clean up on failure
		return err
	}
	
	return nil
}

// Stop stops both the batcher and the underlying database manager
func (b *BatchingDatabaseManager) Stop() error {
	// Stop batcher first to flush pending messages
	if err := b.batcher.Stop(); err != nil {
		log.Printf("Error stopping batcher: %v", err)
	}
	
	// Then stop underlying database manager
	return b.underlying.Stop()
}

// CreateSession delegates to the underlying database manager
func (b *BatchingDatabaseManager) CreateSession(session *Session) error {
	return b.underlying.CreateSession(session)
}

// UpdateSession delegates to the underlying database manager
func (b *BatchingDatabaseManager) UpdateSession(session *Session) error {
	return b.underlying.UpdateSession(session)
}

// GetActiveSession delegates to the underlying database manager
func (b *BatchingDatabaseManager) GetActiveSession() (*Session, error) {
	return b.underlying.GetActiveSession()
}

// WriteMessage adds the message to the batch instead of writing immediately
func (b *BatchingDatabaseManager) WriteMessage(msg *Message) error {
	return b.batcher.AddMessage(msg)
}

// WriteBatch writes the batch immediately to the underlying database
func (b *BatchingDatabaseManager) WriteBatch(msgs []*Message) error {
	return b.underlying.WriteBatch(msgs)
}

// GetSessionMessages delegates to the underlying database manager
func (b *BatchingDatabaseManager) GetSessionMessages(sessionID string) ([]*Message, error) {
	return b.underlying.GetSessionMessages(sessionID)
}