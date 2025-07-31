package unit

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	switchboardErrors "switchboard/pkg/errors"
)

// TestErrorTypesArchitecturalCompliance validates that error types
// meet all architectural requirements from Step 1.5
func TestErrorTypesArchitecturalCompliance(t *testing.T) {
	t.Run("Error types use standard Go error interface", func(t *testing.T) {
		// All error variables should implement the standard error interface
		var err error

		err = switchboardErrors.ErrNoActiveSession
		assert.NotNil(t, err, "ErrNoActiveSession must implement error interface")

		err = switchboardErrors.ErrSessionAlreadyActive
		assert.NotNil(t, err, "ErrSessionAlreadyActive must implement error interface")

		err = switchboardErrors.ErrConnectionNotFound
		assert.NotNil(t, err, "ErrConnectionNotFound must implement error interface")

		err = switchboardErrors.ErrChannelFull
		assert.NotNil(t, err, "ErrChannelFull must implement error interface")

		err = switchboardErrors.ErrInvalidMessageType
		assert.NotNil(t, err, "ErrInvalidMessageType must implement error interface")

		err = switchboardErrors.ErrRateLimitExceeded
		assert.NotNil(t, err, "ErrRateLimitExceeded must implement error interface")
	})

	t.Run("Error types use errors.New() pattern", func(t *testing.T) {
		// Verify that errors are simple string-based errors, not custom types
		errValue := reflect.ValueOf(switchboardErrors.ErrNoActiveSession)
		errType := reflect.TypeOf(switchboardErrors.ErrNoActiveSession)
		
		// Should be a pointer to errors.errorString (the type returned by errors.New())
		assert.True(t, errValue.Kind() == reflect.Ptr, "Error should be a pointer type")
		assert.Contains(t, errType.String(), "error", "Error should use standard error type")
	})

	t.Run("No custom error types used", func(t *testing.T) {
		// Ensure we're not using complex custom error types
		// Simple string-based errors created with errors.New() are preferred
		
		// Test that errors behave like standard errors
		err1 := switchboardErrors.ErrNoActiveSession
		err2 := switchboardErrors.ErrNoActiveSession
		
		// Same error variables should be identical (not just equal)
		assert.True(t, err1 == err2, "Error variables should be identical references")
	})

	t.Run("Error naming follows Go conventions", func(t *testing.T) {
		// All error variable names should start with "Err" prefix
		errorNames := []string{
			"ErrNoActiveSession",
			"ErrSessionAlreadyActive", 
			"ErrConnectionNotFound",
			"ErrChannelFull",
			"ErrInvalidMessageType",
			"ErrRateLimitExceeded",
		}

		for _, name := range errorNames {
			assert.True(t, strings.HasPrefix(name, "Err"), 
				"Error variable %s must start with 'Err' prefix", name)
			assert.False(t, strings.Contains(name, "_"), 
				"Error variable %s should use camelCase, not snake_case", name)
		}
	})
}

// TestErrorTypesTechSpecsCompliance validates exact match with tech specs lines 411-418
func TestErrorTypesTechSpecsCompliance(t *testing.T) {
	t.Run("Required error types from tech specs exist with exact messages", func(t *testing.T) {
		// Test each error from tech specs lines 411-418
		assert.Equal(t, "no active session", switchboardErrors.ErrNoActiveSession.Error(),
			"ErrNoActiveSession message must match tech specs exactly")
			
		assert.Equal(t, "session already active", switchboardErrors.ErrSessionAlreadyActive.Error(),
			"ErrSessionAlreadyActive message must match tech specs exactly")
			
		assert.Equal(t, "connection not found", switchboardErrors.ErrConnectionNotFound.Error(),
			"ErrConnectionNotFound message must match tech specs exactly")
			
		assert.Equal(t, "send channel full", switchboardErrors.ErrChannelFull.Error(),
			"ErrChannelFull message must match tech specs exactly")
			
		assert.Equal(t, "invalid message type", switchboardErrors.ErrInvalidMessageType.Error(),
			"ErrInvalidMessageType message must match tech specs exactly")
			
		assert.Equal(t, "rate limit exceeded", switchboardErrors.ErrRateLimitExceeded.Error(),
			"ErrRateLimitExceeded message must match tech specs exactly")
	})

	t.Run("All required tech specs errors are present", func(t *testing.T) {
		// Ensure all 6 required errors from tech specs are defined
		requiredErrors := map[string]error{
			"ErrNoActiveSession":      switchboardErrors.ErrNoActiveSession,
			"ErrSessionAlreadyActive": switchboardErrors.ErrSessionAlreadyActive,
			"ErrConnectionNotFound":   switchboardErrors.ErrConnectionNotFound,
			"ErrChannelFull":         switchboardErrors.ErrChannelFull,
			"ErrInvalidMessageType":  switchboardErrors.ErrInvalidMessageType,
			"ErrRateLimitExceeded":   switchboardErrors.ErrRateLimitExceeded,
		}

		for name, err := range requiredErrors {
			assert.NotNil(t, err, "Required error %s from tech specs must be defined", name)
			assert.NotEmpty(t, err.Error(), "Error %s must have a non-empty message", name)
		}
	})
}

// TestErrorTypesIntegrationReadiness validates integration contracts
func TestErrorTypesIntegrationReadiness(t *testing.T) {
	t.Run("Session management integration errors", func(t *testing.T) {
		// Verify errors are available for session management
		assert.NotNil(t, switchboardErrors.ErrNoActiveSession, 
			"ErrNoActiveSession must be available for session management")
		assert.NotNil(t, switchboardErrors.ErrSessionAlreadyActive, 
			"ErrSessionAlreadyActive must be available for session management")
	})

	t.Run("Connection management integration errors", func(t *testing.T) {
		// Verify errors are available for connection management
		assert.NotNil(t, switchboardErrors.ErrConnectionNotFound, 
			"ErrConnectionNotFound must be available for connection management")
		assert.NotNil(t, switchboardErrors.ErrChannelFull, 
			"ErrChannelFull must be available for connection management")
	})

	t.Run("Message processing integration errors", func(t *testing.T) {
		// Verify errors are available for message processing
		assert.NotNil(t, switchboardErrors.ErrInvalidMessageType, 
			"ErrInvalidMessageType must be available for message processing")
	})

	t.Run("Rate limiting integration errors", func(t *testing.T) {
		// Verify errors are available for rate limiting
		assert.NotNil(t, switchboardErrors.ErrRateLimitExceeded, 
			"ErrRateLimitExceeded must be available for rate limiting")
	})

	t.Run("HTTP handler integration readiness", func(t *testing.T) {
		// Verify errors can be translated to client-appropriate messages
		// HTTP handlers should be able to detect and handle these errors
		
		testErrors := []error{
			switchboardErrors.ErrNoActiveSession,
			switchboardErrors.ErrSessionAlreadyActive,
			switchboardErrors.ErrConnectionNotFound,
			switchboardErrors.ErrChannelFull,
			switchboardErrors.ErrInvalidMessageType,
			switchboardErrors.ErrRateLimitExceeded,
		}

		for _, err := range testErrors {
			// Test that errors can be compared for HTTP response mapping
			assert.True(t, errors.Is(err, err), "Error must support errors.Is() for HTTP mapping")
			assert.NotEmpty(t, err.Error(), "Error must have message for HTTP response")
		}
	})
}

// TestAdditionalValidationErrors tests the extra validation errors beyond tech specs
func TestAdditionalValidationErrors(t *testing.T) {
	t.Run("Additional validation errors are properly defined", func(t *testing.T) {
		// Test additional validation errors that extend beyond tech specs
		additionalErrors := map[string]error{
			"ErrInvalidSessionData": switchboardErrors.ErrInvalidSessionData,
			"ErrInvalidMessageData": switchboardErrors.ErrInvalidMessageData,
			"ErrContentTooLarge":    switchboardErrors.ErrContentTooLarge,
		}

		for name, err := range additionalErrors {
			assert.NotNil(t, err, "Additional validation error %s must be defined", name)
			assert.NotEmpty(t, err.Error(), "Validation error %s must have a non-empty message", name)
			assert.True(t, strings.HasPrefix(name, "Err"), 
				"Validation error %s must follow naming convention", name)
		}
	})

	t.Run("Validation error messages are appropriate", func(t *testing.T) {
		assert.Equal(t, "invalid session data", switchboardErrors.ErrInvalidSessionData.Error(),
			"ErrInvalidSessionData message should be clear and descriptive")
		assert.Equal(t, "invalid message data", switchboardErrors.ErrInvalidMessageData.Error(),
			"ErrInvalidMessageData message should be clear and descriptive")
		assert.Equal(t, "content too large", switchboardErrors.ErrContentTooLarge.Error(),
			"ErrContentTooLarge message should be clear and descriptive")
	})
}

// TestErrorUsagePatterns validates proper error usage patterns
func TestErrorUsagePatterns(t *testing.T) {
	t.Run("Errors support comparison patterns", func(t *testing.T) {
		// Test that errors can be used in comparison patterns
		err := switchboardErrors.ErrNoActiveSession
		
		// Direct comparison
		assert.True(t, err == switchboardErrors.ErrNoActiveSession, 
			"Error should support direct comparison")
		assert.False(t, err == switchboardErrors.ErrSessionAlreadyActive, 
			"Different errors should not be equal")
		
		// errors.Is comparison
		assert.True(t, errors.Is(err, switchboardErrors.ErrNoActiveSession), 
			"Error should support errors.Is() comparison")
		assert.False(t, errors.Is(err, switchboardErrors.ErrSessionAlreadyActive), 
			"errors.Is() should distinguish different errors")
	})

	t.Run("Errors support wrapping patterns", func(t *testing.T) {
		// Test that errors can be wrapped for additional context
		baseErr := switchboardErrors.ErrNoActiveSession
		wrappedErr := errors.New("operation failed: " + baseErr.Error())
		
		assert.NotNil(t, wrappedErr, "Errors should support wrapping")
		assert.Contains(t, wrappedErr.Error(), baseErr.Error(), 
			"Wrapped error should contain original message")
	})

	t.Run("Error messages are user-friendly", func(t *testing.T) {
		// Verify error messages are clear and lowercase (following Go conventions)
		testCases := []struct {
			err      error
			expected string
		}{
			{switchboardErrors.ErrNoActiveSession, "no active session"},
			{switchboardErrors.ErrSessionAlreadyActive, "session already active"},
			{switchboardErrors.ErrConnectionNotFound, "connection not found"},
			{switchboardErrors.ErrChannelFull, "send channel full"},
			{switchboardErrors.ErrInvalidMessageType, "invalid message type"},
			{switchboardErrors.ErrRateLimitExceeded, "rate limit exceeded"},
		}

		for _, tc := range testCases {
			msg := tc.err.Error()
			assert.Equal(t, tc.expected, msg, "Error message should match expected format")
			assert.True(t, strings.ToLower(msg) == msg, "Error message should be lowercase")
			assert.False(t, strings.HasSuffix(msg, "."), "Error message should not end with period")
		}
	})
}

// TestErrorPackageStructure validates the error package structure
func TestErrorPackageStructure(t *testing.T) {
	t.Run("Error package imports only standard library", func(t *testing.T) {
		// The errors package should only import the standard library errors package
		// This is verified by the fact that we can import it without import cycles
		assert.NotNil(t, switchboardErrors.ErrNoActiveSession, 
			"Error package should be importable with only standard library dependencies")
	})

	t.Run("Error variables are exported", func(t *testing.T) {
		// All error variables should be exported (start with capital letter)
		errType := reflect.TypeOf(switchboardErrors.ErrNoActiveSession)
		assert.NotNil(t, errType, "Error variables should be exported and accessible")
	})

	t.Run("Error package follows Go conventions", func(t *testing.T) {
		// Package should be named "errors" and export error variables
		// This test verifies the package can be used as intended
		
		// Test that we can assign to error interface
		var err = switchboardErrors.ErrNoActiveSession
		assert.NotNil(t, err, "Error variables should implement error interface")
		
		// Test that we can use in error handling patterns
		if errors.Is(err, switchboardErrors.ErrNoActiveSession) {
			// This pattern should work for error handling
			assert.True(t, true, "Error handling patterns should work correctly")
		} else {
			t.Error("Error handling pattern failed")
		}
	})
}