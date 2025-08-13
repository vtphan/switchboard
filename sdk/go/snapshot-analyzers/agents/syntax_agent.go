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
	fmt.Println("🎨 Syntax Analysis Agent Starting...")
	fmt.Println("Specialization: Code style, formatting, and naming conventions")
	
	// Initialize the syntax analysis engine
	syntaxEngine := shared.NewSyntaxAnalysisEngine()
	
	// Create the agent client (instructor role)
	var client *switchboard.Client
	client, err := switchboard.NewClient(switchboard.Config{
		UserID: "syntax_agent",
		Role:   switchboard.RoleInstructor,
		WsURL:  "ws://localhost:8080/ws",
		
		// Listen for student code snapshots
		OnBroadcastToInstructors: func(msg *switchboard.Message) {
			handleCodeSnapshot(client, syntaxEngine, msg)
		},
		
		OnConnectionChange: func(state switchboard.ConnectionState, err error) {
			switch state {
			case switchboard.ConnectionStateConnected:
				fmt.Println("✅ Syntax Agent connected to Switchboard")
			case switchboard.ConnectionStateDisconnected:
				fmt.Println("❌ Syntax Agent disconnected")
			case switchboard.ConnectionStateError:
				fmt.Printf("🚨 Connection error: %v\n", err)
			}
		},
		
		OnSessionChange: func(session *switchboard.Session) {
			if session.Active {
				fmt.Printf("📝 Monitoring session: %s\n", session.Name)
				fmt.Println("🔍 Ready to analyze code style and formatting...")
			} else {
				fmt.Println("⏸️  Session ended - waiting for next session")
			}
		},
	})
	
	if err != nil {
		log.Fatalf("Failed to create syntax agent: %v", err)
	}
	
	// Connect to the server
	fmt.Println("🔌 Connecting to Switchboard...")
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	fmt.Println("🚀 Syntax Agent is active! Press Ctrl+C to stop.")
	fmt.Println("════════════════════════════════════════")
	
	<-sigChan
	fmt.Println("\n👋 Syntax Agent shutting down...")
	client.Disconnect()
}

func handleCodeSnapshot(client *switchboard.Client, engine *shared.SyntaxAnalysisEngine, msg *switchboard.Message) {
	// Force debug output to stdout (not logs)
	fmt.Fprintf(os.Stdout, "🔔 [SYNTAX AGENT] Received message from %s\n", msg.FromUser)
	fmt.Fprintf(os.Stdout, "🔔 [SYNTAX AGENT] Message content keys: %v\n", getKeys(msg.Content))
	fmt.Fprintf(os.Stdout, "🔔 [SYNTAX AGENT] Full message: %+v\n", msg)
	os.Stdout.Sync()
	
	// Parse the code snapshot message - check for submission context and code_snapshot type
	// Context is at message level, type is in content
	msgType, _ := msg.Content["type"].(string)
	if msg.Context != "submission" || msgType != "code_snapshot" {
		fmt.Fprintf(os.Stdout, "   ℹ️  Ignoring non-code-snapshot message (context: %s, type: %s)\n", msg.Context, msgType)
		os.Stdout.Sync()
		return // Not a code snapshot, ignore
	}
	
	studentID, _ := msg.Content["student_id"].(string)
	code, _ := msg.Content["code"].(string)
	fileName, _ := msg.Content["file_name"].(string)
	exerciseID, _ := msg.Content["exercise_id"].(string)
	
	if studentID == "" || code == "" {
		fmt.Printf("⚠️  Received invalid code snapshot from %s\n", msg.FromUser)
		return
	}
	
	fmt.Printf("📥 Analyzing code from %s (%s)\n", studentID, fileName)
	fmt.Printf("   Exercise: %s\n", exerciseID)
	
	// Perform syntax analysis
	startTime := time.Now()
	result := engine.AnalyzeCode(code)
	analysisTime := time.Since(startTime)
	
	// Log analysis summary
	fmt.Printf("   ⚡ Analysis completed in %v\n", analysisTime)
	fmt.Printf("   📊 Score: %d/100 (%d issues found)\n", result.Score, len(result.Issues))
	
	// Display issues found for agent's awareness
	if len(result.Issues) > 0 {
		fmt.Printf("   🔍 Issues detected:\n")
		for i, issue := range result.Issues {
			fmt.Printf("     • %s (Line %d): %s\n", 
				capitalizeFirst(issue.Severity), issue.Line, issue.Message)
			if i >= 2 { // Limit console output
				fmt.Printf("     • ... and %d more issues\n", len(result.Issues)-i-1)
				break
			}
		}
	} else {
		fmt.Printf("   ✨ No style issues found!\n")
	}
	
	// Display positive feedback summary
	if len(result.PositiveFeedback) > 0 {
		fmt.Printf("   ✅ Positive aspects: %d strengths identified\n", len(result.PositiveFeedback))
	}
	
	// Create feedback message
	feedbackMsg := shared.GenerateAnalysisFeedbackMessage(studentID, result)
	
	// Send feedback directly to the student
	err := client.DirectMessage(studentID, feedbackMsg)
	if err != nil {
		fmt.Printf("❌ Failed to send feedback to %s: %v\n", studentID, err)
	} else {
		fmt.Printf("📤 Sent syntax feedback to %s\n", studentID)
	}
	
	fmt.Println("────────────────────────────────────────")
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	// Simple capitalization
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}