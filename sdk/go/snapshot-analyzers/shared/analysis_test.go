package shared

import (
	"testing"
)

func TestSyntaxAnalysisEngine(t *testing.T) {
	engine := NewSyntaxAnalysisEngine()
	
	// Test perfect code
	perfectCode := `// CalculateSum computes the sum of numbers in a slice
func CalculateSum(numbers []int) int {
    if len(numbers) == 0 {
        return 0
    }
    
    total := 0
    for _, num := range numbers {
        total += num
    }
    return total
}`
	
	result := engine.AnalyzeCode(perfectCode)
	
	if result.Score < 80 {
		t.Errorf("Expected high score for perfect code, got %d", result.Score)
	}
	
	if result.AgentType != "syntax" {
		t.Errorf("Expected agent type 'syntax', got %s", result.AgentType)
	}
	
	// Test code with style issues
	badStyleCode := `func calc(x,y int)int{
return x+y
}`
	
	badResult := engine.AnalyzeCode(badStyleCode)
	
	if badResult.Score > result.Score {
		t.Errorf("Bad style code should have lower score than perfect code")
	}
	
	if len(badResult.Issues) == 0 {
		t.Error("Expected issues to be found in bad style code")
	}
}

func TestLogicAnalysisEngine(t *testing.T) {
	engine := NewLogicAnalysisEngine()
	
	// Test good logic
	goodCode := `func CalculateSum(numbers []int) int {
    if numbers == nil {
        return 0
    }
    
    total := 0
    for i := 0; i < len(numbers); i++ {
        total += numbers[i]
    }
    return total
}`
	
	result := engine.AnalyzeCode(goodCode)
	
	if result.AgentType != "logic" {
		t.Errorf("Expected agent type 'logic', got %s", result.AgentType)
	}
	
	// Test code with infinite loop
	buggyCode := `func PrintNumbers(n int) {
    for i := 10; i > 0; i++ {
        fmt.Println(i)
    }
}`
	
	buggyResult := engine.AnalyzeCode(buggyCode)
	
	if buggyResult.Score > result.Score {
		t.Error("Buggy code should have lower score than good code")
	}
	
	criticalIssues := 0
	for _, issue := range buggyResult.Issues {
		if issue.Severity == "critical" {
			criticalIssues++
		}
	}
	
	if criticalIssues == 0 {
		t.Error("Expected critical issues to be found in infinite loop code")
	}
}

func TestCodeSamples(t *testing.T) {
	// Test Alice's profile
	if len(AliceProfile.CodeSamples) == 0 {
		t.Error("Alice should have code samples")
	}
	
	aliceSample := GetNextCodeSample(AliceProfile, 0)
	if aliceSample.Code == "" {
		t.Error("Alice's code sample should not be empty")
	}
	
	// Test Bob's profile
	if len(BobProfile.CodeSamples) == 0 {
		t.Error("Bob should have code samples")
	}
	
	bobSample := GetNextCodeSample(BobProfile, 0)
	if bobSample.Code == "" {
		t.Error("Bob's code sample should not be empty")
	}
	
	// Test cycling through samples
	sample1 := GetNextCodeSample(AliceProfile, 0)
	sample2 := GetNextCodeSample(AliceProfile, len(AliceProfile.CodeSamples))
	
	if sample1.Code != sample2.Code {
		t.Error("Code samples should cycle when index exceeds length")
	}
}

func TestMessageGeneration(t *testing.T) {
	sample := CodeSample{
		Code:       "func test() {}",
		Language:   "go",
		FileName:   "test.go",
		ExerciseID: "basic",
	}
	
	msg := GenerateCodeSnapshotMessage("alice", sample)
	
	if msg["context"] != "code_snapshot" {
		t.Error("Message should have code_snapshot context")
	}
	
	if msg["student_id"] != "alice" {
		t.Error("Message should have correct student_id")
	}
	
	if msg["code"] != sample.Code {
		t.Error("Message should contain the code")
	}
}

func TestAnalysisResult(t *testing.T) {
	result := AnalysisResult{
		Score:            85,
		Issues:           []Issue{},
		PositiveFeedback: []string{"Good naming"},
		AgentType:        "syntax",
	}
	
	msg := GenerateAnalysisFeedbackMessage("alice", result)
	
	if msg["context"] != "analysis_feedback" {
		t.Error("Feedback message should have analysis_feedback context")
	}
	
	if msg["student_id"] != "alice" {
		t.Error("Feedback message should have correct student_id")
	}
	
	if msg["agent_type"] != "syntax" {
		t.Error("Feedback message should have correct agent_type")
	}
}