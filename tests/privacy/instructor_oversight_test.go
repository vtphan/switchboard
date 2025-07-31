// Test 1.2: Instructor Complete Oversight Validation
// P0-Critical Test for Educational Privacy Compliance
// Validates instructors maintain complete educational oversight capability

package privacy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/internal/message"
	"switchboard/internal/rate"
	"switchboard/internal/session"
	"switchboard/internal/websocket"
)

// TestInstructorCompleteOversightValidation implements Test 1.2 from validation test plan
// This is a P0-Critical test that verifies instructors maintain complete educational oversight
func TestInstructorCompleteOversightValidation(t *testing.T) {
	t.Log("👁️  EXECUTING TEST 1.2: Instructor Complete Oversight Validation (P0-Critical)")
	t.Log("📋 Test Objective: Verify instructors maintain complete educational oversight capability")
	t.Log("🏛️  Test Scenario: 2 instructors + 10 students, all 3 message types from various users")
	
	// Setup test environment
	dbManager, sessionManager, connectionRegistry, messageProcessor := setupInstructorOversightTestEnvironment(t)
	defer func() {
		require.NoError(t, dbManager.Stop())
		t.Log("🧹 Test environment cleanup completed")
	}()

	// Create and register connections: 2 instructors + 10 students
	instructors := []*OversightTrackingConnection{
		{userID: "instructor1", role: "instructor", messages: make([]TimestampedMessage, 0)},
		{userID: "instructor2", role: "instructor", messages: make([]TimestampedMessage, 0)},
	}
	
	students := []*OversightTrackingConnection{
		{userID: "student1", role: "student", messages: make([]TimestampedMessage, 0)},
		{userID: "student2", role: "student", messages: make([]TimestampedMessage, 0)},
		{userID: "student3", role: "student", messages: make([]TimestampedMessage, 0)},
		{userID: "student4", role: "student", messages: make([]TimestampedMessage, 0)},
		{userID: "student5", role: "student", messages: make([]TimestampedMessage, 0)},
		{userID: "student6", role: "student", messages: make([]TimestampedMessage, 0)},
		{userID: "student7", role: "student", messages: make([]TimestampedMessage, 0)},
		{userID: "student8", role: "student", messages: make([]TimestampedMessage, 0)},
		{userID: "student9", role: "student", messages: make([]TimestampedMessage, 0)},
		{userID: "student10", role: "student", messages: make([]TimestampedMessage, 0)},
	}

	// Register instructor connections
	for _, instructor := range instructors {
		require.NoError(t, connectionRegistry.Register(instructor.userID, instructor))
		t.Logf("✅ Instructor connection registered: %s", instructor.userID)
	}

	// Register student connections
	for _, student := range students {
		require.NoError(t, connectionRegistry.Register(student.userID, student))
		t.Logf("✅ Student connection registered: %s", student.userID)
	}

	// Create and start active session
	testSession := &database.Session{
		ID:        "oversight-test-session",
		Name:      "Instructor Complete Oversight Test Session",
		CreatedBy: "instructor1",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}
	require.NoError(t, sessionManager.SetActiveSession(testSession))
	t.Log("✅ Active session created and started")

	// Track message sending timing for latency measurement
	messageTimestamps := make(map[string]time.Time)
	
	// Phase 1: Generate 5 broadcast_to_instructors messages from students
	t.Log("📤 Phase 1: Students sending 5 broadcast_to_instructors messages")
	
	sentBroadcastMessages := make([]map[string]interface{}, 5)
	for i := 0; i < 5; i++ {
		student := students[i] // Use first 5 students
		uniqueMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{
				"text":               fmt.Sprintf("Question from %s to instructors", student.userID),
				"unique_identifier":  fmt.Sprintf("BROADCAST-%s-%d", student.userID, time.Now().UnixNano()),
				"from_user":         student.userID,
				"message_sequence":  i + 1,
			},
		}
		
		messageData, err := json.Marshal(uniqueMessage)
		require.NoError(t, err, "Failed to marshal broadcast message from %s", student.userID)
		
		sendTime := time.Now()
		messageTimestamps[uniqueMessage["content"].(map[string]interface{})["unique_identifier"].(string)] = sendTime
		
		err = messageProcessor.ProcessIncomingMessage(messageData, student.userID)
		require.NoError(t, err, "Failed to process broadcast message from %s", student.userID)
		
		sentBroadcastMessages[i] = uniqueMessage
		t.Logf("📤 Broadcast message sent from %s", student.userID)
	}
	
	// Allow processing time
	time.Sleep(50 * time.Millisecond)

	// Phase 2: Generate 3 direct_message exchanges between students  
	t.Log("📤 Phase 2: Students sending 3 direct_message exchanges")
	
	sentDirectMessages := make([]map[string]interface{}, 3)
	directMessagePairs := [][2]*OversightTrackingConnection{
		{students[5], students[6]}, // student6 -> student7
		{students[7], students[8]}, // student8 -> student9  
		{students[8], students[9]}, // student9 -> student10
	}
	
	for i, pair := range directMessagePairs {
		fromStudent, toStudent := pair[0], pair[1]
		directMessage := map[string]interface{}{
			"type":    database.MessageTypeDirectMessage,
			"context": database.ContextGeneral,
			"to_user": toStudent.userID, // Top-level field for direct messages
			"content": map[string]interface{}{
				"text":               fmt.Sprintf("Direct message from %s to %s", fromStudent.userID, toStudent.userID),
				"unique_identifier":  fmt.Sprintf("DIRECT-%s-%s-%d", fromStudent.userID, toStudent.userID, time.Now().UnixNano()),
				"from_user":         fromStudent.userID,
				"message_sequence":  i + 6, // Continue sequence from broadcast messages
			},
		}
		
		messageData, err := json.Marshal(directMessage)
		require.NoError(t, err, "Failed to marshal direct message from %s", fromStudent.userID)
		
		sendTime := time.Now()
		messageTimestamps[directMessage["content"].(map[string]interface{})["unique_identifier"].(string)] = sendTime
		
		err = messageProcessor.ProcessIncomingMessage(messageData, fromStudent.userID)
		require.NoError(t, err, "Failed to process direct message from %s", fromStudent.userID)
		
		sentDirectMessages[i] = directMessage
		t.Logf("📤 Direct message sent from %s to %s", fromStudent.userID, toStudent.userID)
	}
	
	// Allow processing time
	time.Sleep(50 * time.Millisecond)

	// Phase 3: Generate 2 broadcast_to_students announcements from instructor A
	t.Log("📤 Phase 3: Instructor A sending 2 broadcast_to_students announcements")
	
	sentAnnouncementMessages := make([]map[string]interface{}, 2)
	for i := 0; i < 2; i++ {
		announcement := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToStudents,
			"context": database.ContextAnnouncement,
			"content": map[string]interface{}{
				"text":               fmt.Sprintf("Announcement %d from instructor1 to all students", i+1),
				"unique_identifier":  fmt.Sprintf("ANNOUNCE-instructor1-%d", time.Now().UnixNano()),
				"from_user":         "instructor1",
				"message_sequence":  i + 9, // Continue sequence
			},
		}
		
		messageData, err := json.Marshal(announcement)
		require.NoError(t, err, "Failed to marshal announcement from instructor1")
		
		sendTime := time.Now()
		messageTimestamps[announcement["content"].(map[string]interface{})["unique_identifier"].(string)] = sendTime
		
		err = messageProcessor.ProcessIncomingMessage(messageData, "instructor1")
		require.NoError(t, err, "Failed to process announcement from instructor1")
		
		sentAnnouncementMessages[i] = announcement
		t.Logf("📤 Announcement sent from instructor1")
	}

	// Allow final message processing and delivery time
	time.Sleep(200 * time.Millisecond)
	t.Log("⏳ Allowing 200ms for complete message processing and delivery")

	// VERIFICATION PHASE: Critical Instructor Oversight Checks
	t.Log("🔍 Phase 4: Instructor Complete Oversight Verification")

	// Test Criterion 1: Both instructors receive ALL 10 messages (100% visibility)
	instructor1Messages := instructors[0].GetReceivedMessages()
	instructor2Messages := instructors[1].GetReceivedMessages()
	
	t.Logf("📊 Instructor1 received %d messages (expected: 10)", len(instructor1Messages))
	t.Logf("📊 Instructor2 received %d messages (expected: 10)", len(instructor2Messages))
	
	assert.Len(t, instructor1Messages, 10, 
		"CRITICAL FAILURE: Instructor1 must receive all 10 messages for complete oversight")
	assert.Len(t, instructor2Messages, 10, 
		"CRITICAL FAILURE: Instructor2 must receive all 10 messages for complete oversight")
	
	instructor1Success := len(instructor1Messages) == 10
	instructor2Success := len(instructor2Messages) == 10
	
	if instructor1Success && instructor2Success {
		t.Log("✅ SUCCESS: Both instructors have complete oversight - 100% message visibility (10/10 messages each)")
	} else {
		t.Errorf("❌ FAILURE: Instructor oversight compromised - Instructor1: %d/10, Instructor2: %d/10", 
			len(instructor1Messages), len(instructor2Messages))
	}

	// Test Criterion 2: No message filtering applied to instructor connections
	t.Log("🔍 Verifying no message filtering applied to instructor connections...")
	
	// Verify instructors received all message types
	messageTypeCounts1 := make(map[string]int)
	messageTypeCounts2 := make(map[string]int)
	
	for _, msgData := range instructor1Messages {
		var message map[string]interface{}
		if err := json.Unmarshal(msgData.Data, &message); err == nil {
			if msgType, ok := message["type"].(string); ok {
				messageTypeCounts1[msgType]++
			}
		}
	}
	
	for _, msgData := range instructor2Messages {
		var message map[string]interface{}
		if err := json.Unmarshal(msgData.Data, &message); err == nil {
			if msgType, ok := message["type"].(string); ok {
				messageTypeCounts2[msgType]++
			}
		}
	}
	
	// Expected message type distribution: 5 broadcast_to_instructors + 3 direct_message + 2 broadcast_to_students
	expectedCounts := map[string]int{
		database.MessageTypeBroadcastToInstructors: 5,
		database.MessageTypeDirectMessage:         3,
		database.MessageTypeBroadcastToStudents:   2,
	}
	
	filteringSuccessful := true
	for msgType, expectedCount := range expectedCounts {
		count1 := messageTypeCounts1[msgType]
		count2 := messageTypeCounts2[msgType]
		
		if count1 != expectedCount || count2 != expectedCount {
			t.Errorf("❌ FILTERING FAILURE: Expected %d %s messages, Instructor1 got %d, Instructor2 got %d", 
				expectedCount, msgType, count1, count2)
			filteringSuccessful = false
		} else {
			t.Logf("✅ Message type %s correctly received by both instructors (%d each)", msgType, expectedCount)
		}
	}

	// Test Criterion 3: Real-time delivery (<100ms latency) maintained
	t.Log("⚡ Phase 5: Real-time Delivery Latency Verification")
	
	latencyViolations := 0
	totalLatency := time.Duration(0)
	measuredLatencies := 0
	
	// Measure latency for instructor1 messages
	for _, msgData := range instructor1Messages {
		var message map[string]interface{}
		if err := json.Unmarshal(msgData.Data, &message); err == nil {
			if content, ok := message["content"].(map[string]interface{}); ok {
				if uniqueID, exists := content["unique_identifier"].(string); exists {
					if sendTime, found := messageTimestamps[uniqueID]; found {
						latency := msgData.ReceivedAt.Sub(sendTime)
						totalLatency += latency
						measuredLatencies++
						
						if latency > 100*time.Millisecond {
							latencyViolations++
							t.Logf("⚠️  Latency violation: Message %s took %v to deliver", uniqueID, latency)
						}
					}
				}
			}
		}
	}
	
	averageLatency := time.Duration(0)
	if measuredLatencies > 0 {
		averageLatency = totalLatency / time.Duration(measuredLatencies)
	}
	
	t.Logf("📊 Average message delivery latency: %v (target: <100ms)", averageLatency)
	t.Logf("📊 Latency violations: %d/%d messages", latencyViolations, measuredLatencies)
	
	latencySuccess := latencyViolations == 0 && averageLatency < 100*time.Millisecond
	if latencySuccess {
		t.Log("✅ SUCCESS: Real-time delivery requirements met")
	} else {
		t.Errorf("❌ FAILURE: Real-time delivery requirements not met")
	}

	// Test Criterion 4: Message ordering preserved across instructor connections
	t.Log("🔄 Phase 6: Message Ordering Preservation Verification")
	
	// Verify chronological ordering is preserved
	orderingSuccess1 := verifyMessageOrdering(instructor1Messages, t, "Instructor1")
	orderingSuccess2 := verifyMessageOrdering(instructor2Messages, t, "Instructor2")
	
	overallOrderingSuccess := orderingSuccess1 && orderingSuccess2

	// FINAL TEST RESULT DETERMINATION
	t.Log("🏁 Phase 7: Final Test Result Analysis")
	
	// Aggregate all test criteria results
	testResults := map[string]bool{
		"instructor1_complete_visibility": instructor1Success,
		"instructor2_complete_visibility": instructor2Success,
		"no_message_filtering_applied":   filteringSuccessful,
		"realtime_delivery_latency":      latencySuccess,
		"message_ordering_preserved":     overallOrderingSuccess,
	}
	
	allTestsPassed := true
	for criterion, passed := range testResults {
		if passed {
			t.Logf("✅ %s: PASSED", criterion)
		} else {
			t.Errorf("❌ %s: FAILED", criterion)
			allTestsPassed = false
		}
	}
	
	// Generate comprehensive test execution report
	t.Log("\n" + generateInstructorOversightTestReport(testResults, len(instructor1Messages), len(instructor2Messages),
		messageTypeCounts1, messageTypeCounts2, averageLatency, latencyViolations, measuredLatencies))
	
	if allTestsPassed {
		t.Log("🎉 TEST 1.2 FINAL RESULT: ✅ PASSED - Instructor Complete Oversight Validation successful")
		t.Log("👁️  Educational oversight compliance verified - Instructors maintain complete system visibility")
	} else {
		t.Log("⛔ TEST 1.2 FINAL RESULT: ❌ FAILED - Instructor oversight requirements not met")
		t.Log("🚨 PRODUCTION BLOCKER: System must not be deployed until oversight issues are resolved")
		t.Fail() // Explicitly fail the test for CI/CD pipeline
	}
}

// verifyMessageOrdering checks if messages are received in chronological order
func verifyMessageOrdering(messages []TimestampedMessage, t *testing.T, instructorName string) bool {
	if len(messages) < 2 {
		return true // Can't verify ordering with less than 2 messages
	}
	
	orderingViolations := 0
	for i := 1; i < len(messages); i++ {
		if messages[i].ReceivedAt.Before(messages[i-1].ReceivedAt) {
			orderingViolations++
			t.Logf("⚠️  %s ordering violation: Message %d received before message %d", 
				instructorName, i+1, i)
		}
	}
	
	if orderingViolations == 0 {
		t.Logf("✅ %s: Message ordering preserved (%d messages in correct chronological order)", 
			instructorName, len(messages))
		return true
	} else {
		t.Errorf("❌ %s: %d message ordering violations detected", instructorName, orderingViolations)
		return false
	}
}

// setupInstructorOversightTestEnvironment creates the test environment for instructor oversight testing
func setupInstructorOversightTestEnvironment(t *testing.T) (*database.SQLiteDatabaseManager, session.SessionManager, *websocket.ConnectionRegistry, *message.MessageProcessor) {
	// Setup in-memory SQLite database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err, "Failed to create in-memory database")

	// Apply database schema
	schemaPath := "../../internal/database/migrations.sql"
	schema, err := os.ReadFile(schemaPath)
	require.NoError(t, err, "Failed to read database schema")
	
	_, err = db.Exec(string(schema))
	require.NoError(t, err, "Failed to apply database schema")

	// Create database manager
	// Skip table initialization and pragmas since we already applied migrations.sql
	dbManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
		SkipTableInit: true,
		SkipPragmas:   true,
	})
	require.NoError(t, err, "Failed to create database manager")
	require.NoError(t, dbManager.Start(), "Failed to start database manager")

	// Create session manager
	sessionManager := session.NewSessionManager(dbManager)

	// Create connection registry with background cleanup
	connectionRegistry := websocket.NewConnectionRegistry(sessionManager)
	connectionRegistry.Start()

	// Create message filtering and routing components
	roleBasedFilter := message.NewRoleBasedFilter()
	filterAdapter := websocket.NewFilterAdapter(roleBasedFilter)
	broadcastSystem := websocket.NewBroadcastSystem(connectionRegistry, filterAdapter)
	messageRouter := message.NewMessageRouter(connectionRegistry, roleBasedFilter)
	rateLimiter := rate.NewRateLimiter()

	// Create message processor with all components
	messageProcessor := message.NewMessageProcessor(
		sessionManager,
		dbManager,
		rateLimiter,
		messageRouter,
		broadcastSystem,
	)

	t.Log("🏗️  Test environment setup completed successfully")
	return dbManager, sessionManager, connectionRegistry, messageProcessor
}

// TimestampedMessage represents a message with receive timestamp for latency measurement
type TimestampedMessage struct {
	Data       []byte
	ReceivedAt time.Time
}

// OversightTrackingConnection implements connection interface for oversight testing with latency tracking
type OversightTrackingConnection struct {
	userID   string
	role     string
	messages []TimestampedMessage
	mu       sync.RWMutex
	closed   bool
}

func (otc *OversightTrackingConnection) GetUserID() string {
	otc.mu.RLock()
	defer otc.mu.RUnlock()
	return otc.userID
}

func (otc *OversightTrackingConnection) GetRole() string {
	otc.mu.RLock()
	defer otc.mu.RUnlock()
	return otc.role
}

func (otc *OversightTrackingConnection) SendMessage(data []byte) error {
	otc.mu.Lock()
	defer otc.mu.Unlock()
	
	if otc.closed {
		return fmt.Errorf("connection closed for user %s", otc.userID)
	}
	
	// Create timestamped message for latency tracking
	timestampedMsg := TimestampedMessage{
		Data:       make([]byte, len(data)),
		ReceivedAt: time.Now(),
	}
	copy(timestampedMsg.Data, data)
	
	otc.messages = append(otc.messages, timestampedMsg)
	return nil
}

func (otc *OversightTrackingConnection) GetReceivedMessages() []TimestampedMessage {
	otc.mu.RLock()
	defer otc.mu.RUnlock()
	
	// Return copies to prevent external modifications
	result := make([]TimestampedMessage, len(otc.messages))
	for i, msg := range otc.messages {
		result[i] = TimestampedMessage{
			Data:       make([]byte, len(msg.Data)),
			ReceivedAt: msg.ReceivedAt,
		}
		copy(result[i].Data, msg.Data)
	}
	return result
}

func (otc *OversightTrackingConnection) Close() error {
	otc.mu.Lock()
	defer otc.mu.Unlock()
	otc.closed = true
	return nil
}

func (otc *OversightTrackingConnection) GetLastSeen() time.Time {
	return time.Now()
}

func (otc *OversightTrackingConnection) WriteJSON(v interface{}) error {
	return nil // Not needed for this test
}

func (otc *OversightTrackingConnection) SetCredentials(username, role string) error {
	return nil // Not needed for this test
}

func (otc *OversightTrackingConnection) UpdateActivity() {
	// Not needed for this test
}

func (otc *OversightTrackingConnection) SendCloseMessage(reason string) error {
	return nil // Not needed for this test
}

// generateInstructorOversightTestReport creates a comprehensive test execution report
func generateInstructorOversightTestReport(results map[string]bool, inst1MsgCount, inst2MsgCount int, 
	msgTypes1, msgTypes2 map[string]int, avgLatency time.Duration, latencyViolations, totalMeasured int) string {
	
	report := `
==================================================
TEST 1.2: INSTRUCTOR COMPLETE OVERSIGHT VALIDATION
P0-CRITICAL EDUCATIONAL OVERSIGHT COMPLIANCE TEST
==================================================

TEST EXECUTION SUMMARY:
📊 Instructor1 Messages Received: %d/10 (Target: 10)
📊 Instructor2 Messages Received: %d/10 (Target: 10)
📊 Average Message Delivery Latency: %v (Target: <100ms)
📊 Latency Violations: %d/%d messages (Target: 0)

MESSAGE TYPE DISTRIBUTION:
📋 Instructor1 - broadcast_to_instructors: %d/5, direct_message: %d/3, broadcast_to_students: %d/2
📋 Instructor2 - broadcast_to_instructors: %d/5, direct_message: %d/3, broadcast_to_students: %d/2

DETAILED RESULTS:
%s Instructor1 Complete Visibility: %s
%s Instructor2 Complete Visibility: %s
%s No Message Filtering Applied: %s
%s Real-time Delivery Latency: %s
%s Message Ordering Preserved: %s

COMPLIANCE STATUS:
Educational Oversight: %s
Production Readiness: %s
Instructor Capabilities: %s

==================================================
`
	
	getStatus := func(passed bool) string {
		if passed { return "✅ PASSED" }
		return "❌ FAILED"
	}
	
	getIcon := func(passed bool) string {
		if passed { return "✅" }
		return "❌"
	}
	
	overallPass := true
	for _, passed := range results {
		if !passed {
			overallPass = false
			break
		}
	}
	
	return fmt.Sprintf(report,
		inst1MsgCount, inst2MsgCount, avgLatency, latencyViolations, totalMeasured,
		msgTypes1[database.MessageTypeBroadcastToInstructors], msgTypes1[database.MessageTypeDirectMessage], msgTypes1[database.MessageTypeBroadcastToStudents],
		msgTypes2[database.MessageTypeBroadcastToInstructors], msgTypes2[database.MessageTypeDirectMessage], msgTypes2[database.MessageTypeBroadcastToStudents],
		getIcon(results["instructor1_complete_visibility"]), getStatus(results["instructor1_complete_visibility"]),
		getIcon(results["instructor2_complete_visibility"]), getStatus(results["instructor2_complete_visibility"]),
		getIcon(results["no_message_filtering_applied"]), getStatus(results["no_message_filtering_applied"]),
		getIcon(results["realtime_delivery_latency"]), getStatus(results["realtime_delivery_latency"]),
		getIcon(results["message_ordering_preserved"]), getStatus(results["message_ordering_preserved"]),
		getStatus(overallPass),
		getStatus(overallPass),
		getStatus(overallPass),
	)
}