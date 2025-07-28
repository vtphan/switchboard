package session

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"switchboard/pkg/types"
)

// MockDatabaseManager implements a mock database manager
type MockDatabaseManager struct {
	mock.Mock
}

func (m *MockDatabaseManager) StoreMessage(ctx context.Context, message *types.Message) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockDatabaseManager) StoreMessageBatch(ctx context.Context, messages []*types.Message) error {
	args := m.Called(ctx, messages)
	return args.Error(0)
}

func (m *MockDatabaseManager) GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Message), args.Error(1)
}

func (m *MockDatabaseManager) CreateSession(ctx context.Context, session *types.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockDatabaseManager) UpdateSession(ctx context.Context, session *types.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockDatabaseManager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Session), args.Error(1)
}

func (m *MockDatabaseManager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.Session), args.Error(1)
}

func (m *MockDatabaseManager) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDatabaseManager) Close() error {
	args := m.Called()
	return args.Error(0)
}

// TestCreateSession_Success tests successful session creation
func TestCreateSession_Success(t *testing.T) {
	dbManager := new(MockDatabaseManager)
	manager := NewManager(dbManager)

	// Mock empty active sessions
	dbManager.On("ListActiveSessions", mock.Anything).Return([]*types.Session{}, nil).Once()
	if err := manager.LoadActiveSessions(context.Background()); err != nil {
		t.Fatalf("Failed to load active sessions: %v", err)
	}

	// Mock successful database creation
	dbManager.On("CreateSession", mock.Anything, mock.MatchedBy(func(s *types.Session) bool {
		return s.Name == "Test Session" && 
			s.CreatedBy == "instructor1" && 
			len(s.StudentIDs) == 3 &&
			s.Status == "active"
	})).Return(nil)

	// Create session
	session, err := manager.CreateSession(context.Background(), "Test Session", "instructor1", 
		[]string{"student1", "student2", "student3"})

	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, "Test Session", session.Name)
	assert.Equal(t, "instructor1", session.CreatedBy)
	assert.Equal(t, []string{"student1", "student2", "student3"}, session.StudentIDs)
	assert.Equal(t, "active", session.Status)
	assert.NotEmpty(t, session.ID)
	assert.Nil(t, session.EndTime)

	// Verify session is in cache
	assert.True(t, manager.IsSessionActive(session.ID))
}

// TestCreateSession_SingleSessionEnforcement tests that only one active session is allowed
func TestCreateSession_SingleSessionEnforcement(t *testing.T) {
	dbManager := new(MockDatabaseManager)
	manager := NewManager(dbManager)

	// Create first session
	dbManager.On("ListActiveSessions", mock.Anything).Return([]*types.Session{}, nil).Once()
	if err := manager.LoadActiveSessions(context.Background()); err != nil {
		t.Logf("Failed to load active sessions: %v", err)
	}

	dbManager.On("CreateSession", mock.Anything, mock.Anything).Return(nil).Once()

	session1, err := manager.CreateSession(context.Background(), "First Session", "instructor1", 
		[]string{"student1", "student2"})
	assert.NoError(t, err)
	assert.NotNil(t, session1)

	// Try to create second session - should fail
	session2, err := manager.CreateSession(context.Background(), "Second Session", "instructor2", 
		[]string{"student3", "student4"})
	
	assert.Error(t, err)
	assert.Equal(t, ErrActiveSessionExists, err)
	assert.Nil(t, session2)

	// Verify only one active session exists
	activeSessions, _ := manager.ListActiveSessions(context.Background())
	assert.Len(t, activeSessions, 1)
	assert.Equal(t, session1.ID, activeSessions[0].ID)
}

// TestCreateSession_ValidationErrors tests various validation errors
func TestCreateSession_ValidationErrors(t *testing.T) {
	dbManager := new(MockDatabaseManager)
	manager := NewManager(dbManager)

	// Mock empty active sessions
	dbManager.On("ListActiveSessions", mock.Anything).Return([]*types.Session{}, nil).Once()
	if err := manager.LoadActiveSessions(context.Background()); err != nil {
		t.Fatalf("Failed to load active sessions: %v", err)
	}

	tests := []struct {
		name       string
		sessionName string
		createdBy  string
		studentIDs []string
		wantErr    error
	}{
		{
			name:       "empty session name",
			sessionName: "",
			createdBy:  "instructor1",
			studentIDs: []string{"student1"},
			wantErr:    ErrInvalidSessionName,
		},
		{
			name:       "session name too long",
			sessionName: string(make([]byte, 201)),
			createdBy:  "instructor1",
			studentIDs: []string{"student1"},
			wantErr:    ErrInvalidSessionName,
		},
		{
			name:       "invalid creator ID",
			sessionName: "Test Session",
			createdBy:  "invalid@user",
			studentIDs: []string{"student1"},
			wantErr:    ErrInvalidCreatedBy,
		},
		{
			name:       "empty student list",
			sessionName: "Test Session",
			createdBy:  "instructor1",
			studentIDs: []string{},
			wantErr:    ErrEmptyStudentList,
		},
		{
			name:       "invalid student ID",
			sessionName: "Test Session",
			createdBy:  "instructor1",
			studentIDs: []string{"student1", "invalid@student"},
			wantErr:    ErrInvalidStudentID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := manager.CreateSession(context.Background(), tt.sessionName, tt.createdBy, tt.studentIDs)
			assert.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, session)
		})
	}
}

// TestCreateSession_RemovesDuplicates tests duplicate student ID removal
func TestCreateSession_RemovesDuplicates(t *testing.T) {
	dbManager := new(MockDatabaseManager)
	manager := NewManager(dbManager)

	dbManager.On("ListActiveSessions", mock.Anything).Return([]*types.Session{}, nil).Once()
	if err := manager.LoadActiveSessions(context.Background()); err != nil {
		t.Logf("Failed to load active sessions: %v", err)
	}

	// Mock database creation - verify duplicates are removed
	dbManager.On("CreateSession", mock.Anything, mock.MatchedBy(func(s *types.Session) bool {
		return len(s.StudentIDs) == 3 // Should be 3 unique students, not 5
	})).Return(nil)

	// Create session with duplicate student IDs
	session, err := manager.CreateSession(context.Background(), "Test Session", "instructor1", 
		[]string{"student1", "student2", "student1", "student3", "student2"})

	assert.NoError(t, err)
	assert.Equal(t, []string{"student1", "student2", "student3"}, session.StudentIDs)
}

// TestEndSession_Success tests successful session ending
func TestEndSession_Success(t *testing.T) {
	dbManager := new(MockDatabaseManager)
	manager := NewManager(dbManager)

	// Create a session first
	dbManager.On("ListActiveSessions", mock.Anything).Return([]*types.Session{}, nil).Once()
	if err := manager.LoadActiveSessions(context.Background()); err != nil {
		t.Logf("Failed to load active sessions: %v", err)
	}

	dbManager.On("CreateSession", mock.Anything, mock.Anything).Return(nil)
	session, _ := manager.CreateSession(context.Background(), "Test Session", "instructor1", 
		[]string{"student1"})

	// Mock successful update
	dbManager.On("UpdateSession", mock.Anything, mock.MatchedBy(func(s *types.Session) bool {
		return s.ID == session.ID && s.Status == "ended" && s.EndTime != nil
	})).Return(nil)

	// End session
	err := manager.EndSession(context.Background(), session.ID)
	assert.NoError(t, err)

	// Verify session is no longer active in cache
	assert.False(t, manager.IsSessionActive(session.ID))
}

// TestEndSession_Errors tests error cases for ending sessions
func TestEndSession_Errors(t *testing.T) {
	dbManager := new(MockDatabaseManager)
	manager := NewManager(dbManager)

	// Test ending non-existent session
	dbManager.On("GetSession", mock.Anything, "non-existent").Return(nil, ErrSessionNotFound)
	err := manager.EndSession(context.Background(), "non-existent")
	assert.Equal(t, ErrSessionNotFound, err)

	// Test ending already ended session
	endedSession := &types.Session{
		ID:     "ended-session",
		Status: "ended",
	}
	dbManager.On("GetSession", mock.Anything, "ended-session").Return(endedSession, nil)
	err = manager.EndSession(context.Background(), "ended-session")
	assert.Equal(t, ErrSessionAlreadyEnded, err)
}

// TestValidateSessionMembership tests session membership validation
func TestValidateSessionMembership(t *testing.T) {
	dbManager := new(MockDatabaseManager)
	manager := NewManager(dbManager)

	// Create active session
	activeSession := &types.Session{
		ID:         "session1",
		Name:       "Test Session",
		StudentIDs: []string{"student1", "student2", "student3"},
		Status:     "active",
	}
	manager.activeSessions[activeSession.ID] = activeSession

	tests := []struct {
		name      string
		sessionID string
		userID    string
		role      string
		wantErr   error
	}{
		{
			name:      "instructor access allowed",
			sessionID: "session1",
			userID:    "instructor1",
			role:      "instructor",
			wantErr:   nil,
		},
		{
			name:      "enrolled student allowed",
			sessionID: "session1",
			userID:    "student1",
			role:      "student",
			wantErr:   nil,
		},
		{
			name:      "non-enrolled student denied",
			sessionID: "session1",
			userID:    "student4",
			role:      "student",
			wantErr:   ErrUnauthorized,
		},
		{
			name:      "invalid role",
			sessionID: "session1",
			userID:    "user1",
			role:      "admin",
			wantErr:   ErrInvalidRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidateSessionMembership(tt.sessionID, tt.userID, tt.role)
			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestConcurrentSessionOperations tests thread safety of session operations
func TestConcurrentSessionOperations(t *testing.T) {
	dbManager := new(MockDatabaseManager)
	manager := NewManager(dbManager)

	dbManager.On("ListActiveSessions", mock.Anything).Return([]*types.Session{}, nil).Once()
	if err := manager.LoadActiveSessions(context.Background()); err != nil {
		t.Logf("Failed to load active sessions: %v", err)
	}

	// Mock database operations - only first CreateSession should succeed
	dbManager.On("CreateSession", mock.Anything, mock.Anything).Return(nil).Once()
	// Mock GetSession to return session ended status for subsequent EndSession calls
	dbManager.On("GetSession", mock.Anything, mock.Anything).Return(
		&types.Session{Status: "ended"}, nil).Maybe()
	dbManager.On("UpdateSession", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Create first session
	session, err := manager.CreateSession(context.Background(), "Test Session", "instructor1", 
		[]string{"student1"})
	require.NoError(t, err)

	// Run concurrent operations
	var wg sync.WaitGroup
	errors := make([]error, 0)
	var mu sync.Mutex

	// Try to create multiple sessions concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := manager.CreateSession(context.Background(), 
				"Concurrent Session", "instructor2", []string{"student2"})
			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}(i)
	}

	// Wait for all create attempts to complete before trying to end
	wg.Wait()

	// Try to end the session concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = manager.EndSession(context.Background(), session.ID)
		}()
	}

	wg.Wait()

	// All create attempts should fail with ErrActiveSessionExists
	for _, err := range errors {
		assert.Equal(t, ErrActiveSessionExists, err)
	}
}

// TestLoadActiveSessions tests loading sessions from database
func TestLoadActiveSessions(t *testing.T) {
	dbManager := new(MockDatabaseManager)
	manager := NewManager(dbManager)

	// Mock database response
	sessions := []*types.Session{
		{
			ID:     "session1",
			Name:   "Active Session 1",
			Status: "active",
		},
		{
			ID:     "session2",
			Name:   "Active Session 2",
			Status: "active",
		},
	}

	dbManager.On("ListActiveSessions", mock.Anything).Return(sessions, nil)

	// Load sessions
	err := manager.LoadActiveSessions(context.Background())
	assert.NoError(t, err)

	// Verify sessions are in cache
	assert.True(t, manager.IsSessionActive("session1"))
	assert.True(t, manager.IsSessionActive("session2"))

	// Verify count
	activeSessions, _ := manager.ListActiveSessions(context.Background())
	assert.Len(t, activeSessions, 2)
}

// TestGetActiveSession tests retrieving the active session
func TestGetActiveSession(t *testing.T) {
	dbManager := new(MockDatabaseManager)
	manager := NewManager(dbManager)

	// No active session
	session, err := manager.GetActiveSession(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, session)

	// Create active session
	dbManager.On("ListActiveSessions", mock.Anything).Return([]*types.Session{}, nil).Once()
	if err := manager.LoadActiveSessions(context.Background()); err != nil {
		t.Logf("Failed to load active sessions: %v", err)
	}

	dbManager.On("CreateSession", mock.Anything, mock.Anything).Return(nil)
	createdSession, _ := manager.CreateSession(context.Background(), "Test Session", "instructor1", 
		[]string{"student1"})

	// Get active session
	activeSession, err := manager.GetActiveSession(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, activeSession)
	assert.Equal(t, createdSession.ID, activeSession.ID)
}

// TestContextCancellation tests handling of context cancellation
func TestContextCancellation(t *testing.T) {
	dbManager := new(MockDatabaseManager)
	manager := NewManager(dbManager)

	// Mock database calls that might happen before context is checked
	dbManager.On("ListActiveSessions", mock.Anything).Return([]*types.Session{}, nil).Maybe()
	dbManager.On("CreateSession", mock.Anything, mock.Anything).Return(context.Canceled).Maybe()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to create session with cancelled context
	session, err := manager.CreateSession(ctx, "Test Session", "instructor1", []string{"student1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
	assert.Nil(t, session)
}

// TestRefreshCache tests cache refresh functionality
func TestRefreshCache(t *testing.T) {
	dbManager := new(MockDatabaseManager)
	manager := NewManager(dbManager)

	// Initial load
	dbManager.On("ListActiveSessions", mock.Anything).Return([]*types.Session{
		{ID: "session1", Status: "active"},
	}, nil).Once()
	if err := manager.LoadActiveSessions(context.Background()); err != nil {
		t.Logf("Failed to load active sessions: %v", err)
	}

	// Verify initial state
	assert.True(t, manager.IsSessionActive("session1"))

	// Mock new database state
	dbManager.On("ListActiveSessions", mock.Anything).Return([]*types.Session{
		{ID: "session2", Status: "active"},
		{ID: "session3", Status: "active"},
	}, nil).Once()

	// Refresh cache
	err := manager.RefreshCache(context.Background())
	assert.NoError(t, err)

	// Verify new state
	assert.False(t, manager.IsSessionActive("session1"))
	assert.True(t, manager.IsSessionActive("session2"))
	assert.True(t, manager.IsSessionActive("session3"))
}

// TestGetStats tests statistics gathering
func TestGetStats(t *testing.T) {
	dbManager := new(MockDatabaseManager)
	manager := NewManager(dbManager)

	// Add some sessions
	manager.activeSessions["session1"] = &types.Session{ID: "session1", Status: "active"}
	manager.activeSessions["session2"] = &types.Session{ID: "session2", Status: "active"}

	stats := manager.GetStats()
	assert.Equal(t, 2, stats["active_sessions"])
	assert.Equal(t, 2, stats["cache_size"])
}