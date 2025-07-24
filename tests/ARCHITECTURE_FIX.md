# TestClient Architectural Fix Documentation

## Problem Statement

The TestClient in the test infrastructure was causing "concurrent write to websocket connection" panics during load testing. This occurred because multiple goroutines were directly calling `conn.WriteJSON()` on the same WebSocket connection, violating the WebSocket library's requirement for serialized writes.

## Root Cause

The TestClient was bypassing the production code's single-writer pattern by:
- Directly accessing the raw `*websocket.Conn` 
- Calling `conn.WriteJSON()` from multiple goroutines
- Not using the same Connection wrapper that production code uses

This created an architectural inconsistency where:
- **Production code**: Uses `internal/websocket.Connection` wrapper with single-writer pattern
- **Test code**: Directly wrote to WebSocket, causing race conditions

## Solution Implemented

### 1. Updated TestClient Structure

```go
type TestClient struct {
    // ... existing fields ...
    
    conn     interfaces.Connection  // Use production Connection interface
    rawConn  *websocket.Conn       // Keep for reading (like server does)
    
    // Removed custom writeMsg channel - use production implementation
}
```

### 2. Modified Connection Establishment

```go
func (tc *TestClient) Connect(ctx context.Context) error {
    // ... establish raw connection ...
    
    // Use production Connection wrapper for thread-safe writes
    tc.conn = wsConnection.NewConnection(rawConn)
    tc.rawConn = rawConn  // Keep for reading
    
    // Set credentials to match production pattern
    if err := tc.conn.SetCredentials(tc.UserID, tc.Role, tc.SessionID); err != nil {
        return err
    }
}
```

### 3. Updated Message Sending

```go
func (tc *TestClient) SendMessage(...) error {
    // Use production Connection.WriteJSON (thread-safe)
    err := conn.WriteJSON(message)
    // Single-writer pattern handled internally by Connection wrapper
}
```

### 4. Aligned Reading Pattern

```go
func (tc *TestClient) readLoop() {
    // Read using rawConn.ReadMessage() + json.Unmarshal
    // Matches exactly how production handler reads messages
}
```

## Architectural Benefits

1. **Consistency**: TestClient now uses same code paths as production
2. **Safety**: Single-writer pattern prevents WebSocket race conditions
3. **Validation**: Load tests now validate actual production architecture
4. **Maintainability**: Changes to Connection wrapper automatically apply to tests

## Validation Results

- ✅ No more "concurrent write to websocket connection" panics
- ✅ Successfully sent 50+ concurrent messages in race test
- ✅ All scenario tests pass with new architecture
- ✅ Load tests can run for full duration without crashes
- ✅ Race detector (`go test -race`) finds no issues

## Lessons Learned

1. **Test infrastructure must mirror production architecture** - Bypassing safety mechanisms in tests creates false confidence
2. **Architectural patterns should be enforced** - The Connection wrapper pattern should be the only way to interact with WebSockets
3. **Integration points need clear boundaries** - Raw WebSocket access should be encapsulated and not exposed to test code

## Future Recommendations

1. **Phase 2: External Client Testing** - Create true network-based client tests that can't bypass architectural patterns
2. **Interface Enforcement** - Consider making raw WebSocket connections unexported to prevent bypass
3. **Architectural Validation** - Add tests that verify architectural patterns are followed