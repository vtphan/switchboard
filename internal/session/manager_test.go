package session

import (
	"fmt"
	"reflect"
	"testing"
	"sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/pkg/errors"
)

// TestArchitectural_SessionManagerInterface verifies the SessionManager interface
// exists with exact method signatures required by integration contracts
func TestArchitectural_SessionManagerInterface(t *testing.T) {
	// Test that SessionManager interface exists
	sessionManagerType := reflect.TypeOf((*SessionManager)(nil)).Elem()
	assert.Equal(t, "SessionManager", sessionManagerType.Name())
	assert.True(t, sessionManagerType.Kind() == reflect.Interface)

	// Verify exact method count - must be exactly 5 methods
	assert.Equal(t, 5, sessionManagerType.NumMethod())

	// GetActiveSession() *Session
	getMethod, exists := sessionManagerType.MethodByName("GetActiveSession")
	require.True(t, exists, "GetActiveSession method must exist")
	assert.Equal(t, 0, getMethod.Type.NumIn()) // no parameters (except receiver)
	assert.Equal(t, 1, getMethod.Type.NumOut()) // returns *Session
	
	// SetActiveSession(session *Session) error
	setMethod, exists := sessionManagerType.MethodByName("SetActiveSession") 
	require.True(t, exists, "SetActiveSession method must exist")
	assert.Equal(t, 1, setMethod.Type.NumIn()) // one parameter
	assert.Equal(t, 1, setMethod.Type.NumOut()) // returns error
	
	// ClearActiveSession() error
	clearMethod, exists := sessionManagerType.MethodByName("ClearActiveSession")
	require.True(t, exists, "ClearActiveSession method must exist") 
	assert.Equal(t, 0, clearMethod.Type.NumIn()) // no parameters
	assert.Equal(t, 1, clearMethod.Type.NumOut()) // returns error
	
	// HasActiveSession() bool
	hasMethod, exists := sessionManagerType.MethodByName("HasActiveSession")
	require.True(t, exists, "HasActiveSession method must exist")
	assert.Equal(t, 0, hasMethod.Type.NumIn()) // no parameters
	assert.Equal(t, 1, hasMethod.Type.NumOut()) // returns bool
}

// TestArchitectural_SessionManagerImplStruct verifies the SessionManagerImpl struct
// has the exact fields required for atomic session management
func TestArchitectural_SessionManagerImplStruct(t *testing.T) {
	// This test should FAIL until implementation exists
	sessionManagerImplType := reflect.TypeOf(SessionManagerImpl{})
	assert.Equal(t, "SessionManagerImpl", sessionManagerImplType.Name())
	assert.True(t, sessionManagerImplType.Kind() == reflect.Struct)

	// Verify required fields exist with correct types
	activeSessionField, exists := sessionManagerImplType.FieldByName("activeSessionMu")
	require.True(t, exists, "activeSessionMu field must exist")
	assert.Equal(t, "RWMutex", activeSessionField.Type.Name())

	activeSessionField, exists = sessionManagerImplType.FieldByName("activeSession")
	require.True(t, exists, "activeSession field must exist")
	assert.Equal(t, "*database.Session", activeSessionField.Type.String())

	dbManagerField, exists := sessionManagerImplType.FieldByName("dbManager")
	require.True(t, exists, "dbManager field must exist")
	assert.Equal(t, "DatabaseManager", dbManagerField.Type.Name())
}

// TestArchitectural_ImportBoundaries ensures clean dependency structure
func TestArchitectural_ImportBoundaries(t *testing.T) {
	// This package should only import:
	// - Standard library (sync, etc.)
	// - Internal database package (for DatabaseManager, Session)
	// - Internal errors package
	// 
	// FORBIDDEN imports:
	// - internal/websocket (would create circular dependency)
	// - internal/message (would create circular dependency)
	// - web/* packages (wrong layer)

	// Note: This is validated at compile time, but we test the types are available
	// Verify we can reference required types from Phase 1
	assert.NotNil(t, reflect.TypeOf((*database.DatabaseManager)(nil)).Elem())
	assert.NotNil(t, reflect.TypeOf((*database.Session)(nil)))
	assert.NotNil(t, reflect.TypeOf((*error)(nil)).Elem())

	// Verify required error types are accessible
	assert.Equal(t, "no active session", errors.ErrNoActiveSession.Error())
	assert.Equal(t, "session already active", errors.ErrSessionAlreadyActive.Error())
}

// TestArchitectural_InterfaceCompliance verifies SessionManagerImpl implements
// SessionManager interface exactly  
func TestArchitectural_InterfaceCompliance(t *testing.T) {
	// This test should FAIL until implementation exists
	var impl SessionManagerImpl
	var _ SessionManager = &impl // Compile-time interface compliance check

	// Verify implementation is assignable to interface
	sessionManagerType := reflect.TypeOf((*SessionManager)(nil)).Elem()
	implType := reflect.TypeOf(&impl)
	assert.True(t, implType.Implements(sessionManagerType))
}

// TestArchitectural_MutexPattern verifies the exact RWMutex pattern
// specified in tech specs for read-heavy session access
func TestArchitectural_MutexPattern(t *testing.T) {
	// This test should FAIL until implementation exists
	impl := &SessionManagerImpl{}

	// Verify RWMutex field exists and is initialized properly
	value := reflect.ValueOf(impl).Elem()
	mutexField := value.FieldByName("activeSessionMu")
	require.True(t, mutexField.IsValid(), "activeSessionMu field must exist")
	
	// Verify it's a sync.RWMutex type
	mutexType := mutexField.Type()
	assert.Equal(t, "sync.RWMutex", mutexType.String())
}

// TestFunctional_GetActiveSession verifies session retrieval behavior
func TestFunctional_GetActiveSession(t *testing.T) {
	// This test should FAIL until implementation exists
	impl := &SessionManagerImpl{}
	
	// Initially should return nil (no active session)
	session := impl.GetActiveSession()
	assert.Nil(t, session)
}

// TestFunctional_SetActiveSession verifies session setting behavior
func TestFunctional_SetActiveSession(t *testing.T) {
	// This test should FAIL until implementation exists
	impl := &SessionManagerImpl{}
	testSession := &database.Session{
		ID:   "test-session-123",
		Name: "Test Session",
	}
	
	// Should be able to set session when none active
	err := impl.SetActiveSession(testSession)
	assert.NoError(t, err)
	
	// Should reject setting another session when one is already active
	anotherSession := &database.Session{
		ID:   "another-session-456", 
		Name: "Another Session",
	}
	err = impl.SetActiveSession(anotherSession)
	assert.Equal(t, errors.ErrSessionAlreadyActive, err)
}

// TestFunctional_ClearActiveSession verifies session clearing behavior
func TestFunctional_ClearActiveSession(t *testing.T) {
	// This test should FAIL until implementation exists
	impl := &SessionManagerImpl{}
	
	// Should fail to clear when no session is active
	err := impl.ClearActiveSession()
	assert.Equal(t, errors.ErrNoActiveSession, err)
	
	// Set a session first
	testSession := &database.Session{
		ID:   "test-session-123",
		Name: "Test Session",
	}
	err = impl.SetActiveSession(testSession)
	assert.NoError(t, err)
	
	// Should successfully clear active session
	err = impl.ClearActiveSession()
	assert.NoError(t, err)
	
	// Verify session is cleared
	session := impl.GetActiveSession()
	assert.Nil(t, session)
}

// TestFunctional_HasActiveSession verifies session status checking
func TestFunctional_HasActiveSession(t *testing.T) {
	// This test should FAIL until implementation exists
	impl := &SessionManagerImpl{}
	
	// Initially should return false
	hasSession := impl.HasActiveSession()
	assert.False(t, hasSession)
	
	// Set a session
	testSession := &database.Session{
		ID:   "test-session-123", 
		Name: "Test Session",
	}
	err := impl.SetActiveSession(testSession)
	assert.NoError(t, err)
	
	// Should return true when session is active
	hasSession = impl.HasActiveSession()
	assert.True(t, hasSession)
	
	// Clear session
	err = impl.ClearActiveSession()
	assert.NoError(t, err)
	
	// Should return false after clearing
	hasSession = impl.HasActiveSession()
	assert.False(t, hasSession)
}

// TestFunctional_SessionStateConsistency verifies atomic state changes
func TestFunctional_SessionStateConsistency(t *testing.T) {
	// This test should FAIL until implementation exists
	impl := &SessionManagerImpl{}
	testSession := &database.Session{
		ID:   "test-session-123",
		Name: "Test Session",
	}
	
	// Set, verify, and clear in sequence
	err := impl.SetActiveSession(testSession)
	require.NoError(t, err)
	
	retrieved := impl.GetActiveSession()
	require.NotNil(t, retrieved)
	assert.Equal(t, testSession.ID, retrieved.ID)
	assert.Equal(t, testSession.Name, retrieved.Name)
	
	hasSession := impl.HasActiveSession()
	assert.True(t, hasSession)
	
	err = impl.ClearActiveSession()
	require.NoError(t, err)
	
	retrieved = impl.GetActiveSession()
	assert.Nil(t, retrieved)
	
	hasSession = impl.HasActiveSession()
	assert.False(t, hasSession)
}

// TestTechnical_ConcurrentReads verifies read-heavy access pattern
func TestTechnical_ConcurrentReads(t *testing.T) {
	// This test should FAIL until implementation exists
	impl := &SessionManagerImpl{}
	testSession := &database.Session{
		ID:   "concurrent-test-session",
		Name: "Concurrent Test",
	}
	
	// Set an active session
	err := impl.SetActiveSession(testSession)
	require.NoError(t, err)
	
	// Concurrent reads should all succeed and return the same session
	const numReaders = 50
	const readsPerReader = 100
	
	var wg sync.WaitGroup
	errors := make(chan error, numReaders)
	
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for j := 0; j < readsPerReader; j++ {
				session := impl.GetActiveSession()
				if session == nil {
					errors <- assert.AnError
					return
				}
				if session.ID != testSession.ID {
					errors <- assert.AnError
					return
				}
				
				hasSession := impl.HasActiveSession()
				if !hasSession {
					errors <- assert.AnError
					return
				}
			}
		}()
	}
	
	wg.Wait()
	close(errors)
	
	// No errors should occur during concurrent reads
	for err := range errors {
		t.Error("Concurrent read error:", err)
	}
}

// TestTechnical_ConcurrentWriteExclusion verifies write operations are exclusive
func TestTechnical_ConcurrentWriteExclusion(t *testing.T) {
	// This test should FAIL until implementation exists
	impl := &SessionManagerImpl{}
	
	// Multiple concurrent attempts to set session - only one should succeed
	const numWriters = 10
	var wg sync.WaitGroup
	successCount := make(chan bool, numWriters)
	
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			testSession := &database.Session{
				ID:   fmt.Sprintf("session-%d", id),
				Name: fmt.Sprintf("Session %d", id),
			}
			
			err := impl.SetActiveSession(testSession)
			successCount <- (err == nil)
		}(i)
	}
	
	wg.Wait()
	close(successCount)
	
	// Count successful writes - should be exactly 1
	successes := 0
	for success := range successCount {
		if success {
			successes++
		}
	}
	
	assert.Equal(t, 1, successes, "Exactly one concurrent write should succeed")
	
	// Verify a session is active
	session := impl.GetActiveSession()
	assert.NotNil(t, session)
	assert.True(t, impl.HasActiveSession())
}