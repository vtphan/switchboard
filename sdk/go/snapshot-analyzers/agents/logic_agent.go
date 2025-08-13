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
	fmt.Println("🔍 Logic Analysis Agent Starting...")
	fmt.Println("Specialization: Bug detection, logic errors, and performance analysis")
	
	// Initialize the logic analysis engine
	logicEngine := shared.NewLogicAnalysisEngine()
	
	// Create the agent client (instructor role)
	var client *switchboard.Client
	client, err := switchboard.NewClient(switchboard.Config{
		UserID: "logic_agent",
		Role:   switchboard.RoleInstructor,
		WsURL:  "ws://localhost:8080/ws",
		
		// Listen for student code snapshots
		OnBroadcastToInstructors: func(msg *switchboard.Message) {
			handleCodeSnapshot(client, logicEngine, msg)
		},
		
		OnConnectionChange: func(state switchboard.ConnectionState, err error) {
			switch state {
			case switchboard.ConnectionStateConnected:
				fmt.Println("✅ Logic Agent connected to Switchboard")
			case switchboard.ConnectionStateDisconnected:
				fmt.Println("❌ Logic Agent disconnected")
			case switchboard.ConnectionStateError:
				fmt.Printf("🚨 Connection error: %v\n", err)
			}
		},
		
		OnSessionChange: func(session *switchboard.Session) {
			if session.Active {
				fmt.Printf("📝 Monitoring session: %s\n", session.Name)
				fmt.Println("🔍 Ready to analyze logic and detect bugs...")
			} else {
				fmt.Println("⏸️  Session ended - waiting for next session")
			}
		},
	})
	
	if err != nil {
		log.Fatalf("Failed to create logic agent: %v", err)
	}
	
	// Connect to the server
	fmt.Println("🔌 Connecting to Switchboard...")
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	
	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	fmt.Println("🚀 Logic Agent is active! Press Ctrl+C to stop.")
	fmt.Println("════════════════════════════════════════")
	
	<-sigChan
	fmt.Println("\n👋 Logic Agent shutting down...")
	client.Disconnect()
}

func handleCodeSnapshot(client *switchboard.Client, engine *shared.LogicAnalysisEngine, msg *switchboard.Message) {
	// Force debug output to stdout (not logs)
	fmt.Fprintf(os.Stdout, "🔔 [LOGIC AGENT] Received message from %s\n", msg.FromUser)
	fmt.Fprintf(os.Stdout, "🔔 [LOGIC AGENT] Message content keys: %v\n", getKeys(msg.Content))
	fmt.Fprintf(os.Stdout, "🔔 [LOGIC AGENT] Full message: %+v\n", msg)
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
	
	fmt.Printf("📥 Analyzing logic for %s (%s)\n", studentID, fileName)
	fmt.Printf("   Exercise: %s\n", exerciseID)
	
	// Perform logic analysis
	startTime := time.Now()
	result := engine.AnalyzeCode(code)
	analysisTime := time.Since(startTime)
	
	// Log analysis summary with detailed breakdown
	fmt.Printf("   ⚡ Deep analysis completed in %v\n", analysisTime)
	fmt.Printf("   📊 Logic Score: %d/100\n", result.Score)
	
	// Categorize and display issues
	criticalIssues := 0
	majorIssues := 0
	minorIssues := 0
	
	for _, issue := range result.Issues {
		switch issue.Severity {
		case "critical":
			criticalIssues++
		case "major":
			majorIssues++
		case "minor":
			minorIssues++
		}
	}
	
	fmt.Printf("   🔍 Issue breakdown: %d critical, %d major, %d minor\n", 
		criticalIssues, majorIssues, minorIssues)
	
	// Display critical issues with detailed analysis
	if criticalIssues > 0 {
		fmt.Printf("   🚨 CRITICAL BUGS DETECTED:\n")
		for _, issue := range result.Issues {
			if issue.Severity == "critical" {
				fmt.Printf("     🔥 [%s] Line %d: %s\n", 
					issue.Type, issue.Line, issue.Message)
				if issue.Suggestion != "" {
					fmt.Printf("        💡 Fix: %s\n", issue.Suggestion)
				}
			}
		}
	}
	
	// Display performance issues
	performanceIssues := 0
	for _, issue := range result.Issues {
		if issue.Type == "performance" {
			performanceIssues++
		}
	}
	
	if performanceIssues > 0 {
		fmt.Printf("   ⚡ PERFORMANCE ISSUES: %d optimization opportunities\n", performanceIssues)
		for _, issue := range result.Issues {
			if issue.Type == "performance" {
				fmt.Printf("     🚀 %s\n", issue.Message)
			}
		}
	}
	
	// Display positive algorithm analysis
	if len(result.PositiveFeedback) > 0 {
		fmt.Printf("   ✅ Algorithm strengths:\n")
		for _, feedback := range result.PositiveFeedback {
			fmt.Printf("     • %s\n", feedback)
		}
	}
	
	// Special analysis for different students
	studentSpecificAnalysis(studentID, result, code)
	
	// Create feedback message
	feedbackMsg := shared.GenerateAnalysisFeedbackMessage(studentID, result)
	
	// Send feedback directly to the student
	err := client.DirectMessage(studentID, feedbackMsg)
	if err != nil {
		fmt.Printf("❌ Failed to send feedback to %s: %v\n", studentID, err)
	} else {
		priority := "normal"
		if criticalIssues > 0 {
			priority = "urgent"
		} else if majorIssues > 2 {
			priority = "high"
		}
		fmt.Printf("📤 Sent logic feedback to %s (priority: %s)\n", studentID, priority)
	}
	
	fmt.Println("────────────────────────────────────────")
}

func studentSpecificAnalysis(studentID string, result shared.AnalysisResult, code string) {
	switch studentID {
	case "alice":
		// Alice focuses on basic functions - provide educational feedback
		if result.Score >= 80 {
			fmt.Printf("   🎓 Alice is showing good progress with function basics!\n")
		} else {
			fmt.Printf("   📚 Alice needs guidance on fundamental programming concepts\n")
		}
		
	case "bob":
		// Bob works on algorithms - provide more technical analysis
		complexity := analyzeAlgorithmComplexity(code)
		fmt.Printf("   🧠 Algorithm complexity analysis: %s\n", complexity)
		
		if containsRecursion(code) {
			fmt.Printf("   🔄 Recursive algorithm detected - checking base cases\n")
		}
		
		if result.Score >= 85 {
			fmt.Printf("   🏆 Bob demonstrates strong algorithmic thinking!\n")
		} else if result.Score >= 70 {
			fmt.Printf("   📈 Bob's algorithms are improving - focus on edge cases\n")
		} else {
			fmt.Printf("   🔧 Bob needs to review algorithm fundamentals\n")
		}
		
	default:
		fmt.Printf("   👤 General analysis for %s completed\n", studentID)
	}
}

func analyzeAlgorithmComplexity(code string) string {
	// Simple heuristic-based complexity analysis
	if containsNestedLoops(code) {
		if containsTripleNestedLoops(code) {
			return "O(n³) - Consider optimization"
		}
		return "O(n²) - Quadratic time"
	} else if containsLoops(code) {
		return "O(n) - Linear time"
	} else if containsRecursion(code) {
		return "O(log n) or O(n) - Depends on recursion depth"
	} else {
		return "O(1) - Constant time"
	}
}

func containsLoops(code string) bool {
	return contains(code, "for ") || contains(code, "while ")
}

func containsNestedLoops(code string) bool {
	// Count number of for loops
	forCount := 0
	i := 0
	for i < len(code)-3 {
		if code[i:i+4] == "for " {
			forCount++
		}
		i++
	}
	return forCount >= 2
}

func containsTripleNestedLoops(code string) bool {
	// Count number of for loops
	forCount := 0
	i := 0
	for i < len(code)-3 {
		if code[i:i+4] == "for " {
			forCount++
		}
		i++
	}
	return forCount >= 3
}

func containsRecursion(code string) bool {
	// Look for function name appearing in function body
	lines := splitLines(code)
	var functionName string
	
	// Find function name
	for _, line := range lines {
		if contains(line, "func ") {
			start := indexOf(line, "func ") + 5
			end := indexOf(line[start:], "(")
			if end > 0 {
				functionName = line[start : start+end]
				break
			}
		}
	}
	
	// Check if function name appears again in the body
	if functionName != "" {
		for i, line := range lines {
			if i > 0 && contains(line, functionName+"(") {
				return true
			}
		}
	}
	
	return false
}

func splitLines(code string) []string {
	lines := make([]string, 0)
	currentLine := ""
	
	for _, char := range code {
		if char == '\n' {
			lines = append(lines, currentLine)
			currentLine = ""
		} else {
			currentLine += string(char)
		}
	}
	
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	
	return lines
}

func contains(s, substr string) bool {
	return indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}