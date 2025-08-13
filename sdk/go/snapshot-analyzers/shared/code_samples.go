package shared

import (
	"math/rand"
	"time"
)

// CodeSample represents a code snippet with metadata
type CodeSample struct {
	Code       string            `json:"code"`
	Language   string            `json:"language"`
	FileName   string            `json:"file_name"`
	ExerciseID string            `json:"exercise_id"`
	Metadata   map[string]string `json:"metadata"`
}

// StudentProfile represents different types of student coding patterns
type StudentProfile struct {
	Name        string
	FocusAreas  []string
	SkillLevel  string
	CodeSamples []CodeSample
}

// Alice's code samples - focused on basic functions and data structures
var AliceProfile = StudentProfile{
	Name:       "alice",
	FocusAreas: []string{"functions", "structs", "basic_algorithms"},
	SkillLevel: "beginner",
	CodeSamples: []CodeSample{
		{
			Code: `// CalculateCircleArea calculates the area of a circle given its radius
func CalculateCircleArea(radius float64) float64 {
    if radius < 0 {
        return 0
    }
    return math.Pi * radius * radius
}`,
			Language:   "go",
			FileName:   "geometry.go",
			ExerciseID: "basic_functions",
			Metadata:   map[string]string{"topic": "geometry", "difficulty": "easy"},
		},
		{
			Code: `func addNumbers(x, y int) int {
    return x + y
}`,
			Language:   "go",
			FileName:   "calculator.go",
			ExerciseID: "basic_functions",
			Metadata:   map[string]string{"topic": "arithmetic", "style_issues": "no_comments"},
		},
		{
			Code: `type Person struct {
    Name string
    Age  int
}

func (p Person) GetInfo() string {
    return fmt.Sprintf("Name: %s, Age: %d", p.Name, p.Age)
}`,
			Language:   "go",
			FileName:   "person.go",
			ExerciseID: "structs_methods",
			Metadata:   map[string]string{"topic": "data_structures", "difficulty": "medium"},
		},
		{
			Code: `func StringLength(s string) int {
    count := 0
    for range s {
        count++
    }
    return count
}`,
			Language:   "go",
			FileName:   "strings.go",
			ExerciseID: "string_processing",
			Metadata:   map[string]string{"topic": "strings", "performance": "inefficient"},
		},
		{
			Code: `func PrintNumbers(n int) {
    for i := 0; i < n; i-- {
        fmt.Println(i)
    }
}`,
			Language:   "go",
			FileName:   "loops.go",
			ExerciseID: "control_flow",
			Metadata:   map[string]string{"topic": "loops", "bug": "infinite_loop"},
		},
		{
			Code: `// Sum calculates the sum of integers in a slice
func Sum(numbers []int) int {
    if len(numbers) == 0 {
        return 0
    }
    
    total := 0
    for _, num := range numbers {
        total += num
    }
    return total
}`,
			Language:   "go",
			FileName:   "math_utils.go",
			ExerciseID: "slice_operations",
			Metadata:   map[string]string{"topic": "slices", "quality": "good"},
		},
	},
}

// Bob's code samples - focused on algorithms and complex logic
var BobProfile = StudentProfile{
	Name:       "bob",
	FocusAreas: []string{"algorithms", "sorting", "recursion"},
	SkillLevel: "intermediate",
	CodeSamples: []CodeSample{
		{
			Code: `func BubbleSort(arr []int) []int {
    n := len(arr)
    for i := 0; i < n-1; i++ {
        for j := 0; j < n-i-1; j++ {
            if arr[j] > arr[j+1] {
                arr[j], arr[j+1] = arr[j+1], arr[j]
            }
        }
    }
    return arr
}`,
			Language:   "go",
			FileName:   "sorting.go",
			ExerciseID: "sorting_algorithms",
			Metadata:   map[string]string{"topic": "sorting", "complexity": "O(n²)"},
		},
		{
			Code: `func binarySearch(arr []int, target int) int {
    left := 0
    right := len(arr) - 1
    
    for left <= right {
        mid := (left + right) / 2
        if arr[mid] == target {
            return mid
        } else if arr[mid] < target {
            left = mid + 1
        } else {
            right = mid - 1
        }
    }
    return -1
}`,
			Language:   "go",
			FileName:   "search.go",
			ExerciseID: "search_algorithms",
			Metadata:   map[string]string{"topic": "search", "style_issues": "no_comments"},
		},
		{
			Code: `func Factorial(n int) int {
    if n <= 1 {
        return 1
    }
    return n * Factorial(n-1)
}`,
			Language:   "go",
			FileName:   "recursion.go",
			ExerciseID: "recursive_functions",
			Metadata:   map[string]string{"topic": "recursion", "quality": "good"},
		},
		{
			Code: `func FindMax(arr []int) int {
    max := arr[0]
    for i := 0; i < len(arr); i++ {
        for j := 0; j < len(arr); j++ {
            if arr[j] > max {
                max = arr[j]
            }
        }
    }
    return max
}`,
			Language:   "go",
			FileName:   "algorithms.go",
			ExerciseID: "array_operations",
			Metadata:   map[string]string{"topic": "arrays", "performance": "inefficient_nested_loop"},
		},
		{
			Code: `func ProcessSlice(data []string) []string {
    var result []string
    
    for i := 0; i <= len(data); i++ {
        if data[i] != "" {
            result = append(result, data[i])
        }
    }
    
    return result
}`,
			Language:   "go",
			FileName:   "data_processing.go",
			ExerciseID: "slice_operations",
			Metadata:   map[string]string{"topic": "slices", "bug": "index_out_of_bounds"},
		},
		{
			Code: `// QuickSort implements the quicksort algorithm with proper partitioning
func QuickSort(arr []int) []int {
    if len(arr) <= 1 {
        return arr
    }
    
    pivot := partition(arr, 0, len(arr)-1)
    left := QuickSort(arr[:pivot])
    right := QuickSort(arr[pivot+1:])
    
    result := append(left, arr[pivot])
    result = append(result, right...)
    return result
}

func partition(arr []int, low, high int) int {
    pivot := arr[high]
    i := low - 1
    
    for j := low; j < high; j++ {
        if arr[j] < pivot {
            i++
            arr[i], arr[j] = arr[j], arr[i]
        }
    }
    
    arr[i+1], arr[high] = arr[high], arr[i+1]
    return i + 1
}`,
			Language:   "go",
			FileName:   "advanced_sorting.go",
			ExerciseID: "advanced_algorithms",
			Metadata:   map[string]string{"topic": "sorting", "quality": "excellent", "complexity": "O(n_log_n)"},
		},
	},
}

// GetNextCodeSample returns the next code sample for a student in rotation
func GetNextCodeSample(profile StudentProfile, index int) CodeSample {
	if len(profile.CodeSamples) == 0 {
		return CodeSample{
			Code:       "// No code samples available",
			Language:   "go",
			FileName:   "empty.go",
			ExerciseID: "none",
		}
	}
	
	return profile.CodeSamples[index%len(profile.CodeSamples)]
}

// GetRandomCodeSample returns a random code sample for a student
func GetRandomCodeSample(profile StudentProfile) CodeSample {
	if len(profile.CodeSamples) == 0 {
		return CodeSample{
			Code:       "// No code samples available",
			Language:   "go",
			FileName:   "empty.go",
			ExerciseID: "none",
		}
	}
	
	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(profile.CodeSamples))
	return profile.CodeSamples[index]
}

// GenerateCodeSnapshotMessage creates a message for broadcasting code snapshots
func GenerateCodeSnapshotMessage(studentID string, sample CodeSample) map[string]interface{} {
	return map[string]interface{}{
		"context":        "submission",  // Use valid switchboard context
		"type":           "code_snapshot", // Keep our custom type in content
		"student_id":     studentID,
		"code":           sample.Code,
		"language":       sample.Language,
		"file_name":      sample.FileName,
		"timestamp":      time.Now().Format(time.RFC3339),
		"exercise_id":    sample.ExerciseID,
		"metadata":       sample.Metadata,
	}
}

// Demo scenarios for testing specific analysis cases
var DemoScenarios = map[string]CodeSample{
	"perfect_code": {
		Code: `// CalculateTotal computes the sum of all positive numbers in a slice
func CalculateTotal(numbers []int) int {
    if len(numbers) == 0 {
        return 0
    }
    
    total := 0
    for _, num := range numbers {
        if num > 0 {
            total += num
        }
    }
    return total
}`,
		Language:   "go",
		FileName:   "perfect_example.go",
		ExerciseID: "demo_perfect",
		Metadata:   map[string]string{"expected": "all_positive_feedback"},
	},
	"style_issues": {
		Code: `func calc(x,y int)int{
return x+y
}`,
		Language:   "go",
		FileName:   "bad_style.go",
		ExerciseID: "demo_style",
		Metadata:   map[string]string{"expected": "style_warnings"},
	},
	"logic_bug": {
		Code: `func InfiniteLoop() {
    for i := 10; i > 0; i++ {
        fmt.Println(i)
    }
}`,
		Language:   "go",
		FileName:   "buggy_logic.go",
		ExerciseID: "demo_bugs",
		Metadata:   map[string]string{"expected": "logic_error"},
	},
	"performance_issue": {
		Code: `func SlowSearch(arr []int, target int) bool {
    for i := 0; i < len(arr); i++ {
        for j := 0; j < len(arr); j++ {
            if arr[j] == target {
                return true
            }
        }
    }
    return false
}`,
		Language:   "go",
		FileName:   "slow_algorithm.go",
		ExerciseID: "demo_performance",
		Metadata:   map[string]string{"expected": "performance_warning"},
	},
}