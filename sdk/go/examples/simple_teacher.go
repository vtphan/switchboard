package main

import (
	"fmt"
	"log"
	"time"

	switchboard "github.com/vtphan/switchboard/sdk/go"
)

// Simple teacher example demonstrating V2 simplified API
func main() {
	// Create client with V2 simplified configuration
	client, err := switchboard.NewClient(switchboard.Config{
		UserID: "teacher_demo",
		Role:   switchboard.RoleInstructor,
		WsURL:  "ws://localhost:8080/ws",

		// 4 message type hooks
		OnBroadcastToInstructors: func(msg *switchboard.Message) {
			fmt.Printf("📚 Student question from %s: %s\n", 
				msg.FromUser, getMessageText(msg))
		},

		// 2 state change hooks
		OnConnectionChange: func(state switchboard.ConnectionState, err error) {
			fmt.Printf("Connection: %s\n", state)
		},
		OnSessionChange: func(session *switchboard.Session) {
			if session.Active {
				fmt.Printf("Session active: %s\n", session.Name)
			}
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Connect and start session
	fmt.Println("Connecting...")
	if err := client.Connect(); err != nil {
		log.Fatal(err)
	}

	time.Sleep(1 * time.Second)

	fmt.Println("Starting session...")
	if err := client.StartSession("Quick Demo Session"); err != nil {
		log.Fatal(err)
	}

	// Send messages using V2 simple API
	client.BroadcastToStudents("Welcome to the demo!")

	time.Sleep(2 * time.Second)

	// Object-based message sending
	client.BroadcastToStudents(map[string]interface{}{
		"text":         "Here's a basic Go function:",
		"context":      "instruction",
		"code_snippet": "func hello() {\n    fmt.Println(\"Hello, World!\")\n}",
		"important":    true,
	})

	// Keep running for a bit
	fmt.Println("Running for 30 seconds...")
	time.Sleep(30 * time.Second)

	// Clean up
	fmt.Println("Ending session...")
	client.EndSession()
	client.Disconnect()
	fmt.Println("Done!")
}

func getMessageText(msg *switchboard.Message) string {
	if text, ok := msg.Content["text"].(string); ok {
		return text
	}
	return "[no text content]"
}