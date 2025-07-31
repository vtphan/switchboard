package session

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
)

// RED-GREEN-REFACTOR Cycle 1: ValidateSession Function Tests

func TestArchitectural_ValidateSessionFunction(t *testing.T) {
	// Verify function signature exists
	// This will fail initially (RED phase)
	
	// Test that ValidateSession function exists with correct signature
	session := &database.Session{
		ID:        "test-id",
		Name:      "Test Session",
		CreatedBy: "instructor123",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}
	
	// This should compile and be callable
	err := ValidateSession(session)
	_ = err // Function should exist, error content tested separately
}

func TestFunctional_ValidateSession_NilHandling(t *testing.T) {
	// RED: This test will fail until ValidateSession is implemented
	err := ValidateSession(nil)
	
	require.Error(t, err, "ValidateSession should reject nil session")
	assert.Contains(t, err.Error(), "session cannot be nil")
}

func TestFunctional_ValidateSession_RequiredFields(t *testing.T) {
	tests := []struct {
		name          string
		modifySession func(*database.Session)
		expectedError string
	}{
		{
			name: "missing ID",
			modifySession: func(s *database.Session) {
				s.ID = ""
			},
			expectedError: "session ID required",
		},
		{
			name: "missing Name",
			modifySession: func(s *database.Session) {
				s.Name = ""
			},
			expectedError: "session name must be 1-200 characters",
		},
		{
			name: "missing CreatedBy",
			modifySession: func(s *database.Session) {
				s.CreatedBy = ""
			},
			expectedError: "session creator required",
		},
		{
			name: "zero StartTime",
			modifySession: func(s *database.Session) {
				s.StartTime = time.Time{}
			},
			expectedError: "session start time required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create valid session
			session := &database.Session{
				ID:        "test-id",
				Name:      "Test Session",
				CreatedBy: "instructor123",
				StartTime: time.Now(),
				Status:    database.SessionStatusActive,
			}
			
			// Apply modification to make it invalid
			tt.modifySession(session)
			
			// RED: This will fail until ValidateSession is implemented
			err := ValidateSession(session)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestFunctional_ValidateSession_TimeConsistency(t *testing.T) {
	// Test time validation logic
	startTime := time.Now()
	endTime := startTime.Add(-1 * time.Hour) // End before start (invalid)
	
	session := &database.Session{
		ID:        "test-id",
		Name:      "Test Session",
		CreatedBy: "instructor123",
		StartTime: startTime,
		EndTime:   &endTime,
		Status:    database.SessionStatusEnded,
	}
	
	// RED: This will fail until ValidateSession is implemented
	err := ValidateSession(session)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "end time cannot be before start time")
}

func TestFunctional_ValidateSession_ValidSession(t *testing.T) {
	// Test that a completely valid session passes validation
	session := &database.Session{
		ID:        "test-id",
		Name:      "Test Session",
		CreatedBy: "instructor123",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}
	
	// RED: This will fail until ValidateSession is implemented
	err := ValidateSession(session)
	assert.NoError(t, err, "Valid session should pass validation")
}

// RED-GREEN-REFACTOR Cycle 2: ValidateSessionName Function Tests

func TestArchitectural_ValidateSessionNameFunction(t *testing.T) {
	// Verify function signature exists
	err := ValidateSessionName("Test Name")
	_ = err // Function should exist, error content tested separately
}

func TestFunctional_ValidateSessionName_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		sessionName   string
		shouldError   bool
		expectedError string
	}{
		{
			name:          "empty string",
			sessionName:   "",
			shouldError:   true,
			expectedError: "session name must be 1-200 characters",
		},
		{
			name:          "whitespace only",
			sessionName:   "   ",
			shouldError:   false, // Whitespace is valid content
		},
		{
			name:        "single character",
			sessionName: "A",
			shouldError: false,
		},
		{
			name:        "exactly 200 characters",
			sessionName: strings.Repeat("A", 200),
			shouldError: false,
		},
		{
			name:          "201 characters",
			sessionName:   strings.Repeat("A", 201),
			shouldError:   true,
			expectedError: "session name must be 1-200 characters",
		},
		{
			name:        "unicode characters",
			sessionName: "测试会话 🎓",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// RED: This will fail until ValidateSessionName is implemented
			err := ValidateSessionName(tt.sessionName)
			
			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// RED-GREEN-REFACTOR Cycle 3: ValidateSessionStatus Function Tests

func TestArchitectural_ValidateSessionStatusFunction(t *testing.T) {
	// Verify function signature exists
	err := ValidateSessionStatus(database.SessionStatusActive)
	_ = err // Function should exist, error content tested separately
}

func TestFunctional_ValidateSessionStatus_EnumValidation(t *testing.T) {
	tests := []struct {
		name          string
		status        string
		shouldError   bool
		expectedError string
	}{
		{
			name:        "valid active status",
			status:      database.SessionStatusActive,
			shouldError: false,
		},
		{
			name:        "valid ended status", 
			status:      database.SessionStatusEnded,
			shouldError: false,
		},
		{
			name:          "invalid status",
			status:        "invalid",
			shouldError:   true,
			expectedError: "status must be 'active' or 'ended'",
		},
		{
			name:          "empty status",
			status:        "",
			shouldError:   true,
			expectedError: "status must be 'active' or 'ended'",
		},
		{
			name:          "case sensitivity test",
			status:        "ACTIVE",
			shouldError:   true,
			expectedError: "status must be 'active' or 'ended'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// RED: This will fail until ValidateSessionStatus is implemented
			err := ValidateSessionStatus(tt.status)
			
			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Integration Tests

func TestIntegration_ValidationWithSessionLifecycle(t *testing.T) {
	// This test verifies validation functions work with SessionLifecycle patterns
	
	// Test data that should pass validation
	validSession := &database.Session{
		ID:        "integration-test-id",
		Name:      "Integration Test Session",
		CreatedBy: "instructor123",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}
	
	// RED: These will fail until validation functions are implemented
	assert.NoError(t, ValidateSession(validSession))
	assert.NoError(t, ValidateSessionName(validSession.Name))
	assert.NoError(t, ValidateSessionStatus(validSession.Status))
}

func TestIntegration_ValidationPerformance(t *testing.T) {
	// Performance test: validation should complete in <1µs
	session := &database.Session{
		ID:        "perf-test-id",
		Name:      "Performance Test Session",
		CreatedBy: "instructor123",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}
	
	// Measure validation performance
	start := time.Now()
	for i := 0; i < 1000; i++ {
		_ = ValidateSession(session)
	}
	duration := time.Since(start)
	
	// Average should be well under 1µs (1000ns)
	avgNs := duration.Nanoseconds() / 1000
	assert.Less(t, avgNs, int64(1000), "Validation should average <1µs per call")
}

// Contract Verification Tests

func TestContract_ValidationCalledBeforeDatabaseOps(t *testing.T) {
	// This test ensures validation contracts are respected
	// It verifies that validation functions can handle the same data
	// that SessionLifecycle would send to the database
	
	// Test with session data that would be created by SessionLifecycle
	sessionData := &database.Session{
		ID:        "contract-test-session",
		Name:      "Contract Test Session",
		CreatedBy: "instructor456",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}
	
	// RED: This will fail until ValidateSession is implemented
	err := ValidateSession(sessionData)
	assert.NoError(t, err, "Validation should accept SessionLifecycle data format")
}

func TestContract_ValidationErrorMessagesForAPI(t *testing.T) {
	// Test that validation error messages are suitable for HTTP API responses
	
	tests := []struct {
		name    string
		session *database.Session
		checkFn func(error) bool
	}{
		{
			name: "nil session for API",
			session: nil,
			checkFn: func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "session cannot be nil")
			},
		},
		{
			name: "empty name for API",
			session: &database.Session{
				ID:        "test-id",
				Name:      "",
				CreatedBy: "instructor123",
				StartTime: time.Now(),
				Status:    database.SessionStatusActive,
			},
			checkFn: func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "session name must be 1-200 characters")
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// RED: This will fail until ValidateSession is implemented
			err := ValidateSession(tt.session)
			assert.True(t, tt.checkFn(err), "Error message should be suitable for API responses")
		})
	}
}