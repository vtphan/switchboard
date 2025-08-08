package message

import "switchboard/internal/database"

// RoleBasedFilter implements educational privacy rules for message filtering
// This filter ensures students don't see other students' messages for privacy,
// and instructors only see messages they should have access to
type RoleBasedFilter struct{}

// NewRoleBasedFilter creates a new RoleBasedFilter instance
func NewRoleBasedFilter() *RoleBasedFilter {
	return &RoleBasedFilter{}
}

// ShouldReceiveMessage determines if a recipient should receive a message based on their role
// Implements educational privacy rules as specified in tech specs:
// - Students see only their own messages and broadcasts to students
// - Instructors see broadcast_to_instructors messages and direct messages they participate in
func (f *RoleBasedFilter) ShouldReceiveMessage(msg *database.Message, recipient Recipient) bool {
	if msg == nil || recipient == nil {
		return false
	}

	recipientRole := recipient.GetRole()
	recipientUserID := recipient.GetUserID()

	switch msg.Type {
	case database.MessageTypeBroadcastToInstructors:
		// Only instructors should receive broadcast_to_instructors messages
		return recipientRole == "instructor"

	case database.MessageTypeDirectMessage:
		// Direct messages should only be seen by:
		// 1. The sender (always)
		// 2. The recipient (always)

		if msg.ToUser == nil {
			return false // Invalid direct message
		}

		targetUserID := *msg.ToUser

		// Sender can always see their own message
		if recipientUserID == msg.FromUser {
			return true
		}

		// Target user can always see messages sent to them
		if recipientUserID == targetUserID {
			return true
		}

		// Other users cannot see this direct message
		return false

	case database.MessageTypeBroadcastToStudents:
		// Only students should receive broadcast_to_students messages
		return recipientRole == "student"

	case "system":
		// System messages (like session_started, session_ended) should be delivered to all users
		return true

	default:
		// Unknown message types are not delivered
		return false
	}
}

// FilterRecipients filters a list of recipients based on who should receive the message
// This is a convenience method that applies ShouldReceiveMessage to a list of recipients
func (f *RoleBasedFilter) FilterRecipients(msg *database.Message, recipients []Recipient) []Recipient {
	if msg == nil || recipients == nil {
		return []Recipient{}
	}

	var filteredRecipients []Recipient

	for _, recipient := range recipients {
		if f.ShouldReceiveMessage(msg, recipient) {
			filteredRecipients = append(filteredRecipients, recipient)
		}
	}

	return filteredRecipients
}
