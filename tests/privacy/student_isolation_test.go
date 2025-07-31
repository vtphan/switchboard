// Test 1.1: Student Message Isolation Verification
// P0-Critical Test for Educational Privacy Compliance
// Validates complete privacy isolation between students for broadcast_to_instructors messages

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

// TestStudentMessageIsolationVerification implements Test 1.1 from validation test plan
// This is a P0-Critical test that verifies complete privacy isolation between students
func TestStudentMessageIsolationVerification(t *testing.T) {
	t.Log("🔒 EXECUTING TEST 1.1: Student Message Isolation Verification (P0-Critical)")
	t.Log("📋 Test Objective: Verify complete privacy isolation between students")
	t.Log("🏛️  Test Scenario: 1 instructor + 5 students, each student sends broadcast_to_instructors message")
	
	// Setup test environment
	dbManager, sessionManager, connectionRegistry, messageProcessor := setupStudentIsolationTestEnvironment(t)
	defer func() {
		require.NoError(t, dbManager.Stop())
		t.Log("🧹 Test environment cleanup completed")
	}()

	// Create and register connections: 1 instructor + 5 students
	instructor := &IsolationTrackingConnection{
		userID:   "instructor1",
		role:     "instructor",
		messages: make([][]byte, 0),
	}
	
	students := []*IsolationTrackingConnection{
		{userID: "student1", role: "student", messages: make([][]byte, 0)},
		{userID: "student2", role: "student", messages: make([][]byte, 0)},
		{userID: "student3", role: "student", messages: make([][]byte, 0)},
		{userID: "student4", role: "student", messages: make([][]byte, 0)},
		{userID: "student5", role: "student", messages: make([][]byte, 0)},
	}

	// Register instructor connection
	require.NoError(t, connectionRegistry.Register("instructor1", instructor))
	t.Log("✅ Instructor connection registered: instructor1")

	// Register student connections
	for i, student := range students {
		require.NoError(t, connectionRegistry.Register(student.userID, student))
		t.Logf("✅ Student connection registered: %s", student.userID)
		_ = i // avoid unused variable
	}

	// Create and start active session
	testSession := &database.Session{
		ID:        "isolation-test-session",
		Name:      "Student Message Isolation Test Session",
		CreatedBy: "instructor1",
		StartTime: time.Now(),
		Status:    database.SessionStatusActive,
	}
	require.NoError(t, sessionManager.SetActiveSession(testSession))
	t.Log("✅ Active session created and started")

	// Each student sends a broadcast_to_instructors message with unique identifier
	t.Log("📤 Phase 1: Students sending broadcast_to_instructors messages")
	
	sentMessages := make([]map[string]interface{}, 5)
	for i, student := range students {
		uniqueMessage := map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{
				"text":               fmt.Sprintf("Question from %s", student.userID),
				"unique_identifier":  fmt.Sprintf("MSG-%s-%d", student.userID, time.Now().UnixNano()),
				"student_id":        student.userID,
				"test_sequence":     i + 1,
			},
		}
		
		messageData, err := json.Marshal(uniqueMessage)
		require.NoError(t, err, "Failed to marshal message from %s", student.userID)
		
		err = messageProcessor.ProcessIncomingMessage(messageData, student.userID)
		require.NoError(t, err, "Failed to process message from %s", student.userID)
		
		sentMessages[i] = uniqueMessage
		t.Logf("📤 Message sent from %s with unique ID: %s", 
			student.userID, uniqueMessage["content"].(map[string]interface{})["unique_identifier"])
	}

	// Allow message processing and delivery time
	time.Sleep(200 * time.Millisecond)
	t.Log("⏳ Allowing 200ms for message processing and delivery")

	// VERIFICATION PHASE: Critical Privacy Isolation Checks
	t.Log("🔍 Phase 2: Privacy Isolation Verification")

	// Test Criterion 1: Instructor receives ALL 5 messages (100% oversight)
	instructorMessages := instructor.GetReceivedMessages()
	t.Logf("📊 Instructor received %d messages (expected: 5)", len(instructorMessages))
	
	assert.Len(t, instructorMessages, 5, 
		"CRITICAL FAILURE: Instructor must receive all 5 student messages for complete oversight")
	
	if len(instructorMessages) == 5 {
		t.Log("✅ SUCCESS: Instructor oversight - 100% message receipt (5/5 messages)")
	} else {
		t.Errorf("❌ FAILURE: Instructor oversight compromised - received %d/5 messages", len(instructorMessages))
	}

	// Test Criterion 2: Zero cross-student message visibility
	t.Log("🔒 Verifying zero cross-student message visibility...")
	
	totalStudentMessages := 0
	privacyViolations := 0
	studentMessageCounts := make(map[string]int)
	
	for _, student := range students {
		receivedMessages := student.GetReceivedMessages()
		messageCount := len(receivedMessages)
		totalStudentMessages += messageCount
		studentMessageCounts[student.userID] = messageCount
		
		t.Logf("📊 %s received %d messages (expected: 0)", student.userID, messageCount)
		
		// Analyze each message received by student to detect privacy violations
		for _, msgData := range receivedMessages {
			var message map[string]interface{}
			if err := json.Unmarshal(msgData, &message); err == nil {
				if msgType, ok := message["type"].(string); ok && msgType == database.MessageTypeBroadcastToInstructors {
					privacyViolations++
					if fromUser, exists := message["from_user"].(string); exists {
						t.Errorf("❌ PRIVACY VIOLATION: %s received broadcast_to_instructors message from %s", 
							student.userID, fromUser)
					}
				}
			}
		}
		
		assert.Equal(t, 0, messageCount, 
			"CRITICAL PRIVACY FAILURE: Student %s must not receive any broadcast_to_instructors messages", 
			student.userID)
	}

	// Test Criterion 3: Comprehensive privacy isolation validation
	if totalStudentMessages == 0 {
		t.Log("✅ SUCCESS: Complete privacy isolation - Zero cross-student message visibility (0/5000 target met)")
	} else {
		t.Errorf("❌ CRITICAL PRIVACY FAILURE: %d privacy violations detected - students received %d total messages", 
			privacyViolations, totalStudentMessages)
	}

	// Test Criterion 4: Message audit log verification
	t.Log("📋 Phase 3: Message Audit Log Verification")
	
	time.Sleep(100 * time.Millisecond) // Allow async database operations to complete
	
	auditMessages, err := dbManager.GetSessionMessages(testSession.ID)
	require.NoError(t, err, "Failed to retrieve audit messages from database")
	
	t.Logf("📊 Audit log contains %d messages (expected: 5)", len(auditMessages))
	assert.Len(t, auditMessages, 5, "Audit log must contain all 5 messages for compliance")
	
	// Verify each message in audit log has correct type and metadata
	auditedMessageTypes := make(map[string]int)
	for _, auditMsg := range auditMessages {
		auditedMessageTypes[auditMsg.Type]++
		
		// Verify message type is broadcast_to_instructors
		assert.Equal(t, database.MessageTypeBroadcastToInstructors, auditMsg.Type, 
			"Audit message must have correct type")
		
		// Verify from_user is a student
		assert.Contains(t, []string{"student1", "student2", "student3", "student4", "student5"}, 
			auditMsg.FromUser, "Audit message must have valid student sender")
	}
	
	expectedAuditCount := auditedMessageTypes[database.MessageTypeBroadcastToInstructors]
	if expectedAuditCount == 5 {
		t.Log("✅ SUCCESS: Message audit log shows correct filtering applied (5/5 messages audited)")
	} else {
		t.Errorf("❌ AUDIT FAILURE: Expected 5 broadcast_to_instructors messages in audit, found %d", expectedAuditCount)
	}

	// Test Criterion 5: Performance and timing validation
	t.Log("⚡ Phase 4: Performance Validation")
	
	// Verify message processing completed within acceptable timeframe
	startTime := time.Now()
	
	// Send one more test message to verify continued performance
	perfTestMessage := map[string]interface{}{
		"type":    database.MessageTypeBroadcastToInstructors,
		"context": database.ContextQuestion,
		"content": map[string]interface{}{
			"text": "Performance validation message",
			"timestamp": startTime.UnixNano(),
		},
	}
	
	perfMsgData, _ := json.Marshal(perfTestMessage)
	err = messageProcessor.ProcessIncomingMessage(perfMsgData, "student1")
	require.NoError(t, err)
	
	time.Sleep(50 * time.Millisecond)
	processingDuration := time.Since(startTime)
	
	t.Logf("📊 Message processing latency: %v (target: <100ms)", processingDuration)
	assert.Less(t, processingDuration, 100*time.Millisecond, 
		"Message processing must complete within 100ms for real-time requirements")

	// FINAL TEST RESULT DETERMINATION
	t.Log("🏁 Phase 5: Final Test Result Analysis")
	
	// Aggregate all test criteria results
	testResults := map[string]bool{
		"instructor_complete_oversight": len(instructorMessages) == 5,
		"zero_cross_student_visibility": totalStudentMessages == 0,
		"audit_log_compliance": len(auditMessages) == 5 && expectedAuditCount == 5,
		"performance_requirements": processingDuration < 100*time.Millisecond,
		"privacy_violations": privacyViolations == 0,
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
	t.Log("\n" + generateTestExecutionReport(testResults, len(instructorMessages), totalStudentMessages, 
		privacyViolations, len(auditMessages), processingDuration))
	
	if allTestsPassed {
		t.Log("🎉 TEST 1.1 FINAL RESULT: ✅ PASSED - Student Message Isolation Verification successful")
		t.Log("🔒 Educational privacy compliance verified - Ready for production deployment")
	} else {
		t.Log("⛔ TEST 1.1 FINAL RESULT: ❌ FAILED - Privacy isolation requirements not met")
		t.Log("🚨 PRODUCTION BLOCKER: System must not be deployed until privacy issues are resolved")
		t.Fail() // Explicitly fail the test for CI/CD pipeline
	}
}

// setupStudentIsolationTestEnvironment creates the test environment for student isolation testing
func setupStudentIsolationTestEnvironment(t *testing.T) (*database.SQLiteDatabaseManager, session.SessionManager, *websocket.ConnectionRegistry, *message.MessageProcessor) {
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

// IsolationTrackingConnection implements connection interface for isolation testing
type IsolationTrackingConnection struct {
	userID   string
	role     string
	messages [][]byte
	mu       sync.RWMutex
	closed   bool
}

func (itc *IsolationTrackingConnection) GetUserID() string {
	itc.mu.RLock()
	defer itc.mu.RUnlock()
	return itc.userID
}

func (itc *IsolationTrackingConnection) GetRole() string {
	itc.mu.RLock()
	defer itc.mu.RUnlock()
	return itc.role
}

func (itc *IsolationTrackingConnection) SendMessage(data []byte) error {
	itc.mu.Lock()
	defer itc.mu.Unlock()
	
	if itc.closed {
		return fmt.Errorf("connection closed for user %s", itc.userID)
	}
	
	// Create copy to prevent external modifications
	msgCopy := make([]byte, len(data))
	copy(msgCopy, data)
	itc.messages = append(itc.messages, msgCopy)
	
	return nil
}

func (itc *IsolationTrackingConnection) GetReceivedMessages() [][]byte {
	itc.mu.RLock()
	defer itc.mu.RUnlock()
	
	// Return copies to prevent external modifications
	result := make([][]byte, len(itc.messages))
	for i, msg := range itc.messages {
		result[i] = make([]byte, len(msg))
		copy(result[i], msg)
	}
	return result
}

func (itc *IsolationTrackingConnection) Close() error {
	itc.mu.Lock()
	defer itc.mu.Unlock()
	itc.closed = true
	return nil
}

func (itc *IsolationTrackingConnection) GetLastSeen() time.Time {
	return time.Now()
}

func (itc *IsolationTrackingConnection) WriteJSON(v interface{}) error {
	return nil // Not needed for this test
}

func (itc *IsolationTrackingConnection) SetCredentials(username, role string) error {
	return nil // Not needed for this test
}

func (itc *IsolationTrackingConnection) UpdateActivity() {
	// Not needed for this test
}

func (itc *IsolationTrackingConnection) SendCloseMessage(reason string) error {
	return nil // Not needed for this test
}

// generateTestExecutionReport creates a comprehensive test execution report
func generateTestExecutionReport(results map[string]bool, instructorMsgCount, studentMsgCount, privacyViolations, auditMsgCount int, processingTime time.Duration) string {
	report := `
==================================================
TEST 1.1: STUDENT MESSAGE ISOLATION VERIFICATION
P0-CRITICAL EDUCATIONAL PRIVACY COMPLIANCE TEST
==================================================

TEST EXECUTION SUMMARY:
📊 Instructor Messages Received: %d/5 (Target: 5)
📊 Student Messages Received: %d (Target: 0)
📊 Privacy Violations Detected: %d (Target: 0)
📊 Audit Messages Logged: %d/5 (Target: 5)
📊 Processing Latency: %v (Target: <100ms)

DETAILED RESULTS:
%s Instructor Complete Oversight: %s
%s Zero Cross-Student Visibility: %s
%s Audit Log Compliance: %s
%s Performance Requirements: %s
%s Privacy Violation Prevention: %s

COMPLIANCE STATUS:
Educational Privacy: %s
Production Readiness: %s
Security Validation: %s

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
		instructorMsgCount, studentMsgCount, privacyViolations, auditMsgCount, processingTime,
		getIcon(results["instructor_complete_oversight"]), getStatus(results["instructor_complete_oversight"]),
		getIcon(results["zero_cross_student_visibility"]), getStatus(results["zero_cross_student_visibility"]),
		getIcon(results["audit_log_compliance"]), getStatus(results["audit_log_compliance"]),
		getIcon(results["performance_requirements"]), getStatus(results["performance_requirements"]),
		getIcon(results["privacy_violations"]), getStatus(results["privacy_violations"]),
		getStatus(overallPass && privacyViolations == 0),
		getStatus(overallPass),
		getStatus(overallPass && privacyViolations == 0),
	)
}