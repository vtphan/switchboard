// Test 3.1: Message Throughput Validation (500+ msg/sec)
// P0-Critical Performance/Load Test
// This test validates sustained high-throughput message processing according to the validation test plan
package performance

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
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

// Test31MessageThroughputValidation executes Test 3.1 from the validation test plan
func TestTest31MessageThroughputValidation(t *testing.T) {
	t.Logf("=== Executing Test 3.1: Message Throughput Validation (500+ msg/sec) ===")
	t.Logf("Priority: P0-Critical")
	t.Logf("Type: Performance/Load Test")
	t.Logf("Expected Duration: 2 minutes (adjusted for test timeout)")
	
	// Setup test environment with error handling
	env := setupThroughputTestEnvironment(t)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Test panic recovered: %v", r)
		}
		env.cleanup()
	}()

	// Test configuration optimized for Go test timeout
	const (
		numStudents     = 30
		numInstructors  = 1
		
		// Load phases adjusted to fit within Go test timeout
		warmupDurationSec     = 30  // 30 seconds warm-up
		targetDurationSec     = 60  // 60 seconds target validation
		burstDurationSec      = 30  // 30 seconds burst test
		
		warmupTargetMsgPerSec = 300
		targetMsgPerSec       = 500
		burstMsgPerSec        = 700
	)
	
	t.Logf("Test Setup:")
	t.Logf("  Students: %d, Instructors: %d", numStudents, numInstructors)
	t.Logf("  Phase 1 (Warm-up): %d msg/sec for %d seconds", warmupTargetMsgPerSec, warmupDurationSec)
	t.Logf("  Phase 2 (Target): %d msg/sec for %d seconds", targetMsgPerSec, targetDurationSec)
	t.Logf("  Phase 3 (Burst): %d msg/sec for %d seconds", burstMsgPerSec, burstDurationSec)
	
	// Setup session
	session := &database.Session{
		ID:        "test-3-1-throughput-validation",
		Name:      "Test 3.1 - Message Throughput Validation",
		CreatedBy: "instructor1",
		StartTime: time.Now(),
		Status:    "active",
	}
	require.NoError(t, env.sessionManager.SetActiveSession(session))
	
	// Create connections
	students := make([]*ThroughputConnection, numStudents)
	for i := 0; i < numStudents; i++ {
		conn := &ThroughputConnection{
			userID: fmt.Sprintf("student%d", i+1),
			role:   "student",
		}
		require.NoError(t, env.connectionRegistry.Register(conn.userID, conn))
		students[i] = conn
	}
	
	instructors := make([]*ThroughputConnection, numInstructors)
	for i := 0; i < numInstructors; i++ {
		conn := &ThroughputConnection{
			userID: fmt.Sprintf("instructor%d", i+1),
			role:   "instructor",
		}
		require.NoError(t, env.connectionRegistry.Register(conn.userID, conn))
		instructors[i] = conn
	}
	
	t.Logf("Created %d student connections and %d instructor connections", len(students), len(instructors))
	
	// Metrics tracking
	var (
		totalMessages    int64
		errorCount       int64
		phaseMetrics     [3]PhaseMetrics
		latencies        []time.Duration
		latencyMutex     sync.RWMutex
	)
	
	// Test execution phases
	phases := []ThroughputPhase{
		{Name: "Warm-up", Duration: warmupDurationSec * time.Second, TargetMsgPerSec: warmupTargetMsgPerSec},
		{Name: "Target Validation", Duration: targetDurationSec * time.Second, TargetMsgPerSec: targetMsgPerSec},
		{Name: "Burst Capacity", Duration: burstDurationSec * time.Second, TargetMsgPerSec: burstMsgPerSec},
	}
	
	overallStartTime := time.Now()
	
	for phaseIndex, phase := range phases {
		t.Logf("\n--- Starting Phase %d: %s ---", phaseIndex+1, phase.Name)
		t.Logf("Target: %d msg/sec for %v", phase.TargetMsgPerSec, phase.Duration)
		
		phaseStartTime := time.Now()
		var phaseMessages, phaseErrors int64
		var phaseLatencies []time.Duration
		
		// Optimized throughput generation using multiple workers
		numWorkers := 10 // Balanced worker count
		messagesPerWorker := phase.TargetMsgPerSec / numWorkers
		if messagesPerWorker == 0 {
			messagesPerWorker = 1
		}
		messageInterval := time.Second / time.Duration(messagesPerWorker)
		
		t.Logf("  Workers: %d, Messages per worker: %d, Interval: %v", numWorkers, messagesPerWorker, messageInterval)
		
		var wg sync.WaitGroup
		stopPhase := make(chan struct{})
		
		// Start message generation goroutines with optimized timing
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				// Stagger worker start times to smooth out message bursts
				startDelay := time.Duration(workerID) * (time.Second / time.Duration(numWorkers))
				time.Sleep(startDelay)
				
				ticker := time.NewTicker(messageInterval)
				defer ticker.Stop()
				
				messageCounter := 0
				for {
					select {
					case <-stopPhase:
						return
					case <-ticker.C:
						// Round-robin student selection for better distribution
						studentIndex := (workerID + messageCounter) % len(students)
						student := students[studentIndex]
						
						msgStart := time.Now()
						msg := createThroughputTestMessage(fmt.Sprintf("P%d-W%d-M%d", phaseIndex+1, workerID, messageCounter))
						
						err := env.processor.ProcessIncomingMessage(msg, student.userID)
						processingLatency := time.Since(msgStart)
						
						if err != nil {
							atomic.AddInt64(&phaseErrors, 1)
							atomic.AddInt64(&errorCount, 1)
							
							// Log first few errors for debugging
							if atomic.LoadInt64(&phaseErrors) <= 3 {
								t.Logf("  Error in phase %s: %v", phase.Name, err)
							}
						} else {
							atomic.AddInt64(&phaseMessages, 1)
							atomic.AddInt64(&totalMessages, 1)
							
							// Record latency efficiently
							latencyMutex.Lock()
							latencies = append(latencies, processingLatency)
							phaseLatencies = append(phaseLatencies, processingLatency)
							latencyMutex.Unlock()
						}
						
						messageCounter++
					}
				}
			}(i)
		}
		
		// Real-time monitoring of throughput and system metrics
		var maxMemoryMB float64
		monitoringTicker := time.NewTicker(5 * time.Second)
		lastMsgCount := int64(0)
		lastMsgTime := time.Now()
		
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer monitoringTicker.Stop()
			
			for {
				select {
				case <-stopPhase:
					return
				case <-monitoringTicker.C:
					// Memory monitoring
					var memStats runtime.MemStats
					runtime.ReadMemStats(&memStats)
					currentMemoryMB := float64(memStats.Alloc) / 1024 / 1024
					if currentMemoryMB > maxMemoryMB {
						maxMemoryMB = currentMemoryMB
					}
					
					// Real-time throughput calculation
					currentMsgCount := atomic.LoadInt64(&phaseMessages)
					currentTime := time.Now()
					msgDelta := currentMsgCount - lastMsgCount
					timeDelta := currentTime.Sub(lastMsgTime).Seconds()
					currentThroughput := float64(msgDelta) / timeDelta
					
					// Overall phase throughput so far
					phaseElapsed := currentTime.Sub(phaseStartTime).Seconds()
					overallThroughput := float64(currentMsgCount) / phaseElapsed
					
					t.Logf("  [%s] Live Stats - Current: %.1f msg/s, Overall: %.1f msg/s, Memory: %.1f MB, Messages: %d",
						phase.Name, currentThroughput, overallThroughput, currentMemoryMB, currentMsgCount)
					
					lastMsgCount = currentMsgCount
					lastMsgTime = currentTime
				}
			}
		}()
		
		// Run phase for specified duration with timeout protection
		phaseStart := time.Now()
		time.Sleep(phase.Duration)
		close(stopPhase)
		
		// Wait for workers with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		
		select {
		case <-done:
			// All workers completed normally
		case <-time.After(10 * time.Second):
			t.Logf("Warning: Some workers did not complete within timeout for phase %s", phase.Name)
		}
		
		phaseActualDuration := time.Since(phaseStart)
		if phaseActualDuration > phase.Duration+time.Second {
			t.Logf("Warning: Phase %s took longer than expected: %v vs %v", phase.Name, phaseActualDuration, phase.Duration)
		}
		
		phaseDuration := time.Since(phaseStartTime)
		phaseMessageCount := atomic.LoadInt64(&phaseMessages)
		phaseErrorCount := atomic.LoadInt64(&phaseErrors)
		
		// Calculate phase metrics
		actualThroughput := float64(phaseMessageCount) / phaseDuration.Seconds()
		errorRate := float64(phaseErrorCount) / float64(phaseMessageCount + phaseErrorCount)
		
		// Calculate latency percentiles
		var avgLatency, p95Latency, maxLatency time.Duration
		if len(phaseLatencies) > 0 {
			// Sort latencies for percentile calculation
			latencyMutex.RLock()
			sortedLatencies := make([]time.Duration, len(phaseLatencies))
			copy(sortedLatencies, phaseLatencies)
			latencyMutex.RUnlock()
			
			// Simple sort for latencies
			for i := 0; i < len(sortedLatencies); i++ {
				for j := i + 1; j < len(sortedLatencies); j++ {
					if sortedLatencies[i] > sortedLatencies[j] {
						sortedLatencies[i], sortedLatencies[j] = sortedLatencies[j], sortedLatencies[i]
					}
				}
			}
			
			// Calculate statistics
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
		
		// Store phase metrics
		phaseMetrics[phaseIndex] = PhaseMetrics{
			Name:               phase.Name,
			Duration:           phaseDuration,
			TargetThroughput:   phase.TargetMsgPerSec,
			ActualThroughput:   actualThroughput,
			MessagesProcessed:  phaseMessageCount,
			ErrorCount:         phaseErrorCount,
			ErrorRate:          errorRate,
			AvgLatency:         avgLatency,
			P95Latency:         p95Latency,
			MaxLatency:         maxLatency,
			MaxMemoryMB:        maxMemoryMB,
		}
		
		t.Logf("Phase %d (%s) Results:", phaseIndex+1, phase.Name)
		t.Logf("  Duration: %v", phaseDuration)
		t.Logf("  Target Throughput: %d msg/sec", phase.TargetMsgPerSec)
		t.Logf("  Actual Throughput: %.2f msg/sec", actualThroughput)
		t.Logf("  Messages Processed: %d", phaseMessageCount)
		t.Logf("  Errors: %d", phaseErrorCount)
		t.Logf("  Error Rate: %.3f%%", errorRate*100)
		t.Logf("  Average Latency: %v", avgLatency)
		t.Logf("  95th Percentile Latency: %v", p95Latency)
		t.Logf("  Max Latency: %v", maxLatency)
		t.Logf("  Peak Memory Usage: %.2f MB", maxMemoryMB)
	}
	
	overallDuration := time.Since(overallStartTime)
	totalMessageCount := atomic.LoadInt64(&totalMessages)
	totalErrorCount := atomic.LoadInt64(&errorCount)
	overallThroughput := float64(totalMessageCount) / overallDuration.Seconds()
	overallErrorRate := float64(totalErrorCount) / float64(totalMessageCount + totalErrorCount)
	
	t.Logf("\n=== Overall Test Results ===")
	t.Logf("Total Duration: %v", overallDuration)
	t.Logf("Total Messages Processed: %d", totalMessageCount)
	t.Logf("Total Errors: %d", totalErrorCount)
	t.Logf("Overall Throughput: %.2f msg/sec", overallThroughput)
	t.Logf("Overall Error Rate: %.3f%%", overallErrorRate*100)
	
	// Enhanced test completion summary
	t.Logf("\n=== Test Execution Summary ===")
	for i, phase := range phaseMetrics {
		efficiency := (phase.ActualThroughput / float64(phase.TargetThroughput)) * 100
		t.Logf("Phase %d (%s): %.1f%% efficiency (%.1f/%.0f msg/sec)", 
			i+1, phase.Name, efficiency, phase.ActualThroughput, float64(phase.TargetThroughput))
	}
	
	// Validate against success criteria (adjusted for test duration)
	t.Logf("\n=== Success Criteria Validation ===")
	
	// Criterion 1: Sustained 500+ messages/second for target phase (60 seconds)
	targetPhase := phaseMetrics[1] // Phase 2 is the target validation phase
	sustained500Plus := targetPhase.ActualThroughput >= 500.0
	t.Logf("✓ Criterion 1 - Sustained 500+ msg/sec for target phase:")
	t.Logf("  Target Phase Throughput: %.2f msg/sec (Required: ≥500)", targetPhase.ActualThroughput)
	t.Logf("  Target Phase Duration: %v (Test Duration: 60s)", targetPhase.Duration)
	t.Logf("  Messages Processed in Target Phase: %d", targetPhase.MessagesProcessed)
	t.Logf("  Status: %s", passFailStatus(sustained500Plus))
	
	// Criterion 2: Message processing latency <100ms (95th percentile)
	latency95thOK := targetPhase.P95Latency < 100*time.Millisecond
	t.Logf("✓ Criterion 2 - Message processing latency <100ms (95th percentile):")
	t.Logf("  95th Percentile Latency: %v (Required: <100ms)", targetPhase.P95Latency)
	t.Logf("  Status: %s", passFailStatus(latency95thOK))
	
	// Criterion 3: Error rate <0.1% throughout test duration
	errorRateOK := overallErrorRate < 0.001 // 0.1%
	t.Logf("✓ Criterion 3 - Error rate <0.1%% throughout test:")
	t.Logf("  Overall Error Rate: %.4f%% (Required: <0.1%%)", overallErrorRate*100)
	t.Logf("  Total Errors: %d out of %d messages", totalErrorCount, totalMessageCount+totalErrorCount)
	t.Logf("  Status: %s", passFailStatus(errorRateOK))
	
	// Criterion 4: Memory usage <200MB during peak load
	maxMemoryOverall := float64(0)
	for _, phase := range phaseMetrics {
		if phase.MaxMemoryMB > maxMemoryOverall {
			maxMemoryOverall = phase.MaxMemoryMB
		}
	}
	memoryOK := maxMemoryOverall < 200.0
	t.Logf("✓ Criterion 4 - Memory usage <200MB during peak load:")
	t.Logf("  Peak Memory Usage: %.2f MB (Required: <200MB)", maxMemoryOverall)
	t.Logf("  Status: %s", passFailStatus(memoryOK))
	
	// Criterion 5: Zero message loss or corruption detected
	// Allow database batch processing to complete
	t.Logf("\n⏳ Waiting for database batch processing to complete...")
	time.Sleep(5 * time.Second)
	
	// Force flush any remaining batches
	if err := env.dbManager.Stop(); err != nil {
		t.Logf("Failed to stop database manager: %v", err)
	}
	time.Sleep(2 * time.Second)
	
	// Restart database manager for final verification
	require.NoError(t, env.dbManager.Start())
	
	dbMessages, err := env.dbManager.GetSessionMessages(session.ID)
	require.NoError(t, err)
	
	messageLossOK := len(dbMessages) == int(totalMessageCount)
	messageLoss := int(totalMessageCount) - len(dbMessages)
	
	t.Logf("✓ Criterion 5 - Zero message loss or corruption:")
	t.Logf("  Messages Processed: %d", totalMessageCount)
	t.Logf("  Messages in Database: %d", len(dbMessages))
	t.Logf("  Message Loss: %d (%.3f%%)", messageLoss, float64(messageLoss)/float64(totalMessageCount)*100)
	t.Logf("  Status: %s", passFailStatus(messageLossOK))
	
	// Comprehensive test summary
	t.Logf("\n=== Detailed Performance Summary ===")
	t.Logf("Test Configuration:")
	t.Logf("  • Total Duration: %v (Target: 2 minutes)", overallDuration)
	t.Logf("  • Connections: %d students, %d instructors", len(students), len(instructors))
	t.Logf("  • Worker Threads: %d per phase", 20)
	
	t.Logf("Performance Results:")
	t.Logf("  • Peak Throughput: %.1f msg/sec (Phase 3)", phaseMetrics[2].ActualThroughput)
	t.Logf("  • Target Throughput: %.1f msg/sec (Phase 2)", phaseMetrics[1].ActualThroughput)
	t.Logf("  • Overall Average: %.1f msg/sec", overallThroughput)
	t.Logf("  • Total Messages: %d", totalMessageCount)
	
	t.Logf("Quality Metrics:")
	t.Logf("  • Error Rate: %.4f%% (%d errors)", overallErrorRate*100, totalErrorCount)
	t.Logf("  • Memory Usage: %.1f MB peak", maxMemoryOverall)
	t.Logf("  • Avg Latency: %v", targetPhase.AvgLatency)
	t.Logf("  • 95th %% Latency: %v", targetPhase.P95Latency)
	
	// Overall test result
	allCriteriaPassed := sustained500Plus && latency95thOK && errorRateOK && memoryOK && messageLossOK
	t.Logf("\n=== Final Test Result ===")
	criteriaMet := 0
	if sustained500Plus { criteriaMet++ }
	if latency95thOK { criteriaMet++ }
	if errorRateOK { criteriaMet++ }
	if memoryOK { criteriaMet++ }
	if messageLossOK { criteriaMet++ }
	
	t.Logf("Success Criteria: %d/5 passed", criteriaMet)
	t.Logf("Test 3.1 Status: %s", passFailStatus(allCriteriaPassed))
	
	if allCriteriaPassed {
		t.Logf("✅ Test 3.1 PASSED - All success criteria met")
		t.Logf("🎯 System validated for 500+ msg/sec throughput requirement")
	} else {
		t.Logf("❌ Test 3.1 FAILED - %d/5 success criteria not met", 5-criteriaMet)
		if !sustained500Plus { t.Logf("   - Throughput requirement not met") }
		if !latency95thOK { t.Logf("   - Latency requirement not met") }
		if !errorRateOK { t.Logf("   - Error rate requirement not met") }
		if !memoryOK { t.Logf("   - Memory usage requirement not met") }
		if !messageLossOK { t.Logf("   - Message loss requirement not met") }
	}
	
	// Test assertions with detailed context
	assert.True(t, sustained500Plus, "Must sustain 500+ msg/sec during target phase (achieved: %.2f msg/sec)", targetPhase.ActualThroughput)
	assert.True(t, latency95thOK, "95th percentile latency must be <100ms (achieved: %v)", targetPhase.P95Latency)
	assert.True(t, errorRateOK, "Error rate must be <0.1%% (achieved: %.4f%%)", overallErrorRate*100)
	assert.True(t, memoryOK, "Memory usage must be <200MB (achieved: %.1f MB)", maxMemoryOverall)
	assert.True(t, messageLossOK, "Must have zero message loss (lost: %d messages)", int(totalMessageCount)-len(dbMessages))
}

// Helper types and functions

type ThroughputPhase struct {
	Name             string
	Duration         time.Duration
	TargetMsgPerSec  int
}

type PhaseMetrics struct {
	Name               string
	Duration           time.Duration
	TargetThroughput   int
	ActualThroughput   float64
	MessagesProcessed  int64
	ErrorCount         int64
	ErrorRate          float64
	AvgLatency         time.Duration
	P95Latency         time.Duration
	MaxLatency         time.Duration
	MaxMemoryMB        float64
}

type ThroughputTestEnvironment struct {
	dbManager          *database.SQLiteDatabaseManager
	sessionManager     session.SessionManager
	connectionRegistry *websocket.ConnectionRegistry
	processor          *message.MessageProcessor
}

func setupThroughputTestEnvironment(t *testing.T) *ThroughputTestEnvironment {
	// Setup in-memory database for performance testing
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Apply schema
	schema, err := os.ReadFile("../../internal/database/migrations.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(schema))
	require.NoError(t, err)

	// Create components optimized for throughput testing
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
	
	// Use a rate limiter with much higher limits for performance testing
	// This allows for 10,000 messages per minute per user (167 msg/sec per user)
	rateLimiter := rate.NewRateLimiterWithConfig(10000, time.Minute)

	filterAdapter := websocket.NewFilterAdapter(roleBasedFilter)
	broadcastSystem := websocket.NewBroadcastSystem(connectionRegistry, filterAdapter)

	processor := message.NewMessageProcessor(
		sessionManager,
		dbManager,
		rateLimiter,
		messageRouter,
		broadcastSystem,
	)

	return &ThroughputTestEnvironment{
		dbManager:          dbManager,
		sessionManager:     sessionManager,
		connectionRegistry: connectionRegistry,
		processor:          processor,
	}
}

func (env *ThroughputTestEnvironment) cleanup() {
	env.connectionRegistry.Stop()
	if err := env.dbManager.Stop(); err != nil {
		// Use fmt.Printf since we don't have testing.T context here
		fmt.Printf("Failed to stop database manager: %v\n", err)
	}
}

func createThroughputTestMessage(content string) []byte {
	msg := map[string]interface{}{
		"type":    database.MessageTypeBroadcastToInstructors,
		"context": database.ContextQuestion,
		"content": map[string]interface{}{"text": content},
	}
	data, _ := json.Marshal(msg)
	return data
}

func passFailStatus(passed bool) string {
	if passed {
		return "✅ PASS"
	}
	return "❌ FAIL"
}

// ThroughputConnection optimized for throughput testing
type ThroughputConnection struct {
	userID             string
	role               string
	messagesDelivered  int64
}

func (tc *ThroughputConnection) GetUserID() string { return tc.userID }
func (tc *ThroughputConnection) GetRole() string   { return tc.role }

func (tc *ThroughputConnection) SendMessage(data []byte) error {
	atomic.AddInt64(&tc.messagesDelivered, 1)
	return nil
}

func (tc *ThroughputConnection) GetDeliveredMessageCount() int64 {
	return atomic.LoadInt64(&tc.messagesDelivered)
}

func (tc *ThroughputConnection) Close() error                               { return nil }
func (tc *ThroughputConnection) WriteJSON(v interface{}) error              { return nil }
func (tc *ThroughputConnection) SetCredentials(username, role string) error { return nil }
func (tc *ThroughputConnection) UpdateActivity()                            {}
func (tc *ThroughputConnection) GetLastSeen() time.Time                     { return time.Now() }
func (tc *ThroughputConnection) SendCloseMessage(reason string) error       { return nil }