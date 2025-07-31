package unit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"switchboard/internal/database"
)

// TestSessionValidation tests Session struct validation constraints
func TestSessionValidation(t *testing.T) {
	testCases := []struct {
		name        string
		sessionFunc func() database.Session
		shouldPass  bool
		expectedErr string
	}{
		{
			name: "Valid session",
			sessionFunc: func() database.Session {
				return database.Session{
					ID:        "valid-id",
					Name:      "Valid Session Name",
					CreatedBy: "instructor-1",
					Status:    database.SessionStatusActive,
				}
			},
			shouldPass: true,
		},
		{
			name: "Empty ID",
			sessionFunc: func() database.Session {
				return database.Session{
					ID:        "",
					Name:      "Test Session",
					CreatedBy: "instructor-1",
					Status:    database.SessionStatusActive,
				}
			},
			shouldPass:  false,
			expectedErr: "session ID is required",
		},
		{
			name: "Empty name",
			sessionFunc: func() database.Session {
				return database.Session{
					ID:        "test-id",
					Name:      "",
					CreatedBy: "instructor-1",
					Status:    database.SessionStatusActive,
				}
			},
			shouldPass:  false,
			expectedErr: "session name is required",
		},
		{
			name: "Name too long (>200 chars)",
			sessionFunc: func() database.Session {
				return database.Session{
					ID:        "test-id",
					Name:      strings.Repeat("a", 201),
					CreatedBy: "instructor-1",
					Status:    database.SessionStatusActive,
				}
			},
			shouldPass:  false,
			expectedErr: "session name too long",
		},
		{
			name: "Invalid status enum",
			sessionFunc: func() database.Session {
				return database.Session{
					ID:        "test-id",
					Name:      "Test Session",
					CreatedBy: "instructor-1",
					Status:    "invalid-status",
				}
			},
			shouldPass:  false,
			expectedErr: "invalid session status",
		},
		{
			name: "Empty CreatedBy",
			sessionFunc: func() database.Session {
				return database.Session{
					ID:        "test-id",
					Name:      "Test Session",
					CreatedBy: "",
					Status:    database.SessionStatusActive,
				}
			},
			shouldPass:  false,
			expectedErr: "session created_by is required",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			session := tc.sessionFunc()
			err := session.Validate()
			
			if tc.shouldPass {
				assert.NoError(t, err, "Validation should pass for %s", tc.name)
			} else {
				assert.Error(t, err, "Validation should fail for %s", tc.name)
				if tc.expectedErr != "" {
					assert.Contains(t, err.Error(), tc.expectedErr, "Error message should contain expected text")
				}
			}
		})
	}
}

// TestMessageValidation tests Message struct validation constraints
func TestMessageValidation(t *testing.T) {
	testCases := []struct {
		name        string
		messageFunc func() database.Message
		shouldPass  bool
		expectedErr string
	}{
		{
			name: "Valid message",
			messageFunc: func() database.Message {
				return database.Message{
					ID:        "valid-id",
					SessionID: "session-123",
					Type:      database.MessageTypeBroadcastToStudents,
					Context:   database.ContextGeneral,
					FromUser:  "user-123",
					Content:   map[string]interface{}{"text": "Hello"},
				}
			},
			shouldPass: true,
		},
		{
			name: "Empty ID",
			messageFunc: func() database.Message {
				return database.Message{
					ID:        "",
					SessionID: "session-123",
					Type:      database.MessageTypeBroadcastToStudents,
					Context:   database.ContextGeneral,
					FromUser:  "user-123",
					Content:   map[string]interface{}{"text": "Hello"},
				}
			},
			shouldPass:  false,
			expectedErr: "message ID is required",
		},
		{
			name: "Content too large (>64KB)",
			messageFunc: func() database.Message {
				// Create content that's larger than 64KB when JSON serialized
				largeContent := strings.Repeat("a", 65*1024)
				return database.Message{
					ID:        "msg-123",
					SessionID: "session-123",
					Type:      database.MessageTypeBroadcastToStudents,
					Context:   database.ContextGeneral,
					FromUser:  "user-123",
					Content:   map[string]interface{}{"text": largeContent},
				}
			},
			shouldPass:  false,
			expectedErr: "message content too large",
		},
		{
			name: "Invalid message type",
			messageFunc: func() database.Message {
				return database.Message{
					ID:        "msg-123",
					SessionID: "session-123",
					Type:      "invalid-type",
					Context:   database.ContextGeneral,
					FromUser:  "user-123",
					Content:   map[string]interface{}{"text": "Hello"},
				}
			},
			shouldPass:  false,
			expectedErr: "invalid message type",
		},
		{
			name: "Invalid context type",
			messageFunc: func() database.Message {
				return database.Message{
					ID:        "msg-123",
					SessionID: "session-123",
					Type:      database.MessageTypeBroadcastToStudents,
					Context:   "invalid-context",
					FromUser:  "user-123",
					Content:   map[string]interface{}{"text": "Hello"},
				}
			},
			shouldPass:  false,
			expectedErr: "invalid message context",
		},
		{
			name: "Missing required fields - SessionID",
			messageFunc: func() database.Message {
				return database.Message{
					ID:       "msg-123",
					Type:     database.MessageTypeBroadcastToStudents,
					Context:  database.ContextGeneral,
					FromUser: "user-123",
					Content:  map[string]interface{}{"text": "Hello"},
				}
			},
			shouldPass:  false,
			expectedErr: "message session_id is required",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			message := tc.messageFunc()
			err := message.Validate()
			
			if tc.shouldPass {
				assert.NoError(t, err, "Validation should pass for %s", tc.name)
			} else {
				assert.Error(t, err, "Validation should fail for %s", tc.name)
				if tc.expectedErr != "" {
					assert.Contains(t, err.Error(), tc.expectedErr, "Error message should contain expected text")
				}
			}
		})
	}
}

// TestEnumValidation tests that all enum constants are valid
func TestEnumValidation(t *testing.T) {
	t.Run("Message type constants", func(t *testing.T) {
		// Test that all message type constants are valid for validation
		messageTypes := []string{
			database.MessageTypeBroadcastToInstructors,
			database.MessageTypeDirectMessage,
			database.MessageTypeBroadcastToStudents,
			database.MessageTypeSystem,
		}
		
		for _, msgType := range messageTypes {
			t.Run(msgType, func(t *testing.T) {
				message := database.Message{
					ID:        "test-id",
					SessionID: "session-123",
					Type:      msgType,
					Context:   database.ContextGeneral,
					FromUser:  "user-123",
					Content:   map[string]interface{}{"text": "test"},
				}
				
				err := message.Validate()
				assert.NoError(t, err, "Message type %s should be valid", msgType)
			})
		}
	})
	
	t.Run("Context constants", func(t *testing.T) {
		// Test that all context constants are valid for validation
		contexts := []string{
			database.ContextQuestion,
			database.ContextSubmission,
			database.ContextAnalytics,
			database.ContextResponse,
			database.ContextRequest,
			database.ContextPeerHelp,
			database.ContextAnnouncement,
			database.ContextInstruction,
			database.ContextEmergency,
			database.ContextGeneral,
		}
		
		for _, context := range contexts {
			t.Run(context, func(t *testing.T) {
				message := database.Message{
					ID:        "test-id",
					SessionID: "session-123",
					Type:      database.MessageTypeBroadcastToStudents,
					Context:   context,
					FromUser:  "user-123",
					Content:   map[string]interface{}{"text": "test"},
				}
				
				err := message.Validate()
				assert.NoError(t, err, "Context %s should be valid", context)
			})
		}
	})
	
	t.Run("Invalid enum values rejected", func(t *testing.T) {
		// Test that validation rejects invalid enum values
		t.Run("Invalid message type", func(t *testing.T) {
			message := database.Message{
				ID:        "test-id",
				SessionID: "session-123",
				Type:      "invalid-type",
				Context:   database.ContextGeneral,
				FromUser:  "user-123",
				Content:   map[string]interface{}{"text": "test"},
			}
			
			err := message.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid message type")
		})
		
		t.Run("Invalid context", func(t *testing.T) {
			message := database.Message{
				ID:        "test-id", 
				SessionID: "session-123",
				Type:      database.MessageTypeBroadcastToStudents,
				Context:   "invalid-context",
				FromUser:  "user-123",
				Content:   map[string]interface{}{"text": "test"},
			}
			
			err := message.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid message context")
		})
		
		t.Run("Invalid session status", func(t *testing.T) {
			session := database.Session{
				ID:        "test-id",
				Name:      "Test Session",
				CreatedBy: "instructor-1",
				Status:    "invalid-status",
			}
			
			err := session.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid session status")
		})
	})
}

// TestSessionNameValidation tests specific session name validation rules
func TestSessionNameValidation(t *testing.T) {
	t.Run("Name length boundaries", func(t *testing.T) {
		testCases := []struct {
			name       string
			nameLength int
			shouldPass bool
		}{
			{"Empty name", 0, false},
			{"Single character", 1, true},
			{"Exactly 200 chars", 200, true},
			{"Over 200 chars", 201, false},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				sessionName := strings.Repeat("a", tc.nameLength)
				session := database.Session{
					ID:        "test-id",
					Name:      sessionName,
					CreatedBy: "instructor-1",
					Status:    database.SessionStatusActive,
				}
				
				err := session.Validate()
				if tc.shouldPass {
					assert.NoError(t, err, "Session name with length %d should be valid", tc.nameLength)
				} else {
					assert.Error(t, err, "Session name with length %d should be invalid", tc.nameLength)
				}
			})
		}
	})
}

// TestMessageSizeValidation tests message content size limits
func TestMessageSizeValidation(t *testing.T) {
	t.Run("Content size boundaries", func(t *testing.T) {
		// Test content at various sizes around the 64KB limit
		testCases := []struct {
			name        string
			sizeKB      int
			shouldPass  bool
		}{
			{"Empty content", 0, true},
			{"Small content (1KB)", 1, true},
			{"Large content (63KB)", 63, true},
			{"At limit (64KB)", 64, true},
			{"Over limit (65KB)", 65, false},
			{"Way over limit (100KB)", 100, false},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create content of approximately the target size
				var content map[string]interface{}
				if tc.sizeKB == 0 {
					content = map[string]interface{}{}
				} else {
					// Create string of approximately sizeKB * 1024 characters
					// Account for JSON overhead
					textSize := tc.sizeKB * 1024 - 50 // Leave room for JSON structure
					if textSize < 1 {
						textSize = 1
					}
					content = map[string]interface{}{
						"text": strings.Repeat("a", textSize),
					}
				}
				
				message := database.Message{
					ID:        "test-id",
					SessionID: "session-123",
					Type:      database.MessageTypeBroadcastToStudents,
					Context:   database.ContextGeneral,
					FromUser:  "user-123",
					Content:   content,
				}
				
				err := message.Validate()
				if tc.shouldPass {
					assert.NoError(t, err, "Message content of %dKB should be valid", tc.sizeKB)
				} else {
					assert.Error(t, err, "Message content of %dKB should be invalid", tc.sizeKB)
					assert.Contains(t, err.Error(), "message content too large")
				}
			})
		}
	})
}