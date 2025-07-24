package fixtures

import (
	"fmt"
	"math/rand"
	"time"
)

// ClassroomData represents a realistic classroom setup
type ClassroomData struct {
	InstructorIDs []string
	StudentIDs    []string
	SessionName   string
	SessionID     string
}

// TestMessage represents a message for testing with timing information
type TestMessage struct {
	Type      string
	Context   string
	FromUser  string
	ToUser    string
	Content   map[string]interface{}
	DelayMs   int // Realistic timing delay before sending
}

// MessagePattern represents a realistic communication pattern
type MessagePattern struct {
	Name        string
	Description string
	Messages    []*TestMessage
}

// GenerateClassroomScenario creates realistic classroom data with specified participant counts
func GenerateClassroomScenario(instructorCount, studentCount int) *ClassroomData {
	scenario := &ClassroomData{
		InstructorIDs: make([]string, instructorCount),
		StudentIDs:    make([]string, studentCount),
		SessionName:   GenerateSessionName(),
	}
	
	// Generate instructor IDs
	for i := 0; i < instructorCount; i++ {
		scenario.InstructorIDs[i] = fmt.Sprintf("instructor_%d", i+1)
	}
	
	// Generate student IDs
	for i := 0; i < studentCount; i++ {
		scenario.StudentIDs[i] = fmt.Sprintf("student_%d", i+1)
	}
	
	return scenario
}

// GenerateSessionName creates realistic session names
func GenerateSessionName() string {
	subjects := []string{"Math", "Science", "History", "English", "Computer Science", "Physics", "Chemistry", "Biology"}
	topics := []string{"Chapter 5", "Lab Session", "Review Session", "Quiz Prep", "Project Work", "Discussion", "Practice Problems"}
	
	subject := subjects[rand.Intn(len(subjects))]
	topic := topics[rand.Intn(len(topics))]
	
	return fmt.Sprintf("%s - %s", subject, topic)
}

// GenerateContextVariations returns realistic context values for each message type
func GenerateContextVariations() map[string][]string {
	return map[string][]string{
		"instructor_inbox":      {"question", "help_request", "clarification", "technical_issue"},
		"inbox_response":        {"answer", "guidance", "follow_up"},
		"request":              {"code", "execution_output", "explanation", "screenshot"},
		"request_response":     {"code_submission", "output_results", "explanation"},
		"analytics":            {"engagement", "progress", "performance", "errors"},
		"instructor_broadcast": {"announcement", "instruction", "emergency"},
	}
}

// GenerateQASessionFlow creates a realistic Q&A session message flow
func GenerateQASessionFlow(scenario *ClassroomData) *MessagePattern {
	messages := []*TestMessage{}
	
	// Instructor starts with announcement
	messages = append(messages, &TestMessage{
		Type:     "instructor_broadcast",
		Context:  "announcement",
		FromUser: scenario.InstructorIDs[0],
		ToUser:   "",
		Content: map[string]interface{}{
			"text": "Welcome to today's Q&A session. Please feel free to ask questions about the homework.",
		},
		DelayMs: 0,
	})
	
	// Students ask questions (80% participation rate)
	participatingStudents := int(float64(len(scenario.StudentIDs)) * 0.8)
	for i := 0; i < participatingStudents; i++ {
		studentID := scenario.StudentIDs[i]
		
		// Realistic student questions
		questions := []string{
			"I'm having trouble with problem 3. Could you explain the approach?",
			"What's the difference between method A and method B?", 
			"I got a different answer for question 5. Can you check my work?",
			"The example in the textbook doesn't match what we did in class.",
			"I'm confused about the formula we used yesterday.",
		}
		
		messages = append(messages, &TestMessage{
			Type:     "instructor_inbox",
			Context:  "question",
			FromUser: studentID,
			ToUser:   "",
			Content: map[string]interface{}{
				"text":    questions[rand.Intn(len(questions))],
				"problem": fmt.Sprintf("Problem %d", rand.Intn(10)+1),
			},
			DelayMs: 500 + rand.Intn(2000), // 0.5-2.5 seconds between questions
		})
		
		// Instructor responds to each question
		responses := []string{
			"Great question! Let me walk through that step by step.",
			"I can see where the confusion is. Here's the key concept:",
			"That's a common mistake. The correct approach is:",
			"Good observation! You're on the right track, but consider this:",
			"Let me clarify that point from yesterday's lecture.",
		}
		
		messages = append(messages, &TestMessage{
			Type:     "inbox_response", 
			Context:  "answer",
			FromUser: scenario.InstructorIDs[0],
			ToUser:   studentID,
			Content: map[string]interface{}{
				"text":        responses[rand.Intn(len(responses))],
				"explanation": "Detailed explanation would go here...",
				"references":  []string{"Textbook Chapter 3", "Lecture Notes"},
			},
			DelayMs: 1000 + rand.Intn(3000), // 1-4 seconds to respond
		})
	}
	
	return &MessagePattern{
		Name:        "Q&A Session Flow",
		Description: "Realistic classroom Q&A session with instructor announcements and student questions",
		Messages:    messages,
	}
}

// GenerateCodeReviewFlow creates code review session message flow
func GenerateCodeReviewFlow(scenario *ClassroomData) *MessagePattern {
	messages := []*TestMessage{}
	
	// Instructor requests code from students
	for i, studentID := range scenario.StudentIDs[:5] { // First 5 students
		messages = append(messages, &TestMessage{
			Type:     "request",
			Context:  "code",
			FromUser: scenario.InstructorIDs[0],
			ToUser:   studentID,
			Content: map[string]interface{}{
				"text":        "Please share your solution for the sorting algorithm assignment.",
				"requirements": []string{"include comments", "show test cases", "explain time complexity"},
				"assignment":   "Sorting Algorithm Implementation",
			},
			DelayMs: i * 2000, // Stagger requests
		})
		
		// Student submits code
		messages = append(messages, &TestMessage{
			Type:     "request_response",
			Context:  "code_submission",
			FromUser: studentID,
			ToUser:   "",
			Content: map[string]interface{}{
				"code": `
def bubble_sort(arr):
    # Simple bubble sort implementation
    n = len(arr)
    for i in range(n):
        for j in range(0, n-i-1):
            if arr[j] > arr[j+1]:
                arr[j], arr[j+1] = arr[j+1], arr[j]
    return arr
`,
				"test_cases":    []string{"[3,1,4,1,5]", "[9,2,6,5,3]"},
				"time_complexity": "O(n²)",
				"notes":         "Basic implementation, could be optimized",
			},
			DelayMs: 3000 + rand.Intn(5000), // 3-8 seconds to write response
		})
		
		// Instructor provides feedback
		messages = append(messages, &TestMessage{
			Type:     "inbox_response",
			Context:  "guidance",
			FromUser: scenario.InstructorIDs[0],
			ToUser:   studentID,
			Content: map[string]interface{}{
				"text":        "Good implementation! A few suggestions for improvement:",
				"feedback":    []string{"Consider early termination optimization", "Add input validation", "Include more edge cases"},
				"score":       "8/10",
				"next_steps":  "Try implementing quicksort next",
			},
			DelayMs: 2000 + rand.Intn(3000), // 2-5 seconds to review
		})
	}
	
	return &MessagePattern{
		Name:        "Code Review Flow",
		Description: "Code submission, review, and feedback cycle",
		Messages:    messages,
	}
}

// GenerateAnalyticsFlow creates student analytics reporting flow
func GenerateAnalyticsFlow(scenario *ClassroomData) *MessagePattern {
	messages := []*TestMessage{}
	
	// Students send various analytics throughout the session
	for i, studentID := range scenario.StudentIDs {
		// Engagement analytics
		messages = append(messages, &TestMessage{
			Type:     "analytics",
			Context:  "engagement",
			FromUser: studentID,
			ToUser:   "",
			Content: map[string]interface{}{
				"attention_level": rand.Intn(100),
				"participation":   rand.Intn(100),
				"confusion_level": rand.Intn(50),
				"timestamp":       time.Now().Unix(),
			},
			DelayMs: i * 500, // Stagger analytics
		})
		
		// Progress analytics (for some students)
		if rand.Float64() < 0.6 { // 60% of students report progress
			messages = append(messages, &TestMessage{
				Type:     "analytics",
				Context:  "progress",
				FromUser: studentID,
				ToUser:   "",
				Content: map[string]interface{}{
					"problems_completed": rand.Intn(10),
					"problems_attempted": rand.Intn(15),
					"time_spent_minutes": rand.Intn(60),
					"difficulty_rating":  rand.Intn(5) + 1,
				},
				DelayMs: 10000 + rand.Intn(20000), // Progress reports come later
			})
		}
		
		// Error analytics (for students having issues)
		if rand.Float64() < 0.3 { // 30% of students report errors
			messages = append(messages, &TestMessage{
				Type:     "analytics",
				Context:  "errors",
				FromUser: studentID,
				ToUser:   "",
				Content: map[string]interface{}{
					"error_type":    []string{"syntax", "logic", "runtime"}[rand.Intn(3)],
					"error_count":   rand.Intn(5) + 1,
					"stuck_duration": rand.Intn(600), // seconds
					"help_needed":   true,
				},
				DelayMs: 15000 + rand.Intn(10000), // Error reports come later in session
			})
		}
	}
	
	return &MessagePattern{
		Name:        "Analytics Flow",
		Description: "Student analytics reporting during active learning session",
		Messages:    messages,
	}
}

// GenerateEmergencyFlow creates emergency communication pattern
func GenerateEmergencyFlow(scenario *ClassroomData) *MessagePattern {
	messages := []*TestMessage{}
	
	// Emergency announcement
	messages = append(messages, &TestMessage{
		Type:     "instructor_broadcast",
		Context:  "emergency",
		FromUser: scenario.InstructorIDs[0],
		ToUser:   "",
		Content: map[string]interface{}{
			"text":     "IMPORTANT: Please save your work immediately. We need to evacuate the building.",
			"priority": "HIGH",
			"action":   "evacuate",
		},
		DelayMs: 0,
	})
	
	// Students respond with acknowledgment and status
	for i, studentID := range scenario.StudentIDs {
		messages = append(messages, &TestMessage{
			Type:     "instructor_inbox",
			Context:  "technical_issue",
			FromUser: studentID,
			ToUser:   "",
			Content: map[string]interface{}{
				"text":   "Work saved, ready to evacuate",
				"status": "ready",
			},
			DelayMs: 100 + i*50, // Quick responses, slight stagger
		})
	}
	
	return &MessagePattern{
		Name:        "Emergency Communication",
		Description: "Emergency announcement and student acknowledgment",
		Messages:    messages,
	}
}

// GenerateMultiContextFlow creates complex multi-context communication
func GenerateMultiContextFlow(scenario *ClassroomData) *MessagePattern {
	messages := []*TestMessage{}
	
	// Start with multiple concurrent conversation threads
	contexts := GenerateContextVariations()
	
	baseTime := 0
	
	// Create interleaved message patterns across all types and contexts
	for messageType, contextList := range contexts {
		for _, context := range contextList {
			if messageType == "instructor_inbox" || messageType == "request_response" || messageType == "analytics" {
				// Student to instructors
				fromUser := scenario.StudentIDs[rand.Intn(len(scenario.StudentIDs))]
				messages = append(messages, &TestMessage{
					Type:     messageType,
					Context:  context,
					FromUser: fromUser,
					ToUser:   "",
					Content:  generateContentForContext(messageType, context),
					DelayMs:  baseTime + rand.Intn(1000),
				})
			} else {
				// Instructor to students
				fromUser := scenario.InstructorIDs[0]
				var toUser string
				if messageType == "instructor_broadcast" {
					toUser = "" // Broadcast
				} else {
					toUser = scenario.StudentIDs[rand.Intn(len(scenario.StudentIDs))]
				}
				
				messages = append(messages, &TestMessage{
					Type:     messageType,
					Context:  context,
					FromUser: fromUser,
					ToUser:   toUser,
					Content:  generateContentForContext(messageType, context),
					DelayMs:  baseTime + rand.Intn(1000),
				})
			}
			baseTime += 500 // Space out messages
		}
	}
	
	return &MessagePattern{
		Name:        "Multi-Context Communication",
		Description: "All message types with all contexts happening simultaneously",
		Messages:    messages,
	}
}

// generateContentForContext creates realistic content based on message type and context
func generateContentForContext(messageType, context string) map[string]interface{} {
	contentMap := map[string]map[string]interface{}{
		"instructor_inbox": {
			"question":        map[string]interface{}{"text": "I have a question about the assignment", "urgency": "medium"},
			"help_request":    map[string]interface{}{"text": "I need help with this problem", "problem_id": "P3"},
			"clarification":   map[string]interface{}{"text": "Could you clarify the requirements?", "section": "Part B"},
			"technical_issue": map[string]interface{}{"text": "My code isn't compiling", "error": "syntax error"},
		},
		"inbox_response": {
			"answer":    map[string]interface{}{"text": "Here's the solution to your question", "detailed": true},
			"guidance":  map[string]interface{}{"text": "Try this approach instead", "suggestions": []string{"step 1", "step 2"}},
			"follow_up": map[string]interface{}{"text": "Does this help clarify things?", "check_understanding": true},
		},
		"request": {
			"code":             map[string]interface{}{"text": "Please share your solution", "assignment": "Lab 3"},
			"execution_output": map[string]interface{}{"text": "Run your code and share the output", "expected": "numbers 1-10"},
			"explanation":      map[string]interface{}{"text": "Explain your reasoning", "detail_level": "high"},
			"screenshot":       map[string]interface{}{"text": "Share a screenshot of your error", "format": "PNG"},
		},
		"request_response": {
			"code_submission": map[string]interface{}{"code": "def solution(): return 42", "language": "python"},
			"output_results":  map[string]interface{}{"output": "1,2,3,4,5", "execution_time": "0.001s"},
			"explanation":     map[string]interface{}{"reasoning": "I used this approach because...", "confidence": "high"},
		},
		"analytics": {
			"engagement":  map[string]interface{}{"level": 85, "duration": 1200, "interactions": 15},
			"progress":    map[string]interface{}{"completed": 7, "total": 10, "time_spent": 3600},
			"performance": map[string]interface{}{"score": 92, "accuracy": 0.95, "speed": "fast"},
			"errors":      map[string]interface{}{"count": 3, "types": []string{"syntax", "logic"}, "resolved": 2},
		},
		"instructor_broadcast": {
			"announcement": map[string]interface{}{"text": "Class will end 5 minutes early today", "importance": "medium"},
			"instruction":  map[string]interface{}{"text": "Please work on problems 5-8 now", "time_limit": "15 minutes"},
			"emergency":    map[string]interface{}{"text": "Please evacuate immediately", "priority": "critical"},
		},
	}
	
	if typeMap, exists := contentMap[messageType]; exists {
		if content, exists := typeMap[context]; exists {
			return content.(map[string]interface{})
		}
	}
	
	// Default content
	return map[string]interface{}{
		"text":    fmt.Sprintf("Default content for %s/%s", messageType, context),
		"context": context,
		"type":    messageType,
	}
}

// GetRealisticMessageTiming returns appropriate delays between messages for different scenarios
func GetRealisticMessageTiming(messageType string) time.Duration {
	timingMap := map[string]time.Duration{
		"instructor_inbox":      2 * time.Second,  // Students think before asking
		"inbox_response":        3 * time.Second,  // Instructor considers response
		"request":              1 * time.Second,   // Quick instructor requests
		"request_response":     5 * time.Second,   // Students need time to prepare response
		"analytics":            30 * time.Second,  // Analytics sent periodically  
		"instructor_broadcast": 10 * time.Second,  // Instructor paces announcements
	}
	
	if timing, exists := timingMap[messageType]; exists {
		return timing
	}
	
	return 2 * time.Second // Default timing
}