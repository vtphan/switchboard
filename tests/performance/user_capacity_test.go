// Test 3.2: Concurrent User Capacity (50+ Users)
// P0-Critical Performance/Scalability Test
// This test validates maximum concurrent user capacity with realistic usage patterns
package performance

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switchboard/internal/database"
	"switchboard/internal/message"
	"switchboard/internal/rate"
	"switchboard/internal/session"
	"switchboard/internal/websocket"
)

// TestTest32ConcurrentUserCapacity executes Test 3.2 from the validation test plan
func TestTest32ConcurrentUserCapacity(t *testing.T) {
	t.Logf("=== Executing Test 3.2: Concurrent User Capacity (50+ Users) ===")
	t.Logf("Priority: P0-Critical")
	t.Logf("Type: Performance/Scalability Test")
	t.Logf("Expected Duration: ~90 minutes (scaled down for testing)")
	
	// Test configuration optimized for reasonable execution time
	const (
		testDurationSeconds = 30 // Scaled down to 30 seconds for practical testing
		
		// Capacity levels to test - gradual scaling
		_capacityLevels = 3 // Reserved for future capacity level configuration
		
		// Realistic education patterns (scaled for test duration)
		questionsPerStudentPer30Sec = 1   // Students ask 1 question per 30 seconds
		responsesPerInstructorPer30Sec = 1 // Instructors send 1 response per 30 seconds
		announcementsPer30Sec = 1         // 1 announcement per 30 seconds
		
		// Success criteria thresholds
		maxErrorRatePercent = 5.0
		maxMemoryMB = 100.0
		maxLatencyMs = 100
	)
	
	capacityTargets := []int{10, 25, 50}
	
	t.Logf("Test Configuration:")
	t.Logf("  • Duration: %d seconds (scaled from 90 min)", testDurationSeconds)
	t.Logf("  • Capacity Levels: %v users", capacityTargets)
	t.Logf("  • Student Pattern: %d question per %d seconds", questionsPerStudentPer30Sec, testDurationSeconds)
	t.Logf("  • Instructor Pattern: %d responses per %d seconds", responsesPerInstructorPer30Sec, testDurationSeconds)
	t.Logf("  • Announcements: %d per %d seconds", announcementsPer30Sec, testDurationSeconds)
	
	var capacityResults []CapacityTestResult
	
	for levelIndex, targetUsers := range capacityTargets {
		t.Logf("\n🔄 Starting Capacity Level %d: %d Concurrent Users", levelIndex+1, targetUsers)
		
		// Setup fresh environment for each capacity level
		env := setupCapacityTestEnvironment(t)
		
		result := runCapacityLevel(t, env, CapacityLevelConfig{
			Level:                        levelIndex + 1,
			TargetUsers:                  targetUsers,
			TestDurationSeconds:          testDurationSeconds,
			QuestionsPerStudentPer30Sec:  questionsPerStudentPer30Sec,
			ResponsesPerInstructorPer30Sec: responsesPerInstructorPer30Sec,
			AnnouncementsPer30Sec:        announcementsPer30Sec,
		})
		
		capacityResults = append(capacityResults, result)
		env.cleanup()
		
		// Brief cooldown between capacity levels
		t.Logf("⏳ Cooldown before next capacity level...")
		time.Sleep(2 * time.Second)
		
		// Early termination if severe degradation detected
		if result.ErrorRate > 50.0 || result.PeakMemoryMB > 500.0 {
			t.Logf("⚠️  Severe performance degradation detected at %d users. Stopping capacity testing.", targetUsers)
			break
		}
	}
	
	// Comprehensive results analysis
	analyzeCapacityResults(t, capacityResults, maxErrorRatePercent, maxMemoryMB, maxLatencyMs)
}

type CapacityLevelConfig struct {
	Level                        int
	TargetUsers                  int
	TestDurationSeconds          int
	QuestionsPerStudentPer30Sec  int
	ResponsesPerInstructorPer30Sec int
	AnnouncementsPer30Sec        int
}

type CapacityTestResult struct {
	Level                 int
	TargetUsers           int
	ActualUsers           int
	StudentsConnected     int
	InstructorsConnected  int
	
	// Performance metrics
	TestDuration          time.Duration
	MessagesProcessed     int64
	MessageErrors         int64
	ErrorRate             float64
	
	// Latency metrics
	AvgLatency            time.Duration
	P95Latency            time.Duration
	MaxLatency            time.Duration
	
	// Resource usage
	PeakMemoryMB          float64
	FinalMemoryMB         float64
	
	// Connection metrics
	ConnectionErrors      int64
	ConnectionSuccessRate float64
	
	// Success criteria results
	ErrorRatePass         bool
	MemoryUsagePass       bool
	LatencyPass           bool
	SystemResponsivePass  bool
	
	// Additional insights
	QuestionsGenerated    int64
	ResponsesGenerated    int64
	AnnouncementsGenerated int64
}

func runCapacityLevel(t *testing.T, env *CapacityTestEnvironment, config CapacityLevelConfig) CapacityTestResult {
	t.Logf("=== Capacity Level %d: %d Users ===", config.Level, config.TargetUsers)
	
	// Calculate user distribution: ~10% instructors, 90% students
	numInstructors := int(math.Max(1, float64(config.TargetUsers)*0.1))
	numStudents := config.TargetUsers - numInstructors
	
	t.Logf("User Distribution: %d students, %d instructors", numStudents, numInstructors)
	
	// Setup session
	session := &database.Session{
		ID:        fmt.Sprintf("capacity-level-%d-%d-users", config.Level, config.TargetUsers),
		Name:      fmt.Sprintf("Capacity Test Level %d - %d Users", config.Level, config.TargetUsers),
		CreatedBy: "instructor1",
		StartTime: time.Now(),
		Status:    "active",
	}
	require.NoError(t, env.sessionManager.SetActiveSession(session))
	
	// Create connections with connection error tracking
	var connectionErrors int64
	var students []*CapacityConnection
	var instructors []*CapacityConnection
	
	// Create student connections
	t.Logf("Creating %d student connections...", numStudents)
	for i := 0; i < numStudents; i++ {
		conn := &CapacityConnection{
			userID: fmt.Sprintf("student%d", i+1),
			role:   "student",
			lastActivity: time.Now(),
		}
		err := env.connectionRegistry.Register(conn.userID, conn)
		if err != nil {
			atomic.AddInt64(&connectionErrors, 1)
			t.Logf("Failed to register student %d: %v", i+1, err)
		} else {
			students = append(students, conn)
		}
	}
	
	// Create instructor connections
	t.Logf("Creating %d instructor connections...", numInstructors)
	for i := 0; i < numInstructors; i++ {
		conn := &CapacityConnection{
			userID: fmt.Sprintf("instructor%d", i+1),
			role:   "instructor",
			lastActivity: time.Now(),
		}
		err := env.connectionRegistry.Register(conn.userID, conn)
		if err != nil {
			atomic.AddInt64(&connectionErrors, 1)
			t.Logf("Failed to register instructor %d: %v", i+1, err)
		} else {
			instructors = append(instructors, conn)
		}
	}
	
	actualUsers := len(students) + len(instructors)
	connSuccessRate := float64(actualUsers) / float64(config.TargetUsers) * 100
	
	t.Logf("Connection Results: %d/%d users connected (%.1f%% success rate)", 
		actualUsers, config.TargetUsers, connSuccessRate)
	
	if len(students) == 0 || len(instructors) == 0 {
		return CapacityTestResult{
			Level:                 config.Level,
			TargetUsers:          config.TargetUsers,
			ActualUsers:          actualUsers,
			StudentsConnected:    len(students),
			InstructorsConnected: len(instructors),
			ConnectionErrors:     atomic.LoadInt64(&connectionErrors),
			ConnectionSuccessRate: connSuccessRate,
			ErrorRate:            100.0, // Complete failure
		}
	}
	
	// Metrics tracking
	var (
		totalMessages         int64
		totalErrors           int64
		questionsGenerated    int64
		responsesGenerated    int64
		announcementsGenerated int64
		latencies             []time.Duration
		latencyMutex          sync.RWMutex
		peakMemoryMB          float64
		memoryMutex           sync.RWMutex
	)
	
	startTime := time.Now()
	testDuration := time.Duration(config.TestDurationSeconds) * time.Second
	
	var wg sync.WaitGroup
	stopSignal := make(chan struct{})
	
	// Student activity simulation
	for i, student := range students {
		wg.Add(1)
		go func(studentIndex int, studentConn *CapacityConnection) {
			defer wg.Done()
			
			// Calculate intervals for realistic timing
			questionInterval := testDuration / time.Duration(config.QuestionsPerStudentPer30Sec)
			if questionInterval < time.Second {
				questionInterval = time.Second // Minimum 1 second between questions
			}
			
			// Add jitter to prevent synchronized bursts
			jitterFactor := time.Duration(rand.Intn(int(questionInterval.Seconds()/2))) * time.Second
			nextQuestionTime := time.Now().Add(questionInterval + jitterFactor)
			
			questionsAsked := 0
			maxQuestions := config.QuestionsPerStudentPer30Sec
			
			ticker := time.NewTicker(time.Second) // Check every second
			defer ticker.Stop()
			
			for {
				select {
				case <-stopSignal:
					return
				case <-ticker.C:
					now := time.Now()
					
					// Time to ask a question?
					if now.After(nextQuestionTime) && questionsAsked < maxQuestions {
						questionStart := time.Now()
						question := generateRealisticStudentQuestion(studentIndex, questionsAsked)
						msg := createStudentQuestion(question)
						
						err := env.processor.ProcessIncomingMessage(msg, studentConn.userID)
						latency := time.Since(questionStart)
						
						if err != nil {
							atomic.AddInt64(&totalErrors, 1)
						} else {
							atomic.AddInt64(&totalMessages, 1)
							atomic.AddInt64(&questionsGenerated, 1)
							
							// Record latency
							latencyMutex.Lock()
							latencies = append(latencies, latency)
							latencyMutex.Unlock()
						}
						
						questionsAsked++
						
						// Schedule next question with jitter
						if questionsAsked < maxQuestions {
							jitter := time.Duration(rand.Intn(int(questionInterval.Seconds()/4))) * time.Second
							nextQuestionTime = now.Add(questionInterval + jitter)
						}
					}
				}
			}
		}(i, student)
	}
	
	// Instructor activity simulation
	for i, instructor := range instructors {
		wg.Add(1)
		go func(instructorIndex int, instructorConn *CapacityConnection) {
			defer wg.Done()
			
			// Calculate response and announcement intervals
			responseInterval := testDuration / time.Duration(config.ResponsesPerInstructorPer30Sec)
			announcementsPerInstructor := config.AnnouncementsPer30Sec / len(instructors)
			if announcementsPerInstructor == 0 {
				announcementsPerInstructor = 1
			}
			announcementInterval := testDuration / time.Duration(announcementsPerInstructor)
			
			if responseInterval < time.Second {
				responseInterval = time.Second
			}
			if announcementInterval < 10*time.Second {
				announcementInterval = 10 * time.Second
			}
			
			responseJitter := time.Duration(rand.Intn(int(responseInterval.Seconds()/3))) * time.Second
			announcementJitter := time.Duration(rand.Intn(int(announcementInterval.Seconds()/2))) * time.Second
			
			nextResponseTime := time.Now().Add(responseInterval + responseJitter)
			nextAnnouncementTime := time.Now().Add(announcementInterval + announcementJitter)
			
			responsesMade := 0
			announcementsMade := 0
			maxResponses := config.ResponsesPerInstructorPer30Sec
			maxAnnouncements := announcementsPerInstructor
			
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			
			for {
				select {
				case <-stopSignal:
					return
				case <-ticker.C:
					now := time.Now()
					
					// Time for a response?
					if now.After(nextResponseTime) && responsesMade < maxResponses && len(students) > 0 {
						responseStart := time.Now()
						targetStudent := students[rand.Intn(len(students))]
						response := generateRealisticInstructorResponse(instructorIndex, responsesMade)
						msg := createDirectResponse(targetStudent.userID, response)
						
						err := env.processor.ProcessIncomingMessage(msg, instructorConn.userID)
						latency := time.Since(responseStart)
						
						if err != nil {
							atomic.AddInt64(&totalErrors, 1)
						} else {
							atomic.AddInt64(&totalMessages, 1)
							atomic.AddInt64(&responsesGenerated, 1)
							
							latencyMutex.Lock()
							latencies = append(latencies, latency)
							latencyMutex.Unlock()
						}
						
						responsesMade++
						
						if responsesMade < maxResponses {
							jitter := time.Duration(rand.Intn(int(responseInterval.Seconds()/4))) * time.Second
							nextResponseTime = now.Add(responseInterval + jitter)
						}
					}
					
					// Time for an announcement?
					if now.After(nextAnnouncementTime) && announcementsMade < maxAnnouncements {
						announcementStart := time.Now()
						announcement := generateCapacityTestAnnouncement(instructorIndex, announcementsMade)
						msg := createAnnouncement(announcement)
						
						err := env.processor.ProcessIncomingMessage(msg, instructorConn.userID)
						latency := time.Since(announcementStart)
						
						if err != nil {
							atomic.AddInt64(&totalErrors, 1)
						} else {
							atomic.AddInt64(&totalMessages, 1)
							atomic.AddInt64(&announcementsGenerated, 1)
							
							latencyMutex.Lock()
							latencies = append(latencies, latency)
							latencyMutex.Unlock()
						}
						
						announcementsMade++
						
						if announcementsMade < maxAnnouncements {
							jitter := time.Duration(rand.Intn(int(announcementInterval.Seconds()/2))) * time.Second
							nextAnnouncementTime = now.Add(announcementInterval + jitter)
						}
					}
				}
			}
		}(i, instructor)
	}
	
	// Memory monitoring
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-stopSignal:
				return
			case <-ticker.C:
				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)
				currentMemoryMB := float64(memStats.Alloc) / 1024 / 1024
				
				memoryMutex.Lock()
				if currentMemoryMB > peakMemoryMB {
					peakMemoryMB = currentMemoryMB
				}
				memoryMutex.Unlock()
			}
		}
	}()
	
	// Live monitoring and progress reporting
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		ticker := time.NewTicker(10 * time.Second) // Report every 10 seconds
		defer ticker.Stop()
		
		lastMessageCount := int64(0)
		lastTime := startTime
		
		for {
			select {
			case <-stopSignal:
				return
			case <-ticker.C:
				elapsed := time.Since(startTime)
				currentMessages := atomic.LoadInt64(&totalMessages)
				currentErrors := atomic.LoadInt64(&totalErrors)
				
				// Calculate current throughput
				messageDelta := currentMessages - lastMessageCount
				timeDelta := time.Since(lastTime).Seconds()
				currentThroughput := float64(messageDelta) / timeDelta
				
				// Overall metrics
				overallThroughput := float64(currentMessages) / elapsed.Seconds()
				errorRate := float64(currentErrors) / float64(currentMessages+currentErrors) * 100
				
				memoryMutex.RLock()
				currentPeakMB := peakMemoryMB
				memoryMutex.RUnlock()
				
				t.Logf("📊 [Level %d - %d users] %.1f min: %.1f msg/s current, %.1f msg/s avg, %.1f%% errors, %.1f MB peak memory",
					config.Level, actualUsers, elapsed.Minutes(), currentThroughput, overallThroughput, errorRate, currentPeakMB)
				
				lastMessageCount = currentMessages
				lastTime = time.Now()
			}
		}
	}()
	
	// Run test for configured duration
	t.Logf("⏱️  Running capacity test for %v...", testDuration)
	time.Sleep(testDuration)
	
	// Stop all activities
	close(stopSignal)
	
	// Wait for all goroutines with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All activities completed normally
	case <-time.After(30 * time.Second):
		t.Logf("⚠️  Some activities did not complete within timeout")
	}
	
	actualDuration := time.Since(startTime)
	messagesProcessed := atomic.LoadInt64(&totalMessages)
	messageErrors := atomic.LoadInt64(&totalErrors)
	
	// Calculate final metrics
	errorRate := float64(messageErrors) / float64(messagesProcessed+messageErrors) * 100
	if messagesProcessed+messageErrors == 0 {
		errorRate = 0
	}
	
	// Calculate latency statistics
	var avgLatency, p95Latency, maxLatency time.Duration
	if len(latencies) > 0 {
		latencyMutex.RLock()
		sortedLatencies := make([]time.Duration, len(latencies))
		copy(sortedLatencies, latencies)
		latencyMutex.RUnlock()
		
		sort.Slice(sortedLatencies, func(i, j int) bool {
			return sortedLatencies[i] < sortedLatencies[j]
		})
		
		var totalLatency time.Duration
		for _, lat := range sortedLatencies {
			totalLatency += lat
		}
		avgLatency = totalLatency / time.Duration(len(sortedLatencies))
		
		p95Index := int(float64(len(sortedLatencies)) * 0.95)
		if p95Index < len(sortedLatencies) {
			p95Latency = sortedLatencies[p95Index]
		}
		maxLatency = sortedLatencies[len(sortedLatencies)-1]
	}
	
	// Final memory measurement
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	finalMemoryMB := float64(memStats.Alloc) / 1024 / 1024
	
	memoryMutex.RLock()
	finalPeakMemoryMB := peakMemoryMB
	memoryMutex.RUnlock()
	
	if finalMemoryMB > finalPeakMemoryMB {
		finalPeakMemoryMB = finalMemoryMB
	}
	
	// Evaluate success criteria
	errorRatePass := errorRate <= 5.0 // < 5% error rate
	memoryUsagePass := finalPeakMemoryMB <= 100.0 // < 100MB at 50 user capacity
	latencyPass := p95Latency <= 100*time.Millisecond // < 100ms
	systemResponsivePass := avgLatency <= 200*time.Millisecond // System remains responsive
	
	result := CapacityTestResult{
		Level:                 config.Level,
		TargetUsers:          config.TargetUsers,
		ActualUsers:          actualUsers,
		StudentsConnected:    len(students),
		InstructorsConnected: len(instructors),
		
		TestDuration:         actualDuration,
		MessagesProcessed:    messagesProcessed,
		MessageErrors:        messageErrors,
		ErrorRate:            errorRate,
		
		AvgLatency:           avgLatency,
		P95Latency:           p95Latency,
		MaxLatency:           maxLatency,
		
		PeakMemoryMB:         finalPeakMemoryMB,
		FinalMemoryMB:        finalMemoryMB,
		
		ConnectionErrors:     atomic.LoadInt64(&connectionErrors),
		ConnectionSuccessRate: connSuccessRate,
		
		ErrorRatePass:        errorRatePass,
		MemoryUsagePass:      memoryUsagePass,
		LatencyPass:          latencyPass,
		SystemResponsivePass: systemResponsivePass,
		
		QuestionsGenerated:    atomic.LoadInt64(&questionsGenerated),
		ResponsesGenerated:    atomic.LoadInt64(&responsesGenerated),
		AnnouncementsGenerated: atomic.LoadInt64(&announcementsGenerated),
	}
	
	// Log capacity level results
	t.Logf("\n=== Capacity Level %d Results ===", config.Level)
	t.Logf("Users: %d/%d connected (%.1f%% success)", actualUsers, config.TargetUsers, connSuccessRate)
	t.Logf("Duration: %v", actualDuration)
	t.Logf("Messages: %d processed, %d errors (%.2f%% error rate)", messagesProcessed, messageErrors, errorRate)
	t.Logf("Activity: %d questions, %d responses, %d announcements", 
		atomic.LoadInt64(&questionsGenerated), 
		atomic.LoadInt64(&responsesGenerated), 
		atomic.LoadInt64(&announcementsGenerated))
	t.Logf("Latency: %v avg, %v 95th percentile, %v max", avgLatency, p95Latency, maxLatency)
	t.Logf("Memory: %.1f MB peak, %.1f MB final", finalPeakMemoryMB, finalMemoryMB)
	t.Logf("Success Criteria: Error Rate %s, Memory %s, Latency %s, Responsive %s",
		passFailIcon(errorRatePass), passFailIcon(memoryUsagePass), 
		passFailIcon(latencyPass), passFailIcon(systemResponsivePass))
	
	return result
}

func analyzeCapacityResults(t *testing.T, results []CapacityTestResult, maxErrorRate, maxMemoryMB float64, maxLatencyMs int) {
	t.Logf("\n🔍 === Comprehensive Capacity Analysis ===")
	
	if len(results) == 0 {
		t.Logf("❌ No capacity test results to analyze")
		return
	}
	
	// Find successful capacity levels
	var successfulLevels []CapacityTestResult
	var degradationPoint = -1
	
	for i, result := range results {
		allCriteriaMet := result.ErrorRatePass && result.MemoryUsagePass && result.LatencyPass && result.SystemResponsivePass
		
		if allCriteriaMet {
			successfulLevels = append(successfulLevels, result)
		} else if degradationPoint == -1 {
			degradationPoint = i
		}
	}
	
	t.Logf("\n📈 Capacity Scaling Results:")
	for _, result := range results {
		status := "✅ PASS"
		if !result.ErrorRatePass || !result.MemoryUsagePass || !result.LatencyPass || !result.SystemResponsivePass {
			status = "❌ FAIL"
		}
		
		efficiency := "N/A"
		if result.TargetUsers > 0 {
			efficiency = fmt.Sprintf("%.1f%%", float64(result.ActualUsers)/float64(result.TargetUsers)*100)
		}
		
		t.Logf("Level %d: %3d users (%s conn) | %.1f%% errors | %.1f MB | %v latency | %s",
			result.Level, result.ActualUsers, efficiency, result.ErrorRate, result.PeakMemoryMB, result.P95Latency, status)
	}
	
	// Success criteria validation
	t.Logf("\n✅ Success Criteria Validation:")
	
	// Find highest successful capacity
	maxSuccessfulUsers := 0
	var bestResult *CapacityTestResult
	
	for _, result := range successfulLevels {
		if result.ActualUsers > maxSuccessfulUsers {
			maxSuccessfulUsers = result.ActualUsers
			bestResult = &result
		}
	}
	
	// Criterion 1: 50+ concurrent users with <5% error rate
	criterion1Pass := maxSuccessfulUsers >= 50
	t.Logf("1. 50+ concurrent users with <5%% error rate:")
	t.Logf("   Maximum successful capacity: %d users", maxSuccessfulUsers)
	if bestResult != nil {
		t.Logf("   Best result error rate: %.2f%%", bestResult.ErrorRate)
	}
	t.Logf("   Status: %s", passFailIcon(criterion1Pass))
	
	// Criterion 2: Memory usage <100MB at 50 user capacity
	var memoryAt50Users float64 = -1
	var criterion2Pass bool
	
	for _, result := range results {
		if result.ActualUsers >= 50 {
			memoryAt50Users = result.PeakMemoryMB
			criterion2Pass = memoryAt50Users <= maxMemoryMB
			break
		}
	}
	
	t.Logf("2. Memory usage <%v MB at 50 user capacity:", maxMemoryMB)
	if memoryAt50Users >= 0 {
		t.Logf("   Memory usage at 50+ users: %.1f MB", memoryAt50Users)
		t.Logf("   Status: %s", passFailIcon(criterion2Pass))
	} else {
		t.Logf("   50 user capacity not reached")
		t.Logf("   Status: ❌ FAIL")
		criterion2Pass = false
	}
	
	// Criterion 3: Message delivery latency <100ms
	var latencyAt50Users time.Duration
	var criterion3Pass bool
	
	for _, result := range results {
		if result.ActualUsers >= 50 {
			latencyAt50Users = result.P95Latency
			criterion3Pass = latencyAt50Users <= time.Duration(maxLatencyMs)*time.Millisecond
			break
		}
	}
	
	t.Logf("3. Message delivery latency <%d ms:", maxLatencyMs)
	if latencyAt50Users > 0 {
		t.Logf("   95th percentile latency at 50+ users: %v", latencyAt50Users)
		t.Logf("   Status: %s", passFailIcon(criterion3Pass))
	} else {
		t.Logf("   50 user capacity not reached")
		t.Logf("   Status: ❌ FAIL")
		criterion3Pass = false
	}
	
	// Criterion 4: System remains responsive during capacity tests
	criterion4Pass := true
	maxAvgLatency := time.Duration(0)
	for _, result := range results {
		if result.AvgLatency > maxAvgLatency {
			maxAvgLatency = result.AvgLatency
		}
		if result.AvgLatency > 500*time.Millisecond { // Consider >500ms as unresponsive
			criterion4Pass = false
		}
	}
	
	t.Logf("4. System remains responsive during capacity tests:")
	t.Logf("   Maximum average latency: %v", maxAvgLatency)
	t.Logf("   Status: %s", passFailIcon(criterion4Pass))
	
	// Criterion 5: Graceful performance degradation beyond limits
	criterion5Pass := true
	if degradationPoint >= 0 && degradationPoint < len(results)-1 {
		// Check if system didn't crash completely
		degradedResult := results[degradationPoint]
		if degradedResult.ConnectionSuccessRate < 50.0 || degradedResult.ErrorRate > 80.0 {
			criterion5Pass = false
		}
	}
	
	t.Logf("5. Graceful performance degradation beyond limits:")
	if degradationPoint >= 0 {
		degradedResult := results[degradationPoint]
		t.Logf("   Degradation point: Level %d (%d users)", degradedResult.Level, degradedResult.ActualUsers)
		t.Logf("   Connection success rate: %.1f%%", degradedResult.ConnectionSuccessRate)
		t.Logf("   Error rate: %.1f%%", degradedResult.ErrorRate)
		t.Logf("   Status: %s", passFailIcon(criterion5Pass))
	} else {
		t.Logf("   No degradation detected within test range")
		t.Logf("   Status: ✅ PASS")
	}
	
	// Overall test result
	allCriteriaPassed := criterion1Pass && criterion2Pass && criterion3Pass && criterion4Pass && criterion5Pass
	
	t.Logf("\n🏆 === Final Test Result ===")
	criteriaCount := 0
	if criterion1Pass { criteriaCount++ }
	if criterion2Pass { criteriaCount++ }
	if criterion3Pass { criteriaCount++ }
	if criterion4Pass { criteriaCount++ }
	if criterion5Pass { criteriaCount++ }
	
	t.Logf("Success Criteria: %d/5 passed", criteriaCount)
	
	if allCriteriaPassed {
		t.Logf("✅ Test 3.2 PASSED - All success criteria met")
		t.Logf("🎯 System validated for 50+ concurrent user capacity")
		t.Logf("📊 Maximum validated capacity: %d users", maxSuccessfulUsers)
	} else {
		t.Logf("❌ Test 3.2 FAILED - %d/5 success criteria not met", 5-criteriaCount)
		if !criterion1Pass { t.Logf("   - 50+ user capacity requirement not met") }
		if !criterion2Pass { t.Logf("   - Memory usage requirement not met") }
		if !criterion3Pass { t.Logf("   - Latency requirement not met") }
		if !criterion4Pass { t.Logf("   - System responsiveness requirement not met") }
		if !criterion5Pass { t.Logf("   - Graceful degradation requirement not met") }
		
		if maxSuccessfulUsers > 0 {
			t.Logf("📊 Maximum validated capacity: %d users", maxSuccessfulUsers)
		} else {
			t.Logf("⚠️  No capacity level met all requirements")
		}
	}
	
	// Performance insights
	t.Logf("\n💡 Performance Insights:")
	
	if len(results) > 1 {
		// Memory scaling analysis
		memoryGrowthRate := (results[len(results)-1].PeakMemoryMB - results[0].PeakMemoryMB) / float64(results[len(results)-1].ActualUsers - results[0].ActualUsers)
		t.Logf("• Memory scaling: %.2f MB per additional user", memoryGrowthRate)
		
		// Error rate progression
		if results[len(results)-1].ErrorRate > results[0].ErrorRate {
			t.Logf("• Error rate increased from %.2f%% to %.2f%% with scale", results[0].ErrorRate, results[len(results)-1].ErrorRate)
		}
		
		// Latency progression
		latencyIncrease := results[len(results)-1].P95Latency - results[0].P95Latency
		if latencyIncrease > 0 {
			t.Logf("• Latency increased by %v with scale (95th percentile)", latencyIncrease)
		}
	}
	
	// Recommendations
	t.Logf("\n📋 Recommendations:")
	if maxSuccessfulUsers < 50 {
		t.Logf("• System requires optimization to handle 50+ concurrent users")
		t.Logf("• Consider database connection pooling and connection management improvements")
		t.Logf("• Review message processing pipeline for bottlenecks")
	} else if maxSuccessfulUsers >= 50 && maxSuccessfulUsers < 100 {
		t.Logf("• System meets minimum requirements but has room for improvement")
		t.Logf("• Monitor memory usage patterns for optimization opportunities")
	} else {
		t.Logf("• System performs well beyond minimum requirements")
		t.Logf("• Consider this configuration as the baseline for production deployment")
	}
	
	// Final assertions
	assert.True(t, criterion1Pass, "Must support 50+ concurrent users with <5%% error rate (achieved: %d users)", maxSuccessfulUsers)
	assert.True(t, criterion2Pass, "Memory usage must be <100MB at 50 user capacity (achieved: %.1f MB)", memoryAt50Users)
	assert.True(t, criterion3Pass, "Message delivery latency must be <100ms (achieved: %v)", latencyAt50Users)
	assert.True(t, criterion4Pass, "System must remain responsive during capacity tests (max avg latency: %v)", maxAvgLatency)
	assert.True(t, criterion5Pass, "Must demonstrate graceful performance degradation beyond limits")
}

// Helper types and functions

type CapacityTestEnvironment struct {
	dbManager          *database.SQLiteDatabaseManager
	sessionManager     session.SessionManager
	connectionRegistry *websocket.ConnectionRegistry
	processor          *message.MessageProcessor
}

func setupCapacityTestEnvironment(t *testing.T) *CapacityTestEnvironment {
	// Setup in-memory database
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Apply schema
	schema, err := os.ReadFile("../../internal/database/migrations.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(schema))
	require.NoError(t, err)

	// Create components optimized for capacity testing
	dbManager, err := database.NewSQLiteDatabaseManagerWithOptions(db, database.SQLiteOptions{
		SkipTableInit: true,
		SkipPragmas:   true,
	})
	require.NoError(t, err)
	require.NoError(t, dbManager.Start())

	sessionManager := session.NewSessionManager(dbManager)
	connectionRegistry := websocket.NewConnectionRegistry(sessionManager)
	connectionRegistry.Start()

	roleBasedFilter := message.NewRoleBasedFilter()
	messageRouter := message.NewMessageRouter(connectionRegistry, roleBasedFilter)
	
	// Use higher rate limits for capacity testing (300 messages per minute per user)
	rateLimiter := rate.NewRateLimiterWithConfig(300, time.Minute)

	filterAdapter := websocket.NewFilterAdapter(roleBasedFilter)
	broadcastSystem := websocket.NewBroadcastSystem(connectionRegistry, filterAdapter)

	processor := message.NewMessageProcessor(
		sessionManager,
		dbManager,
		rateLimiter,
		messageRouter,
		broadcastSystem,
	)

	return &CapacityTestEnvironment{
		dbManager:          dbManager,
		sessionManager:     sessionManager,
		connectionRegistry: connectionRegistry,
		processor:          processor,
	}
}

func (env *CapacityTestEnvironment) cleanup() {
	if env.connectionRegistry != nil {
		env.connectionRegistry.Stop()
	}
	if env.dbManager != nil {
		env.dbManager.Stop()
	}
}

func generateRealisticStudentQuestion(studentIndex, questionIndex int) string {
	questions := []string{
		"Can you explain the concept of photosynthesis in more detail?",
		"What's the difference between DNA and RNA?",
		"How do enzymes work in biochemical reactions?",
		"Could you clarify the process of cellular respiration?",
		"What are the main stages of mitosis?",
		"How does natural selection drive evolution?",
		"Can you explain the structure of proteins?",
		"What role do ribosomes play in protein synthesis?",
		"How do genetic mutations occur?",
		"What is the significance of biodiversity?",
		"Could you explain the carbon cycle?",
		"How do vaccines work to prevent diseases?",
		"What are the different types of ecosystems?",
		"Can you describe the process of meiosis?",
		"How do neurons transmit electrical signals?",
	}
	
	question := questions[questionIndex%len(questions)]
	return fmt.Sprintf("[Student %d] %s", studentIndex+1, question)
}

func generateRealisticInstructorResponse(instructorIndex, responseIndex int) string {
	responses := []string{
		"Great question! Let me break that down step by step...",
		"That's an excellent observation. The key point to understand is...",
		"I'm glad you asked about that. Here's how it works...",
		"That's a common question, and the answer involves several factors...",
		"Perfect timing for that question! The explanation is...",
		"Interesting question! This relates to what we discussed earlier...",
		"Good thinking! The biological process behind this is...",
		"That's exactly the right question to ask. Consider this...",
		"Excellent! This is a fundamental concept in biology...",
		"I appreciate that question. The scientific explanation is...",
	}
	
	response := responses[responseIndex%len(responses)]
	return fmt.Sprintf("[Instructor %d] %s", instructorIndex+1, response)
}

func generateCapacityTestAnnouncement(instructorIndex, announcementIndex int) string {
	announcements := []string{
		"Welcome everyone! Let's begin today's discussion on cellular biology.",
		"Great participation so far! Keep the questions coming.",
		"Let's take a brief pause to summarize what we've covered.",
		"I'll be sharing additional resources after this session.",
		"Excellent questions today! This shows real engagement with the material.",
		"Let's focus on the practical applications of these concepts.",
		"Remember to review the assigned readings for next week.",
		"We'll have a few more minutes for questions before we wrap up.",
		"This topic connects well with what we'll study next week.",
		"I can see some great critical thinking in your questions!",
	}
	
	announcement := announcements[announcementIndex%len(announcements)]
	return fmt.Sprintf("[Instructor %d] %s", instructorIndex+1, announcement)
}

func createStudentQuestion(content string) []byte {
	msg := map[string]interface{}{
		"type":    database.MessageTypeBroadcastToInstructors,
		"context": database.ContextQuestion,
		"content": map[string]interface{}{"text": content},
	}
	data, _ := json.Marshal(msg)
	return data
}

func createDirectResponse(toUser, content string) []byte {
	msg := map[string]interface{}{
		"type":    database.MessageTypeDirectMessage,
		"context": database.ContextResponse,
		"content": map[string]interface{}{"text": content},
		"to_user": toUser,
	}
	data, _ := json.Marshal(msg)
	return data
}

func createAnnouncement(content string) []byte {
	msg := map[string]interface{}{
		"type":    database.MessageTypeBroadcastToStudents,
		"context": database.ContextAnnouncement,
		"content": map[string]interface{}{"text": content},
	}
	data, _ := json.Marshal(msg)
	return data
}

func passFailIcon(passed bool) string {
	if passed {
		return "✅ PASS"
	}
	return "❌ FAIL"
}

// CapacityConnection optimized for capacity testing
type CapacityConnection struct {
	userID             string
	role               string
	messagesDelivered  int64
	lastActivity       time.Time
	mu                 sync.RWMutex
}

func (cc *CapacityConnection) GetUserID() string { return cc.userID }
func (cc *CapacityConnection) GetRole() string   { return cc.role }

func (cc *CapacityConnection) SendMessage(data []byte) error {
	atomic.AddInt64(&cc.messagesDelivered, 1)
	cc.mu.Lock()
	cc.lastActivity = time.Now()
	cc.mu.Unlock()
	return nil
}

func (cc *CapacityConnection) GetDeliveredMessageCount() int64 {
	return atomic.LoadInt64(&cc.messagesDelivered)
}

func (cc *CapacityConnection) UpdateActivity() {
	cc.mu.Lock()
	cc.lastActivity = time.Now()
	cc.mu.Unlock()
}

func (cc *CapacityConnection) GetLastSeen() time.Time {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	if cc.lastActivity.IsZero() {
		return time.Now() // Return current time if no activity recorded
	}
	return cc.lastActivity
}

func (cc *CapacityConnection) Close() error                               { return nil }
func (cc *CapacityConnection) WriteJSON(v interface{}) error              { return nil }
func (cc *CapacityConnection) SetCredentials(username, role string) error { return nil }
func (cc *CapacityConnection) SendCloseMessage(reason string) error       { return nil }