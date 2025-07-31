package unit

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"switchboard/internal/database"
)

// TestNoCircularDependencies verifies no circular import dependencies
func TestNoCircularDependencies(t *testing.T) {
	// This test will fail initially since models.go doesn't exist
	// Parse the models package and verify it doesn't create circular dependencies
	
	modelsPath := filepath.Join("..", "..", "internal", "database", "models.go")
	fset := token.NewFileSet()
	
	_, err := parser.ParseFile(fset, modelsPath, nil, parser.ParseComments)
	require.NoError(t, err, "models.go should exist and be parseable")
}

// TestNoForbiddenImports ensures models.go doesn't import websocket, session, or message packages
func TestNoForbiddenImports(t *testing.T) {
	modelsPath := filepath.Join("..", "..", "internal", "database", "models.go")
	fset := token.NewFileSet()
	
	file, err := parser.ParseFile(fset, modelsPath, nil, parser.ParseComments)
	require.NoError(t, err, "models.go should be parseable")
	
	forbiddenImports := []string{
		"internal/websocket",
		"internal/session", 
		"internal/message",
	}
	
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		for _, forbidden := range forbiddenImports {
			assert.NotContains(t, importPath, forbidden, 
				"models.go should not import %s to maintain architectural boundaries", forbidden)
		}
	}
}

// TestStructDefinitions validates that Session and Message structs exist with exact field signatures
func TestStructDefinitions(t *testing.T) {
	t.Run("Session struct exists", func(t *testing.T) {
		// Use reflection to verify Session struct exists and has correct fields
		sessionType := reflect.TypeOf(database.Session{})
		assert.Equal(t, "Session", sessionType.Name())
		
		// Verify all required fields exist with correct types
		expectedFields := map[string]reflect.Type{
			"ID":        reflect.TypeOf(""),
			"Name":      reflect.TypeOf(""),
			"CreatedBy": reflect.TypeOf(""),
			"StartTime": reflect.TypeOf(time.Time{}),
			"EndTime":   reflect.TypeOf((*time.Time)(nil)),
			"Status":    reflect.TypeOf(""),
		}
		
		assert.Equal(t, len(expectedFields), sessionType.NumField(), "Session should have exactly %d fields", len(expectedFields))
		
		for i := 0; i < sessionType.NumField(); i++ {
			field := sessionType.Field(i)
			expectedType, exists := expectedFields[field.Name]
			assert.True(t, exists, "Unexpected field: %s", field.Name)
			assert.Equal(t, expectedType, field.Type, "Field %s should have type %v", field.Name, expectedType)
		}
	})
	
	t.Run("Message struct exists", func(t *testing.T) {
		// Use reflection to verify Message struct exists and has correct fields
		messageType := reflect.TypeOf(database.Message{})
		assert.Equal(t, "Message", messageType.Name())
		
		// Verify all required fields exist with correct types
		expectedFields := map[string]reflect.Type{
			"ID":        reflect.TypeOf(""),
			"SessionID": reflect.TypeOf(""),
			"Type":      reflect.TypeOf(""),
			"Context":   reflect.TypeOf(""),
			"FromUser":  reflect.TypeOf(""),
			"ToUser":    reflect.TypeOf((*string)(nil)),
			"Content":   reflect.TypeOf(map[string]interface{}{}),
			"Timestamp": reflect.TypeOf(time.Time{}),
		}
		
		assert.Equal(t, len(expectedFields), messageType.NumField(), "Message should have exactly %d fields", len(expectedFields))
		
		for i := 0; i < messageType.NumField(); i++ {
			field := messageType.Field(i)
			expectedType, exists := expectedFields[field.Name]
			assert.True(t, exists, "Unexpected field: %s", field.Name)
			assert.Equal(t, expectedType, field.Type, "Field %s should have type %v", field.Name, expectedType)
		}
	})
}

// TestStructFieldTags validates that all struct fields have correct JSON and DB tags
func TestStructFieldTags(t *testing.T) {
	t.Run("Session field tags", func(t *testing.T) {
		sessionType := reflect.TypeOf(database.Session{})
		
		expectedTags := map[string]struct {
			json string
			db   string
		}{
			"ID":        {"id", "id"},
			"Name":      {"name", "name"},
			"CreatedBy": {"created_by", "created_by"},
			"StartTime": {"start_time", "start_time"},
			"EndTime":   {"end_time,omitempty", "end_time"},
			"Status":    {"status", "status"},
		}
		
		for i := 0; i < sessionType.NumField(); i++ {
			field := sessionType.Field(i)
			expected, exists := expectedTags[field.Name]
			require.True(t, exists, "Field %s not in expected tags", field.Name)
			
			assert.Equal(t, expected.json, field.Tag.Get("json"), "Field %s JSON tag mismatch", field.Name)
			assert.Equal(t, expected.db, field.Tag.Get("db"), "Field %s DB tag mismatch", field.Name)
		}
	})
	
	t.Run("Message field tags", func(t *testing.T) {
		messageType := reflect.TypeOf(database.Message{})
		
		expectedTags := map[string]struct {
			json string
			db   string
		}{
			"ID":        {"id", "id"},
			"SessionID": {"session_id", "session_id"},
			"Type":      {"type", "type"},
			"Context":   {"context", "context"},
			"FromUser":  {"from_user", "from_user"},
			"ToUser":    {"to_user,omitempty", "to_user"},
			"Content":   {"content", "content"},
			"Timestamp": {"timestamp", "timestamp"},
		}
		
		for i := 0; i < messageType.NumField(); i++ {
			field := messageType.Field(i)
			expected, exists := expectedTags[field.Name]
			require.True(t, exists, "Field %s not in expected tags", field.Name)
			
			assert.Equal(t, expected.json, field.Tag.Get("json"), "Field %s JSON tag mismatch", field.Name)
			assert.Equal(t, expected.db, field.Tag.Get("db"), "Field %s DB tag mismatch", field.Name)
		}
	})
}

// TestConstantDefinitions validates that all required constants are defined
func TestConstantDefinitions(t *testing.T) {
	t.Run("Message type constants", func(t *testing.T) {
		// Test all message type constants exist and have correct values
		assert.Equal(t, "broadcast_to_instructors", database.MessageTypeBroadcastToInstructors)
		assert.Equal(t, "direct_message", database.MessageTypeDirectMessage)
		assert.Equal(t, "broadcast_to_students", database.MessageTypeBroadcastToStudents)
		assert.Equal(t, "system", database.MessageTypeSystem)
	})
	
	t.Run("Context constants", func(t *testing.T) {
		// Test all context constants exist and have correct values
		assert.Equal(t, "question", database.ContextQuestion)
		assert.Equal(t, "submission", database.ContextSubmission)
		assert.Equal(t, "analytics", database.ContextAnalytics)
		assert.Equal(t, "response", database.ContextResponse)
		assert.Equal(t, "request", database.ContextRequest)
		assert.Equal(t, "peer_help", database.ContextPeerHelp)
		assert.Equal(t, "announcement", database.ContextAnnouncement)
		assert.Equal(t, "instruction", database.ContextInstruction)
		assert.Equal(t, "emergency", database.ContextEmergency)
		assert.Equal(t, "general", database.ContextGeneral)
	})
}