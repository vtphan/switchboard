package message

import (
	"errors"
	"testing"

	"switchboard/internal/database"
	pkgErrors "switchboard/pkg/errors"
)

func TestValidateMessage(t *testing.T) {
	t.Run("valid_broadcast_to_instructors_message", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToInstructors,
			FromUser: "student1",
			Context:  database.ContextQuestion,
			Content: map[string]interface{}{
				"text": "I need help with this problem",
			},
		}

		err := validateMessage(msg)
		if err != nil {
			t.Errorf("Expected valid message to pass validation, got error: %v", err)
		}
	})

	t.Run("valid_broadcast_to_students_message", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToStudents,
			FromUser: "instructor1",
			Context:  database.ContextAnnouncement,
			Content: map[string]interface{}{
				"text": "Assignment is due tomorrow",
			},
		}

		err := validateMessage(msg)
		if err != nil {
			t.Errorf("Expected valid message to pass validation, got error: %v", err)
		}
	})

	t.Run("valid_direct_message", func(t *testing.T) {
		toUser := "instructor1"
		msg := &database.Message{
			Type:     database.MessageTypeDirectMessage,
			FromUser: "student1",
			ToUser:   &toUser,
			Context:  database.ContextQuestion,
			Content: map[string]interface{}{
				"text": "Can I ask you a question?",
			},
		}

		err := validateMessage(msg)
		if err != nil {
			t.Errorf("Expected valid direct message to pass validation, got error: %v", err)
		}
	})

	t.Run("nil_message", func(t *testing.T) {
		err := validateMessage(nil)
		if err != pkgErrors.ErrInvalidMessageData {
			t.Errorf("Expected ErrInvalidMessageData for nil message, got: %v", err)
		}
	})

	t.Run("missing_type", func(t *testing.T) {
		msg := &database.Message{
			FromUser: "student1",
			Context:  database.ContextQuestion,
		}

		err := validateMessage(msg)
		if err == nil {
			t.Error("Expected error for missing message type")
		}
		if !containsError(err, pkgErrors.ErrInvalidMessageData) {
			t.Errorf("Expected ErrInvalidMessageData for missing type, got: %v", err)
		}
	})

	t.Run("empty_type", func(t *testing.T) {
		msg := &database.Message{
			Type:     "   ",
			FromUser: "student1",
			Context:  database.ContextQuestion,
		}

		err := validateMessage(msg)
		if err == nil {
			t.Error("Expected error for empty message type")
		}
		if !containsError(err, pkgErrors.ErrInvalidMessageData) {
			t.Errorf("Expected ErrInvalidMessageData for empty type, got: %v", err)
		}
	})

	t.Run("missing_from_user_allowed", func(t *testing.T) {
		// FromUser is set by the processor, not validated during JSON parsing
		msg := &database.Message{
			Type:    database.MessageTypeBroadcastToInstructors,
			Context: database.ContextQuestion,
		}

		err := validateMessage(msg)
		if err != nil {
			t.Errorf("Expected no error for missing from_user (set by processor), got: %v", err)
		}
	})

	t.Run("empty_from_user_allowed", func(t *testing.T) {
		// FromUser is set by the processor, not validated during JSON parsing
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToInstructors,
			FromUser: "   ",
			Context:  database.ContextQuestion,
		}

		err := validateMessage(msg)
		if err != nil {
			t.Errorf("Expected no error for empty from_user (set by processor), got: %v", err)
		}
	})

	t.Run("invalid_message_type", func(t *testing.T) {
		msg := &database.Message{
			Type:     "invalid_type",
			FromUser: "student1",
			Context:  database.ContextQuestion,
		}

		err := validateMessage(msg)
		if err == nil {
			t.Error("Expected error for invalid message type")
		}
		if !containsError(err, pkgErrors.ErrInvalidMessageType) {
			t.Errorf("Expected ErrInvalidMessageType for invalid type, got: %v", err)
		}
	})

	t.Run("invalid_context", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToInstructors,
			FromUser: "student1",
			Context:  "invalid_context",
		}

		err := validateMessage(msg)
		if err == nil {
			t.Error("Expected error for invalid context")
		}
		if !containsError(err, pkgErrors.ErrInvalidMessageData) {
			t.Errorf("Expected ErrInvalidMessageData for invalid context, got: %v", err)
		}
	})

	t.Run("empty_context_allowed", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToInstructors,
			FromUser: "student1",
			Context:  "", // Empty context should be allowed (will be set to default)
		}

		err := validateMessage(msg)
		if err != nil {
			t.Errorf("Expected empty context to be allowed, got error: %v", err)
		}
	})

	t.Run("direct_message_missing_to_user", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeDirectMessage,
			FromUser: "student1",
			Context:  database.ContextQuestion,
		}

		err := validateMessage(msg)
		if err == nil {
			t.Error("Expected error for direct message without to_user")
		}
		if !containsError(err, pkgErrors.ErrInvalidMessageData) {
			t.Errorf("Expected ErrInvalidMessageData for missing to_user, got: %v", err)
		}
	})

	t.Run("direct_message_empty_to_user", func(t *testing.T) {
		emptyToUser := "   "
		msg := &database.Message{
			Type:     database.MessageTypeDirectMessage,
			FromUser: "student1",
			ToUser:   &emptyToUser,
			Context:  database.ContextQuestion,
		}

		err := validateMessage(msg)
		if err == nil {
			t.Error("Expected error for direct message with empty to_user")
		}
		if !containsError(err, pkgErrors.ErrInvalidMessageData) {
			t.Errorf("Expected ErrInvalidMessageData for empty to_user, got: %v", err)
		}
	})

	t.Run("broadcast_message_has_to_user", func(t *testing.T) {
		toUser := "someone"
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToInstructors,
			FromUser: "student1",
			ToUser:   &toUser,
			Context:  database.ContextQuestion,
		}

		err := validateMessage(msg)
		if err == nil {
			t.Error("Expected error for broadcast message with to_user")
		}
		if !containsError(err, pkgErrors.ErrInvalidMessageData) {
			t.Errorf("Expected ErrInvalidMessageData for broadcast with to_user, got: %v", err)
		}
	})

	t.Run("broadcast_to_students_has_to_user", func(t *testing.T) {
		toUser := "someone"
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToStudents,
			FromUser: "instructor1",
			ToUser:   &toUser,
			Context:  database.ContextAnnouncement,
		}

		err := validateMessage(msg)
		if err == nil {
			t.Error("Expected error for broadcast to students with to_user")
		}
		if !containsError(err, pkgErrors.ErrInvalidMessageData) {
			t.Errorf("Expected ErrInvalidMessageData for broadcast with to_user, got: %v", err)
		}
	})

	t.Run("message_with_nil_content", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToInstructors,
			FromUser: "student1",
			Context:  database.ContextQuestion,
			Content:  nil,
		}

		err := validateMessage(msg)
		if err != nil {
			t.Errorf("Expected message with nil content to be valid, got error: %v", err)
		}
	})

	t.Run("message_with_complex_content", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToInstructors,
			FromUser: "student1",
			Context:  database.ContextSubmission,
			Content: map[string]interface{}{
				"text": "My submission",
				"metadata": map[string]interface{}{
					"attachments": []string{"file1.pdf", "file2.jpg"},
					"priority":    "high",
				},
			},
		}

		err := validateMessage(msg)
		if err != nil {
			t.Errorf("Expected message with complex content to be valid, got error: %v", err)
		}
	})
}

func TestIsValidMessageType(t *testing.T) {
	validTypes := []string{
		database.MessageTypeBroadcastToInstructors,
		database.MessageTypeDirectMessage,
		database.MessageTypeBroadcastToStudents,
	}

	for _, msgType := range validTypes {
		t.Run("valid_type_"+msgType, func(t *testing.T) {
			if !isValidMessageType(msgType) {
				t.Errorf("Expected %s to be valid message type", msgType)
			}
		})
	}

	invalidTypes := []string{
		"",
		"invalid",
		"system", // system messages not allowed from clients
		"broadcast",
		"direct",
	}

	for _, msgType := range invalidTypes {
		t.Run("invalid_type_"+msgType, func(t *testing.T) {
			if isValidMessageType(msgType) {
				t.Errorf("Expected %s to be invalid message type", msgType)
			}
		})
	}
}

func TestIsValidContext(t *testing.T) {
	validContexts := []string{
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

	for _, context := range validContexts {
		t.Run("valid_context_"+context, func(t *testing.T) {
			if !isValidContext(context) {
				t.Errorf("Expected %s to be valid context", context)
			}
		})
	}

	invalidContexts := []string{
		"",
		"invalid",
		"chat",
		"message",
	}

	for _, context := range invalidContexts {
		t.Run("invalid_context_"+context, func(t *testing.T) {
			if isValidContext(context) {
				t.Errorf("Expected %s to be invalid context", context)
			}
		})
	}
}

// Helper function to check if an error contains a specific error type
func containsError(err error, target error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, target)
}
