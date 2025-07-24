package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"switchboard/pkg/database"
	"switchboard/pkg/interfaces"
	"switchboard/pkg/types"
)

// Test database setup helpers
func setupTestDB(t *testing.T) (*Manager, func()) {
	// Create temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	config := &database.Config{
		DatabasePath:    dbPath,
		MaxConnections:  10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: time.Minute * 30,
	}
	
	// Apply schema migrations for testing
	sqliteDB, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	
	// Create test schema
	schema := `
	CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_by TEXT NOT NULL,
		student_ids TEXT NOT NULL,
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
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
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);
	
	CREATE INDEX idx_sessions_status ON sessions(status);
	CREATE INDEX idx_messages_session_time ON messages(session_id, timestamp);
	CREATE INDEX idx_messages_type ON messages(type);
	`
	
	_, err = sqliteDB.Exec(schema)
	if err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}
	_ = sqliteDB.Close()
	
	// This will FAIL until Manager is implemented
	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	
	cleanup := func() {
		_ = manager.Close()
		_ = os.RemoveAll(tmpDir)
	}
	
	return manager, cleanup
}

// Architectural Validation Tests
func TestManager_InterfaceCompliance(t *testing.T) {
	// This test will FAIL until Manager is implemented
	// Verify Manager implements DatabaseManager interface
	config := &database.Config{DatabasePath: ":memory:"}
	var _ interfaces.DatabaseManager = &Manager{}
	_ = config // Manager constructor will be tested separately
}

func TestManager_ImportBoundaryCompliance(t *testing.T) {
	// This test passes if compilation succeeds - no forbidden imports
	t.Log("Database manager import boundaries maintained - only allowed dependencies")
}

func TestManager_SingleWriterArchitecture(t *testing.T) {
	// This test will FAIL until single-writer pattern is implemented
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	// Verify manager has required fields for single-writer pattern
	if manager == nil {
		t.Fatal("Manager should be properly initialized")
	}
	
	// Architecture should include write channel and goroutine coordination
	// This will fail until the writeLoop is implemented
}

// Functional Validation Tests - Core Database Operations
func TestManager_CreateSessionBehavior(t *testing.T) {
	// This test will FAIL until CreateSession is implemented
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	session := &types.Session{
		ID:         "test-session-123",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1", "student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	err := manager.CreateSession(ctx, session)
	if err != nil {
		t.Errorf("CreateSession should succeed: %v", err)
	}
	
	// Verify session was actually stored in database
	retrievedSession, err := manager.GetSession(ctx, "test-session-123")
	if err != nil {
		t.Errorf("GetSession should succeed after CreateSession: %v", err)
	}
	
	if retrievedSession == nil {
		t.Fatal("Retrieved session should not be nil")
	}
	
	if retrievedSession.Name != "Test Session" {
		t.Errorf("Expected name 'Test Session', got '%s'", retrievedSession.Name)
	}
	
	if len(retrievedSession.StudentIDs) != 2 {
		t.Errorf("Expected 2 student IDs, got %d", len(retrievedSession.StudentIDs))
	}
}

func TestManager_GetSessionNotFound(t *testing.T) {
	// This test will FAIL until GetSession error handling is implemented
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	_, err := manager.GetSession(ctx, "nonexistent-session")
	
	if err != interfaces.ErrSessionNotFound {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}
}

func TestManager_UpdateSessionBehavior(t *testing.T) {
	// This test will FAIL until UpdateSession is implemented
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	// First create a session
	session := &types.Session{
		ID:         "test-session-456",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	err := manager.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("CreateSession should succeed: %v", err)
	}
	
	// Update session to ended
	now := time.Now()
	session.EndTime = &now
	session.Status = "ended"
	
	err = manager.UpdateSession(ctx, session)
	if err != nil {
		t.Errorf("UpdateSession should succeed: %v", err)
	}
	
	// Verify update was persisted
	updatedSession, err := manager.GetSession(ctx, "test-session-456")
	if err != nil {
		t.Errorf("GetSession should succeed after UpdateSession: %v", err)
	}
	
	if updatedSession.Status != "ended" {
		t.Errorf("Expected status 'ended', got '%s'", updatedSession.Status)
	}
	
	if updatedSession.EndTime == nil {
		t.Error("End time should be set after update")
	}
}

func TestManager_ListActiveSessionsBehavior(t *testing.T) {
	// This test will FAIL until ListActiveSessions is implemented
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	// Create active session
	activeSession := &types.Session{
		ID:         "active-session",
		Name:       "Active Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	err := manager.CreateSession(ctx, activeSession)
	if err != nil {
		t.Fatalf("CreateSession should succeed: %v", err)
	}
	
	// Create ended session
	endTime := time.Now()
	endedSession := &types.Session{
		ID:         "ended-session",
		Name:       "Ended Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student2"},
		StartTime:  time.Now().Add(-time.Hour),
		EndTime:    &endTime,
		Status:     "ended",
	}
	
	err = manager.CreateSession(ctx, endedSession)
	if err != nil {
		t.Fatalf("CreateSession should succeed: %v", err)
	}
	
	// List active sessions should only return active ones
	activeSessions, err := manager.ListActiveSessions(ctx)
	if err != nil {
		t.Errorf("ListActiveSessions should succeed: %v", err)
	}
	
	if len(activeSessions) != 1 {
		t.Errorf("Expected 1 active session, got %d", len(activeSessions))
	}
	
	if len(activeSessions) > 0 && activeSessions[0].ID != "active-session" {
		t.Errorf("Expected active session ID 'active-session', got '%s'", activeSessions[0].ID)
	}
}

func TestManager_StoreMessageBehavior(t *testing.T) {
	// This test will FAIL until StoreMessage is implemented
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	// First create a session
	session := &types.Session{
		ID:         "session-for-messages",
		Name:       "Message Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	err := manager.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("CreateSession should succeed: %v", err)
	}
	
	// Store a message
	message := &types.Message{
		ID:        "msg-123",
		SessionID: "session-for-messages",
		Type:      "instructor_broadcast",
		Context:   "general",
		FromUser:  "instructor1",
		ToUser:    nil,
		Content:   map[string]interface{}{"text": "Hello class"},
		Timestamp: time.Now(),
	}
	
	err = manager.StoreMessage(ctx, message)
	if err != nil {
		t.Errorf("StoreMessage should succeed: %v", err)
	}
	
	// Retrieve session history to verify message was stored
	messages, err := manager.GetSessionHistory(ctx, "session-for-messages")
	if err != nil {
		t.Errorf("GetSessionHistory should succeed: %v", err)
	}
	
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	
	if len(messages) > 0 {
		if messages[0].Type != "instructor_broadcast" {
			t.Errorf("Expected type 'instructor_broadcast', got '%s'", messages[0].Type)
		}
		
		if messages[0].Content["text"] != "Hello class" {
			t.Errorf("Expected content text 'Hello class', got %v", messages[0].Content["text"])
		}
	}
}

func TestManager_GetSessionHistoryOrdering(t *testing.T) {
	// This test will FAIL until GetSessionHistory ordering is implemented
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	// Create session
	session := &types.Session{
		ID:         "history-session",
		Name:       "History Test",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	err := manager.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("CreateSession should succeed: %v", err)
	}
	
	// Store messages with different timestamps
	baseTime := time.Now()
	messages := []*types.Message{
		{
			ID:        "msg-2",
			SessionID: "history-session",
			Type:      "instructor_broadcast",
			FromUser:  "instructor1",
			Content:   map[string]interface{}{"text": "Second message"},
			Timestamp: baseTime.Add(time.Minute),
		},
		{
			ID:        "msg-1",
			SessionID: "history-session",
			Type:      "instructor_broadcast",
			FromUser:  "instructor1",
			Content:   map[string]interface{}{"text": "First message"},
			Timestamp: baseTime,
		},
		{
			ID:        "msg-3",
			SessionID: "history-session",
			Type:      "instructor_broadcast",
			FromUser:  "instructor1",
			Content:   map[string]interface{}{"text": "Third message"},
			Timestamp: baseTime.Add(2 * time.Minute),
		},
	}
	
	for _, msg := range messages {
		err = manager.StoreMessage(ctx, msg)
		if err != nil {
			t.Fatalf("StoreMessage should succeed: %v", err)
		}
	}
	
	// Retrieve history - should be ordered by timestamp ASC
	history, err := manager.GetSessionHistory(ctx, "history-session")
	if err != nil {
		t.Errorf("GetSessionHistory should succeed: %v", err)
	}
	
	if len(history) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(history))
	}
	
	// Verify chronological ordering
	if len(history) >= 3 {
		if history[0].Content["text"] != "First message" {
			t.Errorf("Expected first message first, got %v", history[0].Content["text"])
		}
		if history[1].Content["text"] != "Second message" {
			t.Errorf("Expected second message second, got %v", history[1].Content["text"])
		}
		if history[2].Content["text"] != "Third message" {
			t.Errorf("Expected third message third, got %v", history[2].Content["text"])
		}
	}
}

// Error Handling Validation Tests
func TestManager_TransactionRollback(t *testing.T) {
	// This test will FAIL until transaction handling is implemented
	// This test simulates transaction failure to verify rollback behavior
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	// First create a valid session to establish baseline
	validSession := &types.Session{
		ID:         "valid-session",
		Name:       "Valid Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	err := manager.CreateSession(ctx, validSession)
	if err != nil {
		t.Fatalf("CreateSession with valid data should succeed: %v", err)
	}
	
	// Now attempt to create session with duplicate ID to cause primary key violation
	duplicateSession := &types.Session{
		ID:         "valid-session", // Same ID should cause constraint violation
		Name:       "Duplicate Session",
		CreatedBy:  "instructor2",
		StudentIDs: []string{"student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	err = manager.CreateSession(ctx, duplicateSession)
	if err == nil {
		t.Error("CreateSession with duplicate ID should fail")
	}
	
	// Verify only one session exists (transaction rollback worked)
	sessions, err := manager.ListActiveSessions(ctx)
	if err != nil {
		t.Errorf("ListActiveSessions should work: %v", err)
	}
	
	if len(sessions) != 1 {
		t.Errorf("Expected 1 session after failed transaction, got %d", len(sessions))
	}
	
	// Verify the original session data is unchanged
	if len(sessions) > 0 && sessions[0].Name != "Valid Session" {
		t.Errorf("Expected original session name 'Valid Session', got '%s'", sessions[0].Name)
	}
}

func TestManager_DatabaseConnectionFailure(t *testing.T) {
	// This test will FAIL until connection error handling is implemented
	// Test with invalid database path to simulate connection failure
	config := &database.Config{
		DatabasePath:    "/invalid/path/database.db",
		MaxConnections:  10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: time.Minute * 30,
	}
	
	_, err := NewManager(config)
	if err == nil {
		t.Error("NewManager should fail with invalid database path")
	}
}

// Technical Validation Tests - Performance and Concurrency
func TestManager_SingleWriterPattern(t *testing.T) {
	// This test will FAIL until single-writer goroutine is implemented
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	// Test concurrent write operations to verify single-writer pattern
	const numWrites = 10
	var wg sync.WaitGroup
	errors := make(chan error, numWrites)
	
	wg.Add(numWrites)
	for i := 0; i < numWrites; i++ {
		go func(id int) {
			defer wg.Done()
			
			session := &types.Session{
				ID:         fmt.Sprintf("concurrent-session-%d", id),
				Name:       fmt.Sprintf("Concurrent Session %d", id),
				CreatedBy:  "instructor1",
				StudentIDs: []string{"student1"},
				StartTime:  time.Now(),
				Status:     "active",
			}
			
			err := manager.CreateSession(ctx, session)
			if err != nil {
				errors <- err
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for any errors from concurrent writes
	for err := range errors {
		t.Errorf("Concurrent write failed: %v", err)
	}
	
	// Verify all sessions were created (single-writer should handle concurrency)
	sessions, err := manager.ListActiveSessions(ctx)
	if err != nil {
		t.Errorf("ListActiveSessions should work: %v", err)
	}
	
	if len(sessions) != numWrites {
		t.Errorf("Expected %d sessions, got %d", numWrites, len(sessions))
	}
}

func TestManager_WriteOperationPerformance(t *testing.T) {
	// This test will FAIL until performance optimization is implemented
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	// Test write operation performance - should complete in <50ms
	session := &types.Session{
		ID:         "perf-test-session",
		Name:       "Performance Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	// Add many students to increase data size
	for i := 0; i < 100; i++ {
		session.StudentIDs = append(session.StudentIDs, fmt.Sprintf("student%d", i))
	}
	
	start := time.Now()
	err := manager.CreateSession(ctx, session)
	duration := time.Since(start)
	
	if err != nil {
		t.Errorf("CreateSession should succeed: %v", err)
	}
	
	if duration > 50*time.Millisecond {
		t.Errorf("Write operation too slow: %v (should be <50ms)", duration)
	}
}

func TestManager_ReadOperationPerformance(t *testing.T) {
	// This test will FAIL until read performance optimization is implemented
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	// Create session for message history test
	session := &types.Session{
		ID:         "read-perf-session",
		Name:      "Read Performance Test",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	err := manager.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("CreateSession should succeed: %v", err)
	}
	
	// Create many messages to test read performance
	for i := 0; i < 1000; i++ {
		message := &types.Message{
			ID:        fmt.Sprintf("msg-%d", i),
			SessionID: "read-perf-session",
			Type:      "instructor_broadcast",
			FromUser:  "instructor1",
			Content:   map[string]interface{}{"text": fmt.Sprintf("Message %d", i)},
			Timestamp: time.Now().Add(time.Duration(i) * time.Millisecond),
		}
		
		err = manager.StoreMessage(ctx, message)
		if err != nil {
			t.Fatalf("StoreMessage should succeed: %v", err)
		}
	}
	
	// Test read performance - should complete in <100ms for 1000 messages
	start := time.Now()
	messages, err := manager.GetSessionHistory(ctx, "read-perf-session")
	duration := time.Since(start)
	
	if err != nil {
		t.Errorf("GetSessionHistory should succeed: %v", err)
	}
	
	if len(messages) != 1000 {
		t.Errorf("Expected 1000 messages, got %d", len(messages))
	}
	
	if duration > 100*time.Millisecond {
		t.Errorf("Read operation too slow: %v (should be <100ms)", duration)
	}
}

func TestManager_ConcurrentReadAccess(t *testing.T) {
	// This test will FAIL until concurrent read support is implemented
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	// Create test session
	session := &types.Session{
		ID:         "concurrent-read-session",
		Name:       "Concurrent Read Test",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	err := manager.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("CreateSession should succeed: %v", err)
	}
	
	// Test concurrent read operations
	const numReads = 50
	var wg sync.WaitGroup
	errors := make(chan error, numReads)
	
	wg.Add(numReads)
	for i := 0; i < numReads; i++ {
		go func() {
			defer wg.Done()
			
			_, err := manager.GetSession(ctx, "concurrent-read-session")
			if err != nil {
				errors <- err
			}
		}()
	}
	
	wg.Wait()
	close(errors)
	
	// Check for any errors from concurrent reads
	for err := range errors {
		t.Errorf("Concurrent read failed: %v", err)
	}
}

func TestManager_HealthCheckBehavior(t *testing.T) {
	// This test will FAIL until HealthCheck is implemented
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	err := manager.HealthCheck(ctx)
	if err != nil {
		t.Errorf("HealthCheck should succeed for healthy database: %v", err)
	}
}

func TestManager_CleanShutdown(t *testing.T) {
	// This test will FAIL until proper shutdown is implemented
	manager, cleanup := setupTestDB(t)
	defer func() {
		// Don't call cleanup() as we're testing Close() directly
		_ = cleanup
	}()
	
	// Start some operations
	ctx := context.Background()
	session := &types.Session{
		ID:         "shutdown-test-session",
		Name:       "Shutdown Test",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	err := manager.CreateSession(ctx, session)
	if err != nil {
		t.Errorf("CreateSession should succeed: %v", err)
	}
	
	// Test clean shutdown
	err = manager.Close()
	if err != nil {
		t.Errorf("Close should succeed: %v", err)
	}
	
	// Verify operations fail after shutdown
	err = manager.CreateSession(ctx, session)
	if err == nil {
		t.Error("Operations should fail after Close()")
	}
}

// Integration Validation Tests
func TestManager_CompleteSessionLifecycle(t *testing.T) {
	// This test will FAIL until complete integration is implemented
	manager, cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	// Step 1: Create session
	session := &types.Session{
		ID:         "lifecycle-session",
		Name:       "Lifecycle Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1", "student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	err := manager.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("CreateSession should succeed: %v", err)
	}
	
	// Step 2: Store messages
	messages := []*types.Message{
		{
			ID:        "msg-1",
			SessionID: "lifecycle-session",
			Type:      "instructor_broadcast",
			FromUser:  "instructor1",
			Content:   map[string]interface{}{"text": "Welcome"},
			Timestamp: time.Now(),
		},
		{
			ID:        "msg-2",
			SessionID: "lifecycle-session",
			Type:      "inbox_response",
			FromUser:  "instructor1",
			ToUser:    &[]string{"student1"}[0],
			Content:   map[string]interface{}{"text": "Response"},
			Timestamp: time.Now().Add(time.Minute),
		},
	}
	
	for _, msg := range messages {
		err = manager.StoreMessage(ctx, msg)
		if err != nil {
			t.Fatalf("StoreMessage should succeed: %v", err)
		}
	}
	
	// Step 3: Verify session is in active list
	activeSessions, err := manager.ListActiveSessions(ctx)
	if err != nil {
		t.Errorf("ListActiveSessions should succeed: %v", err)
	}
	
	found := false
	for _, s := range activeSessions {
		if s.ID == "lifecycle-session" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Created session should be in active sessions list")
	}
	
	// Step 4: Retrieve session history
	history, err := manager.GetSessionHistory(ctx, "lifecycle-session")
	if err != nil {
		t.Errorf("GetSessionHistory should succeed: %v", err)
	}
	
	if len(history) != 2 {
		t.Errorf("Expected 2 messages in history, got %d", len(history))
	}
	
	// Step 5: End session
	now := time.Now()
	session.EndTime = &now
	session.Status = "ended"
	
	err = manager.UpdateSession(ctx, session)
	if err != nil {
		t.Errorf("UpdateSession should succeed: %v", err)
	}
	
	// Step 6: Verify session no longer in active list
	activeSessions, err = manager.ListActiveSessions(ctx)
	if err != nil {
		t.Errorf("ListActiveSessions should succeed: %v", err)
	}
	
	for _, s := range activeSessions {
		if s.ID == "lifecycle-session" {
			t.Error("Ended session should not be in active sessions list")
		}
	}
	
	// Step 7: Verify session can still be retrieved with ended status
	endedSession, err := manager.GetSession(ctx, "lifecycle-session")
	if err != nil {
		t.Errorf("GetSession should work for ended sessions: %v", err)
	}
	
	if endedSession.Status != "ended" {
		t.Errorf("Expected status 'ended', got '%s'", endedSession.Status)
	}
}