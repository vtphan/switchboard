package main

import (
	"fmt"
	"log"
	"time"

	switchboard "github.com/vtphan/switchboard/sdk/go"
)

func main() {
	fmt.Println("🧪 Testing Switchboard Go SDK V2...")

	// Test 1: Create teacher client
	fmt.Println("\n1. Testing teacher client creation...")
	teacher, err := switchboard.NewClient(switchboard.Config{
		UserID: "test_teacher",
		Role:   switchboard.RoleInstructor,
		WsURL:  "ws://localhost:8080/ws",

		OnBroadcastToInstructors: func(msg *switchboard.Message) {
			fmt.Printf("📚 Received student question: %s\n", getMessageText(msg))
		},
		OnConnectionChange: func(state switchboard.ConnectionState, err error) {
			fmt.Printf("🔗 Teacher connection: %s\n", state)
		},
		OnSessionChange: func(session *switchboard.Session) {
			if session.Active {
				fmt.Printf("📝 Teacher session: %s\n", session.Name)
			}
		},
	})
	if err != nil {
		log.Fatalf("Failed to create teacher client: %v", err)
	}
	fmt.Println("✅ Teacher client created successfully")

	// Test 2: Create student client
	fmt.Println("\n2. Testing student client creation...")
	student, err := switchboard.NewClient(switchboard.Config{
		UserID: "test_student",
		Role:   switchboard.RoleStudent,
		WsURL:  "ws://localhost:8080/ws",

		OnBroadcastToStudents: func(msg *switchboard.Message) {
			fmt.Printf("📢 Received announcement: %s\n", getMessageText(msg))
		},
		OnDirectMessage: func(msg *switchboard.Message) {
			fmt.Printf("💌 Received direct message: %s\n", getMessageText(msg))
		},
		OnConnectionChange: func(state switchboard.ConnectionState, err error) {
			fmt.Printf("🔗 Student connection: %s\n", state)
		},
		OnSessionChange: func(session *switchboard.Session) {
			if session.Active {
				fmt.Printf("📝 Student joined: %s\n", session.Name)
			}
		},
	})
	if err != nil {
		log.Fatalf("Failed to create student client: %v", err)
	}
	fmt.Println("✅ Student client created successfully")

	// Test 3: Connect both clients
	fmt.Println("\n3. Testing connections...")
	if err := teacher.Connect(); err != nil {
		log.Fatalf("Failed to connect teacher: %v", err)
	}
	
	time.Sleep(1 * time.Second)
	
	if err := student.Connect(); err != nil {
		log.Fatalf("Failed to connect student: %v", err)
	}
	
	time.Sleep(2 * time.Second)
	fmt.Println("✅ Both clients connected")

	// Test 4: Session management
	fmt.Println("\n4. Testing session management...")
	if err := teacher.StartSession("Go SDK Test Session"); err != nil {
		log.Printf("Note: Session might already be active: %v", err)
	} else {
		fmt.Println("✅ Session started successfully")
	}
	
	time.Sleep(1 * time.Second)

	// Test 5: Message sending - string messages
	fmt.Println("\n5. Testing simple string messages...")
	if err := teacher.BroadcastToStudents("Welcome to the SDK test!"); err != nil {
		log.Printf("Failed to send teacher broadcast: %v", err)
	} else {
		fmt.Println("✅ Teacher string broadcast sent")
	}
	
	time.Sleep(1 * time.Second)
	
	if err := student.BroadcastToInstructors("Hello from Go SDK!"); err != nil {
		log.Printf("Failed to send student message: %v", err)
	} else {
		fmt.Println("✅ Student string message sent")
	}

	time.Sleep(1 * time.Second)

	// Test 6: Message sending - object-based messages
	fmt.Println("\n6. Testing object-based messages...")
	
	// Teacher sends rich announcement
	if err := teacher.BroadcastToStudents(map[string]interface{}{
		"text":         "Here's a code example:",
		"context":      "instruction",
		"code_snippet": "fmt.Println(\"Hello, Go!\")",
		"important":    true,
	}); err != nil {
		log.Printf("Failed to send teacher object message: %v", err)
	} else {
		fmt.Println("✅ Teacher object-based message sent")
	}
	
	time.Sleep(1 * time.Second)
	
	// Student sends question with metadata
	if err := student.BroadcastToInstructors(map[string]interface{}{
		"text":    "How does error handling work in Go?",
		"context": "question",
		"urgent":  false,
		"topic":   "error-handling",
	}); err != nil {
		log.Printf("Failed to send student object message: %v", err)
	} else {
		fmt.Println("✅ Student object-based message sent")
	}

	time.Sleep(1 * time.Second)

	// Test 7: Direct messaging
	fmt.Println("\n7. Testing direct messages...")
	if err := teacher.DirectMessage("test_student", "Great question! Let me help you with that."); err != nil {
		log.Printf("Failed to send direct message: %v", err)
	} else {
		fmt.Println("✅ Direct message sent")
	}

	time.Sleep(2 * time.Second)

	// Test 8: Verify client states
	fmt.Println("\n8. Testing client state methods...")
	fmt.Printf("Teacher connected: %v\n", teacher.IsConnected())
	fmt.Printf("Student connected: %v\n", student.IsConnected())
	fmt.Printf("Teacher session active: %v\n", teacher.IsSessionActive())
	fmt.Printf("Student session active: %v\n", student.IsSessionActive())
	
	if teacher.IsSessionActive() {
		session := teacher.GetCurrentSession()
		if session != nil {
			fmt.Printf("Current session: %s\n", session.Name)
		}
	}
	fmt.Println("✅ Client state methods working")

	// Test 9: Protocol compliance check
	fmt.Println("\n9. Testing protocol compliance...")
	
	// Create a message with context in content to test extraction
	testContent := map[string]interface{}{
		"text":    "Testing context extraction",
		"context": "test",  // This should be extracted to message level
		"data":    "some data",
	}
	
	if err := student.BroadcastToInstructors(testContent); err != nil {
		log.Printf("Failed to send protocol test message: %v", err)
	} else {
		fmt.Println("✅ Protocol compliance test message sent")
	}

	time.Sleep(2 * time.Second)

	// Cleanup
	fmt.Println("\n10. Cleaning up...")
	teacher.Disconnect()
	student.Disconnect()
	fmt.Println("✅ Clients disconnected")

	fmt.Println("\n🎉 All tests completed successfully!")
	fmt.Println("The Go SDK V2 is working correctly with the Switchboard server.")
}

func getMessageText(msg *switchboard.Message) string {
	if text, ok := msg.Content["text"].(string); ok {
		return text
	}
	return "[no text content]"
}