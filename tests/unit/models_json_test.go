package unit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"switchboard/internal/database"
)

// TestSessionJSONMarshaling tests Session struct JSON marshaling behavior
func TestSessionJSONMarshaling(t *testing.T) {
	t.Run("Session with EndTime nil", func(t *testing.T) {
		// Test that EndTime field is omitted when nil due to omitempty tag
		session := database.Session{
			ID:        "test-session",
			Name:      "Test Session",
			CreatedBy: "instructor-1",
			StartTime: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			EndTime:   nil,
			Status:    database.SessionStatusActive,
		}
		
		jsonData, err := json.Marshal(session)
		require.NoError(t, err)
		
		// Verify EndTime is not present in JSON
		assert.NotContains(t, string(jsonData), "end_time")
		assert.Contains(t, string(jsonData), `"id":"test-session"`)
		assert.Contains(t, string(jsonData), `"status":"active"`)
	})
	
	t.Run("Session with EndTime set", func(t *testing.T) {
		// Test that EndTime field is included when set
		endTime := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)
		session := database.Session{
			ID:        "test-session",
			Name:      "Test Session",
			CreatedBy: "instructor-1",
			StartTime: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			EndTime:   &endTime,
			Status:    database.SessionStatusEnded,
		}
		
		jsonData, err := json.Marshal(session)
		require.NoError(t, err)
		
		// Verify EndTime is present in JSON
		assert.Contains(t, string(jsonData), "end_time")
		assert.Contains(t, string(jsonData), `"status":"ended"`)
	})
	
	t.Run("Time format compliance", func(t *testing.T) {
		// Test that time fields marshal to proper JSON format
		session := database.Session{
			ID:        "test-session",
			Name:      "Test Session",
			CreatedBy: "instructor-1",
			StartTime: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			Status:    database.SessionStatusActive,
		}
		
		jsonData, err := json.Marshal(session)
		require.NoError(t, err)
		
		// Verify time is in RFC3339 format
		assert.Contains(t, string(jsonData), `"start_time":"2024-01-01T10:00:00Z"`)
	})
}

// TestSessionJSONUnmarshaling tests Session struct JSON unmarshaling behavior
func TestSessionJSONUnmarshaling(t *testing.T) {
	t.Run("Valid JSON with all fields", func(t *testing.T) {
		jsonData := `{
			"id": "session-123",
			"name": "Test Session",
			"created_by": "instructor-1",
			"start_time": "2024-01-01T10:00:00Z",
			"end_time": "2024-01-01T11:00:00Z",
			"status": "active"
		}`
		
		var session database.Session
		err := json.Unmarshal([]byte(jsonData), &session)
		require.NoError(t, err)
		
		assert.Equal(t, "session-123", session.ID)
		assert.Equal(t, "Test Session", session.Name)
		assert.Equal(t, "instructor-1", session.CreatedBy)
		assert.Equal(t, "active", session.Status)
		assert.NotNil(t, session.EndTime)
		assert.Equal(t, time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC), *session.EndTime)
	})
	
	t.Run("JSON missing optional EndTime", func(t *testing.T) {
		jsonData := `{
			"id": "session-123",
			"name": "Test Session", 
			"created_by": "instructor-1",
			"start_time": "2024-01-01T10:00:00Z",
			"status": "active"
		}`
		
		var session database.Session
		err := json.Unmarshal([]byte(jsonData), &session)
		require.NoError(t, err)
		
		assert.Equal(t, "session-123", session.ID)
		assert.Nil(t, session.EndTime)
	})
	
	t.Run("Invalid time formats", func(t *testing.T) {
		jsonData := `{
			"id": "session-123",
			"name": "Test Session",
			"created_by": "instructor-1", 
			"start_time": "invalid-time",
			"status": "active"
		}`
		
		var session database.Session
		err := json.Unmarshal([]byte(jsonData), &session)
		assert.Error(t, err, "Should fail to unmarshal invalid time format")
	})
}

// TestMessageJSONMarshaling tests Message struct JSON marshaling behavior
func TestMessageJSONMarshaling(t *testing.T) {
	t.Run("Message with ToUser nil", func(t *testing.T) {
		// Test that ToUser field is omitted when nil due to omitempty tag
		message := database.Message{
			ID:        "msg-123",
			SessionID: "session-123",
			Type:      database.MessageTypeBroadcastToStudents,
			Context:   database.ContextGeneral,
			FromUser:  "instructor-1",
			ToUser:    nil,
			Content:   map[string]interface{}{"text": "Hello students"},
			Timestamp: time.Date(2024, 1, 1, 10, 5, 0, 0, time.UTC),
		}
		
		jsonData, err := json.Marshal(message)
		require.NoError(t, err)
		
		// Verify ToUser is not present in JSON
		assert.NotContains(t, string(jsonData), "to_user")
		assert.Contains(t, string(jsonData), `"type":"broadcast_to_students"`)
	})
	
	t.Run("Message with ToUser set", func(t *testing.T) {
		// Test that ToUser field is included when set
		toUser := "student-1"
		message := database.Message{
			ID:        "msg-123",
			SessionID: "session-123",
			Type:      database.MessageTypeDirectMessage,
			Context:   database.ContextGeneral,
			FromUser:  "instructor-1",
			ToUser:    &toUser,
			Content:   map[string]interface{}{"text": "Hello student"},
			Timestamp: time.Date(2024, 1, 1, 10, 5, 0, 0, time.UTC),
		}
		
		jsonData, err := json.Marshal(message)
		require.NoError(t, err)
		
		// Verify ToUser is present in JSON
		assert.Contains(t, string(jsonData), "to_user")
		assert.Contains(t, string(jsonData), `"to_user":"student-1"`)
	})
	
	t.Run("Complex Content map", func(t *testing.T) {
		// Test marshaling of map[string]interface{} with various types
		message := database.Message{
			ID:        "msg-123",
			SessionID: "session-123",
			Type:      database.MessageTypeBroadcastToStudents,
			Context:   database.ContextGeneral,
			FromUser:  "instructor-1",
			Content: map[string]interface{}{
				"text": "Hello students",
				"metadata": map[string]interface{}{
					"priority": "high",
					"count":    42,
					"enabled":  true,
				},
			},
			Timestamp: time.Date(2024, 1, 1, 10, 5, 0, 0, time.UTC),
		}
		
		jsonData, err := json.Marshal(message)
		require.NoError(t, err)
		
		// Verify complex content is marshaled correctly
		assert.Contains(t, string(jsonData), `"text":"Hello students"`)
		assert.Contains(t, string(jsonData), `"priority":"high"`)
		assert.Contains(t, string(jsonData), `"count":42`)
		assert.Contains(t, string(jsonData), `"enabled":true`)
	})
	
	t.Run("Empty Content map", func(t *testing.T) {
		// Test marshaling of empty content map
		message := database.Message{
			ID:        "msg-123",
			SessionID: "session-123",
			Type:      database.MessageTypeBroadcastToStudents,
			Context:   database.ContextGeneral,
			FromUser:  "instructor-1",
			Content:   map[string]interface{}{},
			Timestamp: time.Date(2024, 1, 1, 10, 5, 0, 0, time.UTC),
		}
		
		jsonData, err := json.Marshal(message)
		require.NoError(t, err)
		
		// Verify empty content becomes {}
		assert.Contains(t, string(jsonData), `"content":{}`)
	})
}

// TestMessageJSONUnmarshaling tests Message struct JSON unmarshaling behavior  
func TestMessageJSONUnmarshaling(t *testing.T) {
	t.Run("Valid JSON with complex content", func(t *testing.T) {
		jsonData := `{
			"id": "msg-123",
			"session_id": "session-123",
			"type": "broadcast_to_students",
			"context": "general",
			"from_user": "instructor-1",
			"to_user": "student-1",
			"content": {
				"text": "Hello students",
				"metadata": {
					"priority": "high",
					"count": 42
				}
			},
			"timestamp": "2024-01-01T10:05:00Z"
		}`
		
		var message database.Message
		err := json.Unmarshal([]byte(jsonData), &message)
		require.NoError(t, err)
		
		assert.Equal(t, "msg-123", message.ID)
		assert.Equal(t, "broadcast_to_students", message.Type)
		assert.NotNil(t, message.ToUser)
		assert.Equal(t, "student-1", *message.ToUser)
		
		// Check complex content
		assert.Equal(t, "Hello students", message.Content["text"])
		metadata, ok := message.Content["metadata"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "high", metadata["priority"])
		assert.Equal(t, float64(42), metadata["count"]) // JSON numbers are float64
	})
	
	t.Run("JSON missing optional ToUser", func(t *testing.T) {
		jsonData := `{
			"id": "msg-123",
			"session_id": "session-123", 
			"type": "broadcast_to_students",
			"context": "general",
			"from_user": "instructor-1",
			"content": {"text": "Hello everyone"},
			"timestamp": "2024-01-01T10:05:00Z"
		}`
		
		var message database.Message
		err := json.Unmarshal([]byte(jsonData), &message)
		require.NoError(t, err)
		
		assert.Equal(t, "msg-123", message.ID)
		assert.Nil(t, message.ToUser)
		assert.Equal(t, "Hello everyone", message.Content["text"])
	})
}

// TestJSONRoundTrip ensures marshal->unmarshal produces identical structs
func TestJSONRoundTrip(t *testing.T) {
	t.Run("Session round trip", func(t *testing.T) {
		// Test that Session marshaled to JSON and back produces identical struct
		original := database.Session{
			ID:        "test-session",
			Name:      "Test Session",
			CreatedBy: "instructor-1",
			StartTime: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			Status:    database.SessionStatusActive,
		}
		
		jsonData, err := json.Marshal(original)
		require.NoError(t, err)
		
		var reconstructed database.Session
		err = json.Unmarshal(jsonData, &reconstructed)
		require.NoError(t, err)
		
		assert.Equal(t, original, reconstructed)
	})
	
	t.Run("Message round trip", func(t *testing.T) {
		// Test that Message marshaled to JSON and back produces identical struct
		original := database.Message{
			ID:        "msg-123",
			SessionID: "session-123",
			Type:      database.MessageTypeBroadcastToStudents,
			Context:   database.ContextGeneral,
			FromUser:  "instructor-1",
			Content:   map[string]interface{}{"text": "Hello"},
			Timestamp: time.Date(2024, 1, 1, 10, 5, 0, 0, time.UTC),
		}
		
		jsonData, err := json.Marshal(original)
		require.NoError(t, err)
		
		var reconstructed database.Message
		err = json.Unmarshal(jsonData, &reconstructed)
		require.NoError(t, err)
		
		assert.Equal(t, original, reconstructed)
	})
	
	t.Run("Message with complex content round trip", func(t *testing.T) {
		// Test round trip with complex nested content
		toUser := "student-1"
		original := database.Message{
			ID:        "msg-123",
			SessionID: "session-123",
			Type:      database.MessageTypeDirectMessage,
			Context:   database.ContextQuestion,
			FromUser:  "instructor-1",
			ToUser:    &toUser,
			Content: map[string]interface{}{
				"text": "Complex message",
				"data": map[string]interface{}{
					"num":  42,
					"bool": true,
					"arr":  []interface{}{"a", "b", "c"},
				},
			},
			Timestamp: time.Date(2024, 1, 1, 10, 5, 0, 0, time.UTC),
		}
		
		jsonData, err := json.Marshal(original)
		require.NoError(t, err)
		
		var reconstructed database.Message
		err = json.Unmarshal(jsonData, &reconstructed)
		require.NoError(t, err)
		
		// Note: JSON round trip changes numeric types to float64 and arrays to []interface{}
		// So we need to compare the important fields manually
		assert.Equal(t, original.ID, reconstructed.ID)
		assert.Equal(t, original.Type, reconstructed.Type)
		assert.Equal(t, *original.ToUser, *reconstructed.ToUser)
		assert.Equal(t, original.Content["text"], reconstructed.Content["text"])
	})
}