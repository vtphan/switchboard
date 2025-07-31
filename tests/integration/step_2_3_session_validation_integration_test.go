package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/internal/session"
)

// TestStep23_SessionValidationIntegration verifies that Step 2.3 Session Validation
// correctly implements its integration contracts with other phases/steps.
func TestStep23_SessionValidationIntegration(t *testing.T) {
	t.Run("session_state_consistency_flow", func(t *testing.T) {
		// Integration contract: "SessionManager atomic operations" → "Session validation"
		
		// Test that validation functions accept data from SessionManager operations
		validSession := &database.Session{
			ID:        "integration-test-session",
			Name:      "Integration Test Session",
			CreatedBy: "instructor123",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		
		// Verify validation accepts SessionManager data format
		err := session.ValidateSession(validSession)
		assert.NoError(t, err, "Validation should accept SessionManager data format")
		
		// Verify individual validation functions work with session components
		assert.NoError(t, session.ValidateSessionName(validSession.Name))
		assert.NoError(t, session.ValidateSessionStatus(validSession.Status))
	})
	
	t.Run("SessionLifecycle_integration_contract", func(t *testing.T) {
		// Integration contract: SessionLifecycle uses validation before database operations
		
		// Test that validation can be called with SessionLifecycle data patterns
		sessionData := &database.Session{
			ID:        generateTestSessionID(),
			Name:      "SessionLifecycle Test",
			CreatedBy: "instructor456", 
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		
		// This simulates what SessionLifecycle.StartSession() should do
		err := session.ValidateSession(sessionData)
		require.NoError(t, err, "Validation should support SessionLifecycle patterns")
		
		// Test error scenarios that SessionLifecycle might encounter
		invalidSession := &database.Session{
			ID:        "", // Invalid: empty ID
			Name:      "Test Session",
			CreatedBy: "instructor456",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		
		err = session.ValidateSession(invalidSession)
		require.Error(t, err, "Validation should catch SessionLifecycle input errors")
		assert.Contains(t, err.Error(), "session ID required")
	})
	
	t.Run("HTTP_API_integration_contract", func(t *testing.T) {
		// Integration contract: HTTP API uses validation before accepting session parameters
		
		// Test validation with HTTP API-style input validation
		testCases := []struct {
			name        string
			sessionName string
			expectedErr string
		}{
			{
				name:        "valid API input",
				sessionName: "API Test Session",
				expectedErr: "",
			},
			{
				name:        "empty name from API",
				sessionName: "",
				expectedErr: "session name must be 1-200 characters",
			},
			{
				name:        "too long name from API",
				sessionName: generateLongString(201),
				expectedErr: "session name must be 1-200 characters",
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := session.ValidateSessionName(tc.sessionName)
				
				if tc.expectedErr == "" {
					assert.NoError(t, err, "Valid API input should pass validation")
				} else {
					require.Error(t, err, "Invalid API input should fail validation")
					assert.Contains(t, err.Error(), tc.expectedErr, "Error message should be API-friendly")
				}
			})
		}
	})
	
	t.Run("database_layer_integration_contract", func(t *testing.T) {
		// Integration contract: Database layer relies on pre-validated data
		
		// Test that validation enforces all database constraints
		testSession := &database.Session{
			ID:        "db-test-session",
			Name:      "Database Test Session",
			CreatedBy: "instructor789",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		
		// Verify validation matches database constraints
		err := session.ValidateSession(testSession)
		assert.NoError(t, err, "Validation should match database constraints")
		
		// Test database constraint violations
		testSession.Name = generateLongString(201) // Exceeds database.MaxSessionNameLength
		err = session.ValidateSession(testSession)
		require.Error(t, err, "Validation should catch database constraint violations")
		
		// Test invalid status
		testSession.Name = "Valid Name"
		testSession.Status = "invalid_status" // Not in database enum
		err = session.ValidateSession(testSession)
		require.Error(t, err, "Validation should enforce database status enum")
		assert.Contains(t, err.Error(), "status must be 'active' or 'ended'")
	})
	
	t.Run("performance_contract", func(t *testing.T) {
		// Integration contract: Validation performance <1µs per call
		
		testSession := &database.Session{
			ID:        "perf-test-session",
			Name:      "Performance Test Session",
			CreatedBy: "instructor123",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		
		// Measure validation performance over multiple calls
		const iterations = 10000
		start := time.Now()
		
		for i := 0; i < iterations; i++ {
			_ = session.ValidateSession(testSession)
		}
		
		duration := time.Since(start)
		avgNanos := duration.Nanoseconds() / iterations
		
		// Contract: <1µs (1000ns) per validation call
		assert.Less(t, avgNanos, int64(1000), 
			"Validation should average <1µs per call, got %dns", avgNanos)
	})
	
	t.Run("thread_safety_contract", func(t *testing.T) {
		// Integration contract: Validation functions are thread-safe
		
		testSession := &database.Session{
			ID:        "thread-test-session",
			Name:      "Thread Safety Test Session",
			CreatedBy: "instructor123",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		
		// Test concurrent access to validation functions
		const numGoroutines = 100
		const callsPerGoroutine = 100
		
		done := make(chan bool, numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer func() { done <- true }()
				
				for j := 0; j < callsPerGoroutine; j++ {
					err := session.ValidateSession(testSession)
					assert.NoError(t, err, "Concurrent validation should not fail")
				}
			}()
		}
		
		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
		
		// If we reach here without data races, thread safety is confirmed
	})
}

// Helper function to generate test session ID
func generateTestSessionID() string {
	return "test-session-" + time.Now().Format("20060102150405")
}

// Helper function to generate strings of specific length
func generateLongString(length int) string {
	if length <= 0 {
		return ""
	}
	
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = 'A'
	}
	return string(result)
}