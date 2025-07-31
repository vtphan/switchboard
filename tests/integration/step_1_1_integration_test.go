package integration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"

	"switchboard/internal/database"
)

// TestStep11_DatabaseIntegration tests that Session and Message structs integrate with SQLite database
func TestStep11_DatabaseIntegration(t *testing.T) {
	// This test verifies the integration contracts from planning/integration-graph.yaml
	// Specifically: "DatabaseManager can serialize/deserialize these structs to SQLite"
	
	t.Run("Session database integration", func(t *testing.T) {
		// Setup in-memory SQLite database
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()
		
		// Apply database schema (this will fail until models and migrations exist)
		schemaPath := "../../internal/database/migrations.sql"
		schema, err := os.ReadFile(schemaPath)
		assert.NoError(t, err, "Database migrations should exist")
		
		_, err = db.Exec(string(schema))
		assert.NoError(t, err, "Database schema should apply successfully")
		
		// Create a Session struct with all fields
		session := &database.Session{
			ID:        "test-session-001",
			Name:      "Test Session Integration",
			CreatedBy: "instructor123",
			StartTime: time.Now().Truncate(time.Second), // Truncate for DB precision
			Status:    database.SessionStatusActive,
		}
		
		// Test JSON serialization
		jsonData, err := json.Marshal(session)
		require.NoError(t, err)
		
		var deserializedSession database.Session
		err = json.Unmarshal(jsonData, &deserializedSession)
		require.NoError(t, err)
		
		assert.Equal(t, session.ID, deserializedSession.ID)
		assert.Equal(t, session.Name, deserializedSession.Name)
		assert.Equal(t, session.CreatedBy, deserializedSession.CreatedBy)
		assert.Equal(t, session.Status, deserializedSession.Status)
		
		// Test database insertion and retrieval
		_, err = db.Exec(`INSERT INTO sessions (id, name, created_by, start_time, status) 
			VALUES (?, ?, ?, ?, ?)`,
			session.ID, session.Name, session.CreatedBy, session.StartTime, session.Status)
		require.NoError(t, err)
		
		// Query back from database
		var retrievedSession database.Session
		row := db.QueryRow(`SELECT id, name, created_by, start_time, end_time, status FROM sessions WHERE id = ?`, session.ID)
		err = row.Scan(&retrievedSession.ID, &retrievedSession.Name, &retrievedSession.CreatedBy, 
			&retrievedSession.StartTime, &retrievedSession.EndTime, &retrievedSession.Status)
		require.NoError(t, err)
		
		assert.Equal(t, session.ID, retrievedSession.ID)
		assert.Equal(t, session.Name, retrievedSession.Name)
		assert.Equal(t, session.CreatedBy, retrievedSession.CreatedBy)
		assert.Equal(t, session.Status, retrievedSession.Status)
		assert.Nil(t, retrievedSession.EndTime, "EndTime should be null for active session")
	})
	
	t.Run("Message database integration", func(t *testing.T) {
		// Setup in-memory SQLite database  
		db, err := sql.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer func() { _ = db.Close() }()
		
		// Apply database schema
		schemaPath := "../../internal/database/migrations.sql"
		schema, err := os.ReadFile(schemaPath)
		assert.NoError(t, err, "Database migrations should exist")
		
		_, err = db.Exec(string(schema))
		assert.NoError(t, err, "Database schema should apply successfully")
		
		// Test all 3 message types
		messageTypes := []string{
			database.MessageTypeBroadcastToInstructors,
			database.MessageTypeDirectMessage,
			database.MessageTypeBroadcastToStudents,
		}
		
		for _, msgType := range messageTypes {
			t.Run(fmt.Sprintf("Message type %s", msgType), func(t *testing.T) {
				// Create Message struct with complex content
				content := map[string]interface{}{
					"text": "Test message content",
					"metadata": map[string]interface{}{
						"priority": "high",
						"tags": []string{"urgent", "test"},
					},
				}
				
				msg := &database.Message{
					ID:        fmt.Sprintf("msg-%s-001", msgType),
					SessionID: "test-session-001",
					Type:      msgType,
					Context:   database.ContextGeneral,
					FromUser:  "user123",
					Content:   content,
					Timestamp: time.Now().Truncate(time.Second),
				}
				
				// Test nullable ToUser field
				if msgType == database.MessageTypeDirectMessage {
					toUser := "recipient123"
					msg.ToUser = &toUser
				}
				
				// Test JSON serialization
				jsonData, err := json.Marshal(msg)
				require.NoError(t, err)
				
				var deserializedMessage database.Message
				err = json.Unmarshal(jsonData, &deserializedMessage)
				require.NoError(t, err)
				
				assert.Equal(t, msg.Type, deserializedMessage.Type)
				assert.Equal(t, msg.Content, deserializedMessage.Content)
				
				// Test database insertion with JSON content
				contentJSON, err := json.Marshal(msg.Content)
				require.NoError(t, err)
				
				_, err = db.Exec(`INSERT INTO messages (id, session_id, type, context, from_user, to_user, content, timestamp) 
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					msg.ID, msg.SessionID, msg.Type, msg.Context, msg.FromUser, msg.ToUser, string(contentJSON), msg.Timestamp)
				require.NoError(t, err)
				
				// Query back and verify content reconstruction
				var retrievedMessage database.Message
				var contentStr string
				row := db.QueryRow(`SELECT id, session_id, type, context, from_user, to_user, content, timestamp FROM messages WHERE id = ?`, msg.ID)
				err = row.Scan(&retrievedMessage.ID, &retrievedMessage.SessionID, &retrievedMessage.Type, 
					&retrievedMessage.Context, &retrievedMessage.FromUser, &retrievedMessage.ToUser, &contentStr, &retrievedMessage.Timestamp)
				require.NoError(t, err)
				
				// Reconstruct content map from JSON
				err = json.Unmarshal([]byte(contentStr), &retrievedMessage.Content)
				require.NoError(t, err)
				
				assert.Equal(t, msg.ID, retrievedMessage.ID)
				assert.Equal(t, msg.Type, retrievedMessage.Type)
				assert.Equal(t, msg.Content, retrievedMessage.Content)
				assert.Equal(t, msg.ToUser, retrievedMessage.ToUser)
			})
		}
	})
}

// TestStep11_SessionManagerIntegration tests SessionManager contract compatibility
func TestStep11_SessionManagerIntegration(t *testing.T) {
	// This test verifies: "SessionManager can create Session structs with validation"
	
	t.Run("Session creation contract", func(t *testing.T) {
		// Test valid Session creation
		validSession := &database.Session{
			ID:        "valid-session-123",
			Name:      "Valid Session Name",
			CreatedBy: "instructor456",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		
		// Test validation passes for valid session
		err := validSession.Validate()
		assert.NoError(t, err, "Valid session should pass validation")
		
		// Test session lifecycle methods
		assert.True(t, validSession.IsActive(), "Session with active status should be active")
		
		// Test ending a session
		_ = validSession.End()
		assert.Equal(t, database.SessionStatusEnded, validSession.Status)
		assert.NotNil(t, validSession.EndTime)
		assert.False(t, validSession.IsActive(), "Ended session should not be active")
	})
	
	t.Run("Session validation contract", func(t *testing.T) {
		// Test name length validation constraints
		testCases := []struct {
			name        string
			sessionName string
			shouldFail  bool
		}{
			{"empty name", "", true},
			{"valid short name", "A", false},
			{"valid long name", string(make([]byte, 200)), false},
			{"too long name", string(make([]byte, 201)), true},
			{"normal name", "Regular Session Name", false},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				session := &database.Session{
					ID:        "test-validation",
					Name:      tc.sessionName,
					CreatedBy: "instructor",
					StartTime: time.Now(),
					Status:    database.SessionStatusActive,
				}
				
				err := session.Validate()
				if tc.shouldFail {
					assert.Error(t, err, "Validation should fail for %s", tc.name)
				} else {
					assert.NoError(t, err, "Validation should pass for %s", tc.name)
				}
			})
		}
		
		// Test status enum validation
		invalidStatusSession := &database.Session{
			ID:        "test-status",
			Name:      "Valid Name",
			CreatedBy: "instructor",
			StartTime: time.Now(),
			Status:    "invalid-status",
		}
		err := invalidStatusSession.Validate()
		assert.Error(t, err, "Invalid status should fail validation")
	})
}

// TestStep11_MessageProcessorIntegration tests MessageProcessor contract compatibility
func TestStep11_MessageProcessorIntegration(t *testing.T) {
	// This test verifies: "MessageProcessor can create Message structs with type validation"
	
	t.Run("Message creation contract", func(t *testing.T) {
		// Test creating Message structs with all 3 types
		messageTypes := []string{
			database.MessageTypeBroadcastToInstructors,
			database.MessageTypeDirectMessage,
			database.MessageTypeBroadcastToStudents,
		}
		
		for _, msgType := range messageTypes {
			msg := &database.Message{
				ID:        fmt.Sprintf("msg-processor-%s", msgType),
				SessionID: "test-session",
				Type:      msgType,
				Context:   database.ContextGeneral,
				FromUser:  "user123",
				Content:   map[string]interface{}{"text": "test message"},
				Timestamp: time.Now(),
			}
			
			// Test validation passes for all message types
			err := msg.Validate()
			assert.NoError(t, err, "Message with type %s should be valid", msgType)
			
			// Test JSON serialization preserves type
			jsonData, err := json.Marshal(msg)
			require.NoError(t, err)
			
			var deserializedMsg database.Message
			err = json.Unmarshal(jsonData, &deserializedMsg)
			require.NoError(t, err)
			assert.Equal(t, msgType, deserializedMsg.Type)
		}
	})
	
	t.Run("Message type validation contract", func(t *testing.T) {
		// Test all 3 message types work with validation
		messageTypes := []string{
			database.MessageTypeBroadcastToInstructors,
			database.MessageTypeDirectMessage,
			database.MessageTypeBroadcastToStudents,
		}
		
		for _, msgType := range messageTypes {
			t.Run(fmt.Sprintf("Message type %s", msgType), func(t *testing.T) {
				// Create Message with specific type
				msg := &database.Message{
					ID:        fmt.Sprintf("validation-%s", msgType),
					SessionID: "test-session",
					Type:      msgType,
					Context:   database.ContextGeneral,
					FromUser:  "user123",
					Content:   map[string]interface{}{"text": "type test"},
					Timestamp: time.Now(),
				}
				
				// Verify validation passes for this type
				err := msg.Validate()
				assert.NoError(t, err, "Message type %s should be valid", msgType)
				
				// Verify JSON serialization preserves type
				jsonData, err := json.Marshal(msg)
				require.NoError(t, err)
				
				var deserializedMsg database.Message
				err = json.Unmarshal(jsonData, &deserializedMsg)
				require.NoError(t, err)
				assert.Equal(t, msgType, deserializedMsg.Type)
			})
		}
	})
	
	t.Run("Message context validation contract", func(t *testing.T) {
		// Test all context types work with validation
		contexts := []string{
			"question", "submission", "analytics", "response", "request",
			"peer_help", "announcement", "instruction", "emergency", "general",
		}
		
		for _, context := range contexts {
			t.Run(fmt.Sprintf("Context %s", context), func(t *testing.T) {
				// Create Message with specific context
				msg := &database.Message{
					ID:        fmt.Sprintf("context-%s", context),
					SessionID: "test-session",
					Type:      database.MessageTypeBroadcastToStudents,
					Context:   context,
					FromUser:  "instructor",
					Content:   map[string]interface{}{"text": "context test"},
					Timestamp: time.Now(),
				}
				
				// Verify validation passes for this context
				err := msg.Validate()
				assert.NoError(t, err, "Message context %s should be valid", context)
				
				// Verify context preserved in serialization
				jsonData, err := json.Marshal(msg)
				require.NoError(t, err)
				
				var deserializedMsg database.Message
				err = json.Unmarshal(jsonData, &deserializedMsg)
				require.NoError(t, err)
				assert.Equal(t, context, deserializedMsg.Context)
			})
		}
	})
}

// TestStep11_CrossPhaseIntegration tests integration across multiple phases
func TestStep11_CrossPhaseIntegration(t *testing.T) {
	// This verifies the foundation is ready for future phases
	
	t.Run("JSON serialization for WebSocket", func(t *testing.T) {
		// Test Session JSON compatibility for WebSocket
		session := &database.Session{
			ID:        "websocket-session",
			Name:      "WebSocket Test Session",
			CreatedBy: "instructor",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		
		jsonData, err := json.Marshal(session)
		require.NoError(t, err)
		
		// Verify JSON is valid UTF-8 and contains no binary data
		assert.True(t, json.Valid(jsonData), "Session JSON should be valid")
		assert.NotContains(t, string(jsonData), "\x00", "JSON should not contain null bytes")
		
		// Test Message JSON compatibility for WebSocket
		msg := &database.Message{
			ID:        "websocket-msg",
			SessionID: session.ID,
			Type:      database.MessageTypeBroadcastToStudents,
			Context:   database.ContextAnnouncement,
			FromUser:  "instructor",
			Content:   map[string]interface{}{"text": "Hello via WebSocket!", "priority": 1},
			Timestamp: time.Now(),
		}
		
		msgJSON, err := json.Marshal(msg)
		require.NoError(t, err)
		
		assert.True(t, json.Valid(msgJSON), "Message JSON should be valid")
		assert.NotContains(t, string(msgJSON), "\x00", "JSON should not contain null bytes")
	})
	
	t.Run("Database manager interface readiness", func(t *testing.T) {
		// Test that Session and Message structs are compatible with DatabaseManager interface
		// This validates the contracts needed for Phase 2+ integration
		
		// Test Session struct can be used as DatabaseManager parameter
		session := &database.Session{
			ID:        "interface-test-session",
			Name:      "Interface Test",
			CreatedBy: "instructor",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		
		// Verify Session has all fields needed for DatabaseManager methods
		assert.NotEmpty(t, session.ID, "Session must have ID for database operations")
		assert.NotEmpty(t, session.Name, "Session must have Name for validation")
		assert.NotEmpty(t, session.Status, "Session must have Status for state management")
		
		// Test Message struct can be used as DatabaseManager parameter
		msg := &database.Message{
			ID:        "interface-test-msg",
			SessionID: session.ID,
			Type:      database.MessageTypeDirectMessage,
			Context:   database.ContextResponse,
			FromUser:  "user",
			Content:   map[string]interface{}{"text": "interface test"},
			Timestamp: time.Now(),
		}
		
		// Verify Message has all fields needed for DatabaseManager methods
		assert.NotEmpty(t, msg.ID, "Message must have ID for database operations")
		assert.NotEmpty(t, msg.SessionID, "Message must have SessionID for database relations")
		assert.NotEmpty(t, msg.Type, "Message must have Type for routing")
		assert.NotNil(t, msg.Content, "Message must have Content for serialization")
	})
}

// TestStep11_PerformanceIntegration tests performance characteristics needed by other phases
func TestStep11_PerformanceIntegration(t *testing.T) {
	t.Run("JSON marshaling performance", func(t *testing.T) {
		// Test JSON performance for real-time messaging requirements
		session := &database.Session{
			ID:        "perf-test-session",
			Name:      "Performance Test Session",
			CreatedBy: "instructor",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		
		// Test Session JSON marshal/unmarshal performance
		start := time.Now()
		for i := 0; i < 1000; i++ {
			jsonData, err := json.Marshal(session)
			require.NoError(t, err)
			
			var deserializedSession database.Session
			err = json.Unmarshal(jsonData, &deserializedSession)
			require.NoError(t, err)
		}
		duration := time.Since(start)
		avgTime := duration / 1000
		
		// Should be well under 1ms per operation for real-time messaging
		assert.Less(t, avgTime, 100*time.Microsecond, "Session JSON operations should be fast")
		t.Logf("Session JSON marshal/unmarshal average time: %v", avgTime)
		
		// Test Message with large content (approaching 64KB limit)
		largeContent := map[string]interface{}{
			"text":     string(make([]byte, 32*1024)), // 32KB text
			"metadata": map[string]interface{}{"large": true},
		}
		
		msg := &database.Message{
			ID:        "perf-test-msg",
			SessionID: session.ID,
			Type:      database.MessageTypeBroadcastToStudents,
			Context:   database.ContextGeneral,
			FromUser:  "instructor",
			Content:   largeContent,
			Timestamp: time.Now(),
		}
		
		// Test large message performance
		start = time.Now()
		jsonData, err := json.Marshal(msg)
		require.NoError(t, err)
		
		var deserializedMsg database.Message
		err = json.Unmarshal(jsonData, &deserializedMsg)
		require.NoError(t, err)
		duration = time.Since(start)
		
		// Large messages should still be reasonable for real-time use
		assert.Less(t, duration, 10*time.Millisecond, "Large message JSON operations should be reasonable")
		t.Logf("Large message JSON marshal/unmarshal time: %v", duration)
	})
	
	t.Run("Memory allocation patterns", func(t *testing.T) {
		// Test memory allocation patterns for high-frequency operations
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		
		// Perform many struct creations and JSON operations
		for i := 0; i < 10000; i++ {
			session := &database.Session{
				ID:        fmt.Sprintf("mem-test-%d", i),
				Name:      "Memory Test Session",
				CreatedBy: "instructor",
				StartTime: time.Now(),
				Status:    database.SessionStatusActive,
			}
			
			jsonData, _ := json.Marshal(session)
			var deserializedSession database.Session
			_ = json.Unmarshal(jsonData, &deserializedSession)
		}
		
		runtime.GC()
		runtime.ReadMemStats(&m2)
		
		// Check memory allocation patterns
		allocDiff := m2.TotalAlloc - m1.TotalAlloc
		avgAllocPerOp := allocDiff / 10000
		
		// Should not have excessive allocations per operation
		assert.Less(t, avgAllocPerOp, uint64(1024), "Should not allocate excessive memory per operation")
		t.Logf("Average allocation per session operation: %d bytes", avgAllocPerOp)
	})
}

// TestStep11_ConcurrencyIntegration tests thread safety for concurrent phases
func TestStep11_ConcurrencyIntegration(t *testing.T) {
	t.Run("Concurrent JSON operations", func(t *testing.T) {
		// Test JSON marshal/unmarshal safety under concurrent access
		session := &database.Session{
			ID:        "concurrent-session",
			Name:      "Concurrent Test Session",
			CreatedBy: "instructor",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		
		// Run concurrent JSON operations
		const numGoroutines = 100
		const opsPerGoroutine = 100
		
		var wg sync.WaitGroup
		results := make(chan bool, numGoroutines*opsPerGoroutine)
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				for j := 0; j < opsPerGoroutine; j++ {
					// Marshal session
					jsonData, err := json.Marshal(session)
					if err != nil {
						results <- false
						continue
					}
					
					// Unmarshal session
					var deserializedSession database.Session
					err = json.Unmarshal(jsonData, &deserializedSession)
					if err != nil {
						results <- false
						continue
					}
					
					// Verify consistency
					if deserializedSession.ID != session.ID || deserializedSession.Name != session.Name {
						results <- false
						continue
					}
					
					results <- true
				}
			}(i)
		}
		
		wg.Wait()
		close(results)
		
		// Verify all operations succeeded
		successCount := 0
		for result := range results {
			if result {
				successCount++
			}
		}
		
		expectedOps := numGoroutines * opsPerGoroutine
		assert.Equal(t, expectedOps, successCount, "All concurrent JSON operations should succeed")
	})
	
	t.Run("Concurrent validation", func(t *testing.T) {
		// Test validation method safety under concurrent access
		validSession := &database.Session{
			ID:        "valid-concurrent",
			Name:      "Valid Session",
			CreatedBy: "instructor",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		
		invalidSession := &database.Session{
			ID:        "invalid-concurrent",
			Name:      "", // Invalid empty name
			CreatedBy: "instructor",
			StartTime: time.Now(),
			Status:    database.SessionStatusActive,
		}
		
		// Run concurrent validation operations
		const numGoroutines = 50
		const opsPerGoroutine = 100
		
		var wg sync.WaitGroup
		validResults := make(chan bool, numGoroutines*opsPerGoroutine)
		invalidResults := make(chan bool, numGoroutines*opsPerGoroutine)
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				for j := 0; j < opsPerGoroutine; j++ {
					// Test valid session validation
					err := validSession.Validate()
					validResults <- (err == nil)
					
					// Test invalid session validation
					err = invalidSession.Validate()
					invalidResults <- (err != nil)
				}
			}()
		}
		
		wg.Wait()
		close(validResults)
		close(invalidResults)
		
		// Verify validation consistency
		validSuccessCount := 0
		for result := range validResults {
			if result {
				validSuccessCount++
			}
		}
		
		invalidSuccessCount := 0
		for result := range invalidResults {
			if result {
				invalidSuccessCount++
			}
		}
		
		expectedOps := numGoroutines * opsPerGoroutine
		assert.Equal(t, expectedOps, validSuccessCount, "Valid sessions should always pass validation")
		assert.Equal(t, expectedOps, invalidSuccessCount, "Invalid sessions should always fail validation")
	})
}