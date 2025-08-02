// Simple Performance Tests - Optimized for Speed (under 30 seconds total)
// Basic validation of system performance characteristics
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

// TestSimplePerformance runs all essential performance tests in under 30 seconds
func TestSimplePerformance(t *testing.T) {
	t.Run("basic_throughput", func(t *testing.T) {
		t.Parallel()
		env := setupSimpleTestEnvironment(t)
		defer env.cleanup()

		// Test basic message throughput - 5 seconds, 10 connections
		session := &database.Session{
			ID:        "basic-throughput",
			Name:      "Basic Throughput Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		// Create connections
		const numConnections = 10
		connections := make([]*SimpleConnection, numConnections)
		for i := 0; i < numConnections; i++ {
			conn := &SimpleConnection{
				userID: fmt.Sprintf("student%d", i+1),
				role:   "student",
			}
			require.NoError(t, env.connectionRegistry.Register(conn.userID, conn))
			connections[i] = conn
		}

		var messagesProcessed int64
		var wg sync.WaitGroup

		// Send messages for 3 seconds
		startTime := time.Now()
		testDuration := 3 * time.Second

		// Start message generators
		for i, conn := range connections {
			wg.Add(1)
			go func(connIndex int, connection *SimpleConnection) {
				defer wg.Done()
				
				msgCount := 0
				for time.Since(startTime) < testDuration {
					msg := createSimpleMessage(fmt.Sprintf("Message %d from %s", msgCount, connection.userID))
					err := env.processor.ProcessIncomingMessage(msg, connection.userID)
					if err == nil {
						atomic.AddInt64(&messagesProcessed, 1)
					}
					msgCount++
					time.Sleep(100 * time.Millisecond) // 10 msg/sec per connection
				}
			}(i, conn)
		}

		wg.Wait()
		duration := time.Since(startTime)
		throughput := float64(atomic.LoadInt64(&messagesProcessed)) / duration.Seconds()

		t.Logf("Basic Throughput Test Results:")
		t.Logf("  Duration: %v", duration)
		t.Logf("  Messages: %d", atomic.LoadInt64(&messagesProcessed))
		t.Logf("  Throughput: %.1f msg/sec", throughput)

		// Assertions
		assert.Greater(t, throughput, 10.0, "Should achieve >10 msg/sec")
		assert.Greater(t, atomic.LoadInt64(&messagesProcessed), int64(20), "Should process >20 messages")
	})

	t.Run("user_capacity", func(t *testing.T) {
		t.Parallel()
		env := setupSimpleTestEnvironment(t)
		defer env.cleanup()

		// Test user capacity - 20 users for 3 seconds
		session := &database.Session{
			ID:        "user-capacity",
			Name:      "User Capacity Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		const numUsers = 20
		connections := make([]*SimpleConnection, numUsers)
		
		// Create connections
		for i := 0; i < numUsers; i++ {
			conn := &SimpleConnection{
				userID: fmt.Sprintf("user%d", i+1),
				role:   "student",
			}
			err := env.connectionRegistry.Register(conn.userID, conn)
			require.NoError(t, err)
			connections[i] = conn
		}

		var messagesProcessed int64
		var wg sync.WaitGroup

		// Each user sends 2 messages over 2 seconds
		for i, conn := range connections {
			wg.Add(1)
			go func(userIndex int, connection *SimpleConnection) {
				defer wg.Done()
				
				for msgNum := 0; msgNum < 2; msgNum++ {
					msg := createSimpleMessage(fmt.Sprintf("User %d message %d", userIndex, msgNum))
					err := env.processor.ProcessIncomingMessage(msg, connection.userID)
					if err == nil {
						atomic.AddInt64(&messagesProcessed, 1)
					}
					time.Sleep(time.Duration(userIndex*50) * time.Millisecond) // Stagger messages
				}
			}(i, conn)
		}

		wg.Wait()
		totalMessages := atomic.LoadInt64(&messagesProcessed)

		// Check memory usage
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		memoryMB := float64(memStats.Alloc) / 1024 / 1024

		t.Logf("User Capacity Test Results:")
		t.Logf("  Users: %d", numUsers)
		t.Logf("  Messages: %d", totalMessages)
		t.Logf("  Memory: %.1f MB", memoryMB)

		// Assertions
		assert.Equal(t, numUsers, len(connections), "All users should connect")
		assert.Greater(t, totalMessages, int64(30), "Should process most messages")
		assert.Less(t, memoryMB, 50.0, "Memory should be reasonable")
	})

	t.Run("message_latency", func(t *testing.T) {
		t.Parallel()
		env := setupSimpleTestEnvironment(t)
		defer env.cleanup()

		// Test message processing latency
		session := &database.Session{
			ID:        "latency-test",
			Name:      "Message Latency Test",
			CreatedBy: "instructor1",
			StartTime: time.Now(),
			Status:    "active",
		}
		require.NoError(t, env.sessionManager.SetActiveSession(session))

		conn := &SimpleConnection{
			userID: "test-user",
			role:   "student",
		}
		require.NoError(t, env.connectionRegistry.Register(conn.userID, conn))

		// Measure latency for 10 messages
		var latencies []time.Duration
		var latencyMutex sync.Mutex

		for i := 0; i < 10; i++ {
			start := time.Now()
			msg := createSimpleMessage(fmt.Sprintf("Latency test message %d", i))
			err := env.processor.ProcessIncomingMessage(msg, conn.userID)
			latency := time.Since(start)
			
			require.NoError(t, err)
			
			latencyMutex.Lock()
			latencies = append(latencies, latency)
			latencyMutex.Unlock()
			
			time.Sleep(10 * time.Millisecond) // Small gap between messages
		}

		// Calculate average latency
		var totalLatency time.Duration
		maxLatency := latencies[0]
		for _, lat := range latencies {
			totalLatency += lat
			if lat > maxLatency {
				maxLatency = lat
			}
		}
		avgLatency := totalLatency / time.Duration(len(latencies))

		t.Logf("Message Latency Test Results:")
		t.Logf("  Messages tested: %d", len(latencies))
		t.Logf("  Average latency: %v", avgLatency)
		t.Logf("  Maximum latency: %v", maxLatency)

		// Assertions
		assert.Less(t, avgLatency, 50*time.Millisecond, "Average latency should be <50ms")
		assert.Less(t, maxLatency, 200*time.Millisecond, "Max latency should be <200ms")
	})
}

// Helper types and functions

type SimpleTestEnvironment struct {
	dbManager          *database.SQLiteDatabaseManager
	sessionManager     session.SessionManager
	connectionRegistry *websocket.ConnectionRegistry
	processor          *message.MessageProcessor
}

func setupSimpleTestEnvironment(t *testing.T) *SimpleTestEnvironment {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	schema, err := os.ReadFile("../../internal/database/migrations.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(schema))
	require.NoError(t, err)

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
	rateLimiter := rate.NewRateLimiterWithConfig(1000, time.Minute) // High limit for testing

	filterAdapter := websocket.NewFilterAdapter(roleBasedFilter)
	broadcastSystem := websocket.NewBroadcastSystem(connectionRegistry, filterAdapter)

	processor := message.NewMessageProcessor(
		sessionManager,
		dbManager,
		rateLimiter,
		messageRouter,
		broadcastSystem,
	)

	return &SimpleTestEnvironment{
		dbManager:          dbManager,
		sessionManager:     sessionManager,
		connectionRegistry: connectionRegistry,
		processor:          processor,
	}
}

func (env *SimpleTestEnvironment) cleanup() {
	if env.connectionRegistry != nil {
		env.connectionRegistry.Stop()
	}
	if env.dbManager != nil {
		env.dbManager.Stop()
	}
}

func createSimpleMessage(content string) []byte {
	msg := map[string]interface{}{
		"type":    database.MessageTypeBroadcastToInstructors,
		"context": database.ContextQuestion,
		"content": map[string]interface{}{"text": content},
	}
	data, _ := json.Marshal(msg)
	return data
}

type SimpleConnection struct {
	userID             string
	role               string
	messagesDelivered  int64
}

func (sc *SimpleConnection) GetUserID() string { return sc.userID }
func (sc *SimpleConnection) GetRole() string   { return sc.role }

func (sc *SimpleConnection) SendMessage(data []byte) error {
	atomic.AddInt64(&sc.messagesDelivered, 1)
	return nil
}

func (sc *SimpleConnection) Close() error                               { return nil }
func (sc *SimpleConnection) WriteJSON(v interface{}) error              { return nil }
func (sc *SimpleConnection) SetCredentials(username, role string) error { return nil }
func (sc *SimpleConnection) UpdateActivity()                            {}
func (sc *SimpleConnection) GetLastSeen() time.Time                     { return time.Now() }
func (sc *SimpleConnection) SendCloseMessage(reason string) error       { return nil }