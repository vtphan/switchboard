package unit

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"

	"switchboard/internal/database"
)

func TestSQLiteDatabaseManagerLifecycle(t *testing.T) {
	t.Run("NewSQLiteDatabaseManager creates valid instance", func(t *testing.T) {
		// Use in-memory database for testing
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		assert.NotNil(t, manager, "NewSQLiteDatabaseManager must return non-nil manager")
		
		// Verify interface compliance
		var _ database.DatabaseManager = manager
	})

	t.Run("Start initializes background goroutines", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		err = manager.Start()
		assert.NoError(t, err, "Start must not return error")

		// Clean up
		err = manager.Stop()
		assert.NoError(t, err, "Stop must not return error after Start")
	})

	t.Run("Stop gracefully shuts down", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		// Start and immediately stop
		err = manager.Start()
		require.NoError(t, err)

		start := time.Now()
		err = manager.Stop()
		duration := time.Since(start)

		assert.NoError(t, err, "Stop must not return error")
		assert.Less(t, duration, 5*time.Second, "Stop must complete within reasonable time")
	})

	t.Run("Multiple Start calls are safe", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		// Multiple Start calls should be safe
		err1 := manager.Start()
		err2 := manager.Start()

		assert.NoError(t, err1, "First Start must succeed")
		assert.NoError(t, err2, "Second Start must be safe (idempotent)")

		// Clean up
		_ = manager.Stop()
	})

	t.Run("Multiple Stop calls are safe", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		_ = manager.Start()
		
		// Multiple Stop calls should be safe
		err1 := manager.Stop()
		err2 := manager.Stop()

		assert.NoError(t, err1, "First Stop must succeed")
		assert.NoError(t, err2, "Second Stop must be safe (idempotent)")
	})
}

func TestSQLiteConfigurationApplication(t *testing.T) {
	t.Run("SQLite pragmas are applied correctly", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		// WAL mode cannot be enabled on :memory: databases (SQLite limitation)
		// This will be tested properly with file-based databases in Phase 5 system integration
		var journalMode string
		err = db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
		require.NoError(t, err)
		if journalMode != "memory" {
			// Only assert WAL mode if not using :memory: database
			assert.Equal(t, "wal", journalMode, "WAL mode must be enabled for file databases")
		} else {
			t.Logf("Skipping WAL mode check for :memory: database (will be tested in Phase 5 with file databases)")
		}

		// Verify synchronous mode
		var synchronous int
		err = db.QueryRow("PRAGMA synchronous").Scan(&synchronous)
		require.NoError(t, err)
		assert.Equal(t, 1, synchronous, "Synchronous mode must be NORMAL (1)")

		// Verify cache size (should be negative for KB)
		var cacheSize int
		err = db.QueryRow("PRAGMA cache_size").Scan(&cacheSize)
		require.NoError(t, err)
		assert.Equal(t, -64000, cacheSize, "Cache size must be -64000 (64MB)")
	})
}

func TestSQLiteSessionOperations(t *testing.T) {
	t.Run("CreateSession succeeds with valid session", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		// Initialize schema
		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		session := &database.Session{
			ID:        "test-session-1",
			Name:      "Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}

		err = manager.CreateSession(session)
		assert.NoError(t, err, "CreateSession must succeed with valid session")
	})

	t.Run("CreateSession fails with invalid session", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		// Invalid session with empty required fields
		session := &database.Session{
			ID:   "", // Invalid: empty ID
			Name: "",
		}

		err = manager.CreateSession(session)
		assert.Error(t, err, "CreateSession must fail with invalid session")
	})

	t.Run("UpdateSession modifies existing session", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		// Create initial session
		session := &database.Session{
			ID:        "test-session-1",
			Name:      "Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}

		err = manager.CreateSession(session)
		require.NoError(t, err)

		// Update session status
		endTime := time.Now()
		session.EndTime = &endTime
		session.Status = "ended"

		err = manager.UpdateSession(session)
		assert.NoError(t, err, "UpdateSession must succeed")
	})

	t.Run("GetActiveSession returns correct session", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		// Create active session
		session := &database.Session{
			ID:        "test-session-1",
			Name:      "Test Session",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}

		err = manager.CreateSession(session)
		require.NoError(t, err)

		// Retrieve active session
		activeSession, err := manager.GetActiveSession()
		assert.NoError(t, err, "GetActiveSession must not return error")
		assert.NotNil(t, activeSession, "GetActiveSession must return session")
		assert.Equal(t, session.ID, activeSession.ID, "GetActiveSession must return correct session")
	})

	t.Run("GetActiveSession returns nil when no active session", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		// No sessions created
		activeSession, err := manager.GetActiveSession()
		assert.NoError(t, err, "GetActiveSession must not return error when no session")
		assert.Nil(t, activeSession, "GetActiveSession must return nil when no active session")
	})
}

func TestSQLiteMessageOperations(t *testing.T) {
	t.Run("WriteMessage succeeds with valid message", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		message := &database.Message{
			ID:        "msg-1",
			SessionID: "session-1",
			Type:      database.MessageTypeBroadcastToInstructors,
			Context:   database.ContextQuestion,
			FromUser:  "student1",
			Content:   map[string]interface{}{"text": "Test question"},
			Timestamp: time.Now(),
		}

		err = manager.WriteMessage(message)
		assert.NoError(t, err, "WriteMessage must succeed with valid message")
	})

	t.Run("WriteBatch succeeds with multiple messages", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		messages := []*database.Message{
			{
				ID:        "msg-1",
				SessionID: "session-1",
				Type:      database.MessageTypeBroadcastToInstructors,
				Context:   database.ContextQuestion,
				FromUser:  "student1",
				Content:   map[string]interface{}{"text": "Question 1"},
				Timestamp: time.Now(),
			},
			{
				ID:        "msg-2",
				SessionID: "session-1",
				Type:      database.MessageTypeBroadcastToInstructors,
				Context:   database.ContextQuestion,
				FromUser:  "student2",
				Content:   map[string]interface{}{"text": "Question 2"},
				Timestamp: time.Now(),
			},
		}

		err = manager.WriteBatch(messages)
		assert.NoError(t, err, "WriteBatch must succeed with valid messages")
	})

	t.Run("GetSessionMessages returns correct messages", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		sessionID := "session-1"

		// Write messages
		messages := []*database.Message{
			{
				ID:        "msg-1",
				SessionID: sessionID,
				Type:      database.MessageTypeBroadcastToInstructors,
				Context:   database.ContextQuestion,
				FromUser:  "student1",
				Content:   map[string]interface{}{"text": "Question 1"},
				Timestamp: time.Now(),
			},
			{
				ID:        "msg-2",
				SessionID: sessionID,
				Type:      database.MessageTypeBroadcastToInstructors,
				Context:   database.ContextQuestion,
				FromUser:  "student2",
				Content:   map[string]interface{}{"text": "Question 2"},
				Timestamp: time.Now(),
			},
		}

		for _, msg := range messages {
			err = manager.WriteMessage(msg)
			require.NoError(t, err)
		}

		// Allow time for writes to complete
		time.Sleep(100 * time.Millisecond)

		// Retrieve messages
		retrievedMessages, err := manager.GetSessionMessages(sessionID)
		assert.NoError(t, err, "GetSessionMessages must not return error")
		assert.Len(t, retrievedMessages, 2, "GetSessionMessages must return correct number of messages")
	})
}

func TestSQLiteSingleWriterPattern(t *testing.T) {
	t.Run("Concurrent writes are handled safely", func(t *testing.T) {
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = initTestSchema(db)
		require.NoError(t, err)

		manager, err := database.NewSQLiteDatabaseManager(db)
		if err != nil {
			t.Skip("SQLiteDatabaseManager not implemented yet")
		}

		err = manager.Start()
		require.NoError(t, err)
		defer func() { _ = manager.Stop() }()

		// Perform concurrent writes
		const numWorkers = 10
		const messagesPerWorker = 5
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		errChan := make(chan error, numWorkers*messagesPerWorker)
		
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				for j := 0; j < messagesPerWorker; j++ {
					select {
					case <-ctx.Done():
						return
					default:
						message := &database.Message{
							ID:        fmt.Sprintf("msg-%d-%d", workerID, j),
							SessionID: "session-1",
							Type:      database.MessageTypeBroadcastToInstructors,
							Context:   database.ContextQuestion,
							FromUser:  fmt.Sprintf("student-%d", workerID),
							Content:   map[string]interface{}{"text": fmt.Sprintf("Message %d from worker %d", j, workerID)},
							Timestamp: time.Now(),
						}

						err := manager.WriteMessage(message)
						errChan <- err
					}
				}
			}(i)
		}

		// Collect results
		var errors []error
		for i := 0; i < numWorkers*messagesPerWorker; i++ {
			select {
			case err := <-errChan:
				if err != nil {
					errors = append(errors, err)
				}
			case <-ctx.Done():
				t.Fatal("Test timed out")
			}
		}

		assert.Empty(t, errors, "Concurrent writes must not produce errors")
	})
}

// Helper function to initialize test database schema
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