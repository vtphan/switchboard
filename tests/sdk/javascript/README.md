# JavaScript SDK Tests

Comprehensive test suite for the Switchboard JavaScript SDK, covering core functionality, message handling, WebSocket connections, and client application integration.

## Overview

This test suite validates the JavaScript SDK components:

- **SwitchboardClient** - Core client functionality
- **MessageBuilder** - Fluent message construction API  
- **Client Examples** - StudentApp and TeacherApp integration
- **WebSocket Layer** - Connection management and messaging

## Test Structure

```
tests/sdk/javascript/
├── client.test.js           # Core SwitchboardClient tests
├── message-builder.test.js  # MessageBuilder class tests
├── integration.test.js      # Client example integration tests
├── websocket.test.js        # WebSocket connection and messaging tests
├── package.json             # Test dependencies and scripts
├── jest.setup.js           # Jest configuration and global mocks
└── README.md               # This file
```

## Running Tests

### Prerequisites

```bash
cd tests/sdk/javascript
npm install
```

### Test Commands

```bash
# Run all tests
npm test

# Run with watch mode for development
npm run test:watch

# Generate coverage report
npm run test:coverage

# Run with verbose output
npm run test:verbose

# Run specific test files
npm run test:client           # Core client tests
npm run test:message-builder  # Message builder tests
npm run test:integration      # Integration tests
npm run test:websocket        # WebSocket tests
```

## Test Categories

### 1. Core Client Tests (`client.test.js`)

Tests the main `SwitchboardClient` class functionality:

**Constructor & Configuration**
- Validates required options (userId, role, wsUrl)
- Sets default values correctly
- Rejects invalid roles
- Derives API URL from WebSocket URL

**Connection Management**
- Establishes WebSocket connections
- Handles connection failures and timeouts
- Manages connection state transitions
- Prevents duplicate connections

**Message Handling**
- Parses incoming WebSocket messages
- Routes messages to appropriate hooks
- Stores messages with size limits
- Handles malformed JSON gracefully

**Message Sending**
- Validates session requirements
- Enforces rate limiting (100 msg/min)
- Enforces message size limits (64KB)
- Queues messages when disconnected

**Session Management**
- Starts/ends sessions via HTTP API
- Retrieves active session information
- Handles session state changes
- Processes session events

**Utility Methods**
- Connection status queries
- Message filtering and retrieval
- Rate limit status checking
- Debug logging control

### 2. Message Builder Tests (`message-builder.test.js`)

Tests the fluent `MessageBuilder` API:

**Message Construction**
- Creates builders with correct type/name
- Maps message names to contexts
- Supports method chaining

**Content Methods**
- `withText()` - adds text content with validation
- `withCode()` - adds code snippets with optional language
- `withLineNumber()` - adds line references  
- `withData()` - merges custom data objects

**Metadata Methods**
- `markAsImportant()` - flags urgent messages
- `withUrgency()` - sets urgency levels
- `withTags()` - adds searchable tags
- `referencingMessage()` - links to other messages
- `withContext()` - overrides default context

**Message Sending**
- Validates content requirements
- Integrates with client validation
- Handles rate limiting and size limits
- Queues messages when disconnected

**Complex Examples**
- Student help requests with code
- Instructor announcements  
- Code review responses
- Multi-part messages

### 3. Integration Tests (`integration.test.js`)

Tests the client application examples:

**StudentApp Integration**
- Connection workflow
- Question submission (text + code)
- Session event handling
- Message reception
- UI state management

**TeacherApp Integration**  
- Connection workflow
- Session management (start/end)
- Announcement broadcasting
- Direct student responses
- Question handling

**Student-Teacher Interaction**
- Complete workflow simulation
- Message routing verification
- Session state synchronization
- UI updates across both clients

### 4. WebSocket Tests (`websocket.test.js`)

Tests WebSocket-specific functionality:

**Connection Management**
- Connection establishment
- Timeout handling
- Connection failure recovery
- State transition tracking

**Reconnection Logic**
- Automatic reconnection attempts
- Exponential backoff delays
- Maximum attempt limits
- Successful reconnection handling

**Message Queueing**
- Queues messages when disconnected
- Flushes queue on reconnection
- Maintains message ordering
- Handles partial send failures

**Protocol Compliance**
- Validates WebSocket states
- Handles connection errors
- Manages close codes/reasons
- Processes malformed messages

**Performance & Resilience**
- High-frequency message handling
- Memory leak prevention
- Connection interruption recovery
- Rate limiting integration

## Test Utilities

### Mock Objects

**MockWebSocket**
- Simulates WebSocket behavior
- Tracks sent messages
- Supports connection scenarios
- Handles event simulation

**MockElement**
- Simulates DOM elements
- Tracks property changes
- Handles event listeners
- Supports user interactions

### Global Utilities (`TestUtils`)

```javascript
// Create mock WebSocket with options
const MockWS = TestUtils.createMockWebSocket({ 
  readyState: WebSocket.OPEN,
  autoConnect: true,
  connectionDelay: 10 
});

// Create mock DOM element
const mockButton = TestUtils.createMockElement('button');

// Wait for async operations
await TestUtils.waitFor(50);

// Generate test message
const message = TestUtils.createTestMessage('broadcast_to_students', {
  text: 'Test announcement'
});
```

### Custom Matchers

```javascript
// Validate message structure
expect(message).toBeValidMessage();

// Check hook calls
expect(mockHook).toHaveBeenCalledWithMessage('broadcast_to_students', { 
  text: 'Hello' 
});
```

## Coverage Requirements

The test suite aims for comprehensive coverage:

- **Statements**: >95%
- **Branches**: >90% 
- **Functions**: >95%
- **Lines**: >95%

Critical paths with 100% coverage:
- Connection establishment
- Message sending/receiving
- Session management
- Error handling
- Rate limiting

## Testing Best Practices

### Mock Strategy
- Mock external dependencies (WebSocket, fetch, DOM)
- Use realistic mock data
- Test both success and failure scenarios
- Isolate units under test

### Async Testing
- Use async/await for promises
- Mock timers for delays
- Test timeout scenarios
- Handle race conditions

### Error Testing
- Test all error conditions
- Validate error messages
- Check error propagation
- Test recovery scenarios

### Integration Testing
- Test realistic user workflows
- Validate cross-component communication
- Check state synchronization
- Test UI interactions

## Common Test Patterns

### Connection Testing
```javascript
test('should connect successfully', async () => {
  await client.connect();
  
  expect(mockHooks.onConnected).toHaveBeenCalled();
  expect(client.connectionState).toBe('connected');
});
```

### Message Testing
```javascript
test('should send message correctly', () => {
  client.broadcast_to_instructors('helpRequest')
    .withText('Need help')
    .send();
    
  expect(client.ws.sentMessages).toHaveLength(1);
  const sent = JSON.parse(client.ws.sentMessages[0]);
  expect(sent.content.text).toBe('Need help');
});
```

### Hook Testing
```javascript
test('should call appropriate hook', () => {
  const message = { type: 'broadcast_to_students', content: { text: 'Hi' } };
  client.ws.simulateMessage(message);
  
  expect(mockHooks.onBroadcastToStudents).toHaveBeenCalledWith(message);
});
```

## Troubleshooting

### Common Issues

**Tests timing out**
- Check async/await usage
- Verify mock timer handling
- Increase Jest timeout if needed

**Mock not working**
- Ensure mocks are set up before imports
- Check mock implementation
- Verify mock reset between tests

**Coverage gaps**
- Check for untested error paths
- Add edge case tests
- Test async error scenarios

### Debug Tips

```javascript
// Enable client debug logging
client.setDebug(true);

// Log mock calls
console.log(mockHooks.onConnected.mock.calls);

// Check sent messages
console.log(client.ws.sentMessages.map(m => JSON.parse(m)));
```

## Contributing

When adding new tests:

1. Follow existing naming conventions
2. Group related tests in describe blocks
3. Use clear, descriptive test names
4. Add setup/teardown as needed
5. Update this README for new test categories
6. Maintain high coverage standards

## CI/CD Integration

Tests are designed to run in CI environments:

- No external dependencies
- Deterministic timing
- Clean state between tests
- Comprehensive error checking
- Coverage reporting compatible with CI tools