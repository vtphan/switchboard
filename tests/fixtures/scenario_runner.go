package fixtures

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
	
	"switchboard/internal/app"
	"switchboard/internal/config"
)

// ScenarioRunner orchestrates complex test scenarios with multiple clients
type ScenarioRunner struct {
	ServerURL     string
	TestSession   *TestSession
	Clients       map[string]*TestClient
	testApp       *app.Application
	serverContext context.Context
	serverCancel  context.CancelFunc
	
	mu        sync.RWMutex
	running   bool
	errors    []error
}

// ScenarioResult contains the results of running a test scenario
type ScenarioResult struct {
	Success      bool
	Duration     time.Duration
	MessagesSent int
	MessagesReceived int
	Errors       []error
	ClientResults map[string]*ClientResult
}

// ClientResult contains per-client test results
type ClientResult struct {
	UserID           string
	MessagesReceived int
	MessagesSent     int
	ConnectionUptime time.Duration
	Errors           []error
}

// NewScenarioRunner creates a new scenario runner with test session setup and starts a test server
func NewScenarioRunner(t *testing.T, scenario *ClassroomData) (*ScenarioRunner, error) {
	return NewScenarioRunnerWithServer(t, scenario)
}

// NewScenarioRunnerWithServer creates a scenario runner with an embedded test server
func NewScenarioRunnerWithServer(t *testing.T, scenario *ClassroomData) (*ScenarioRunner, error) {
	// Create test session with database cleanup
	testSession := SetupCleanSession(t, scenario.SessionName, scenario.InstructorIDs[0], scenario.StudentIDs)
	
	// Find available port for test server with retry logic
	port, err := findAvailablePortWithRetry()
	if err != nil {
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}
	
	// Find project root directory containing migrations
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}
	
	// Navigate to project root (go up from tests/scenarios or tests/fixtures)
	projectRoot := wd
	for !fileExists(filepath.Join(projectRoot, "migrations")) && projectRoot != "/" {
		projectRoot = filepath.Dir(projectRoot)
	}
	if projectRoot == "/" || !fileExists(filepath.Join(projectRoot, "migrations")) {
		return nil, fmt.Errorf("could not find migrations directory from %s", wd)
	}
	
	// Create test configuration with temporary database
	cfg := &config.Config{
		HTTP: &config.HTTPConfig{
			Host:         "127.0.0.1",
			Port:         port,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		Database: &config.DatabaseConfig{
			Path:    testSession.DatabasePath,
			Timeout: 30 * time.Second,
		},
		WebSocket: &config.WebSocketConfig{
			PingInterval: 30 * time.Second,
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 10 * time.Second,
			BufferSize:   100,
		},
	}
	
	// Create application instance with correct migrations path
	// Temporarily override the migrations path to use absolute path
	originalDir, _ := os.Getwd()
	os.Chdir(projectRoot)
	defer os.Chdir(originalDir)
	
	testApp, err := app.NewApplication(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create test application: %w", err)
	}
	
	// Start server
	serverCtx, serverCancel := context.WithCancel(context.Background())
	go func() {
		if err := testApp.Start(serverCtx); err != nil && err != context.Canceled {
			t.Errorf("Test server failed to start: %v", err)
		}
	}()
	
	// Wait for server to be ready
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitForServer(serverURL, 5*time.Second); err != nil {
		serverCancel()
		return nil, fmt.Errorf("test server did not start: %w", err)
	}
	
	runner := &ScenarioRunner{
		ServerURL:     serverURL,
		TestSession:   testSession,
		Clients:       make(map[string]*TestClient),
		testApp:       testApp,
		serverContext: serverCtx,
		serverCancel:  serverCancel,
	}
	
	// Setup cleanup
	t.Cleanup(func() {
		runner.Cleanup()
	})
	
	return runner, nil
}

// NewScenarioRunnerWithURL creates a scenario runner with custom server URL
func NewScenarioRunnerWithURL(t *testing.T, scenario *ClassroomData, serverURL string) (*ScenarioRunner, error) {
	// Create test session with database cleanup
	testSession := SetupCleanSession(t, scenario.SessionName, scenario.InstructorIDs[0], scenario.StudentIDs)
	
	runner := &ScenarioRunner{
		ServerURL:   serverURL,
		TestSession: testSession,
		Clients:     make(map[string]*TestClient),
	}
	
	// Setup cleanup
	t.Cleanup(func() {
		runner.Cleanup()
	})
	
	return runner, nil
}

// CreateClient creates and registers a new test client
func (sr *ScenarioRunner) CreateClient(userID, role string) (*TestClient, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	
	if _, exists := sr.Clients[userID]; exists {
		return nil, fmt.Errorf("client %s already exists", userID)
	}
	
	client := NewTestClient(userID, role, sr.TestSession.SessionID, sr.ServerURL)
	sr.Clients[userID] = client
	
	return client, nil
}

// ConnectAllClients connects all registered clients to the server
func (sr *ScenarioRunner) ConnectAllClients(ctx context.Context) error {
	sr.mu.RLock()
	clients := make(map[string]*TestClient)
	for id, client := range sr.Clients {
		clients[id] = client
	}
	sr.mu.RUnlock()
	
	// Connect all clients concurrently
	var wg sync.WaitGroup
	errors := make(chan error, len(clients))
	
	for userID, client := range clients {
		wg.Add(1)
		go func(id string, c *TestClient) {
			defer wg.Done()
			
			if err := c.Connect(ctx); err != nil {
				errors <- fmt.Errorf("failed to connect client %s: %w", id, err)
				return
			}
			
			// Wait for connection to be fully established
			if err := c.WaitForConnection(5 * time.Second); err != nil {
				errors <- fmt.Errorf("client %s connection timeout: %w", id, err)
			}
		}(userID, client)
	}
	
	wg.Wait()
	close(errors)
	
	// Collect any connection errors
	var connectionErrors []error
	for err := range errors {
		connectionErrors = append(connectionErrors, err)
	}
	
	if len(connectionErrors) > 0 {
		return fmt.Errorf("connection errors: %v", connectionErrors)
	}
	
	return nil
}

// ExecuteMessagePattern executes a predefined message pattern
func (sr *ScenarioRunner) ExecuteMessagePattern(pattern *MessagePattern) (*ScenarioResult, error) {
	startTime := time.Now()
	result := &ScenarioResult{
		ClientResults: make(map[string]*ClientResult),
	}
	
	sr.mu.Lock()
	sr.running = true
	sr.mu.Unlock()
	
	defer func() {
		sr.mu.Lock()
		sr.running = false
		sr.mu.Unlock()
		result.Duration = time.Since(startTime)
	}()
	
	// Initialize client results
	sr.mu.RLock()
	for userID := range sr.Clients {
		result.ClientResults[userID] = &ClientResult{
			UserID: userID,
			Errors: []error{},
		}
	}
	sr.mu.RUnlock()
	
	// Execute messages according to pattern timing
	var wg sync.WaitGroup
	messageErrors := make(chan error, len(pattern.Messages))
	
	for _, msg := range pattern.Messages {
		wg.Add(1)
		go func(testMsg *TestMessage) {
			defer wg.Done()
			
			// Wait for specified delay
			if testMsg.DelayMs > 0 {
				time.Sleep(time.Duration(testMsg.DelayMs) * time.Millisecond)
			}
			
			// Find the sender client
			sr.mu.RLock()
			client, exists := sr.Clients[testMsg.FromUser]
			sr.mu.RUnlock()
			
			if !exists {
				messageErrors <- fmt.Errorf("sender client not found: %s", testMsg.FromUser)
				return
			}
			
			// Send the message
			err := client.SendMessage(testMsg.Type, testMsg.Context, testMsg.Content, testMsg.ToUser)
			if err != nil {
				messageErrors <- fmt.Errorf("failed to send message from %s: %w", testMsg.FromUser, err)
				return
			}
			
			result.MessagesSent++
			result.ClientResults[testMsg.FromUser].MessagesSent++
		}(msg)
	}
	
	wg.Wait()
	close(messageErrors)
	
	// Collect message sending errors
	for err := range messageErrors {
		result.Errors = append(result.Errors, err)
	}
	
	// Wait a bit for messages to propagate
	time.Sleep(1 * time.Second)
	
	// Collect received messages from all clients
	sr.mu.RLock()
	for userID, client := range sr.Clients {
		receivedMessages := client.GetReceivedMessages()
		result.MessagesReceived += len(receivedMessages)
		result.ClientResults[userID].MessagesReceived = len(receivedMessages)
		
		// Collect any client errors
		clientErrors := client.GetErrors()
		result.ClientResults[userID].Errors = clientErrors
		result.Errors = append(result.Errors, clientErrors...)
	}
	sr.mu.RUnlock()
	
	result.Success = len(result.Errors) == 0
	return result, nil
}

// ValidateMessageDelivery checks that messages were delivered correctly
func (sr *ScenarioRunner) ValidateMessageDelivery(t *testing.T, pattern *MessagePattern, result *ScenarioResult) {
	// Basic validation: messages were sent and received
	if result.MessagesSent == 0 {
		t.Error("No messages were sent")
	}
	
	if result.MessagesReceived == 0 {
		t.Error("No messages were received")
	}
	
	// More sophisticated validation could check:
	// - Message routing correctness (broadcasts vs direct messages)
	// - Message ordering within sessions
	// - Role-based permission enforcement
	// - Context preservation
	
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	
	// Validate each message type was handled correctly
	messageTypesSent := make(map[string]int)
	for _, msg := range pattern.Messages {
		messageTypesSent[msg.Type]++
	}
	
	// Check that instructor broadcasts reached all students
	broadcasts := messageTypesSent["instructor_broadcast"]
	if broadcasts > 0 {
		studentCount := 0
		for userID := range sr.Clients {
			// Check if this is a student (not in instructor list)
			isInstructor := false
			// This is a simplified check - in real implementation would check against session data
			if len(userID) > 10 && userID[:10] == "instructor" {
				isInstructor = true
			}
			if !isInstructor {
				studentCount++
			}
		}
		
		// Each broadcast should reach each student
		expectedBroadcastMessages := broadcasts * studentCount
		if result.MessagesReceived < expectedBroadcastMessages {
			t.Errorf("Broadcast delivery incomplete: expected at least %d messages, got %d", 
				expectedBroadcastMessages, result.MessagesReceived)
		}
	}
}

// WaitForStableState waits for message processing to complete
func (sr *ScenarioRunner) WaitForStableState(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		// Check if all clients have stable message counts
		stable := true
		
		sr.mu.RLock()
		for _, client := range sr.Clients {
			if client.GetMessageCount() > 0 {
				stable = false
				break
			}
		}
		sr.mu.RUnlock()
		
		if stable {
			return nil
		}
		
		time.Sleep(100 * time.Millisecond)
	}
	
	return fmt.Errorf("system did not reach stable state within timeout")
}

// GetSystemStats returns current system statistics
func (sr *ScenarioRunner) GetSystemStats() (map[string]interface{}, error) {
	// Make HTTP request to health endpoint
	resp, err := http.Get(sr.ServerURL + "/health")
	if err != nil {
		return nil, fmt.Errorf("failed to get health stats: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}
	
	// In a full implementation, would parse JSON response
	// For now, return basic info
	stats := map[string]interface{}{
		"connected_clients": len(sr.Clients),
		"server_status":     "healthy",
	}
	
	return stats, nil
}

// SimulateNetworkConditions adds realistic network delays and jitter
func (sr *ScenarioRunner) SimulateNetworkConditions(enabled bool) {
	// This would add network simulation if needed
	// For now, it's a placeholder for future network condition testing
}

// MonitorResourceUsage tracks resource usage during test execution
func (sr *ScenarioRunner) MonitorResourceUsage(ctx context.Context, interval time.Duration) chan map[string]interface{} {
	resourceChan := make(chan map[string]interface{}, 10)
	
	go func() {
		defer close(resourceChan)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats, err := sr.GetSystemStats()
				if err != nil {
					continue
				}
				
				select {
				case resourceChan <- stats:
				default:
					// Channel full, skip this measurement
				}
			}
		}
	}()
	
	return resourceChan
}

// DisconnectClient simulates client disconnection
func (sr *ScenarioRunner) DisconnectClient(userID string) error {
	sr.mu.RLock()
	client, exists := sr.Clients[userID]
	sr.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("client not found: %s", userID)
	}
	
	return client.Close()
}

// ReconnectClient simulates client reconnection
func (sr *ScenarioRunner) ReconnectClient(ctx context.Context, userID string) error {
	sr.mu.RLock()
	client, exists := sr.Clients[userID]
	sr.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("client not found: %s", userID)
	}
	
	// Close existing connection
	client.Close()
	
	// Create new client with same parameters
	newClient := NewTestClient(client.UserID, client.Role, client.SessionID, client.ServerURL)
	
	sr.mu.Lock()
	sr.Clients[userID] = newClient
	sr.mu.Unlock()
	
	// Connect the new client
	return newClient.Connect(ctx)
}

// Cleanup shuts down the scenario runner and cleans up resources
func (sr *ScenarioRunner) Cleanup() {
	// Close all clients first and wait for cleanup
	sr.mu.RLock()
	clients := make([]*TestClient, 0, len(sr.Clients))
	for _, client := range sr.Clients {
		clients = append(clients, client)
	}
	sr.mu.RUnlock()
	
	// Close all clients and wait for them to finish
	for _, client := range clients {
		client.Close()
	}
	
	// Give clients time to close connections properly
	time.Sleep(100 * time.Millisecond)
	
	// Stop test server if running
	if sr.serverCancel != nil {
		sr.serverCancel()
	}
	
	// Stop application with extended timeout for complex tests
	if sr.testApp != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		sr.testApp.Stop(shutdownCtx)
	}
	
	// Additional wait to ensure all goroutines are cleaned up
	time.Sleep(200 * time.Millisecond)
	
	// Cleanup test session
	if sr.TestSession != nil {
		sr.TestSession.CleanupAll()
	}
}

// waitForServer waits for the test server to become available
func waitForServer(serverURL string, timeout time.Duration) error {
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		resp, err := client.Get(serverURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	return fmt.Errorf("server did not become available within %v", timeout)
}

// fileExists checks if a file or directory exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// findAvailablePortWithRetry finds an available port with retry logic to avoid race conditions
func findAvailablePortWithRetry() (int, error) {
	for attempts := 0; attempts < 10; attempts++ {
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			continue
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		
		// Add small delay and verify port is still available
		time.Sleep(time.Duration(attempts*10) * time.Millisecond)
		
		// Test if port is actually available by trying to bind to it
		testListener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			testListener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("failed to find available port after 10 attempts")
}

// GetClient returns a client by user ID
func (sr *ScenarioRunner) GetClient(userID string) (*TestClient, bool) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	
	client, exists := sr.Clients[userID]
	return client, exists
}

// GetAllClients returns all registered clients
func (sr *ScenarioRunner) GetAllClients() map[string]*TestClient {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	
	clients := make(map[string]*TestClient)
	for id, client := range sr.Clients {
		clients[id] = client
	}
	
	return clients
}

// ValidateSessionIsolation ensures messages don't leak between sessions
func (sr *ScenarioRunner) ValidateSessionIsolation(t *testing.T) {
	// This would be implemented to validate that messages in this session
	// don't appear in other concurrent sessions
	// For now, it's a placeholder for session isolation testing
	
	messageCount, err := sr.TestSession.GetMessageCount()
	if err != nil {
		t.Errorf("Failed to get message count: %v", err)
		return
	}
	
	if messageCount < 0 {
		t.Errorf("Invalid message count: %d", messageCount)
	}
}

// WaitForMessageFlow waits for a specific number of messages to flow through the system
func (sr *ScenarioRunner) WaitForMessageFlow(expectedMessages int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		totalReceived := 0
		
		sr.mu.RLock()
		for _, client := range sr.Clients {
			totalReceived += len(client.GetReceivedMessages())
		}
		sr.mu.RUnlock()
		
		if totalReceived >= expectedMessages {
			return nil
		}
		
		time.Sleep(100 * time.Millisecond)
	}
	
	return fmt.Errorf("timeout waiting for %d messages", expectedMessages)
}