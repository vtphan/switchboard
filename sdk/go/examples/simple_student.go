package main

import (
	"fmt"
	"log"
	"time"

	switchboard "github.com/vtphan/switchboard/sdk/go"
)

// Simple student example demonstrating V2 simplified API
func main() {
	// Create client with V2 simplified configuration
	client, err := switchboard.NewClient(switchboard.Config{
		UserID: "student_demo",
		Role:   switchboard.RoleStudent,
		WsURL:  "ws://localhost:8080/ws",

		// 4 message type hooks (only the ones students receive)
		OnBroadcastToStudents: func(msg *switchboard.Message) {
			fmt.Printf("📢 %s: %s\n", msg.FromUser, getMessageText(msg))

			// Show code if included
			if code, ok := msg.Content["code_snippet"].(string); ok {
				fmt.Printf("💻 Code:\n%s\n", code)
			}
		},
		OnDirectMessage: func(msg *switchboard.Message) {
			fmt.Printf("💌 %s: %s\n", msg.FromUser, getMessageText(msg))
		},

		// 2 state change hooks
		OnConnectionChange: func(state switchboard.ConnectionState, err error) {
			fmt.Printf("Connection: %s\n", state)
		},
		OnSessionChange: func(session *switchboard.Session) {
			if session.Active {
				fmt.Printf("Joined session: %s\n", session.Name)
			} else {
				fmt.Println("Session ended")
			}
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Connect
	fmt.Println("Connecting to class...")
	if err := client.Connect(); err != nil {
		log.Fatal(err)
	}

	// Wait a moment then send some messages if session is active
	time.Sleep(2 * time.Second)

	if client.IsSessionActive() {
		fmt.Println("Asking a question...")

		// Simple string message
		client.BroadcastToInstructors("How do I create a slice in Go?")

		time.Sleep(3 * time.Second)

		// Object-based message with context and metadata
		client.BroadcastToInstructors(map[string]interface{}{
			"text":         "Here's my slice attempt:",
			"context":      "submission",
			"code_snippet": "var numbers []int = []int{1, 2, 3}",
			"exercise":     1,
		})
	} else {
		fmt.Println("No active session - waiting...")
	}

	// Keep running
	fmt.Println("Running for 30 seconds...")
	time.Sleep(30 * time.Second)

	// Disconnect
	client.Disconnect()
	fmt.Println("Disconnected")
}

func getMessageText(msg *switchboard.Message) string {
	if text, ok := msg.Content["text"].(string); ok {
		return text
	}
	return "[no text content]"
}