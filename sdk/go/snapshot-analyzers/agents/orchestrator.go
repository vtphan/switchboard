package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	switchboard "github.com/vtphan/switchboard/sdk/go"
)

func main() {
	fmt.Println("🎭 Code Snapshot Analyzer Orchestrator")
	fmt.Println("Coordinating session lifecycle and participant activity")
	
	// Create orchestrator client (instructor role)
	client, err := switchboard.NewClient(switchboard.Config{
		UserID: "orchestrator",
		Role:   switchboard.RoleInstructor,
		WsURL:  "ws://localhost:8080/ws",
		
		OnConnectionChange: func(state switchboard.ConnectionState, err error) {
			switch state {
			case switchboard.ConnectionStateConnected:
				fmt.Println("✅ Orchestrator connected")
			case switchboard.ConnectionStateDisconnected:
				fmt.Println("❌ Orchestrator disconnected")
			case switchboard.ConnectionStateError:
				fmt.Printf("🚨 Connection error: %v\n", err)
			}
		},
		
		OnSessionChange: func(session *switchboard.Session) {
			if session.Active {
				fmt.Printf("📝 Session active: %s\n", session.Name)
			} else {
				fmt.Println("⏸️  No active session")
			}
		},
		
		OnBroadcastToInstructors: func(msg *switchboard.Message) {
			// Monitor student activity
			msgType, _ := msg.Content["type"].(string)
			if msg.Context == "submission" && msgType == "code_snapshot" {
				studentID, _ := msg.Content["student_id"].(string)
				fileName, _ := msg.Content["file_name"].(string)
				fmt.Printf("📊 Code snapshot from %s: %s\n", studentID, fileName)
			}
		},
	})
	
	if err != nil {
		log.Fatalf("Failed to create orchestrator: %v", err)
	}
	
	// Connect to the server
	fmt.Println("🔌 Connecting to Switchboard...")
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	
	// Wait a moment then start the demo session
	time.Sleep(2 * time.Second)
	
	fmt.Println("🚀 Starting Code Analysis Demo Session...")
	sessionName := fmt.Sprintf("Code Analysis Demo - %s", time.Now().Format("15:04:05"))
	
	if err := client.StartSession(sessionName); err != nil {
		// Session might already be active, that's OK
		if err.Error() != "API error (409): Session already active" {
			log.Printf("Note: %v", err)
		}
		fmt.Println("📝 Session is ready (possibly already active)")
	} else {
		fmt.Println("📝 Demo session started successfully!")
	}
	
	// Send welcome message to students
	time.Sleep(1 * time.Second)
	welcomeMsg := "🎉 Welcome to the Code Snapshot Analyzer Demo! " +
		"Students Alice and Bob will be sending code snapshots every 15 seconds. " +
		"Our syntax and logic analysis agents will provide real-time feedback."
	
	if err := client.BroadcastToStudents(welcomeMsg); err != nil {
		fmt.Printf("Warning: Could not send welcome message: %v\n", err)
	} else {
		fmt.Println("📢 Welcome message sent to students")
	}
	
	// Start monitoring and status updates
	go monitorDemoActivity(client)
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	fmt.Println("🎯 Demo session is running! Press Ctrl+C to end.")
	fmt.Println("════════════════════════════════════════")
	fmt.Println("Expected participants:")
	fmt.Println("  👩‍💻 Alice (student) - Basic functions")
	fmt.Println("  👨‍💻 Bob (student) - Algorithms")
	fmt.Println("  🎨 Syntax Agent (instructor) - Style analysis")
	fmt.Println("  🔍 Logic Agent (instructor) - Bug detection")
	fmt.Println("════════════════════════════════════════")
	
	<-sigChan
	fmt.Println("\n🛑 Ending demo session...")
	
	// Send goodbye message
	goodbyeMsg := "👋 Demo session ending. Thank you for participating in the Code Snapshot Analyzer demo!"
	client.BroadcastToStudents(goodbyeMsg)
	
	// End the session
	if err := client.EndSession(); err != nil {
		fmt.Printf("Note: %v\n", err)
	} else {
		fmt.Println("📝 Demo session ended successfully")
	}
	
	client.Disconnect()
	fmt.Println("👋 Orchestrator shutting down...")
}

func monitorDemoActivity(client *switchboard.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	startTime := time.Now()
	
	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(startTime)
			
			// Send periodic status updates
			statusMsg := fmt.Sprintf("📊 Demo Status: Running for %v | "+
				"Students are sending code snapshots every 15s | "+
				"Agents are providing real-time analysis",
				elapsed.Round(time.Second))
			
			if err := client.BroadcastToStudents(statusMsg); err == nil {
				fmt.Printf("📈 Status update sent (demo running %v)\n", 
					elapsed.Round(time.Second))
			}
			
			// Every 2 minutes, send an encouraging message
			if int(elapsed.Minutes())%2 == 0 && elapsed.Minutes() >= 2 {
				encouragementMsgs := []string{
					"🌟 Great job everyone! The analysis agents are working hard to help improve your code!",
					"💡 Remember: Every piece of feedback is an opportunity to learn and grow as a programmer!",
					"🚀 Keep coding! The agents are analyzing your algorithms and style in real-time!",
					"🎯 Demo tip: Try implementing different algorithms to see how the logic agent responds!",
				}
				
				msgIndex := int(elapsed.Minutes()/2) % len(encouragementMsgs)
				client.BroadcastToStudents(encouragementMsgs[msgIndex])
				fmt.Println("💬 Sent encouragement message to students")
			}
		}
	}
}