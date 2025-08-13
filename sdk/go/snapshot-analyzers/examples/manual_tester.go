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
	"github.com/vtphan/switchboard/sdk/go/snapshot-analyzers/shared"
)

func main() {
	fmt.Println("🧪 Manual Testing Interface for Code Snapshot Analyzer")
	fmt.Println("═══════════════════════════════════════════════════════")
	
	// Check command line arguments
	args := os.Args[1:]
	
	if len(args) > 0 && args[0] == "--monitor" {
		startMonitorMode()
		return
	}
	
	if len(args) > 0 && args[0] == "--demo-scenarios" {
		runDemoScenarios()
		return
	}
	
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		showHelp()
		return
	}
	
	startInteractiveMode()
}

func showHelp() {
	fmt.Println("Manual Testing Interface - Usage:")
	fmt.Println("")
	fmt.Println("  go run manual_test.go                # Interactive mode")
	fmt.Println("  go run manual_test.go --monitor      # Monitor all traffic")
	fmt.Println("  go run manual_test.go --demo-scenarios # Run demo scenarios")
	fmt.Println("  go run manual_test.go --help         # Show this help")
	fmt.Println("")
	fmt.Println("Interactive Mode:")
	fmt.Println("  • Choose student identity (alice/bob)")
	fmt.Println("  • Enter custom Go code")
	fmt.Println("  • Send to analysis agents")
	fmt.Println("  • View real-time feedback")
	fmt.Println("")
	fmt.Println("Monitor Mode:")
	fmt.Println("  • Observe all demo traffic without sending")
	fmt.Println("  • See code snapshots and analysis feedback")
	fmt.Println("  • Real-time message monitoring")
}

func startInteractiveMode() {
	fmt.Println("Choose your role:")
	fmt.Println("1. Send custom code as Alice (basic functions)")
	fmt.Println("2. Send custom code as Bob (algorithms)")
	fmt.Println("3. Send predefined demo scenarios")
	fmt.Print("\nEnter choice (1-3): ")
	
	scanner := bufio.NewScanner(os.Stdin)
	var choice string
	if scanner.Scan() {
		choice = strings.TrimSpace(scanner.Text())
	}
	
	switch choice {
	case "1":
		startCustomStudentMode("alice")
	case "2":
		startCustomStudentMode("bob")
	case "3":
		runDemoScenarios()
	default:
		fmt.Println("Invalid choice. Starting as Alice...")
		startCustomStudentMode("alice")
	}
}

func startCustomStudentMode(studentID string) {
	fmt.Printf("🎓 Starting interactive mode as %s\n", studentID)
	fmt.Println("You can send custom Go code to the analysis agents")
	
	// Create client
	client, err := switchboard.NewClient(switchboard.Config{
		UserID: fmt.Sprintf("manual_%s", studentID),
		Role:   switchboard.RoleStudent,
		WsURL:  "ws://localhost:8080/ws",
		
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
				fmt.Println("⏸️  No active session")
			}
		},
	})
	
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	
	// Connect
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Interactive loop
	go interactiveLoop(client, studentID)
	
	fmt.Println("🚀 Ready! Type your Go code and press Enter twice to send.")
	fmt.Println("Type 'quit' to exit, 'help' for commands.")
	fmt.Println("════════════════════════════════════════")
	
	<-sigChan
	fmt.Println("\n👋 Exiting manual test mode...")
	client.Disconnect()
}

func interactiveLoop(client *switchboard.Client, studentID string) {
	scanner := bufio.NewScanner(os.Stdin)
	
	for {
		fmt.Print("\n📝 Enter Go code (or command): ")
		
		var lines []string
		for scanner.Scan() {
			line := scanner.Text()
			
			// Handle commands
			if len(lines) == 0 {
				switch strings.ToLower(strings.TrimSpace(line)) {
				case "quit", "exit":
					os.Exit(0)
				case "help":
					showInteractiveHelp()
					break
				case "samples":
					showSampleCode(studentID)
					break
				case "scenarios":
					sendDemoScenario(client, studentID)
					break
				}
			}
			
			// Stop collecting when we hit an empty line
			if strings.TrimSpace(line) == "" && len(lines) > 0 {
				break
			}
			
			lines = append(lines, line)
		}
		
		if len(lines) > 0 {
			code := strings.Join(lines, "\n")
			if strings.TrimSpace(code) != "" {
				sendCustomCode(client, studentID, code)
			}
		}
	}
}

func sendCustomCode(client *switchboard.Client, studentID string, code string) {
	if !client.IsSessionActive() {
		fmt.Println("⏳ No active session. Waiting for session to start...")
		return
	}
	
	// Create code sample
	sample := shared.CodeSample{
		Code:       code,
		Language:   "go",
		FileName:   "manual_test.go",
		ExerciseID: "manual_input",
		Metadata:   map[string]string{"source": "manual_test"},
	}
	
	// Send to agents
	message := shared.GenerateCodeSnapshotMessage(studentID, sample)
	
	err := client.BroadcastToInstructors(message)
	if err != nil {
		fmt.Printf("❌ Failed to send code: %v\n", err)
		return
	}
	
	fmt.Printf("📤 Sent custom code to analysis agents\n")
	fmt.Printf("   Lines: %d\n", len(strings.Split(code, "\n")))
	fmt.Printf("   ⏱️  Waiting for analysis...\n")
}

func sendDemoScenario(client *switchboard.Client, studentID string) {
	fmt.Println("\nAvailable demo scenarios:")
	scenarios := []string{"perfect_code", "style_issues", "logic_bug", "performance_issue"}
	
	for i, scenario := range scenarios {
		fmt.Printf("%d. %s\n", i+1, strings.Replace(scenario, "_", " ", -1))
	}
	
	fmt.Print("Choose scenario (1-4): ")
	scanner := bufio.NewScanner(os.Stdin)
	var choice string
	if scanner.Scan() {
		choice = strings.TrimSpace(scanner.Text())
	}
	
	var scenarioKey string
	switch choice {
	case "1":
		scenarioKey = "perfect_code"
	case "2":
		scenarioKey = "style_issues"
	case "3":
		scenarioKey = "logic_bug"
	case "4":
		scenarioKey = "performance_issue"
	default:
		fmt.Println("Invalid choice")
		return
	}
	
	if sample, exists := shared.DemoScenarios[scenarioKey]; exists {
		message := shared.GenerateCodeSnapshotMessage(studentID, sample)
		
		err := client.BroadcastToInstructors(message)
		if err != nil {
			fmt.Printf("❌ Failed to send scenario: %v\n", err)
			return
		}
		
		fmt.Printf("📤 Sent demo scenario: %s\n", scenarioKey)
		fmt.Printf("   Expected: %s\n", sample.Metadata["expected"])
	}
}

func showInteractiveHelp() {
	fmt.Println("\n🆘 Interactive Mode Commands:")
	fmt.Println("  help      - Show this help")
	fmt.Println("  samples   - Show example code for your student type")
	fmt.Println("  scenarios - Send a predefined demo scenario")
	fmt.Println("  quit      - Exit the program")
	fmt.Println("")
	fmt.Println("📝 To send code:")
	fmt.Println("  1. Type or paste your Go code")
	fmt.Println("  2. Press Enter twice to send")
	fmt.Println("  3. Wait for analysis feedback")
}

func showSampleCode(studentID string) {
	fmt.Printf("\n📚 Sample code for %s:\n", studentID)
	
	var profile shared.StudentProfile
	switch studentID {
	case "alice":
		profile = shared.AliceProfile
	case "bob":
		profile = shared.BobProfile
	default:
		fmt.Println("No samples available for this student")
		return
	}
	
	for i, sample := range profile.CodeSamples {
		fmt.Printf("\n%d. %s (%s)\n", i+1, sample.FileName, sample.ExerciseID)
		lines := strings.Split(sample.Code, "\n")
		if len(lines) > 3 {
			for _, line := range lines[:3] {
				fmt.Printf("   %s\n", line)
			}
			fmt.Printf("   ... (%d more lines)\n", len(lines)-3)
		} else {
			for _, line := range lines {
				fmt.Printf("   %s\n", line)
			}
		}
	}
}

func startMonitorMode() {
	fmt.Println("🔍 Monitor Mode - Observing all demo traffic")
	fmt.Println("This will show code snapshots and analysis feedback")
	
	// Create monitor client (instructor role to see everything)
	client, err := switchboard.NewClient(switchboard.Config{
		UserID: "monitor_client",
		Role:   switchboard.RoleInstructor,
		WsURL:  "ws://localhost:8080/ws",
		
		OnBroadcastToInstructors: func(msg *switchboard.Message) {
			handleCodeSnapshotMonitor(msg)
		},
		
		OnDirectMessage: func(msg *switchboard.Message) {
			handleFeedbackMonitor(msg)
		},
		
		OnBroadcastToStudents: func(msg *switchboard.Message) {
			if content, ok := msg.Content["text"].(string); ok {
				fmt.Printf("📢 [INSTRUCTOR BROADCAST] %s\n", content)
			}
		},
		
		OnConnectionChange: func(state switchboard.ConnectionState, err error) {
			fmt.Printf("🔗 Monitor connection: %s\n", state)
		},
		
		OnSessionChange: func(session *switchboard.Session) {
			if session.Active {
				fmt.Printf("📝 [SESSION ACTIVE] %s\n", session.Name)
			} else {
				fmt.Println("⏸️  [SESSION INACTIVE]")
			}
		},
	})
	
	if err != nil {
		log.Fatalf("Failed to create monitor: %v", err)
	}
	
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	
	fmt.Println("🚀 Monitor active! Press Ctrl+C to stop.")
	fmt.Println("════════════════════════════════════════")
	
	// Wait for shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	fmt.Println("\n👋 Monitor shutting down...")
	client.Disconnect()
}

func handleCodeSnapshotMonitor(msg *switchboard.Message) {
	if context, ok := msg.Content["context"].(string); ok && context == "code_snapshot" {
		studentID, _ := msg.Content["student_id"].(string)
		fileName, _ := msg.Content["file_name"].(string)
		exerciseID, _ := msg.Content["exercise_id"].(string)
		code, _ := msg.Content["code"].(string)
		
		fmt.Printf("📥 [CODE SNAPSHOT] %s → %s (%s)\n", studentID, fileName, exerciseID)
		
		// Show first line of code
		lines := strings.Split(code, "\n")
		if len(lines) > 0 {
			fmt.Printf("   Code: %s", lines[0])
			if len(lines) > 1 {
				fmt.Printf("... (%d lines total)\n", len(lines))
			} else {
				fmt.Println()
			}
		}
	}
}

func handleFeedbackMonitor(msg *switchboard.Message) {
	if context, ok := msg.Content["context"].(string); ok && context == "analysis_feedback" {
		agentType, _ := msg.Content["agent_type"].(string)
		studentID, _ := msg.Content["student_id"].(string)
		
		if analysisData, ok := msg.Content["analysis"].(map[string]interface{}); ok {
			score, _ := analysisData["score"].(float64)
			issuesCount := 0
			if issues, ok := analysisData["issues"].([]interface{}); ok {
				issuesCount = len(issues)
			}
			
			fmt.Printf("📤 [ANALYSIS] %s → %s (Score: %.0f, Issues: %d)\n", 
				agentType, studentID, score, issuesCount)
		}
	}
}

func runDemoScenarios() {
	fmt.Println("🎬 Running Demo Scenarios")
	fmt.Println("This will send predefined scenarios to test the analysis agents")
	
	// Create client
	client, err := switchboard.NewClient(switchboard.Config{
		UserID: "scenario_runner",
		Role:   switchboard.RoleStudent,
		WsURL:  "ws://localhost:8080/ws",
		
		OnDirectMessage: func(msg *switchboard.Message) {
			handleAnalysisFeedback(msg)
		},
		
		OnConnectionChange: func(state switchboard.ConnectionState, err error) {
			fmt.Printf("🔗 Connection: %s\n", state)
		},
		
		OnSessionChange: func(session *switchboard.Session) {
			if session.Active {
				fmt.Printf("📝 Session: %s\n", session.Name)
			}
		},
	})
	
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	
	// Wait for session
	fmt.Println("⏳ Waiting for active session...")
	for !client.IsSessionActive() {
		time.Sleep(1 * time.Second)
	}
	
	fmt.Println("🚀 Running demo scenarios...")
	
	scenarios := []string{"perfect_code", "style_issues", "logic_bug", "performance_issue"}
	
	for i, scenarioKey := range scenarios {
		fmt.Printf("\n🎯 Scenario %d: %s\n", i+1, strings.Replace(scenarioKey, "_", " ", -1))
		
		if sample, exists := shared.DemoScenarios[scenarioKey]; exists {
			message := shared.GenerateCodeSnapshotMessage("scenario_runner", sample)
			
			err := client.BroadcastToInstructors(message)
			if err != nil {
				fmt.Printf("❌ Failed to send scenario: %v\n", err)
				continue
			}
			
			fmt.Printf("📤 Sent to agents, waiting for feedback...\n")
			time.Sleep(5 * time.Second) // Wait for analysis
		}
	}
	
	fmt.Println("\n✅ All demo scenarios completed!")
	client.Disconnect()
}

func handleAnalysisFeedback(msg *switchboard.Message) {
	if analysisData, ok := msg.Content["analysis"].(map[string]interface{}); ok {
		agentType, _ := analysisData["agent_type"].(string)
		score, _ := analysisData["score"].(float64)
		summary, _ := analysisData["summary"].(string)
		
		fmt.Printf("\n🤖 %s Analysis (Score: %.0f/100)\n", 
			strings.ToUpper(agentType), score)
		fmt.Printf("   %s\n", summary)
		
		if issues, ok := analysisData["issues"].([]interface{}); ok && len(issues) > 0 {
			fmt.Printf("   Issues (%d):\n", len(issues))
			for i, issueData := range issues {
				if issue, ok := issueData.(map[string]interface{}); ok {
					severity, _ := issue["severity"].(string)
					message, _ := issue["message"].(string)
					fmt.Printf("     • [%s] %s\n", strings.ToUpper(severity), message)
				}
				if i >= 2 { // Limit output
					fmt.Printf("     ... and %d more\n", len(issues)-i-1)
					break
				}
			}
		}
		
		fmt.Println("────────────────────────────────────────")
	}
}