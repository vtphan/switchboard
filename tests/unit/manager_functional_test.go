package unit

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"switchboard/internal/database"
)

// TestDatabaseManagerMethodCount validates exact number of interface methods
func TestDatabaseManagerMethodCount(t *testing.T) {
	// Use reflection to verify DatabaseManager interface has exactly 8 methods
	interfaceType := reflect.TypeOf((*database.DatabaseManager)(nil)).Elem()
	assert.Equal(t, reflect.Interface, interfaceType.Kind(), "DatabaseManager should be an interface")
	assert.Equal(t, 9, interfaceType.NumMethod(), "DatabaseManager should have exactly 9 methods")
}

// TestDatabaseManagerSessionMethods validates all session-related method signatures
func TestDatabaseManagerSessionMethods(t *testing.T) {
	interfaceType := reflect.TypeOf((*database.DatabaseManager)(nil)).Elem()
	
	// Test CreateSession method
	createMethod, found := interfaceType.MethodByName("CreateSession")
	assert.True(t, found, "CreateSession method should exist")
	assert.Equal(t, 1, createMethod.Type.NumIn(), "CreateSession should have 1 parameter (session)")
	assert.Equal(t, 1, createMethod.Type.NumOut(), "CreateSession should have 1 return value (error)")
	
	// Test UpdateSession method
	updateMethod, found := interfaceType.MethodByName("UpdateSession")
	assert.True(t, found, "UpdateSession method should exist")
	assert.Equal(t, 1, updateMethod.Type.NumIn(), "UpdateSession should have 1 parameter (session)")
	assert.Equal(t, 1, updateMethod.Type.NumOut(), "UpdateSession should have 1 return value (error)")
	
	// Test GetActiveSession method
	getMethod, found := interfaceType.MethodByName("GetActiveSession")
	assert.True(t, found, "GetActiveSession method should exist")
	assert.Equal(t, 0, getMethod.Type.NumIn(), "GetActiveSession should have 0 parameters")
	assert.Equal(t, 2, getMethod.Type.NumOut(), "GetActiveSession should have 2 return values (session, error)")
}

// TestDatabaseManagerMessageMethods validates all message-related method signatures
func TestDatabaseManagerMessageMethods(t *testing.T) {
	interfaceType := reflect.TypeOf((*database.DatabaseManager)(nil)).Elem()
	
	// Test WriteMessage method
	writeMethod, found := interfaceType.MethodByName("WriteMessage")
	assert.True(t, found, "WriteMessage method should exist")
	assert.Equal(t, 1, writeMethod.Type.NumIn(), "WriteMessage should have 1 parameter (message)")
	assert.Equal(t, 1, writeMethod.Type.NumOut(), "WriteMessage should have 1 return value (error)")
	
	// Test WriteBatch method
	batchMethod, found := interfaceType.MethodByName("WriteBatch")
	assert.True(t, found, "WriteBatch method should exist")
	assert.Equal(t, 1, batchMethod.Type.NumIn(), "WriteBatch should have 1 parameter (messages)")
	assert.Equal(t, 1, batchMethod.Type.NumOut(), "WriteBatch should have 1 return value (error)")
	
	// Test GetSessionMessages method
	getMessagesMethod, found := interfaceType.MethodByName("GetSessionMessages")
	assert.True(t, found, "GetSessionMessages method should exist")
	assert.Equal(t, 1, getMessagesMethod.Type.NumIn(), "GetSessionMessages should have 1 parameter (sessionID)")
	assert.Equal(t, 2, getMessagesMethod.Type.NumOut(), "GetSessionMessages should have 2 return values (messages, error)")
}

// TestDatabaseManagerLifecycleMethods validates lifecycle method signatures
func TestDatabaseManagerLifecycleMethods(t *testing.T) {
	interfaceType := reflect.TypeOf((*database.DatabaseManager)(nil)).Elem()
	
	// Test Start method
	startMethod, found := interfaceType.MethodByName("Start")
	assert.True(t, found, "Start method should exist")
	assert.Equal(t, 0, startMethod.Type.NumIn(), "Start should have 0 parameters")
	assert.Equal(t, 1, startMethod.Type.NumOut(), "Start should have 1 return value (error)")
	
	// Test Stop method
	stopMethod, found := interfaceType.MethodByName("Stop")
	assert.True(t, found, "Stop method should exist")
	assert.Equal(t, 0, stopMethod.Type.NumIn(), "Stop should have 0 parameters")
	assert.Equal(t, 1, stopMethod.Type.NumOut(), "Stop should have 1 return value (error)")
}

// TestDatabaseManagerErrorReturnTypes validates all methods return proper error types
func TestDatabaseManagerErrorReturnTypes(t *testing.T) {
	interfaceType := reflect.TypeOf((*database.DatabaseManager)(nil)).Elem()
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	
	// Check all methods return error as last parameter
	for i := 0; i < interfaceType.NumMethod(); i++ {
		method := interfaceType.Method(i)
		numOut := method.Type.NumOut()
		assert.True(t, numOut > 0, "Method %s should have at least one return value", method.Name)
		
		lastReturnType := method.Type.Out(numOut - 1)
		assert.True(t, lastReturnType.Implements(errorType), 
			"Method %s should return error as last return value", method.Name)
	}
}

// TestDatabaseManagerParameterTypes validates all parameter types are correct
func TestDatabaseManagerParameterTypes(t *testing.T) {
	interfaceType := reflect.TypeOf((*database.DatabaseManager)(nil)).Elem()
	sessionPtrType := reflect.TypeOf((*database.Session)(nil))
	messagePtrType := reflect.TypeOf((*database.Message)(nil))
	messageSliceType := reflect.TypeOf([]*database.Message(nil))
	stringType := reflect.TypeOf("")
	
	// Test CreateSession parameter types
	createMethod, _ := interfaceType.MethodByName("CreateSession")
	assert.Equal(t, sessionPtrType, createMethod.Type.In(0), "CreateSession first parameter should be *Session")
	
	// Test UpdateSession parameter types
	updateMethod, _ := interfaceType.MethodByName("UpdateSession")
	assert.Equal(t, sessionPtrType, updateMethod.Type.In(0), "UpdateSession first parameter should be *Session")
	
	// Test WriteMessage parameter types
	writeMethod, _ := interfaceType.MethodByName("WriteMessage")
	assert.Equal(t, messagePtrType, writeMethod.Type.In(0), "WriteMessage first parameter should be *Message")
	
	// Test WriteBatch parameter types
	batchMethod, _ := interfaceType.MethodByName("WriteBatch")
	assert.Equal(t, messageSliceType, batchMethod.Type.In(0), "WriteBatch first parameter should be []*Message")
	
	// Test GetSessionMessages parameter types
	getMessagesMethod, _ := interfaceType.MethodByName("GetSessionMessages")
	assert.Equal(t, stringType, getMessagesMethod.Type.In(0), "GetSessionMessages first parameter should be string")
}

// TestDatabaseManagerBatchOperationSignature validates WriteBatch method for exactly 100 message support
func TestDatabaseManagerBatchOperationSignature(t *testing.T) {
	interfaceType := reflect.TypeOf((*database.DatabaseManager)(nil)).Elem()
	messageSliceType := reflect.TypeOf([]*database.Message(nil))
	
	// Test that WriteBatch accepts []*Message slice (no size restriction in signature)
	// The 100 message limit is enforced by implementation, not interface
	batchMethod, found := interfaceType.MethodByName("WriteBatch")
	assert.True(t, found, "WriteBatch method should exist")
	assert.Equal(t, messageSliceType, batchMethod.Type.In(0), 
		"WriteBatch should accept []*Message slice (size limit enforced by implementation)")
}

// TestDatabaseManagerIntegrationTypes validates compatibility with Step 1.1 domain models
func TestDatabaseManagerIntegrationTypes(t *testing.T) {
	// Verify that the interface uses Session and Message types from Step 1.1
	interfaceType := reflect.TypeOf((*database.DatabaseManager)(nil)).Elem()
	
	// Check that methods reference the correct types from the database package
	createMethod, _ := interfaceType.MethodByName("CreateSession")
	sessionType := createMethod.Type.In(0).Elem() // Get Session from *Session
	assert.Equal(t, "Session", sessionType.Name(), "Should use Session type from Step 1.1")
	
	writeMethod, _ := interfaceType.MethodByName("WriteMessage")
	messageType := writeMethod.Type.In(0).Elem() // Get Message from *Message
	assert.Equal(t, "Message", messageType.Name(), "Should use Message type from Step 1.1")
	
	// Verify return types
	getMethod, _ := interfaceType.MethodByName("GetActiveSession")
	returnSessionType := getMethod.Type.Out(0).Elem() // Get Session from *Session
	assert.Equal(t, "Session", returnSessionType.Name(), "Should return Session type from Step 1.1")
	
	getMessagesMethod, _ := interfaceType.MethodByName("GetSessionMessages")
	returnMessageSliceType := getMessagesMethod.Type.Out(0)
	assert.Equal(t, reflect.Slice, returnMessageSliceType.Kind(), "Should return slice of messages")
	returnMessageType := returnMessageSliceType.Elem().Elem() // Get Message from []*Message
	assert.Equal(t, "Message", returnMessageType.Name(), "Should return Message slice from Step 1.1")
}