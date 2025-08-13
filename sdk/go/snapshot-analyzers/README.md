# Code Snapshot Analyzer Demo

A real-time collaborative code analysis system demonstrating the Switchboard Go SDK V2 with automated code review agents.

## 🎯 Overview

This demo showcases:
- **2 Student Clients**: Periodically send code snapshots (every 15 seconds)
- **2 Agent Clients**: Analyze code in real-time and provide feedback
- **Live Communication**: Students broadcast code, agents respond with analysis

## 🏗️ Architecture

### Component Design
```
Student Clients (Role: "student")
├── Alice (student_alice) → Sends Go function implementations
└── Bob (student_bob)     → Sends algorithm implementations

Agent Clients (Role: "instructor") 
├── Orchestrator → Session management, coordination, status updates
├── Syntax Agent → Style, formatting, naming conventions
└── Logic Agent  → Bug detection, logic errors, performance
```

### Communication Flow
```
1. Students → broadcast_to_instructors (context: "submission")
2. Agents   → direct_message to student (context: "response")
```

### Message Protocol

**Important**: Uses valid Switchboard contexts (`submission` for code, `response` for feedback) to ensure proper message validation.

#### Code Snapshot Message
```go
{
    "context": "submission",
    "type": "code_snapshot",
    "student_id": "alice",
    "code": "func Calculate(x int) int {\n    return x * 2\n}",
    "language": "go",
    "file_name": "calculator.go",
    "timestamp": "2025-01-15T10:30:00Z",
    "exercise_id": "basic_functions"
}
```

#### Analysis Feedback Message
```go
{
    "context": "response",
    "agent_type": "syntax",
    "student_id": "alice",
    "analysis": {
        "score": 85,
        "issues": [
            {
                "type": "style",
                "severity": "minor",
                "line": 1,
                "message": "Consider adding a comment explaining the function purpose",
                "suggestion": "// Calculate doubles the input value\nfunc Calculate(x int) int {"
            }
        ],
        "positive_feedback": [
            "Good function naming convention",
            "Proper parameter naming"
        ]
    }
}
```

## 🚀 Quick Start

### Prerequisites
```bash
# Start Switchboard server in main project
cd /path/to/switchboard
make run

# Verify server is running
curl http://localhost:8080/api/status
```

### Run Complete Demo
```bash
cd sdk/go/snapshot-analyzers
chmod +x demo/run_demo.sh
./demo/run_demo.sh
```

### Run Individual Components
```bash
# Start agents first
go run agents/syntax_agent.go
go run agents/logic_agent.go

# Start students (in separate terminals)
go run students/student_alice.go
go run students/student_bob.go

# Start orchestrator
go run agents/orchestrator.go
```

## 📁 Project Structure

```
snapshot-analyzers/
├── README.md                 # This documentation
├── go.mod                    # Go module definition
├── students/
│   ├── student_alice.go      # Student client "Alice"
│   └── student_bob.go        # Student client "Bob"  
├── agents/
│   ├── orchestrator.go       # Session management & coordination
│   ├── syntax_agent.go       # Style & formatting analyzer
│   └── logic_agent.go        # Logic & bug detection analyzer
├── shared/
│   ├── code_samples.go       # Sample code snippets for students
│   └── analysis.go           # Common analysis utilities
└── demo/
    └── run_demo.sh          # Automated demo runner
```

## 👩‍💻 Student Clients

### Alice (student_alice.go)
- **Focus**: Basic Go functions and data structures
- **Code Types**: Functions, structs, interfaces
- **Sending Pattern**: Every 15 seconds, cycles through sample functions
- **Sample Code**: Calculator functions, string utilities, data structures

### Bob (student_bob.go)  
- **Focus**: Algorithms and complex logic
- **Code Types**: Sorting algorithms, search functions, data processing
- **Sending Pattern**: Every 15 seconds (offset by 7.5s from Alice)
- **Sample Code**: Bubble sort, binary search, recursive functions

## 🤖 Agent Clients

### Syntax Agent (syntax_agent.go)
Analyzes code style and formatting:
- **Function naming** (camelCase, descriptive names)
- **Variable naming** (meaningful, not single letters)  
- **Comments** (function documentation, inline comments)
- **Code formatting** (proper indentation, spacing)
- **Go conventions** (exported vs unexported, receiver naming)

#### Analysis Rules
```go
// Good ✅
func CalculateTotal(items []Item) int {
    // Calculate the sum of all item prices
    total := 0
    for _, item := range items {
        total += item.Price
    }
    return total
}

// Issues ❌  
func calc(x []int) int {
    t := 0
    for i := 0; i < len(x); i++ {
        t = t + x[i]
    }
    return t
}
```

### Logic Agent (logic_agent.go)
Detects bugs and logic issues:
- **Infinite loops** (missing increment, wrong conditions)
- **Null pointer access** (missing nil checks)
- **Array bounds** (index out of range)
- **Logic errors** (off-by-one, wrong operators)
- **Performance issues** (inefficient algorithms, unnecessary operations)

#### Detection Patterns
```go
// Bug Detection Examples
func BuggyFunction() {
    // Infinite loop detection
    for i := 0; i > 0; i++ { } // Will never terminate
    
    // Nil pointer access
    var slice []int
    fmt.Println(slice[0]) // Panic: index out of range
    
    // Off-by-one error
    arr := [5]int{1,2,3,4,5}
    for i := 0; i <= len(arr); i++ { } // Will panic
}
```

## 📊 Demo Scenarios

### Scenario 1: Perfect Code
**Alice sends well-formatted function**
```go
// CalculateCircleArea calculates the area of a circle given its radius
func CalculateCircleArea(radius float64) float64 {
    if radius < 0 {
        return 0
    }
    return math.Pi * radius * radius
}
```
**Expected Feedback**:
- Syntax Agent: ✅ "Excellent code style and documentation"
- Logic Agent: ✅ "Good input validation and logic"

### Scenario 2: Style Issues
**Bob sends poorly formatted code**
```go
func calc(x,y int)int{
return x+y
}
```
**Expected Feedback**:
- Syntax Agent: ⚠️ "Add function comments, improve spacing, use descriptive names"
- Logic Agent: ✅ "Logic is correct"

### Scenario 3: Logic Bug
**Alice sends buggy loop**
```go
func PrintNumbers(n int) {
    for i := 0; i < n; i-- {
        fmt.Println(i)
    }
}
```
**Expected Feedback**:
- Syntax Agent: ✅ "Good formatting"
- Logic Agent: 🚨 "Infinite loop detected: increment should be i++"

### Scenario 4: Performance Issue
**Bob sends inefficient algorithm**
```go
func FindMax(arr []int) int {
    max := arr[0]
    for i := 0; i < len(arr); i++ {
        for j := 0; j < len(arr); j++ {
            if arr[j] > max {
                max = arr[j]
            }
        }
    }
    return max
}
```
**Expected Feedback**:
- Syntax Agent: ✅ "Good naming and structure"
- Logic Agent: ⚠️ "Nested loop is inefficient O(n²), use single pass O(n)"

## 🔧 Configuration

### Rate Limiting
- **Students**: 4 messages/minute (every 15 seconds)
- **Agents**: Reactive only (respond to student messages)
- **Total Load**: ~16 messages/minute (well under 100/minute limit)

### Message Size
- **Code Snapshots**: ~2KB typical (well under 64KB limit)
- **Analysis Feedback**: ~1KB typical

### Timing
- **Alice**: Sends at 0s, 15s, 30s, 45s...
- **Bob**: Sends at 7.5s, 22.5s, 37.5s, 52.5s...
- **Staggered Timing**: Prevents message bursts

## 🧪 Testing

### Unit Tests
```bash
cd shared && go test -v
```
Tests the analysis functions and code sample generators.

### Integration Test
Run individual components manually to test specific scenarios.

### Full Demo Test
```bash
./demo/run_demo.sh --test
```
Automated test with predefined scenarios and expected outcomes.

## 🎮 Manual Testing

### Send Custom Code
Modify the code samples in `shared/code_samples.go` to test different scenarios:
1. Edit Alice or Bob's code samples
2. Restart the student clients
3. Observe agent analysis responses

### Monitor All Traffic
Watch the console output from each component to see real-time message processing and analysis results.

## 📈 Performance Metrics

The demo tracks:
- **Message Latency**: Time from code send to feedback received
- **Analysis Quality**: Accuracy of bug detection and style suggestions  
- **Throughput**: Messages processed per minute
- **Connection Stability**: Reconnection events and reliability

## 🚨 Error Handling

### Student Client Errors
- **No Active Session**: Queue messages until session starts
- **Rate Limit**: Skip sending cycle, resume at next interval
- **Connection Lost**: Auto-reconnect with exponential backoff

### Agent Client Errors  
- **Analysis Failure**: Send error message to student
- **Parse Error**: Request code resend with error details
- **Overload**: Prioritize by message timestamp (FIFO)

## 🔮 Future Enhancements

### Advanced Analysis
- **AST Parsing**: Full Go syntax tree analysis
- **Complexity Metrics**: Cyclomatic complexity scoring
- **Security Scanning**: Vulnerability detection
- **Performance Profiling**: Memory and CPU usage analysis

### Interactive Features
- **Code Challenges**: Automated problem assignment
- **Peer Review**: Student-to-student code review
- **Progress Tracking**: Learning analytics and improvement metrics
- **Real-time Collaboration**: Shared code editing sessions

### Multi-Language Support
- **Python**: Add Python code analysis agents
- **JavaScript**: Web development focused analysis
- **C++**: Systems programming analysis
- **Language Detection**: Automatic language identification

## 🎯 Learning Objectives

This demo demonstrates:
1. **Real-time Communication**: WebSocket messaging patterns
2. **Role-based Systems**: Student/teacher interaction models
3. **Event-driven Architecture**: Message routing and handling
4. **Automated Code Review**: Static analysis and feedback systems
5. **Educational Technology**: Interactive learning tools

Perfect for understanding how to build real-time collaborative development environments using Switchboard's educational communication patterns.

---

## 🚀 Ready to Start?

```bash
# Clone and run the demo
cd sdk/go/snapshot-analyzers
./demo/run_demo.sh

# Watch the magic happen! ✨
```