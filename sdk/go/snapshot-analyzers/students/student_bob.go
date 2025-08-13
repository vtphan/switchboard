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
	fmt.Println("🧠 Bob's Coding Session Starting...")
	fmt.Println("Focus: Algorithms and complex logic")
	fmt.Println("Sending code snapshots every 15 seconds (offset by 7.5s)")
	
	// Create Bob's client
	client, err := switchboard.NewClient(switchboard.Config{
		UserID: "bob",
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
		log.Fatalf("Failed to create Bob's client: %v", err)
	}
	
	// Connect to the server
	fmt.Println("🔌 Connecting to Switchboard...")
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	
	// Start sending code snapshots (offset by 7.5 seconds from Alice)
	go startCodeSnapshotSender(client)
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	fmt.Println("🚀 Bob is ready! Press Ctrl+C to stop.")
	fmt.Println("────────────────────────────────────────")
	
	<-sigChan
	fmt.Println("\n👋 Bob is logging off...")
	client.Disconnect()
}

func startCodeSnapshotSender(client *switchboard.Client) {
	codeIndex := 0
	
	// Start with a 7.5 second offset from Alice
	time.Sleep(9500 * time.Millisecond) // 2s base + 7.5s offset from Alice
	
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	
	// Send first snapshot immediately
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
	
	// Get next code sample from Bob's profile
	sample := shared.GetNextCodeSample(shared.BobProfile, *codeIndex)
	*codeIndex++
	
	// Create the message
	message := shared.GenerateCodeSnapshotMessage("bob", sample)
	
	// Send to all instructors (agents)
	err := client.BroadcastToInstructors(message)
	if err != nil {
		fmt.Printf("❌ Failed to send code snapshot: %v\n", err)
		return
	}
	
	// Display what was sent
	fmt.Printf("📤 Sent code snapshot #%d: %s\n", *codeIndex, sample.FileName)
	fmt.Printf("   Exercise: %s\n", sample.ExerciseID)
	
	// Show a preview of the code with complexity indication
	lines := splitLines(sample.Code)
	if len(lines) > 0 {
		fmt.Printf("   Code: %s", lines[0])
		if len(lines) > 1 {
			complexity := getComplexityIndicator(len(lines), sample.Code)
			fmt.Printf("... (%d more lines) %s\n", len(lines)-1, complexity)
		} else {
			fmt.Println()
		}
	}
	
	// Show metadata hints
	if metadata, ok := sample.Metadata["topic"]; ok {
		fmt.Printf("   📝 Topic: %s\n", metadata)
	}
	if complexity, ok := sample.Metadata["complexity"]; ok {
		fmt.Printf("   ⚡ Complexity: %s\n", complexity)
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
	
	fmt.Printf("%s %s Agent Analysis (Score: %.0f/100)\n", agentIcon, 
		capitalizeFirst(agentType), score)
	fmt.Printf("   %s\n", summary)
	
	// Show issues with more technical detail for Bob
	if issues, ok := analysisData["issues"].([]interface{}); ok && len(issues) > 0 {
		fmt.Printf("   🔍 Technical Issues:\n")
		for i, issueData := range issues {
			if issue, ok := issueData.(map[string]interface{}); ok {
				issueType, _ := issue["type"].(string)
				severity, _ := issue["severity"].(string)
				message, _ := issue["message"].(string)
				line, _ := issue["line"].(float64)
				ruleID, _ := issue["rule_id"].(string)
				
				severityIcon := getSeverityIcon(severity)
				fmt.Printf("     %s [%s] Line %.0f: %s\n", severityIcon, issueType, line, message)
				
				if suggestion, ok := issue["suggestion"].(string); ok && suggestion != "" {
					fmt.Printf("       💡 Fix: %s\n", suggestion)
				}
				
				if ruleID != "" {
					fmt.Printf("       🏷️  Rule: %s\n", ruleID)
				}
			}
			
			// Show more issues for Bob since he's working on complex algorithms
			if i >= 4 {
				remaining := len(issues) - i - 1
				if remaining > 0 {
					fmt.Printf("     ... and %d more issues\n", remaining)
				}
				break
			}
		}
	}
	
	// Show positive feedback
	if feedback, ok := analysisData["positive_feedback"].([]interface{}); ok && len(feedback) > 0 {
		fmt.Printf("   ✨ Algorithmic Strengths:\n")
		for _, feedbackItem := range feedback {
			if fb, ok := feedbackItem.(string); ok {
				fmt.Printf("     ✅ %s\n", fb)
			}
		}
	}
	
	// Show processing time for performance awareness
	if processingTime, ok := analysisData["processing_time"].(string); ok {
		fmt.Printf("   ⏱️  Analysis time: %s\n", processingTime)
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

func getComplexityIndicator(lineCount int, code string) string {
	if lineCount > 20 {
		return "🔴 Complex"
	} else if lineCount > 10 {
		return "🟡 Medium"
	} else if containsRecursion(code) {
		return "🟣 Recursive"
	} else if containsNestedLoops(code) {
		return "🟠 Nested"
	} else {
		return "🟢 Simple"
	}
}

func containsRecursion(code string) bool {
	// Simple check for recursive calls
	return len(code) > 0 && (
		// Look for function name followed by recursive call pattern
		(code[0] >= 'A' && code[0] <= 'Z') && 
		len(code) > 50) // Rough heuristic
}

func containsNestedLoops(code string) bool {
	// Simple check for nested loops
	forCount := 0
	for i := 0; i < len(code)-2; i++ {
		if code[i:i+3] == "for" {
			forCount++
		}
	}
	return forCount >= 2
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