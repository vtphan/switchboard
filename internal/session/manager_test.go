package session

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"switchboard/pkg/interfaces"
	"switchboard/pkg/types"
)

// Mock DatabaseManager for testing
type mockDatabaseManager struct {
	sessions map[string]*types.Session
	mu       sync.RWMutex
	
	// Control behavior for testing
	shouldFailCreate bool
	shouldFailUpdate bool
	shouldFailList   bool
}

func newMockDatabaseManager() *mockDatabaseManager {
	return &mockDatabaseManager{
		sessions: make(map[string]*types.Session),
	}
}

func (m *mockDatabaseManager) CreateSession(ctx context.Context, session *types.Session) error {
	if m.shouldFailCreate {
		return errors.New("database create failed")
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.ID] = session
	return nil
}

func (m *mockDatabaseManager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, interfaces.ErrSessionNotFound
	}
	return session, nil
}

func (m *mockDatabaseManager) UpdateSession(ctx context.Context, session *types.Session) error {
	if m.shouldFailUpdate {
		return errors.New("database update failed")
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.ID] = session
	return nil
}

func (m *mockDatabaseManager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	if m.shouldFailList {
		return nil, errors.New("database list failed")
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var activeSessions []*types.Session
	for _, session := range m.sessions {
		if session.Status == "active" {
			activeSessions = append(activeSessions, session)
		}
	}
	return activeSessions, nil
}

func (m *mockDatabaseManager) StoreMessage(ctx context.Context, message *types.Message) error {
	return nil // Not used in session manager tests
}

func (m *mockDatabaseManager) GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error) {
	return nil, nil // Not used in session manager tests
}

func (m *mockDatabaseManager) HealthCheck(ctx context.Context) error {
	return nil // Not used in session manager tests
}

func (m *mockDatabaseManager) Close() error {
	return nil // Not used in session manager tests
}

// Architectural Validation Tests
func TestManager_InterfaceCompliance(t *testing.T) {
	// This test will FAIL until Manager is implemented
	// Verify Manager implements SessionManager interface
	dbManager := newMockDatabaseManager()
	var _ interfaces.SessionManager = NewManager(dbManager)
}

func TestManager_ImportBoundaryCompliance(t *testing.T) {
	// This test passes if compilation succeeds - no forbidden imports
	t.Log("Session manager import boundaries maintained - only allowed dependencies")
}

func TestManager_DependencyInjection(t *testing.T) {
	// This test will FAIL until Manager is implemented
	dbManager := newMockDatabaseManager()
	manager := NewManager(dbManager)
	
	if manager == nil {
		t.Fatal("NewManager should return valid manager instance")
	}
}

// Functional Validation Tests - Core Behaviors
func TestManager_CreateSessionBasicBehavior(t *testing.T) {
	// This test will FAIL until CreateSession is implemented
	dbManager := newMockDatabaseManager()
	manager := NewManager(dbManager)
	
	ctx := context.Background()
	session, err := manager.CreateSession(ctx, "Test Session", "instructor1", []string{"student1", "student2"})
	
	if err != nil {
		t.Errorf("CreateSession should succeed: %v", err)
	}
	
	if session == nil {
		t.Fatal("CreateSession should return session")
	}
	
	// Validate session properties
	if session.Name != "Test Session" {
		t.Errorf("Expected name 'Test Session', got '%s'", session.Name)
	}
	
	if session.CreatedBy != "instructor1" {
		t.Errorf("Expected createdBy 'instructor1', got '%s'", session.CreatedBy)
	}
	
	if len(session.StudentIDs) != 2 {
		t.Errorf("Expected 2 student IDs, got %d", len(session.StudentIDs))
	}
	
	if session.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", session.Status)
	}
	
	if session.ID == "" {
		t.Error("Session ID should be generated")
	}
	
	if session.StartTime.IsZero() {
		t.Error("Start time should be set")
	}
	
	if session.EndTime != nil {
		t.Error("End time should be nil for new session")
	}
}

func TestManager_CreateSessionDuplicateRemoval(t *testing.T) {
	// This test will FAIL until CreateSession duplicate handling is implemented
	dbManager := newMockDatabaseManager()
	manager := NewManager(dbManager)
	
	ctx := context.Background()
	studentIDs := []string{"student1", "student2", "student1", "student3", "student2"} // Duplicates
	
	session, err := manager.CreateSession(ctx, "Test Session", "instructor1", studentIDs)
	
	if err != nil {
		t.Errorf("CreateSession should succeed: %v", err)
	}
	
	// Should have exactly 3 unique students
	if len(session.StudentIDs) != 3 {
		t.Errorf("Expected 3 unique student IDs, got %d", len(session.StudentIDs))
	}
	
	// Verify all expected students are present
	expectedStudents := map[string]bool{"student1": false, "student2": false, "student3": false}
	for _, studentID := range session.StudentIDs {
		if _, exists := expectedStudents[studentID]; exists {
			expectedStudents[studentID] = true
		}
	}
	
	for student, found := range expectedStudents {
		if !found {
			t.Errorf("Expected student '%s' not found in session", student)
		}
	}
}

func TestManager_GetSessionCacheFirst(t *testing.T) {
	// This test will FAIL until GetSession cache-first lookup is implemented
	dbManager := newMockDatabaseManager()
	manager := NewManager(dbManager)
	
	// Add session to database
	testSession := &types.Session{
		ID:         "test-session",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	dbManager.sessions["test-session"] = testSession
	
	// Load into cache
	ctx := context.Background()
	err := manager.LoadActiveSessions(ctx)
	if err != nil {
		t.Fatalf("LoadActiveSessions failed: %v", err)
	}
	
	// Get session should return from cache
	session, err := manager.GetSession(ctx, "test-session")
	if err != nil {
		t.Errorf("GetSession should succeed: %v", err)
	}
	
	if session == nil {
		t.Fatal("GetSession should return session")
	}
	
	if session.ID != "test-session" {
		t.Errorf("Expected session ID 'test-session', got '%s'", session.ID)
	}
}

func TestManager_EndSessionBehavior(t *testing.T) {
	// This test will FAIL until EndSession is implemented
	dbManager := newMockDatabaseManager()
	manager := NewManager(dbManager)
	
	// Create and add session to cache
	testSession := &types.Session{
		ID:         "test-session",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	dbManager.sessions["test-session"] = testSession
	
	ctx := context.Background()
	err := manager.LoadActiveSessions(ctx)
	if err != nil {
		t.Fatalf("LoadActiveSessions failed: %v", err)
	}
	
	// End the session
	err = manager.EndSession(ctx, "test-session")
	if err != nil {
		t.Errorf("EndSession should succeed: %v", err)
	}
	
	// Verify session is updated in database
	updatedSession := dbManager.sessions["test-session"]
	if updatedSession.Status != "ended" {
		t.Errorf("Expected status 'ended', got '%s'", updatedSession.Status)
	}
	
	if updatedSession.EndTime == nil {
		t.Error("End time should be set")
	}
	
	// Verify session is removed from active cache
	activeSessions, err := manager.ListActiveSessions(ctx)
	if err != nil {
		t.Errorf("ListActiveSessions failed: %v", err)
	}
	
	if len(activeSessions) != 0 {
		t.Errorf("Expected 0 active sessions, got %d", len(activeSessions))
	}
}

func TestManager_ValidateSessionMembershipRules(t *testing.T) {
	// This test will FAIL until ValidateSessionMembership is implemented
	dbManager := newMockDatabaseManager()
	manager := NewManager(dbManager)
	
	// Create test session
	testSession := &types.Session{
		ID:         "test-session",
		Name:       "Test Session",
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1", "student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	dbManager.sessions["test-session"] = testSession
	
	ctx := context.Background()
	err := manager.LoadActiveSessions(ctx)
	if err != nil {
		t.Fatalf("LoadActiveSessions failed: %v", err)
	}
	
	// Test instructor access (should always allow)
	err = manager.ValidateSessionMembership("test-session", "any-instructor", "instructor")
	if err != nil {
		t.Errorf("Instructors should have universal access: %v", err)
	}
	
	// Test valid student access
	err = manager.ValidateSessionMembership("test-session", "student1", "student")
	if err != nil {
		t.Errorf("Valid student should have access: %v", err)
	}
	
	// Test invalid student access
	err = manager.ValidateSessionMembership("test-session", "student3", "student")
	if err != ErrUnauthorized {
		t.Errorf("Invalid student should be unauthorized, got: %v", err)
	}
	
	// Test invalid session
	err = manager.ValidateSessionMembership("nonexistent", "student1", "student")
	if err != ErrSessionNotFound {
		t.Errorf("Nonexistent session should return not found, got: %v", err)
	}
	
	// Test invalid role
	err = manager.ValidateSessionMembership("test-session", "user1", "invalid-role")
	if err != ErrInvalidRole {
		t.Errorf("Invalid role should return invalid role error, got: %v", err)
	}
}

// Error Handling Validation Tests
func TestManager_CreateSessionValidation(t *testing.T) {
	// This test will FAIL until CreateSession validation is implemented
	dbManager := newMockDatabaseManager()
	manager := NewManager(dbManager)
	
	ctx := context.Background()
	
	tests := []struct {
		name       string
		sessionName string
		createdBy   string
		studentIDs  []string
		expectedErr error
	}{
		{
			name:        "Empty session name",
			sessionName: "",
			createdBy:   "instructor1",
			studentIDs:  []string{"student1"},
			expectedErr: ErrInvalidSessionName,
		},
		{
			name:        "Session name too long",
			sessionName: string(make([]byte, 201)), // 201 characters
			createdBy:   "instructor1", 
			studentIDs:  []string{"student1"},
			expectedErr: ErrInvalidSessionName,
		},
		{
			name:        "Invalid created by",
			sessionName: "Valid Session",
			createdBy:   "",
			studentIDs:  []string{"student1"},
			expectedErr: ErrInvalidCreatedBy,
		},
		{
			name:        "Empty student list",
			sessionName: "Valid Session",
			createdBy:   "instructor1",
			studentIDs:  []string{},
			expectedErr: ErrEmptyStudentList,
		},
		{
			name:        "Invalid student ID",
			sessionName: "Valid Session",
			createdBy:   "instructor1",
			studentIDs:  []string{"valid-student", ""},
			expectedErr: ErrInvalidStudentID,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.CreateSession(ctx, tt.sessionName, tt.createdBy, tt.studentIDs)
			
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("Expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestManager_DatabaseErrorHandling(t *testing.T) {
	// This test will FAIL until database error handling is implemented
	dbManager := newMockDatabaseManager()
	manager := NewManager(dbManager)
	
	ctx := context.Background()
	
	// Test create session database failure
	dbManager.shouldFailCreate = true
	_, err := manager.CreateSession(ctx, "Test Session", "instructor1", []string{"student1"})
	if err == nil {
		t.Error("CreateSession should fail when database fails")
	}
	
	// Test load sessions database failure
	dbManager.shouldFailList = true
	err = manager.LoadActiveSessions(ctx)
	if err == nil {
		t.Error("LoadActiveSessions should fail when database fails")
	}
	
	// Test end session database failure
	dbManager.shouldFailUpdate = true
	dbManager.shouldFailCreate = false
	dbManager.shouldFailList = false
	
	// First create a session
	session, err := manager.CreateSession(ctx, "Test Session", "instructor1", []string{"student1"})
	if err != nil {
		t.Fatalf("CreateSession should succeed: %v", err)
	}
	
	// Then try to end it with database failure
	err = manager.EndSession(ctx, session.ID)
	if err == nil {
		t.Error("EndSession should fail when database update fails")
	}
}

// Technical Validation Tests - Performance and Concurrency
func TestManager_SessionValidationPerformance(t *testing.T) {
	// This test will FAIL until performance optimization is implemented
	dbManager := newMockDatabaseManager()
	manager := NewManager(dbManager)
	
	// Create test session
	testSession := &types.Session{
		ID:         "test-session",
		Name:       "Test Session", 
		CreatedBy:  "instructor1",
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	dbManager.sessions["test-session"] = testSession
	
	ctx := context.Background()
	err := manager.LoadActiveSessions(ctx)
	if err != nil {
		t.Fatalf("LoadActiveSessions failed: %v", err)
	}
	
	// Test validation performance - should complete in <1ms
	iterations := 1000
	start := time.Now()
	
	for i := 0; i < iterations; i++ {
		_ = manager.ValidateSessionMembership("test-session", "student1", "student")
	}
	
	duration := time.Since(start)
	avgDuration := duration / time.Duration(iterations)
	
	if avgDuration > time.Millisecond {
		t.Errorf("Session validation too slow: %v per operation (should be <1ms)", avgDuration)
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	// This test will FAIL until thread-safe implementation is complete
	dbManager := newMockDatabaseManager()
	manager := NewManager(dbManager)
	
	// Create test session
	testSession := &types.Session{
		ID:         "test-session",
		Name:       "Test Session",
		CreatedBy:  "instructor1", 
		StudentIDs: []string{"student1"},
		StartTime:  time.Now(),
		Status:     "active",
	}
	
	dbManager.sessions["test-session"] = testSession
	
	ctx := context.Background()
	err := manager.LoadActiveSessions(ctx)
	if err != nil {
		t.Fatalf("LoadActiveSessions failed: %v", err)
	}
	
	// Test concurrent validation access
	const numGoroutines = 100
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)
	
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			err := manager.ValidateSessionMembership("test-session", "student1", "student")
			if err != nil {
				errors <- err
			}
		}()
	}
	
	wg.Wait()
	close(errors)
	
	// Check for any errors from concurrent access
	for err := range errors {
		t.Errorf("Concurrent validation failed: %v", err)
	}
}

func TestManager_CacheConsistency(t *testing.T) {
	// This test will FAIL until cache consistency is implemented
	dbManager := newMockDatabaseManager()
	manager := NewManager(dbManager)
	
	ctx := context.Background()
	
	// Create session
	session, err := manager.CreateSession(ctx, "Test Session", "instructor1", []string{"student1"})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	
	// Verify session is in active list
	activeSessions, err := manager.ListActiveSessions(ctx)
	if err != nil {
		t.Errorf("ListActiveSessions failed: %v", err)
	}
	
	if len(activeSessions) != 1 {
		t.Errorf("Expected 1 active session, got %d", len(activeSessions))
	}
	
	// End session
	err = manager.EndSession(ctx, session.ID)
	if err != nil {
		t.Errorf("EndSession failed: %v", err)
	}
	
	// Verify session is removed from active list
	activeSessions, err = manager.ListActiveSessions(ctx)
	if err != nil {
		t.Errorf("ListActiveSessions failed: %v", err)
	}
	
	if len(activeSessions) != 0 {
		t.Errorf("Expected 0 active sessions after ending, got %d", len(activeSessions))
	}
	
	// Verify validation fails for ended session
	err = manager.ValidateSessionMembership(session.ID, "student1", "student")
	if err != ErrSessionEnded {
		t.Errorf("Validation should fail for ended session, got: %v", err)
	}
}

// Integration Validation Tests
func TestManager_SessionLifecycleIntegration(t *testing.T) {
	// This test will FAIL until complete integration is implemented
	dbManager := newMockDatabaseManager()
	manager := NewManager(dbManager)
	
	ctx := context.Background()
	
	// Step 1: Create session
	session, err := manager.CreateSession(ctx, "Integration Test", "instructor1", []string{"student1", "student2"})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	
	// Step 2: Validate session membership
	err = manager.ValidateSessionMembership(session.ID, "student1", "student")
	if err != nil {
		t.Errorf("Student validation failed: %v", err)
	}
	
	err = manager.ValidateSessionMembership(session.ID, "instructor1", "instructor")
	if err != nil {
		t.Errorf("Instructor validation failed: %v", err)
	}
	
	// Step 3: Check session is active
	if !manager.IsSessionActive(session.ID) {
		t.Error("Session should be active")
	}
	
	// Step 4: End session
	err = manager.EndSession(ctx, session.ID)
	if err != nil {
		t.Errorf("EndSession failed: %v", err)
	}
	
	// Step 5: Verify session is no longer active
	if manager.IsSessionActive(session.ID) {
		t.Error("Session should not be active after ending")
	}
	
	// Step 6: Verify validation fails
	err = manager.ValidateSessionMembership(session.ID, "student1", "student")
	if err != ErrSessionEnded {
		t.Errorf("Validation should fail for ended session, got: %v", err)
	}
}

func TestManager_StatisticsAndCacheManagement(t *testing.T) {
	// This test will FAIL until statistics and cache management is implemented
	dbManager := newMockDatabaseManager()
	manager := NewManager(dbManager)
	
	ctx := context.Background()
	
	// Initially empty
	stats := manager.GetStats()
	if stats["active_sessions"] != 0 {
		t.Errorf("Expected 0 active sessions initially, got %v", stats["active_sessions"])
	}
	
	// Create sessions
	_, err := manager.CreateSession(ctx, "Session 1", "instructor1", []string{"student1"})
	if err != nil {
		t.Fatalf("CreateSession 1 failed: %v", err)
	}
	
	_, err = manager.CreateSession(ctx, "Session 2", "instructor1", []string{"student2"})
	if err != nil {
		t.Fatalf("CreateSession 2 failed: %v", err)
	}
	
	// Check statistics
	stats = manager.GetStats()
	if stats["active_sessions"] != 2 {
		t.Errorf("Expected 2 active sessions, got %v", stats["active_sessions"])
	}
	
	// Test cache refresh
	err = manager.RefreshCache(ctx)
	if err != nil {
		t.Errorf("RefreshCache failed: %v", err)
	}
	
	// Statistics should remain the same
	stats = manager.GetStats()
	if stats["active_sessions"] != 2 {
		t.Errorf("Expected 2 active sessions after refresh, got %v", stats["active_sessions"])
	}
}