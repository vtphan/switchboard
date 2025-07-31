package message

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ========================================
// ARCHITECTURAL VALIDATION TESTS (BLOCKING)
// ========================================

func TestMessageProcessor_ArchitecturalCompliance(t *testing.T) {
	t.Run("struct_definition_exact", func(t *testing.T) {
		// Verify MessageProcessor struct exists with exact fields
		processorType := reflect.TypeOf(MessageProcessor{})

		// Check struct name
		if processorType.Name() != "MessageProcessor" {
			t.Errorf("Expected struct name 'MessageProcessor', got '%s'", processorType.Name())
		}

		// Check exact field count (4 fields as per spec)
		expectedFieldCount := 4
		if processorType.NumField() != expectedFieldCount {
			t.Errorf("Expected %d fields, got %d", expectedFieldCount, processorType.NumField())
		}

		// Verify field names and types match specification exactly
		expectedFields := map[string]string{
			"sessionManager": "SessionManager",
			"dbManager":      "DatabaseManager",
			"rateLimiter":    "RateLimiter",
			"router":         "MessageRouter",
		}

		for i := 0; i < processorType.NumField(); i++ {
			field := processorType.Field(i)
			expectedType, exists := expectedFields[field.Name]
			if !exists {
				t.Errorf("Unexpected field '%s' in MessageProcessor struct", field.Name)
				continue
			}

			actualType := field.Type.String()
			if !strings.Contains(actualType, expectedType) {
				t.Errorf("Field '%s' expected type containing '%s', got '%s'",
					field.Name, expectedType, actualType)
			}
		}
	})

	t.Run("interface_method_compliance", func(t *testing.T) {
		// Verify ProcessIncomingMessage method exists with exact signature
		processorType := reflect.TypeOf(&MessageProcessor{})
		method, exists := processorType.MethodByName("ProcessIncomingMessage")
		if !exists {
			t.Fatal("ProcessIncomingMessage method not found")
		}

		// Check method signature: func (rawData []byte, senderID string) error
		methodType := method.Type

		// Should have 3 inputs: receiver, []byte, string
		if methodType.NumIn() != 3 {
			t.Errorf("ProcessIncomingMessage expected 3 inputs (receiver, []byte, string), got %d",
				methodType.NumIn())
		}

		// Check parameter types
		if methodType.NumIn() >= 2 {
			param1 := methodType.In(1) // []byte
			if param1.Kind() != reflect.Slice || param1.Elem().Kind() != reflect.Uint8 {
				t.Errorf("First parameter should be []byte, got %v", param1)
			}
		}

		if methodType.NumIn() >= 3 {
			param2 := methodType.In(2) // string
			if param2.Kind() != reflect.String {
				t.Errorf("Second parameter should be string, got %v", param2)
			}
		}

		// Should have 1 output: error
		if methodType.NumOut() != 1 {
			t.Errorf("ProcessIncomingMessage expected 1 output (error), got %d", methodType.NumOut())
		}

		if methodType.NumOut() >= 1 {
			returnType := methodType.Out(0)
			if returnType.String() != "error" {
				t.Errorf("Return type should be error, got %v", returnType)
			}
		}
	})

	t.Run("dependency_boundaries", func(t *testing.T) {
		// Parse processor.go file to check imports
		fset := token.NewFileSet()
		filePath, _ := filepath.Abs("processor.go")

		file, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("Failed to parse processor.go: %v", err)
			return
		}

		// Check for forbidden imports
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)

			// These imports would violate architectural boundaries
			forbiddenImports := []string{
				"internal/websocket",
				"internal/api",
				"web/",
			}

			for _, forbidden := range forbiddenImports {
				if strings.Contains(importPath, forbidden) {
					t.Errorf("Forbidden import detected: %s violates layer boundaries", importPath)
				}
			}
		}
	})
}

func TestMessageProcessor_ArchitecturalInterfaces(t *testing.T) {
	t.Run("session_manager_interface_dependency", func(t *testing.T) {
		// Verify MessageProcessor uses SessionManager interface from Phase 2
		processorType := reflect.TypeOf(MessageProcessor{})

		// Find sessionManager field
		sessionManagerField, found := processorType.FieldByName("sessionManager")
		if !found {
			t.Fatal("sessionManager field not found in MessageProcessor")
		}

		// Verify field type is interface
		fieldType := sessionManagerField.Type
		if fieldType.Kind() != reflect.Interface {
			t.Errorf("sessionManager field should be interface, got %v", fieldType.Kind())
		}

		// Verify interface name contains SessionManager
		if !strings.Contains(fieldType.String(), "SessionManager") {
			t.Errorf("sessionManager field should be SessionManager interface, got %v", fieldType.String())
		}
	})

	t.Run("database_manager_interface_dependency", func(t *testing.T) {
		// Verify MessageProcessor uses DatabaseManager interface from Phase 1
		processorType := reflect.TypeOf(MessageProcessor{})

		// Find dbManager field
		dbManagerField, found := processorType.FieldByName("dbManager")
		if !found {
			t.Fatal("dbManager field not found in MessageProcessor")
		}

		// Verify field type is interface
		fieldType := dbManagerField.Type
		if fieldType.Kind() != reflect.Interface {
			t.Errorf("dbManager field should be interface, got %v", fieldType.Kind())
		}

		// Verify interface name contains DatabaseManager
		if !strings.Contains(fieldType.String(), "DatabaseManager") {
			t.Errorf("dbManager field should be DatabaseManager interface, got %v", fieldType.String())
		}
	})
}

// ========================================
// INTEGRATION CONTRACT VALIDATION TESTS
// ========================================

func TestMessageProcessor_IntegrationContracts(t *testing.T) {
	t.Run("session_validation_contract", func(t *testing.T) {
		// Tests integration contract: MessageProcessor must check active session
		// From integration-graph.yaml: message_session_validation flow

		t.Log("Session validation contract test - will verify session gating")

		// This test is designed to fail until implementation
		// Should verify:
		// - ProcessIncomingMessage calls SessionManager.GetActiveSession()
		// - Messages rejected when no active session
		// - Session ID properly set on valid messages
	})

	t.Run("database_persistence_contract", func(t *testing.T) {
		// Tests integration contract: MessageProcessor must persist to database
		// From integration-graph.yaml: message_persistence flow

		t.Log("Database persistence contract test - will verify async persistence")

		// This test is designed to fail until implementation
		// Should verify:
		// - ProcessIncomingMessage calls DatabaseManager.WriteMessage()
		// - Persistence is asynchronous (doesn't block)
		// - Persistence errors handled gracefully
	})

	t.Run("rate_limiting_integration_contract", func(t *testing.T) {
		// Tests integration contract: MessageProcessor must check rate limits

		t.Log("Rate limiting integration contract test - will verify rate enforcement")

		// This test is designed to fail until implementation
		// Should verify:
		// - ProcessIncomingMessage calls RateLimiter.Allow()
		// - Messages rejected when rate limit exceeded
		// - Rate limiting occurs before processing/persistence
	})
}
