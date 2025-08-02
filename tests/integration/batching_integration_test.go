package integration

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	"switchboard/internal/websocket"
	"switchboard/pkg/config"
)

// BatchingMockBroadcastSystem tracks when broadcasts happen during batching tests
type BatchingMockBroadcastSystem struct {
	mu               sync.Mutex
	broadcastedMsgs  []string
	broadcastTimes   []time.Time
}

func (m *BatchingMockBroadcastSystem) BroadcastMessage(msg *database.Message, recipients []message.Recipient) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.broadcastedMsgs = append(m.broadcastedMsgs, msg.ID)
	m.broadcastTimes = append(m.broadcastTimes, time.Now())
	return nil
}

func (m *BatchingMockBroadcastSystem) GetBroadcastCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.broadcastedMsgs)
}

func (m *BatchingMockBroadcastSystem) GetBroadcastTimes() []time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	times := make([]time.Time, len(m.broadcastTimes))
	copy(times, m.broadcastTimes)
	return times
}

// TestBatchingIntegration verifies end-to-end message flow with batching
func TestBatchingIntegration(t *testing.T) {
	t.Run("broadcast happens before batch persistence", func(t *testing.T) {
		// Setup components
		db := setupIntegrationDatabase(t)
		dbManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
			SkipTableInit: true,
			SkipPragmas:   true,
		})
		require.NoError(t, err)
		
		err = dbManager.Start()
		require.NoError(t, err)
		defer dbManager.Stop()

		sessionManager := session.NewSessionManager(dbManager)
		connectionRegistry := websocket.NewConnectionRegistry(sessionManager)
		rateLimiter := rate.NewRateLimiter()
		
		// Use mock broadcast system to track timing
		mockBroadcast := &BatchingMockBroadcastSystem{}
		
		// Create message processor
		roleFilter := &message.RoleBasedFilter{}
		router := message.NewMessageRouter(connectionRegistry, roleFilter)
		processor := message.NewMessageProcessor(
			sessionManager,
			dbManager,
			rateLimiter,
			router,
			mockBroadcast,
		)

		// Start session
		testSession := &database.Session{
			ID:        "test-broadcast-timing",
			Name:      "Broadcast Timing Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		err = sessionManager.SetActiveSession(testSession)
		require.NoError(t, err)
		err = dbManager.CreateSession(testSession)
		require.NoError(t, err)

		// Process messages to fill a batch
		messageCount := config.DatabaseBatchSize
		startTime := time.Now()
		
		for i := 0; i < messageCount; i++ {
			msgData := map[string]interface{}{
				"type":    database.MessageTypeBroadcastToStudents,
				"content": map[string]interface{}{"text": fmt.Sprintf("msg_%d", i)},
			}
			rawData, _ := json.Marshal(msgData)
			
			err := processor.ProcessIncomingMessage(rawData, fmt.Sprintf("user_%d", i))
			require.NoError(t, err)
		}

		// Get broadcast times
		broadcastTimes := mockBroadcast.GetBroadcastTimes()
		assert.Equal(t, messageCount, len(broadcastTimes))

		// Broadcasts should happen immediately (within 10ms of processing)
		for _, broadcastTime := range broadcastTimes {
			assert.Less(t, broadcastTime.Sub(startTime), 10*time.Millisecond,
				"Broadcast should happen immediately, not wait for batch")
		}

		// But database writes should be batched - check they're not written immediately
		immediateMessages, err := dbManager.GetSessionMessages(testSession.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, len(immediateMessages), 
			"Messages should not be in database immediately (they're batched)")

		// Wait for batch to be written
		err = dbManager.WaitForPendingWrites()
		require.NoError(t, err)

		// Now all messages should be persisted
		persistedMessages, err := dbManager.GetSessionMessages(testSession.ID)
		require.NoError(t, err)
		assert.Equal(t, messageCount, len(persistedMessages))
	})

	t.Run("system handles mixed operations with batching", func(t *testing.T) {
		// Setup components
		db := setupIntegrationDatabase(t)
		dbManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
			SkipTableInit: true,
			SkipPragmas:   true,
		})
		require.NoError(t, err)
		
		err = dbManager.Start()
		require.NoError(t, err)
		defer dbManager.Stop()

		sessionManager := session.NewSessionManager(dbManager)
		sessionLifecycle := session.NewSessionLifecycle(sessionManager, dbManager)
		connectionRegistry := websocket.NewConnectionRegistry(sessionManager)
		rateLimiter := rate.NewRateLimiter()
		
		mockBroadcast := &BatchingMockBroadcastSystem{}
		roleFilter := &message.RoleBasedFilter{}
		router := message.NewMessageRouter(connectionRegistry, roleFilter)
		processor := message.NewMessageProcessor(
			sessionManager,
			dbManager,
			rateLimiter,
			router,
			mockBroadcast,
		)

		// Start session via lifecycle (non-batchable operation)
		session1, err := sessionLifecycle.StartSession("Mixed Ops Test", "instructor1")
		require.NoError(t, err)
		assert.NotNil(t, session1)

		// Send some messages (batchable)
		for i := 0; i < 5; i++ {
			msgData := map[string]interface{}{
				"type":    database.MessageTypeBroadcastToInstructors,
				"content": map[string]interface{}{"text": fmt.Sprintf("question_%d", i)},
			}
			rawData, _ := json.Marshal(msgData)
			
			err := processor.ProcessIncomingMessage(rawData, fmt.Sprintf("student_%d", i))
			require.NoError(t, err)
		}

		// End session (non-batchable operation)
		endedSession, err := sessionLifecycle.EndSession("instructor1")
		require.NoError(t, err)
		require.NotNil(t, endedSession)

		// Start new session (non-batchable operation)
		session2, err := sessionLifecycle.StartSession("Second Session", "instructor2")
		require.NoError(t, err)

		// Send more messages to new session
		for i := 5; i < 10; i++ {
			msgData := map[string]interface{}{
				"type":    database.MessageTypeBroadcastToStudents,
				"content": map[string]interface{}{"text": fmt.Sprintf("announcement_%d", i)},
			}
			rawData, _ := json.Marshal(msgData)
			
			err := processor.ProcessIncomingMessage(rawData, "instructor2")
			require.NoError(t, err)
		}

		// Force batch flush
		err = dbManager.WaitForPendingWrites()
		require.NoError(t, err)

		// Verify all operations succeeded
		msgs1, err := dbManager.GetSessionMessages(endedSession.ID)
		require.NoError(t, err)
		assert.Equal(t, 5, len(msgs1))

		msgs2, err := dbManager.GetSessionMessages(session2.ID)
		require.NoError(t, err)
		assert.Equal(t, 5, len(msgs2))

		activeSession, err := dbManager.GetActiveSession()
		require.NoError(t, err)
		assert.Equal(t, session2.ID, activeSession.ID)
	})

	t.Run("high throughput with batching maintains low latency", func(t *testing.T) {
		// Setup components
		db := setupIntegrationDatabase(t)
		dbManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
			SkipTableInit: true,
			SkipPragmas:   true,
		})
		require.NoError(t, err)
		
		err = dbManager.Start()
		require.NoError(t, err)
		defer dbManager.Stop()

		sessionManager := session.NewSessionManager(dbManager)
		connectionRegistry := websocket.NewConnectionRegistry(sessionManager)
		rateLimiter := rate.NewRateLimiter()
		
		mockBroadcast := &BatchingMockBroadcastSystem{}
		roleFilter := &message.RoleBasedFilter{}
		router := message.NewMessageRouter(connectionRegistry, roleFilter)
		processor := message.NewMessageProcessor(
			sessionManager,
			dbManager,
			rateLimiter,
			router,
			mockBroadcast,
		)

		// Start session
		testSession := &database.Session{
			ID:        "test-throughput",
			Name:      "Throughput Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		err = sessionManager.SetActiveSession(testSession)
		require.NoError(t, err)
		err = dbManager.CreateSession(testSession)
		require.NoError(t, err)

		// Simulate 10 messages/second for 2 seconds (educational workload)
		messageRate := 10
		duration := 2 * time.Second
		totalMessages := int(duration.Seconds()) * messageRate

		var wg sync.WaitGroup
		latencies := make([]time.Duration, totalMessages)
		errors := make([]error, totalMessages)

		ticker := time.NewTicker(time.Second / time.Duration(messageRate))
		defer ticker.Stop()

		for i := 0; i < totalMessages; i++ {
			<-ticker.C
			
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				
				msgStart := time.Now()
				msgData := map[string]interface{}{
					"type":    database.MessageTypeBroadcastToStudents,
					"content": map[string]interface{}{
						"text": fmt.Sprintf("snapshot_%d", idx),
						"code": "function main() { /* student code */ }",
					},
				}
				rawData, _ := json.Marshal(msgData)
				
				errors[idx] = processor.ProcessIncomingMessage(rawData, fmt.Sprintf("student_%d", idx%100))
				latencies[idx] = time.Since(msgStart)
			}(i)
		}

		// Wait for all messages to be processed
		wg.Wait()

		// Check for errors
		for i, err := range errors {
			assert.NoError(t, err, "Message %d should process without error", i)
		}

		// Analyze latencies
		var maxLatency time.Duration
		var totalLatency time.Duration
		for _, latency := range latencies {
			totalLatency += latency
			if latency > maxLatency {
				maxLatency = latency
			}
		}
		avgLatency := totalLatency / time.Duration(len(latencies))

		// Verify performance meets requirements
		assert.Less(t, maxLatency, 50*time.Millisecond, 
			"Max latency should be under 50ms for real-time feel")
		assert.Less(t, avgLatency, 10*time.Millisecond,
			"Average latency should be under 10ms")

		// Verify all messages were broadcasted immediately
		assert.Equal(t, totalMessages, mockBroadcast.GetBroadcastCount())

		// Wait for all batches to be written
		err = dbManager.WaitForPendingWrites()
		require.NoError(t, err)

		// Verify all messages persisted
		messages, err := dbManager.GetSessionMessages(testSession.ID)
		require.NoError(t, err)
		assert.Equal(t, totalMessages, len(messages))
	})
}

func setupIntegrationDatabase(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Apply full schema
	schema := `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL CHECK (length(name) >= 1 AND length(name) <= 200),
			created_by TEXT NOT NULL,
			start_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			end_time DATETIME,
			status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'ended'))
		);
		
		CREATE TABLE messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			type TEXT NOT NULL CHECK (type IN ('broadcast_to_instructors', 'direct_message', 'broadcast_to_students')),
			context TEXT NOT NULL DEFAULT 'general' CHECK (length(context) >= 1 AND length(context) <= 50),
			from_user TEXT NOT NULL CHECK (length(from_user) >= 1 AND length(from_user) <= 50),
			to_user TEXT,
			content TEXT NOT NULL CHECK (length(content) <= 65536),
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);
		
		CREATE INDEX idx_sessions_status ON sessions(status);
		CREATE INDEX idx_sessions_start_time ON sessions(start_time DESC);
		CREATE INDEX idx_messages_session_time ON messages(session_id, timestamp);
		CREATE INDEX idx_messages_type_context ON messages(type, context);
		CREATE INDEX idx_messages_to_user ON messages(to_user) WHERE to_user IS NOT NULL;
		CREATE UNIQUE INDEX idx_unique_active_session ON sessions(status) WHERE status = 'active';
	`
	
	_, err = db.Exec(schema)
	require.NoError(t, err)

	// Apply pragmas for performance
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -64000",
		"PRAGMA wal_autocheckpoint = 1000",
	}
	
	for _, pragma := range pragmas {
		_, err = db.Exec(pragma)
		require.NoError(t, err)
	}

	return db
}