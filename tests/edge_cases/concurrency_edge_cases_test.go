// Concurrency and Edge Case Tests
// Tests system behavior under extreme conditions, race conditions, and error scenarios
package edge_cases

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
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

// TestConcurrencyEdgeCases validates system behavior under concurrent stress
func TestConcurrencyEdgeCases(t *testing.T) {
	t.Run("session_state_race_conditions", func(t *testing.T) {
		// Test concurrent session start/end operations don't cause inconsistent state
		env := setupEdgeCaseEnvironment(t)
		defer env.cleanup()

		const numGoroutines = 50
		const numOperations = 10

		results := make(chan TestResult, numGoroutines*numOperations)
		var wg sync.WaitGroup

		// Launch multiple goroutines doing rapid session start/end cycles
		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				for op := 0; op < numOperations; op++ {
					sessionID := fmt.Sprintf("race-test-%d-%d", goroutineID, op)
					
					// Try to start session
					session := &database.Session{
						ID:        sessionID,
						Name:      fmt.Sprintf("Race Test %d-%d", goroutineID, op),
						CreatedBy: fmt.Sprintf("instructor%d", goroutineID),
						StartTime: time.Now(),
						Status:    "active",
					}
					
					startErr := env.sessionManager.SetActiveSession(session)
					
					// If start succeeded, try to end it
					var endErr error
					if startErr == nil {
						time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
						endErr = env.sessionManager.ClearActiveSession()
					}
					
					results <- TestResult{
						GoroutineID: goroutineID,
						Operation:   op,
						StartError:  startErr,
						EndError:    endErr,
					}
				}
			}(g)
		}

		wg.Wait()
		close(results)

		// Analyze results for consistency
		successfulStarts := 0
		successfulEnds := 0
		
		for result := range results {
			if result.StartError == nil {
				successfulStarts++
			}
			// Only count as successful end if there was no error AND we had a successful start
			if result.StartError == nil && result.EndError == nil {
				successfulEnds++
			}
		}

		// Verify system maintained consistency
		// Either starts should equal ends, or there should be at most one active session
		activeSession := env.sessionManager.GetActiveSession()
		
		if activeSession != nil {
			assert.Equal(t, successfulStarts, successfulEnds+1, 
				"If session is active, starts should be exactly one more than ends")
		} else {
			assert.Equal(t, successfulStarts, successfulEnds, 
				"If no active session, starts should equal ends")
		}

		t.Logf("Concurrent operations: %d starts succeeded, %d ends succeeded", 
			successfulStarts, successfulEnds)
	})

	t.Run("connection_registry_concurrent_modifications", func(t *testing.T) {
		// Test concurrent connection registration/unregistration doesn't corrupt registry
		env := setupEdgeCaseEnvironment(t)
		defer env.cleanup()

		const numConnections = 100
		const numOperationsPerConnection = 20

		connections := make([]*ConcurrentTestConnection, numConnections)
		for i := 0; i < numConnections; i++ {
			connections[i] = &ConcurrentTestConnection{
				userID: fmt.Sprintf("user%d", i),
				role:   "student",
			}
		}

		var wg sync.WaitGroup
		registrationErrors := make(chan error, numConnections*numOperationsPerConnection)

		// Concurrent register/unregister operations
		for i := range connections {
			wg.Add(1)
			go func(connIndex int, connection *ConcurrentTestConnection) {
				defer wg.Done()
				
				for op := 0; op < numOperationsPerConnection; op++ {
					// Register
					regErr := env.connectionRegistry.Register(connection.GetUserID(), connection)
					if regErr != nil {
						registrationErrors <- regErr
					}
					
					// Small random delay
					time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
					
					// Unregister
					env.connectionRegistry.Unregister(connection.GetUserID())
				}
			}(i, connections[i])
		}

		wg.Wait()
		close(registrationErrors)

		// Check for any errors during concurrent operations
		errorCount := 0
		for err := range registrationErrors {
			errorCount++
			t.Logf("Concurrent operation error: %v", err)
		}

		// Some errors are expected due to race conditions, but not too many
		assert.Less(t, errorCount, numConnections*numOperationsPerConnection/10, 
			"Error rate should be low during concurrent operations")

		// Registry should be in consistent state (empty or with predictable contents)
		allUsers := env.connectionRegistry.GetAllUsers()
		t.Logf("Final registry state: %d users", len(allUsers))
		
		// Verify no memory leaks by checking internal consistency
		instructors := env.connectionRegistry.GetInstructors()
		students := env.connectionRegistry.GetStudents()
		assert.Equal(t, len(allUsers), len(instructors)+len(students), 
			"User counts should be consistent across different views")
	})

	t.Run("message_processing_under_extreme_load", func(t *testing.T) {
		// Test message processing doesn't break under rapid concurrent message sending
		env := setupEdgeCaseEnvironment(t)
		defer env.cleanup()

		// Setup active session
		session := &database.Session{
			ID:        "extreme-load-test",
			Name:      "Extreme Load Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		// Create connections
		const numStudents = 20
		const numInstructors = 5
		const messagesPerUser = 50

		createTestConnections(t, env, numStudents, numInstructors)

		var wg sync.WaitGroup
		messageErrors := make(chan error, (numStudents+numInstructors)*messagesPerUser)
		messagesSent := make(chan int, (numStudents+numInstructors)*messagesPerUser)

		// Students send questions rapidly
		for s := 1; s <= numStudents; s++ {
			wg.Add(1)
			go func(studentID int) {
				defer wg.Done()
				
				for m := 0; m < messagesPerUser; m++ {
					msg := createTestMessage(
						database.MessageTypeBroadcastToInstructors,
						fmt.Sprintf("Student %d message %d under extreme load", studentID, m),
					)
					
					err := env.processor.ProcessIncomingMessage(msg, fmt.Sprintf("student%d", studentID))
					if err != nil {
						messageErrors <- err
					} else {
						messagesSent <- 1
					}
					
					// Random tiny delay to create realistic timing
					if rand.Intn(10) == 0 {
						time.Sleep(time.Microsecond * time.Duration(rand.Intn(100)))
					}
				}
			}(s)
		}

		// Instructors send announcements rapidly
		for i := 1; i <= numInstructors; i++ {
			wg.Add(1)
			go func(instructorID int) {
				defer wg.Done()
				
				for m := 0; m < messagesPerUser; m++ {
					msg := createTestMessage(
						database.MessageTypeBroadcastToStudents,
						fmt.Sprintf("Instructor %d announcement %d under extreme load", instructorID, m),
					)
					
					err := env.processor.ProcessIncomingMessage(msg, fmt.Sprintf("instructor%d", instructorID))
					if err != nil {
						messageErrors <- err
					} else {
						messagesSent <- 1
					}
				}
			}(i)
		}

		wg.Wait()
		close(messageErrors)
		close(messagesSent)

		// Analyze results
		errorCount := 0
		for err := range messageErrors {
			errorCount++
			if errorCount <= 5 { // Log first few errors
				t.Logf("Message processing error: %v", err)
			}
		}

		successCount := 0
		for range messagesSent {
			successCount++
		}

		totalAttempted := (numStudents + numInstructors) * messagesPerUser
		successRate := float64(successCount) / float64(totalAttempted)

		t.Logf("Extreme load test: %d/%d messages succeeded (%.2f%% success rate)", 
			successCount, totalAttempted, successRate*100)

		// Under extreme load, some messages might fail due to rate limiting or system limits
		// But success rate should still be reasonable
		assert.Greater(t, successRate, 0.7, "Success rate should be > 70% even under extreme load")

		// Verify database consistency after extreme load
		time.Sleep(500 * time.Millisecond) // Allow batched writes to complete
		messages, err := env.dbManager.GetSessionMessages(session.ID)
		require.NoError(t, err)
		
		// Messages persisted should match successful processing
		assert.Equal(t, successCount, len(messages), 
			"All successfully processed messages should be persisted")
	})

	t.Run("database_batch_writer_edge_cases", func(t *testing.T) {
		// Test database batch writer under various edge conditions
		env := setupEdgeCaseEnvironment(t)
		defer env.cleanup()

		session := &database.Session{
			ID:        "batch-edge-test",
			Name:      "Batch Writer Edge Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		createTestConnections(t, env, 3, 1)

		// Test Case 1: Rapid burst hitting batch size limit
		t.Run("batch_size_limit", func(t *testing.T) {
			const burstSize = 105 // Exceeds typical batch size of 100
			
			var wg sync.WaitGroup
			for i := 0; i < burstSize; i++ {
				wg.Add(1)
				go func(msgIndex int) {
					defer wg.Done()
					msg := createTestMessage(
						database.MessageTypeBroadcastToInstructors,
						fmt.Sprintf("Batch size test message %d", msgIndex),
					)
					if err := env.processor.ProcessIncomingMessage(msg, "student1"); err != nil {
						t.Logf("Failed to process message in batch size test: %v", err)
					}
				}(i)
			}
			wg.Wait()

			// Wait for batch processing
			time.Sleep(300 * time.Millisecond)
			
			messages, err := env.dbManager.GetSessionMessages(session.ID)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(messages), burstSize-10, 
				"Most burst messages should be persisted despite batch limits")
		})

		// Test Case 2: Messages with varying processing times
		t.Run("mixed_processing_times", func(t *testing.T) {
			const numMixedMessages = 30
			
			for i := 0; i < numMixedMessages; i++ {
				// Some messages with complex content, others simple
				var content string
				if i%3 == 0 {
					content = generateComplexContent(1000) // Large content
				} else {
					content = fmt.Sprintf("Simple message %d", i)
				}
				
				msg := createTestMessage(database.MessageTypeBroadcastToStudents, content)
				require.NoError(t, env.processor.ProcessIncomingMessage(msg, "instructor1"))
				
				// Vary timing
				if i%5 == 0 {
					time.Sleep(10 * time.Millisecond)
				}
			}

			time.Sleep(400 * time.Millisecond) // Allow processing
			
			messages, err := env.dbManager.GetSessionMessages(session.ID)
			require.NoError(t, err)
			
			// Count mixed messages (excluding previous burst messages)
			mixedMessages := 0
			for _, msg := range messages {
				if msg.FromUser == "instructor1" {
					mixedMessages++
				}
			}
			assert.Equal(t, numMixedMessages, mixedMessages, 
				"All mixed processing time messages should be persisted")
		})
	})

	t.Run("memory_pressure_and_cleanup", func(t *testing.T) {
		// Test system behavior under memory pressure and cleanup scenarios
		env := setupEdgeCaseEnvironment(t)
		defer env.cleanup()

		session := &database.Session{
			ID:        "memory-pressure-test",
			Name:      "Memory Pressure Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		// Create many connections to simulate memory pressure
		const numConnections = 200
		connections := make([]*ConcurrentTestConnection, numConnections)
		
		for i := 0; i < numConnections; i++ {
			conn := &ConcurrentTestConnection{
				userID: fmt.Sprintf("user%d", i),
				role:   "student",
			}
			connections[i] = conn
			require.NoError(t, env.connectionRegistry.Register(conn.GetUserID(), conn))
		}

		// Generate message traffic to fill buffers
		const messagesPerConnection = 5
		var wg sync.WaitGroup
		
		for i, conn := range connections {
			wg.Add(1)
			go func(connIndex int, connection *ConcurrentTestConnection) {
				defer wg.Done()
				
				for m := 0; m < messagesPerConnection; m++ {
					msg := createTestMessage(
						database.MessageTypeBroadcastToInstructors,
						fmt.Sprintf("Memory pressure message from %s #%d", connection.GetUserID(), m),
					)
					if err := env.processor.ProcessIncomingMessage(msg, connection.GetUserID()); err != nil {
						t.Logf("Failed to process message in memory pressure test: %v", err)
					}
					
					time.Sleep(time.Millisecond) // Small delay
				}
			}(i, conn)
		}

		wg.Wait()

		// Force cleanup of stale connections
		time.Sleep(200 * time.Millisecond)
		
		// Simulate some connections becoming stale by setting old heartbeat timestamps
		staleTime := time.Now().Add(-30 * time.Minute) // Older than 25-minute timeout
		for i := 0; i < 50; i++ {
			connections[i].SetStale(true)
			env.connectionRegistry.SetHeartbeatTimeForTest(connections[i].GetUserID(), staleTime)
		}

		// Temporarily clear active session to allow cleanup (cleanup only works with no active session)
		require.NoError(t, env.sessionManager.ClearActiveSession())
		
		// Trigger cleanup
		env.connectionRegistry.CleanupStaleConnectionsForTest()
		
		// Restore active session for final test
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		// Verify system is still functional after cleanup
		activeConnections := env.connectionRegistry.GetAllUsers()
		t.Logf("Active connections after cleanup: %d (started with %d, marked %d as stale)", 
			len(activeConnections), numConnections, 50)
		
		// Should have cleaned up the 50 stale connections
		assert.Equal(t, numConnections-50, len(activeConnections), 
			"Should have exactly 150 connections after cleaning up 50 stale ones")

		// System should still process messages normally
		testMsg := createTestMessage(database.MessageTypeBroadcastToStudents, "Post-cleanup test")
		require.NoError(t, env.processor.ProcessIncomingMessage(testMsg, "instructor1"))
		
		// Verify the message was delivered to remaining active connections (not stale ones)
		time.Sleep(100 * time.Millisecond) // Allow message delivery
		deliveredCount := 0
		for _, conn := range activeConnections {
			if testConn, ok := conn.(*ConcurrentTestConnection); ok {
				messages := testConn.GetDeliveredMessages()
				if len(messages) > 0 {
					deliveredCount++
				}
			}
		}
		assert.Greater(t, deliveredCount, 0, "Message should be delivered to some active connections")
	})

	t.Run("rate_limiter_boundary_conditions", func(t *testing.T) {
		// Test rate limiter behavior at boundaries and edge cases
		env := setupEdgeCaseEnvironment(t)
		defer env.cleanup()

		session := &database.Session{
			ID:        "rate-limit-boundary-test",
			Name:      "Rate Limit Boundary Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		createTestConnections(t, env, 2, 1)

		// Test Case 1: Exactly at rate limit boundary
		t.Run("exact_rate_limit", func(t *testing.T) {
			const rateLimitPerMinute = 100 // From config
			
			// Send messages rapidly to test rate limiting
			successCount := 0
			rejectedCount := 0
			
			// Send 150 messages rapidly (should exceed 100/minute limit)
			for i := 0; i < 150; i++ {
				msg := createTestMessage(database.MessageTypeBroadcastToInstructors, 
					fmt.Sprintf("Rate limit test message %d", i))
				
				err := env.processor.ProcessIncomingMessage(msg, "student1")
				if err == nil {
					successCount++
				} else {
					rejectedCount++
					t.Logf("Message %d rejected: %v", i, err)
				}
			}
			
			t.Logf("Rate limit test: %d allowed, %d rejected", 
				successCount, rejectedCount)
			
			// Should allow exactly 100 messages (rate limit), reject 50
			assert.Equal(t, rateLimitPerMinute, successCount, 
				"Should allow exactly the rate limit number of messages")
			assert.Equal(t, 50, rejectedCount, 
				"Should reject messages over the rate limit")
		})

		// Test Case 2: Burst followed by normal rate
		t.Run("burst_then_normal", func(t *testing.T) {
			// Use a different user to avoid interference with previous test
			// Send initial burst of 120 messages (should hit limit at 100)
			burstSize := 120
			burstSuccesses := 0
			burstRejected := 0
			
			for i := 0; i < burstSize; i++ {
				msg := createTestMessage(database.MessageTypeBroadcastToInstructors, 
					fmt.Sprintf("Burst message %d", i))
				err := env.processor.ProcessIncomingMessage(msg, "student2")
				if err == nil {
					burstSuccesses++
				} else {
					burstRejected++
				}
			}
			
			t.Logf("Burst test: %d/%d burst messages succeeded, %d rejected", 
				burstSuccesses, burstSize, burstRejected)
			
			// Burst should hit rate limit: 100 accepted, 20 rejected
			assert.Equal(t, 100, burstSuccesses, "Should accept exactly 100 burst messages")
			assert.Equal(t, 20, burstRejected, "Should reject 20 messages over limit")
		})
	})
}

// Helper functions and test utilities

type TestResult struct {
	GoroutineID int
	Operation   int
	StartError  error
	EndError    error
}

type EdgeCaseEnvironment struct {
	dbManager          *database.SQLiteDatabaseManager
	sessionManager     session.SessionManager
	connectionRegistry *websocket.ConnectionRegistry
	processor          *message.MessageProcessor
}

func setupEdgeCaseEnvironment(t *testing.T) *EdgeCaseEnvironment {
	// Setup in-memory database
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

	return &EdgeCaseEnvironment{
		dbManager:          dbManager,
		sessionManager:     sessionManager,
		connectionRegistry: connectionRegistry,
		processor:          processor,
	}
}

func (env *EdgeCaseEnvironment) cleanup() {
	env.connectionRegistry.Stop()
	if err := env.dbManager.Stop(); err != nil {
		log.Printf("Failed to stop database manager: %v", err)
	}
}

func createTestConnections(t *testing.T, env *EdgeCaseEnvironment, numStudents, numInstructors int) {
	for i := 1; i <= numStudents; i++ {
		conn := &ConcurrentTestConnection{
			userID: fmt.Sprintf("student%d", i),
			role:   "student",
		}
		require.NoError(t, env.connectionRegistry.Register(conn.GetUserID(), conn))
	}

	for i := 1; i <= numInstructors; i++ {
		conn := &ConcurrentTestConnection{
			userID: fmt.Sprintf("instructor%d", i),
			role:   "instructor",
		}
		require.NoError(t, env.connectionRegistry.Register(conn.GetUserID(), conn))
	}
}

func createTestMessage(msgType, content string) []byte {
	msg := map[string]interface{}{
		"type":    msgType,
		"context": database.ContextQuestion,
		"content": map[string]interface{}{"text": content},
	}
	data, _ := json.Marshal(msg)
	return data
}

func generateComplexContent(size int) string {
	// Generate complex JSON-like content
	result := "{"
	for i := 0; i < size/50; i++ {
		result += fmt.Sprintf(`"field_%d": "value with special chars <>!@#$%%^&*()_+{}[]", `, i)
	}
	result += `"final": "content"}`
	
	if len(result) > size {
		return result[:size]
	}
	return result
}

// ConcurrentTestConnection for edge case testing
type ConcurrentTestConnection struct {
	userID            string
	role              string
	deliveredMessages [][]byte
	mu                sync.RWMutex
	stale             bool
}

func (ctc *ConcurrentTestConnection) GetUserID() string { return ctc.userID }
func (ctc *ConcurrentTestConnection) GetRole() string   { return ctc.role }

func (ctc *ConcurrentTestConnection) SendMessage(data []byte) error {
	ctc.mu.Lock()
	defer ctc.mu.Unlock()
	
	if ctc.stale {
		return fmt.Errorf("connection is stale")
	}
	
	msgCopy := make([]byte, len(data))
	copy(msgCopy, data)
	ctc.deliveredMessages = append(ctc.deliveredMessages, msgCopy)
	return nil
}

func (ctc *ConcurrentTestConnection) SetStale(stale bool) {
	ctc.mu.Lock()
	defer ctc.mu.Unlock()
	ctc.stale = stale
}

func (ctc *ConcurrentTestConnection) GetLastSeen() time.Time {
	ctc.mu.RLock()
	defer ctc.mu.RUnlock()
	
	if ctc.stale {
		return time.Now().Add(-10 * time.Minute) // Simulate old timestamp
	}
	return time.Now()
}

func (ctc *ConcurrentTestConnection) GetDeliveredMessages() [][]byte {
	ctc.mu.RLock()
	defer ctc.mu.RUnlock()
	
	result := make([][]byte, len(ctc.deliveredMessages))
	for i, msg := range ctc.deliveredMessages {
		result[i] = make([]byte, len(msg))
		copy(result[i], msg)
	}
	return result
}

func (ctc *ConcurrentTestConnection) Close() error                               { return nil }
func (ctc *ConcurrentTestConnection) WriteJSON(v interface{}) error              { return nil }
func (ctc *ConcurrentTestConnection) SetCredentials(username, role string) error { return nil }
func (ctc *ConcurrentTestConnection) UpdateActivity()                            {}
func (ctc *ConcurrentTestConnection) SendCloseMessage(reason string) error       { return nil }