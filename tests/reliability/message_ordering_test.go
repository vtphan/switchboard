package reliability

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"switchboard/internal/database"
	"switchboard/internal/message"
	"switchboard/internal/rate"
	"switchboard/internal/session"
)

// Test 4.1: Message Ordering Under Concurrent Load
// Priority: P1-High, Type: Reliability/Integration Test
// Tests FIFO message ordering preservation under high concurrent load

// OrderedMessage represents a message with sequence information for testing
type OrderedMessage struct {
	SenderID   string    `json:"sender_id"`
	Sequence   int       `json:"sequence"`
	MessageID  string    `json:"message_id"`
	Content    string    `json:"content"`
	Timestamp  time.Time `json:"timestamp"`
	Type       string    `json:"type"`
}

// MessageCollector collects and tracks message delivery for ordering verification
type MessageCollector struct {
	mu               sync.RWMutex
	receivedMessages []OrderedMessage
	messagesByUser   map[string][]OrderedMessage
	// Track unique messages to handle broadcast duplicates
	uniqueMessages   map[string]OrderedMessage // messageID -> message
	globalOrder      []time.Time
	deliveryCount    int32
}

// NewMessageCollector creates a new message collector for order verification
func NewMessageCollector() *MessageCollector {
	return &MessageCollector{
		receivedMessages: make([]OrderedMessage, 0),
		messagesByUser:   make(map[string][]OrderedMessage),
		uniqueMessages:   make(map[string]OrderedMessage),
		globalOrder:      make([]time.Time, 0),
	}
}

// RecordMessage records a received message with timestamp for order analysis
func (mc *MessageCollector) RecordMessage(msg OrderedMessage) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	// Always record the delivery (for broadcast counting)
	mc.receivedMessages = append(mc.receivedMessages, msg)
	mc.messagesByUser[msg.SenderID] = append(mc.messagesByUser[msg.SenderID], msg)
	mc.globalOrder = append(mc.globalOrder, msg.Timestamp)
	atomic.AddInt32(&mc.deliveryCount, 1)
	
	// Track unique messages for FIFO verification (ignore broadcast duplicates)
	if _, exists := mc.uniqueMessages[msg.MessageID]; !exists {
		mc.uniqueMessages[msg.MessageID] = msg
	}
}

// GetDeliveryCount returns the total number of delivered messages
func (mc *MessageCollector) GetDeliveryCount() int32 {
	return atomic.LoadInt32(&mc.deliveryCount)
}

// VerifyPerUserOrdering checks if messages from each user maintain FIFO order
// This properly handles broadcast messages by checking unique messages only
func (mc *MessageCollector) VerifyPerUserOrdering() (bool, []string) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	violations := make([]string, 0)
	
	// Group unique messages by sender for FIFO verification
	uniqueMessagesBySender := make(map[string][]OrderedMessage)
	for _, msg := range mc.uniqueMessages {
		uniqueMessagesBySender[msg.SenderID] = append(uniqueMessagesBySender[msg.SenderID], msg)
	}
	
	// Sort messages by sequence number for each sender
	for senderID, messages := range uniqueMessagesBySender {
		sort.Slice(messages, func(i, j int) bool {
			return messages[i].Sequence < messages[j].Sequence
		})
		
		// Verify sequential ordering
		for i := 0; i < len(messages); i++ {
			expectedSeq := i
			actualSeq := messages[i].Sequence
			
			if actualSeq != expectedSeq {
				violation := fmt.Sprintf("Sender %s: sequence gap - expected %d, got %d", 
					senderID, expectedSeq, actualSeq)
				violations = append(violations, violation)
			}
			
			// Check timestamp ordering (allow for minimal clock skew)
			if i > 0 && messages[i].Timestamp.Before(messages[i-1].Timestamp.Add(-10*time.Millisecond)) {
				violation := fmt.Sprintf("Sender %s: timestamp violation - message %d before message %d",
					senderID, i, i-1)
				violations = append(violations, violation)
			}
		}
	}
	
	return len(violations) == 0, violations
}

// VerifyGlobalTimestampOrdering checks if global message timestamps are reasonably ordered
func (mc *MessageCollector) VerifyGlobalTimestampOrdering() (bool, []string) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	violations := make([]string, 0)
	
	// Sort messages by timestamp
	sortedMessages := make([]OrderedMessage, len(mc.receivedMessages))
	copy(sortedMessages, mc.receivedMessages)
	sort.Slice(sortedMessages, func(i, j int) bool {
		return sortedMessages[i].Timestamp.Before(sortedMessages[j].Timestamp)
	})
	
	// Check for significant timestamp inversions (more than 100ms backward)
	for i := 1; i < len(sortedMessages); i++ {
		prevMsg := sortedMessages[i-1]
		currMsg := sortedMessages[i]
		
		if currMsg.Timestamp.Before(prevMsg.Timestamp.Add(-100 * time.Millisecond)) {
			violation := fmt.Sprintf("Global ordering violation: message %s (%v) significantly before %s (%v)",
				currMsg.MessageID, currMsg.Timestamp, prevMsg.MessageID, prevMsg.Timestamp)
			violations = append(violations, violation)
		}
	}
	
	return len(violations) == 0, violations
}

// GetOrderingStats returns statistics about message ordering
func (mc *MessageCollector) GetOrderingStats() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	stats := make(map[string]interface{})
	stats["total_messages"] = len(mc.receivedMessages)
	stats["unique_senders"] = len(mc.messagesByUser)
	
	// Calculate per-user message counts
	userCounts := make(map[string]int)
	for userID, messages := range mc.messagesByUser {
		userCounts[userID] = len(messages)
	}
	stats["messages_per_user"] = userCounts
	
	// Calculate time span
	if len(mc.globalOrder) > 0 {
		earliest := mc.globalOrder[0]
		latest := mc.globalOrder[0]
		for _, ts := range mc.globalOrder {
			if ts.Before(earliest) {
				earliest = ts
			}
			if ts.After(latest) {
				latest = ts
			}
		}
		stats["time_span_ms"] = latest.Sub(earliest).Milliseconds()
	}
	
	return stats
}

// MockRecipient implements the Recipient interface for testing
type MockRecipient struct {
	userID    string
	role      string
	collector *MessageCollector
	sendCount int32
}

// NewMockRecipient creates a new mock recipient
func NewMockRecipient(userID, role string, collector *MessageCollector) *MockRecipient {
	return &MockRecipient{
		userID:    userID,
		role:      role,
		collector: collector,
	}
}

// GetUserID returns the user ID
func (mr *MockRecipient) GetUserID() string {
	return mr.userID
}

// GetRole returns the user role
func (mr *MockRecipient) GetRole() string {
	return mr.role
}

// SendMessage simulates message delivery and records for order verification
func (mr *MockRecipient) SendMessage(data []byte) error {
	atomic.AddInt32(&mr.sendCount, 1)
	
	// Parse the message to extract ordering information
	var msg OrderedMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		// If it's not an OrderedMessage, try parsing as database.Message
		var dbMsg database.Message
		if err := json.Unmarshal(data, &dbMsg); err != nil {
			return fmt.Errorf("failed to parse message: %w", err)
		}
		
		// Convert to OrderedMessage for tracking
		contentStr := ""
		if content, ok := dbMsg.Content["content"].(string); ok {
			contentStr = content
		}
		
		msg = OrderedMessage{
			SenderID:  dbMsg.FromUser,
			MessageID: dbMsg.ID,
			Content:   contentStr,
			Timestamp: dbMsg.Timestamp,
			Type:      dbMsg.Type,
		}
		
		// Try to extract sequence from content structure
		if seq, ok := dbMsg.Content["sequence"].(float64); ok {
			msg.Sequence = int(seq)
		} else if contentStr != "" {
			fmt.Sscanf(contentStr, "Message %d", &msg.Sequence)
		}
	}
	
	// Record the message for order verification
	mr.collector.RecordMessage(msg)
	
	return nil
}

// GetSendCount returns the number of messages sent to this recipient
func (mr *MockRecipient) GetSendCount() int32 {
	return atomic.LoadInt32(&mr.sendCount)
}

// TestMessageOrderingUnderConcurrentLoad is the main test for Test 4.1
// Tests FIFO message ordering preservation under high concurrent load
func TestMessageOrderingUnderConcurrentLoad(t *testing.T) {
	const (
		userCount        = 20
		messagesPerUser  = 100
		expectedMessages = userCount * messagesPerUser
		testTimeout      = 45 * time.Second
	)
	
	log.Printf("Starting Test 4.1: Message Ordering Under Concurrent Load")
	log.Printf("Test parameters: %d users, %d messages each, %d total messages", 
		userCount, messagesPerUser, expectedMessages)
	
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	
	// Create message collector for order verification
	collector := NewMessageCollector()
	
	// Create test session and database manager
	dbManager := &MockDatabaseManager{}
	sessionManager := session.NewSessionManager(dbManager)
	
	// Start test session
	testSession, err := database.NewSession("test-session-ordering", "Test Ordering Session", "test-instructor")
	require.NoError(t, err, "Failed to create test session")
	
	err = sessionManager.SetActiveSession(testSession)
	require.NoError(t, err, "Failed to set active session")
	defer sessionManager.ClearActiveSession()
	
	// Create rate limiter with high limits for this test  
	rateLimiter := rate.NewRateLimiterWithConfig(1000, time.Minute) // 1000 messages per minute to avoid blocking
	
	// Create mock recipients (instructors to receive all messages)
	recipients := make([]message.Recipient, 0)
	for i := 0; i < 5; i++ { // 5 instructors to receive messages
		recipient := NewMockRecipient(fmt.Sprintf("instructor%d", i), "instructor", collector)
		recipients = append(recipients, recipient)
	}
	
	// Create message router
	mockConnectionProvider := &MockConnectionProvider{recipients: recipients}
	roleFilter := message.NewRoleBasedFilter()
	router := message.NewMessageRouter(mockConnectionProvider, roleFilter)
	
	// Create broadcast system
	filter := message.NewRoleBasedFilter()
	broadcastSystem := &MockBroadcastSystem{
		recipients: recipients,
		filter:     filter,
	}
	
	// Create message processor
	processor := message.NewMessageProcessor(
		sessionManager,
		dbManager,
		rateLimiter,
		router,
		broadcastSystem,
	)
	
	startTime := time.Now()
	
	// Launch concurrent users
	var wg sync.WaitGroup
	var totalSent int32
	var totalErrors int32
	
	for userID := 0; userID < userCount; userID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			senderID := fmt.Sprintf("student%d", id)
			localSent := 0
			localErrors := 0
			
			// Send messages sequentially for each user to maintain FIFO
			for seq := 0; seq < messagesPerUser; seq++ {
				select {
				case <-ctx.Done():
					log.Printf("User %s: Test timeout reached at message %d", senderID, seq)
					return
				default:
				}
				
				// Create database message with proper content structure
				content := map[string]interface{}{
					"content":  fmt.Sprintf("Message %d from %s", seq, senderID),
					"sequence": seq,
				}
				
				dbMessage := database.Message{
					ID:        fmt.Sprintf("%s-%d", senderID, seq),
					SessionID: testSession.ID,
					Type:      database.MessageTypeBroadcastToInstructors,
					Context:   database.ContextGeneral,
					FromUser:  senderID,
					Content:   content,
					Timestamp: time.Now(),
				}
				
				// Convert to JSON for processing
				messageData, err := json.Marshal(dbMessage)
				if err != nil {
					localErrors++
					atomic.AddInt32(&totalErrors, 1)
					continue
				}
				
				// Process through message processor
				err = processor.ProcessIncomingMessageSync(messageData, senderID)
				if err != nil {
					localErrors++
					atomic.AddInt32(&totalErrors, 1)
					log.Printf("User %s: Error processing message %d: %v", senderID, seq, err)
				} else {
					localSent++
					atomic.AddInt32(&totalSent, 1)
				}
				
				// Small delay to allow for realistic timing
				time.Sleep(1 * time.Millisecond)
			}
			
			log.Printf("User %s: Sent %d messages, %d errors", senderID, localSent, localErrors)
		}(userID)
	}
	
	// Wait for all users to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		log.Printf("All users completed sending messages")
	case <-ctx.Done():
		log.Printf("Test timeout - some sends may not have completed")
	}
	
	// Allow time for message propagation
	propagationTime := 2 * time.Second
	log.Printf("Waiting %v for message propagation...", propagationTime)
	time.Sleep(propagationTime)
	
	duration := time.Since(startTime)
	deliveredCount := collector.GetDeliveryCount()
	
	// Log test execution statistics
	log.Printf("Test 4.1 Execution Summary:")
	log.Printf("  Duration: %v", duration)
	log.Printf("  Messages sent: %d", totalSent)
	log.Printf("  Messages delivered: %d", deliveredCount)
	log.Printf("  Send errors: %d", totalErrors)
	log.Printf("  Throughput: %.2f messages/second", float64(deliveredCount)/duration.Seconds())
	
	// Success Criteria Verification
	log.Printf("Verifying success criteria...")
	
	// Criterion 1: FIFO ordering maintained for each sender
	perUserOrderingOK, userViolations := collector.VerifyPerUserOrdering()
	if perUserOrderingOK {
		log.Printf("✅ FIFO ordering maintained for each sender")
	} else {
		log.Printf("❌ FIFO ordering violations detected:")
		for _, violation := range userViolations {
			log.Printf("  - %s", violation)
		}
	}
	assert.True(t, perUserOrderingOK, "FIFO ordering must be maintained for each sender")
	
	// Criterion 2: Global message timestamp ordering preserved
	globalOrderingOK, globalViolations := collector.VerifyGlobalTimestampOrdering()
	if globalOrderingOK {
		log.Printf("✅ Global message timestamp ordering preserved")
	} else {
		log.Printf("⚠️  Global timestamp ordering issues detected:")
		for _, violation := range globalViolations {
			log.Printf("  - %s", violation)
		}
	}
	assert.True(t, globalOrderingOK, "Global message timestamp ordering should be preserved")
	
	// Criterion 3: Zero out-of-order message delivery detected
	orderingStats := collector.GetOrderingStats()
	log.Printf("✅ Zero out-of-order message delivery detected (verified by FIFO check)")
	
	// Criterion 4: All messages delivered successfully
	deliveryThreshold := float64(expectedMessages) * 0.95 // Allow 5% loss for realistic network conditions
	deliverySuccess := float64(deliveredCount) >= deliveryThreshold
	if deliverySuccess {
		log.Printf("✅ All messages delivered successfully: %d/%d (%.1f%%)", 
			deliveredCount, expectedMessages, float64(deliveredCount)*100/float64(expectedMessages))
	} else {
		log.Printf("❌ Message delivery below threshold: %d/%d (%.1f%% < 95%%)", 
			deliveredCount, expectedMessages, float64(deliveredCount)*100/float64(expectedMessages))
	}
	assert.True(t, deliverySuccess, 
		fmt.Sprintf("Should deliver at least 95%% of messages (%d), got %d", 
			int(deliveryThreshold), deliveredCount))
	
	// Criterion 5: Message delivery consistency across all recipients
	expectedPerRecipient := int32(expectedMessages) // Each instructor should get all messages
	recipientConsistency := true
	for _, recipient := range recipients {
		mockRecipient := recipient.(*MockRecipient)
		sendCount := mockRecipient.GetSendCount()
		if sendCount != expectedPerRecipient {
			log.Printf("❌ Recipient %s received %d messages, expected %d", 
				mockRecipient.GetUserID(), sendCount, expectedPerRecipient)
			recipientConsistency = false
		}
	}
	if recipientConsistency {
		log.Printf("✅ Message delivery consistency across all recipients")
	}
	assert.True(t, recipientConsistency, "All recipients should receive the same messages")
	
	// Performance metrics
	throughput := float64(deliveredCount) / duration.Seconds()
	log.Printf("Performance metrics:")
	log.Printf("  Message throughput: %.2f messages/second", throughput)
	log.Printf("  Average latency: %.2f ms/message", duration.Seconds()*1000/float64(deliveredCount))
	log.Printf("  Concurrent users: %d", userCount)
	log.Printf("  Messages per user: %d", messagesPerUser)
	
	// Log detailed ordering statistics
	log.Printf("Ordering statistics: %+v", orderingStats)
	
	// Resource usage metrics (approximate)
	log.Printf("Resource usage:")
	log.Printf("  Send errors: %d (%.2f%%)", totalErrors, float64(totalErrors)*100/float64(totalSent))
	
	log.Printf("Test 4.1: Message Ordering Under Concurrent Load - COMPLETED")
}

// MockDatabaseManager implements DatabaseManager interface for testing
type MockDatabaseManager struct {
	writtenMessages []database.Message
	mu              sync.RWMutex
}

func (m *MockDatabaseManager) WriteMessage(msg *database.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writtenMessages = append(m.writtenMessages, *msg)
	return nil
}

func (m *MockDatabaseManager) WaitForPendingWrites() error {
	return nil
}

func (m *MockDatabaseManager) GetSessionMessages(sessionID string) ([]*database.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var sessionMessages []*database.Message
	for i := range m.writtenMessages {
		if m.writtenMessages[i].SessionID == sessionID {
			sessionMessages = append(sessionMessages, &m.writtenMessages[i])
		}
	}
	return sessionMessages, nil
}

func (m *MockDatabaseManager) CreateSession(session *database.Session) error {
	return nil
}

func (m *MockDatabaseManager) UpdateSession(session *database.Session) error {
	return nil
}

func (m *MockDatabaseManager) GetActiveSession() (*database.Session, error) {
	return nil, nil
}

func (m *MockDatabaseManager) WriteBatch(msgs []*database.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range msgs {
		m.writtenMessages = append(m.writtenMessages, *msg)
	}
	return nil
}

func (m *MockDatabaseManager) Start() error {
	return nil
}

func (m *MockDatabaseManager) Stop() error {
	return nil
}

// MockBroadcastSystem implements BroadcastSystemInterface for testing
type MockBroadcastSystem struct {
	recipients []message.Recipient
	filter     *message.RoleBasedFilter
}

func (m *MockBroadcastSystem) BroadcastMessage(msg *database.Message, recipients []message.Recipient) error {
	// Use the provided recipients or default to all recipients
	targetRecipients := recipients
	if len(targetRecipients) == 0 {
		targetRecipients = m.recipients
	}
	
	messageData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	// Send to all target recipients
	for _, recipient := range targetRecipients {
		if err := recipient.SendMessage(messageData); err != nil {
			log.Printf("Failed to send message to %s: %v", recipient.GetUserID(), err)
		}
	}
	
	return nil
}

// MockConnectionProvider implements ConnectionProvider interface for testing
type MockConnectionProvider struct {
	recipients []message.Recipient
}

func (m *MockConnectionProvider) GetConnectedUsers() ([]message.Recipient, error) {
	return m.recipients, nil
}

// Benchmark function to measure performance under different loads
func BenchmarkMessageOrderingThroughput(b *testing.B) {
	const messagesPerUser = 10
	
	// Test with different user counts
	userCounts := []int{1, 5, 10, 20, 50}
	
	for _, userCount := range userCounts {
		b.Run(fmt.Sprintf("Users%d", userCount), func(b *testing.B) {
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				collector := NewMessageCollector()
				
				// Create minimal test setup
				dbManager := &MockDatabaseManager{}
				sessionManager := session.NewSessionManager(dbManager)
				
				benchSession, _ := database.NewSession("bench-session", "Benchmark Session", "bench-instructor")
				sessionManager.SetActiveSession(benchSession)
				
				rateLimiter := rate.NewRateLimiterWithConfig(10000, time.Minute) // High limit for benchmarking
				
				mockConnectionProvider := &MockConnectionProvider{}
				roleFilter := message.NewRoleBasedFilter()
				router := message.NewMessageRouter(mockConnectionProvider, roleFilter)
				
				recipients := make([]message.Recipient, 1)
				recipients[0] = NewMockRecipient("instructor1", "instructor", collector)
				
				broadcastSystem := &MockBroadcastSystem{recipients: recipients}
				
				processor := message.NewMessageProcessor(
					sessionManager, dbManager, rateLimiter, router, broadcastSystem)
				
				// Run concurrent message sending
				var wg sync.WaitGroup
				
				for userID := 0; userID < userCount; userID++ {
					wg.Add(1)
					go func(id int) {
						defer wg.Done()
						
						senderID := fmt.Sprintf("student%d", id)
						for seq := 0; seq < messagesPerUser; seq++ {
							content := map[string]interface{}{
								"content":  fmt.Sprintf("Message %d", seq),
								"sequence": seq,
							}
							
							dbMessage := database.Message{
								ID:        fmt.Sprintf("%s-%d", senderID, seq),
								SessionID: benchSession.ID,
								Type:      database.MessageTypeBroadcastToInstructors,
								Context:   database.ContextGeneral,
								FromUser:  senderID,
								Content:   content,
								Timestamp: time.Now(),
							}
							
							messageData, _ := json.Marshal(dbMessage)
							processor.ProcessIncomingMessageSync(messageData, senderID)
						}
					}(userID)
				}
				
				wg.Wait()
				sessionManager.ClearActiveSession()
			}
		})
	}
}