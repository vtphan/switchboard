package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	switchboard "github.com/vtphan/switchboard/sdk/go"
	"github.com/vtphan/switchboard/sdk/go/snapshot-analyzers/shared"
)

func main() {
	fmt.Println("🎓 Alice's Coding Session Starting...")
	fmt.Println("Focus: Basic functions and data structures")
	fmt.Println("Sending code snapshots every 15 seconds")
	
	// Create Alice's client
	client, err := switchboard.NewClient(switchboard.Config{
		UserID: "alice",
		Role:   switchboard.RoleStudent,
		WsURL:  "ws://localhost:8080/ws",
		
		// Student receives feedback from agents
		OnDirectMessage: func(msg *switchboard.Message) {
			handleAnalysisFeedback(msg)
		},
		
		OnBroadcastToStudents: func(msg *switchboard.Message) {
			if content, ok := msg.Content["text"].(string); ok {
				fmt.Printf("📢 Instructor: %s\n", content)
			}
		},
		
		OnConnectionChange: func(state switchboard.ConnectionState, err error) {
			switch state {
			case switchboard.ConnectionStateConnected:
				fmt.Println("✅ Connected to Switchboard")
			case switchboard.ConnectionStateDisconnected:
				fmt.Println("❌ Disconnected from Switchboard")
			case switchboard.ConnectionStateError:
				fmt.Printf("🚨 Connection error: %v\n", err)
			}
		},
		
		OnSessionChange: func(session *switchboard.Session) {
			if session.Active {
				fmt.Printf("📝 Joined session: %s\n", session.Name)
			} else {
				fmt.Println("⏸️  Session ended - waiting for next session")
			}
		},
	})
	
	if err != nil {
		log.Fatalf("Failed to create Alice's client: %v", err)
	}
	
	// Connect to the server
	fmt.Println("🔌 Connecting to Switchboard...")
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	
	// Start sending code snapshots
	go startCodeSnapshotSender(client)
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	fmt.Println("🚀 Alice is ready! Press Ctrl+C to stop.")
	fmt.Println("────────────────────────────────────────")
	
	<-sigChan
	fmt.Println("\n👋 Alice is logging off...")
	client.Disconnect()
}

func startCodeSnapshotSender(client *switchboard.Client) {
	codeIndex := 0
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	
	// Send first snapshot immediately after a short delay
	time.Sleep(2 * time.Second)
	sendCodeSnapshot(client, &codeIndex)
	
	for {
		select {
		case <-ticker.C:
			sendCodeSnapshot(client, &codeIndex)
		}
	}
}

func sendCodeSnapshot(client *switchboard.Client, codeIndex *int) {
	if !client.IsSessionActive() {
		fmt.Println("⏳ Waiting for session to start...")
		return
	}
	
	// Get next code sample from Alice's profile
	sample := shared.GetNextCodeSample(shared.AliceProfile, *codeIndex)
	*codeIndex++
	
	// Create the message
	message := shared.GenerateCodeSnapshotMessage("alice", sample)
	
	// Send to all instructors (agents)
	err := client.BroadcastToInstructors(message)
	if err != nil {
		fmt.Printf("❌ Failed to send code snapshot: %v\n", err)
		return
	}
	
	// Display what was sent
	fmt.Printf("📤 Sent code snapshot #%d: %s\n", *codeIndex, sample.FileName)
	fmt.Printf("   Exercise: %s\n", sample.ExerciseID)
	
	// Show a preview of the code
	lines := splitLines(sample.Code)
	if len(lines) > 0 {
		fmt.Printf("   Code: %s", lines[0])
		if len(lines) > 1 {
			fmt.Printf("... (%d more lines)\n", len(lines)-1)
		} else {
			fmt.Println()
		}
	}
	
	fmt.Println("   ⏱️  Waiting for agent analysis...")
}

func handleAnalysisFeedback(msg *switchboard.Message) {
	// Extract analysis data
	analysisData, ok := msg.Content["analysis"].(map[string]interface{})
	if !ok {
		fmt.Printf("📥 Received feedback from %s (unable to parse analysis)\n", msg.FromUser)
		return
	}
	
	agentType, _ := analysisData["agent_type"].(string)
	score, _ := analysisData["score"].(float64)
	summary, _ := analysisData["summary"].(string)
	
	// Color-coded output based on agent type
	var agentIcon string
	switch agentType {
	case "syntax":
		agentIcon = "🎨"
	case "logic":
		agentIcon = "🔍"
	default:
		agentIcon = "🤖"
	}
	
	fmt.Printf("%s %s Agent Feedback (Score: %.0f/100)\n", agentIcon, 
		capitalizeFirst(agentType), score)
	fmt.Printf("   %s\n", summary)
	
	// Show issues if any
	if issues, ok := analysisData["issues"].([]interface{}); ok && len(issues) > 0 {
		fmt.Printf("   Issues found:\n")
		for i, issueData := range issues {
			if issue, ok := issueData.(map[string]interface{}); ok {
				severity, _ := issue["severity"].(string)
				message, _ := issue["message"].(string)
				line, _ := issue["line"].(float64)
				
				severityIcon := getSeverityIcon(severity)
				fmt.Printf("     %s Line %.0f: %s\n", severityIcon, line, message)
				
				if suggestion, ok := issue["suggestion"].(string); ok && suggestion != "" {
					fmt.Printf("       💡 Suggestion: %s\n", suggestion)
				}
			}
			
			// Limit to first 3 issues for readability
			if i >= 2 {
				remaining := len(issues) - i - 1
				if remaining > 0 {
					fmt.Printf("     ... and %d more issues\n", remaining)
				}
				break
			}
		}
	}
	
	// Show positive feedback if any
	if feedback, ok := analysisData["positive_feedback"].([]interface{}); ok && len(feedback) > 0 {
		fmt.Printf("   ✨ Positive feedback:\n")
		for _, feedbackItem := range feedback {
			if fb, ok := feedbackItem.(string); ok {
				fmt.Printf("     ✅ %s\n", fb)
			}
		}
	}
	
	fmt.Println("────────────────────────────────────────")
}

func splitLines(code string) []string {
	lines := make([]string, 0)
	currentLine := ""
	
	for _, char := range code {
		if char == '\n' {
			if currentLine != "" {
				lines = append(lines, currentLine)
				currentLine = ""
			}
		} else {
			currentLine += string(char)
		}
	}
	
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	
	return lines
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}

func getSeverityIcon(severity string) string {
	switch severity {
	case "minor":
		return "⚠️"
	case "major":
		return "🚨"
	case "critical":
		return "🔥"
	default:
		return "ℹ️"
	}
}