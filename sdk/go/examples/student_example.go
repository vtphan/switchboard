package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	switchboard "github.com/vtphan/switchboard/sdk/go"
)

// Full-featured student example demonstrating V2 simplified API
func main() {
	// Get student ID from command line or use default
	studentID := "student123"
	if len(os.Args) > 1 {
		studentID = os.Args[1]
	}

	// Create client with V2 simplified configuration
	client, err := switchboard.NewClient(switchboard.Config{
		UserID: studentID,
		Role:   switchboard.RoleStudent,
		WsURL:  "ws://localhost:8080/ws",

		// 4 message type hooks (only the ones students receive)
		OnBroadcastToStudents: handleAnnouncement,
		OnDirectMessage:       handleDirectMessage,
		OnBroadcastToInstructors: handleOwnQuestion, // Echo of own questions
		OnSystem:              handleSystemMessage,

		// 2 state change hooks
		OnConnectionChange: handleConnectionChange,
		OnSessionChange:    handleSessionChange,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Connect to the server
	fmt.Printf("Connecting to class as %s...\n", studentID)
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Start interactive mode
	go commandLoop(client)

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\nStudent client connected!")
	fmt.Println("Commands:")
	fmt.Println("  ask <question>      - Ask question to instructors")
	fmt.Println("  dm <user> <text>    - Send direct message")
	fmt.Println("  submit <description> - Submit code")
	fmt.Println("  quit                - Exit")
	fmt.Println()

	// Simulate some student activities after a delay
	go func() {
		time.Sleep(3 * time.Second)
		
		if client.IsSessionActive() {
			// Ask a sample question using simple string
			client.BroadcastToInstructors("How do I declare a variable in Go?")
			
			time.Sleep(5 * time.Second)
			
			// Submit code using object-based message
			client.BroadcastToInstructors(map[string]interface{}{
				"text":         "Here's my variable declaration attempt:",
				"context":      "submission",
				"code_snippet": "var name string = \"Alice\"",
				"exercise":     1,
			})
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down...")

	client.Disconnect()
	fmt.Println("Student client disconnected")
}

// Interactive command loop
func commandLoop(client *switchboard.Client) {
	scanner := bufio.NewScanner(os.Stdin)
	
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		parts := strings.SplitN(line, " ", 3)
		command := parts[0]
		
		if !client.IsSessionActive() && command != "quit" {
			fmt.Println("⚠️  No active session. Waiting for instructor to start class...")
			continue
		}
		
		switch command {
		case "ask":
			if len(parts) < 2 {
				fmt.Println("Usage: ask <question>")
				continue
			}
			question := strings.Join(parts[1:], " ")
			
			// Object-based message with context
			if err := client.BroadcastToInstructors(map[string]interface{}{
				"text":    question,
				"context": "question",
				"urgent":  strings.Contains(strings.ToLower(question), "urgent"),
			}); err != nil {
				fmt.Printf("Failed to send question: %v\n", err)
			} else {
				fmt.Println("✓ Question sent to instructors")
			}
			
		case "dm":
			if len(parts) < 3 {
				fmt.Println("Usage: dm <user> <message>")
				continue
			}
			userID := parts[1]
			message := strings.Join(parts[2:], " ")
			if err := client.DirectMessage(userID, message); err != nil {
				fmt.Printf("Failed to send direct message: %v\n", err)
			} else {
				fmt.Printf("✓ Direct message sent to %s\n", userID)
			}
			
		case "submit":
			if len(parts) < 2 {
				fmt.Println("Usage: submit <description>")
				fmt.Print("Enter your code: ")
				continue
			}
			description := strings.Join(parts[1:], " ")
			fmt.Print("Enter your code: ")
			if scanner.Scan() {
				code := scanner.Text()
				
				// Rich object-based submission
				if err := client.BroadcastToInstructors(map[string]interface{}{
					"text":         description,
					"context":      "submission",
					"code_snippet": code,
					"student_id":   client.GetConnectionStatus(),
					"timestamp":    time.Now().Format("15:04:05"),
				}); err != nil {
					fmt.Printf("Failed to submit code: %v\n", err)
				} else {
					fmt.Println("✓ Code submitted to instructors")
				}
			}
			
		case "quit":
			return
			
		default:
			fmt.Println("Unknown command. Try: ask, dm, submit, quit")
		}
	}
}

// Event handlers for the 6 hooks

func handleAnnouncement(msg *switchboard.Message) {
	text := getMessageText(msg)
	
	// Check if it's important/urgent
	if important, ok := msg.Content["important"].(bool); ok && important {
		fmt.Printf("\n🚨 URGENT ANNOUNCEMENT from %s: %s\n", msg.FromUser, text)
	} else {
		fmt.Printf("\n📢 %s: %s\n", msg.FromUser, text)
	}
	
	// Show code examples if included
	if code, ok := msg.Content["code_snippet"].(string); ok {
		fmt.Printf("💻 Code example:\n%s\n", code)
	}
	
	fmt.Print("> ")
}

func handleDirectMessage(msg *switchboard.Message) {
	text := getMessageText(msg)
	fmt.Printf("\n💌 Direct message from %s: %s\n> ", msg.FromUser, text)
}

func handleOwnQuestion(msg *switchboard.Message) {
	// This is the echo of the student's own question
	text := getMessageText(msg)
	fmt.Printf("\n✓ Your question was sent: %s\n> ", text)
}

func handleSystemMessage(msg *switchboard.Message) {
	// System messages are handled internally by the client
	// This hook is optional for custom system message handling
	if event, ok := msg.Content["event"].(string); ok {
		switch event {
		case "history_delivered":
			fmt.Println("📖 Class history loaded")
		}
	}
}

func handleConnectionChange(state switchboard.ConnectionState, err error) {
	switch state {
	case switchboard.ConnectionStateConnecting:
		fmt.Println("🔄 Connecting to class...")
	case switchboard.ConnectionStateConnected:
		fmt.Println("✅ Connected to class!")
	case switchboard.ConnectionStateDisconnected:
		fmt.Println("❌ Disconnected from class")
	case switchboard.ConnectionStateError:
		fmt.Printf("⚠️  Connection error: %v\n", err)
	}
}

func handleSessionChange(session *switchboard.Session) {
	if session.Active {
		fmt.Printf("📚 Joined session: %s (started by %s)\n", session.Name, session.StartedBy)
	} else {
		fmt.Println("📚 Session ended. Waiting for next session...")
	}
}

func getMessageText(msg *switchboard.Message) string {
	if text, ok := msg.Content["text"].(string); ok {
		return text
	}
	return "[no text content]"
}