package switchboard

import (
	"encoding/json"
	"testing"
)

func TestNewClient(t *testing.T) {
	// Test valid configuration
	config := Config{
		UserID: "test_user",
		Role:   RoleStudent,
	}
	
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if client.config.UserID != "test_user" {
		t.Errorf("Expected UserID 'test_user', got %s", client.config.UserID)
	}
	
	if client.config.Role != RoleStudent {
		t.Errorf("Expected Role 'student', got %s", client.config.Role)
	}
	
	// Test default values
	if client.config.WsURL != "ws://localhost:8080/ws" {
		t.Errorf("Expected default WsURL, got %s", client.config.WsURL)
	}
	
	if client.config.APIURL != "http://localhost:8080/api" {
		t.Errorf("Expected default APIURL, got %s", client.config.APIURL)
	}
	
	if client.config.MaxReconnectAttempts != 10 {
		t.Errorf("Expected default MaxReconnectAttempts 10, got %d", client.config.MaxReconnectAttempts)
	}
}

func TestNewClientValidation(t *testing.T) {
	// Test missing UserID
	config := Config{
		Role: RoleStudent,
	}
	
	_, err := NewClient(config)
	if err == nil {
		t.Fatal("Expected error for missing UserID")
	}
	
	// Test invalid Role
	config = Config{
		UserID: "test_user",
		Role:   "invalid",
	}
	
	_, err = NewClient(config)
	if err == nil {
		t.Fatal("Expected error for invalid Role")
	}
}

func TestMessageContent(t *testing.T) {
	// Test NewMessageContent
	content := NewMessageContent("Hello, World!")
	if content["text"] != "Hello, World!" {
		t.Errorf("Expected text 'Hello, World!', got %v", content["text"])
	}
	
	// Test WithContext
	content = content.WithContext("question")
	if content["context"] != "question" {
		t.Errorf("Expected context 'question', got %v", content["context"])
	}
	
	// Test WithField
	content = content.WithField("urgent", true)
	if content["urgent"] != true {
		t.Errorf("Expected urgent true, got %v", content["urgent"])
	}
}

func TestConnectionStates(t *testing.T) {
	states := []ConnectionState{
		ConnectionStateDisconnected,
		ConnectionStateConnecting,
		ConnectionStateConnected,
		ConnectionStateError,
	}
	
	expectedStrings := []string{
		"disconnected",
		"connecting", 
		"connected",
		"error",
	}
	
	for i, state := range states {
		if string(state) != expectedStrings[i] {
			t.Errorf("Expected state %s, got %s", expectedStrings[i], string(state))
		}
	}
}

func TestMessageParsing(t *testing.T) {
	// Test parsing a valid message
	msgJSON := `{
		"id": "msg-123",
		"type": "broadcast_to_students",
		"context": "announcement",
		"from_user": "teacher",
		"content": {
			"text": "Hello students",
			"important": true
		},
		"timestamp": "2025-01-15T10:30:00Z"
	}`
	
	msg, err := parseMessage([]byte(msgJSON))
	if err != nil {
		t.Fatalf("Expected no error parsing message, got %v", err)
	}
	
	if msg.ID != "msg-123" {
		t.Errorf("Expected ID 'msg-123', got %s", msg.ID)
	}
	
	if msg.Type != "broadcast_to_students" {
		t.Errorf("Expected Type 'broadcast_to_students', got %s", msg.Type)
	}
	
	if msg.Context != "announcement" {
		t.Errorf("Expected Context 'announcement', got %s", msg.Context)
	}
	
	if msg.FromUser != "teacher" {
		t.Errorf("Expected FromUser 'teacher', got %s", msg.FromUser)
	}
	
	if text, ok := msg.Content["text"].(string); !ok || text != "Hello students" {
		t.Errorf("Expected content text 'Hello students', got %v", msg.Content["text"])
	}
	
	if important, ok := msg.Content["important"].(bool); !ok || !important {
		t.Errorf("Expected content important true, got %v", msg.Content["important"])
	}
}

func TestOutgoingMessageSerialization(t *testing.T) {
	// Test creating and serializing an outgoing message
	content := map[string]interface{}{
		"text":    "Test message",
		"urgent":  true,
		"context": "question", // This should be removed during send
	}
	
	msg := OutgoingMessage{
		Type:    "broadcast_to_instructors",
		Context: "question",
		Content: content,
	}
	
	// Simulate the context extraction that happens in send()
	if ctx, ok := msg.Content["context"].(string); ok {
		msg.Context = ctx
		delete(msg.Content, "context")
	}
	
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Expected no error serializing message, got %v", err)
	}
	
	// Parse it back to verify structure
	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Expected no error parsing serialized message, got %v", err)
	}
	
	// Verify context is at top level
	if parsed["context"] != "question" {
		t.Errorf("Expected context 'question' at top level, got %v", parsed["context"])
	}
	
	// Verify content doesn't have context
	content = parsed["content"].(map[string]interface{})
	if _, hasContext := content["context"]; hasContext {
		t.Error("Expected content to not have context field (double-nesting)")
	}
	
	// Verify other content fields are preserved
	if content["text"] != "Test message" {
		t.Errorf("Expected content text 'Test message', got %v", content["text"])
	}
	
	if content["urgent"] != true {
		t.Errorf("Expected content urgent true, got %v", content["urgent"])
	}
}

func TestRoleValidation(t *testing.T) {
	validRoles := []Role{RoleStudent, RoleInstructor}
	validRoleStrings := []string{"student", "instructor"}
	
	for i, role := range validRoles {
		if string(role) != validRoleStrings[i] {
			t.Errorf("Expected role %s, got %s", validRoleStrings[i], string(role))
		}
	}
}

func TestSessionStruct(t *testing.T) {
	session := Session{
		Active:    true,
		ID:        "session-123",
		Name:      "Test Session",
		StartedBy: "teacher",
	}
	
	if !session.Active {
		t.Error("Expected session to be active")
	}
	
	if session.ID != "session-123" {
		t.Errorf("Expected ID 'session-123', got %s", session.ID)
	}
	
	if session.Name != "Test Session" {
		t.Errorf("Expected Name 'Test Session', got %s", session.Name)
	}
	
	if session.StartedBy != "teacher" {
		t.Errorf("Expected StartedBy 'teacher', got %s", session.StartedBy)
	}
}

func TestClientUtilityMethods(t *testing.T) {
	client, err := NewClient(Config{
		UserID: "test_user",
		Role:   RoleStudent,
	})
	if err != nil {
		t.Fatalf("Expected no error creating client, got %v", err)
	}
	
	// Test initial state
	if client.IsConnected() {
		t.Error("Expected client to not be connected initially")
	}
	
	if client.IsSessionActive() {
		t.Error("Expected session to not be active initially")
	}
	
	if client.GetConnectionStatus() != ConnectionStateDisconnected {
		t.Errorf("Expected initial connection status to be disconnected, got %s", client.GetConnectionStatus())
	}
	
	if client.GetCurrentSession() != nil {
		t.Error("Expected current session to be nil initially")
	}
}

// Test the getString helper function
func TestGetStringHelper(t *testing.T) {
	testMap := map[string]interface{}{
		"string_field": "test_value",
		"int_field":    42,
		"bool_field":   true,
		"nil_field":    nil,
	}
	
	// Test getting a string value
	result := getString(testMap, "string_field")
	if result != "test_value" {
		t.Errorf("Expected 'test_value', got %s", result)
	}
	
	// Test getting a non-string value
	result = getString(testMap, "int_field")
	if result != "" {
		t.Errorf("Expected empty string for non-string field, got %s", result)
	}
	
	// Test getting a non-existent field
	result = getString(testMap, "nonexistent")
	if result != "" {
		t.Errorf("Expected empty string for non-existent field, got %s", result)
	}
}