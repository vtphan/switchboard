/**
 * Test Suite for SwitchboardClient V2
 * 
 * Tests the simplified 6-hook architecture and validates protocol compliance.
 */

import SwitchboardClient from '../src/client-v2.js';

// Mock WebSocket for testing
global.WebSocket = class MockWebSocket {
  constructor(url) {
    this.url = url;
    this.readyState = MockWebSocket.CONNECTING;
    this.onopen = null;
    this.onclose = null;
    this.onerror = null;
    this.onmessage = null;
    this.sentMessages = [];
    
    // Simulate async connection
    setTimeout(() => {
      this.readyState = MockWebSocket.OPEN;
      this.onopen?.();
    }, 10);
  }
  
  send(data) {
    this.sentMessages.push(data);
  }
  
  close(code, reason) {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.({ code, reason });
  }
  
  // Simulate receiving a message
  simulateMessage(data) {
    this.onmessage?.({ data: JSON.stringify(data) });
  }
  
  static get CONNECTING() { return 0; }
  static get OPEN() { return 1; }
  static get CLOSING() { return 2; }
  static get CLOSED() { return 3; }
};

// Mock fetch for session management tests
global.fetch = jest.fn();

describe('SwitchboardClient V2 - Constructor and Validation', () => {
  test('should require userId', () => {
    expect(() => new SwitchboardClient({ role: 'student' }))
      .toThrow('userId required');
  });
  
  test('should validate role as student or instructor', () => {
    expect(() => new SwitchboardClient({ userId: 'test', role: 'invalid' }))
      .toThrow('role must be "student" or "instructor"');
  });
  
  test('should set default wsUrl', () => {
    const client = new SwitchboardClient({
      userId: 'test',
      role: 'student'
    });
    expect(client.wsUrl).toBe('ws://localhost:8080/ws');
  });
  
  test('should accept all 6 hooks', () => {
    const handlers = {
      onBroadcastToInstructors: jest.fn(),
      onBroadcastToStudents: jest.fn(),
      onDirectMessage: jest.fn(),
      onSystem: jest.fn(),
      onConnectionChange: jest.fn(),
      onSessionChange: jest.fn()
    };
    
    const client = new SwitchboardClient({
      userId: 'test',
      role: 'student',
      ...handlers
    });
    
    expect(client.messageHandlers.broadcast_to_instructors).toBe(handlers.onBroadcastToInstructors);
    expect(client.messageHandlers.broadcast_to_students).toBe(handlers.onBroadcastToStudents);
    expect(client.messageHandlers.direct_message).toBe(handlers.onDirectMessage);
    expect(client.messageHandlers.system).toBe(handlers.onSystem);
    expect(client.onConnectionChange).toBe(handlers.onConnectionChange);
    expect(client.onSessionChange).toBe(handlers.onSessionChange);
  });
  
  test('should use default handlers when hooks not provided', () => {
    const client = new SwitchboardClient({
      userId: 'test',
      role: 'student'
    });
    
    expect(client.messageHandlers.broadcast_to_instructors).toBeNull();
    expect(typeof client.onConnectionChange).toBe('function');
    expect(typeof client.onSessionChange).toBe('function');
  });
});

describe('SwitchboardClient V2 - Connection Management', () => {
  let client;
  let connectionChangeHandler;
  
  beforeEach(() => {
    connectionChangeHandler = jest.fn();
    client = new SwitchboardClient({
      userId: 'test',
      role: 'student',
      onConnectionChange: connectionChangeHandler
    });
  });
  
  test('should transition through connection states correctly', async () => {
    const connectPromise = client.connect();
    
    expect(client.connectionState).toBe('connecting');
    expect(connectionChangeHandler).toHaveBeenCalledWith('connecting');
    
    await connectPromise;
    
    expect(client.connectionState).toBe('connected');
    expect(connectionChangeHandler).toHaveBeenCalledWith('connected');
  });
  
  test('should handle connection timeout', async () => {
    // Override WebSocket to not trigger onopen
    global.WebSocket = class extends global.WebSocket {
      constructor(url) {
        super(url);
        clearTimeout(this._timer); // Prevent auto-open
      }
    };
    
    await expect(client.connect()).rejects.toThrow('Connection timeout');
    expect(connectionChangeHandler).toHaveBeenCalledWith('error', expect.any(Error));
  });
  
  test('should prevent multiple simultaneous connections', async () => {
    const promise1 = client.connect();
    const promise2 = client.connect();
    
    await promise1;
    await promise2;
    
    // Should only call connecting once
    expect(connectionChangeHandler).toHaveBeenCalledWith('connecting');
    expect(connectionChangeHandler).toHaveBeenCalledTimes(2); // connecting + connected
  });
  
  test('should clean up on disconnect', () => {
    client.disconnect();
    
    expect(client.connectionState).toBe('disconnected');
    expect(client.reconnectAttempts).toBe(0);
    expect(connectionChangeHandler).toHaveBeenCalledWith('disconnected');
  });
});

describe('SwitchboardClient V2 - Message Routing', () => {
  let client;
  let handlers;
  
  beforeEach(() => {
    handlers = {
      onBroadcastToInstructors: jest.fn(),
      onBroadcastToStudents: jest.fn(),
      onDirectMessage: jest.fn(),
      onSystem: jest.fn()
    };
    
    client = new SwitchboardClient({
      userId: 'test',
      role: 'student',
      ...handlers
    });
  });
  
  test('should route broadcast_to_instructors to correct handler', () => {
    const message = { type: 'broadcast_to_instructors', content: { text: 'test' } };
    client._routeMessage(message);
    
    expect(handlers.onBroadcastToInstructors).toHaveBeenCalledWith(message);
  });
  
  test('should route broadcast_to_students to correct handler', () => {
    const message = { type: 'broadcast_to_students', content: { text: 'test' } };
    client._routeMessage(message);
    
    expect(handlers.onBroadcastToStudents).toHaveBeenCalledWith(message);
  });
  
  test('should route direct_message to correct handler', () => {
    const message = { type: 'direct_message', content: { text: 'test' } };
    client._routeMessage(message);
    
    expect(handlers.onDirectMessage).toHaveBeenCalledWith(message);
  });
  
  test('should route system messages to handler and update state', () => {
    const sessionChangeHandler = jest.fn();
    client.onSessionChange = sessionChangeHandler;
    
    const message = {
      type: 'system',
      content: {
        event: 'session_started',
        session_id: 'test-session',
        session_name: 'Test Session',
        started_by: 'instructor'
      }
    };
    
    client._routeMessage(message);
    
    expect(handlers.onSystem).toHaveBeenCalledWith(message);
    expect(sessionChangeHandler).toHaveBeenCalledWith({
      active: true,
      id: 'test-session',
      name: 'Test Session',
      startedBy: 'instructor'
    });
  });
  
  test('should handle missing handlers gracefully', () => {
    const clientWithoutHandlers = new SwitchboardClient({
      userId: 'test',
      role: 'student'
    });
    
    const message = { type: 'broadcast_to_instructors', content: { text: 'test' } };
    
    // Should not throw
    expect(() => clientWithoutHandlers._routeMessage(message)).not.toThrow();
  });
});

describe('SwitchboardClient V2 - Message Sending', () => {
  let client;
  
  beforeEach(async () => {
    client = new SwitchboardClient({
      userId: 'test',
      role: 'student'
    });
    
    // Simulate session active
    client.sessionActive = true;
    
    await client.connect();
  });
  
  test('should send objects directly without builder pattern', () => {
    const content = {
      text: 'Hello',
      context: 'question',
      important: true
    };
    
    const message = client.broadcastToInstructors(content);
    
    expect(message.type).toBe('broadcast_to_instructors');
    expect(message.context).toBe('question');
    expect(message.content.text).toBe('Hello');
    expect(message.content.important).toBe(true);
    // Context should NOT be in content (avoid double-nesting)
    expect(message.content.context).toBeUndefined();
  });
  
  test('should support string content (convert to {text: string})', () => {
    const message = client.broadcastToInstructors('Hello world');
    
    expect(message.content.text).toBe('Hello world');
    expect(message.context).toBe('general'); // default context
  });
  
  test('should enforce rate limiting', () => {
    // Fill up rate limit
    for (let i = 0; i < 100; i++) {
      client._trackRateLimit();
    }
    
    expect(() => client.broadcastToInstructors('test')).toThrow('Rate limit exceeded');
  });
  
  test('should queue messages when disconnected', () => {
    client.disconnect();
    
    client.broadcastToInstructors('queued message');
    
    expect(client.messageQueue.length).toBe(1);
  });
  
  test('should validate message size', () => {
    const largeContent = { text: 'x'.repeat(70000) }; // > 64KB
    
    expect(() => client.broadcastToInstructors(largeContent))
      .toThrow('Message too large');
  });
  
  test('should require active session for non-system messages', () => {
    client.sessionActive = false;
    
    expect(() => client.broadcastToInstructors('test'))
      .toThrow('No active session');
  });
  
  test('should properly structure context field (protocol compliance)', () => {
    const content = {
      text: 'Question about homework',
      context: 'question',
      urgent: true,
      tags: ['homework', 'urgent']
    };
    
    const message = client.broadcastToInstructors(content);
    
    // Verify correct protocol structure
    expect(message).toEqual({
      type: 'broadcast_to_instructors',
      context: 'question',           // Context as separate field
      content: {                     // Clean content without context
        text: 'Question about homework',
        urgent: true,
        tags: ['homework', 'urgent']
      }
    });
    
    // Ensure context is not double-nested
    expect(message.content.context).toBeUndefined();
  });
});

describe('SwitchboardClient V2 - Session State Management', () => {
  let client;
  let sessionChangeHandler;
  
  beforeEach(() => {
    sessionChangeHandler = jest.fn();
    client = new SwitchboardClient({
      userId: 'test',
      role: 'student',
      onSessionChange: sessionChangeHandler
    });
  });
  
  test('should update session state from system messages', () => {
    const message = {
      type: 'system',
      content: {
        event: 'session_started',
        session_id: 'session-123',
        session_name: 'Math Class',
        started_by: 'teacher1'
      }
    };
    
    client._handleSystemMessage(message);
    
    expect(client.sessionActive).toBe(true);
    expect(client.currentSession).toEqual({
      active: true,
      id: 'session-123',
      name: 'Math Class',
      startedBy: 'teacher1'
    });
  });
  
  test('should call onSessionChange when session changes', () => {
    const message = {
      type: 'system',
      content: {
        event: 'session_ended'
      }
    };
    
    client._handleSystemMessage(message);
    
    expect(sessionChangeHandler).toHaveBeenCalledWith({ active: false });
    expect(client.sessionActive).toBe(false);
    expect(client.currentSession).toBeNull();
  });
  
  test('should track active session correctly', () => {
    expect(client.isSessionActive()).toBe(false);
    
    client._setSession({
      active: true,
      id: 'test',
      name: 'Test Session',
      startedBy: 'instructor'
    });
    
    expect(client.isSessionActive()).toBe(true);
    expect(client.getCurrentSession().name).toBe('Test Session');
  });
});

describe('SwitchboardClient V2 - Session Management API', () => {
  let client;
  
  beforeEach(() => {
    client = new SwitchboardClient({
      userId: 'instructor1',
      role: 'instructor'
    });
    
    // Reset fetch mock
    fetch.mockClear();
  });
  
  test('should start session with API call', async () => {
    const mockResponse = { session_id: 'test-123', message: 'Session started' };
    fetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockResponse)
    });
    
    const result = await client.startSession('Test Session');
    
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/session/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: 'Test Session',
        instructor_id: 'instructor1'
      })
    });
    
    expect(result).toEqual(mockResponse);
  });
  
  test('should end session with API call', async () => {
    const mockResponse = { message: 'Session ended' };
    fetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockResponse)
    });
    
    const result = await client.endSession();
    
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/session/end', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        instructor_id: 'instructor1'
      })
    });
    
    expect(result).toEqual(mockResponse);
  });
  
  test('should handle API errors', async () => {
    fetch.mockResolvedValueOnce({
      ok: false,
      status: 400,
      json: () => Promise.resolve({ message: 'Invalid session name' })
    });
    
    await expect(client.startSession('')).rejects.toThrow('Invalid session name');
  });
});

describe('SwitchboardClient V2 - Usage Pattern Support', () => {
  let client;
  
  beforeEach(async () => {
    client = new SwitchboardClient({
      userId: 'test',
      role: 'student'
    });
    
    client.sessionActive = true;
    await client.connect();
  });
  
  test('should support simple object-based message sending', () => {
    // This should not throw
    const message = client.broadcastToInstructors({
      text: 'Hello',
      context: 'question'
    });
    
    expect(message.type).toBe('broadcast_to_instructors');
    expect(message.content.text).toBe('Hello');
  });
  
  test('should support arbitrary content properties with correct structure', () => {
    const message = client.broadcastToStudents({
      text: 'Announcement',
      context: 'announcement',
      important: true,
      customField: 'value',
      tags: ['urgent', 'exam']
    });
    
    // Verify correct protocol structure (context separate from content)
    expect(message.type).toBe('broadcast_to_students');
    expect(message.context).toBe('announcement');
    expect(message.content.text).toBe('Announcement');
    expect(message.content.important).toBe(true);
    expect(message.content.customField).toBe('value');
    expect(message.content.tags).toEqual(['urgent', 'exam']);
    // Context should NOT be in content (avoid double-nesting)
    expect(message.content.context).toBeUndefined();
  });
  
  test('should support async/await for connect()', async () => {
    const newClient = new SwitchboardClient({
      userId: 'test2',
      role: 'student'
    });
    
    // This should work with async/await
    await newClient.connect();
    
    expect(newClient.isConnected()).toBe(true);
  });
  
  test('should work with minimal configuration', () => {
    // Should work with just required fields
    const minimalClient = new SwitchboardClient({
      userId: 'test',
      role: 'student'
    });
    
    expect(minimalClient.userId).toBe('test');
    expect(minimalClient.role).toBe('student');
    expect(minimalClient.wsUrl).toBe('ws://localhost:8080/ws');
  });
});

describe('SwitchboardClient V2 - Protocol Compliance', () => {
  let client;
  
  beforeEach(async () => {
    client = new SwitchboardClient({
      userId: 'test',
      role: 'student'
    });
    
    client.sessionActive = true;
    await client.connect();
  });
  
  test('should ensure context field is never double-nested', () => {
    const testCases = [
      { text: 'Hello', context: 'question' },
      { text: 'Announcement', context: 'announcement', important: true },
      { code: 'console.log("hi")', context: 'submission', language: 'javascript' }
    ];
    
    testCases.forEach(content => {
      const message = client.broadcastToInstructors(content);
      
      // Context should be a separate field
      expect(message.context).toBeDefined();
      expect(typeof message.context).toBe('string');
      
      // Content should NOT contain context
      expect(message.content.context).toBeUndefined();
      
      // Content should contain everything else
      Object.keys(content).forEach(key => {
        if (key !== 'context') {
          expect(message.content[key]).toEqual(content[key]);
        }
      });
    });
  });
  
  test('should handle missing context gracefully', () => {
    const message = client.broadcastToInstructors({ text: 'No context provided' });
    
    expect(message.context).toBe('general'); // default
    expect(message.content.context).toBeUndefined();
  });
  
  test('should preserve all non-context fields in content', () => {
    const complexContent = {
      text: 'Complex message',
      context: 'question',
      metadata: { source: 'test' },
      tags: ['important'],
      customProperty: 'value',
      nested: { data: { value: 123 } }
    };
    
    const message = client.broadcastToInstructors(complexContent);
    
    expect(message.context).toBe('question');
    expect(message.content).toEqual({
      text: 'Complex message',
      metadata: { source: 'test' },
      tags: ['important'],
      customProperty: 'value',
      nested: { data: { value: 123 } }
    });
  });
});