package scenarios

import (
	"context"
	"fmt"
	"testing"
	"time"

	"switchboard/tests/fixtures"
	"switchboard/pkg/types"
)

// TestFoundation validates basic system functionality and message type infrastructure
// Estimated execution time: 2 hours
func TestFoundation(t *testing.T) {
	t.Run("DatabaseIntegration", TestDatabaseIntegration)
	t.Run("BasicMessageTypes", TestBasicMessageTypes) 
	t.Run("SingleConnectionScenarios", TestSingleConnectionScenarios)
	t.Run("RoleBasedPermissions", TestRoleBasedPermissions)
	t.Run("ContextFieldHandling", TestContextFieldHandling)
}

// TestDatabaseIntegration validates clean database operations and schema
func TestDatabaseIntegration(t *testing.T) {
	// Create simple classroom scenario
	scenario := fixtures.GenerateClassroomScenario(1, 2)
	
	// Create test session with database cleanup
	testSession := fixtures.SetupCleanSession(t, scenario.SessionName, scenario.InstructorIDs[0], scenario.StudentIDs)
	
	// Test session creation
	if testSession.Session == nil {
		t.Fatal("Failed to create test session")
	}
	
	if testSession.Session.Name != scenario.SessionName {
		t.Errorf("Session name mismatch: expected %s, got %s", scenario.SessionName, testSession.Session.Name)
	}
	
	if testSession.Session.CreatedBy != scenario.InstructorIDs[0] {
		t.Errorf("Session creator mismatch: expected %s, got %s", scenario.InstructorIDs[0], testSession.Session.CreatedBy)
	}
	
	if len(testSession.Session.StudentIDs) != len(scenario.StudentIDs) {
		t.Errorf("Student count mismatch: expected %d, got %d", len(scenario.StudentIDs), len(testSession.Session.StudentIDs))
	}
	
	// Test message persistence
	ctx := context.Background()
	testMessage := &types.Message{
		SessionID: testSession.SessionID,
		Type:      "instructor_broadcast",
		Context:   "announcement",
		FromUser:  scenario.InstructorIDs[0],
		Content: map[string]interface{}{
			"text": "Test message for database persistence",
		},
	}
	
	err := testSession.DbManager.StoreMessage(ctx, testMessage)
	if err != nil {
		t.Fatalf("Failed to store message: %v", err)
	}
	
	// Retrieve message history
	messages, err := testSession.DbManager.GetSessionHistory(ctx, testSession.SessionID)
	if err != nil {
		t.Fatalf("Failed to retrieve session history: %v", err)
	}
	
	if len(messages) != 1 {
		t.Errorf("Expected 1 message in history, got %d", len(messages))
	}
	
	if len(messages) > 0 {
		retrieved := messages[0]
		if retrieved.Type != testMessage.Type {
			t.Errorf("Message type mismatch: expected %s, got %s", testMessage.Type, retrieved.Type)
		}
		if retrieved.Context != testMessage.Context {
			t.Errorf("Message context mismatch: expected %s, got %s", testMessage.Context, retrieved.Context)
		}
		if retrieved.FromUser != testMessage.FromUser {
			t.Errorf("Message from_user mismatch: expected %s, got %s", testMessage.FromUser, retrieved.FromUser)
		}
	}
}

// TestBasicMessageTypes validates all 6 message types structure and validation
func TestBasicMessageTypes(t *testing.T) {
	// For now, we'll test message structure validation without WebSocket connections
	// This validates the foundation is working before adding full integration tests
	
	// Test all 6 message types with their expected structure
	messageTypes := []struct {
		msgType   string
		context   string
		fromRole  string
		hasToUser bool
	}{
		{"instructor_inbox", "question", "student", false},
		{"inbox_response", "answer", "instructor", true},
		{"request", "code", "instructor", true},
		{"request_response", "code_submission", "student", false},
		{"analytics", "engagement", "student", false},
		{"instructor_broadcast", "announcement", "instructor", false},
	}
	
	for i, msgTest := range messageTypes {
		t.Run(msgTest.msgType, func(t *testing.T) {
			// Create a message structure for validation
			content := map[string]interface{}{
				"text":      fmt.Sprintf("Test message %d: %s", i+1, msgTest.msgType),
				"test_id":   i + 1,
				"timestamp": time.Now().Unix(),
			}
			
			message := &types.Message{
				ID:        fmt.Sprintf("msg-%d", i+1),
				SessionID: "test-session",
				Type:      msgTest.msgType,
				Context:   msgTest.context,
				FromUser:  "test-user",
				Content:   content,
				Timestamp: time.Now(),
			}
			
			if msgTest.hasToUser {
				toUser := "target-user"
				message.ToUser = &toUser
			}
			
			// Validate message structure
			if err := message.Validate(); err != nil {
				t.Errorf("Message validation failed for %s: %v", msgTest.msgType, err)
			}
			
			// Validate message type is recognized
			validTypes := []string{"instructor_inbox", "inbox_response", "request", "request_response", "analytics", "instructor_broadcast"}
			found := false
			for _, validType := range validTypes {
				if message.Type == validType {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Message type %s not in valid types", message.Type)
			}
			
			// Test context handling
			if message.Context == "" {
				t.Errorf("Message context should not be empty")
			}
			
			// Test to_user field handling
			if msgTest.hasToUser && message.ToUser == nil {
				t.Errorf("Message type %s should have to_user field", msgTest.msgType)
			} else if !msgTest.hasToUser && message.ToUser != nil {
				t.Errorf("Message type %s should not have to_user field", msgTest.msgType)
			}
		})
	}
}

// TestSingleConnectionScenarios validates basic instructor-student communication
func TestSingleConnectionScenarios(t *testing.T) {
	// Generate simple scenario
	scenario := fixtures.GenerateClassroomScenario(1, 1)
	
	// Create scenario runner
	runner, err := fixtures.NewScenarioRunner(t, scenario)
	if err != nil {
		t.Fatalf("Failed to create scenario runner: %v", err)
	}
	
	// Create and connect clients
	instructorClient, _ := runner.CreateClient(scenario.InstructorIDs[0], "instructor")
	studentClient, _ := runner.CreateClient(scenario.StudentIDs[0], "student")
	
	ctx := context.Background()
	if err := runner.ConnectAllClients(ctx); err != nil {
		t.Fatalf("Failed to connect clients: %v", err)
	}
	
	// Wait for connections
	time.Sleep(200 * time.Millisecond)
	
	t.Run("InstructorBroadcast", func(t *testing.T) {
		// Instructor sends broadcast
		content := map[string]interface{}{
			"text": "Welcome to the class!",
		}
		
		err := instructorClient.SendMessage("instructor_broadcast", "announcement", content, "")
		if err != nil {
			t.Fatalf("Failed to send broadcast: %v", err)
		}
		
		// Student should receive it (skip system messages)
		message, err := studentClient.ReceiveMessageOfType("instructor_broadcast", 3 * time.Second)
		if err != nil {
			t.Fatalf("Student did not receive broadcast: %v", err)
		}
		
		if message.Type != "instructor_broadcast" {
			t.Errorf("Wrong message type: expected instructor_broadcast, got %s", message.Type)
		}
	})
	
	t.Run("StudentQuestion", func(t *testing.T) {
		// Student asks question
		content := map[string]interface{}{
			"text": "I have a question about homework",
		}
		
		err := studentClient.SendMessage("instructor_inbox", "question", content, "")
		if err != nil {
			t.Fatalf("Failed to send question: %v", err)
		}
		
		// Instructor should receive it (skip system messages)
		message, err := instructorClient.ReceiveMessageOfType("instructor_inbox", 3 * time.Second)
		if err != nil {
			t.Fatalf("Instructor did not receive question: %v", err)
		}
		
		if message.Type != "instructor_inbox" {
			t.Errorf("Wrong message type: expected instructor_inbox, got %s", message.Type)
		}
	})
	
	t.Run("BidirectionalExchange", func(t *testing.T) {
		// Instructor requests code
		requestContent := map[string]interface{}{
			"text": "Please share your solution to problem 1",
		}
		
		err := instructorClient.SendMessage("request", "code", requestContent, scenario.StudentIDs[0])
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		
		// Student receives request (skip system messages)
		request, err := studentClient.ReceiveMessageOfType("request", 3 * time.Second)
		if err != nil {
			t.Fatalf("Student did not receive request: %v", err)
		}
		
		var actualToUser string
		if request.ToUser != nil {
			actualToUser = *request.ToUser
		}
		if request.Type != "request" || actualToUser != scenario.StudentIDs[0] {
			t.Errorf("Request not properly delivered: type=%s, to_user=%s", request.Type, actualToUser)
		}
		
		// Student responds
		responseContent := map[string]interface{}{
			"code": "def solution(): return 42",
			"language": "python",
		}
		
		err = studentClient.SendMessage("request_response", "code_submission", responseContent, "")
		if err != nil {
			t.Fatalf("Failed to send response: %v", err)
		}
		
		// Instructor receives response (skip system messages)
		response, err := instructorClient.ReceiveMessageOfType("request_response", 3 * time.Second)
		if err != nil {
			t.Fatalf("Instructor did not receive response: %v", err)
		}
		
		if response.Type != "request_response" {
			t.Errorf("Wrong response type: expected request_response, got %s", response.Type)
		}
	})
}

// TestRoleBasedPermissions validates that role restrictions are enforced
func TestRoleBasedPermissions(t *testing.T) {
	scenario := fixtures.GenerateClassroomScenario(1, 1)
	runner, err := fixtures.NewScenarioRunner(t, scenario)
	if err != nil {
		t.Fatalf("Failed to create scenario runner: %v", err)
	}
	
	instructorClient, _ := runner.CreateClient(scenario.InstructorIDs[0], "instructor")
	studentClient, _ := runner.CreateClient(scenario.StudentIDs[0], "student")
	
	ctx := context.Background()
	if err := runner.ConnectAllClients(ctx); err != nil {
		t.Fatalf("Failed to connect clients: %v", err)
	}
	
	time.Sleep(200 * time.Millisecond)
	
	// Test student message types (should work)
	studentMessageTypes := []string{"instructor_inbox", "request_response", "analytics"}
	for _, msgType := range studentMessageTypes {
		content := map[string]interface{}{"text": "test"}
		err := studentClient.SendMessage(msgType, "general", content, "")
		if err != nil {
			t.Errorf("Student should be able to send %s: %v", msgType, err)
		}
	}
	
	// Test instructor message types (should work)
	instructorMessageTypes := []string{"inbox_response", "request", "instructor_broadcast"}
	for _, msgType := range instructorMessageTypes {
		content := map[string]interface{}{"text": "test"}
		toUser := ""
		if msgType != "instructor_broadcast" {
			toUser = scenario.StudentIDs[0]
		}
		err := instructorClient.SendMessage(msgType, "general", content, toUser)
		if err != nil {
			t.Errorf("Instructor should be able to send %s: %v", msgType, err)
		}
	}
	
	// Note: Invalid role/message type combinations would be caught by the server
	// and result in messages being dropped. Full validation would require
	// checking that invalid messages don't appear in recipients.
}

// TestContextFieldHandling validates context field behavior
func TestContextFieldHandling(t *testing.T) {
	scenario := fixtures.GenerateClassroomScenario(1, 1)
	runner, err := fixtures.NewScenarioRunner(t, scenario)
	if err != nil {
		t.Fatalf("Failed to create scenario runner: %v", err)
	}
	
	instructorClient, _ := runner.CreateClient(scenario.InstructorIDs[0], "instructor")
	studentClient, _ := runner.CreateClient(scenario.StudentIDs[0], "student")
	
	ctx := context.Background()
	if err := runner.ConnectAllClients(ctx); err != nil {
		t.Fatalf("Failed to connect clients: %v", err)
	}
	
	time.Sleep(200 * time.Millisecond)
	
	t.Run("CustomContext", func(t *testing.T) {
		// Send message with custom context
		content := map[string]interface{}{"text": "Test with custom context"}
		err := instructorClient.SendMessage("instructor_broadcast", "emergency", content, "")
		if err != nil {
			t.Fatalf("Failed to send message with custom context: %v", err)
		}
		
		// Receive and verify context is preserved (skip system messages)
		message, err := studentClient.ReceiveMessageOfType("instructor_broadcast", 3 * time.Second)
		if err != nil {
			t.Fatalf("Failed to receive message: %v", err)
		}
		
		if message.Context != "emergency" {
			t.Errorf("Context not preserved: expected 'emergency', got '%s'", message.Context)
		}
	})
	
	t.Run("DefaultContext", func(t *testing.T) {
		// Send message without context (should default to "general")
		content := map[string]interface{}{"text": "Test with default context"}
		err := instructorClient.SendMessage("instructor_broadcast", "", content, "")
		if err != nil {
			t.Fatalf("Failed to send message with empty context: %v", err)
		}
		
		// Receive and verify context defaults to "general" (skip system messages)
		message, err := studentClient.ReceiveMessageOfType("instructor_broadcast", 3 * time.Second)
		if err != nil {
			t.Fatalf("Failed to receive message: %v", err)
		}
		
		if message.Context != "general" {
			t.Errorf("Context should default to 'general', got '%s'", message.Context)
		}
	})
	
	// Test various context variations for each message type
	contextVariations := fixtures.GenerateContextVariations()
	
	for msgType, contexts := range contextVariations {
		if len(contexts) == 0 {
			continue
		}
		
		t.Run(fmt.Sprintf("ContextVariations_%s", msgType), func(t *testing.T) {
			for _, context := range contexts {
				content := map[string]interface{}{
					"text": fmt.Sprintf("Testing context: %s", context),
				}
				
				var sender *fixtures.TestClient
				var toUser string
				
				// Choose appropriate sender based on message type
				if msgType == "instructor_inbox" || msgType == "request_response" || msgType == "analytics" {
					sender = studentClient
				} else {
					sender = instructorClient
					if msgType != "instructor_broadcast" {
						toUser = scenario.StudentIDs[0]
					}
				}
				
				err := sender.SendMessage(msgType, context, content, toUser)
				if err != nil {
					t.Errorf("Failed to send %s with context %s: %v", msgType, context, err)
				}
			}
		})
	}
}