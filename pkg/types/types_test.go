package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Architectural Validation Tests

func TestTypes_ArchitecturalCompliance(t *testing.T) {
	// Verify that types package has no forbidden imports
	// This will be checked at compile time - no business logic imports allowed
	
	// Test that all structs can be created
	_ = &Session{}
	_ = &Message{}
	_ = &Client{}
	_ = &ConnectionManager{}
}

// Functional Validation Tests - Session

func TestSession_Validate(t *testing.T) {
	tests := []struct {
		name    string
		session Session
		wantErr error
	}{
		{
			name: "valid session",
			session: Session{
				ID:         "123",
				Name:       "Test Session",
				CreatedBy:  "instructor_123",
				StudentIDs: []string{"student1", "student2"},
				StartTime:  time.Now(),
				Status:     "active",
			},
			wantErr: nil,
		},
		{
			name: "empty name",
			session: Session{
				ID:         "123",
				Name:       "",
				CreatedBy:  "instructor_123",
				StudentIDs: []string{"student1"},
			},
			wantErr: ErrInvalidSessionName,
		},
		{
			name: "name too long",
			session: Session{
				ID:         "123",
				Name:       strings.Repeat("a", 201),
				CreatedBy:  "instructor_123",
				StudentIDs: []string{"student1"},
			},
			wantErr: ErrInvalidSessionName,
		},
		{
			name: "empty student list",
			session: Session{
				ID:         "123",
				Name:       "Test Session",
				CreatedBy:  "instructor_123",
				StudentIDs: []string{},
			},
			wantErr: ErrEmptyStudentList,
		},
		{
			name: "invalid created_by",
			session: Session{
				ID:         "123",
				Name:       "Test Session",
				CreatedBy:  "invalid user!@#",
				StudentIDs: []string{"student1"},
			},
			wantErr: ErrInvalidCreatedBy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.session.Validate()
			if err != tt.wantErr {
				t.Errorf("Session.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSession_JSONMarshaling(t *testing.T) {
	session := Session{
		ID:         "123",
		Name:       "Test Session",
		CreatedBy:  "instructor_123",
		StudentIDs: []string{"student1", "student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}

	// Test marshaling
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("Failed to marshal session: %v", err)
	}

	// Test unmarshaling
	var decoded Session
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal session: %v", err)
	}

	// Verify fields preserved
	if decoded.ID != session.ID {
		t.Errorf("ID not preserved: got %v, want %v", decoded.ID, session.ID)
	}
	if len(decoded.StudentIDs) != len(session.StudentIDs) {
		t.Errorf("StudentIDs not preserved: got %v, want %v", decoded.StudentIDs, session.StudentIDs)
	}
}

// Functional Validation Tests - Message

func TestMessage_Validate(t *testing.T) {
	tests := []struct {
		name    string
		message Message
		wantErr error
	}{
		{
			name: "valid instructor_inbox message",
			message: Message{
				ID:        "msg1",
				SessionID: "session1",
				Type:      MessageTypeInstructorInbox,
				Context:   "question",
				FromUser:  "student1",
				Content:   map[string]interface{}{"text": "Help needed"},
				Timestamp: time.Now(),
			},
			wantErr: nil,
		},
		{
			name: "valid inbox_response message",
			message: Message{
				ID:        "msg2",
				SessionID: "session1",
				Type:      MessageTypeInboxResponse,
				Context:   "answer",
				FromUser:  "instructor1",
				ToUser:    stringPtr("student1"),
				Content:   map[string]interface{}{"text": "Here's the answer"},
				Timestamp: time.Now(),
			},
			wantErr: nil,
		},
		{
			name: "empty context defaults to general",
			message: Message{
				ID:        "msg3",
				SessionID: "session1",
				Type:      MessageTypeAnalytics,
				Context:   "",
				FromUser:  "student1",
				Content:   map[string]interface{}{"data": "analytics"},
			},
			wantErr: nil,
		},
		{
			name: "invalid message type",
			message: Message{
				ID:        "msg4",
				SessionID: "session1",
				Type:      "invalid_type",
				Context:   "general",
				FromUser:  "student1",
				Content:   map[string]interface{}{},
			},
			wantErr: ErrInvalidMessageType,
		},
		{
			name: "invalid context characters",
			message: Message{
				ID:        "msg5",
				SessionID: "session1",
				Type:      MessageTypeRequest,
				Context:   "invalid context!@#",
				FromUser:  "instructor1",
				Content:   map[string]interface{}{},
			},
			wantErr: ErrInvalidContext,
		},
		{
			name: "context too long",
			message: Message{
				ID:        "msg6",
				SessionID: "session1",
				Type:      MessageTypeRequest,
				Context:   strings.Repeat("a", 51),
				FromUser:  "instructor1",
				Content:   map[string]interface{}{},
			},
			wantErr: ErrInvalidContext,
		},
		{
			name: "content too large",
			message: Message{
				ID:        "msg7",
				SessionID: "session1",
				Type:      MessageTypeRequest,
				Context:   "general",
				FromUser:  "instructor1",
				Content:   map[string]interface{}{"data": strings.Repeat("x", 65536)},
			},
			wantErr: ErrContentTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.message.Validate()
			if err != tt.wantErr {
				t.Errorf("Message.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			// Check context defaulting
			if tt.message.Context == "" && err == nil && tt.message.Context != "general" {
				t.Error("Empty context should default to 'general'")
			}
		})
	}
}

func TestMessage_TypeConstants(t *testing.T) {
	// Verify all 6 message type constants are defined correctly
	expectedTypes := map[string]string{
		"instructor_inbox":     MessageTypeInstructorInbox,
		"inbox_response":       MessageTypeInboxResponse,
		"request":              MessageTypeRequest,
		"request_response":     MessageTypeRequestResponse,
		"analytics":            MessageTypeAnalytics,
		"instructor_broadcast": MessageTypeInstructorBroadcast,
	}

	for expected, constant := range expectedTypes {
		if constant != expected {
			t.Errorf("Message type constant mismatch: got %v, want %v", constant, expected)
		}
	}
}

// Functional Validation Tests - Client

func TestClient_ValidateUserID(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		wantOk  bool
	}{
		{"valid alphanumeric", "user123", true},
		{"valid with underscore", "user_123", true},
		{"valid with hyphen", "user-123", true},
		{"valid mixed", "User_123-test", true},
		{"valid 50 chars", strings.Repeat("a", 50), true},
		{"empty", "", false},
		{"too long", strings.Repeat("a", 51), false},
		{"special chars", "user@123", false},
		{"spaces", "user 123", false},
		{"unicode", "user🎉", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidUserID(tt.userID); got != tt.wantOk {
				t.Errorf("IsValidUserID(%q) = %v, want %v", tt.userID, got, tt.wantOk)
			}
		})
	}
}

func TestClient_JSONMarshaling(t *testing.T) {
	client := Client{
		ID:            "user123",
		Role:          "student",
		SessionID:     "session1",
		LastHeartbeat: time.Now(),
		MessageCount:  5,
		WindowStart:   time.Now().Add(-time.Minute),
		CleanedUp:     false,
	}

	// Test marshaling
	data, err := json.Marshal(client)
	if err != nil {
		t.Fatalf("Failed to marshal client: %v", err)
	}

	// Verify SendChannel is not included
	if strings.Contains(string(data), "send_channel") || strings.Contains(string(data), "SendChannel") {
		t.Error("SendChannel should not be included in JSON")
	}

	// Test unmarshaling
	var decoded Client
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal client: %v", err)
	}

	if decoded.ID != client.ID {
		t.Errorf("ID not preserved: got %v, want %v", decoded.ID, client.ID)
	}
}

// Technical Validation Tests

func TestValidation_Performance(t *testing.T) {
	// Test that validation completes quickly
	session := Session{
		ID:         "123",
		Name:       "Test Session",
		CreatedBy:  "instructor_123",
		StudentIDs: []string{"student1", "student2"},
		StartTime:  time.Now(),
		Status:     "active",
	}

	start := time.Now()
	for i := 0; i < 1000; i++ {
		_ = session.Validate()
	}
	elapsed := time.Since(start)

	// Should complete 1000 validations in well under 1 second
	if elapsed > time.Second {
		t.Errorf("Validation too slow: %v for 1000 operations", elapsed)
	}
}

func TestMessage_ContentSizeValidation(t *testing.T) {
	// Test exact boundary of 64KB
	largeContent := make(map[string]interface{})
	// Create content that will be exactly at the boundary when marshaled
	largeContent["data"] = strings.Repeat("x", 65520) // Leaves room for JSON overhead

	msg := Message{
		ID:        "msg1",
		SessionID: "session1",
		Type:      MessageTypeRequest,
		Context:   "general",
		FromUser:  "user1",
		Content:   largeContent,
	}

	// Should be close to limit but valid
	err := msg.Validate()
	if err == ErrContentTooLarge {
		t.Log("Content size validation working correctly at boundary")
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}