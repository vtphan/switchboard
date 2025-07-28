package router

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ws "switchboard/internal/websocket"
	"switchboard/pkg/types"
)

// LoadTestMetrics tracks performance metrics during load testing
type LoadTestMetrics struct {
	MessagesRouted       int64
	MessagesPersisted    int64
	MessagesDropped      int64
	ConnectionErrors     int64
	RoutingErrors        int64
	TotalLatency         int64
	MaxLatency           int64
	MinLatency           int64
	ActiveConnections    int64
	PeakMemoryUsage      uint64
	StartTime            time.Time
	EndTime              time.Time
}

func (m *LoadTestMetrics) AddLatency(latency time.Duration) {
	nanos := latency.Nanoseconds()
	atomic.AddInt64(&m.TotalLatency, nanos)
	
	// Update max latency
	for {
		currentMax := atomic.LoadInt64(&m.MaxLatency)
		if nanos <= currentMax || atomic.CompareAndSwapInt64(&m.MaxLatency, currentMax, nanos) {
			break
		}
	}
	
	// Update min latency (initialize if zero)
	for {
		currentMin := atomic.LoadInt64(&m.MinLatency)
		if currentMin == 0 {
			if atomic.CompareAndSwapInt64(&m.MinLatency, 0, nanos) {
				break
			}
		} else if nanos >= currentMin || atomic.CompareAndSwapInt64(&m.MinLatency, currentMin, nanos) {
			break
		}
	}
}

func (m *LoadTestMetrics) GetAverageLatency() time.Duration {
	totalLatency := atomic.LoadInt64(&m.TotalLatency)
	messagesRouted := atomic.LoadInt64(&m.MessagesRouted)
	if messagesRouted == 0 {
		return 0
	}
	return time.Duration(totalLatency / messagesRouted)
}

func (m *LoadTestMetrics) GetThroughput() float64 {
	endTime := m.EndTime
	if endTime.IsZero() {
		endTime = time.Now() // Use current time if EndTime not set yet
	}
	
	duration := endTime.Sub(m.StartTime).Seconds()
	if duration <= 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&m.MessagesRouted)) / duration
}

// MockRateLimiter always allows messages for load testing
type MockRateLimiter struct{}

func (m *MockRateLimiter) Allow(userID string) bool {
	return true
}

// LoadTestSetup manages load testing infrastructure
type LoadTestSetup struct {
	server      *httptest.Server
	registry    *ws.Registry
	router      *Router
	mockDB      *TrackedMockDB
	upgrader    websocket.Upgrader
	connections []*ws.Connection
	metrics     *LoadTestMetrics
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewLoadTestSetup() *LoadTestSetup {
	return NewLoadTestSetupWithOptions(false)
}

func NewLoadTestSetupWithOptions(useRealRateLimiter bool) *LoadTestSetup {
	ctx, cancel := context.WithCancel(context.Background())
	
	setup := &LoadTestSetup{
		registry:    ws.NewRegistry(),
		mockDB:      NewTrackedMockDB(),
		upgrader:    websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		connections: make([]*ws.Connection, 0),
		metrics:     &LoadTestMetrics{StartTime: time.Now()},
		ctx:         ctx,
		cancel:      cancel,
	}
	
	setup.router = NewRouter(setup.registry, setup.mockDB)
	
	// Only replace rate limiter if not testing rate limiting
	if !useRealRateLimiter {
		setup.router.rateLimiter = &MockRateLimiter{}
	}
	
	// Enable batching for load testing
	if err := setup.router.EnableBatching(50, 100*time.Millisecond); err != nil {
		panic(fmt.Sprintf("Failed to enable batching: %v", err))
	}
	
	// Create test WebSocket server
	setup.server = httptest.NewServer(http.HandlerFunc(setup.handleWebSocket))
	
	return setup
}

func (s *LoadTestSetup) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		atomic.AddInt64(&s.metrics.ConnectionErrors, 1)
		return
	}
	
	// Create connection wrapper
	wsConn := ws.NewConnection(conn)
	
	// Keep connection alive for the duration of the test with periodic ping
	go func() {
		defer func() {
			if err := wsConn.Close(); err != nil {
				// Connection close error is expected during cleanup, ignore
				_ = err
			}
		}()
		
		// More frequent pings for load testing stability
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		// Set up pong handler to track connection health
		conn.SetPongHandler(func(appData string) error {
			// Reset deadline on successful pong
			return conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		})
		
		// Set initial read deadline
		conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		
		// Start a goroutine to consume any incoming messages (to prevent buffer overflow)
		go func() {
			for {
				select {
				case <-s.ctx.Done():
					return
				default:
					_, _, err := conn.ReadMessage()
					if err != nil {
						return // Connection closed
					}
				}
			}
		}()
		
		for {
			select {
			case <-ticker.C:
				// Send ping with shorter write deadline
				if err := conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
					return
				}
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					// Connection is dead, exit gracefully
					return
				}
			case <-s.ctx.Done():
				return
			}
		}
	}()
}

func (s *LoadTestSetup) CreateClassroom(sessionID string, numStudents, numInstructors int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Enable batch mode to prevent broadcast storm during setup
	s.registry.SetBatchMode(true)
	defer func() {
		// Disable batch mode and send single batch presence update
		s.registry.SetBatchMode(false)
		s.registry.SendBatchPresenceUpdate()
	}()
	
	// Create student connections
	for i := 0; i < numStudents; i++ {
		userID := fmt.Sprintf("student%d_%s", i+1, sessionID)
		conn, err := s.createConnection(userID, "student", sessionID)
		if err != nil {
			atomic.AddInt64(&s.metrics.ConnectionErrors, 1)
			return err
		}
		s.connections = append(s.connections, conn)
		atomic.AddInt64(&s.metrics.ActiveConnections, 1)
	}
	
	// Create instructor connections
	for i := 0; i < numInstructors; i++ {
		userID := fmt.Sprintf("instructor%d_%s", i+1, sessionID)
		conn, err := s.createConnection(userID, "instructor", sessionID)
		if err != nil {
			atomic.AddInt64(&s.metrics.ConnectionErrors, 1)
			return err
		}
		s.connections = append(s.connections, conn)
		atomic.AddInt64(&s.metrics.ActiveConnections, 1)
	}
	
	return nil
}

func (s *LoadTestSetup) createConnection(userID, role, sessionID string) (*ws.Connection, error) {
	wsURL := "ws" + strings.TrimPrefix(s.server.URL, "http")
	rawConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}
	
	conn := ws.NewConnection(rawConn)
	err = conn.SetCredentials(userID, role, sessionID)
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			// Connection close error during cleanup, ignore
			_ = closeErr
		}
		return nil, err
	}
	
	err = s.registry.RegisterConnection(conn)
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			// Connection close error during cleanup, ignore
			_ = closeErr
		}
		return nil, err
	}
	
	return conn, nil
}

func (s *LoadTestSetup) SendMessage(message *types.Message) {
	start := time.Now()
	
	err := s.router.RouteMessage(s.ctx, message)
	
	latency := time.Since(start)
	s.metrics.AddLatency(latency)
	
	if err != nil {
		atomic.AddInt64(&s.metrics.RoutingErrors, 1)
	} else {
		atomic.AddInt64(&s.metrics.MessagesRouted, 1)
	}
}

func (s *LoadTestSetup) Cleanup() {
	s.cancel()
	s.metrics.EndTime = time.Now()
	
	if s.router.batcher != nil {
		if err := s.router.batcher.Stop(); err != nil {
			// Batcher stop error during cleanup, log but continue
			_ = err // Explicitly ignore error for cleanup
		}
	}
	
	s.mu.Lock()
	for _, conn := range s.connections {
		if err := conn.Close(); err != nil {
			// Connection close error during cleanup, expected behavior
			_ = err // Explicitly ignore error for cleanup
		}
	}
	s.connections = nil
	s.mu.Unlock()
	
	if s.server != nil {
		s.server.Close()
	}
}

func (s *LoadTestSetup) GetMetrics() *LoadTestMetrics {
	s.metrics.MessagesPersisted = int64(s.mockDB.GetStoreMessageCalls() + s.mockDB.GetStoreMessageBatchCalls())
	
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	s.metrics.PeakMemoryUsage = m.Alloc
	
	return s.metrics
}

// Rate Limiting Load Test
func TestRouter_LoadTest_RateLimitingUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}
	
	setup := NewLoadTestSetupWithOptions(true) // Use real rate limiter
	defer setup.Cleanup()
	
	// Create smaller classroom for rate limit testing
	err := setup.CreateClassroom("rate_test", 5, 1)
	require.NoError(t, err)
	
	// Send messages rapidly to trigger rate limiting
	var wg sync.WaitGroup
	messagesPerUser := 150 // Above the 100/minute limit
	
	// Create message sending goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(studentNum int) {
			defer wg.Done()
			
			userID := fmt.Sprintf("student%d_rate_test", studentNum+1)
			
			for j := 0; j < messagesPerUser; j++ {
				message := &types.Message{
					SessionID: "rate_test",
					Type:      types.MessageTypeAnalytics,
					FromUser:  userID,
					Content: map[string]interface{}{
						"event":     "rapid_fire_test",
						"sequence":  j,
						"timestamp": time.Now(),
					},
				}
				
				setup.SendMessage(message)
				
				// Small delay between messages to simulate realistic sending
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Allow time for processing
	time.Sleep(2 * time.Second)
	
	metrics := setup.GetMetrics()
	
	t.Logf("Rate Limiting Load Test Results:")
	t.Logf("  Messages Attempted: %d", 5*messagesPerUser)
	t.Logf("  Messages Routed: %d", metrics.MessagesRouted)
	t.Logf("  Messages Blocked: %d", metrics.RoutingErrors)
	t.Logf("  Block Rate: %.1f%%", float64(metrics.RoutingErrors)/float64(5*messagesPerUser)*100)
	t.Logf("  Average Latency: %v", metrics.GetAverageLatency())
	t.Logf("  Peak Memory: %d KB", metrics.PeakMemoryUsage/1024)
	
	// Expect significant rate limiting
	assert.Greater(t, metrics.RoutingErrors, int64(100), "Should have blocked many messages due to rate limiting")
	assert.Less(t, metrics.MessagesRouted, int64(600), "Should have limited throughput significantly")
	
	// Should still maintain reasonable performance for allowed messages
	assert.Less(t, metrics.GetAverageLatency(), 50*time.Millisecond, "Should maintain low latency for allowed messages")
}

// Memory Stability Load Test
func TestRouter_LoadTest_MemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}
	
	setup := NewLoadTestSetup()
	defer setup.Cleanup()
	
	// Create larger classroom for memory testing
	err := setup.CreateClassroom("memory_test", 30, 2)
	require.NoError(t, err)
	
	// Track memory usage over time
	memorySnapshots := make([]uint64, 0)
	
	// Send messages continuously and monitor memory
	var wg sync.WaitGroup
	duration := 60 * time.Second
	startTime := time.Now()
	
	// Memory monitoring goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for time.Since(startTime) < duration {
			select {
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				memorySnapshots = append(memorySnapshots, m.Alloc)
			}
		}
	}()
	
	// Message sending goroutines
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(studentNum int) {
			defer wg.Done()
			
			userID := fmt.Sprintf("student%d_memory_test", studentNum+1)
			
			for time.Since(startTime) < duration {
				message := &types.Message{
					SessionID: "memory_test",
					Type:      types.MessageTypeAnalytics,
					FromUser:  userID,
					Content: map[string]interface{}{
						"event":     "memory_test_event",
						"timestamp": time.Now(),
						"data":      strings.Repeat("x", 100), // Small payload
					},
				}
				
				setup.SendMessage(message)
				time.Sleep(500 * time.Millisecond) // 2 messages per second per student
			}
		}(i)
	}
	
	wg.Wait()
	
	// Allow cleanup time
	time.Sleep(5 * time.Second)
	
	metrics := setup.GetMetrics()
	
	t.Logf("Memory Stability Load Test Results:")
	t.Logf("  Test Duration: %v", duration)
	t.Logf("  Messages Routed: %d", metrics.MessagesRouted)
	t.Logf("  Messages Persisted: %d", metrics.MessagesPersisted)
	t.Logf("  Routing Errors: %d", metrics.RoutingErrors)
	t.Logf("  Peak Memory: %d KB", metrics.PeakMemoryUsage/1024)
	
	// Analyze memory growth
	if len(memorySnapshots) >= 2 {
		startMem := memorySnapshots[0]
		endMem := memorySnapshots[len(memorySnapshots)-1]
		growth := float64(endMem-startMem) / float64(startMem) * 100
		t.Logf("  Memory Growth: %.1f%% (from %d KB to %d KB)", 
			growth, startMem/1024, endMem/1024)
		
		// Should not have excessive memory growth
		assert.Less(t, growth, 200.0, "Memory growth should be reasonable (<200%)")
	}
	
	// Should handle sustained load without errors
	assert.Equal(t, int64(0), metrics.RoutingErrors, "Should have no routing errors during sustained load")
	assert.Greater(t, metrics.MessagesRouted, int64(1000), "Should handle substantial message volume")
	assert.Less(t, metrics.GetAverageLatency(), 100*time.Millisecond, "Should maintain reasonable latency")
}

// Connection Stability Load Test
func TestRouter_LoadTest_ConnectionStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}
	
	setup := NewLoadTestSetup()
	defer setup.Cleanup()
	
	// Test connection churn - connections joining and leaving frequently
	var wg sync.WaitGroup
	duration := 45 * time.Second
	startTime := time.Now()
	
	// Stable connection pool
	err := setup.CreateClassroom("stable_test", 10, 1)
	require.NoError(t, err)
	
	// Churning connections
	wg.Add(1)
	go func() {
		defer wg.Done()
		churnCount := 0
		
		for time.Since(startTime) < duration {
			// Create a temporary connection
			userID := fmt.Sprintf("churn_user_%d", churnCount)
			conn, err := setup.createConnection(userID, "student", "stable_test")
			if err != nil {
				atomic.AddInt64(&setup.metrics.ConnectionErrors, 1)
				continue
			}
			
			// Send a few messages
			for j := 0; j < 3; j++ {
				message := &types.Message{
					SessionID: "stable_test",
					Type:      types.MessageTypeAnalytics,
					FromUser:  userID,
					Content: map[string]interface{}{
						"event": "churn_message",
						"count": j,
					},
				}
				setup.SendMessage(message)
			}
			
			// Disconnect after short time
			time.Sleep(2 * time.Second)
			setup.registry.UnregisterConnection(conn)
			conn.Close()
			
			churnCount++
			time.Sleep(1 * time.Second) // Brief pause between churns
		}
		
		t.Logf("Connection churn cycles completed: %d", churnCount)
	}()
	
	// Stable connections sending messages
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(studentNum int) {
			defer wg.Done()
			
			userID := fmt.Sprintf("student%d_stable_test", studentNum+1)
			
			for time.Since(startTime) < duration {
				message := &types.Message{
					SessionID: "stable_test",
					Type:      types.MessageTypeAnalytics,
					FromUser:  userID,
					Content: map[string]interface{}{
						"event":     "stability_test",
						"timestamp": time.Now(),
					},
				}
				
				setup.SendMessage(message)
				time.Sleep(1 * time.Second)
			}
		}(i)
	}
	
	wg.Wait()
	
	metrics := setup.GetMetrics()
	
	t.Logf("Connection Stability Load Test Results:")
	t.Logf("  Test Duration: %v", duration)
	t.Logf("  Messages Routed: %d", metrics.MessagesRouted)
	t.Logf("  Connection Errors: %d", metrics.ConnectionErrors)
	t.Logf("  Routing Errors: %d", metrics.RoutingErrors)
	t.Logf("  Average Latency: %v", metrics.GetAverageLatency())
	t.Logf("  Peak Memory: %d KB", metrics.PeakMemoryUsage/1024)
	
	// Should handle connection churn gracefully
	assert.Less(t, metrics.ConnectionErrors, int64(10), "Should have minimal connection errors")
	assert.Equal(t, int64(0), metrics.RoutingErrors, "Should have no routing errors despite connection churn")
	assert.Greater(t, metrics.MessagesRouted, int64(200), "Should successfully route messages during churn")
	assert.Less(t, metrics.GetAverageLatency(), 150*time.Millisecond, "Should maintain reasonable latency during churn")
}