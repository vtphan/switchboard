package message

import (
	"testing"

	"switchboard/internal/database"
)

func TestNewRoleBasedFilter(t *testing.T) {
	filter := NewRoleBasedFilter()
	if filter == nil {
		t.Error("Expected NewRoleBasedFilter to return non-nil filter")
	}
}

func TestRoleBasedFilter_ShouldReceiveMessage(t *testing.T) {
	filter := NewRoleBasedFilter()

	// Create test recipients
	instructor1 := NewRecipient("instructor1", "instructor")
	instructor2 := NewRecipient("instructor2", "instructor")
	student1 := NewRecipient("student1", "student")
	student2 := NewRecipient("student2", "student")
	admin := NewRecipient("admin1", "admin")

	t.Run("broadcast_to_instructors_message", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToInstructors,
			FromUser: "student1",
			Context:  database.ContextQuestion,
		}

		// Instructors should receive the message
		if !filter.ShouldReceiveMessage(msg, instructor1) {
			t.Error("Expected instructor1 to receive broadcast_to_instructors message")
		}

		if !filter.ShouldReceiveMessage(msg, instructor2) {
			t.Error("Expected instructor2 to receive broadcast_to_instructors message")
		}

		// Students should NOT receive the message
		if filter.ShouldReceiveMessage(msg, student1) {
			t.Error("Expected student1 to NOT receive broadcast_to_instructors message")
		}

		if filter.ShouldReceiveMessage(msg, student2) {
			t.Error("Expected student2 to NOT receive broadcast_to_instructors message")
		}

		// Other roles should NOT receive the message
		if filter.ShouldReceiveMessage(msg, admin) {
			t.Error("Expected admin to NOT receive broadcast_to_instructors message")
		}
	})

	t.Run("broadcast_to_students_message", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToStudents,
			FromUser: "instructor1",
			Context:  database.ContextAnnouncement,
		}

		// Students should receive the message
		if !filter.ShouldReceiveMessage(msg, student1) {
			t.Error("Expected student1 to receive broadcast_to_students message")
		}

		if !filter.ShouldReceiveMessage(msg, student2) {
			t.Error("Expected student2 to receive broadcast_to_students message")
		}

		// Instructors should NOT receive broadcast_to_students messages (updated privacy rules)
		if filter.ShouldReceiveMessage(msg, instructor1) {
			t.Error("Expected instructor1 to NOT receive broadcast_to_students message")
		}

		if filter.ShouldReceiveMessage(msg, instructor2) {
			t.Error("Expected instructor2 to NOT receive broadcast_to_students message")
		}

		// Other roles should NOT receive the message
		if filter.ShouldReceiveMessage(msg, admin) {
			t.Error("Expected admin to NOT receive broadcast_to_students message")
		}
	})

	t.Run("direct_message_sender_can_see", func(t *testing.T) {
		toUser := "instructor1"
		msg := &database.Message{
			Type:     database.MessageTypeDirectMessage,
			FromUser: "student1",
			ToUser:   &toUser,
			Context:  database.ContextQuestion,
		}

		// Sender should always see their own message
		if !filter.ShouldReceiveMessage(msg, student1) {
			t.Error("Expected sender (student1) to see their own direct message")
		}
	})

	t.Run("direct_message_recipient_can_see", func(t *testing.T) {
		toUser := "instructor1"
		msg := &database.Message{
			Type:     database.MessageTypeDirectMessage,
			FromUser: "student1",
			ToUser:   &toUser,
			Context:  database.ContextQuestion,
		}

		// Target user should always see messages sent to them
		if !filter.ShouldReceiveMessage(msg, instructor1) {
			t.Error("Expected recipient (instructor1) to see direct message sent to them")
		}
	})

	t.Run("direct_message_instructors_can_monitor", func(t *testing.T) {
		toUser := "student2"
		msg := &database.Message{
			Type:     database.MessageTypeDirectMessage,
			FromUser: "student1",
			ToUser:   &toUser,
			Context:  database.ContextPeerHelp,
		}

		// Instructors should NOT see direct messages between students (updated privacy rules)
		if filter.ShouldReceiveMessage(msg, instructor1) {
			t.Error("Expected instructor1 to NOT see direct message between students")
		}

		if filter.ShouldReceiveMessage(msg, instructor2) {
			t.Error("Expected instructor2 to NOT see direct message between students")
		}
	})

	t.Run("direct_message_other_students_cannot_see", func(t *testing.T) {
		toUser := "instructor1"
		msg := &database.Message{
			Type:     database.MessageTypeDirectMessage,
			FromUser: "student1",
			ToUser:   &toUser,
			Context:  database.ContextQuestion,
		}

		// Other students should NOT see the direct message (privacy rule)
		if filter.ShouldReceiveMessage(msg, student2) {
			t.Error("Expected student2 to NOT see direct message between student1 and instructor1")
		}
	})

	t.Run("direct_message_student_to_student_privacy", func(t *testing.T) {
		toUser := "student2"
		msg := &database.Message{
			Type:     database.MessageTypeDirectMessage,
			FromUser: "student1",
			ToUser:   &toUser,
			Context:  database.ContextPeerHelp,
		}

		// Sender can see their own message
		if !filter.ShouldReceiveMessage(msg, student1) {
			t.Error("Expected sender (student1) to see their own message")
		}

		// Recipient can see the message
		if !filter.ShouldReceiveMessage(msg, student2) {
			t.Error("Expected recipient (student2) to see message sent to them")
		}

		// Instructors cannot monitor student-to-student messages (updated privacy rules)
		if filter.ShouldReceiveMessage(msg, instructor1) {
			t.Error("Expected instructor1 to NOT see student-to-student message")
		}

		// Other students cannot see (privacy)
		otherStudent := NewRecipient("student3", "student")
		if filter.ShouldReceiveMessage(msg, otherStudent) {
			t.Error("Expected student3 to NOT see direct message between student1 and student2")
		}
	})

	t.Run("direct_message_missing_to_user", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeDirectMessage,
			FromUser: "student1",
			ToUser:   nil, // Missing ToUser
			Context:  database.ContextQuestion,
		}

		// No one should receive invalid direct message
		if filter.ShouldReceiveMessage(msg, student1) {
			t.Error("Expected no one to receive direct message with missing to_user")
		}

		if filter.ShouldReceiveMessage(msg, instructor1) {
			t.Error("Expected no one to receive direct message with missing to_user")
		}
	})

	t.Run("nil_message", func(t *testing.T) {
		// No one should receive nil message
		if filter.ShouldReceiveMessage(nil, student1) {
			t.Error("Expected no one to receive nil message")
		}

		if filter.ShouldReceiveMessage(nil, instructor1) {
			t.Error("Expected no one to receive nil message")
		}
	})

	t.Run("nil_recipient", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToStudents,
			FromUser: "instructor1",
			Context:  database.ContextAnnouncement,
		}

		// Nil recipient should not receive any message
		if filter.ShouldReceiveMessage(msg, nil) {
			t.Error("Expected nil recipient to not receive any message")
		}
	})

	t.Run("unknown_message_type", func(t *testing.T) {
		msg := &database.Message{
			Type:     "unknown_type",
			FromUser: "student1",
			Context:  database.ContextQuestion,
		}

		// No one should receive unknown message types
		if filter.ShouldReceiveMessage(msg, student1) {
			t.Error("Expected no one to receive unknown message type")
		}

		if filter.ShouldReceiveMessage(msg, instructor1) {
			t.Error("Expected no one to receive unknown message type")
		}
	})
}

func TestRoleBasedFilter_FilterRecipients(t *testing.T) {
	filter := NewRoleBasedFilter()

	// Create test recipients
	instructor1 := NewRecipient("instructor1", "instructor")
	instructor2 := NewRecipient("instructor2", "instructor")
	student1 := NewRecipient("student1", "student")
	student2 := NewRecipient("student2", "student")
	admin := NewRecipient("admin1", "admin")

	allRecipients := []Recipient{instructor1, instructor2, student1, student2, admin}

	t.Run("filter_broadcast_to_instructors", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToInstructors,
			FromUser: "student1",
			Context:  database.ContextQuestion,
		}

		filtered := filter.FilterRecipients(msg, allRecipients)

		// Should only include instructors
		expectedCount := 2
		if len(filtered) != expectedCount {
			t.Errorf("Expected %d filtered recipients, got %d", expectedCount, len(filtered))
		}

		// Verify all filtered recipients are instructors
		for _, recipient := range filtered {
			if recipient.GetRole() != "instructor" {
				t.Errorf("Expected all filtered recipients to be instructors, got role: %s", recipient.GetRole())
			}
		}
	})

	t.Run("filter_broadcast_to_students", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToStudents,
			FromUser: "instructor1",
			Context:  database.ContextAnnouncement,
		}

		filtered := filter.FilterRecipients(msg, allRecipients)

		// Should include only students (updated privacy rules)
		expectedCount := 2
		if len(filtered) != expectedCount {
			t.Errorf("Expected %d filtered recipients, got %d", expectedCount, len(filtered))
		}

		// Verify filtered recipients are students only
		for _, recipient := range filtered {
			role := recipient.GetRole()
			if role != "student" {
				t.Errorf("Expected filtered recipients to be students only, got role: %s", role)
			}
		}
	})

	t.Run("filter_direct_message", func(t *testing.T) {
		toUser := "instructor1"
		msg := &database.Message{
			Type:     database.MessageTypeDirectMessage,
			FromUser: "student1",
			ToUser:   &toUser,
			Context:  database.ContextQuestion,
		}

		filtered := filter.FilterRecipients(msg, allRecipients)

		// Should include only sender and recipient (updated privacy rules)
		expectedCount := 2 // student1 (sender), instructor1 (recipient)
		if len(filtered) != expectedCount {
			t.Errorf("Expected %d filtered recipients, got %d", expectedCount, len(filtered))
		}

		// Verify specific recipients are included
		recipientIDs := make(map[string]bool)
		for _, recipient := range filtered {
			recipientIDs[recipient.GetUserID()] = true
		}

		expectedRecipients := []string{"student1", "instructor1"}
		for _, expectedID := range expectedRecipients {
			if !recipientIDs[expectedID] {
				t.Errorf("Expected %s to be in filtered recipients", expectedID)
			}
		}

		// Verify student2, instructor2, and admin are NOT included
		if recipientIDs["student2"] {
			t.Error("Expected student2 to NOT be in filtered recipients")
		}

		if recipientIDs["instructor2"] {
			t.Error("Expected instructor2 to NOT be in filtered recipients")
		}

		if recipientIDs["admin1"] {
			t.Error("Expected admin1 to NOT be in filtered recipients")
		}
	})

	t.Run("filter_nil_message", func(t *testing.T) {
		filtered := filter.FilterRecipients(nil, allRecipients)

		// Should return empty list
		if len(filtered) != 0 {
			t.Errorf("Expected 0 filtered recipients for nil message, got %d", len(filtered))
		}
	})

	t.Run("filter_nil_recipients", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToStudents,
			FromUser: "instructor1",
			Context:  database.ContextAnnouncement,
		}

		filtered := filter.FilterRecipients(msg, nil)

		// Should return empty list
		if len(filtered) != 0 {
			t.Errorf("Expected 0 filtered recipients for nil recipients list, got %d", len(filtered))
		}
	})

	t.Run("filter_empty_recipients", func(t *testing.T) {
		msg := &database.Message{
			Type:     database.MessageTypeBroadcastToStudents,
			FromUser: "instructor1",
			Context:  database.ContextAnnouncement,
		}

		filtered := filter.FilterRecipients(msg, []Recipient{})

		// Should return empty list
		if len(filtered) != 0 {
			t.Errorf("Expected 0 filtered recipients for empty recipients list, got %d", len(filtered))
		}
	})
}
