// High Load and Performance Tests
// Tests system behavior under realistic classroom loads and performance benchmarks
package performance

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
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

// TestHighLoadScenarios validates system performance under realistic classroom loads (optimized for speed)
func TestHighLoadScenarios(t *testing.T) {
	t.Parallel() // Run subtests in parallel
	t.Run("typical_classroom_load", func(t *testing.T) {
		t.Parallel() // Allow parallel execution
		// Simulate typical classroom: 10 students, 1 instructor, 3-second burst
		env := setupPerformanceEnvironment(t)
		defer env.cleanup()

		const (
			numStudents          = 20
			numInstructors       = 2
			sessionDurationSecs  = 15   // Reduced from 45 minutes to 15 seconds
			questionsPerStudent  = 2    // Students ask 2 questions per test
			responsesPerQuestion = 1.0  // All questions get responses for faster test
			announcementsPerSession = 3 // Reduced announcements
		)

		// Setup session
		session := &database.Session{
			ID:        "typical-classroom-load",
			Name:      "Biology 101 - Typical Load Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		// Create connections
		students := createPerformanceConnections(t, env, "student", numStudents)
		instructors := createPerformanceConnections(t, env, "instructor", numInstructors)

		// Simulate fast message timing for quick test
		startTime := time.Now()
		var totalMessages int64
		var messageErrors int64
		var wg sync.WaitGroup

		// Student question generation with faster timing
		for i, student := range students {
			wg.Add(1)
			go func(studentIndex int, studentConn *PerformanceConnection) {
				defer wg.Done()
				
				questionsAsked := 0
				sessionStart := time.Now()
				
				for questionsAsked < questionsPerStudent {
					// Fast timing - questions every 1-3 seconds
					nextQuestionDelay := time.Duration(1000+rand.Intn(2000)) * time.Millisecond
					time.Sleep(nextQuestionDelay)
					
					// Check if still within session time
					if time.Since(sessionStart) > time.Duration(sessionDurationSecs)*time.Second {
						break
					}
					
					question := generateRealisticQuestion(studentIndex, questionsAsked)
					msg := createQuestionMessage(question)
					
					err := env.processor.ProcessIncomingMessage(msg, studentConn.userID)
					if err != nil {
						atomic.AddInt64(&messageErrors, 1)
					} else {
						atomic.AddInt64(&totalMessages, 1)
					}
					
					questionsAsked++
				}
			}(i, student)
		}

		// Instructor responses and announcements
		for i, instructor := range instructors {
			wg.Add(1)
			go func(instructorIndex int, instructorConn *PerformanceConnection) {
				defer wg.Done()
				
				announcementsMade := 0
				responsesMade := 0
				sessionStart := time.Now()
				
				for time.Since(sessionStart) < time.Duration(sessionDurationSecs)*time.Second {
					// Decide between announcement or response
					if rand.Float64() < 0.3 && announcementsMade < announcementsPerSession/numInstructors {
						// Make announcement
						announcement := generateRealisticAnnouncement(instructorIndex, announcementsMade)
						msg := createAnnouncementMessage(announcement)
						
						err := env.processor.ProcessIncomingMessage(msg, instructorConn.userID)
						if err != nil {
							atomic.AddInt64(&messageErrors, 1)
						} else {
							atomic.AddInt64(&totalMessages, 1)
						}
						
						announcementsMade++
						time.Sleep(time.Duration(2+rand.Intn(3)) * time.Second) // 2-5 sec between announcements
						
					} else if responsesMade < int(float64(numStudents*questionsPerStudent)*responsesPerQuestion)/numInstructors {
						// Send direct response to a student
						targetStudent := fmt.Sprintf("student%d", rand.Intn(numStudents)+1)
						response := generateRealisticResponse(instructorIndex, responsesMade)
						msg := createDirectResponseMessage(targetStudent, response)
						
						err := env.processor.ProcessIncomingMessage(msg, instructorConn.userID)
						if err != nil {
							atomic.AddInt64(&messageErrors, 1)
						} else {
							atomic.AddInt64(&totalMessages, 1)
						}
						
						responsesMade++
						time.Sleep(time.Duration(500+rand.Intn(1500)) * time.Millisecond) // 0.5-2s between responses
					} else {
						time.Sleep(200 * time.Millisecond) // Short wait before next check
					}
				}
			}(i, instructor)
		}

		// Wait for all message generation to complete
		wg.Wait()
		
		// Allow final message processing
		time.Sleep(500 * time.Millisecond)
		
		duration := time.Since(startTime)
		messagesProcessed := atomic.LoadInt64(&totalMessages)
		errors := atomic.LoadInt64(&messageErrors)
		
		// Performance metrics
		messagesPerSecond := float64(messagesProcessed) / duration.Seconds()
		errorRate := float64(errors) / float64(messagesProcessed+errors)
		
		t.Logf("Typical Classroom Load Results:")
		t.Logf("  Duration: %v", duration)
		t.Logf("  Messages processed: %d", messagesProcessed)
		t.Logf("  Message errors: %d", errors)
		t.Logf("  Messages/second: %.2f", messagesPerSecond)
		t.Logf("  Error rate: %.2f%%", errorRate*100)
		
		// Performance assertions (adjusted for faster test)
		assert.Greater(t, messagesPerSecond, 2.0, "Should process at least 2 messages/second")
		assert.Less(t, errorRate, 0.05, "Error rate should be < 5%")
		
		// Verify database consistency
		dbMessages, err := env.dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Equal(t, int(messagesProcessed), len(dbMessages), "All processed messages should be persisted")
		
		// Memory usage check
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		memoryMB := float64(memStats.Alloc) / 1024 / 1024
		t.Logf("Memory usage: %.2f MB", memoryMB)
		assert.Less(t, memoryMB, 30.0, "Memory usage should be reasonable for fast test")
	})

	t.Run("peak_usage_scenario", func(t *testing.T) {
		t.Parallel() // Allow parallel execution
		// Simulate peak usage: concentrated burst activity
		env := setupPerformanceEnvironment(t)
		defer env.cleanup()

		const (
			numStudents     = 30  // Reduced from 100
			numInstructors  = 3   // Reduced from 5
			testDurationSec = 10  // Reduced from 300 to 10 seconds
			msgBurstSize    = 10  // Smaller bursts
		)

		session := &database.Session{
			ID:        "peak-usage-scenario",
			Name:      "Large Lecture - Peak Usage Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		students := createPerformanceConnections(t, env, "student", numStudents)
		instructors := createPerformanceConnections(t, env, "instructor", numInstructors)

		var wg sync.WaitGroup
		var totalMessages int64
		var messageErrors int64
		var peakMemoryMB float64

		startTime := time.Now()

		// Simulate burst periods of high activity
		for burst := 0; burst < testDurationSec/2; burst++ { // Every 2 seconds
			t.Logf("Starting burst %d", burst+1)
			
			// Student question burst
			for i := 0; i < msgBurstSize && i < len(students); i++ {
				wg.Add(1)
				go func(studentIndex int) {
					defer wg.Done()
					
					student := students[studentIndex]
					msg := createQuestionMessage(fmt.Sprintf("Burst question from student %d in burst %d", studentIndex, burst))
					
					err := env.processor.ProcessIncomingMessage(msg, student.userID)
					if err != nil {
						atomic.AddInt64(&messageErrors, 1)
					} else {
						atomic.AddInt64(&totalMessages, 1)
					}
				}(i)
			}
			
			// Instructor response burst
			for i := 0; i < len(instructors); i++ {
				wg.Add(1)
				go func(instructorIndex int) {
					defer wg.Done()
					
					instructor := instructors[instructorIndex]
					
					// Send announcement
					msg := createAnnouncementMessage(fmt.Sprintf("Instructor %d announcement in burst %d", instructorIndex, burst))
					err := env.processor.ProcessIncomingMessage(msg, instructor.userID)
					if err != nil {
						atomic.AddInt64(&messageErrors, 1)
					} else {
						atomic.AddInt64(&totalMessages, 1)
					}
					
					// Send direct response
					targetStudent := fmt.Sprintf("student%d", rand.Intn(numStudents)+1)
					responseMsg := createDirectResponseMessage(targetStudent, fmt.Sprintf("Response from instructor %d", instructorIndex))
					err = env.processor.ProcessIncomingMessage(responseMsg, instructor.userID)
					if err != nil {
						atomic.AddInt64(&messageErrors, 1)
					} else {
						atomic.AddInt64(&totalMessages, 1)
					}
				}(i)
			}
			
			// Wait for burst to complete
			wg.Wait()
			
			// Check memory usage during peak
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			currentMemoryMB := float64(memStats.Alloc) / 1024 / 1024
			if currentMemoryMB > peakMemoryMB {
				peakMemoryMB = currentMemoryMB
			}
			
			// Small pause between bursts
			time.Sleep(500 * time.Millisecond)
		}

		duration := time.Since(startTime)
		messagesProcessed := atomic.LoadInt64(&totalMessages)
		errors := atomic.LoadInt64(&messageErrors)
		
		// Performance metrics
		messagesPerSecond := float64(messagesProcessed) / duration.Seconds()
		errorRate := float64(errors) / float64(messagesProcessed+errors)
		
		t.Logf("Peak Usage Scenario Results:")
		t.Logf("  Duration: %v", duration)
		t.Logf("  Messages processed: %d", messagesProcessed)
		t.Logf("  Message errors: %d", errors)
		t.Logf("  Messages/second: %.2f", messagesPerSecond)
		t.Logf("  Error rate: %.2f%%", errorRate*100)
		t.Logf("  Peak memory usage: %.2f MB", peakMemoryMB)
		
		// Peak performance assertions (adjusted for faster test)
		assert.Greater(t, messagesPerSecond, 5.0, "Should handle at least 5 messages/second during peak")
		assert.Less(t, errorRate, 0.10, "Error rate should be < 10% even during peak")
		assert.Less(t, peakMemoryMB, 50.0, "Peak memory usage should be reasonable for fast test")
		
		// Verify system stability after peak load
		testMsg := createAnnouncementMessage("Post-peak stability test")
		require.NoError(t, env.processor.ProcessIncomingMessage(testMsg, "instructor1"))
	})

	t.Run("connection_churn_performance", func(t *testing.T) {
		t.Parallel() // Allow parallel execution
		// Test performance with frequent connection/disconnection
		env := setupPerformanceEnvironment(t)
		defer env.cleanup()

		session := &database.Session{
			ID:        "connection-churn-test",
			Name:      "Connection Churn Performance Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		const (
			baseConnections    = 8   // Drastically reduced
			churnConnections   = 5   // Drastically reduced
			testDurationSec    = 3   // Just 3 seconds
			churnIntervalSec   = 1   // Every 1 second
		)

		// Create base stable connections
		stableConnections := createPerformanceConnections(t, env, "student", baseConnections)
		_ = createPerformanceConnections(t, env, "instructor", 2) // instructors not used in this test

		var totalConnections int64
		var connectionErrors int64
		var messagesProcessed int64
		var wg sync.WaitGroup

		startTime := time.Now()

		// Background message traffic from stable connections
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for time.Since(startTime) < time.Duration(testDurationSec)*time.Second {
				// Random stable connection sends message
				connIndex := rand.Intn(len(stableConnections))
				conn := stableConnections[connIndex]
				
				msg := createQuestionMessage(fmt.Sprintf("Stable connection %s message", conn.userID))
				err := env.processor.ProcessIncomingMessage(msg, conn.userID)
				if err == nil {
					atomic.AddInt64(&messagesProcessed, 1)
				}
				
				time.Sleep(time.Duration(100+rand.Intn(200)) * time.Millisecond) // Faster messages
			}
		}()

		// Connection churn simulation
		churnTicker := time.NewTicker(time.Duration(churnIntervalSec) * time.Second)
		defer churnTicker.Stop()

		churnConnectionsList := make([]*PerformanceConnection, 0, churnConnections)
		
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			churnCycle := 0
			for {
				select {
				case <-churnTicker.C:
					// Disconnect existing churn connections
					for _, conn := range churnConnectionsList {
						env.connectionRegistry.Unregister(conn.userID)
					}
					
					// Create new churn connections
					churnConnectionsList = churnConnectionsList[:0]
					for i := 0; i < churnConnections; i++ {
						conn := &PerformanceConnection{
							userID: fmt.Sprintf("churn_%d_%d", churnCycle, i),
							role:   "student",
						}
						
						err := env.connectionRegistry.Register(conn.userID, conn)
						if err != nil {
							atomic.AddInt64(&connectionErrors, 1)
						} else {
							atomic.AddInt64(&totalConnections, 1)
							churnConnectionsList = append(churnConnectionsList, conn)
							
							// New connection sends a message
							msg := createQuestionMessage(fmt.Sprintf("New connection %s message", conn.userID))
							if err := env.processor.ProcessIncomingMessage(msg, conn.userID); err != nil {
								// Log error but don't fail the test as this is a performance test
								t.Logf("Failed to process message from new connection %s: %v", conn.userID, err)
							}
						}
					}
					
					churnCycle++
					
				default:
					if time.Since(startTime) >= time.Duration(testDurationSec)*time.Second {
						return
					}
					time.Sleep(50 * time.Millisecond) // Faster polling
				}
			}
		}()

		wg.Wait()

		duration := time.Since(startTime)
		connections := atomic.LoadInt64(&totalConnections)
		connErrors := atomic.LoadInt64(&connectionErrors)
		messages := atomic.LoadInt64(&messagesProcessed)
		
		connectionSuccessRate := float64(connections) / float64(connections+connErrors)
		connectionsPerSecond := float64(connections) / duration.Seconds()
		
		t.Logf("Connection Churn Performance Results:")
		t.Logf("  Duration: %v", duration)
		t.Logf("  Connections created: %d", connections)
		t.Logf("  Connection errors: %d", connErrors)
		t.Logf("  Connection success rate: %.2f%%", connectionSuccessRate*100)
		t.Logf("  Connections/second: %.2f", connectionsPerSecond)
		t.Logf("  Messages processed: %d", messages)
		
		// Connection churn assertions (adjusted for faster test)
		assert.Greater(t, connectionSuccessRate, 0.90, "Connection success rate should be > 90%")
		assert.Greater(t, connectionsPerSecond, 0.5, "Should handle at least 0.5 connections/second")
		
		// Verify system state after churn
		allUsers := env.connectionRegistry.GetAllUsers()
		t.Logf("Final active connections: %d", len(allUsers))
		assert.GreaterOrEqual(t, len(allUsers), baseConnections, "Base connections should remain active")
	})

	t.Run("message_throughput_benchmark", func(t *testing.T) {
		t.Parallel() // Allow parallel execution
		// Pure throughput test to establish baseline performance
		env := setupPerformanceEnvironment(t)
		defer env.cleanup()

		session := &database.Session{
			ID:        "throughput-benchmark",
			Name:      "Message Throughput Benchmark",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		// Create minimal connections for pure throughput test
		const numConnections = 8
		connections := createPerformanceConnections(t, env, "student", numConnections)
		createPerformanceConnections(t, env, "instructor", 2)

		const testDurationSec = 10 // Reduced from 60 to 10 seconds
		var messagesProcessed int64
		var processingLatencies []time.Duration
		var latencyMutex sync.Mutex

		startTime := time.Now()
		var wg sync.WaitGroup

		// Each connection sends messages as fast as possible
		for i, conn := range connections {
			wg.Add(1)
			go func(connIndex int, connection *PerformanceConnection) {
				defer wg.Done()
				
				messageIndex := 0
				for time.Since(startTime) < time.Duration(testDurationSec)*time.Second {
					msgStart := time.Now()
					
					msg := createQuestionMessage(fmt.Sprintf("Throughput test from %s #%d", connection.userID, messageIndex))
					err := env.processor.ProcessIncomingMessage(msg, connection.userID)
					
					if err == nil {
						atomic.AddInt64(&messagesProcessed, 1)
						
						// Record latency
						latency := time.Since(msgStart)
						latencyMutex.Lock()
						processingLatencies = append(processingLatencies, latency)
						latencyMutex.Unlock()
					}
					
					messageIndex++
					
					// Small delay to prevent overwhelming rate limiter
					time.Sleep(50 * time.Millisecond) // Faster for shorter test
				}
			}(i, conn)
		}

		wg.Wait()

		duration := time.Since(startTime)
		messages := atomic.LoadInt64(&messagesProcessed)
		throughput := float64(messages) / duration.Seconds()

		// Calculate latency statistics
		var avgLatency, maxLatency time.Duration
		if len(processingLatencies) > 0 {
			var totalLatency time.Duration
			maxLatency = processingLatencies[0]
			
			for _, latency := range processingLatencies {
				totalLatency += latency
				if latency > maxLatency {
					maxLatency = latency
				}
			}
			avgLatency = totalLatency / time.Duration(len(processingLatencies))
		}

		t.Logf("Message Throughput Benchmark Results:")
		t.Logf("  Duration: %v", duration)
		t.Logf("  Messages processed: %d", messages)
		t.Logf("  Throughput: %.2f messages/second", throughput)
		t.Logf("  Average latency: %v", avgLatency)
		t.Logf("  Maximum latency: %v", maxLatency)
		
		// Throughput benchmarks (adjusted for faster test)
		assert.Greater(t, throughput, 10.0, "Should achieve at least 10 messages/second throughput")
		assert.Less(t, avgLatency, 100*time.Millisecond, "Average latency should be < 100ms")
		assert.Less(t, maxLatency, 500*time.Millisecond, "Maximum latency should be < 500ms")
		
		// Verify data integrity
		time.Sleep(2 * time.Second) // Allow batch processing to complete
		dbMessages, err := env.dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		assert.Equal(t, int(messages), len(dbMessages), "All processed messages should be persisted")
	})
}

// Helper functions for performance testing

type PerformanceEnvironment struct {
	dbManager          *database.SQLiteDatabaseManager
	sessionManager     session.SessionManager
	connectionRegistry *websocket.ConnectionRegistry
	processor          *message.MessageProcessor
}

func setupPerformanceEnvironment(t *testing.T) *PerformanceEnvironment {
	// Setup in-memory database for performance testing
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Apply schema
	schema, err := os.ReadFile("../../internal/database/migrations.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(schema))
	require.NoError(t, err)

	// Create components
	// Skip table initialization and pragmas since we already applied migrations.sql
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
	rateLimiter := rate.NewRateLimiter()

	filterAdapter := websocket.NewFilterAdapter(roleBasedFilter)
	broadcastSystem := websocket.NewBroadcastSystem(connectionRegistry, filterAdapter)

	processor := message.NewMessageProcessor(
		sessionManager,
		dbManager,
		rateLimiter,
		messageRouter,
		broadcastSystem,
	)

	return &PerformanceEnvironment{
		dbManager:          dbManager,
		sessionManager:     sessionManager,
		connectionRegistry: connectionRegistry,
		processor:          processor,
	}
}

func (env *PerformanceEnvironment) cleanup() {
	env.connectionRegistry.Stop()
	if err := env.dbManager.Stop(); err != nil {
		// Use fmt.Printf since we don't have testing.T context here
		fmt.Printf("Failed to stop database manager: %v\n", err)
	}
}

func createPerformanceConnections(t *testing.T, env *PerformanceEnvironment, rolePrefix string, count int) []*PerformanceConnection {
	connections := make([]*PerformanceConnection, count)
	
	for i := 0; i < count; i++ {
		conn := &PerformanceConnection{
			userID: fmt.Sprintf("%s%d", rolePrefix, i+1),
			role:   rolePrefix,
		}
		require.NoError(t, env.connectionRegistry.Register(conn.userID, conn))
		connections[i] = conn
	}
	
	return connections
}

func generateRealisticQuestion(studentIndex, questionIndex int) string {
	questions := []string{
		"Can you explain how photosynthesis works?",
		"What's the difference between mitosis and meiosis?",
		"How do enzymes catalyze biochemical reactions?",
		"What role does DNA play in protein synthesis?",
		"Can you describe the structure of a cell membrane?",
		"How does cellular respiration produce energy?",
		"What are the different types of RNA?",
		"How do genetic mutations occur?",
		"Can you explain the process of evolution?",
		"What is the significance of genetic diversity?",
	}
	
	baseQuestion := questions[questionIndex%len(questions)]
	return fmt.Sprintf("[Student %d] %s", studentIndex+1, baseQuestion)
}

func generateRealisticAnnouncement(instructorIndex, announcementIndex int) string {
	announcements := []string{
		"Welcome to today's biology session!",
		"Let's focus on cellular processes for the next 10 minutes",
		"Great questions everyone! Keep them coming",
		"We'll take a 5-minute break in a few minutes",
		"Remember to review chapter 3 for next week",
		"I'll post additional resources after class",
		"Let's summarize what we've covered so far",
		"Any final questions before we wrap up?",
	}
	
	baseAnnouncement := announcements[announcementIndex%len(announcements)]
	return fmt.Sprintf("[Instructor %d] %s", instructorIndex+1, baseAnnouncement)
}

func generateRealisticResponse(instructorIndex, responseIndex int) string {
	responses := []string{
		"Excellent question! The key point is...",
		"That's a common misconception. Actually...",
		"Let me break that down into simpler terms...",
		"Good observation! This connects to what we discussed earlier...",
		"I'm glad you asked that. The answer involves...",
		"That's exactly right! To expand on that...",
		"Interesting perspective. Consider this additional factor...",
		"Perfect timing for that question. Here's the explanation...",
	}
	
	baseResponse := responses[responseIndex%len(responses)]
	return fmt.Sprintf("[Instructor %d] %s", instructorIndex+1, baseResponse)
}

func createQuestionMessage(content string) []byte {
	msg := map[string]interface{}{
		"type":    database.MessageTypeBroadcastToInstructors,
		"context": database.ContextQuestion,
		"content": map[string]interface{}{"text": content},
	}
	data, _ := json.Marshal(msg)
	return data
}

func createAnnouncementMessage(content string) []byte {
	msg := map[string]interface{}{
		"type":    database.MessageTypeBroadcastToStudents,
		"context": database.ContextAnnouncement,
		"content": map[string]interface{}{"text": content},
	}
	data, _ := json.Marshal(msg)
	return data
}

func createDirectResponseMessage(toUser, content string) []byte {
	msg := map[string]interface{}{
		"type":    database.MessageTypeDirectMessage,
		"context": database.ContextResponse,
		"content": map[string]interface{}{"text": content},
		"to_user": toUser,
	}
	data, _ := json.Marshal(msg)
	return data
}

// PerformanceConnection optimized for performance testing
type PerformanceConnection struct {
	userID            string
	role              string
	deliveredMessages int64 // Use atomic counter instead of slice for performance
}

func (pc *PerformanceConnection) GetUserID() string { return pc.userID }
func (pc *PerformanceConnection) GetRole() string   { return pc.role }

func (pc *PerformanceConnection) SendMessage(data []byte) error {
	// For performance testing, just count messages instead of storing them
	atomic.AddInt64(&pc.deliveredMessages, 1)
	return nil
}

func (pc *PerformanceConnection) GetDeliveredMessageCount() int64 {
	return atomic.LoadInt64(&pc.deliveredMessages)
}

func (pc *PerformanceConnection) Close() error                               { return nil }
func (pc *PerformanceConnection) WriteJSON(v interface{}) error              { return nil }
func (pc *PerformanceConnection) SetCredentials(username, role string) error { return nil }
func (pc *PerformanceConnection) UpdateActivity()                            {}
func (pc *PerformanceConnection) GetLastSeen() time.Time                     { return time.Now() }
func (pc *PerformanceConnection) SendCloseMessage(reason string) error       { return nil }