package unit

import (
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/pkg/config"
)

// TestSQLiteBatching verifies that the SQLiteDatabaseManager correctly batches messages
func TestSQLiteBatching(t *testing.T) {
	t.Run("messages are batched up to DatabaseBatchSize", func(t *testing.T) {
		db := setupTestDatabase(t)
		dbManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
			SkipTableInit: true,
			SkipPragmas:   true,
		})
		require.NoError(t, err)

		err = dbManager.Start()
		require.NoError(t, err)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Error stopping dbManager: %v", err)
			}
		}()

		// Create test session
		session := createTestSession(t, dbManager)

		// Send messages concurrently so they can be batched together
		messageCount := config.DatabaseBatchSize
		var wg sync.WaitGroup
		
		for i := 0; i < messageCount; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				msg := createTestMessage(session.ID, fmt.Sprintf("msg_%d", idx))
				err := dbManager.WriteMessage(msg)
				require.NoError(t, err)
			}(i)
		}

		// Wait for all messages to be sent
		wg.Wait()

		// Wait for batch to be written
		err = dbManager.WaitForPendingWrites()
		require.NoError(t, err)

		// Verify all messages are now written
		messages, err := dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Equal(t, config.DatabaseBatchSize, len(messages))
	})

	t.Run("batches flush after DatabaseFlushInterval", func(t *testing.T) {
		db := setupTestDatabase(t)
		dbManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
			SkipTableInit: true,
			SkipPragmas:   true,
		})
		require.NoError(t, err)

		err = dbManager.Start()
		require.NoError(t, err)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Error stopping dbManager: %v", err)
			}
		}()

		// Create test session
		session := createTestSession(t, dbManager)

		// Send a few messages concurrently (less than batch size)
		messageCount := 5
		var wg sync.WaitGroup
		
		for i := 0; i < messageCount; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				msg := createTestMessage(session.ID, fmt.Sprintf("timed_%d", idx))
				err := dbManager.WriteMessage(msg)
				require.NoError(t, err)
			}(i)
		}

		// Wait for all messages to be sent
		wg.Wait()

		// Wait for flush interval plus buffer
		time.Sleep(config.DatabaseFlushInterval + 100*time.Millisecond)

		// Messages should now be flushed by timer
		messages, err := dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Equal(t, messageCount, len(messages))
	})

	t.Run("responses are sent after batch commit", func(t *testing.T) {
		db := setupTestDatabase(t)
		dbManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
			SkipTableInit: true,
			SkipPragmas:   true,
		})
		require.NoError(t, err)

		err = dbManager.Start()
		require.NoError(t, err)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Error stopping dbManager: %v", err)
			}
		}()

		// Create test session
		session := createTestSession(t, dbManager)

		// Send messages and track response times
		var wg sync.WaitGroup
		responseTimes := make([]time.Time, config.DatabaseBatchSize)
		startTime := time.Now()

		for i := 0; i < config.DatabaseBatchSize; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				msg := createTestMessage(session.ID, fmt.Sprintf("resp_%d", idx))
				err := dbManager.WriteMessage(msg)
				assert.NoError(t, err)
				responseTimes[idx] = time.Now()
			}(i)
		}

		// Wait for all responses
		wg.Wait()

		// All responses should come at approximately the same time (after batch commit)
		minTime := responseTimes[0]
		maxTime := responseTimes[0]
		for _, t := range responseTimes {
			if t.Before(minTime) {
				minTime = t
			}
			if t.After(maxTime) {
				maxTime = t
			}
		}

		// Responses should be clustered together (within 50ms)
		assert.Less(t, maxTime.Sub(minTime), 50*time.Millisecond, 
			"All responses should arrive close together after batch commit")
		
		// Total time should be less than flush interval
		assert.Less(t, time.Since(startTime), config.DatabaseFlushInterval,
			"Batch should flush when full, not wait for timer")
	})

	t.Run("graceful shutdown flushes pending batches", func(t *testing.T) {
		db := setupTestDatabase(t)
		dbManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
			SkipTableInit: true,
			SkipPragmas:   true,
		})
		require.NoError(t, err)

		err = dbManager.Start()
		require.NoError(t, err)

		// Create test session
		session := createTestSession(t, dbManager)

		// Send messages but don't fill the batch
		messageCount := 3
		for i := 0; i < messageCount; i++ {
			msg := createTestMessage(session.ID, fmt.Sprintf("shutdown_%d", i))
			err := dbManager.WriteMessage(msg)
			require.NoError(t, err)
		}

		// Stop the manager (should flush pending messages)
		err = dbManager.Stop()
		require.NoError(t, err)

		// Verify messages were flushed during shutdown
		messages, err := dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Equal(t, messageCount, len(messages))
	})

	t.Run("batch error handling propagates to all messages", func(t *testing.T) {
		db := setupTestDatabase(t)
		dbManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
			SkipTableInit: true,
			SkipPragmas:   true,
		})
		require.NoError(t, err)

		err = dbManager.Start()
		require.NoError(t, err)
		defer func() {
			if err := dbManager.Stop(); err != nil {
				t.Logf("Error stopping dbManager: %v", err)
			}
		}()

		// Create test session
		session := createTestSession(t, dbManager)

		// Close the database to force write errors
		if err := db.Close(); err != nil {
			t.Logf("Error closing database: %v", err)
		}

		// Send messages and collect errors
		var wg sync.WaitGroup
		errors := make([]error, config.DatabaseBatchSize)

		for i := 0; i < config.DatabaseBatchSize; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				msg := createTestMessage(session.ID, fmt.Sprintf("error_%d", idx))
				errors[idx] = dbManager.WriteMessage(msg)
			}(i)
		}

		// Wait for all responses
		wg.Wait()

		// All messages in the batch should receive the same error
		for i, err := range errors {
			assert.Error(t, err, "Message %d should have received an error", i)
		}
	})

}

// Test helper functions

func setupTestDatabase(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Apply schema
	schema := `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_by TEXT NOT NULL,
			start_time DATETIME NOT NULL,
			end_time DATETIME,
			status TEXT NOT NULL DEFAULT 'active'
		);
		
		CREATE TABLE messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			type TEXT NOT NULL,
			context TEXT NOT NULL DEFAULT 'general',
			from_user TEXT NOT NULL,
			to_user TEXT,
			content TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		);
		
		CREATE UNIQUE INDEX idx_unique_active_session ON sessions(status) WHERE status = 'active';
	`
	_, err = db.Exec(schema)
	require.NoError(t, err)

	return db
}

func createTestSession(t *testing.T, dbManager database.DatabaseManager) *database.Session {
	session := &database.Session{
		ID:        "test-session-1",
		Name:      "Test Session",
		CreatedBy: "instructor1",
		StartTime: time.Now(),
		Status:    "active",
	}
	err := dbManager.CreateSession(session)
	require.NoError(t, err)
	return session
}

func createTestMessage(sessionID, content string) *database.Message {
	return &database.Message{
		ID:        fmt.Sprintf("msg-%s-%d", content, time.Now().UnixNano()),
		SessionID: sessionID,
		Type:      database.MessageTypeBroadcastToStudents,
		Context:   database.ContextGeneral,
		FromUser:  "test_user",
		Content:   map[string]interface{}{"text": content},
		Timestamp: time.Now(),
	}
}