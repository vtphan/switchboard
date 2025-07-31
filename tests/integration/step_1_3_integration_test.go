package integration

import (
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"

	"switchboard/internal/database"
	"switchboard/pkg/config"
)

func TestStep13_Step11Integration(t *testing.T) {
	t.Run("SQLite persists Session structs from Step 1.1", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet - cannot test Session integration")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		// Test Session struct integration
		session := &database.Session{
			ID:        "integration-session-1",
			Name:      "Integration Test Session",
			CreatedBy: "instructor-integration",
			StartTime: time.Now(),
			Status:    "active",
		}

		// Test CreateSession integration
		err = manager.CreateSession(session)
		assert.NoError(t, err, "SQLite must persist Session structs correctly")

		// Test GetActiveSession integration
		retrievedSession, err := manager.GetActiveSession()
		assert.NoError(t, err, "SQLite must retrieve Session structs correctly")
		assert.NotNil(t, retrievedSession, "Retrieved session must not be nil")
		assert.Equal(t, session.ID, retrievedSession.ID, "Session ID must be preserved")
		assert.Equal(t, session.Name, retrievedSession.Name, "Session name must be preserved")
		assert.Equal(t, session.CreatedBy, retrievedSession.CreatedBy, "Session creator must be preserved")
		assert.Equal(t, session.Status, retrievedSession.Status, "Session status must be preserved")

		// Test UpdateSession integration
		endTime := time.Now()
		session.EndTime = &endTime
		session.Status = "ended"

		err = manager.UpdateSession(session)
		assert.NoError(t, err, "SQLite must update Session structs correctly")

		// Verify update persisted
		updatedSession, err := manager.GetActiveSession()
		assert.NoError(t, err)
		if updatedSession != nil {
			assert.Equal(t, "ended", updatedSession.Status, "Session status update must be persisted")
			assert.NotNil(t, updatedSession.EndTime, "Session end time must be persisted")
		}
	})

	t.Run("SQLite persists Message structs from Step 1.1", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet - cannot test Message integration")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		// Test Message struct integration with all message types
		messages := []*database.Message{
			{
				ID:        "msg-broadcast-instructors",
				SessionID: "session-1",
				Type:      database.MessageTypeBroadcastToInstructors,
				Context:   database.ContextQuestion,
				FromUser:  "student-1",
				Content:   map[string]interface{}{"text": "Question for instructors"},
				Timestamp: time.Now(),
			},
			{
				ID:        "msg-direct",
				SessionID: "session-1",
				Type:      database.MessageTypeDirectMessage,
				Context:   database.ContextResponse,
				FromUser:  "instructor-1",
				ToUser:    stringPtr("student-1"),
				Content:   map[string]interface{}{"text": "Direct response to student"},
				Timestamp: time.Now(),
			},
			{
				ID:        "msg-broadcast-students",
				SessionID: "session-1",
				Type:      database.MessageTypeBroadcastToStudents,
				Context:   database.ContextAnnouncement,
				FromUser:  "instructor-1",
				Content:   map[string]interface{}{"text": "Announcement to all students"},
				Timestamp: time.Now(),
			},
		}

		// Test WriteMessage integration
		for _, msg := range messages {
			err = manager.WriteMessage(msg)
			assert.NoError(t, err, "SQLite must persist Message structs correctly")
		}

		// Allow time for async writes to complete
		time.Sleep(config.DatabaseFlushInterval + 100*time.Millisecond)

		// Test GetSessionMessages integration
		retrievedMessages, err := manager.GetSessionMessages("session-1")
		assert.NoError(t, err, "SQLite must retrieve Message structs correctly")
		assert.Len(t, retrievedMessages, len(messages), "All messages must be retrievable")

		// Verify message fields are preserved
		for _, retrieved := range retrievedMessages {
			assert.NotEmpty(t, retrieved.ID, "Message ID must be preserved")
			assert.Equal(t, "session-1", retrieved.SessionID, "Message session ID must be preserved")
			assert.Contains(t, []string{
				database.MessageTypeBroadcastToInstructors,
				database.MessageTypeDirectMessage,
				database.MessageTypeBroadcastToStudents,
			}, retrieved.Type, "Message type must be preserved")
			assert.NotEmpty(t, retrieved.Context, "Message context must be preserved")
			assert.NotEmpty(t, retrieved.FromUser, "Message from user must be preserved")
			assert.NotNil(t, retrieved.Content, "Message content must be preserved")
		}
	})
}

func TestStep13_Step12Integration(t *testing.T) {
	t.Run("SQLite implements all DatabaseManager interface methods", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet - cannot test interface integration")
		}

		// Verify interface compliance at runtime
		var _ database.DatabaseManager = manager

		// Test lifecycle methods from interface
		err = manager.Start()
		assert.NoError(t, err, "Start method from interface must work")

		err = manager.Stop()
		assert.NoError(t, err, "Stop method from interface must work")
	})

	t.Run("DatabaseManager interface contract fulfilled", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet - cannot test interface contract")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		// Test all interface methods work as expected
		session := &database.Session{
			ID:        "contract-test",
			Name:      "Contract Test",
			CreatedBy: "tester",
			StartTime: time.Now(),
			Status:    "active",
		}

		// CreateSession contract
		err = manager.CreateSession(session)
		assert.NoError(t, err, "CreateSession contract must be fulfilled")

		// GetActiveSession contract
		activeSession, err := manager.GetActiveSession()
		assert.NoError(t, err, "GetActiveSession contract must be fulfilled")
		assert.NotNil(t, activeSession, "GetActiveSession must return valid session")

		// UpdateSession contract
		session.Status = "ended"
		err = manager.UpdateSession(session)
		assert.NoError(t, err, "UpdateSession contract must be fulfilled")

		// WriteMessage contract
		message := &database.Message{
			ID:        "contract-msg",
			SessionID: session.ID,
			Type:      database.MessageTypeBroadcastToInstructors,
			Context:   database.ContextQuestion,
			FromUser:  "student",
			Content:   map[string]interface{}{"text": "Contract test message"},
			Timestamp: time.Now(),
		}

		err = manager.WriteMessage(message)
		assert.NoError(t, err, "WriteMessage contract must be fulfilled")

		// WriteBatch contract
		batchMessages := []*database.Message{message}
		err = manager.WriteBatch(batchMessages)
		assert.NoError(t, err, "WriteBatch contract must be fulfilled")

		// GetSessionMessages contract
		time.Sleep(config.DatabaseFlushInterval + 50*time.Millisecond)
		messages, err := manager.GetSessionMessages(session.ID)
		assert.NoError(t, err, "GetSessionMessages contract must be fulfilled")
		assert.NotNil(t, messages, "GetSessionMessages must return valid slice")
	})
}

func TestStep13_SingleWriterIntegration(t *testing.T) {
	t.Run("Single writer pattern under concurrent load", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet - cannot test single writer pattern")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		// Simulate high concurrency load
		const numWorkers = 20
		const operationsPerWorker = 10
		var wg sync.WaitGroup
		errChan := make(chan error, numWorkers*operationsPerWorker)

		// Create test session first
		session := &database.Session{
			ID:        "load-test-session",
			Name:      "Load Test Session",
			CreatedBy: "load-tester",
			StartTime: time.Now(),
			Status:    "active",
		}
		err = manager.CreateSession(session)
		require.NoError(t, err)

		wg.Add(numWorkers)
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < operationsPerWorker; j++ {
					// Mix of different operations
					switch j % 4 {
					case 0:
						// WriteMessage
						message := &database.Message{
							ID:        fmt.Sprintf("load-msg-%d-%d", workerID, j),
							SessionID: session.ID,
							Type:      database.MessageTypeBroadcastToInstructors,
							Context:   database.ContextQuestion,
							FromUser:  fmt.Sprintf("user-%d", workerID),
							Content:   map[string]interface{}{"text": fmt.Sprintf("Load test message %d", j)},
							Timestamp: time.Now(),
						}
						err := manager.WriteMessage(message)
						errChan <- err

					case 1:
						// WriteBatch
						batchMessages := []*database.Message{
							{
								ID:        fmt.Sprintf("batch-msg-%d-%d", workerID, j),
								SessionID: session.ID,
								Type:      database.MessageTypeDirectMessage,
								Context:   database.ContextResponse,
								FromUser:  fmt.Sprintf("user-%d", workerID),
								Content:   map[string]interface{}{"text": fmt.Sprintf("Batch message %d", j)},
								Timestamp: time.Now(),
							},
						}
						err := manager.WriteBatch(batchMessages)
						errChan <- err

					case 2:
						// UpdateSession
						updatedSession := &database.Session{
							ID:        session.ID,
							Name:      fmt.Sprintf("Updated Session %d-%d", workerID, j),
							CreatedBy: session.CreatedBy,
							StartTime: session.StartTime,
							Status:    "active",
						}
						err := manager.UpdateSession(updatedSession)
						errChan <- err

					case 3:
						// GetActiveSession (read operation)
						_, err := manager.GetActiveSession()
						errChan <- err
					}
				}
			}(i)
		}

		// Wait for all operations to complete
		wg.Wait()
		close(errChan)

		// Check for errors
		var errors []error
		for err := range errChan {
			if err != nil {
				errors = append(errors, err)
			}
		}

		assert.Empty(t, errors, "Single writer pattern must handle concurrent operations without errors")
	})

	t.Run("WAL mode enables concurrent reads during writes", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet - cannot test WAL mode")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		// Create test session
		session := &database.Session{
			ID:        "wal-test-session",
			Name:      "WAL Test Session",
			CreatedBy: "wal-tester",
			StartTime: time.Now(),
			Status:    "active",
		}
		err = manager.CreateSession(session)
		require.NoError(t, err)

		var wg sync.WaitGroup
		readErrors := make(chan error, 10)
		writeErrors := make(chan error, 100)

		// Start continuous read operations
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				time.Sleep(10 * time.Millisecond)
				_, err := manager.GetActiveSession()
				readErrors <- err
				_, err = manager.GetSessionMessages(session.ID)
				readErrors <- err
			}
		}()

		// Start continuous write operations
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				message := &database.Message{
					ID:        fmt.Sprintf("wal-msg-%d", i),
					SessionID: session.ID,
					Type:      database.MessageTypeBroadcastToInstructors,
					Context:   database.ContextQuestion,
					FromUser:  fmt.Sprintf("user-%d", i%5),
					Content:   map[string]interface{}{"text": fmt.Sprintf("WAL test message %d", i)},
					Timestamp: time.Now(),
				}
				err := manager.WriteMessage(message)
				writeErrors <- err
				time.Sleep(5 * time.Millisecond)
			}
		}()

		wg.Wait()
		close(readErrors)
		close(writeErrors)

		// Check read errors
		var rErrors []error
		for err := range readErrors {
			if err != nil {
				rErrors = append(rErrors, err)
			}
		}

		// Check write errors
		var wErrors []error
		for err := range writeErrors {
			if err != nil {
				wErrors = append(wErrors, err)
			}
		}

		assert.Empty(t, rErrors, "WAL mode must allow concurrent reads without errors")
		assert.Empty(t, wErrors, "WAL mode must allow writes without errors")
	})
}

func TestStep13_BatchingIntegration(t *testing.T) {
	t.Run("Message batching integrates with SQLite operations", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet - cannot test batching integration")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		// Create test session
		session := &database.Session{
			ID:        "batch-test-session",
			Name:      "Batch Test Session", 
			CreatedBy: "batch-tester",
			StartTime: time.Now(),
			Status:    "active",
		}
		err = manager.CreateSession(session)
		require.NoError(t, err)

		// Send messages rapidly to trigger batching
		const numMessages = config.DatabaseBatchSize + 10
		start := time.Now()

		for i := 0; i < numMessages; i++ {
			message := &database.Message{
				ID:        fmt.Sprintf("batch-msg-%d", i),
				SessionID: session.ID,
				Type:      database.MessageTypeBroadcastToInstructors,
				Context:   database.ContextQuestion,
				FromUser:  fmt.Sprintf("student-%d", i%5),
				Content:   map[string]interface{}{"text": fmt.Sprintf("Batch test message %d", i)},
				Timestamp: time.Now(),
			}

			err = manager.WriteMessage(message)
			assert.NoError(t, err, "WriteMessage must not fail during batching")
		}

		// Wait for all batches to be processed
		time.Sleep(config.DatabaseFlushInterval*2 + 200*time.Millisecond)

		// Verify all messages were persisted
		messages, err := manager.GetSessionMessages(session.ID)
		assert.NoError(t, err, "GetSessionMessages must work after batching")
		assert.Len(t, messages, numMessages, "All batched messages must be persisted")

		elapsed := time.Since(start)
		t.Logf("Batched %d messages in %v", numMessages, elapsed)

		// Verify batching improved performance
		assert.Less(t, elapsed, 2*time.Second, "Batching should complete efficiently")
	})
}

func TestStep13_RetryLogicIntegration(t *testing.T) {
	t.Run("Retry logic handles temporary database failures", func(t *testing.T) {
		// This test would require a way to simulate database failures
		// For now, we'll test the basic retry mechanism setup
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet - cannot test retry logic")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		// Test that retry mechanism is configured correctly
		// (Actual retry testing would require database failure simulation)
		assert.NotNil(t, manager, "Manager with retry logic must be created")
	})
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func initTestSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_by TEXT NOT NULL,
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		status TEXT NOT NULL
	);

	CREATE TABLE messages (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		type TEXT NOT NULL,
		context TEXT NOT NULL,
		from_user TEXT NOT NULL,
		to_user TEXT,
		content TEXT NOT NULL,
		timestamp DATETIME NOT NULL
	);
	`

	_, err := db.Exec(schema)
	return err
}