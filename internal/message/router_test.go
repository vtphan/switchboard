package message

import (
	"testing"

	"switchboard/internal/database"
	"switchboard/pkg/errors"
)

// Mock ConnectionProvider for testing
type mockConnectionProvider struct {
	connectedUsers []Recipient
	shouldError    bool
}

func (m *mockConnectionProvider) GetConnectedUsers() ([]Recipient, error) {
	if m.shouldError {
		return nil, errors.ErrConnectionNotFound
	}
	return m.connectedUsers, nil
}

func TestNewMessageRouter(t *testing.T) {
	provider := &mockConnectionProvider{}
	filter := NewRoleBasedFilter()

	router := NewMessageRouter(provider, filter)
	if router == nil {
		t.Error("Expected NewMessageRouter to return non-nil router")
	}
}

func TestMessageRouter_GetRecipients(t *testing.T) {
	// Setup mock users
	instructor1 := NewRecipient("instructor1", "instructor")
	instructor2 := NewRecipient("instructor2", "instructor")
	student1 := NewRecipient("student1", "student")
	student2 := NewRecipient("student2", "student")

	connectedUsers := []Recipient{instructor1, instructor2, student1, student2}

	provider := &mockConnectionProvider{
		connectedUsers: connectedUsers,
		shouldError:    false,
	}
	filter := NewRoleBasedFilter()
	router := NewMessageRouter(provider, filter)

	t.Run("broadcast_to_instructors_message", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToInstructors,
			FromUser: "student1",
			Context:  database.ContextQuestion,
		}

		recipients, err := router.GetRecipients(msg)
		if err != nil {
			t.Errorf("Expected no error for broadcast_to_instructors, got: %v", err)
		}

		// Should only return instructors
		expectedCount := 2
		if len(recipients) != expectedCount {
			t.Errorf("Expected %d recipients, got %d", expectedCount, len(recipients))
		}

		// Verify all recipients are instructors
		for _, recipient := range recipients {
			if recipient.GetRole() != "instructor" {
				t.Errorf("Expected all recipients to be instructors, got role: %s", recipient.GetRole())
			}
		}

		// Verify specific instructors are included
		instructorIDs := make(map[string]bool)
		for _, recipient := range recipients {
			instructorIDs[recipient.GetUserID()] = true
		}

		if !instructorIDs["instructor1"] || !instructorIDs["instructor2"] {
			t.Error("Expected both instructor1 and instructor2 to be recipients")
		}
	})

	t.Run("broadcast_to_students_message", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToStudents,
			FromUser: "instructor1",
			Context:  database.ContextAnnouncement,
		}

		recipients, err := router.GetRecipients(msg)
		if err != nil {
			t.Errorf("Expected no error for broadcast_to_students, got: %v", err)
		}

		// Should return only students (updated privacy rules)
		expectedCount := 2
		if len(recipients) != expectedCount {
			t.Errorf("Expected %d recipients, got %d", expectedCount, len(recipients))
		}

		// Verify only students are included (no instructors due to privacy rules)
		userIDs := make(map[string]bool)
		for _, recipient := range recipients {
			userIDs[recipient.GetUserID()] = true
		}

		expectedUsers := []string{"student1", "student2"}
		for _, userID := range expectedUsers {
			if !userIDs[userID] {
				t.Errorf("Expected %s to be a recipient", userID)
			}
		}

		// Verify instructors are NOT included
		excludedUsers := []string{"instructor1", "instructor2"}
		for _, userID := range excludedUsers {
			if userIDs[userID] {
				t.Errorf("Expected %s to NOT be a recipient due to privacy rules", userID)
			}
		}
	})

	t.Run("direct_message_target_connected", func(t *testing.T) {
		toUser := "instructor1"
		msg := &database.Message{
			Type:     database.MessageTypeDirectMessage,
			FromUser: "student1",
			ToUser:   &toUser,
			Context:  database.ContextQuestion,
		}

		recipients, err := router.GetRecipients(msg)
		if err != nil {
			t.Errorf("Expected no error for direct message, got: %v", err)
		}

		// Should return both sender and target user
		expectedCount := 2
		if len(recipients) != expectedCount {
			t.Errorf("Expected %d recipients, got %d", expectedCount, len(recipients))
		}

		// Verify both sender and target are included
		userIDs := make(map[string]bool)
		for _, recipient := range recipients {
			userIDs[recipient.GetUserID()] = true
		}

		if !userIDs["student1"] {
			t.Error("Expected sender (student1) to be a recipient")
		}
		if !userIDs["instructor1"] {
			t.Error("Expected target (instructor1) to be a recipient")
		}
	})

	t.Run("direct_message_target_not_connected", func(t *testing.T) {
		toUser := "instructor999" // Not in connected users
		msg := &database.Message{
			Type:     database.MessageTypeDirectMessage,
			FromUser: "student1",
			ToUser:   &toUser,
			Context:  database.ContextQuestion,
		}

		recipients, err := router.GetRecipients(msg)
		if err == nil {
			t.Error("Expected error for direct message to non-existent user")
		}

		// Should return nil recipients when target user doesn't exist
		if recipients != nil {
			t.Errorf("Expected nil recipients for non-existent target, got %v", recipients)
		}

		// Verify specific error message
		expectedError := "target user 'instructor999' not found or not connected"
		if err != nil && !contains(err.Error(), expectedError) {
			t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
		}
	})

	t.Run("direct_message_missing_to_user", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeDirectMessage,
			FromUser: "student1",
			ToUser:   nil, // Missing ToUser
			Context:  database.ContextQuestion,
		}

		recipients, err := router.GetRecipients(msg)
		if err == nil {
			t.Error("Expected error for direct message without to_user")
		}

		if recipients != nil {
			t.Error("Expected nil recipients when error occurs")
		}
	})

	t.Run("nil_message", func(t *testing.T) {
		recipients, err := router.GetRecipients(nil)
		if err == nil {
			t.Error("Expected error for nil message")
		}

		if recipients != nil {
			t.Error("Expected nil recipients when error occurs")
		}
	})

	t.Run("invalid_message_type", func(t *testing.T) {
		msg := &database.Message{
			Type:     "invalid_type",
			FromUser: "student1",
			Context:  database.ContextQuestion,
		}

		recipients, err := router.GetRecipients(msg)
		if err == nil {
			t.Error("Expected error for invalid message type")
		}

		if recipients != nil {
			t.Error("Expected nil recipients when error occurs")
		}
	})

	t.Run("connection_provider_error", func(t *testing.T) {
		errorProvider := &mockConnectionProvider{
			shouldError: true,
		}
		router := NewMessageRouter(errorProvider, filter)

		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToInstructors,
			FromUser: "student1",
			Context:  database.ContextQuestion,
		}

		recipients, err := router.GetRecipients(msg)
		if err == nil {
			t.Error("Expected error when connection provider fails")
		}

		if recipients != nil {
			t.Error("Expected nil recipients when error occurs")
		}
	})
}

func TestMessageRouter_SpecialRoles(t *testing.T) {
	// Test edge cases with different user roles
	admin := NewRecipient("admin1", "admin")
	guest := NewRecipient("guest1", "guest")
	instructor := NewRecipient("instructor1", "instructor")
	student := NewRecipient("student1", "student")

	connectedUsers := []Recipient{admin, guest, instructor, student}

	provider := &mockConnectionProvider{
		connectedUsers: connectedUsers,
		shouldError:    false,
	}
	filter := NewRoleBasedFilter()
	router := NewMessageRouter(provider, filter)

	t.Run("broadcast_to_instructors_with_special_roles", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToInstructors,
			FromUser: "student1",
			Context:  database.ContextQuestion,
		}

		recipients, err := router.GetRecipients(msg)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		// Should only return instructors (admin and guest are not instructors)
		expectedCount := 1
		if len(recipients) != expectedCount {
			t.Errorf("Expected %d recipient, got %d", expectedCount, len(recipients))
		}

		if recipients[0].GetUserID() != "instructor1" {
			t.Errorf("Expected recipient to be instructor1, got: %s", recipients[0].GetUserID())
		}
	})

	t.Run("broadcast_to_students_with_special_roles", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToStudents,
			FromUser: "instructor1",
			Context:  database.ContextAnnouncement,
		}

		recipients, err := router.GetRecipients(msg)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		// Should only return students (not instructors, admin, or guest due to privacy rules)
		expectedCount := 1
		if len(recipients) != expectedCount {
			t.Errorf("Expected %d recipients, got %d", expectedCount, len(recipients))
		}

		// Verify only student is included
		recipientIDs := make(map[string]bool)
		for _, recipient := range recipients {
			recipientIDs[recipient.GetUserID()] = true
		}

		if !recipientIDs["student1"] {
			t.Error("Expected student1 to be a recipient")
		}

		// Verify instructor is NOT included due to privacy rules
		if recipientIDs["instructor1"] {
			t.Error("Expected instructor1 to NOT be a recipient due to privacy rules")
		}

		if recipientIDs["admin1"] || recipientIDs["guest1"] {
			t.Error("Expected admin and guest to NOT be recipients")
		}
	})
}
