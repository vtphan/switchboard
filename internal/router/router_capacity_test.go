package router

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"switchboard/pkg/types"
)

func TestRouter_InfrastructureCapacityBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping capacity benchmark in short mode")
	}
	
	// Infrastructure Capacity Benchmark
	// Tests maximum sustainable concurrent students and message throughput
	
	testDuration := 30 * time.Second // Fixed test duration for consistent measurement
	results := make([]CapacityResult, 0)
	
	// Test different student counts to find infrastructure limits
	studentCounts := []int{10, 25, 50, 75, 100, 150, 200}
	
	for _, numStudents := range studentCounts {
		t.Logf("Testing capacity with %d students...", numStudents)
		
		result := runCapacityTest(t, numStudents, testDuration)
		results = append(results, result)
		
		// Stop testing if we hit critical failure thresholds
		if result.ErrorRate > 0.10 || result.AvgLatency > 1000*time.Millisecond {
			t.Logf("Hit critical failure threshold at %d students, stopping capacity test", numStudents)
			break
		}
	}
	
	// Report infrastructure capacity findings
	reportCapacityFindings(t, results)
}

type CapacityResult struct {
	StudentCount      int
	MessageCount      int64
	Throughput        float64
	AvgLatency        time.Duration
	MaxLatency        time.Duration
	ErrorRate         float64
	MemoryUsage       uint64
	ConnectionsFailed int64
	Success           bool
}

func runCapacityTest(t *testing.T, numStudents int, duration time.Duration) CapacityResult {
	setup := NewLoadTestSetup()
	defer setup.Cleanup()
	
	// Create classroom with specified student count + 3 instructors
	err := setup.CreateClassroom("capacity_test", numStudents, 3)
	if err != nil {
		return CapacityResult{
			StudentCount: numStudents,
			Success:      false,
		}
	}
	
	var wg sync.WaitGroup
	startTime := time.Now()
	
	t.Logf("Starting capacity test with %d students for %v", numStudents, duration)
	
	// High-frequency message generation from all students
	// Each student sends messages at 2-5 second intervals for sustained load
	for i := 0; i < numStudents; i++ {
		wg.Add(1)
		go func(studentNum int) {
			defer wg.Done()
			
			studentID := fmt.Sprintf("student%d_capacity_test", studentNum+1)
			t.Logf("Student %d starting message loop", studentNum+1)
			
			// Each student sends messages continuously for the test duration
			// Stagger initial start times to avoid synchronized sleeping
			initialDelay := time.Duration(studentNum*100) * time.Millisecond // 0ms, 100ms, 200ms, etc.
			time.Sleep(initialDelay)
			
			endTime := startTime.Add(duration)
			messageCount := 0
			
			for time.Now().Before(endTime) {
				messageCount++
				
				message := &types.Message{
					SessionID: "capacity_test",
					Type:      types.MessageTypeAnalytics,
					FromUser:  studentID,
					Content: map[string]interface{}{
						"event":     "code_change",
						"timestamp": time.Now(),
						"sequence":  messageCount,
					},
				}
				
				if messageCount == 1 {
					t.Logf("Student %d sending first message", studentNum+1)
				}
				
				latencyStart := time.Now()
				ctx := context.Background()
				if err := setup.router.RouteMessage(ctx, message); err != nil {
					atomic.AddInt64(&setup.metrics.RoutingErrors, 1)
				} else {
					latency := time.Since(latencyStart)
					atomic.AddInt64(&setup.metrics.MessagesRouted, 1)
					
					// Track latency metrics
					if latency > time.Duration(atomic.LoadInt64((*int64)(&setup.metrics.MaxLatency))) {
						atomic.StoreInt64((*int64)(&setup.metrics.MaxLatency), int64(latency))
					}
				}
				
				// Fixed 3-second interval between messages
				time.Sleep(3 * time.Second)
			}
		}(i)
	}
	
	// Wait for all students to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All goroutines completed successfully
	case <-time.After(duration + 5*time.Second):
		// Timeout - some goroutines may still be running
		t.Logf("Capacity test timed out for %d students", numStudents)
	}
	
	// Calculate final metrics
	metrics := setup.GetMetrics()
	testDuration := time.Since(startTime)
	
	errorRate := float64(0)
	if metrics.MessagesRouted+metrics.RoutingErrors > 0 {
		errorRate = float64(metrics.RoutingErrors) / float64(metrics.MessagesRouted+metrics.RoutingErrors)
	}
	
	return CapacityResult{
		StudentCount:      numStudents,
		MessageCount:      metrics.MessagesRouted,
		Throughput:        float64(metrics.MessagesRouted) / testDuration.Seconds(),
		AvgLatency:        time.Duration(metrics.GetAverageLatency()),
		MaxLatency:        time.Duration(metrics.MaxLatency),
		ErrorRate:         errorRate,
		MemoryUsage:       metrics.PeakMemoryUsage,
		ConnectionsFailed: metrics.ConnectionErrors,
		Success:           metrics.RoutingErrors == 0,
	}
}

func reportCapacityFindings(t *testing.T, results []CapacityResult) {
	t.Logf("\n%s", strings.Repeat("=", 80))
	t.Logf("INFRASTRUCTURE CAPACITY BENCHMARK RESULTS")
	t.Logf("%s", strings.Repeat("=", 80))
	
	var maxSuccessfulStudents int
	var bestThroughput float64
	var degradationPoint int
	
	t.Logf("%-12s %-10s %-12s %-12s %-12s %-10s %-8s", 
		"Students", "Messages", "Throughput", "Avg Latency", "Max Latency", "Error Rate", "Memory")
	t.Logf("%s", strings.Repeat("-", 80))
	
	for _, result := range results {
		status := "✓"
		if !result.Success || result.ErrorRate > 0.05 {
			status = "✗"
			if degradationPoint == 0 {
				degradationPoint = result.StudentCount
			}
		} else {
			maxSuccessfulStudents = result.StudentCount
		}
		
		if result.Throughput > bestThroughput {
			bestThroughput = result.Throughput
		}
		
		t.Logf("%s %-10d %-10d %-12.1f %-12v %-12v %-10.2f%% %-8d KB",
			status,
			result.StudentCount,
			result.MessageCount,
			result.Throughput,
			result.AvgLatency,
			result.MaxLatency,
			result.ErrorRate*100,
			result.MemoryUsage/1024)
	}
	
	t.Logf("%s", strings.Repeat("=", 80))
	t.Logf("CAPACITY FINDINGS:")
	t.Logf("• Maximum Sustainable Students: %d", maxSuccessfulStudents)
	t.Logf("• Peak Throughput Achieved: %.1f messages/second", bestThroughput)
	
	if degradationPoint > 0 {
		t.Logf("• Performance Degradation Starts: %d students", degradationPoint)
	} else {
		t.Logf("• No Degradation Observed: Infrastructure can likely handle >%d students", maxSuccessfulStudents)
	}
	
	// Memory scaling analysis
	if len(results) >= 2 {
		memoryPerStudent := float64(results[len(results)-1].MemoryUsage-results[0].MemoryUsage) / float64(results[len(results)-1].StudentCount-results[0].StudentCount)
		t.Logf("• Memory Scaling: ~%.1f KB per student", memoryPerStudent/1024)
	}
	
	t.Logf("• Test Infrastructure: WebSocket connections, message routing, presence broadcasts")
	t.Logf("%s", strings.Repeat("=", 80))
}