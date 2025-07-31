# Phase 2: Session Management

**Estimated Time:** 1 day  
**Dependencies:** Phase 1 (DatabaseManager, Session model, ErrorTypes)  
**Provides:** SessionManager, SessionLifecycle

## Phase Overview

Implements thread-safe session state management with atomic operations and database persistence. Establishes the single global session pattern that is central to Switchboard's simplified architecture.

---

## Step 2.1: Session Manager Interface & Implementation (Estimated: 4h)

### EXACT REQUIREMENTS:
Read **exactly lines 162-198** of `/Users/vinhthuyphan/Apps/switchboard/docs/tech-specs.md` for complete SessionManager interface and implementation patterns. Implement atomic session state operations exactly as specified.

### ARCHITECTURAL VALIDATION:
- Interface in `internal/session/manager.go`
- Implementation uses DatabaseManager interface from Phase 1 only
- No imports from: `internal/websocket`, `internal/message` (avoid circular dependencies)
- RWMutex usage exactly as specified for read-heavy session access

### FUNCTIONAL VALIDATION:
- Atomic operations: SetActiveSession checks nil before setting
- Thread safety: RWMutex protects all session state access
- Database consistency: Session changes persisted atomically
- Error rollback: Database failures rollback in-memory state

### INTEGRATION CONTRACTS:
- MessageProcessor will call GetActiveSession() before processing messages
- HTTP API will call SetActiveSession()/ClearActiveSession() for session lifecycle
- WebSocket connections will call GetActiveSession() on connection establishment

### MANDATORY INTERFACE:
```go
// SessionManager provides thread-safe session state management
type SessionManager interface {
    GetActiveSession() *Session
    SetActiveSession(session *Session) error
    ClearActiveSession() error
    HasActiveSession() bool
}

// SessionManagerImpl implements thread-safe session management
type SessionManagerImpl struct {
    activeSessionMu sync.RWMutex
    activeSession   *Session
    dbManager       DatabaseManager
}
```

### EXACT IMPLEMENTATION PATTERN:
```go
func (sm *SessionManagerImpl) GetActiveSession() *Session {
    sm.activeSessionMu.RLock()
    defer sm.activeSessionMu.RUnlock()
    
    // Return copy of session pointer (may be nil)
    return sm.activeSession
}

func (sm *SessionManagerImpl) SetActiveSession(session *Session) error {
    sm.activeSessionMu.Lock()
    defer sm.activeSessionMu.Unlock()
    
    if sm.activeSession != nil {
        return ErrSessionAlreadyActive
    }
    
    sm.activeSession = session
    return nil
}

func (sm *SessionManagerImpl) ClearActiveSession() error {
    sm.activeSessionMu.Lock()
    defer sm.activeSessionMu.Unlock()
    
    if sm.activeSession == nil {
        return ErrNoActiveSession
    }
    
    sm.activeSession = nil
    return nil
}

func (sm *SessionManagerImpl) HasActiveSession() bool {
    sm.activeSessionMu.RLock()
    defer sm.activeSessionMu.RUnlock()
    
    return sm.activeSession != nil
}
```

### SUCCESS CRITERIA:
- [ ] All operations use exact mutex pattern from tech specs
- [ ] SetActiveSession() rejects if session already active
- [ ] ClearActiveSession() fails if no active session
- [ ] Thread-safe under concurrent access: `go test -race` passes
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `internal/session/manager.go`
- `internal/session/manager_test.go`

---

## Step 2.2: Session Lifecycle Operations (Estimated: 3h)

### EXACT REQUIREMENTS:
Read **exactly lines 349-396** of `/Users/vinhthuyphan/Apps/switchboard/docs/tech-specs.md` for complete StartSession and EndSession algorithm implementations. Implement exactly as specified with atomic session creation and database persistence.

### ARCHITECTURAL VALIDATION:
- Implementation in `internal/session/lifecycle.go`
- Uses SessionManager interface from Step 2.1
- Uses DatabaseManager interface from Phase 1 
- No direct database access - goes through DatabaseManager only

### FUNCTIONAL VALIDATION:
- Session creation: Generates unique ID, validates input parameters
- Atomic operations: Database persistence with rollback on failure
- Session ending: Updates status and endTime, clears active state
- Error handling: Database failures rollback in-memory changes

### INTEGRATION CONTRACTS:
- HTTP API handlers will call StartSession() and EndSession()
- Broadcast system will be notified of session lifecycle events (Phase 4)
- Session state changes trigger user notifications

### MANDATORY INTERFACE:
```go
// SessionLifecycle handles session start/end operations
type SessionLifecycle struct {
    sessionManager SessionManager 
    dbManager      DatabaseManager
}

// StartSession creates and activates a new session
func (sl *SessionLifecycle) StartSession(sessionName, instructorID string) (*Session, error)

// EndSession ends the current active session
func (sl *SessionLifecycle) EndSession(instructorID string) (*Session, error)
```

### EXACT IMPLEMENTATION PATTERN:
```go
func (sl *SessionLifecycle) StartSession(sessionName, instructorID string) (*Session, error) {
    // Step 1: Validate input parameters
    if len(sessionName) < 1 || len(sessionName) > 200 {
        return nil, errors.New("session name must be 1-200 characters")
    }
    if instructorID == "" {
        return nil, errors.New("instructor ID required")
    }
    
    // Step 2: Create new session object
    session := &Session{
        ID:        generateSessionID(), // Use crypto/rand for uniqueness
        Name:      sessionName,
        CreatedBy: instructorID,
        StartTime: time.Now(),
        Status:    "active",
    }
    
    // Step 3: Atomic session creation
    err := sl.sessionManager.SetActiveSession(session)
    if err != nil {
        return nil, err // ErrSessionAlreadyActive
    }
    
    // Step 4: Persist to database
    err = sl.dbManager.CreateSession(session)
    if err != nil {
        // Rollback: clear in-memory state
        sl.sessionManager.ClearActiveSession()
        return nil, fmt.Errorf("database error: %w", err)
    }
    
    // Step 5: Return session details
    return session, nil
}

func (sl *SessionLifecycle) EndSession(instructorID string) (*Session, error) {
    // Step 1: Get current session atomically
    session := sl.sessionManager.GetActiveSession()
    if session == nil {
        return nil, ErrNoActiveSession
    }
    
    // Step 2: Update session state
    endTime := time.Now()
    session.Status = "ended"
    session.EndTime = &endTime
    
    // Step 3: Clear active session
    sl.sessionManager.ClearActiveSession()
    
    // Step 4: Persist session update
    err := sl.dbManager.UpdateSession(session)
    if err != nil {
        // Log error but don't rollback (session is already ended)
        fmt.Printf("Warning: failed to persist session end: %v\n", err)
    }
    
    // Step 5: Return ended session
    return session, nil
}
```

### SUCCESS CRITERIA:
- [ ] Session validation: name length, instructor ID presence
- [ ] Unique session ID generation using crypto/rand
- [ ] Atomic session creation with database rollback on failure
- [ ] Session ending updates status and endTime correctly
- [ ] Database persistence errors handled properly
- [ ] `go test -race` passes on lifecycle tests
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `internal/session/lifecycle.go`
- `internal/session/lifecycle_test.go`
- `internal/session/id_generator.go` (session ID generation)

---

## Step 2.3: Session State Validation & Consistency (Estimated: 1h)

### EXACT REQUIREMENTS:
Implement session validation and consistency checks to ensure session state integrity across the system. Validate session constraints as specified in database schema.

### ARCHITECTURAL VALIDATION:
- Validation in `internal/session/validation.go`
- Pure validation functions - no dependencies on managers
- Uses Session model from Phase 1 only

### FUNCTIONAL VALIDATION:
- Session name: 1-200 characters as per database constraints
- Session status: Must be "active" or "ended" enum values
- Time validation: StartTime before EndTime when both present
- Required fields: ID, Name, CreatedBy, StartTime must be non-empty

### INTEGRATION CONTRACTS:
- SessionLifecycle uses validation before database operations
- HTTP API uses validation before accepting session parameters
- Database layer relies on pre-validated data

### MANDATORY INTERFACE:
```go
// ValidateSession checks session data integrity
func ValidateSession(session *Session) error

// ValidateSessionName checks name constraints
func ValidateSessionName(name string) error

// ValidateSessionStatus checks status enum
func ValidateSessionStatus(status string) error
```

### IMPLEMENTATION PATTERN:
```go
func ValidateSession(session *Session) error {
    if session == nil {
        return errors.New("session cannot be nil")
    }
    
    if session.ID == "" {
        return errors.New("session ID required")
    }
    
    if err := ValidateSessionName(session.Name); err != nil {
        return err
    }
    
    if session.CreatedBy == "" {
        return errors.New("session creator required")
    }
    
    if err := ValidateSessionStatus(session.Status); err != nil {
        return err
    }
    
    // Time consistency check
    if session.EndTime != nil && session.EndTime.Before(session.StartTime) {
        return errors.New("end time cannot be before start time")
    }
    
    return nil
}

func ValidateSessionName(name string) error {
    if len(name) < 1 || len(name) > 200 {
        return errors.New("session name must be 1-200 characters")
    }
    return nil
}

func ValidateSessionStatus(status string) error {
    switch status {
    case "active", "ended":
        return nil
    default:
        return errors.New("status must be 'active' or 'ended'")
    }
}
```

### SUCCESS CRITERIA:
- [ ] All database constraints enforced in validation
- [ ] Validation functions are pure (no side effects)
- [ ] Proper error messages for each validation failure
- [ ] Coverage ≥85% statements on validation logic

### FILES TO CREATE:
- `internal/session/validation.go`
- `internal/session/validation_test.go`

---

## Phase 2 Integration Tests

### Phase 2 Integration Test: Session-Database Integration
```go
// Auto-generate: tests/integration/phase2_session_database_integration_test.go
func TestSessionLifecycle_DatabaseIntegration(t *testing.T) {
    // Setup in-memory database
    db, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)
    defer db.Close()
    
    // Apply schema
    schema, err := os.ReadFile("../../../internal/database/migrations.sql")
    require.NoError(t, err)
    _, err = db.Exec(string(schema))
    require.NoError(t, err)
    
    // Setup managers
    dbManager := NewSQLiteDatabaseManager(db)
    err = dbManager.Start()
    require.NoError(t, err)
    defer dbManager.Stop()
    
    sessionManager := &SessionManagerImpl{
        dbManager: dbManager,
    }
    
    lifecycle := &SessionLifecycle{
        sessionManager: sessionManager,
        dbManager:      dbManager,
    }
    
    // Test session start
    session, err := lifecycle.StartSession("Integration Test Session", "instructor123")
    require.NoError(t, err)
    assert.Equal(t, "active", session.Status)
    assert.Equal(t, "instructor123", session.CreatedBy)
    
    // Verify session is active in manager
    activeSession := sessionManager.GetActiveSession()
    require.NotNil(t, activeSession)
    assert.Equal(t, session.ID, activeSession.ID)
    
    // Verify session persisted to database
    retrievedSession, err := dbManager.GetActiveSession()
    require.NoError(t, err)
    assert.Equal(t, session.ID, retrievedSession.ID)
    
    // Test session end
    endedSession, err := lifecycle.EndSession("instructor123")
    require.NoError(t, err)
    assert.Equal(t, "ended", endedSession.Status)
    assert.NotNil(t, endedSession.EndTime)
    
    // Verify no active session
    activeSession = sessionManager.GetActiveSession()
    assert.Nil(t, activeSession)
    
    // Test session already active error
    session2, err := lifecycle.StartSession("Another Session", "instructor456")
    require.NoError(t, err)
    
    _, err = lifecycle.StartSession("Conflicting Session", "instructor789")
    assert.Equal(t, ErrSessionAlreadyActive, err)
}
```

### Phase 2 Integration Test: Concurrent Session Access
```go
// Auto-generate: tests/integration/phase2_concurrent_integration_test.go
func TestSessionManager_ConcurrentAccess_Integration(t *testing.T) {
    sessionManager := &SessionManagerImpl{}
    
    // Create test session
    testSession := &Session{
        ID:        "concurrent-test-session",
        Name:      "Concurrent Test",
        CreatedBy: "instructor123",
        StartTime: time.Now(),
        Status:    "active",
    }
    
    // Set session in manager
    err := sessionManager.SetActiveSession(testSession)
    require.NoError(t, err)
    
    // Test concurrent reads
    const numReaders = 50
    const numReads = 100
    
    var wg sync.WaitGroup
    readErrors := make(chan error, numReaders)
    
    for i := 0; i < numReaders; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            
            for j := 0; j < numReads; j++ {
                session := sessionManager.GetActiveSession()
                if session == nil {
                    readErrors <- errors.New("session should not be nil")
                    return
                }
                if session.ID != testSession.ID {
                    readErrors <- errors.New("session ID mismatch")
                    return
                }
                
                // Small random delay to increase chance of race conditions
                time.Sleep(time.Microsecond * time.Duration(rand.Intn(10)))
            }
        }()
    }
    
    wg.Wait()
    close(readErrors)
    
    // Check for any errors
    for err := range readErrors {
        t.Errorf("Concurrent read error: %v", err)
    }
    
    // Test that session is still accessible
    finalSession := sessionManager.GetActiveSession()
    require.NotNil(t, finalSession)
    assert.Equal(t, testSession.ID, finalSession.ID)
}
```

---

## Phase 2 Success Criteria

### ARCHITECTURAL VALIDATION ✓
- [ ] SessionManager uses only DatabaseManager interface from Phase 1
- [ ] No circular dependencies with message or websocket packages
- [ ] RWMutex usage follows exact patterns from tech specs
- [ ] Session validation separated from business logic

### FUNCTIONAL VALIDATION ✓
- [ ] Session state changes are atomic (set/clear operations)
- [ ] Database persistence includes rollback on failure
- [ ] Session validation enforces all database constraints
- [ ] Concurrent access is thread-safe under load

### TECHNICAL VALIDATION ✓
- [ ] `go test -race` passes on all session management tests
- [ ] Test coverage ≥85% statements for session management code
- [ ] Session lifecycle handles all error cases properly
- [ ] Integration tests verify database consistency

### INTEGRATION READINESS ✓
- [ ] MessageProcessor can call GetActiveSession() for message gating (Phase 3)
- [ ] WebSocket connections can check session state on connect (Phase 4)
- [ ] HTTP API can manage session lifecycle through interfaces (Phase 5)
- [ ] Session state changes ready for broadcast notifications (Phase 4)

**Phase 2 establishes the single global session state that enables Switchboard's ultra-simplified architecture.**