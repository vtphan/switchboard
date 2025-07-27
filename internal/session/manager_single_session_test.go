package session

import (
	"context"
	"sync"
	"testing"
	"time"

	"switchboard/pkg/interfaces"
	"switchboard/pkg/types"
)

// Test suite for single session enforcement functionality
// Following TDD methodology from .claude/commands/implement-step.md

// ARCHITECTURAL VALIDATION TESTS

func TestManager_SingleSessionEnforcement_ArchitecturalCompliance(t *testing.T) {
	// Verify interface compliance and clean boundaries
	var _ interfaces.SessionManager = (*Manager)(nil) // Should compile
	
	// Test that new methods exist and follow interface patterns
	manager := createTestManager(t)
	
	// Test that methods exist and work correctly
	hasActive, err := manager.HasActiveSession(context.Background())
	if err != nil {
		t.Errorf("HasActiveSession failed: %v", err)
	}
	
	// Should be false when no sessions exist
	if hasActive {
		t.Error("Expected no active session initially")
	}
	
	activeSession, err := manager.GetActiveSession(context.Background())
	if err != nil {
		t.Errorf("GetActiveSession failed: %v", err)
	}
	
	// Should be nil when no sessions exist
	if activeSession != nil {
		t.Error("Expected nil active session initially")
	}
}

// FUNCTIONAL VALIDATION TESTS

func TestManager_HasActiveSession_NoActiveSession(t *testing.T) {
	// Should fail - HasActiveSession method doesn't exist yet
	manager := createTestManager(t)
	
	hasActive, err := manager.HasActiveSession(context.Background())
	if err != nil {
		t.Fatalf("HasActiveSession failed: %v", err)
	}
	
	if hasActive {
		t.Error("Expected no active session when none exist")
	}
}

func TestManager_HasActiveSession_WithActiveSession(t *testing.T) {
	// Should fail - HasActiveSession method doesn't exist yet
	manager := createTestManager(t)
	
	// Create a session first
	session, err := manager.CreateSession(context.Background(), "Test Session", "instructor1", []string{"student1"})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	hasActive, err := manager.HasActiveSession(context.Background())
	if err != nil {
		t.Fatalf("HasActiveSession failed: %v", err)
	}
	
	if !hasActive {
		t.Error("Expected active session to be detected")
	}
	
	// Clean up
	_ = manager.EndSession(context.Background(), session.ID)
}

func TestManager_GetActiveSession_NoActiveSession(t *testing.T) {
	// Should fail - GetActiveSession method doesn't exist yet
	manager := createTestManager(t)
	
	session, err := manager.GetActiveSession(context.Background())
	if err != nil {
		t.Fatalf("GetActiveSession failed: %v", err)
	}
	
	if session != nil {
		t.Error("Expected nil session when no active session exists")
	}
}

func TestManager_GetActiveSession_WithActiveSession(t *testing.T) {
	// Should fail - GetActiveSession method doesn't exist yet
	manager := createTestManager(t)
	
	// Create a session first
	createdSession, err := manager.CreateSession(context.Background(), "Test Session", "instructor1", []string{"student1"})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	activeSession, err := manager.GetActiveSession(context.Background())
	if err != nil {
		t.Fatalf("GetActiveSession failed: %v", err)
	}
	
	if activeSession == nil {
		t.Fatal("Expected active session to be returned")
	}
	
	if activeSession.ID != createdSession.ID {
		t.Errorf("Expected session ID %s, got %s", createdSession.ID, activeSession.ID)
	}
	
	// Clean up
	_ = manager.EndSession(context.Background(), createdSession.ID)
}

func TestManager_CreateSession_RejectsWhenActiveExists(t *testing.T) {
	// Should fail - CreateSession doesn't validate existing sessions yet
	manager := createTestManager(t)
	
	// Create first session
	session1, err := manager.CreateSession(context.Background(), "First Session", "instructor1", []string{"student1"})
	if err != nil {
		t.Fatalf("Failed to create first session: %v", err)
	}
	
	// Try to create second session - should fail
	_, err = manager.CreateSession(context.Background(), "Second Session", "instructor1", []string{"student2"})
	if err == nil {
		t.Error("Expected CreateSession to fail when active session exists")
	}
	
	// Verify error type
	if err != ErrActiveSessionExists {
		t.Errorf("Expected ErrActiveSessionExists, got: %v", err)
	}
	
	// Clean up
	_ = manager.EndSession(context.Background(), session1.ID)
}

func TestManager_CreateSession_SucceedsWhenNoActiveSession(t *testing.T) {
	// Should pass - this tests existing functionality
	manager := createTestManager(t)
	
	// Verify no active session
	hasActive, err := manager.HasActiveSession(context.Background())
	if err != nil {
		t.Fatalf("HasActiveSession failed: %v", err)
	}
	if hasActive {
		t.Fatal("Expected no active session initially")
	}
	
	// Create session should succeed
	session, err := manager.CreateSession(context.Background(), "Test Session", "instructor1", []string{"student1"})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	if session == nil {
		t.Fatal("Expected session to be created")
	}
	
	// Clean up
	_ = manager.EndSession(context.Background(), session.ID)
}

// TECHNICAL VALIDATION TESTS

func TestManager_SingleSessionEnforcement_Concurrency(t *testing.T) {
	// Should fail initially - need to implement concurrent session creation protection
	manager := createTestManager(t)
	
	const numGoroutines = 10
	results := make(chan error, numGoroutines)
	
	// Try to create multiple sessions concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			_, err := manager.CreateSession(
				context.Background(), 
				"Concurrent Session", 
				"instructor1", 
				[]string{"student1"},
			)
			results <- err
		}(i)
	}
	
	// Collect results
	var successCount, failCount int
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		if err == nil {
			successCount++
		} else if err == ErrActiveSessionExists {
			failCount++
		} else {
			t.Errorf("Unexpected error: %v", err)
		}
	}
	
	// Only one should succeed, rest should fail with ErrActiveSessionExists
	if successCount != 1 {
		t.Errorf("Expected exactly 1 success, got %d", successCount)
	}
	
	if failCount != numGoroutines-1 {
		t.Errorf("Expected %d failures, got %d", numGoroutines-1, failCount)
	}
	
	// Clean up - end the session that was created
	if successCount > 0 {
		if activeSession, err := manager.GetActiveSession(context.Background()); err == nil && activeSession != nil {
			_ = manager.EndSession(context.Background(), activeSession.ID)
		}
	}
}

func TestManager_GetActiveSession_Performance(t *testing.T) {
	// Should pass - testing performance of cache lookup
	manager := createTestManager(t)
	
	// Create a session
	session, err := manager.CreateSession(context.Background(), "Performance Test", "instructor1", []string{"student1"})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	// Measure GetActiveSession performance
	start := time.Now()
	for i := 0; i < 1000; i++ {
		_, err := manager.GetActiveSession(context.Background())
		if err != nil {
			t.Fatalf("GetActiveSession failed on iteration %d: %v", i, err)
		}
	}
	duration := time.Since(start)
	
	// Should be fast (< 1ms average)
	avgDuration := duration / 1000
	if avgDuration > time.Millisecond {
		t.Errorf("GetActiveSession too slow: average %v, expected < 1ms", avgDuration)
	}
	
	// Clean up
	_ = manager.EndSession(context.Background(), session.ID)
}

// Helper functions

func createTestManager(t *testing.T) *Manager {
	// Create test database manager
	dbManager := createTestDBManager(t)
	
	// Create session manager
	manager := NewManager(dbManager)
	
	// Load active sessions
	if err := manager.LoadActiveSessions(context.Background()); err != nil {
		t.Fatalf("Failed to load active sessions: %v", err)
	}
	
	// Clean up on test completion
	t.Cleanup(func() {
		_ = manager.Close()
	})
	
	return manager
}

func createTestDBManager(t *testing.T) *TestDBManager {
	return &TestDBManager{
		sessions: make(map[string]*types.Session),
		messages: make(map[string][]*types.Message),
	}
}

// Mock database manager for testing
type TestDBManager struct {
	mu       sync.RWMutex
	sessions map[string]*types.Session
	messages map[string][]*types.Message
}

func (db *TestDBManager) CreateSession(ctx context.Context, session *types.Session) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.sessions[session.ID] = session
	return nil
}

func (db *TestDBManager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if session, exists := db.sessions[sessionID]; exists {
		return session, nil
	}
	return nil, ErrSessionNotFound
}

func (db *TestDBManager) UpdateSession(ctx context.Context, session *types.Session) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.sessions[session.ID] = session
	return nil
}

func (db *TestDBManager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	var active []*types.Session
	for _, session := range db.sessions {
		if session.Status == "active" {
			active = append(active, session)
		}
	}
	return active, nil
}

func (db *TestDBManager) StoreMessage(ctx context.Context, message *types.Message) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.messages[message.SessionID] = append(db.messages[message.SessionID], message)
	return nil
}

func (db *TestDBManager) GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.messages[sessionID], nil
}

func (db *TestDBManager) HealthCheck(ctx context.Context) error {
	return nil
}

func (db *TestDBManager) Close() error {
	return nil
}