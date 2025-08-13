package shared

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// AnalysisResult represents the result of code analysis
type AnalysisResult struct {
	Score            int                    `json:"score"`
	Issues           []Issue                `json:"issues"`
	PositiveFeedback []string               `json:"positive_feedback"`
	Summary          string                 `json:"summary"`
	AgentType        string                 `json:"agent_type"`
	Timestamp        time.Time              `json:"timestamp"`
	ProcessingTime   time.Duration          `json:"processing_time"`
}

// Issue represents a specific code issue found during analysis
type Issue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`  // "minor", "major", "critical"
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Message     string `json:"message"`
	Suggestion  string `json:"suggestion,omitempty"`
	RuleID      string `json:"rule_id"`
}

// SeverityLevel defines the severity of issues
type SeverityLevel string

const (
	SeverityMinor    SeverityLevel = "minor"
	SeverityMajor    SeverityLevel = "major"
	SeverityCritical SeverityLevel = "critical"
)

// AnalysisRule represents a rule for code analysis
type AnalysisRule struct {
	ID          string
	Name        string
	Description string
	Pattern     *regexp.Regexp
	Severity    SeverityLevel
	Category    string
	Checker     func(code string) []Issue
}

// SyntaxAnalysisEngine performs style and formatting analysis
type SyntaxAnalysisEngine struct {
	Rules []AnalysisRule
}

// LogicAnalysisEngine performs bug detection and logic analysis
type LogicAnalysisEngine struct {
	Rules []AnalysisRule
}

// NewSyntaxAnalysisEngine creates a new syntax analysis engine
func NewSyntaxAnalysisEngine() *SyntaxAnalysisEngine {
	return &SyntaxAnalysisEngine{
		Rules: []AnalysisRule{
			{
				ID:          "function_comments",
				Name:        "Function Documentation",
				Description: "Functions should have comments explaining their purpose",
				Pattern:     regexp.MustCompile(`func\s+[A-Z][a-zA-Z0-9]*\s*\(`),
				Severity:    SeverityMinor,
				Category:    "documentation",
				Checker:     checkFunctionComments,
			},
			{
				ID:          "function_naming",
				Name:        "Function Naming Convention",
				Description: "Function names should be descriptive and follow Go conventions",
				Severity:    SeverityMajor,
				Category:    "naming",
				Checker:     checkFunctionNaming,
			},
			{
				ID:          "variable_naming",
				Name:        "Variable Naming Convention",
				Description: "Variables should have meaningful names",
				Severity:    SeverityMajor,
				Category:    "naming",
				Checker:     checkVariableNaming,
			},
			{
				ID:          "code_formatting",
				Name:        "Code Formatting",
				Description: "Code should be properly formatted with appropriate spacing",
				Severity:    SeverityMinor,
				Category:    "formatting",
				Checker:     checkCodeFormatting,
			},
		},
	}
}

// NewLogicAnalysisEngine creates a new logic analysis engine
func NewLogicAnalysisEngine() *LogicAnalysisEngine {
	return &LogicAnalysisEngine{
		Rules: []AnalysisRule{
			{
				ID:          "infinite_loops",
				Name:        "Infinite Loop Detection",
				Description: "Detect loops that may run infinitely",
				Severity:    SeverityCritical,
				Category:    "logic",
				Checker:     checkInfiniteLoops,
			},
			{
				ID:          "array_bounds",
				Name:        "Array Bounds Checking",
				Description: "Check for potential array index out of bounds",
				Severity:    SeverityCritical,
				Category:    "safety",
				Checker:     checkArrayBounds,
			},
			{
				ID:          "nil_pointer",
				Name:        "Nil Pointer Access",
				Description: "Check for potential nil pointer dereference",
				Severity:    SeverityCritical,
				Category:    "safety",
				Checker:     checkNilPointer,
			},
			{
				ID:          "performance_issues",
				Name:        "Performance Optimization",
				Description: "Identify potential performance bottlenecks",
				Severity:    SeverityMajor,
				Category:    "performance",
				Checker:     checkPerformanceIssues,
			},
		},
	}
}

// AnalyzeCode performs syntax analysis on the provided code
func (e *SyntaxAnalysisEngine) AnalyzeCode(code string) AnalysisResult {
	start := time.Now()
	var issues []Issue
	var positiveFeedback []string
	
	// Run all syntax rules
	for _, rule := range e.Rules {
		ruleIssues := rule.Checker(code)
		issues = append(issues, ruleIssues...)
	}
	
	// Generate positive feedback
	if hasGoodNaming(code) {
		positiveFeedback = append(positiveFeedback, "Good function and variable naming conventions")
	}
	if hasProperFormatting(code) {
		positiveFeedback = append(positiveFeedback, "Well-formatted and readable code structure")
	}
	if hasGoodComments(code) {
		positiveFeedback = append(positiveFeedback, "Good use of comments for documentation")
	}
	
	// Calculate score based on issues
	score := calculateSyntaxScore(issues)
	
	return AnalysisResult{
		Score:            score,
		Issues:           issues,
		PositiveFeedback: positiveFeedback,
		Summary:          generateSyntaxSummary(score, len(issues)),
		AgentType:        "syntax",
		Timestamp:        time.Now(),
		ProcessingTime:   time.Since(start),
	}
}

// AnalyzeCode performs logic analysis on the provided code
func (e *LogicAnalysisEngine) AnalyzeCode(code string) AnalysisResult {
	start := time.Now()
	var issues []Issue
	var positiveFeedback []string
	
	// Run all logic rules
	for _, rule := range e.Rules {
		ruleIssues := rule.Checker(code)
		issues = append(issues, ruleIssues...)
	}
	
	// Generate positive feedback
	if hasGoodErrorHandling(code) {
		positiveFeedback = append(positiveFeedback, "Good error handling and input validation")
	}
	if hasEfficientAlgorithms(code) {
		positiveFeedback = append(positiveFeedback, "Efficient algorithm implementation")
	}
	if hasProperLoopLogic(code) {
		positiveFeedback = append(positiveFeedback, "Correct loop logic and termination conditions")
	}
	
	// Calculate score based on issues
	score := calculateLogicScore(issues)
	
	return AnalysisResult{
		Score:            score,
		Issues:           issues,
		PositiveFeedback: positiveFeedback,
		Summary:          generateLogicSummary(score, len(issues)),
		AgentType:        "logic",
		Timestamp:        time.Now(),
		ProcessingTime:   time.Since(start),
	}
}

// Individual checker functions

func checkFunctionComments(code string) []Issue {
	var issues []Issue
	lines := strings.Split(code, "\n")
	
	funcPattern := regexp.MustCompile(`func\s+([A-Z][a-zA-Z0-9]*)\s*\(`)
	commentPattern := regexp.MustCompile(`^\s*//`)
	
	for i, line := range lines {
		if funcPattern.MatchString(line) {
			// Check if previous line(s) contain comments
			hasComment := false
			for j := i - 1; j >= 0 && j >= i-3; j-- {
				if strings.TrimSpace(lines[j]) == "" {
					continue
				}
				if commentPattern.MatchString(lines[j]) {
					hasComment = true
					break
				}
				break
			}
			
			if !hasComment {
				funcName := funcPattern.FindStringSubmatch(line)[1]
				issues = append(issues, Issue{
					Type:       "documentation",
					Severity:   string(SeverityMinor),
					Line:       i + 1,
					Message:    fmt.Sprintf("Function '%s' should have a comment explaining its purpose", funcName),
					Suggestion: fmt.Sprintf("// %s ...\n%s", funcName, line),
					RuleID:     "function_comments",
				})
			}
		}
	}
	
	return issues
}

func checkFunctionNaming(code string) []Issue {
	var issues []Issue
	
	// Check for poor function names (too short, non-descriptive)
	funcPattern := regexp.MustCompile(`func\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)
	matches := funcPattern.FindAllStringSubmatch(code, -1)
	
	for _, match := range matches {
		funcName := match[1]
		if len(funcName) <= 2 && !isAcceptableShortName(funcName) {
			issues = append(issues, Issue{
				Type:       "naming",
				Severity:   string(SeverityMajor),
				Line:       findLineNumber(code, match[0]),
				Message:    fmt.Sprintf("Function name '%s' is too short and not descriptive", funcName),
				Suggestion: "Use descriptive names like 'CalculateSum' instead of 'calc'",
				RuleID:     "function_naming",
			})
		}
	}
	
	return issues
}

func checkVariableNaming(code string) []Issue {
	var issues []Issue
	
	// Check for single-letter variables in longer contexts
	varPattern := regexp.MustCompile(`([a-z])\s*:=`)
	matches := varPattern.FindAllStringSubmatch(code, -1)
	
	for _, match := range matches {
		varName := match[1]
		if len(varName) == 1 && !isAcceptableShortVar(varName) {
			issues = append(issues, Issue{
				Type:       "naming",
				Severity:   string(SeverityMinor),
				Line:       findLineNumber(code, match[0]),
				Message:    fmt.Sprintf("Variable name '%s' is not descriptive", varName),
				Suggestion: "Use meaningful variable names like 'total' instead of 't'",
				RuleID:     "variable_naming",
			})
		}
	}
	
	return issues
}

func checkCodeFormatting(code string) []Issue {
	var issues []Issue
	
	// Check for missing spaces around operators
	if strings.Contains(code, "x+y") || strings.Contains(code, "x-y") {
		issues = append(issues, Issue{
			Type:       "formatting",
			Severity:   string(SeverityMinor),
			Line:       findLineNumber(code, "x+y"),
			Message:    "Missing spaces around operators",
			Suggestion: "Use 'x + y' instead of 'x+y'",
			RuleID:     "code_formatting",
		})
	}
	
	// Check for missing spaces after commas
	if regexp.MustCompile(`\w,\w`).MatchString(code) {
		issues = append(issues, Issue{
			Type:       "formatting",
			Severity:   string(SeverityMinor),
			Line:       0,
			Message:    "Missing spaces after commas in parameter lists",
			Suggestion: "Use 'func calc(x, y int)' instead of 'func calc(x,y int)'",
			RuleID:     "code_formatting",
		})
	}
	
	return issues
}

func checkInfiniteLoops(code string) []Issue {
	var issues []Issue
	
	// Pattern for potential infinite loops
	patterns := []string{
		`for\s+i\s*:=\s*\d+;\s*i\s*>\s*\d+;\s*i\+\+`,
		`for\s+i\s*:=\s*\d+;\s*i\s*<\s*\d+;\s*i--`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(code) {
			issues = append(issues, Issue{
				Type:       "logic",
				Severity:   string(SeverityCritical),
				Line:       findLineNumber(code, re.FindString(code)),
				Message:    "Potential infinite loop detected",
				Suggestion: "Check loop condition and increment/decrement logic",
				RuleID:     "infinite_loops",
			})
		}
	}
	
	return issues
}

func checkArrayBounds(code string) []Issue {
	var issues []Issue
	
	// Check for i <= len(arr) which causes out of bounds
	if strings.Contains(code, "i <= len(") {
		issues = append(issues, Issue{
			Type:       "safety",
			Severity:   string(SeverityCritical),
			Line:       findLineNumber(code, "i <= len("),
			Message:    "Potential array index out of bounds",
			Suggestion: "Use 'i < len(arr)' instead of 'i <= len(arr)'",
			RuleID:     "array_bounds",
		})
	}
	
	return issues
}

func checkNilPointer(code string) []Issue {
	var issues []Issue
	
	// Simple nil pointer check
	if strings.Contains(code, "var slice []int") && strings.Contains(code, "slice[0]") {
		issues = append(issues, Issue{
			Type:       "safety",
			Severity:   string(SeverityCritical),
			Line:       findLineNumber(code, "slice[0]"),
			Message:    "Potential nil pointer/slice access",
			Suggestion: "Check if slice is not nil and has elements before accessing",
			RuleID:     "nil_pointer",
		})
	}
	
	return issues
}

func checkPerformanceIssues(code string) []Issue {
	var issues []Issue
	
	// Check for nested loops that could be optimized
	nestedLoopPattern := regexp.MustCompile(`for\s+[^{]+{\s*[^}]*for\s+[^{]+{`)
	if nestedLoopPattern.MatchString(code) {
		issues = append(issues, Issue{
			Type:       "performance",
			Severity:   string(SeverityMajor),
			Line:       0,
			Message:    "Nested loops detected - consider optimization",
			Suggestion: "Review if the nested loop is necessary or can be optimized",
			RuleID:     "performance_issues",
		})
	}
	
	return issues
}

// Helper functions

func findLineNumber(code, pattern string) int {
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		if strings.Contains(line, pattern) {
			return i + 1
		}
	}
	return 0
}

func isAcceptableShortName(name string) bool {
	acceptable := []string{"id", "ok", "err"}
	for _, acc := range acceptable {
		if name == acc {
			return true
		}
	}
	return false
}

func isAcceptableShortVar(name string) bool {
	acceptable := []string{"i", "j", "k", "x", "y", "z"}
	for _, acc := range acceptable {
		if name == acc {
			return true
		}
	}
	return false
}

func hasGoodNaming(code string) bool {
	// Simple heuristic: if we have descriptive function names
	return regexp.MustCompile(`func\s+[A-Z][a-zA-Z]{3,}`).MatchString(code)
}

func hasProperFormatting(code string) bool {
	// Simple heuristic: proper spacing around operators
	return !strings.Contains(code, "x+y") && !strings.Contains(code, "x,y")
}

func hasGoodComments(code string) bool {
	// Check if there are meaningful comments
	return strings.Contains(code, "//") && len(strings.Split(code, "//")) > 2
}

func hasGoodErrorHandling(code string) bool {
	// Check for error handling patterns
	return strings.Contains(code, "if") && (strings.Contains(code, "< 0") || strings.Contains(code, "== nil"))
}

func hasEfficientAlgorithms(code string) bool {
	// Simple heuristic: single loops are generally more efficient
	return !regexp.MustCompile(`for\s+[^{]+{\s*[^}]*for\s+[^{]+{`).MatchString(code)
}

func hasProperLoopLogic(code string) bool {
	// Check that loops don't have obvious infinite loop patterns
	return !regexp.MustCompile(`for\s+i\s*:=\s*\d+;\s*i\s*>\s*\d+;\s*i\+\+`).MatchString(code)
}

func calculateSyntaxScore(issues []Issue) int {
	score := 100
	for _, issue := range issues {
		switch SeverityLevel(issue.Severity) {
		case SeverityMinor:
			score -= 5
		case SeverityMajor:
			score -= 15
		case SeverityCritical:
			score -= 25
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

func calculateLogicScore(issues []Issue) int {
	score := 100
	for _, issue := range issues {
		switch SeverityLevel(issue.Severity) {
		case SeverityMinor:
			score -= 10
		case SeverityMajor:
			score -= 20
		case SeverityCritical:
			score -= 30
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

func generateSyntaxSummary(score int, issueCount int) string {
	if score >= 90 {
		return "Excellent code style and formatting!"
	} else if score >= 70 {
		return "Good code style with minor improvements needed"
	} else if score >= 50 {
		return "Decent code style but several issues to address"
	} else {
		return "Code style needs significant improvement"
	}
}

func generateLogicSummary(score int, issueCount int) string {
	if score >= 90 {
		return "Excellent logic and no bugs detected!"
	} else if score >= 70 {
		return "Good logic with minor issues to fix"
	} else if score >= 50 {
		return "Logic is mostly correct but has some problems"
	} else {
		return "Significant logic issues detected that need fixing"
	}
}

// GenerateAnalysisFeedbackMessage creates a message for sending analysis feedback
func GenerateAnalysisFeedbackMessage(studentID string, result AnalysisResult) map[string]interface{} {
	return map[string]interface{}{
		"context":    "response",  // Use valid switchboard context
		"agent_type": result.AgentType,
		"student_id": studentID,
		"analysis":   result,
		"timestamp":  time.Now().Format(time.RFC3339),
	}
}