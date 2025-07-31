package unit

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/pkg/config"
)

func TestMessageBatcherLifecycle(t *testing.T) {
	t.Run("NewMessageBatcher creates valid instance", func(t *testing.T) {
		batcher, err := database.NewMessageBatcher()
		if err != nil {
			t.Skip("MessageBatcher not implemented yet")
		}

		assert.NotNil(t, batcher, "NewMessageBatcher must return non-nil batcher")
	})

	t.Run("Start initializes batcher goroutines", func(t *testing.T) {
		batcher, err := database.NewMessageBatcher()
		if err != nil {
			t.Skip("MessageBatcher not implemented yet")
		}

		err = batcher.Start()
		assert.NoError(t, err, "Start must not return error")

		// Clean up
		err = batcher.Stop()
		assert.NoError(t, err, "Stop must not return error after Start")
	})

	t.Run("Stop gracefully shuts down batcher", func(t *testing.T) {
		batcher, err := database.NewMessageBatcher()
		if err != nil {
			t.Skip("MessageBatcher not implemented yet")
		}

		err = batcher.Start()
		require.NoError(t, err)

		start := time.Now()
		err = batcher.Stop()
		duration := time.Since(start)

		assert.NoError(t, err, "Stop must not return error")
		assert.Less(t, duration, 2*time.Second, "Stop must complete within reasonable time")
	})
}

func TestMessageBatchingSizeLimit(t *testing.T) {
	t.Run("Batch flushes when reaching DatabaseBatchSize", func(t *testing.T) {
		batcher, err := database.NewMessageBatcher()
		if err != nil {
			t.Skip("MessageBatcher not implemented yet")
		}

		err = batcher.Start()
		require.NoError(t, err)
		defer func() { _ = batcher.Stop() }()

		// Channel to receive batches
		batchChan := make(chan []*database.Message, 1)
		batcher.SetBatchHandler(func(messages []*database.Message) {
			batchChan <- messages
		})

		// Add messages to reach batch size
		messages := make([]*database.Message, config.DatabaseBatchSize)
		for i := 0; i < config.DatabaseBatchSize; i++ {
			messages[i] = &database.Message{
				ID:        fmt.Sprintf("msg-%d", i),
				SessionID: "session-1",
				Type:      database.MessageTypeBroadcastToInstructors,
				Context:   database.ContextQuestion,
				FromUser:  fmt.Sprintf("student-%d", i),
				Content:   map[string]interface{}{"text": fmt.Sprintf("Message %d", i)},
				Timestamp: time.Now(),
			}

			err = batcher.AddMessage(messages[i])
			require.NoError(t, err)
		}

		// Wait for batch to be flushed
		select {
		case batch := <-batchChan:
			assert.Len(t, batch, config.DatabaseBatchSize, 
				"Batch must contain exactly DatabaseBatchSize messages")
		case <-time.After(1 * time.Second):
			t.Fatal("Batch should have been flushed immediately when reaching size limit")
		}
	})

	t.Run("Partial batch stays pending until size limit reached", func(t *testing.T) {
		batcher, err := database.NewMessageBatcher()
		if err != nil {
			t.Skip("MessageBatcher not implemented yet")
		}

		err = batcher.Start()
		require.NoError(t, err)
		defer func() { _ = batcher.Stop() }()

		// Channel to receive batches
		batchChan := make(chan []*database.Message, 1)
		batcher.SetBatchHandler(func(messages []*database.Message) {
			batchChan <- messages
		})

		// Add fewer messages than batch size
		partialSize := config.DatabaseBatchSize / 2
		for i := 0; i < partialSize; i++ {
			message := &database.Message{
				ID:        fmt.Sprintf("msg-%d", i),
				SessionID: "session-1",
				Type:      database.MessageTypeBroadcastToInstructors,
				Context:   database.ContextQuestion,
				FromUser:  fmt.Sprintf("student-%d", i),
				Content:   map[string]interface{}{"text": fmt.Sprintf("Message %d", i)},
				Timestamp: time.Now(),
			}

			err = batcher.AddMessage(message)
			require.NoError(t, err)
		}

		// Wait briefly - batch should not be flushed yet
		select {
		case <-batchChan:
			t.Fatal("Partial batch should not be flushed immediately")
		case <-time.After(50 * time.Millisecond):
			// Expected - no batch flushed yet
		}
	})
}

func TestMessageBatchingTimeLimit(t *testing.T) {
	t.Run("Batch flushes after DatabaseFlushInterval timeout", func(t *testing.T) {
		batcher, err := database.NewMessageBatcher()
		if err != nil {
			t.Skip("MessageBatcher not implemented yet")
		}

		err = batcher.Start()
		require.NoError(t, err)
		defer func() { _ = batcher.Stop() }()

		// Channel to receive batches
		batchChan := make(chan []*database.Message, 1)
		batcher.SetBatchHandler(func(messages []*database.Message) {
			batchChan <- messages
		})

		// Add a single message
		message := &database.Message{
			ID:        "msg-1",
			SessionID: "session-1",
			Type:      database.MessageTypeBroadcastToInstructors,
			Context:   database.ContextQuestion,
			FromUser:  "student-1",
			Content:   map[string]interface{}{"text": "Test message"},
			Timestamp: time.Now(),
		}

		start := time.Now()
		err = batcher.AddMessage(message)
		require.NoError(t, err)

		// Wait for timeout-based flush
		select {
		case batch := <-batchChan:
			elapsed := time.Since(start)
			assert.Len(t, batch, 1, "Batch must contain the single message")
			assert.GreaterOrEqual(t, elapsed, config.DatabaseFlushInterval, 
				"Batch should be flushed after DatabaseFlushInterval")
			assert.Less(t, elapsed, config.DatabaseFlushInterval+100*time.Millisecond, 
				"Batch should be flushed promptly after timeout")
		case <-time.After(config.DatabaseFlushInterval + 500*time.Millisecond):
			t.Fatal("Batch should have been flushed after DatabaseFlushInterval timeout")
		}
	})

	t.Run("Timer resets with new messages", func(t *testing.T) {
		batcher, err := database.NewMessageBatcher()
		if err != nil {
			t.Skip("MessageBatcher not implemented yet")
		}

		err = batcher.Start()
		require.NoError(t, err)
		defer func() { _ = batcher.Stop() }()

		// Channel to receive batches
		batchChan := make(chan []*database.Message, 1)
		batcher.SetBatchHandler(func(messages []*database.Message) {
			batchChan <- messages
		})

		// Add first message
		message1 := &database.Message{
			ID:        "msg-1",
			SessionID: "session-1",
			Type:      database.MessageTypeBroadcastToInstructors,
			Context:   database.ContextQuestion,
			FromUser:  "student-1",
			Content:   map[string]interface{}{"text": "Message 1"},
			Timestamp: time.Now(),
		}

		start := time.Now()
		err = batcher.AddMessage(message1)
		require.NoError(t, err)

		// Add second message halfway through flush interval
		time.Sleep(config.DatabaseFlushInterval / 2)
		message2 := &database.Message{
			ID:        "msg-2",
			SessionID: "session-1",
			Type:      database.MessageTypeBroadcastToInstructors,
			Context:   database.ContextQuestion,
			FromUser:  "student-2",
			Content:   map[string]interface{}{"text": "Message 2"},
			Timestamp: time.Now(),
		}

		err = batcher.AddMessage(message2)
		require.NoError(t, err)

		// Wait for batch to be flushed
		select {
		case batch := <-batchChan:
			elapsed := time.Since(start)
			assert.Len(t, batch, 2, "Batch must contain both messages")
			// Should take longer than initial interval because timer was reset
			assert.GreaterOrEqual(t, elapsed, config.DatabaseFlushInterval+config.DatabaseFlushInterval/2, 
				"Timer should reset with new message")
		case <-time.After(config.DatabaseFlushInterval*2 + 500*time.Millisecond):
			t.Fatal("Batch should have been flushed after timer reset")
		}
	})
}

func TestMessageBatchingEdgeCases(t *testing.T) {
	t.Run("Empty batches are not flushed", func(t *testing.T) {
		batcher, err := database.NewMessageBatcher()
		if err != nil {
			t.Skip("MessageBatcher not implemented yet")
		}

		err = batcher.Start()
		require.NoError(t, err)
		defer func() { _ = batcher.Stop() }()

		// Channel to receive batches
		batchChan := make(chan []*database.Message, 1)
		batcher.SetBatchHandler(func(messages []*database.Message) {
			batchChan <- messages
		})

		// Wait longer than flush interval without adding messages
		select {
		case batch := <-batchChan:
			t.Fatalf("Empty batch should not be flushed, got batch with %d messages", len(batch))
		case <-time.After(config.DatabaseFlushInterval + 100*time.Millisecond):
			// Expected - no empty batch flushed
		}
	})

	t.Run("Concurrent AddMessage calls are safe", func(t *testing.T) {
		batcher, err := database.NewMessageBatcher()
		if err != nil {
			t.Skip("MessageBatcher not implemented yet")
		}

		err = batcher.Start()
		require.NoError(t, err)
		defer func() { _ = batcher.Stop() }()

		// Channel to receive batches
		batchChan := make(chan []*database.Message, 10) // Buffer for multiple batches
		batcher.SetBatchHandler(func(messages []*database.Message) {
			batchChan <- messages
		})

		// Concurrent workers adding messages
		const numWorkers = 10
		const messagesPerWorker = 10
		errChan := make(chan error, numWorkers*messagesPerWorker)

		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				for j := 0; j < messagesPerWorker; j++ {
					message := &database.Message{
						ID:        fmt.Sprintf("msg-%d-%d", workerID, j),
						SessionID: "session-1",
						Type:      database.MessageTypeBroadcastToInstructors,
						Context:   database.ContextQuestion,
						FromUser:  fmt.Sprintf("student-%d", workerID),
						Content:   map[string]interface{}{"text": fmt.Sprintf("Message %d from worker %d", j, workerID)},
						Timestamp: time.Now(),
					}

					err := batcher.AddMessage(message)
					errChan <- err
				}
			}(i)
		}

		// Collect all errors
		var errors []error
		for i := 0; i < numWorkers*messagesPerWorker; i++ {
			if err := <-errChan; err != nil {
				errors = append(errors, err)
			}
		}

		assert.Empty(t, errors, "Concurrent AddMessage calls must not produce errors")

		// Wait for all batches to be processed
		time.Sleep(config.DatabaseFlushInterval + 100*time.Millisecond)

		// Count total messages in all batches
		totalMessages := 0
		timeout := time.After(1 * time.Second)
		for {
			select {
			case batch := <-batchChan:
				totalMessages += len(batch)
			case <-timeout:
				goto done
			default:
				time.Sleep(10 * time.Millisecond)
			}
		}
		done:

		assert.Equal(t, numWorkers*messagesPerWorker, totalMessages, 
			"All messages must be processed through batches")
	})

	t.Run("Stop flushes pending messages", func(t *testing.T) {
		batcher, err := database.NewMessageBatcher()
		if err != nil {
			t.Skip("MessageBatcher not implemented yet")
		}

		err = batcher.Start()
		require.NoError(t, err)

		// Channel to receive batches
		batchChan := make(chan []*database.Message, 1)
		batcher.SetBatchHandler(func(messages []*database.Message) {
			batchChan <- messages
		})

		// Add a single message (won't trigger size-based flush)
		message := &database.Message{
			ID:        "msg-1",
			SessionID: "session-1",
			Type:      database.MessageTypeBroadcastToInstructors,
			Context:   database.ContextQuestion,
			FromUser:  "student-1",
			Content:   map[string]interface{}{"text": "Test message"},
			Timestamp: time.Now(),
		}

		err = batcher.AddMessage(message)
		require.NoError(t, err)

		// Stop immediately (before timeout)
		err = batcher.Stop()
		assert.NoError(t, err)

		// Should receive the pending message
		select {
		case batch := <-batchChan:
			assert.Len(t, batch, 1, "Stop must flush pending messages")
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Stop should flush pending messages")
		}
	})
}

func TestMessageBatchingConfiguration(t *testing.T) {
	t.Run("Batcher uses correct configuration constants", func(t *testing.T) {
		batcher, err := database.NewMessageBatcher()
		if err != nil {
			t.Skip("MessageBatcher not implemented yet")
		}

		// Verify configuration is applied
		batchSize := batcher.GetBatchSize()
		flushInterval := batcher.GetFlushInterval()

		assert.Equal(t, config.DatabaseBatchSize, batchSize, 
			"Batcher must use DatabaseBatchSize constant")
		assert.Equal(t, config.DatabaseFlushInterval, flushInterval, 
			"Batcher must use DatabaseFlushInterval constant")
	})
}