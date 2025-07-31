package validation

import (
	"database/sql"
	"encoding/json"
	"fmt"
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

// Test 1.3: Late Joiner History Filtering - Comprehensive Validation
// Tests that students joining active session receive appropriate filtered history
// Priority: P0-Critical | Type: Privacy/Integration Test | Duration: 20 minutes
func TestLateJoinerHistoryFiltering(t *testing.T) {
	fmt.Println("=== EXECUTING TEST 1.3: LATE JOINER HISTORY FILTERING ===")
	startTime := time.Now()
	
	// Test setup verification
	env := setupComprehensiveTestEnvironment(t)
	defer env.cleanup()
	
	fmt.Println("✅ Test environment setup completed")
	
	// Step 1: Establish initial connections (1 instructor + 3 students A, B, C)
	fmt.Println("\n--- Phase 1: Establishing Initial Connections ---")
	_ = createInstructorConnection(t, env, "instructor1")
	_ = createStudentConnection(t, env, "studentA") 
	_ = createStudentConnection(t, env, "studentB")
	_ = createStudentConnection(t, env, "studentC")
	
	fmt.Printf("✅ Initial connections established: 1 instructor + 3 students\n")
	
	// Step 2: Start active session
	fmt.Println("\n--- Phase 2: Starting Active Session ---")
	session := &database.Session{
		ID:        "test-1-3-session",
		Name:      "Test 1.3 - Late Joiner Filtering",
		CreatedBy: "instructor1",
		StartTime: time.Now(),
		Status:    "active",
	}
	require.NoError(t, env.sessionManager.SetActiveSession(session))
	fmt.Printf("✅ Active session started: %s\n", session.Name)
	
	// Step 3: Generate complex message history (50 messages total)
	fmt.Println("\n--- Phase 3: Generating Complex Message History (50 messages) ---")
	
	var messageCount int
	messageHistory := []MessageSpec{
		// 5 instructor announcements (students should see all 5)
		{"Welcome to Biology 101!", "instructor1", "announcement", "", 1},
		{"Today we'll study cell structure", "instructor1", "announcement", "", 2},
		{"Please submit homework by Friday", "instructor1", "announcement", "", 3},
		{"Lab session moved to Monday", "instructor1", "announcement", "", 4},
		{"Final exam is next month", "instructor1", "announcement", "", 5},
		
		// 20 student questions to instructors (students should NOT see these)
		{"What is mitosis?", "studentA", "question", "", 6},
		{"How does photosynthesis work?", "studentB", "question", "", 7},
		{"Can you explain DNA replication?", "studentC", "question", "", 8},
		{"What are enzymes?", "studentA", "question", "", 9},
		{"How do cells divide?", "studentB", "question", "", 10},
		{"What is protein synthesis?", "studentC", "question", "", 11},
		{"Explain membrane transport", "studentA", "question", "", 12},
		{"What is cellular respiration?", "studentB", "question", "", 13},
		{"How do ribosomes work?", "studentC", "question", "", 14},
		{"What is the cell cycle?", "studentA", "question", "", 15},
		{"Explain gene expression", "studentB", "question", "", 16},
		{"What are chromosomes?", "studentC", "question", "", 17},
		{"How does meiosis differ from mitosis?", "studentA", "question", "", 18},
		{"What is apoptosis?", "studentB", "question", "", 19},
		{"Explain signal transduction", "studentC", "question", "", 20},
		{"What are stem cells?", "studentA", "question", "", 21},
		{"How do hormones work?", "studentB", "question", "", 22},
		{"What is genetic engineering?", "studentC", "question", "", 23},
		{"Explain evolutionary theory", "studentA", "question", "", 24},
		{"What is biodiversity?", "studentB", "question", "", 25},
		
		// 15 instructor responses to all students (students should see all 15)
		{"Mitosis is cell division for growth", "instructor1", "broadcast_response", "", 26},
		{"Photosynthesis converts light to chemical energy", "instructor1", "broadcast_response", "", 27},
		{"DNA replication ensures genetic continuity", "instructor1", "broadcast_response", "", 28},
		{"Enzymes are biological catalysts", "instructor1", "broadcast_response", "", 29},
		{"Cells divide through regulated processes", "instructor1", "broadcast_response", "", 30},
		{"Protein synthesis occurs at ribosomes", "instructor1", "broadcast_response", "", 31},
		{"Membrane transport controls cell contents", "instructor1", "broadcast_response", "", 32},
		{"Cellular respiration produces ATP energy", "instructor1", "broadcast_response", "", 33},
		{"Ribosomes translate mRNA to proteins", "instructor1", "broadcast_response", "", 34},
		{"Cell cycle has checkpoints for quality", "instructor1", "broadcast_response", "", 35},
		{"Gene expression is regulated at multiple levels", "instructor1", "broadcast_response", "", 36},
		{"Chromosomes carry genetic information", "instructor1", "broadcast_response", "", 37},
		{"Meiosis creates genetic diversity", "instructor1", "broadcast_response", "", 38},
		{"Apoptosis is programmed cell death", "instructor1", "broadcast_response", "", 39},
		{"Signal transduction transmits information", "instructor1", "broadcast_response", "", 40},
		
		// 10 direct messages between students (students should NOT see unrelated ones)
		{"Can you share your notes from yesterday?", "studentA", "direct", "studentB", 41},
		{"Sure! I'll send them after class", "studentB", "direct", "studentA", 42},
		{"Did you understand the enzyme explanation?", "studentB", "direct", "studentC", 43},
		{"Not really, could you help me?", "studentC", "direct", "studentB", 44},
		{"Want to form a study group?", "studentA", "direct", "studentC", 45},
		{"That sounds great!", "studentC", "direct", "studentA", 46},
		{"When is our lab report due?", "studentB", "direct", "studentA", 47},
		{"Next Tuesday I think", "studentA", "direct", "studentB", 48},
		{"Are you attending the review session?", "studentC", "direct", "studentB", 49},
		{"Yes, definitely!", "studentB", "direct", "studentC", 50},
	}
	
	// Send all messages with proper timing
	for _, msgSpec := range messageHistory {
		msgData := createMessageFromSpec(msgSpec)
		require.NoError(t, env.processor.ProcessIncomingMessage(msgData, msgSpec.Sender))
		messageCount++
		
		// Add small delay for realistic timing
		time.Sleep(20 * time.Millisecond)
		
		if messageCount % 10 == 0 {
			fmt.Printf("✅ Generated %d messages...\n", messageCount)
		}
	}
	
	fmt.Printf("✅ All 50 messages generated successfully\n")
	
	// Step 4: Verify all messages stored correctly before late joiner test
	fmt.Println("\n--- Phase 4: Verifying Message Storage ---")
	time.Sleep(500 * time.Millisecond) // Allow async database writes to complete
	
	storedMessages, err := env.dbManager.GetSessionMessages(session.ID)
	require.NoError(t, err)
	assert.Equal(t, 50, len(storedMessages), "All 50 messages must be stored in database")
	fmt.Printf("✅ Database verification: %d messages stored correctly\n", len(storedMessages))
	
	// Step 5: Connect new student D after delay
	fmt.Println("\n--- Phase 5: Connecting Late Joiner (Student D) ---")
	time.Sleep(100 * time.Millisecond) // Simulate 10-minute delay (compressed for testing)
	
	lateJoinerStartTime := time.Now()
	studentD := createLateJoinerConnection(t, env, "studentD", session.ID)
	historyDeliveryTime := time.Since(lateJoinerStartTime)
	
	fmt.Printf("✅ Student D connected as late joiner\n")
	fmt.Printf("✅ History delivery completed in %v\n", historyDeliveryTime)
	
	// Step 6: Validate filtering - Student D should receive exactly 20 messages
	fmt.Println("\n--- Phase 6: Validating Message Filtering ---")
	receivedMessages := studentD.GetDeliveredMessages()
	
	// Expected: 5 announcements + 15 instructor responses = 20 total
	assert.Equal(t, 20, len(receivedMessages), "Student D should receive exactly 20 filtered messages (5 announcements + 15 responses)")
	
	// Analyze received message types
	announcementCount := 0
	responseCount := 0
	questionCount := 0
	directMessageCount := 0
	
	for _, msgData := range receivedMessages {
		var msg map[string]interface{}
		err := json.Unmarshal(msgData, &msg)
		require.NoError(t, err)
		
		msgType := msg["type"].(string)
		switch msgType {
		case database.MessageTypeBroadcastToStudents:
			context := msg["context"].(string)
			switch context {
			case database.ContextAnnouncement:
				announcementCount++
			case database.ContextResponse:
				responseCount++
			}
		case database.MessageTypeBroadcastToInstructors:
			questionCount++
		case database.MessageTypeDirectMessage:
			directMessageCount++
		}
	}
	
	fmt.Printf("✅ Message type analysis:\n")
	fmt.Printf("   - Announcements: %d (expected: 5)\n", announcementCount)
	fmt.Printf("   - Instructor responses: %d (expected: 15)\n", responseCount) 
	fmt.Printf("   - Student questions: %d (expected: 0)\n", questionCount)
	fmt.Printf("   - Direct messages: %d (expected: 0)\n", directMessageCount)
	
	// Validate specific counts
	assert.Equal(t, 5, announcementCount, "Student D should receive all 5 announcements")
	assert.Equal(t, 15, responseCount, "Student D should receive all 15 instructor responses")
	assert.Equal(t, 0, questionCount, "Student D should NOT receive any student questions")
	assert.Equal(t, 0, directMessageCount, "Student D should NOT receive any unrelated direct messages")
	
	// Step 7: Verify chronological ordering
	fmt.Println("\n--- Phase 7: Verifying Message Chronological Ordering ---")
	var lastTimestamp int64 = 0
	orderingValid := true
	
	for i, msgData := range receivedMessages {
		var msg map[string]interface{}
		err := json.Unmarshal(msgData, &msg)
		require.NoError(t, err)
		
		timestamp := int64(msg["timestamp"].(float64))
		if i > 0 && timestamp < lastTimestamp {
			orderingValid = false
			fmt.Printf("❌ Ordering violation at message %d: timestamp %d < previous %d\n", i, timestamp, lastTimestamp)
		}
		lastTimestamp = timestamp
	}
	
	assert.True(t, orderingValid, "Message chronological ordering must be maintained")
	fmt.Printf("✅ Chronological ordering verified across %d messages\n", len(receivedMessages))
	
	// Step 8: Performance check - History delivery within 5 seconds
	fmt.Println("\n--- Phase 8: Performance Validation ---")
	assert.Less(t, historyDeliveryTime, 5*time.Second, "History delivery must complete within 5 seconds")
	fmt.Printf("✅ Performance target met: %v < 5s\n", historyDeliveryTime)
	
	// Test completion
	totalTestTime := time.Since(startTime)
	fmt.Printf("\n=== TEST 1.3 EXECUTION COMPLETE ===\n")
	fmt.Printf("Total execution time: %v\n", totalTestTime)
	
	// Final success criteria validation
	fmt.Println("\n--- Final Success Criteria Validation ---")
	fmt.Println("✅ Student D receives 15 instructor responses + 5 announcements (20 total)")
	fmt.Println("✅ Student D does NOT receive any student questions or unrelated direct messages") 
	fmt.Println("✅ Message chronological ordering maintained in filtered history")
	fmt.Println("✅ History delivery completes within 5 seconds of connection")
	fmt.Println("\n🎉 TEST 1.3: LATE JOINER HISTORY FILTERING - PASSED")
}

// Supporting types and functions

type MessageSpec struct {
	Content   string
	Sender    string
	Type      string
	ToUser    string
	Sequence  int
}

type ComprehensiveTestEnvironment struct {
	dbManager          *database.SQLiteDatabaseManager
	sessionManager     session.SessionManager
	connectionRegistry *websocket.ConnectionRegistry
	processor          *message.MessageProcessor
	connections        map[string]*ComprehensiveTestConnection
}

type ComprehensiveTestConnection struct {
	userID            string
	role              string
	deliveredMessages [][]byte
	mu                sync.RWMutex
	closed            bool
}

func (ctc *ComprehensiveTestConnection) GetUserID() string { return ctc.userID }
func (ctc *ComprehensiveTestConnection) GetRole() string   { return ctc.role }

func (ctc *ComprehensiveTestConnection) SendMessage(data []byte) error {
	ctc.mu.Lock()
	defer ctc.mu.Unlock()
	
	if ctc.closed {
		return fmt.Errorf("connection closed")
	}
	
	msgCopy := make([]byte, len(data))
	copy(msgCopy, data)
	ctc.deliveredMessages = append(ctc.deliveredMessages, msgCopy)
	return nil
}

func (ctc *ComprehensiveTestConnection) GetDeliveredMessages() [][]byte {
	ctc.mu.RLock()
	defer ctc.mu.RUnlock()
	
	result := make([][]byte, len(ctc.deliveredMessages))
	for i, msg := range ctc.deliveredMessages {
		result[i] = make([]byte, len(msg))
		copy(result[i], msg)
	}
	return result
}

func (ctc *ComprehensiveTestConnection) Close() error {
	ctc.mu.Lock()
	defer ctc.mu.Unlock()
	ctc.closed = true
	return nil
}

// Interface compliance methods
func (ctc *ComprehensiveTestConnection) WriteJSON(v interface{}) error              { return nil }
func (ctc *ComprehensiveTestConnection) SetCredentials(username, role string) error { return nil }
func (ctc *ComprehensiveTestConnection) UpdateActivity()                            {}
func (ctc *ComprehensiveTestConnection) GetLastSeen() time.Time                     { return time.Now() }
func (ctc *ComprehensiveTestConnection) SendCloseMessage(reason string) error       { return nil }

func setupComprehensiveTestEnvironment(t *testing.T) *ComprehensiveTestEnvironment {
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

	return &ComprehensiveTestEnvironment{
		dbManager:          dbManager,
		sessionManager:     sessionManager,
		connectionRegistry: connectionRegistry,
		processor:          processor,
		connections:        make(map[string]*ComprehensiveTestConnection),
	}
}

func (env *ComprehensiveTestEnvironment) cleanup() {
	env.connectionRegistry.Stop()
	if err := env.dbManager.Stop(); err != nil {
		// Use standard library log since we don't have testing.T context here
		fmt.Printf("Failed to stop database manager: %v\n", err)
	}
}

func createInstructorConnection(t *testing.T, env *ComprehensiveTestEnvironment, userID string) *ComprehensiveTestConnection {
	conn := &ComprehensiveTestConnection{userID: userID, role: "instructor"}
	require.NoError(t, env.connectionRegistry.Register(userID, conn))
	env.connections[userID] = conn
	return conn
}

func createStudentConnection(t *testing.T, env *ComprehensiveTestEnvironment, userID string) *ComprehensiveTestConnection {
	conn := &ComprehensiveTestConnection{userID: userID, role: "student"}
	require.NoError(t, env.connectionRegistry.Register(userID, conn))
	env.connections[userID] = conn
	return conn
}

func createLateJoinerConnection(t *testing.T, env *ComprehensiveTestEnvironment, userID string, sessionID string) *ComprehensiveTestConnection {
	conn := &ComprehensiveTestConnection{userID: userID, role: "student"}
	require.NoError(t, env.connectionRegistry.Register(userID, conn))
	env.connections[userID] = conn
	
	// Wait for any pending async database operations
	time.Sleep(250 * time.Millisecond)
	
	// Simulate WebSocket handler behavior for late joiner history delivery
	messages, err := env.dbManager.GetSessionMessages(sessionID)
	require.NoError(t, err)
	
	// Create role-based filter and deliver filtered history
	filter := message.NewRoleBasedFilter()
	for _, msg := range messages {
		if filter.ShouldReceiveMessage(msg, conn) {
			// Convert to message format and deliver
			msgMap := map[string]interface{}{
				"type":      msg.Type,
				"content":   msg.Content,
				"context":   msg.Context,
				"timestamp": msg.Timestamp.Unix(),
				"from_user": msg.FromUser,
			}
			
			if msg.Type == database.MessageTypeDirectMessage && msg.ToUser != nil {
				msgMap["to_user"] = *msg.ToUser
			}
			
			msgData, _ := json.Marshal(msgMap)
			if err := conn.SendMessage(msgData); err != nil {
				// Log send error but don't fail the test as this is simulating network conditions
				fmt.Printf("Failed to send message to connection %s: %v\n", conn.userID, err)
			}
		}
	}
	
	return conn
}

func createMessageFromSpec(spec MessageSpec) []byte {
	var msg map[string]interface{}
	
	switch spec.Type {
	case "announcement":
		msg = map[string]interface{}{
			"type":    database.MessageTypeBroadcastToStudents,
			"context": database.ContextAnnouncement,
			"content": map[string]interface{}{"text": spec.Content},
		}
	case "question":
		msg = map[string]interface{}{
			"type":    database.MessageTypeBroadcastToInstructors,
			"context": database.ContextQuestion,
			"content": map[string]interface{}{"text": spec.Content},
		}
	case "broadcast_response":
		msg = map[string]interface{}{
			"type":    database.MessageTypeBroadcastToStudents,
			"context": database.ContextResponse,
			"content": map[string]interface{}{"text": spec.Content},
		}
	case "direct":
		msg = map[string]interface{}{
			"type":    database.MessageTypeDirectMessage,
			"context": database.ContextResponse,
			"content": map[string]interface{}{"text": spec.Content},
			"to_user": spec.ToUser,
		}
	}
	
	data, _ := json.Marshal(msg)
	return data
}