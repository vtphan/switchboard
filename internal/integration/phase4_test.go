package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
	
	"switchboard/internal/database"
	"switchboard/internal/session"
	"switchboard/internal/websocket"
	dbconfig "switchboard/pkg/database"
	"switchboard/pkg/types"
)

// TestPhase4_SessionLifecycleIntegration validates complete session lifecycle
func TestPhase4_SessionLifecycleIntegration(t *testing.T) {
	// Setup test database
	dbPath := "./test_phase4_lifecycle.db"
	defer func() {
		if err := os.Remove(dbPath); err != nil {
			t.Logf("Failed to remove test database: %v", err)
		}
	}()
	
	// Initialize database with schema
	InitializeTestDatabase(t, dbPath)
	
	// Create database configuration
	config := &dbconfig.Config{
		DatabasePath:     dbPath,
		MaxConnections:   10,
		ConnMaxLifetime:  time.Hour,
		ConnMaxIdleTime:  time.Minute * 10,
		MigrationsPath:   "../../migrations",
	}
	
	// Initialize database manager
	dbManager, err := database.NewManager(config)
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}
	defer func() {
		if err := dbManager.Close(); err != nil {
			t.Logf("Failed to close database manager: %v", err)
		}
	}()
	
	// Initialize session manager
	sessionManager := session.NewManager(dbManager)
	
	// Load active sessions
	ctx := context.Background()
	if err := sessionManager.LoadActiveSessions(ctx); err != nil {
		t.Fatalf("Failed to load active sessions: %v", err)
	}
	
	// Create a new session
	sessionName := "Integration Test Session"
	createdBy := "instructor1"
	studentIDs := []string{"student1", "student2", "student3"}
	
	session, err := sessionManager.CreateSession(ctx, sessionName, createdBy, studentIDs)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	// Validate session was created
	if session.ID == "" {
		t.Error("Session ID should not be empty")
	}
	if session.Status != "active" {
		t.Errorf("Expected session status to be 'active', got %s", session.Status)
	}
	
	// Test session validation for different roles
	t.Run("RoleBasedAccess", func(t *testing.T) {
		// Instructor should have access
		if err := sessionManager.ValidateSessionMembership(session.ID, "instructor2", "instructor"); err != nil {
			t.Errorf("Instructor should have access to any session: %v", err)
		}
		
		// Student in list should have access
		if err := sessionManager.ValidateSessionMembership(session.ID, "student1", "student"); err != nil {
			t.Errorf("Student1 should have access: %v", err)
		}
		
		// Student not in list should not have access
		if err := sessionManager.ValidateSessionMembership(session.ID, "student99", "student"); err == nil {
			t.Error("Student99 should not have access")
		}
	})
	
	// Test cache performance
	t.Run("CachePerformance", func(t *testing.T) {
		start := time.Now()
		for i := 0; i < 1000; i++ {
			if err := sessionManager.ValidateSessionMembership(session.ID, "student1", "student"); err != nil {
				t.Fatalf("Validation failed: %v", err)
			}
		}
		elapsed := time.Since(start)
		avgTime := elapsed / 1000
		
		if avgTime > time.Millisecond {
			t.Errorf("Average validation time %v exceeds 1ms requirement", avgTime)
		}
		t.Logf("Average validation time: %v (target: <1ms)", avgTime)
	})
	
	// End the session
	if err := sessionManager.EndSession(ctx, session.ID); err != nil {
		t.Fatalf("Failed to end session: %v", err)
	}
	
	// Validate session ended
	endedSession, err := sessionManager.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get ended session: %v", err)
	}
	if endedSession.Status != "ended" {
		t.Errorf("Expected session status to be 'ended', got %s", endedSession.Status)
	}
	if endedSession.EndTime == nil {
		t.Error("End time should be set for ended session")
	}
	
	// Validate cache was updated
	if sessionManager.IsSessionActive(session.ID) {
		t.Error("Ended session should not be in active cache")
	}
}

// TestPhase4_DatabaseSingleWriterPattern validates single-writer concurrency
func TestPhase4_DatabaseSingleWriterPattern(t *testing.T) {
	// Setup test database
	dbPath := "./test_phase4_concurrency.db"
	defer func() {
		if err := os.Remove(dbPath); err != nil {
			t.Logf("Failed to remove test database: %v", err)
		}
	}()
	
	// Initialize database with schema
	InitializeTestDatabase(t, dbPath)
	
	config := &dbconfig.Config{
		DatabasePath:     dbPath,
		MaxConnections:   10,
		ConnMaxLifetime:  time.Hour,
		ConnMaxIdleTime:  time.Minute * 10,
		MigrationsPath:   "../../migrations",
	}
	
	dbManager, err := database.NewManager(config)
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}
	defer func() {
		if err := dbManager.Close(); err != nil {
			t.Logf("Failed to close database manager: %v", err)
		}
	}()
	
	ctx := context.Background()
	
	// Concurrent session creation
	t.Run("ConcurrentWrites", func(t *testing.T) {
		const numGoroutines = 10
		errors := make(chan error, numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				session := &types.Session{
					ID:         fmt.Sprintf("concurrent-session-%d", idx),
					Name:       fmt.Sprintf("Concurrent Session %d", idx),
					CreatedBy:  "instructor1",
					StudentIDs: []string{"student1"},
					StartTime:  time.Now(),
					Status:     "active",
				}
				
				err := dbManager.CreateSession(ctx, session)
				errors <- err
			}(i)
		}
		
		// Collect results
		for i := 0; i < numGoroutines; i++ {
			if err := <-errors; err != nil {
				t.Errorf("Concurrent write %d failed: %v", i, err)
			}
		}
	})
	
	// Verify all sessions were created
	sessions, err := dbManager.ListActiveSessions(ctx)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}
	if len(sessions) != 10 {
		t.Errorf("Expected 10 sessions, got %d", len(sessions))
	}
}

// TestPhase4_MessagePersistence validates message storage and retrieval
func TestPhase4_MessagePersistence(t *testing.T) {
	// Setup test database
	dbPath := "./test_phase4_messages.db"
	defer func() {
		if err := os.Remove(dbPath); err != nil {
			t.Logf("Failed to remove test database: %v", err)
		}
	}()
	
	// Initialize database with schema
	InitializeTestDatabase(t, dbPath)
	
	config := &dbconfig.Config{
		DatabasePath:     dbPath,
		MaxConnections:   10,
		ConnMaxLifetime:  time.Hour,
		ConnMaxIdleTime:  time.Minute * 10,
		MigrationsPath:   "../../migrations",
	}
	
	dbManager, err := database.NewManager(config)
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}
	defer func() {
		if err := dbManager.Close(); err != nil {
			t.Logf("Failed to close database manager: %v", err)
		}
	}()
	
	ctx := context.Background()
	
	// Create a session first
	session := &types.Session{
		ID:         "message-test-session",
		Name:       "Message Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1", "student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	if err := dbManager.CreateSession(ctx, session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	// Store messages of different types
	messageTypes := []struct {
		msgType string
		fromUser string
		toUser  *string
	}{
		{"instructor_inbox", "student1", nil},
		{"inbox_response", "instructor1", &[]string{"student1"}[0]},
		{"request", "instructor1", &[]string{"student2"}[0]},
		{"request_response", "student2", nil},
		{"analytics", "student1", nil},
		{"instructor_broadcast", "instructor1", nil},
	}
	
	baseTime := time.Now()
	for i, mt := range messageTypes {
		message := &types.Message{
			ID:        fmt.Sprintf("msg-%d", i),
			SessionID: session.ID,
			Type:      mt.msgType,
			Context:   "general",
			FromUser:  mt.fromUser,
			ToUser:    mt.toUser,
			Content:   map[string]interface{}{"text": fmt.Sprintf("Test message %d", i)},
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
		}
		
		if err := dbManager.StoreMessage(ctx, message); err != nil {
			t.Fatalf("Failed to store message %s: %v", mt.msgType, err)
		}
	}
	
	// Retrieve session history
	messages, err := dbManager.GetSessionHistory(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get session history: %v", err)
	}
	
	// Validate message count
	if len(messages) != 6 {
		t.Errorf("Expected 6 messages, got %d", len(messages))
	}
	
	// Validate chronological order
	for i := 1; i < len(messages); i++ {
		if messages[i].Timestamp.Before(messages[i-1].Timestamp) {
			t.Error("Messages not in chronological order")
		}
	}
	
	// Validate message content preservation
	for i, msg := range messages {
		if text, ok := msg.Content["text"].(string); !ok || text != fmt.Sprintf("Test message %d", i) {
			t.Errorf("Message %d content not preserved correctly", i)
		}
	}
}

// TestPhase4_WebSocketIntegration validates session management with WebSocket
func TestPhase4_WebSocketIntegration(t *testing.T) {
	// Setup test database
	dbPath := "./test_phase4_websocket.db"
	defer func() {
		if err := os.Remove(dbPath); err != nil {
			t.Logf("Failed to remove test database: %v", err)
		}
	}()
	
	// Initialize database with schema
	InitializeTestDatabase(t, dbPath)
	
	config := &dbconfig.Config{
		DatabasePath:     dbPath,
		MaxConnections:   10,
		ConnMaxLifetime:  time.Hour,
		ConnMaxIdleTime:  time.Minute * 10,
		MigrationsPath:   "../../migrations",
	}
	
	dbManager, err := database.NewManager(config)
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}
	defer func() {
		if err := dbManager.Close(); err != nil {
			t.Logf("Failed to close database manager: %v", err)
		}
	}()
	
	// Initialize components
	sessionManager := session.NewManager(dbManager)
	_ = websocket.NewRegistry() // registry would be used in real integration
	
	ctx := context.Background()
	
	// Create a session
	session, err := sessionManager.CreateSession(ctx, "WebSocket Test", "instructor1", []string{"student1", "student2"})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	// Simulate WebSocket authentication validation
	t.Run("StudentValidation", func(t *testing.T) {
		// Valid student
		if err := sessionManager.ValidateSessionMembership(session.ID, "student1", "student"); err != nil {
			t.Errorf("Student1 validation failed: %v", err)
		}
		
		// Invalid student
		if err := sessionManager.ValidateSessionMembership(session.ID, "student3", "student"); err == nil {
			t.Error("Student3 should not have access")
		}
	})
	
	t.Run("InstructorValidation", func(t *testing.T) {
		// Any instructor should have access
		if err := sessionManager.ValidateSessionMembership(session.ID, "instructor2", "instructor"); err != nil {
			t.Errorf("Instructor validation failed: %v", err)
		}
	})
	
	// Test session termination effect on connections
	t.Run("SessionTermination", func(t *testing.T) {
		// End session
		if err := sessionManager.EndSession(ctx, session.ID); err != nil {
			t.Fatalf("Failed to end session: %v", err)
		}
		
		// Validation should now fail
		if err := sessionManager.ValidateSessionMembership(session.ID, "student1", "student"); err == nil {
			t.Error("Validation should fail for ended session")
		}
	})
}

// TestPhase4_PerformanceTargets validates performance requirements
func TestPhase4_PerformanceTargets(t *testing.T) {
	// Setup test database
	dbPath := "./test_phase4_performance.db"
	defer func() {
		if err := os.Remove(dbPath); err != nil {
			t.Logf("Failed to remove test database: %v", err)
		}
	}()
	
	// Initialize database with schema
	InitializeTestDatabase(t, dbPath)
	
	config := &dbconfig.Config{
		DatabasePath:     dbPath,
		MaxConnections:   10,
		ConnMaxLifetime:  time.Hour,
		ConnMaxIdleTime:  time.Minute * 10,
		MigrationsPath:   "../../migrations",
	}
	
	dbManager, err := database.NewManager(config)
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}
	defer func() {
		if err := dbManager.Close(); err != nil {
			t.Logf("Failed to close database manager: %v", err)
		}
	}()
	
	sessionManager := session.NewManager(dbManager)
	ctx := context.Background()
	
	// Create test session
	session, err := sessionManager.CreateSession(ctx, "Performance Test", "instructor1", []string{"student1", "student2", "student3"})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	// Populate with messages
	for i := 0; i < 1000; i++ {
		message := &types.Message{
			ID:        fmt.Sprintf("perf-msg-%d", i),
			SessionID: session.ID,
			Type:      "instructor_inbox",
			Context:   "general",
			FromUser:  "student1",
			ToUser:    nil,
			Content:   map[string]interface{}{"index": i},
			Timestamp: time.Now(),
		}
		
		start := time.Now()
		if err := dbManager.StoreMessage(ctx, message); err != nil {
			t.Fatalf("Failed to store message %d: %v", i, err)
		}
		elapsed := time.Since(start)
		
		if elapsed > 50*time.Millisecond {
			t.Errorf("Write operation %d took %v, exceeds 50ms target", i, elapsed)
		}
	}
	
	// Test history retrieval performance
	start := time.Now()
	messages, err := dbManager.GetSessionHistory(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get session history: %v", err)
	}
	elapsed := time.Since(start)
	
	if len(messages) != 1000 {
		t.Errorf("Expected 1000 messages, got %d", len(messages))
	}
	
	if elapsed > 100*time.Millisecond {
		t.Errorf("History retrieval took %v, exceeds 100ms target for 1000 messages", elapsed)
	}
	
	t.Logf("Retrieved %d messages in %v (target: <100ms)", len(messages), elapsed)
}