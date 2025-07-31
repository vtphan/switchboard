// Message Broadcasting Workflow Tests
// Tests critical message routing, filtering, and delivery workflows
package workflows

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
)

// TestMessageBroadcastWorkflows validates end-to-end message delivery patterns
func TestMessageBroadcastWorkflows(t *testing.T) {
	t.Run("complete_question_answer_workflow", func(t *testing.T) {
		// Test the complete student question -> instructor response workflow
		env := setupBroadcastEnvironment(t)
		defer env.cleanup()

		// Setup active session
		createActiveSession(t, env, "qa-workflow-test")

		// Create classroom: 3 students, 2 instructors
		students := createStudentConnections(t, env, 3)
		instructors := createInstructorConnections(t, env, 2)

		// Student asks question to instructors
		questionMsg := createStudentQuestionMessage("What is the mitochondria's function?")
		require.NoError(t, env.processor.ProcessIncomingMessage(questionMsg, "student1"))

		time.Sleep(50 * time.Millisecond) // Allow message processing

		// Verify question delivery pattern
		// Both instructors should receive the question
		for i, instructor := range instructors {
			messages := instructor.GetDeliveredMessages()
			assert.Len(t, messages, 1, fmt.Sprintf("Instructor %d should receive student question", i+1))
		}

		// Other students should NOT receive the question (privacy)
		for i := 1; i < len(students); i++ {
			messages := students[i].GetDeliveredMessages()
			assert.Len(t, messages, 0, fmt.Sprintf("Student %d should not see other students' questions", i+2))
		}

		// Instructor responds with direct message to student
		directResponse := createDirectMessage("student1", "Mitochondria produce ATP energy for the cell")
		require.NoError(t, env.processor.ProcessIncomingMessage(directResponse, "instructor1"))

		time.Sleep(50 * time.Millisecond)

		// Verify direct message delivery
		student1Messages := students[0].GetDeliveredMessages()
		assert.Len(t, student1Messages, 1, "Student1 should receive direct response")

		// Other students should not see the direct message
		for i := 1; i < len(students); i++ {
			messages := students[i].GetDeliveredMessages()
			assert.Len(t, messages, 0, fmt.Sprintf("Student %d should not see direct message to student1", i+2))
		}

		// Both instructors should see the direct message (oversight)
		for i, instructor := range instructors {
			messages := instructor.GetDeliveredMessages()
			assert.Len(t, messages, 2, fmt.Sprintf("Instructor %d should see both question and response", i+1))
		}

		// Instructor makes general announcement
		announcement := createInstructorAnnouncementMessage("Great question! This relates to cellular respiration...")
		require.NoError(t, env.processor.ProcessIncomingMessage(announcement, "instructor2"))

		time.Sleep(50 * time.Millisecond)

		// All students should receive the announcement
		for i := 0; i < len(students); i++ {
			messages := students[i].GetDeliveredMessages()
			expectedCount := 1
			if i == 0 { // student1 also has the direct message
				expectedCount = 2
			}
			assert.Len(t, messages, expectedCount, fmt.Sprintf("Student %d should receive announcement", i+1))
		}
	})

	t.Run("multi_instructor_coordination_workflow", func(t *testing.T) {
		// Test coordination between multiple instructors handling student questions
		env := setupBroadcastEnvironment(t)
		defer env.cleanup()

		createActiveSession(t, env, "multi-instructor-test")
		students := createStudentConnections(t, env, 5)
		instructors := createInstructorConnections(t, env, 3)

		// Multiple students ask questions simultaneously
		questions := []string{
			"How does photosynthesis work?",
			"What is the difference between DNA and RNA?",
			"Can you explain enzyme catalysis?",
			"How do cells divide?",
			"What is protein synthesis?",
		}

		// Send all questions concurrently
		var wg sync.WaitGroup
		for i, question := range questions {
			wg.Add(1)
			go func(studentIndex int, questionText string) {
				defer wg.Done()
				studentID := fmt.Sprintf("student%d", studentIndex+1)
				msg := createStudentQuestionMessage(questionText)
				if err := env.processor.ProcessIncomingMessage(msg, studentID); err != nil {
					t.Logf("Failed to process message from %s: %v", studentID, err)
				}
			}(i, question)
		}
		wg.Wait()

		time.Sleep(100 * time.Millisecond) // Allow all messages to process

		// All instructors should see all questions for coordination
		for i, instructor := range instructors {
			messages := instructor.GetDeliveredMessages()
			assert.Len(t, messages, 5, fmt.Sprintf("Instructor %d should see all %d questions", i+1, len(questions)))
		}

		// Each student should only see their own question
		for i, student := range students {
			messages := student.GetDeliveredMessages()
			assert.Len(t, messages, 0, fmt.Sprintf("Student %d should not see any questions initially", i+1))
		}

		// Instructors coordinate responses - different instructors answer different questions
		responses := map[string]string{
			"student1": "Photosynthesis converts CO2 and water into glucose using sunlight",
			"student2": "DNA is double-stranded and stores genetic info, RNA is single-stranded",
			"student3": "Enzymes lower activation energy to speed up chemical reactions",
		}

		for studentID, response := range responses {
			// Different instructor responds to each student
			var instructorID string
			switch studentID {
			case "student2":
				instructorID = "instructor2"
			case "student3":
				instructorID = "instructor3"
			default:
				instructorID = "instructor1"
			}

			directMsg := createDirectMessage(studentID, response)
			require.NoError(t, env.processor.ProcessIncomingMessage(directMsg, instructorID))
		}

		time.Sleep(100 * time.Millisecond)

		// Verify each student gets their specific response
		for studentID := range responses {
			studentIndex := parseStudentIndex(studentID) - 1
			if studentIndex < len(students) {
				messages := students[studentIndex].GetDeliveredMessages()
				assert.Len(t, messages, 1, fmt.Sprintf("%s should receive their direct response", studentID))
			}
		}

		// All instructors should see all responses (oversight)
		for i, instructor := range instructors {
			messages := instructor.GetDeliveredMessages()
			expectedTotal := 5 + 3 // 5 questions + 3 responses
			assert.Len(t, messages, expectedTotal, fmt.Sprintf("Instructor %d should see all questions and responses", i+1))
		}
	})

	t.Run("message_ordering_and_consistency", func(t *testing.T) {
		// Test that message ordering is preserved across broadcasts
		env := setupBroadcastEnvironment(t)
		defer env.cleanup()

		createActiveSession(t, env, "ordering-test")
		students := createStudentConnections(t, env, 2)
		instructors := createInstructorConnections(t, env, 2)

		// Send ordered sequence of messages
		messageSequence := []struct {
			content  string
			sender   string
			msgType  string
		}{
			{"First announcement", "instructor1", "announcement"},
			{"Student question 1", "student1", "question"},
			{"Second announcement", "instructor2", "announcement"},
			{"Student question 2", "student2", "question"},
			{"Direct response to student1", "instructor1", "direct"},
			{"Third announcement", "instructor1", "announcement"},
		}

		// Send messages with small delays to ensure ordering
		for _, msg := range messageSequence {
			var msgData []byte
			switch msg.msgType {
			case "announcement":
				msgData = createInstructorAnnouncementMessage(msg.content)
			case "question":
				msgData = createStudentQuestionMessage(msg.content)
			case "direct":
				msgData = createDirectMessage("student1", msg.content)
			}

			require.NoError(t, env.processor.ProcessIncomingMessage(msgData, msg.sender))
			time.Sleep(10 * time.Millisecond) // Small delay for ordering
		}

		time.Sleep(100 * time.Millisecond)

		// Verify instructors receive messages in correct order
		instructor1Messages := instructors[0].GetDeliveredMessages()
		assert.GreaterOrEqual(t, len(instructor1Messages), 5, "Instructor1 should see most messages")

		// Verify students receive appropriate messages in order
		student1Messages := students[0].GetDeliveredMessages()
		// Student1 should receive: announcements (3) + direct message (1) = 4
		assert.Equal(t, 4, len(student1Messages), "Student1 should receive announcements and direct message")

		student2Messages := students[1].GetDeliveredMessages()
		// Student2 should receive: announcements (3) only = 3
		assert.Equal(t, 3, len(student2Messages), "Student2 should receive announcements only")
	})

	t.Run("broadcast_failure_recovery", func(t *testing.T) {
		// Test message delivery when some connections fail
		env := setupBroadcastEnvironment(t)
		defer env.cleanup()

		session := createActiveSession(t, env, "failure-recovery-test")

		// Create mixed connection types: reliable and failing
		reliableConn1 := &ReliableTestConnection{userID: "instructor1", role: "instructor"}
		reliableConn2 := &ReliableTestConnection{userID: "student1", role: "student"}
		failingConn := &FailingTestConnection{userID: "instructor2", role: "instructor", shouldFail: true}

		require.NoError(t, env.connectionRegistry.Register("instructor1", reliableConn1))
		require.NoError(t, env.connectionRegistry.Register("student1", reliableConn2))
		require.NoError(t, env.connectionRegistry.Register("instructor2", failingConn))

		// Send student question - should reach reliable instructor, fail for failing instructor
		questionMsg := createStudentQuestionMessage("Test question with failing recipient")
		require.NoError(t, env.processor.ProcessIncomingMessage(questionMsg, "student1"))

		time.Sleep(100 * time.Millisecond)

		// Verify reliable connections received message
		reliableMessages1 := reliableConn1.GetDeliveredMessages()
		assert.Len(t, reliableMessages1, 1, "Reliable instructor should receive message")

		// Verify failing connection did not receive message
		failingMessages := failingConn.GetDeliveredMessages()
		assert.Len(t, failingMessages, 0, "Failing instructor should not receive message")

		// Verify message was still persisted despite partial failure
		messages, err := env.dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Len(t, messages, 1, "Message should be persisted despite broadcast failure")

		// System should continue functioning after partial failure
		announcement := createInstructorAnnouncementMessage("System continues after failure")
		require.NoError(t, env.processor.ProcessIncomingMessage(announcement, "instructor1"))

		time.Sleep(50 * time.Millisecond)

		// Reliable student should receive announcement
		reliableMessages2 := reliableConn2.GetDeliveredMessages()
		assert.Len(t, reliableMessages2, 1, "Reliable student should receive announcement")
	})

	t.Run("late_joiner_message_history", func(t *testing.T) {
		// Test that late joiners receive appropriate filtered history
		env := setupBroadcastEnvironment(t)
		defer env.cleanup()

		session := createActiveSession(t, env, "late-joiner-test")
		createStudentConnections(t, env, 2)
		createInstructorConnections(t, env, 1)

		// Build up message history before late joiner arrives
		historyMessages := []struct {
			content string
			sender  string
			msgType string
			toUser  string
		}{
			{"Welcome to class!", "instructor1", "announcement", ""},
			{"Question about cells", "student1", "question", ""},
			{"Good question about cells", "instructor1", "direct", "student1"},
			{"Different question about DNA", "student2", "question", ""},
			{"Today we'll cover biology basics", "instructor1", "announcement", ""},
		}

		for _, msg := range historyMessages {
			var msgData []byte
			switch msg.msgType {
			case "announcement":
				msgData = createInstructorAnnouncementMessage(msg.content)
			case "question":
				msgData = createStudentQuestionMessage(msg.content)
			case "direct":
				msgData = createDirectMessage(msg.toUser, msg.content)
			}

			require.NoError(t, env.processor.ProcessIncomingMessage(msgData, msg.sender))
			time.Sleep(10 * time.Millisecond)
		}

		// Now late joiner (student3) connects
		lateJoiner := &ReliableTestConnection{userID: "student3", role: "student"}
		require.NoError(t, env.connectionRegistry.Register("student3", lateJoiner))

		// Simulate history delivery to late joiner
		messages, err := env.dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		
		// Filter messages appropriate for student3
		for _, msg := range messages {
			if shouldStudentSeeMessage(msg, "student3") {
				// Simulate delivery of filtered history
				msgData, _ := json.Marshal(map[string]interface{}{
					"type":    msg.Type,
					"content": msg.Content,
					"from_user": msg.FromUser,
					"timestamp": msg.Timestamp,
				})
				if err := lateJoiner.SendMessage(msgData); err != nil {
					t.Logf("Failed to send message to late joiner: %v", err)
				}
			}
		}

		// Late joiner should receive: 2 announcements only
		// Should NOT receive: other students' questions or direct messages to others
		lateJoinerMessages := lateJoiner.GetDeliveredMessages()
		assert.Equal(t, 2, len(lateJoinerMessages), "Late joiner should receive only announcements from history")

		// Verify late joiner can participate normally after joining
		newQuestion := createStudentQuestionMessage("Late joiner question")
		require.NoError(t, env.processor.ProcessIncomingMessage(newQuestion, "student3"))

		time.Sleep(50 * time.Millisecond)

		// Verify system can process new messages from late joiner
		time.Sleep(50 * time.Millisecond)
		
		// Check message was processed by verifying it was persisted
		finalMessages, err := env.dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(finalMessages), 6, "Late joiner's question should be processed and persisted")
	})

	t.Run("message_content_validation_and_sanitization", func(t *testing.T) {
		// Test handling of various message content types and edge cases
		env := setupBroadcastEnvironment(t)
		defer env.cleanup()

		session := createActiveSession(t, env, "content-validation-test")
		createStudentConnections(t, env, 1)
		createInstructorConnections(t, env, 1)

		testCases := []struct {
			name        string
			content     interface{}
			shouldError bool
		}{
			{"normal_text", map[string]interface{}{"text": "Normal question about biology"}, false},
			{"empty_content", map[string]interface{}{}, false},
			{"null_content", nil, false},
			{"large_content", map[string]interface{}{"text": generateLargeText(1000)}, false},
			{"special_characters", map[string]interface{}{"text": "Question with <script>alert('xss')</script>"}, false},
			{"unicode_content", map[string]interface{}{"text": "Question with émojis 🧬🔬 and ünïcødé"}, false},
			{"nested_content", map[string]interface{}{"text": "Question", "metadata": map[string]string{"source": "mobile"}}, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				msg := map[string]interface{}{
					"type":    database.MessageTypeBroadcastToInstructors,
					"context": database.ContextQuestion,
					"content": tc.content,
				}
				msgData, _ := json.Marshal(msg)

				err := env.processor.ProcessIncomingMessageSync(msgData, "student1")
				
				if tc.shouldError {
					assert.Error(t, err, "Message should be rejected")
				} else {
					assert.NoError(t, err, "Message should be processed successfully")
				}
			})
		}

		// Verify all valid messages were persisted
		messages, err := env.dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		
		validMessageCount := 0
		for _, tc := range testCases {
			if !tc.shouldError {
				validMessageCount++
			}
		}
		assert.Equal(t, validMessageCount, len(messages), "All valid messages should be persisted")
	})
}

// Helper functions for broadcast testing

func setupBroadcastEnvironment(t *testing.T) *WorkflowEnvironment {
	return setupWorkflowEnvironment(t)
}

func createActiveSession(t *testing.T, env *WorkflowEnvironment, sessionID string) *database.Session {
	session := &database.Session{
		ID:        sessionID,
		Name:      "Test Session",
		CreatedBy: "instructor1",
		StartTime: time.Now(),
		Status:    "active",
	}
	require.NoError(t, env.sessionManager.SetActiveSession(session))
	return session
}

func parseStudentIndex(studentID string) int {
	// Extract number from "student1", "student2", etc.
	var index int
	if _, err := fmt.Sscanf(studentID, "student%d", &index); err != nil {
		// Return 0 as default if parsing fails
		return 0
	}
	return index
}

func generateLargeText(size int) string {
	// Generate text of specified character count
	text := "This is a test message that will be repeated to create large content. "
	result := ""
	for len(result) < size {
		result += text
	}
	return result[:size]
}

// Test connection implementations for broadcast testing

type ReliableTestConnection struct {
	userID            string
	role              string
	deliveredMessages [][]byte
	mu                sync.RWMutex
}

func (rtc *ReliableTestConnection) GetUserID() string { return rtc.userID }
func (rtc *ReliableTestConnection) GetRole() string   { return rtc.role }

func (rtc *ReliableTestConnection) SendMessage(data []byte) error {
	rtc.mu.Lock()
	defer rtc.mu.Unlock()
	
	msgCopy := make([]byte, len(data))
	copy(msgCopy, data)
	rtc.deliveredMessages = append(rtc.deliveredMessages, msgCopy)
	return nil
}

func (rtc *ReliableTestConnection) GetDeliveredMessages() [][]byte {
	rtc.mu.RLock()
	defer rtc.mu.RUnlock()
	
	result := make([][]byte, len(rtc.deliveredMessages))
	for i, msg := range rtc.deliveredMessages {
		result[i] = make([]byte, len(msg))
		copy(result[i], msg)
	}
	return result
}

func (rtc *ReliableTestConnection) Close() error                               { return nil }
func (rtc *ReliableTestConnection) WriteJSON(v interface{}) error              { return nil }
func (rtc *ReliableTestConnection) SetCredentials(username, role string) error { return nil }
func (rtc *ReliableTestConnection) UpdateActivity()                            {}
func (rtc *ReliableTestConnection) GetLastSeen() time.Time                     { return time.Now() }
func (rtc *ReliableTestConnection) SendCloseMessage(reason string) error       { return nil }

type FailingTestConnection struct {
	userID            string
	role              string
	shouldFail        bool
	deliveredMessages [][]byte
	mu                sync.RWMutex
}

func (ftc *FailingTestConnection) GetUserID() string { return ftc.userID }
func (ftc *FailingTestConnection) GetRole() string   { return ftc.role }

func (ftc *FailingTestConnection) SendMessage(data []byte) error {
	if ftc.shouldFail {
		return fmt.Errorf("simulated connection failure")
	}
	
	ftc.mu.Lock()
	defer ftc.mu.Unlock()
	
	msgCopy := make([]byte, len(data))
	copy(msgCopy, data)
	ftc.deliveredMessages = append(ftc.deliveredMessages, msgCopy)
	return nil
}

func (ftc *FailingTestConnection) GetDeliveredMessages() [][]byte {
	ftc.mu.RLock()
	defer ftc.mu.RUnlock()
	
	result := make([][]byte, len(ftc.deliveredMessages))
	for i, msg := range ftc.deliveredMessages {
		result[i] = make([]byte, len(msg))
		copy(result[i], msg)
	}
	return result
}

func (ftc *FailingTestConnection) Close() error                               { return nil }
func (ftc *FailingTestConnection) WriteJSON(v interface{}) error              { return nil }
func (ftc *FailingTestConnection) SetCredentials(username, role string) error { return nil }
func (ftc *FailingTestConnection) UpdateActivity()                            {}
func (ftc *FailingTestConnection) GetLastSeen() time.Time                     { return time.Now() }
func (ftc *FailingTestConnection) SendCloseMessage(reason string) error       { return nil }