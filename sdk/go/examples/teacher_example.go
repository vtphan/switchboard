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

// Full-featured teacher example demonstrating V2 simplified API
func main() {
	// Create client with all 6 hooks configured
	client, err := switchboard.NewClient(switchboard.Config{
		UserID: "prof_smith",
		Role:   switchboard.RoleInstructor,
		WsURL:  "ws://localhost:8080/ws",

		// 4 message type hooks
		OnBroadcastToInstructors: handleStudentQuestion,
		OnBroadcastToStudents:    handleBroadcast,
		OnDirectMessage:          handleDirectMessage,
		OnSystem:                 handleSystemMessage,

		// 2 state change hooks
		OnConnectionChange: handleConnectionChange,
		OnSessionChange:    handleSessionChange,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Connect to the server
	fmt.Println("Connecting to Switchboard...")
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Start interactive mode
	go commandLoop(client)

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\nTeacher client connected!")
	fmt.Println("Commands:")
	fmt.Println("  start <name>        - Start session")
	fmt.Println("  end                 - End session") 
	fmt.Println("  announce <text>     - Send announcement")
	fmt.Println("  dm <user> <text>    - Send direct message")
	fmt.Println("  quit                - Exit")
	fmt.Println()

	// Auto-start session after a moment
	time.Sleep(2 * time.Second)
	if !client.IsSessionActive() {
		fmt.Println("Auto-starting demo session...")
		client.StartSession("Go Programming Demo")
		
		time.Sleep(1 * time.Second)
		client.BroadcastToStudents("Welcome to our Go programming class!")
	}

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down...")

	// End session if active
	if client.IsSessionActive() {
		client.EndSession()
	}

	client.Disconnect()
	fmt.Println("Teacher client disconnected")
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
		
		switch command {
		case "start":
			if len(parts) < 2 {
				fmt.Println("Usage: start <session_name>")
				continue
			}
			sessionName := strings.Join(parts[1:], " ")
			if err := client.StartSession(sessionName); err != nil {
				fmt.Printf("Failed to start session: %v\n", err)
			} else {
				fmt.Printf("✓ Session '%s' started\n", sessionName)
			}
			
		case "end":
			if err := client.EndSession(); err != nil {
				fmt.Printf("Failed to end session: %v\n", err)
			} else {
				fmt.Println("✓ Session ended")
			}
			
		case "announce":
			if len(parts) < 2 {
				fmt.Println("Usage: announce <text>")
				continue
			}
			text := strings.Join(parts[1:], " ")
			
			// Use object-based message with metadata
			if err := client.BroadcastToStudents(map[string]interface{}{
				"text":         text,
				"context":      "announcement",
				"instructor":   client.GetConnectionStatus(),
				"timestamp":    time.Now().Format("15:04:05"),
			}); err != nil {
				fmt.Printf("Failed to send announcement: %v\n", err)
			} else {
				fmt.Println("✓ Announcement sent")
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
			
		case "quit":
			return
			
		default:
			fmt.Println("Unknown command. Try: start, end, announce, dm, quit")
		}
	}
}

// Event handlers for the 6 hooks

func handleStudentQuestion(msg *switchboard.Message) {
	text := getMessageText(msg)
	urgentFlag := ""
	if urgent, ok := msg.Content["urgent"].(bool); ok && urgent {
		urgentFlag = " 🚨"
	}
	
	fmt.Printf("\n📚 Question from %s%s: %s\n", msg.FromUser, urgentFlag, text)
	
	// Show code snippet if included
	if code, ok := msg.Content["code_snippet"].(string); ok {
		fmt.Printf("💻 Code: %s\n", code)
	}
	
	fmt.Print("> ")
}

func handleBroadcast(msg *switchboard.Message) {
	text := getMessageText(msg)
	fmt.Printf("📢 Broadcast sent: %s\n> ", text)
}

func handleDirectMessage(msg *switchboard.Message) {
	text := getMessageText(msg)
	fmt.Printf("💌 Direct message from %s: %s\n> ", msg.FromUser, text)
}

func handleSystemMessage(msg *switchboard.Message) {
	if event, ok := msg.Content["event"].(string); ok {
		switch event {
		case "user_connected":
			if userID, ok := msg.Content["user_id"].(string); ok {
				if role, ok := msg.Content["role"].(string); ok && role == "student" {
					fmt.Printf("👋 Student %s joined\n> ", userID)
				}
			}
		case "user_disconnected":
			if userID, ok := msg.Content["user_id"].(string); ok {
				fmt.Printf("👋 %s left\n> ", userID)
			}
		}
	}
}

func handleConnectionChange(state switchboard.ConnectionState, err error) {
	switch state {
	case switchboard.ConnectionStateConnecting:
		fmt.Println("🔄 Connecting...")
	case switchboard.ConnectionStateConnected:
		fmt.Println("✅ Connected!")
	case switchboard.ConnectionStateDisconnected:
		fmt.Println("❌ Disconnected")
	case switchboard.ConnectionStateError:
		fmt.Printf("⚠️ Connection error: %v\n", err)
	}
}

func handleSessionChange(session *switchboard.Session) {
	if session.Active {
		fmt.Printf("📚 Session active: %s\n", session.Name)
	} else {
		fmt.Println("📚 Session ended")
	}
}

func getMessageText(msg *switchboard.Message) string {
	if text, ok := msg.Content["text"].(string); ok {
		return text
	}
	return "[no text content]"
}