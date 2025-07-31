package websocket

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"switchboard/internal/database"
	msg "switchboard/internal/message"
)

// MockConnection implements ConnectionInterface for testing
type MockConnection struct {
	mu           sync.Mutex
	userID       string
	role         string
	sentMessages [][]byte
	sendError    error
	closed       bool
}

func (mc *MockConnection) WriteJSON(v interface{}) error {
	return mc.SendMessage([]byte(`{"mock":"json"}`))
}

func (mc *MockConnection) SendMessage(data []byte) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if mc.sendError != nil {
		return mc.sendError
	}
	mc.sentMessages = append(mc.sentMessages, data)
	return nil
}

func (mc *MockConnection) Close() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.closed = true
	return nil
}

func (mc *MockConnection) GetUserID() string {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.userID
}

func (mc *MockConnection) GetRole() string {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.role
}

func (mc *MockConnection) SetCredentials(username, role string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.userID = username
	mc.role = role
	return nil
}

func (mc *MockConnection) UpdateActivity() {}

func (mc *MockConnection) GetLastSeen() time.Time {
	return time.Now()
}

func (mc *MockConnection) SendCloseMessage(reason string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	// Mock implementation - just mark as closed
	mc.closed = true
	return nil
}

// Helper method for thread-safe access to sentMessages in tests
func (mc *MockConnection) GetSentMessageCount() int {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return len(mc.sentMessages)
}

// MockRoleBasedFilter for testing
type MockRoleBasedFilter struct {
	filterResults map[string]bool // key: messageID+recipientRole+recipientUserID
}

func (mf *MockRoleBasedFilter) ShouldReceiveMessage(msg *database.Message, recipientRole, recipientUserID string) bool {
	key := msg.ID + recipientRole + recipientUserID
	if result, exists := mf.filterResults[key]; exists {
		return result
	}
	return true // Default allow
}

// Test data factory
func createTestMessage(msgType, fromUser string, toUser *string) *database.Message {
	return &database.Message{
		ID:        "test-msg-001",
		SessionID: "session-123",
		Type:      msgType,
		Context:   "question",
		FromUser:  fromUser,
		ToUser:    toUser,
		Content:   map[string]interface{}{"text": "test message"},
		Timestamp: time.Now(),
	}
}

func createTestRecipients() []msg.Recipient {
	return []msg.Recipient{
		&MockConnection{userID: "instructor1", role: "instructor"},
		&MockConnection{userID: "student1", role: "student"},
		&MockConnection{userID: "student2", role: "student"},
	}
}

// ARCHITECTURAL TESTS
func TestBroadcastSystem_Architecture_FileStructure(t *testing.T) {
	// Test that BroadcastSystem is in correct package
	var bs *BroadcastSystem
	assert.IsType(t, &BroadcastSystem{}, bs)
}

func TestBroadcastSystem_Architecture_Dependencies(t *testing.T) {
	// Test required imports are available
	registry := &ConnectionRegistry{}
	mockFilter := &MockRoleBasedFilter{}
	
	// This should compile without errors if dependencies are correct
	_ = NewBroadcastSystem(registry, mockFilter)
}

func TestBroadcastSystem_Architecture_InterfaceCompliance(t *testing.T) {
	// Test BroadcastSystem implements expected interface patterns
	registry := &ConnectionRegistry{}
	mockFilter := &MockRoleBasedFilter{}
	bs := NewBroadcastSystem(registry, mockFilter)
	
	// Should have BroadcastMessage method
	recipients := createTestRecipients()
	message := createTestMessage("broadcast_to_instructors", "student1", nil)
	
	err := bs.BroadcastMessage(message, recipients)
	// BroadcastSystem is now implemented, should work
	assert.NoError(t, err) // Expecting success now that it's implemented
}

// FUNCTIONAL TESTS
func TestBroadcastSystem_BroadcastMessage_EmptyRecipients(t *testing.T) {
	registry := &ConnectionRegistry{}
	filter := &MockRoleBasedFilter{}
	bs := NewBroadcastSystem(registry, filter)
	
	message := createTestMessage("broadcast_to_instructors", "student1", nil)
	
	var recipients []msg.Recipient
	err := bs.BroadcastMessage(message, recipients)
	// Should not error with empty recipients
	assert.NoError(t, err)
}

func TestBroadcastSystem_BroadcastMessage_RoleBasedFiltering(t *testing.T) {
	registry := &ConnectionRegistry{}
	filter := &MockRoleBasedFilter{
		filterResults: map[string]bool{
			"test-msg-001instructorinstructor1": true,  // instructor should receive
			"test-msg-001studentstudent1":       false, // students should not receive
		},
	}
	bs := NewBroadcastSystem(registry, filter)
	
	recipients := []msg.Recipient{
		&MockConnection{userID: "instructor1", role: "instructor"},
		&MockConnection{userID: "student1", role: "student"},
	}
	message := createTestMessage("broadcast_to_instructors", "student1", nil)
	
	err := bs.BroadcastMessage(message, recipients)
	
	// Should succeed but only deliver to instructor
	assert.NoError(t, err)
	
	// Verify filtering applied correctly
	instructorConn := recipients[0].(*MockConnection)
	studentConn := recipients[1].(*MockConnection)
	
	assert.Equal(t, 1, instructorConn.GetSentMessageCount())
	assert.Equal(t, 0, studentConn.GetSentMessageCount())
}

func TestBroadcastSystem_BroadcastMessage_NonBlockingDelivery(t *testing.T) {
	registry := &ConnectionRegistry{}
	filter := &MockRoleBasedFilter{}
	bs := NewBroadcastSystem(registry, filter)
	
	// Create recipients with one failing connection
	recipients := []msg.Recipient{
		&MockConnection{userID: "good1", role: "instructor"},
		&MockConnection{userID: "fail1", role: "instructor", sendError: errors.New("send failed")},
		&MockConnection{userID: "good2", role: "instructor"},
	}
	message := createTestMessage("broadcast_to_instructors", "student1", nil)
	
	err := bs.BroadcastMessage(message, recipients)
	
	// Should not error - failed deliveries don't block others
	assert.NoError(t, err)
	
	// Verify successful deliveries happened
	goodConn1 := recipients[0].(*MockConnection)
	goodConn2 := recipients[2].(*MockConnection)
	
	assert.Equal(t, 1, goodConn1.GetSentMessageCount())
	assert.Equal(t, 1, goodConn2.GetSentMessageCount())
}

func TestBroadcastSystem_BroadcastMessage_JSONMarshaling(t *testing.T) {
	registry := &ConnectionRegistry{}
	filter := &MockRoleBasedFilter{}
	bs := NewBroadcastSystem(registry, filter)
	
	recipients := []msg.Recipient{
		&MockConnection{userID: "instructor1", role: "instructor"},
	}
	
	// Test with complex message content
	message := &database.Message{
		ID:          "complex-msg",
		SessionID:   "session-123",
		Type:        "broadcast_to_instructors",
		Context:     "question",
		FromUser:    "student1",
		ToUser:      nil,
		Content:     map[string]interface{}{"text": "test", "priority": 1, "tags": []string{"urgent"}},
		Timestamp:   time.Now(),
	}
	
	err := bs.BroadcastMessage(message, recipients)
	assert.NoError(t, err)
	
	// Verify message was delivered
	conn := recipients[0].(*MockConnection)
	assert.Equal(t, 1, conn.GetSentMessageCount())
}

func TestBroadcastSystem_BroadcastMessage_DirectMessageHandling(t *testing.T) {
	registry := &ConnectionRegistry{}
	filter := &MockRoleBasedFilter{}
	bs := NewBroadcastSystem(registry, filter)
	
	toUser := "student2"
	recipients := []msg.Recipient{
		&MockConnection{userID: "instructor1", role: "instructor"},
		&MockConnection{userID: "student2", role: "student"},
	}
	message := createTestMessage("direct_message", "instructor1", &toUser)
	
	err := bs.BroadcastMessage(message, recipients)
	assert.NoError(t, err)
	
	// Both should receive (instructor sees all, student2 is recipient)
	instructorConn := recipients[0].(*MockConnection)
	studentConn := recipients[1].(*MockConnection)
	
	assert.Equal(t, 1, instructorConn.GetSentMessageCount())
	assert.Equal(t, 1, studentConn.GetSentMessageCount())
}

// INTEGRATION TESTS
func TestBroadcastSystem_Integration_WithConnectionRegistry(t *testing.T) {
	sessionManager := &mockSessionManager{}
	registry := NewConnectionRegistry(sessionManager)
	registry.Start()
	defer registry.Stop()
	
	filter := &MockRoleBasedFilter{}
	bs := NewBroadcastSystem(registry, filter)
	
	// Register connections
	instructorConn := &MockConnection{userID: "instructor1", role: "instructor"}
	studentConn := &MockConnection{userID: "student1", role: "student"}
	
	_ = registry.Register("instructor1", instructorConn)
	_ = registry.Register("student1", studentConn)
	
	// Get recipients from registry
	allRecipients := registry.GetAllUsers()
	message := createTestMessage("broadcast_to_students", "instructor1", nil)
	
	err := bs.BroadcastMessage(message, allRecipients)
	assert.NoError(t, err)
	
	// Verify integration works
	assert.Equal(t, 1, instructorConn.GetSentMessageCount()) // Instructor sees all
	assert.Equal(t, 1, studentConn.GetSentMessageCount())   // Student receives announcement
}

func TestBroadcastSystem_Integration_WithRoleBasedFilter(t *testing.T) {
	registry := &ConnectionRegistry{}
	
	// Integration with actual RoleBasedFilter patterns
	filter := &MockRoleBasedFilter{
		filterResults: map[string]bool{
			// Student question to instructors - student privacy
			"test-msg-001instructorinstructor1": true,  // Instructor sees question
			"test-msg-001studentstudent1":       false, // Asking student doesn't see own question in broadcast
			"test-msg-001studentstudent2":       false, // Other students don't see question
		},
	}
	bs := NewBroadcastSystem(registry, filter)
	
	recipients := []msg.Recipient{
		&MockConnection{userID: "instructor1", role: "instructor"},
		&MockConnection{userID: "student1", role: "student"},  // Original sender
		&MockConnection{userID: "student2", role: "student"},  // Other student
	}
	message := createTestMessage("broadcast_to_instructors", "student1", nil)
	
	err := bs.BroadcastMessage(message, recipients)
	assert.NoError(t, err)
	
	// Verify educational privacy preserved
	instructorConn := recipients[0].(*MockConnection)
	student1Conn := recipients[1].(*MockConnection)
	student2Conn := recipients[2].(*MockConnection)
	
	assert.Equal(t, 1, instructorConn.GetSentMessageCount()) // Instructor sees question
	assert.Equal(t, 0, student1Conn.GetSentMessageCount())  // Sender doesn't see echo
	assert.Equal(t, 0, student2Conn.GetSentMessageCount())  // Other students don't see
}

func TestBroadcastSystem_Integration_MessageProcessorCompatibility(t *testing.T) {
	registry := &ConnectionRegistry{}
	filter := &MockRoleBasedFilter{}
	bs := NewBroadcastSystem(registry, filter)
	
	// Test that BroadcastSystem can be called from MessageProcessor context
	recipients := createTestRecipients()
	message := createTestMessage("broadcast_to_students", "instructor1", nil)
	
	// This simulates MessageProcessor calling BroadcastSystem
	err := bs.BroadcastMessage(message, recipients)
	assert.NoError(t, err)
	
	// Verify all recipients received message (no filtering for instructor announcement)
	for _, recipient := range recipients {
		conn := recipient.(*MockConnection)
		assert.Equal(t, 1, conn.GetSentMessageCount())
	}
}

// TECHNICAL TESTS
func TestBroadcastSystem_Technical_ConcurrentBroadcasts(t *testing.T) {
	registry := &ConnectionRegistry{}
	filter := &MockRoleBasedFilter{}
	bs := NewBroadcastSystem(registry, filter)
	
	recipients := createTestRecipients()
	
	// Concurrent broadcasts should not interfere
	const numBroadcasts = 10
	done := make(chan bool, numBroadcasts)
	
	for i := 0; i < numBroadcasts; i++ {
		go func(index int) {
			message := &database.Message{
				ID:        "concurrent-msg-" + string(rune(index)),
				SessionID: "session-123",
				Type:      "broadcast_to_students",
				Context:   "announcement",
				FromUser:  "instructor1",
				Content:   map[string]interface{}{"text": "message " + string(rune(index))},
				Timestamp: time.Now(),
			}
			err := bs.BroadcastMessage(message, recipients)
			assert.NoError(t, err)
			done <- true
		}(i)
	}
	
	// Wait for all broadcasts to complete
	for i := 0; i < numBroadcasts; i++ {
		<-done
	}
	
	// Verify all messages were delivered
	for _, recipient := range recipients {
		conn := recipient.(*MockConnection)
		assert.Equal(t, numBroadcasts, conn.GetSentMessageCount())
	}
}

func TestBroadcastSystem_Technical_ErrorHandling(t *testing.T) {
	registry := &ConnectionRegistry{}
	filter := &MockRoleBasedFilter{}
	bs := NewBroadcastSystem(registry, filter)
	
	// Test various error scenarios
	t.Run("AllRecipientsFailDelivery", func(t *testing.T) {
		recipients := []msg.Recipient{
			&MockConnection{userID: "fail1", role: "instructor", sendError: errors.New("network error")},
			&MockConnection{userID: "fail2", role: "instructor", sendError: errors.New("connection closed")},
		}
		message := createTestMessage("broadcast_to_instructors", "student1", nil)
		
		err := bs.BroadcastMessage(message, recipients)
		// Should error when all deliveries fail
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to deliver message to any recipients")
	})
	
	t.Run("PartialDeliverySuccess", func(t *testing.T) {
		recipients := []msg.Recipient{
			&MockConnection{userID: "good1", role: "instructor"},
			&MockConnection{userID: "fail1", role: "instructor", sendError: errors.New("network error")},
		}
		message := createTestMessage("broadcast_to_instructors", "student1", nil)
		
		err := bs.BroadcastMessage(message, recipients)
		// Should succeed with partial delivery
		assert.NoError(t, err)
	})
}

func TestBroadcastSystem_Technical_MemoryUsage(t *testing.T) {
	registry := &ConnectionRegistry{}
	filter := &MockRoleBasedFilter{}
	bs := NewBroadcastSystem(registry, filter)
	
	recipients := createTestRecipients()
	
	// Test that BroadcastSystem doesn't accumulate memory
	for i := 0; i < 1000; i++ {
		message := &database.Message{
			ID:        "memory-test-" + string(rune(i)),
			SessionID: "session-123",
			Type:      "broadcast_to_students",
			Context:   "test",
			FromUser:  "instructor1",
			Content:   map[string]interface{}{"iteration": i},
			Timestamp: time.Now(),
		}
		err := bs.BroadcastMessage(message, recipients)
		assert.NoError(t, err)
	}
	
	// BroadcastSystem should not retain message references
	// This is a basic test - production would use memory profiling
	assert.NotNil(t, bs) // BroadcastSystem still functional
}

func TestBroadcastSystem_Technical_Performance(t *testing.T) {
	registry := &ConnectionRegistry{}
	filter := &MockRoleBasedFilter{}
	bs := NewBroadcastSystem(registry, filter)
	
	recipients := make([]msg.Recipient, 50) // 50 connections
	for i := 0; i < 50; i++ {
		recipients[i] = &MockConnection{
			userID: "user" + string(rune(i)),
			role:   "student",
		}
	}
	
	message := createTestMessage("broadcast_to_students", "instructor1", nil)
	
	// Measure broadcast performance
	start := time.Now()
	err := bs.BroadcastMessage(message, recipients)
	duration := time.Since(start)
	
	assert.NoError(t, err)
	// Broadcast to 50 recipients should complete quickly
	assert.Less(t, duration, 100*time.Millisecond)
	
	// Verify all recipients received message
	for _, recipient := range recipients {
		conn := recipient.(*MockConnection)
		assert.Equal(t, 1, conn.GetSentMessageCount())
	}
}