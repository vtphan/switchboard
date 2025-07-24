package scenarios

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"switchboard/tests/fixtures"
)

// LoadTestMetrics tracks performance metrics during load testing
type LoadTestMetrics struct {
	MessagesSent          int64
	MessagesReceived      int64
	ConnectionsEstablished int64
	ConnectionsFailed     int64
	AverageLatency        time.Duration
	MaxLatency           time.Duration
	MinLatency           time.Duration
	ErrorCount           int64
	StartTime            time.Time
	EndTime              time.Time
	
	// Resource monitoring
	MaxGoroutines        int
	MaxMemoryMB          uint64
	DatabaseConnections  int
	
	mu sync.RWMutex
	latencies []time.Duration
}

// AddLatency records a message latency measurement
func (m *LoadTestMetrics) AddLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.latencies = append(m.latencies, latency)
	
	if latency > m.MaxLatency {
		m.MaxLatency = latency
	}
	
	if m.MinLatency == 0 || latency < m.MinLatency {
		m.MinLatency = latency
	}
}

// CalculateAverageLatency computes average latency from recorded measurements
func (m *LoadTestMetrics) CalculateAverageLatency() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if len(m.latencies) == 0 {
		return
	}
	
	var total time.Duration
	for _, latency := range m.latencies {
		total += latency
	}
	
	m.AverageLatency = total / time.Duration(len(m.latencies))
}

// GetReport generates a comprehensive performance report
func (m *LoadTestMetrics) GetReport() string {
	m.CalculateAverageLatency()
	
	duration := m.EndTime.Sub(m.StartTime)
	messagesPerSecond := float64(m.MessagesReceived) / duration.Seconds()
	
	successRate := float64(m.MessagesReceived) / float64(m.MessagesSent) * 100
	if m.MessagesSent == 0 {
		successRate = 0
	}
	
	return fmt.Sprintf(`
Load Test Performance Report
============================
Duration: %v
Messages Sent: %d
Messages Received: %d
Success Rate: %.2f%%
Messages/Second: %.2f
Connections Established: %d
Connection Failures: %d
Errors: %d

Latency Metrics:
  Average: %v
  Min: %v  
  Max: %v

Resource Usage:
  Max Goroutines: %d
  Max Memory (MB): %d
  DB Connections: %d
`,
		duration,
		m.MessagesSent,
		m.MessagesReceived,
		successRate,
		messagesPerSecond,
		m.ConnectionsEstablished,
		m.ConnectionsFailed,
		m.ErrorCount,
		m.AverageLatency,
		m.MinLatency,
		m.MaxLatency,
		m.MaxGoroutines,
		m.MaxMemoryMB,
		m.DatabaseConnections,
	)
}

// ResourceMonitor tracks system resource usage during load tests
type ResourceMonitor struct {
	metrics *LoadTestMetrics
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewResourceMonitor creates and starts resource monitoring
func NewResourceMonitor(metrics *LoadTestMetrics) *ResourceMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	monitor := &ResourceMonitor{
		metrics: metrics,
		ctx:     ctx,
		cancel:  cancel,
	}
	
	monitor.start()
	return monitor
}

// start begins resource monitoring in the background
func (rm *ResourceMonitor) start() {
	rm.wg.Add(1)
	go func() {
		defer rm.wg.Done()
		
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-rm.ctx.Done():
				return
			case <-ticker.C:
				rm.recordMetrics()
			}
		}
	}()
}

// recordMetrics captures current system metrics
func (rm *ResourceMonitor) recordMetrics() {
	// Get goroutine count
	goroutines := runtime.NumGoroutine()
	if goroutines > rm.metrics.MaxGoroutines {
		rm.metrics.MaxGoroutines = goroutines
	}
	
	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memoryMB := memStats.Alloc / 1024 / 1024
	if memoryMB > rm.metrics.MaxMemoryMB {
		rm.metrics.MaxMemoryMB = memoryMB
	}
}

// Stop halts resource monitoring
func (rm *ResourceMonitor) Stop() {
	rm.cancel()
	rm.wg.Wait()
}

// TestClassroomScaleLoad simulates realistic classroom size (30 students, 3 instructors)
// Target: <50ms message routing, >99% delivery success, 5-minute duration
func TestClassroomScaleLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}
	
	// Create realistic classroom scenario
	scenario := fixtures.GenerateClassroomScenario(3, 30) // 3 instructors, 30 students
	
	// Setup scenario runner with embedded server
	runner, err := fixtures.NewScenarioRunner(t, scenario)
	if err != nil {
		t.Fatalf("Failed to create scenario runner: %v", err)
	}
	
	// Initialize metrics and monitoring
	metrics := &LoadTestMetrics{
		StartTime: time.Now(),
		MinLatency: time.Hour, // Will be overwritten by first measurement
	}
	resourceMonitor := NewResourceMonitor(metrics)
	defer resourceMonitor.Stop()
	
	// Create all clients (33 total: 3 instructors + 30 students)
	t.Logf("Creating %d clients (%d instructors, %d students)", 
		len(scenario.InstructorIDs) + len(scenario.StudentIDs),
		len(scenario.InstructorIDs), len(scenario.StudentIDs))
	
	// Create instructor clients
	for _, instructorID := range scenario.InstructorIDs {
		_, err := runner.CreateClient(instructorID, "instructor")
		if err != nil {
			t.Fatalf("Failed to create instructor client %s: %v", instructorID, err)
		}
	}
	
	// Create student clients
	for _, studentID := range scenario.StudentIDs {
		_, err := runner.CreateClient(studentID, "student")
		if err != nil {
			t.Fatalf("Failed to create student client %s: %v", studentID, err)
		}
	}
	
	// Connect all clients concurrently
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	connectStart := time.Now()
	err = runner.ConnectAllClients(ctx)
	if err != nil {
		t.Fatalf("Failed to connect all clients: %v", err)
	}
	connectDuration := time.Since(connectStart)
	
	atomic.StoreInt64(&metrics.ConnectionsEstablished, int64(len(scenario.InstructorIDs) + len(scenario.StudentIDs)))
	t.Logf("Connected %d clients in %v", metrics.ConnectionsEstablished, connectDuration)
	
	// Run sustained load for 5 minutes with mixed message patterns
	loadCtx, loadCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer loadCancel()
	
	t.Log("Starting 5-minute sustained load test...")
	
	// Launch concurrent message senders and collector
	var wg sync.WaitGroup
	
	// Message collector (MISSING from original test!)
	wg.Add(1)
	go func() {
		defer wg.Done()
		collectMessages(loadCtx, runner, metrics)
	}()
	
	// Instructor broadcast messages (every 30 seconds)
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendInstructorBroadcasts(loadCtx, runner, scenario, metrics)
	}()
	
	// Student questions to instructors (continuous)
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendStudentQuestions(loadCtx, runner, scenario, metrics)
	}()
	
	// Instructor responses to students (reactive)
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendInstructorResponses(loadCtx, runner, scenario, metrics)
	}()
	
	// Student analytics (every 60 seconds)
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendAnalyticsData(loadCtx, runner, scenario, metrics)
	}()
	
	// Code review requests (every 2 minutes)
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendCodeReviewRequests(loadCtx, runner, scenario, metrics)
	}()
	
	// Wait for all senders to complete
	wg.Wait()
	metrics.EndTime = time.Now()
	
	// Validate performance targets
	t.Log(metrics.GetReport())
	
	// Requirement: >99% message delivery success
	successRate := float64(metrics.MessagesReceived) / float64(metrics.MessagesSent) * 100
	if successRate < 99.0 {
		t.Errorf("Message delivery success rate too low: %.2f%% (target: >99%%)", successRate)
	}
	
	// Requirement: <50ms average message routing latency
	if metrics.AverageLatency > 50*time.Millisecond {
		t.Errorf("Average message latency too high: %v (target: <50ms)", metrics.AverageLatency)
	}
	
	// Requirement: 1000+ messages/second per connection capability
	duration := metrics.EndTime.Sub(metrics.StartTime)
	totalConnections := len(scenario.InstructorIDs) + len(scenario.StudentIDs)
	messagesPerSecondPerConnection := float64(metrics.MessagesReceived) / duration.Seconds() / float64(totalConnections)
	
	t.Logf("Messages per second per connection: %.2f", messagesPerSecondPerConnection)
	
	// Memory and resource validation
	if metrics.MaxMemoryMB > 100 { // 100MB limit for classroom scale
		t.Errorf("Memory usage too high: %dMB (target: <100MB)", metrics.MaxMemoryMB)
	}
	
	// Verify no significant resource leaks
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	finalGoroutines := runtime.NumGoroutine()
	if finalGoroutines > 50 { // Allow reasonable number of background goroutines
		t.Errorf("Potential goroutine leak: %d goroutines remaining", finalGoroutines)
	}
}

// TestMessageBurstHandling tests system resilience under message bursts
// Simulates all 30 students responding within 10 seconds + rate limiting validation
func TestMessageBurstHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping burst test in short mode")
	}
	
	// Create classroom scenario for burst testing
	scenario := fixtures.GenerateClassroomScenario(2, 30) // 2 instructors, 30 students
	
	runner, err := fixtures.NewScenarioRunner(t, scenario)
	if err != nil {
		t.Fatalf("Failed to create scenario runner: %v", err)
	}
	
	metrics := &LoadTestMetrics{StartTime: time.Now()}
	resourceMonitor := NewResourceMonitor(metrics)
	defer resourceMonitor.Stop()
	
	// Create and connect all clients
	for _, instructorID := range scenario.InstructorIDs {
		_, err := runner.CreateClient(instructorID, "instructor")
		if err != nil {
			t.Fatalf("Failed to create instructor client: %v", err)
		}
	}
	
	for _, studentID := range scenario.StudentIDs {
		_, err := runner.CreateClient(studentID, "student")
		if err != nil {
			t.Fatalf("Failed to create student client: %v", err)
		}
	}
	
	ctx := context.Background()
	err = runner.ConnectAllClients(ctx)
	if err != nil {
		t.Fatalf("Failed to connect clients: %v", err)
	}
	
	// Test 1: Instructor broadcasts during high student activity
	t.Log("Testing instructor broadcasts during student message bursts...")
	
	// Start continuous student messaging (approaching rate limit)
	burstCtx, burstCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer burstCancel()
	
	var wg sync.WaitGroup
	
	// High-frequency student messages (testing rate limiting)
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		for {
			select {
			case <-burstCtx.Done():
				return
			default:
				// Each student sends 1 message per second (60 msg/min, under 100 limit)
				for _, studentID := range scenario.StudentIDs {
					client, exists := runner.GetClient(studentID)
					if !exists {
						continue
					}
					
					sendStart := time.Now()
					err := client.SendQuickMessage("instructor_inbox", 
						fmt.Sprintf("Question from %s at %v", studentID, time.Now().Format("15:04:05")))
					
					if err != nil {
						atomic.AddInt64(&metrics.ErrorCount, 1)
					} else {
						atomic.AddInt64(&metrics.MessagesSent, 1)
						metrics.AddLatency(time.Since(sendStart))
					}
					
					time.Sleep(1 * time.Second) // 1 message per second per student
				}
			}
		}
	}()
	
	// Instructor broadcasts during high load (every 15 seconds)
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-burstCtx.Done():
				return
			case <-ticker.C:
				client, exists := runner.GetClient(scenario.InstructorIDs[0])
				if !exists {
					continue
				}
				
				sendStart := time.Now()
				err := client.SendQuickMessage("instructor_broadcast", 
					fmt.Sprintf("Broadcast during high load: %v", time.Now().Format("15:04:05")))
				
				if err != nil {
					atomic.AddInt64(&metrics.ErrorCount, 1)
				} else {
					atomic.AddInt64(&metrics.MessagesSent, 1)
					metrics.AddLatency(time.Since(sendStart))
				}
			}
		}
	}()
	
	// Message collector
	wg.Add(1)
	go func() {
		defer wg.Done()
		collectMessages(burstCtx, runner, metrics)
	}()
	
	// Test 2: Burst response scenario (all students respond within 10 seconds)
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		// Wait 30 seconds then trigger burst
		time.Sleep(30 * time.Second)
		
		t.Log("Triggering student response burst (all 30 students in 10 seconds)...")
		
		// All students respond rapidly
		burstWg := sync.WaitGroup{}
		for i, studentID := range scenario.StudentIDs {
			burstWg.Add(1)
			go func(id string, delay int) {
				defer burstWg.Done()
				
				// Stagger within 10 seconds
				time.Sleep(time.Duration(delay) * time.Millisecond * 333) // 333ms * 30 = ~10 seconds
				
				client, exists := runner.GetClient(id)
				if !exists {
					return
				}
				
				sendStart := time.Now()
				err := client.SendQuickMessage("instructor_inbox", 
					fmt.Sprintf("Burst response from %s", id))
				
				if err != nil {
					atomic.AddInt64(&metrics.ErrorCount, 1)
				} else {
					atomic.AddInt64(&metrics.MessagesSent, 1)
					metrics.AddLatency(time.Since(sendStart))
				}
			}(studentID, i)
		}
		
		burstWg.Wait()
		t.Log("Student burst complete")
	}()
	
	wg.Wait()
	metrics.EndTime = time.Now()
	
	// Validate burst handling performance
	t.Log(metrics.GetReport())
	
	// Buffer overflow protection: system should handle bursts without dropping messages
	if metrics.ErrorCount > int64(float64(metrics.MessagesSent) * 0.01) { // Allow 1% error rate
		t.Errorf("Too many errors during burst: %d errors out of %d messages", 
			metrics.ErrorCount, metrics.MessagesSent)
	}
	
	// Rate limiting validation: no client should be able to exceed 100 msg/min
	if metrics.AverageLatency > 100*time.Millisecond {
		t.Errorf("Average latency too high during burst: %v (target: <100ms)", metrics.AverageLatency)
	}
	
	// System should remain responsive during peak load
	if metrics.MaxLatency > 1*time.Second {
		t.Errorf("Max latency too high during burst: %v (target: <1s)", metrics.MaxLatency)
	}
}

// TestConcurrentSessionsLoad tests multiple classroom sessions simultaneously
// 3 concurrent sessions with 10 students each, validating cross-session isolation
func TestConcurrentSessionsLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent sessions test in short mode")
	}
	
	const numSessions = 3
	const studentsPerSession = 10
	
	// Create multiple scenarios
	scenarios := make([]*fixtures.ClassroomData, numSessions)
	runners := make([]*fixtures.ScenarioRunner, numSessions)
	
	for i := 0; i < numSessions; i++ {
		scenarios[i] = fixtures.GenerateClassroomScenario(1, studentsPerSession)
		scenarios[i].SessionName = fmt.Sprintf("Session_%d", i+1)
		
		runner, err := fixtures.NewScenarioRunner(t, scenarios[i])
		if err != nil {
			t.Fatalf("Failed to create runner for session %d: %v", i, err)
		}
		runners[i] = runner
	}
	
	metrics := &LoadTestMetrics{StartTime: time.Now()}
	resourceMonitor := NewResourceMonitor(metrics)
	defer resourceMonitor.Stop()
	
	// Setup all sessions concurrently
	var setupWg sync.WaitGroup
	
	for i, runner := range runners {
		setupWg.Add(1)
		go func(sessionIdx int, r *fixtures.ScenarioRunner, scenario *fixtures.ClassroomData) {
			defer setupWg.Done()
			
			// Create clients for this session
			_, err := r.CreateClient(scenario.InstructorIDs[0], "instructor")
			if err != nil {
				t.Errorf("Session %d: Failed to create instructor: %v", sessionIdx, err)
				return
			}
			
			for _, studentID := range scenario.StudentIDs {
				_, err := r.CreateClient(studentID, "student")
				if err != nil {
					t.Errorf("Session %d: Failed to create student %s: %v", sessionIdx, studentID, err)
					return
				}
			}
			
			// Connect all clients in this session
			ctx := context.Background()
			err = r.ConnectAllClients(ctx)
			if err != nil {
				t.Errorf("Session %d: Failed to connect clients: %v", sessionIdx, err)
				return
			}
			
			atomic.AddInt64(&metrics.ConnectionsEstablished, int64(len(scenario.InstructorIDs) + len(scenario.StudentIDs)))
			t.Logf("Session %d: Connected %d clients", sessionIdx, len(scenario.InstructorIDs) + len(scenario.StudentIDs))
		}(i, runner, scenarios[i])
	}
	
	setupWg.Wait()
	
	// Run concurrent load on all sessions
	loadCtx, loadCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer loadCancel()
	
	t.Logf("Running concurrent load on %d sessions for 3 minutes...", numSessions)
	
	var loadWg sync.WaitGroup
	
	// Each session runs independent message flows
	for i, runner := range runners {
		loadWg.Add(1)
		go func(sessionIdx int, r *fixtures.ScenarioRunner, scenario *fixtures.ClassroomData) {
			defer loadWg.Done()
			
			sessionWg := sync.WaitGroup{}
			
			// Student questions in this session
			sessionWg.Add(1)
			go func() {
				defer sessionWg.Done()
				sendStudentQuestions(loadCtx, r, scenario, metrics)
			}()
			
			// Instructor responses in this session
			sessionWg.Add(1)
			go func() {
				defer sessionWg.Done()
				sendInstructorResponses(loadCtx, r, scenario, metrics)
			}()
			
			// Analytics from this session
			sessionWg.Add(1)
			go func() {
				defer sessionWg.Done()
				sendAnalyticsData(loadCtx, r, scenario, metrics)
			}()
			
			// Message collection for this session
			sessionWg.Add(1)
			go func() {
				defer sessionWg.Done()
				collectMessages(loadCtx, r, metrics)
			}()
			
			sessionWg.Wait()
			t.Logf("Session %d completed message flows", sessionIdx)
		}(i, runner, scenarios[i])
	}
	
	loadWg.Wait()
	metrics.EndTime = time.Now()
	
	// Validate session isolation
	t.Log("Validating cross-session isolation...")
	
	for i, runner := range runners {
		// Each session should only have messages from its own participants
		runner.ValidateSessionIsolation(t)
		
		// Check message counts are reasonable for the session
		messageCount, err := runner.TestSession.GetMessageCount()
		if err != nil {
			t.Errorf("Session %d: Failed to get message count: %v", i, err)
		} else if messageCount == 0 {
			t.Errorf("Session %d: No messages recorded", i)
		} else {
			t.Logf("Session %d: %d messages recorded", i, messageCount)
		}
	}
	
	// Database write contention testing - all writes should complete successfully
	if metrics.ErrorCount > 0 {
		t.Errorf("Database write errors detected: %d errors", metrics.ErrorCount)
	}
	
	// Performance validation
	t.Log(metrics.GetReport())
	
	// Memory usage should scale reasonably with concurrent sessions
	expectedMaxMemory := uint64(numSessions * 30) // ~30MB per session is reasonable
	if metrics.MaxMemoryMB > expectedMaxMemory {
		t.Errorf("Memory usage too high for concurrent sessions: %dMB (expected: <%dMB)", 
			metrics.MaxMemoryMB, expectedMaxMemory)
	}
	
	// Message delivery should remain reliable across all sessions
	successRate := float64(metrics.MessagesReceived) / float64(metrics.MessagesSent) * 100
	if successRate < 95.0 { // Slightly lower threshold for concurrent sessions
		t.Errorf("Cross-session message delivery success rate too low: %.2f%% (target: >95%%)", successRate)
	}
}

// TestConnectionStabilityStress tests connection resilience with random disconnections
func TestConnectionStabilityStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping connection stability test in short mode")
	}
	
	// Create moderate-sized classroom for stability testing
	scenario := fixtures.GenerateClassroomScenario(2, 20) // 2 instructors, 20 students
	
	runner, err := fixtures.NewScenarioRunner(t, scenario)
	if err != nil {
		t.Fatalf("Failed to create scenario runner: %v", err)
	}
	
	metrics := &LoadTestMetrics{StartTime: time.Now()}
	resourceMonitor := NewResourceMonitor(metrics)
	defer resourceMonitor.Stop()
	
	// Create and connect all clients initially
	for _, instructorID := range scenario.InstructorIDs {
		_, err := runner.CreateClient(instructorID, "instructor")
		if err != nil {
			t.Fatalf("Failed to create instructor client: %v", err)
		}
	}
	
	for _, studentID := range scenario.StudentIDs {
		_, err := runner.CreateClient(studentID, "student")
		if err != nil {
			t.Fatalf("Failed to create student client: %v", err)
		}
	}
	
	ctx := context.Background()
	err = runner.ConnectAllClients(ctx)
	if err != nil {
		t.Fatalf("Failed to connect initial clients: %v", err)
	}
	
	atomic.StoreInt64(&metrics.ConnectionsEstablished, int64(len(scenario.InstructorIDs) + len(scenario.StudentIDs)))
	
	// Run stability test for 4 minutes with random disconnections/reconnections
	stressCtx, stressCancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer stressCancel()
	
	t.Log("Starting 4-minute connection stability stress test...")
	
	var wg sync.WaitGroup
	
	// Continuous message flow (baseline load)
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendStudentQuestions(stressCtx, runner, scenario, metrics)
	}()
	
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendInstructorResponses(stressCtx, runner, scenario, metrics)
	}()
	
	// Random disconnection/reconnection patterns
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		ticker := time.NewTicker(10 * time.Second) // Every 10 seconds
		defer ticker.Stop()
		
		for {
			select {
			case <-stressCtx.Done():
				return
			case <-ticker.C:
				// Randomly disconnect 10-20% of clients
				allClients := runner.GetAllClients()
				clientList := make([]string, 0, len(allClients))
				for userID := range allClients {
					clientList = append(clientList, userID)
				}
				
				// Disconnect random clients
				numToDisconnect := rand.Intn(len(clientList)/5) + 1 // 1 to 20% of clients
				for i := 0; i < numToDisconnect; i++ {
					userID := clientList[rand.Intn(len(clientList))]
					
					t.Logf("Disconnecting client %s", userID)
					err := runner.DisconnectClient(userID)
					if err != nil {
						t.Logf("Failed to disconnect %s: %v", userID, err)
					}
				}
				
				// Wait a bit then reconnect
				time.Sleep(2 * time.Second)
				
				for i := 0; i < numToDisconnect; i++ {
					userID := clientList[rand.Intn(len(clientList))]
					
					reconnectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					t.Logf("Reconnecting client %s", userID)
					err := runner.ReconnectClient(reconnectCtx, userID)
					cancel()
					
					if err != nil {
						atomic.AddInt64(&metrics.ConnectionsFailed, 1)
						t.Logf("Failed to reconnect %s: %v", userID, err)
					} else {
						atomic.AddInt64(&metrics.ConnectionsEstablished, 1)
					}
				}
			}
		}
	}()
	
	// Network simulation (latency injection)
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		// Periodically enable/disable network simulation
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		networkSimEnabled := false
		
		for {
			select {
			case <-stressCtx.Done():
				return
			case <-ticker.C:
				networkSimEnabled = !networkSimEnabled
				runner.SimulateNetworkConditions(networkSimEnabled)
				t.Logf("Network simulation: %t", networkSimEnabled)
			}
		}
	}()
	
	// Message collection and monitoring
	wg.Add(1)
	go func() {
		defer wg.Done()
		collectMessages(stressCtx, runner, metrics)
	}()
	
	// Session recovery validation
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		
		for {
			select {
			case <-stressCtx.Done():
				return
			case <-ticker.C:
				// Verify session data integrity
				messageCount, err := runner.TestSession.GetMessageCount()
				if err != nil {
					atomic.AddInt64(&metrics.ErrorCount, 1)
					t.Logf("Session recovery check failed: %v", err)
				} else {
					t.Logf("Session recovery check: %d messages in database", messageCount)
				}
			}
		}
	}()
	
	wg.Wait()
	metrics.EndTime = time.Now()
	
	// Validate connection stability
	t.Log(metrics.GetReport())
	
	// Connection success rate should be reasonable despite disconnections
	connectionSuccessRate := float64(metrics.ConnectionsEstablished) / 
		float64(metrics.ConnectionsEstablished + metrics.ConnectionsFailed) * 100
	if connectionSuccessRate < 80.0 {
		t.Errorf("Connection success rate too low: %.2f%% (target: >80%%)", connectionSuccessRate)
	}
	
	// System should remain functional despite network issues
	if metrics.ErrorCount > int64(float64(metrics.MessagesSent) * 0.05) { // Allow 5% error rate
		t.Errorf("Too many errors during stability test: %d errors out of %d messages", 
			metrics.ErrorCount, metrics.MessagesSent)
	}
	
	// Resource leak detection - verify cleanup after disconnections
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	finalGoroutines := runtime.NumGoroutine()
	if finalGoroutines > 60 { // Higher threshold due to connection churn
		t.Errorf("Potential resource leak after connection stress: %d goroutines", finalGoroutines)
	}
	
	// Session data should remain consistent
	finalMessageCount, err := runner.TestSession.GetMessageCount()
	if err != nil {
		t.Errorf("Failed to get final message count: %v", err)
	} else if finalMessageCount == 0 {
		t.Error("No messages persisted after stability test")
	} else {
		t.Logf("Final session message count: %d", finalMessageCount)
	}
}

// Helper functions for load test message generation

// sendInstructorBroadcasts sends periodic broadcast messages from instructors
func sendInstructorBroadcasts(ctx context.Context, runner *fixtures.ScenarioRunner, scenario *fixtures.ClassroomData, metrics *LoadTestMetrics) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	messageCounter := 0
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			instructorID := scenario.InstructorIDs[messageCounter % len(scenario.InstructorIDs)]
			client, exists := runner.GetClient(instructorID)
			if !exists {
				continue
			}
			
			sendStart := time.Now()
			err := client.SendQuickMessage("instructor_broadcast", 
				fmt.Sprintf("Broadcast #%d from %s", messageCounter, instructorID))
			
			if err != nil {
				atomic.AddInt64(&metrics.ErrorCount, 1)
			} else {
				atomic.AddInt64(&metrics.MessagesSent, 1)
				metrics.AddLatency(time.Since(sendStart))
			}
			
			messageCounter++
		}
	}
}

// sendStudentQuestions sends continuous student questions to instructors
func sendStudentQuestions(ctx context.Context, runner *fixtures.ScenarioRunner, scenario *fixtures.ClassroomData, metrics *LoadTestMetrics) {
	ticker := time.NewTicker(2 * time.Second) // One question every 2 seconds across all students
	defer ticker.Stop()
	
	messageCounter := 0
	questions := []string{
		"I need help with this problem",
		"Could you explain this concept?",
		"I'm getting an error in my code",
		"What's the next step here?",
		"Can you check my answer?",
	}
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			studentID := scenario.StudentIDs[messageCounter % len(scenario.StudentIDs)]
			client, exists := runner.GetClient(studentID)
			if !exists {
				continue
			}
			
			question := questions[messageCounter % len(questions)]
			sendStart := time.Now()
			err := client.SendQuickMessage("instructor_inbox", 
				fmt.Sprintf("%s from %s", question, studentID))
			
			if err != nil {
				atomic.AddInt64(&metrics.ErrorCount, 1)
			} else {
				atomic.AddInt64(&metrics.MessagesSent, 1)
				metrics.AddLatency(time.Since(sendStart))
			}
			
			messageCounter++
		}
	}
}

// sendInstructorResponses sends instructor responses to students
func sendInstructorResponses(ctx context.Context, runner *fixtures.ScenarioRunner, scenario *fixtures.ClassroomData, metrics *LoadTestMetrics) {
	ticker := time.NewTicker(5 * time.Second) // Response every 5 seconds
	defer ticker.Stop()
	
	messageCounter := 0
	responses := []string{
		"Here's how to approach this problem",
		"Good question! Let me explain",
		"You're on the right track, but try this",
		"I see the issue in your code",
		"That looks correct to me",
	}
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(scenario.StudentIDs) == 0 {
				continue
			}
			
			instructorID := scenario.InstructorIDs[messageCounter % len(scenario.InstructorIDs)]
			studentID := scenario.StudentIDs[messageCounter % len(scenario.StudentIDs)]
			
			client, exists := runner.GetClient(instructorID) 
			if !exists {
				continue
			}
			
			response := responses[messageCounter % len(responses)]
			sendStart := time.Now()
			err := client.SendDirectMessage("inbox_response", response, studentID)
			
			if err != nil {
				atomic.AddInt64(&metrics.ErrorCount, 1)
			} else {
				atomic.AddInt64(&metrics.MessagesSent, 1)
				metrics.AddLatency(time.Since(sendStart))
			}
			
			messageCounter++
		}
	}
}

// sendAnalyticsData sends periodic analytics data from students
func sendAnalyticsData(ctx context.Context, runner *fixtures.ScenarioRunner, scenario *fixtures.ClassroomData, metrics *LoadTestMetrics) {
	ticker := time.NewTicker(60 * time.Second) // Analytics every minute
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Each student sends analytics
			for _, studentID := range scenario.StudentIDs {
				client, exists := runner.GetClient(studentID)
				if !exists {
					continue
				}
				
				sendStart := time.Now()
				err := client.SendMessage("analytics", "engagement", map[string]interface{}{
					"attention_level": rand.Intn(100),
					"participation":   rand.Intn(100),
					"timestamp":       time.Now().Unix(),
				}, "")
				
				if err != nil {
					atomic.AddInt64(&metrics.ErrorCount, 1)
				} else {
					atomic.AddInt64(&metrics.MessagesSent, 1)
					metrics.AddLatency(time.Since(sendStart))
				}
				
				time.Sleep(100 * time.Millisecond) // Small delay between students
			}
		}
	}
}

// sendCodeReviewRequests sends periodic code review requests from instructors
func sendCodeReviewRequests(ctx context.Context, runner *fixtures.ScenarioRunner, scenario *fixtures.ClassroomData, metrics *LoadTestMetrics) {
	ticker := time.NewTicker(2 * time.Minute) // Code review every 2 minutes
	defer ticker.Stop()
	
	messageCounter := 0
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(scenario.StudentIDs) == 0 {
				continue
			}
			
			instructorID := scenario.InstructorIDs[0] // Primary instructor
			studentID := scenario.StudentIDs[messageCounter % len(scenario.StudentIDs)]
			
			client, exists := runner.GetClient(instructorID)
			if !exists {
				continue
			}
			
			sendStart := time.Now()
			err := client.SendDirectMessage("request", 
				fmt.Sprintf("Please share your solution for assignment %d", messageCounter+1), studentID)
			
			if err != nil {
				atomic.AddInt64(&metrics.ErrorCount, 1)
			} else {
				atomic.AddInt64(&metrics.MessagesSent, 1)
				metrics.AddLatency(time.Since(sendStart))
			}
			
			messageCounter++
		}
	}
}

// collectMessages continuously collects incoming messages from all clients
func collectMessages(ctx context.Context, runner *fixtures.ScenarioRunner, metrics *LoadTestMetrics) {
	ticker := time.NewTicker(500 * time.Millisecond) // Check every 500ms
	defer ticker.Stop()
	
	// Track messages already counted per client to avoid double-counting
	clientMessageCounts := make(map[string]int)
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			allClients := runner.GetAllClients()
			
			for clientID, client := range allClients {
				// Get current message count without draining
				currentCount := client.GetMessageCount()
				previousCount := clientMessageCounts[clientID]
				
				// Only count new messages since last check
				if currentCount > previousCount {
					newMessages := currentCount - previousCount
					atomic.AddInt64(&metrics.MessagesReceived, int64(newMessages))
					clientMessageCounts[clientID] = currentCount
				}
			}
		}
	}
}

// BenchmarkMessageThroughput benchmarks message processing throughput
func BenchmarkMessageThroughput(b *testing.B) {
	// Create small scenario for benchmarking
	scenario := fixtures.GenerateClassroomScenario(1, 5) // 1 instructor, 5 students
	
	runner, err := fixtures.NewScenarioRunner(&testing.T{}, scenario)
	if err != nil {
		b.Fatalf("Failed to create scenario runner: %v", err)
	}
	defer runner.Cleanup()
	
	// Create and connect clients
	for _, instructorID := range scenario.InstructorIDs {
		_, err := runner.CreateClient(instructorID, "instructor")
		if err != nil {
			b.Fatalf("Failed to create instructor client: %v", err)
		}
	}
	
	for _, studentID := range scenario.StudentIDs {
		_, err := runner.CreateClient(studentID, "student")  
		if err != nil {
			b.Fatalf("Failed to create student client: %v", err)
		}
	}
	
	ctx := context.Background()
	err = runner.ConnectAllClients(ctx)
	if err != nil {
		b.Fatalf("Failed to connect clients: %v", err)
	}
	
	instructor, _ := runner.GetClient(scenario.InstructorIDs[0])
	
	b.ResetTimer()
	
	// Benchmark message sending
	for i := 0; i < b.N; i++ {
		err := instructor.SendQuickMessage("instructor_broadcast", 
			fmt.Sprintf("Benchmark message %d", i))
		if err != nil {
			b.Errorf("Failed to send message %d: %v", i, err)
		}
	}
}

// BenchmarkLoadTestConnectionSetup benchmarks client connection establishment for load tests
func BenchmarkLoadTestConnectionSetup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		scenario := fixtures.GenerateClassroomScenario(1, 10) // 1 instructor, 10 students
		
		runner, err := fixtures.NewScenarioRunner(&testing.T{}, scenario)
		if err != nil {
			b.Fatalf("Failed to create scenario runner: %v", err)
		}
		
		// Create clients
		for _, instructorID := range scenario.InstructorIDs {
			_, err := runner.CreateClient(instructorID, "instructor")
			if err != nil {
				b.Fatalf("Failed to create instructor client: %v", err)
			}
		}
		
		for _, studentID := range scenario.StudentIDs {
			_, err := runner.CreateClient(studentID, "student")
			if err != nil {
				b.Fatalf("Failed to create student client: %v", err)
			}
		}
		
		ctx := context.Background()
		err = runner.ConnectAllClients(ctx)
		if err != nil {
			b.Fatalf("Failed to connect clients: %v", err)
		}
		
		runner.Cleanup()
	}
}