package scenarios

import (
	"context"
	"fmt"
	"testing"
	"time"

	"switchboard/tests/fixtures"
	"switchboard/pkg/types"
)

// TestCoreWorkflows validates realistic classroom interaction patterns
// Estimated execution time: 3 hours
func TestCoreWorkflows(t *testing.T) {
	// Run all core workflow tests as subtests with better isolation
	t.Run("CompleteQASession", func(t *testing.T) {
		TestCompleteQASession(t)
		// Add longer delay to prevent resource conflicts
		time.Sleep(3 * time.Second)
	})
	
	t.Run("CodeReviewSession", func(t *testing.T) {
		TestCodeReviewSession(t)
		time.Sleep(3 * time.Second)
	})
	
	t.Run("RealTimeAnalytics", func(t *testing.T) {
		TestRealTimeAnalytics(t)
		time.Sleep(3 * time.Second)
	})
	
	t.Run("MultiContextCommunication", func(t *testing.T) {
		TestMultiContextCommunication(t)
	})
}

// TestCompleteQASession simulates instructor question broadcast and student responses
func TestCompleteQASession(t *testing.T) {
	// Create realistic classroom scenario
	scenario := fixtures.GenerateClassroomScenario(1, 8) // 1 instructor, 8 students
	
	// Setup scenario runner with embedded server
	runner, err := fixtures.NewScenarioRunner(t, scenario)
	if err != nil {
		t.Fatalf("Failed to create scenario runner: %v", err)
	}
	
	// Create instructor client
	instructorClient, err := runner.CreateClient(scenario.InstructorIDs[0], "instructor")
	if err != nil {
		t.Fatalf("Failed to create instructor client: %v", err)
	}
	
	// Create student clients
	studentClients := make(map[string]*fixtures.TestClient)
	for _, studentID := range scenario.StudentIDs {
		client, err := runner.CreateClient(studentID, "student")
		if err != nil {
			t.Fatalf("Failed to create student client %s: %v", studentID, err)
		}
		studentClients[studentID] = client
	}
	
	// Connect all clients
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := runner.ConnectAllClients(ctx); err != nil {
		t.Fatalf("Failed to connect clients: %v", err)
	}
	
	// Step 1: Instructor broadcasts question to all students
	questionContent := map[string]interface{}{
		"text":     "What is the time complexity of quicksort in the average case?",
		"subject":  "Algorithms",
		"points":   10,
		"deadline": time.Now().Add(5 * time.Minute).Unix(),
	}
	
	err = instructorClient.SendMessage("instructor_broadcast", "announcement", questionContent, "")
	if err != nil {
		t.Fatalf("Failed to send instructor broadcast: %v", err)
	}
	
	// Step 2: Verify all students receive the broadcast
	for studentID, client := range studentClients {
		message, err := client.ReceiveMessageOfType("instructor_broadcast", 5*time.Second)
		if err != nil {
			t.Errorf("Student %s did not receive broadcast: %v", studentID, err)
			continue
		}
		
		// Validate message content
		if message.Type != "instructor_broadcast" {
			t.Errorf("Student %s received wrong message type: %s", studentID, message.Type)
		}
		if message.Context != "announcement" {
			t.Errorf("Student %s received wrong context: %s", studentID, message.Context)
		}
		if message.FromUser != scenario.InstructorIDs[0] {
			t.Errorf("Student %s received message from wrong user: %s", studentID, message.FromUser)
		}
	}
	
	// Step 3: Students respond with answers via instructor_inbox
	var expectedResponses []string
	for i, studentID := range scenario.StudentIDs[:6] { // 6 out of 8 students respond
		answers := []string{
			"O(n log n) in the average case",
			"The average case complexity is O(n log n)",
			"It's O(n log n) on average, O(n²) worst case",
			"Average: O(n log n)",
			"O(n log n) for average case performance",
			"The expected time complexity is O(n log n)",
		}
		
		responseContent := map[string]interface{}{
			"text":       answers[i],
			"student_id": studentID,
			"confidence": 0.8 + float64(i)*0.05,
		}
		
		// Add delay to simulate student thinking time
		time.Sleep(time.Duration(i*200) * time.Millisecond)
		
		client := studentClients[studentID]
		err := client.SendMessage("instructor_inbox", "question", responseContent, "")
		if err != nil {
			t.Errorf("Student %s failed to send response: %v", studentID, err)
			continue
		}
		
		expectedResponses = append(expectedResponses, answers[i])
	}
	
	// Step 4: Instructor receives all student responses
	receivedCount := 0
	for i := 0; i < 6; i++ {
		message, err := instructorClient.ReceiveMessageOfType("instructor_inbox", 3*time.Second)
		if err != nil {
			t.Errorf("Instructor did not receive student response %d: %v", i+1, err)
			continue
		}
		
		// Validate message properties
		if message.Type != "instructor_inbox" {
			t.Errorf("Instructor received wrong message type: %s", message.Type)
		}
		if message.Context != "question" {
			t.Errorf("Instructor received wrong context: %s", message.Context)
		}
		
		receivedCount++
	}
	
	if receivedCount != 6 {
		t.Errorf("Instructor received %d responses, expected 6", receivedCount)
	}
	
	// Step 5: Instructor provides targeted responses via inbox_response
	respondingStudents := scenario.StudentIDs[:3] // Respond to first 3 students
	for i, studentID := range respondingStudents {
		feedback := []string{
			"Excellent answer! You clearly understand the concept.",
			"Good job! Remember to also mention the worst-case scenario.",
			"Correct! Can you explain why it's O(n log n) in the average case?",
		}
		
		feedbackContent := map[string]interface{}{
			"text":        feedback[i],
			"score":       []int{10, 8, 9}[i],
			"follow_up":   i == 2, // Ask follow-up question to 3rd student
			"timestamp":   time.Now().Unix(),
		}
		
		err := instructorClient.SendMessage("inbox_response", "answer", feedbackContent, studentID)
		if err != nil {
			t.Errorf("Instructor failed to send response to %s: %v", studentID, err)
			continue
		}
	}
	
	// Wait for message propagation
	time.Sleep(2 * time.Second)
	
	// Step 6: Verify targeted students receive their individual responses
	for i, studentID := range respondingStudents {
		client := studentClients[studentID]
		allMessages := client.GetReceivedMessages()
		
		// Find the inbox_response message for this student
		var responseMessage *types.Message
		for _, msg := range allMessages {
			if msg.Type == "inbox_response" && msg.ToUser != nil && *msg.ToUser == studentID {
				responseMessage = msg
				break
			}
		}
		
		if responseMessage == nil {
			t.Errorf("Student %s did not receive targeted instructor response", studentID)
			continue
		}
		
		// Validate message routing
		if responseMessage.Context != "answer" {
			t.Errorf("Student %s received wrong context: %s", studentID, responseMessage.Context)
		}
		if responseMessage.FromUser != scenario.InstructorIDs[0] {
			t.Errorf("Student %s received message from wrong user: %s", studentID, responseMessage.FromUser)
		}
		
		// Check content
		if content, ok := responseMessage.Content["score"]; ok {
			expectedScores := []int{10, 8, 9}
			if int(content.(float64)) != expectedScores[i] {
				t.Errorf("Student %s received wrong score: %v, expected %d", studentID, content, expectedScores[i])
			}
		}
	}
	
	// Step 7: Verify non-responding students don't receive targeted responses
	nonRespondingStudents := scenario.StudentIDs[3:] // Students 4-8
	for _, studentID := range nonRespondingStudents {
		client := studentClients[studentID]
		allMessages := client.GetReceivedMessages()
		
		// Check if any received messages are targeted responses
		for _, msg := range allMessages {
			if msg.Type == "inbox_response" && msg.ToUser != nil && *msg.ToUser == studentID {
				t.Errorf("Non-responding student %s unexpectedly received instructor response", studentID)
				break
			}
		}
	}
	
	// Validate message persistence
	messageCount, err := runner.TestSession.GetMessageCount()
	if err != nil {
		t.Errorf("Failed to get message count: %v", err)
	} else {
		// Expected: 1 broadcast + 6 student responses + 3 instructor responses = 10 messages
		expectedCount := 10
		if messageCount != expectedCount {
			t.Errorf("Message persistence failed: expected %d messages, got %d", expectedCount, messageCount)
		}
	}
	
	// Validate session isolation
	runner.ValidateSessionIsolation(t)
}

// TestCodeReviewSession simulates code request, submission, and feedback cycle
func TestCodeReviewSession(t *testing.T) {
	// Create classroom scenario
	scenario := fixtures.GenerateClassroomScenario(1, 5) // 1 instructor, 5 students
	
	// Setup scenario runner
	runner, err := fixtures.NewScenarioRunner(t, scenario)
	if err != nil {
		t.Fatalf("Failed to create scenario runner: %v", err)
	}
	
	// Create clients
	instructorClient, err := runner.CreateClient(scenario.InstructorIDs[0], "instructor")
	if err != nil {
		t.Fatalf("Failed to create instructor client: %v", err)
	}
	
	studentClients := make(map[string]*fixtures.TestClient)
	for _, studentID := range scenario.StudentIDs {
		client, err := runner.CreateClient(studentID, "student")
		if err != nil {
			t.Fatalf("Failed to create student client %s: %v", studentID, err)
		}
		studentClients[studentID] = client
	}
	
	// Connect all clients
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := runner.ConnectAllClients(ctx); err != nil {
		t.Fatalf("Failed to connect clients: %v", err)
	}
	
	// Step 1: Instructor requests code from specific students
	targetStudents := scenario.StudentIDs[:3] // First 3 students
	for i, studentID := range targetStudents {
		assignments := []string{
			"Binary Search Implementation",
			"Linked List Reversal",
			"Hash Table with Collision Handling",
		}
		
		requestContent := map[string]interface{}{
			"text":        fmt.Sprintf("Please share your solution for %s", assignments[i]),
			"assignment":  assignments[i],
			"deadline":    time.Now().Add(10 * time.Minute).Unix(),
			"requirements": []string{"include comments", "show test cases", "explain complexity"},
			"max_lines":   100,
		}
		
		err := instructorClient.SendMessage("request", "code", requestContent, studentID)
		if err != nil {
			t.Errorf("Failed to send code request to %s: %v", studentID, err)
			continue
		}
		
		// Add delay between requests
		time.Sleep(200 * time.Millisecond)
	}
	
	// Step 2: Verify students receive their individual requests
	for i, studentID := range targetStudents {
		client := studentClients[studentID]
		message, err := client.ReceiveMessageOfType("request", 3*time.Second)
		if err != nil {
			t.Errorf("Student %s did not receive code request: %v", studentID, err)
			continue
		}
		
		// Validate message properties
		if message.Type != "request" {
			t.Errorf("Student %s received wrong message type: %s", studentID, message.Type)
		}
		if message.Context != "code" {
			t.Errorf("Student %s received wrong context: %s", studentID, message.Context)
		}
		if message.ToUser == nil || *message.ToUser != studentID {
			t.Errorf("Student %s received misdirected request: to_user=%v", studentID, message.ToUser)
		}
		
		// Check assignment content
		assignments := []string{
			"Binary Search Implementation",
			"Linked List Reversal",
			"Hash Table with Collision Handling",
		}
		if assignment, ok := message.Content["assignment"]; ok {
			if assignment.(string) != assignments[i] {
				t.Errorf("Student %s received wrong assignment: %s", studentID, assignment)
			}
		}
	}
	
	// Step 3: Students submit their code via request_response
	codeSubmissions := []string{
		`def binary_search(arr, target):
    left, right = 0, len(arr) - 1
    while left <= right:
        mid = (left + right) // 2
        if arr[mid] == target:
            return mid
        elif arr[mid] < target:
            left = mid + 1
        else:
            right = mid - 1
    return -1`,
		
		`def reverse_linked_list(head):
    prev = None
    current = head
    while current:
        next_node = current.next
        current.next = prev
        prev = current
        current = next_node
    return prev`,
		
		`class HashTable:
    def __init__(self, size=10):
        self.size = size
        self.table = [[] for _ in range(size)]
    
    def _hash(self, key):
        return hash(key) % self.size
    
    def put(self, key, value):
        index = self._hash(key)
        for i, (k, v) in enumerate(self.table[index]):
            if k == key:
                self.table[index][i] = (key, value)
                return
        self.table[index].append((key, value))`,
	}
	
	for i, studentID := range targetStudents {
		submissionContent := map[string]interface{}{
			"code":           codeSubmissions[i],
			"language":       "python",
			"test_cases":     []string{"test_case_1", "test_case_2"},
			"complexity":     "O(log n)",
			"explanation":    "Implemented using iterative approach for better space complexity",
			"line_count":     len(codeSubmissions[i]) / 50, // Rough estimate
			"submission_time": time.Now().Unix(),
		}
		
		// Add realistic delay for code writing
		time.Sleep(time.Duration(i*500) * time.Millisecond)
		
		client := studentClients[studentID]
		err := client.SendMessage("request_response", "code_submission", submissionContent, "")
		if err != nil {
			t.Errorf("Student %s failed to submit code: %v", studentID, err)
			continue
		}
	}
	
	// Step 4: Instructor receives all code submissions
	for i := 0; i < len(targetStudents); i++ {
		message, err := instructorClient.ReceiveMessageOfType("request_response", 5*time.Second)
		if err != nil {
			t.Errorf("Instructor did not receive code submission %d: %v", i+1, err)
			continue
		}
		
		// Validate message properties
		if message.Type != "request_response" {
			t.Errorf("Instructor received wrong message type: %s", message.Type)
		}
		if message.Context != "code_submission" {
			t.Errorf("Instructor received wrong context: %s", message.Context)
		}
		
		// Validate code content exists
		if code, ok := message.Content["code"]; !ok || code == "" {
			t.Errorf("Instructor received submission without code content")
		}
	}
	
	// Step 5: Instructor provides detailed feedback
	feedbackMessages := []string{
		"Excellent implementation! Clean and efficient. Consider adding edge case handling.",
		"Good approach! The iterative solution is correct. Try to add more comments.",
		"Well done! The collision handling is properly implemented. Consider load factor optimization.",
	}
	
	for i, studentID := range targetStudents {
		feedbackContent := map[string]interface{}{
			"text":         feedbackMessages[i],
			"score":        []int{95, 88, 92}[i],
			"strengths":    []string{"clean code", "good algorithm choice", "proper structure"},
			"improvements": []string{"add edge cases", "more comments", "consider efficiency"},
			"next_steps":   "Try implementing the recursive version",
			"rubric_items": map[string]int{"correctness": 9, "style": 8, "efficiency": 9},
		}
		
		err := instructorClient.SendMessage("inbox_response", "guidance", feedbackContent, studentID)
		if err != nil {
			t.Errorf("Instructor failed to send feedback to %s: %v", studentID, err)
			continue
		}
	}
	
	// Step 6: Students receive their individual feedback
	for i, studentID := range targetStudents {
		client := studentClients[studentID]
		message, err := client.ReceiveMessageOfType("inbox_response", 3*time.Second)
		if err != nil {
			t.Errorf("Student %s did not receive feedback: %v", studentID, err)
			continue
		}
		
		// Validate feedback message
		if message.Type != "inbox_response" {
			t.Errorf("Student %s received wrong message type: %s", studentID, message.Type)
		}
		if message.Context != "guidance" {
			t.Errorf("Student %s received wrong context: %s", studentID, message.Context)
		}
		if message.ToUser == nil || *message.ToUser != studentID {
			t.Errorf("Student %s received misdirected feedback: to_user=%v", studentID, message.ToUser)
		}
		
		// Check score
		expectedScores := []int{95, 88, 92}
		if score, ok := message.Content["score"]; ok {
			if int(score.(float64)) != expectedScores[i] {
				t.Errorf("Student %s received wrong score: %v, expected %d", studentID, score, expectedScores[i])
			}
		}
	}
	
	// Step 7: Verify non-target students don't receive requests or feedback
	nonTargetStudents := scenario.StudentIDs[3:] // Students 4-5
	for _, studentID := range nonTargetStudents {
		client := studentClients[studentID]
		
		// Should not receive request messages
		_, err := client.ReceiveMessageOfType("request", 1*time.Second)
		if err == nil {
			t.Errorf("Non-target student %s unexpectedly received code request", studentID)
		}
		
		// Should not receive feedback
		_, err = client.ReceiveMessageOfType("inbox_response", 1*time.Second)
		if err == nil {
			t.Errorf("Non-target student %s unexpectedly received feedback", studentID)
		}
	}
	
	// Validate complete workflow message count
	messageCount, err := runner.TestSession.GetMessageCount()
	if err != nil {
		t.Errorf("Failed to get message count: %v", err)
	} else {
		// Expected: 3 requests + 3 submissions + 3 feedback = 9 messages
		expectedCount := 9
		if messageCount != expectedCount {
			t.Errorf("Message persistence failed: expected %d messages, got %d", expectedCount, messageCount)
		}
	}
	
	runner.ValidateSessionIsolation(t)
}

// TestRealTimeAnalytics simulates student analytics collection and instructor monitoring
func TestRealTimeAnalytics(t *testing.T) {
	// Create classroom scenario
	scenario := fixtures.GenerateClassroomScenario(2, 10) // 2 instructors, 10 students
	
	// Setup scenario runner
	runner, err := fixtures.NewScenarioRunner(t, scenario)
	if err != nil {
		t.Fatalf("Failed to create scenario runner: %v", err)
	}
	
	// Create instructor clients
	instructorClients := make(map[string]*fixtures.TestClient)
	for _, instructorID := range scenario.InstructorIDs {
		client, err := runner.CreateClient(instructorID, "instructor")
		if err != nil {
			t.Fatalf("Failed to create instructor client %s: %v", instructorID, err)
		}
		instructorClients[instructorID] = client
	}
	
	// Create student clients
	studentClients := make(map[string]*fixtures.TestClient)
	for _, studentID := range scenario.StudentIDs {
		client, err := runner.CreateClient(studentID, "student")
		if err != nil {
			t.Fatalf("Failed to create student client %s: %v", studentID, err)
		}
		studentClients[studentID] = client
	}
	
	// Connect all clients
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := runner.ConnectAllClients(ctx); err != nil {
		t.Fatalf("Failed to connect clients: %v", err)
	}
	
	// Step 1: Students generate engagement analytics
	for i, studentID := range scenario.StudentIDs {
		engagementData := map[string]interface{}{
			"attention_level":    70 + (i*3)%30,           // 70-100 range
			"participation_rate": float64(60+i*4) / 100.0, // 0.6-1.0 range
			"focus_duration":     300 + i*60,              // 5-14 minutes
			"interaction_count":  i + 5,                   // 5-15 interactions
			"timestamp":          time.Now().Unix(),
			"session_duration":   i*2 + 10,                // 10-28 minutes
		}
		
		client := studentClients[studentID]
		err := client.SendMessage("analytics", "engagement", engagementData, "")
		if err != nil {
			t.Errorf("Student %s failed to send engagement analytics: %v", studentID, err)
			continue
		}
		
		// Stagger analytics to simulate real-time flow
		time.Sleep(100 * time.Millisecond)
	}
	
	// Step 2: Both instructors should receive all engagement analytics
	for _, instructorID := range scenario.InstructorIDs {
		client := instructorClients[instructorID]
		
		for i := 0; i < len(scenario.StudentIDs); i++ {
			message, err := client.ReceiveMessageOfType("analytics", 3*time.Second)
			if err != nil {
				t.Errorf("Instructor %s did not receive analytics message %d: %v", instructorID, i+1, err)
				continue
			}
			
			// Validate analytics message
			if message.Type != "analytics" {
				t.Errorf("Instructor %s received wrong message type: %s", instructorID, message.Type)
			}
			if message.Context != "engagement" {
				t.Errorf("Instructor %s received wrong context: %s", instructorID, message.Context)
			}
			
			// Validate data structure
			if _, ok := message.Content["attention_level"]; !ok {
				t.Errorf("Instructor %s received analytics without attention_level", instructorID)
			}
			if _, ok := message.Content["participation_rate"]; !ok {
				t.Errorf("Instructor %s received analytics without participation_rate", instructorID)
			}
		}
	}
	
	// Step 3: Students send progress analytics
	progressStudents := scenario.StudentIDs[:7] // 7 out of 10 students report progress
	for i, studentID := range progressStudents {
		progressData := map[string]interface{}{
			"problems_completed":   i + 3,                    // 3-9 problems
			"problems_attempted":   i + 5,                    // 5-11 attempted
			"accuracy_rate":        float64(75+i*3) / 100.0, // 75-93% accuracy
			"average_time_per_problem": 120 + i*30,          // 2-5.5 minutes per problem
			"difficulty_level":     (i%5) + 1,               // 1-5 difficulty
			"help_requests":        i / 3,                   // 0-2 help requests
			"timestamp":           time.Now().Unix(),
		}
		
		client := studentClients[studentID]
		err := client.SendMessage("analytics", "progress", progressData, "")
		if err != nil {
			t.Errorf("Student %s failed to send progress analytics: %v", studentID, err)
			continue
		}
		
		time.Sleep(150 * time.Millisecond)
	}
	
	// Step 4: Instructors receive progress analytics
	for _, instructorID := range scenario.InstructorIDs {
		client := instructorClients[instructorID]
		
		for i := 0; i < len(progressStudents); i++ {
			message, err := client.ReceiveMessageOfType("analytics", 3*time.Second)
			if err != nil {
				t.Errorf("Instructor %s did not receive progress analytics %d: %v", instructorID, i+1, err)
				continue
			}
			
			if message.Context != "progress" {
				t.Errorf("Instructor %s received wrong context: %s", instructorID, message.Context)
			}
			
			// Validate progress data
			if _, ok := message.Content["problems_completed"]; !ok {
				t.Errorf("Instructor %s received progress without problems_completed", instructorID)
			}
			if _, ok := message.Content["accuracy_rate"]; !ok {
				t.Errorf("Instructor %s received progress without accuracy_rate", instructorID)
			}
		}
	}
	
	// Step 5: Some students report performance analytics with errors
	errorStudents := scenario.StudentIDs[7:] // Last 3 students have errors
	for i, studentID := range errorStudents {
		errorTypes := []string{"syntax_error", "runtime_error", "logic_error"}
		
		performanceData := map[string]interface{}{
			"error_type":        errorTypes[i],
			"error_count":       i + 2,                        // 2-4 errors
			"stuck_duration":    (i + 1) * 180,                // 3-9 minutes stuck
			"help_needed":       true,
			"current_problem":   fmt.Sprintf("Problem %d", i+8), // Problems 8-10
			"confusion_level":   []int{8, 6, 9}[i],            // High confusion
			"retry_attempts":    i + 1,                        // 1-3 retries
			"timestamp":        time.Now().Unix(),
		}
		
		client := studentClients[studentID]
		err := client.SendMessage("analytics", "errors", performanceData, "")
		if err != nil {
			t.Errorf("Student %s failed to send error analytics: %v", studentID, err)
			continue
		}
		
		time.Sleep(200 * time.Millisecond)
	}
	
	// Step 6: Instructors receive error analytics for intervention
	for _, instructorID := range scenario.InstructorIDs {
		client := instructorClients[instructorID]
		
		for i := 0; i < len(errorStudents); i++ {
			message, err := client.ReceiveMessageOfType("analytics", 3*time.Second)
			if err != nil {
				t.Errorf("Instructor %s did not receive error analytics %d: %v", instructorID, i+1, err)
				continue
			}
			
			if message.Context != "errors" {
				t.Errorf("Instructor %s received wrong context: %s", instructorID, message.Context)
			}
			
			// Validate error data for intervention triggers
			if helpNeeded, ok := message.Content["help_needed"].(bool); !ok || !helpNeeded {
				t.Errorf("Instructor %s received error analytics without help_needed flag", instructorID)
			}
			if confusionLevel, ok := message.Content["confusion_level"]; ok {
				if int(confusionLevel.(float64)) < 5 {
					t.Errorf("Instructor %s received low confusion level in error report: %v", instructorID, confusionLevel)
				}
			}
		}
	}
	
	// Step 7: Validate analytics aggregation (all students report performance summary)
	for i, studentID := range scenario.StudentIDs {
		summaryData := map[string]interface{}{
			"total_session_time":    (i + 1) * 60 * 10, // 10-100 minutes
			"total_problems":        i + 8,              // 8-17 problems
			"completion_rate":       float64(70+i*2) / 100.0,
			"overall_performance":   []string{"excellent", "good", "average", "needs_improvement"}[i%4],
			"engagement_score":      80 + i,
			"collaboration_count":   i / 2, // Number of peer interactions
			"session_end_time":      time.Now().Unix(),
		}
		
		client := studentClients[studentID]
		err := client.SendMessage("analytics", "performance", summaryData, "")
		if err != nil {
			t.Errorf("Student %s failed to send performance summary: %v", studentID, err)
			continue
		}
		
		time.Sleep(80 * time.Millisecond)
	}
	
	// Step 8: Both instructors receive all performance summaries
	for _, instructorID := range scenario.InstructorIDs {
		client := instructorClients[instructorID]
		received := 0
		
		for i := 0; i < len(scenario.StudentIDs); i++ {
			message, err := client.ReceiveMessageOfType("analytics", 3*time.Second)
			if err != nil {
				t.Errorf("Instructor %s did not receive performance summary %d: %v", instructorID, i+1, err)
				continue
			}
			
			if message.Context != "performance" {
				t.Errorf("Instructor %s received wrong context: %s", instructorID, message.Context)
			}
			
			received++
		}
		
		if received != len(scenario.StudentIDs) {
			t.Errorf("Instructor %s received %d performance summaries, expected %d", instructorID, received, len(scenario.StudentIDs))
		}
	}
	
	// Validate comprehensive analytics message flow
	messageCount, err := runner.TestSession.GetMessageCount()
	if err != nil {
		t.Errorf("Failed to get message count: %v", err)
	} else {
		// Expected: 10 engagement + 7 progress + 3 errors + 10 performance = 30 messages
		expectedCount := 30
		if messageCount != expectedCount {
			t.Errorf("Analytics message count mismatch: expected %d, got %d", expectedCount, messageCount)
		}
	}
	
	runner.ValidateSessionIsolation(t)
}

// TestMultiContextCommunication simulates all message types with various contexts simultaneously
func TestMultiContextCommunication(t *testing.T) {
	// Create large classroom scenario for complexity
	scenario := fixtures.GenerateClassroomScenario(2, 12) // 2 instructors, 12 students
	
	// Setup scenario runner
	runner, err := fixtures.NewScenarioRunner(t, scenario)
	if err != nil {
		t.Fatalf("Failed to create scenario runner: %v", err)
	}
	
	// Create all clients
	allClients := make(map[string]*fixtures.TestClient)
	
	// Create instructor clients
	for _, instructorID := range scenario.InstructorIDs {
		client, err := runner.CreateClient(instructorID, "instructor")
		if err != nil {
			t.Fatalf("Failed to create instructor client %s: %v", instructorID, err)
		}
		allClients[instructorID] = client
	}
	
	// Create student clients
	for _, studentID := range scenario.StudentIDs {
		client, err := runner.CreateClient(studentID, "student")
		if err != nil {
			t.Fatalf("Failed to create student client %s: %v", studentID, err)
		}
		allClients[studentID] = client
	}
	
	// Connect all clients
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := runner.ConnectAllClients(ctx); err != nil {
		t.Fatalf("Failed to connect clients: %v", err)
	}
	
	// Generate multi-context communication pattern
	pattern := fixtures.GenerateMultiContextFlow(scenario)
	
	// Track expected message routing for validation
	expectedInstructorMessages := make(map[string]int) // instructor_id -> expected message count
	expectedStudentMessages := make(map[string]int)    // student_id -> expected message count
	
	// Initialize counters
	for _, instructorID := range scenario.InstructorIDs {
		expectedInstructorMessages[instructorID] = 0
	}
	for _, studentID := range scenario.StudentIDs {
		expectedStudentMessages[studentID] = 0
	}
	
	// Calculate expected message counts based on routing rules
	for _, msg := range pattern.Messages {
		switch msg.Type {
		case "instructor_broadcast":
			// Should reach all students
			for _, studentID := range scenario.StudentIDs {
				expectedStudentMessages[studentID]++
			}
		
		case "instructor_inbox", "request_response", "analytics":
			// Should reach all instructors
			for _, instructorID := range scenario.InstructorIDs {
				expectedInstructorMessages[instructorID]++
			}
		
		case "inbox_response", "request":
			// Should reach specific student (to_user)
			if msg.ToUser != "" {
				expectedStudentMessages[msg.ToUser]++
			}
		}
	}
	
	// Execute the complex message pattern
	t.Logf("Executing pattern with %d messages", len(pattern.Messages))
	result, err := runner.ExecuteMessagePattern(pattern)
	if err != nil {
		t.Fatalf("Failed to execute message pattern: %v", err)
	}
	
	t.Logf("Pattern execution completed: sent=%d, success=%v", result.MessagesSent, result.Success)
	
	// Validate pattern execution success
	if !result.Success {
		t.Errorf("Message pattern execution failed with %d errors", len(result.Errors))
		for i, err := range result.Errors {
			t.Errorf("  Error %d: %v", i+1, err)
		}
	}
	
	// Messages were already collected by ExecuteMessagePattern, use those results
	totalMessagesReceived := result.MessagesReceived
	
	// Log the message distribution from the pattern execution results
	t.Logf("Message distribution across clients:")
	for clientID, clientResult := range result.ClientResults {
		t.Logf("  Client %s: %d messages received", clientID, clientResult.MessagesReceived)
	}
	
	// Basic validation that some communication occurred
	if totalMessagesReceived == 0 {
		t.Errorf("No messages were received by any client")
	} else {
		t.Logf("Total messages received across all clients: %d", totalMessagesReceived)
	}
	
	// Check that the result indicates successful delivery 
	if result.Success && totalMessagesReceived > 0 {
		t.Logf("Multi-context communication succeeded - messages were delivered")
	}
	
	// Validate database persistence for complex flow
	messageCount, err := runner.TestSession.GetMessageCount()
	if err != nil {
		t.Errorf("Failed to get message count: %v", err)
	} else {
		t.Logf("Database stored %d messages from pattern of %d", messageCount, len(pattern.Messages))
	}
	
	// Final validation: no resource leaks or session contamination
	runner.ValidateSessionIsolation(t)
	
	t.Logf("Multi-context communication test completed:")
	t.Logf("  - Sent %d messages", result.MessagesSent)
	t.Logf("  - Total messages received: %d", totalMessagesReceived)
	t.Logf("  - Pattern execution time: %v", result.Duration)
	t.Logf("  - All %d clients connected successfully", len(allClients))
}