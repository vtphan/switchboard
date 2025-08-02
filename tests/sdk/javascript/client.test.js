/**
 * Unit Tests for SwitchboardClient
 * Tests core client functionality including connection management, message handling, and session management
 */

const SwitchboardClient = require('../../../sdk/javascript/switchboard-client.js');

// Mock WebSocket for testing
class MockWebSocket {
  constructor(url) {
    this.url = url;
    this.readyState = MockWebSocket.CONNECTING;
    this.onopen = null;
    this.onclose = null;
    this.onerror = null;
    this.onmessage = null;
    this.sentMessages = [];
    
    // Simulate connection after a short delay
    setTimeout(() => {
      this.readyState = MockWebSocket.OPEN;
      if (this.onopen) this.onopen();
    }, 10);
  }
  
  send(data) {
    if (this.readyState !== MockWebSocket.OPEN) {
      throw new Error('WebSocket is not open');
    }
    this.sentMessages.push(data);
  }
  
  close(code, reason) {
    this.readyState = MockWebSocket.CLOSED;
    if (this.onclose) this.onclose({ code, reason });
  }
  
  // Simulate receiving a message
  simulateMessage(data) {
    if (this.onmessage) {
      this.onmessage({ data: JSON.stringify(data) });
    }
  }
  
  static get CONNECTING() { return 0; }
  static get OPEN() { return 1; }
  static get CLOSING() { return 2; }
  static get CLOSED() { return 3; }
}

// Mock fetch for testing HTTP endpoints
global.fetch = jest.fn();

// Replace global WebSocket with mock
global.WebSocket = MockWebSocket;

describe('SwitchboardClient', () => {
  let client;
  let mockHooks;

  beforeEach(() => {
    mockHooks = {
      onConnecting: jest.fn(),
      onConnected: jest.fn(),
      onDisconnected: jest.fn(),
      onConnectionError: jest.fn(),
      onMessage: jest.fn(),
      onBroadcastToInstructors: jest.fn(),
      onDirectMessage: jest.fn(),
      onBroadcastToStudents: jest.fn(),
      onSystemMessage: jest.fn(),
      onError: jest.fn(),
      onSessionStarted: jest.fn(),
      onSessionEnded: jest.fn(),
      onWaitingForSession: jest.fn(),
      onSessionActive: jest.fn(),
      onHistoryDelivered: jest.fn(),
      onRateLimited: jest.fn(),
      onNoActiveSession: jest.fn(),
      onMessageTooLarge: jest.fn()
    };

    client = new SwitchboardClient({
      userId: 'test-user',
      role: 'student',
      wsUrl: 'ws://localhost:8080/ws',
      hooks: mockHooks
    });

    // Clear fetch mock
    fetch.mockClear();
  });

  afterEach(() => {
    if (client && client.ws) {
      client.disconnect();
    }
  });

  describe('Constructor', () => {
    test('should initialize with required options', () => {
      expect(client.userId).toBe('test-user');
      expect(client.role).toBe('student');
      expect(client.wsUrl).toBe('ws://localhost:8080/ws');
      expect(client.apiUrl).toBe('http://localhost:8080/api');
    });

    test('should throw error for invalid role', () => {
      expect(() => {
        new SwitchboardClient({
          userId: 'test',
          role: 'invalid',
          wsUrl: 'ws://localhost:8080/ws'
        });
      }).toThrow('Role must be "student" or "instructor"');
    });

    test('should set defaults correctly', () => {
      expect(client.connectionState).toBe('disconnected');
      expect(client.sessionActive).toBe(false);
      expect(client.maxReconnectAttempts).toBe(10);
      expect(client.maxMessageSize).toBe(64 * 1024);
      expect(client.queueMessages).toBe(true);
      expect(client.maxStoredMessages).toBe(1000);
    });
  });

  describe('Connection Management', () => {
    test('should connect successfully', async () => {
      await client.connect();
      
      expect(mockHooks.onConnecting).toHaveBeenCalled();
      expect(mockHooks.onConnected).toHaveBeenCalled();
      expect(client.connectionState).toBe('connected');
      expect(client.ws).toBeTruthy();
    });

    test('should not connect if already connected', async () => {
      await client.connect();
      mockHooks.onConnecting.mockClear();
      
      await client.connect();
      expect(mockHooks.onConnecting).not.toHaveBeenCalled();
    });

    test('should handle connection errors', async () => {
      // Mock WebSocket to fail connection
      global.WebSocket = class extends MockWebSocket {
        constructor(url) {
          super(url);
          setTimeout(() => {
            if (this.onerror) this.onerror(new Error('Connection failed'));
          }, 5);
        }
      };

      await expect(client.connect()).rejects.toThrow();
      expect(mockHooks.onConnectionError).toHaveBeenCalled();
    });

    test('should disconnect properly', async () => {
      await client.connect();
      client.disconnect();
      
      expect(client.connectionState).toBe('disconnected');
      expect(client.sessionActive).toBe(false);
      expect(client.currentSession).toBe(null);
      expect(client.reconnectAttempts).toBe(0);
    });

    test('should build WebSocket URL with query parameters', async () => {
      await client.connect();
      
      expect(client.ws.url).toBe('ws://localhost:8080/ws?user_id=test-user&role=student');
    });
  });

  describe('Message Handling', () => {
    beforeEach(async () => {
      await client.connect();
    });

    test('should handle broadcast_to_instructors messages', () => {
      const message = {
        type: 'broadcast_to_instructors',
        from_user: 'student1',
        content: { text: 'I need help' },
        timestamp: new Date().toISOString()
      };

      client.ws.simulateMessage(message);

      expect(mockHooks.onBroadcastToInstructors).toHaveBeenCalledWith(message);
      expect(mockHooks.onMessage).toHaveBeenCalledWith(message);
      expect(client.messages).toContainEqual(message);
    });

    test('should handle direct_message messages', () => {
      const message = {
        type: 'direct_message',
        from_user: 'instructor1',
        to_user: 'test-user',
        content: { text: 'Good question!' },
        timestamp: new Date().toISOString()
      };

      client.ws.simulateMessage(message);

      expect(mockHooks.onDirectMessage).toHaveBeenCalledWith(message);
      expect(mockHooks.onMessage).toHaveBeenCalledWith(message);
      expect(client.messages).toContainEqual(message);
    });

    test('should handle broadcast_to_students messages', () => {
      const message = {
        type: 'broadcast_to_students',
        from_user: 'instructor1',
        content: { text: 'Class announcement' },
        timestamp: new Date().toISOString()
      };

      client.ws.simulateMessage(message);

      expect(mockHooks.onBroadcastToStudents).toHaveBeenCalledWith(message);
      expect(mockHooks.onMessage).toHaveBeenCalledWith(message);
      expect(client.messages).toContainEqual(message);
    });

    test('should handle system messages', () => {
      const message = {
        type: 'system',
        content: {
          event: 'session_started',
          session_id: 'session-123',
          session_name: 'Test Session',
          started_by: 'instructor1'
        }
      };

      client.ws.simulateMessage(message);

      expect(mockHooks.onSessionStarted).toHaveBeenCalled();
      expect(client.sessionActive).toBe(true);
      expect(client.currentSession).toMatchObject({
        id: 'session-123',
        name: 'Test Session',
        startedBy: 'instructor1'
      });
    });

    test('should handle error messages', () => {
      const message = {
        type: 'error',
        error: 'rate_limit_exceeded',
        message: 'Too many messages'
      };

      client.ws.simulateMessage(message);

      expect(mockHooks.onRateLimited).toHaveBeenCalledWith(message);
      expect(mockHooks.onError).toHaveBeenCalledWith(message);
    });

    test('should store messages with size limit', () => {
      client.maxStoredMessages = 2;

      const messages = [
        { type: 'broadcast_to_students', content: { text: 'Message 1' } },
        { type: 'broadcast_to_students', content: { text: 'Message 2' } },
        { type: 'broadcast_to_students', content: { text: 'Message 3' } }
      ];

      messages.forEach(msg => client.ws.simulateMessage(msg));

      expect(client.messages).toHaveLength(2);
      expect(client.messages[0].content.text).toBe('Message 2');
      expect(client.messages[1].content.text).toBe('Message 3');
    });
  });

  describe('Message Sending', () => {
    beforeEach(async () => {
      await client.connect();
      client.sessionActive = true; // Simulate active session
    });

    test('should send message successfully', () => {
      const message = client.sendMessage('broadcast_to_instructors', { text: 'Help request' });

      expect(client.ws.sentMessages).toHaveLength(1);
      const sentMessage = JSON.parse(client.ws.sentMessages[0]);
      expect(sentMessage.type).toBe('broadcast_to_instructors');
      expect(sentMessage.content.text).toBe('Help request');
      expect(sentMessage.context).toBe('general');
    });

    test('should enforce session requirement', () => {
      client.sessionActive = false;

      expect(() => {
        client.sendMessage('broadcast_to_instructors', { text: 'Help request' });
      }).toThrow('Please wait for a session to start');

      expect(mockHooks.onError).toHaveBeenCalled();
    });

    test('should enforce rate limiting', () => {
      // Fill rate limit window
      for (let i = 0; i < 100; i++) {
        client.messagesSent.push(Date.now());
      }

      expect(() => {
        client.sendMessage('broadcast_to_instructors', { text: 'Help request' });
      }).toThrow('Rate limit exceeded');

      expect(mockHooks.onRateLimited).toHaveBeenCalled();
    });

    test('should enforce message size limit', () => {
      const largeContent = { text: 'x'.repeat(client.maxMessageSize) };

      expect(() => {
        client.sendMessage('broadcast_to_instructors', largeContent);
      }).toThrow('Message exceeds');

      expect(mockHooks.onMessageTooLarge).toHaveBeenCalled();
    });

    test('should queue messages when disconnected', () => {
      client.connectionState = 'disconnected';
      client.ws.readyState = MockWebSocket.CLOSED;

      client.sendMessage('broadcast_to_instructors', { text: 'Queued message' });

      expect(client.messageQueue).toHaveLength(1);
      expect(client.ws.sentMessages).toHaveLength(0);
    });

    test('should flush message queue on reconnection', async () => {
      client.connectionState = 'disconnected';
      client.ws.readyState = MockWebSocket.CLOSED;

      // Queue some messages
      client.sendMessage('broadcast_to_instructors', { text: 'Message 1' });
      client.sendMessage('broadcast_to_instructors', { text: 'Message 2' });

      expect(client.messageQueue).toHaveLength(2);

      // Simulate reconnection
      client.ws.readyState = MockWebSocket.OPEN;
      client._flushMessageQueue();

      expect(client.messageQueue).toHaveLength(0);
      expect(client.ws.sentMessages).toHaveLength(2);
    });
  });

  describe('Message Factory Methods', () => {
    beforeEach(async () => {
      await client.connect();
      client.sessionActive = true;
    });

    test('should create broadcast_to_instructors message', () => {
      const builder = client.broadcast_to_instructors('helpRequest');
      expect(builder.type).toBe('broadcast_to_instructors');
      expect(builder.name).toBe('helpRequest');
    });

    test('should create broadcast_to_students message', () => {
      const builder = client.broadcast_to_students('announcement');
      expect(builder.type).toBe('broadcast_to_students');
      expect(builder.name).toBe('announcement');
    });

    test('should create direct_message with recipient', () => {
      const builder = client.direct_message('student1', 'response');
      expect(builder.type).toBe('direct_message');
      expect(builder.name).toBe('response');
      expect(builder.toUser).toBe('student1');
    });

    test('should validate message types', () => {
      expect(() => {
        client.Message('invalid_type', 'test');
      }).toThrow('Invalid message type');
    });

    test('should require toUser for direct messages', () => {
      expect(() => {
        client.Message('direct_message', 'response');
      }).toThrow('direct_message type requires toUser parameter');
    });

    test('should reject toUser for non-direct messages', () => {
      expect(() => {
        client.Message('broadcast_to_students', 'announcement', 'someone');
      }).toThrow('toUser parameter only valid for direct_message type');
    });
  });

  describe('Session Management', () => {
    beforeEach(async () => {
      await client.connect();
    });

    test('should start session successfully', async () => {
      const mockResponse = {
        id: 'session-123',
        name: 'Test Session',
        instructor_id: 'test-user'
      };

      fetch.mockResolvedValueOnce({
        ok: true,
        json: jest.fn().mockResolvedValue(mockResponse)
      });

      const result = await client.startSession('Test Session');

      expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/session/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: 'Test Session',
          instructor_id: 'test-user'
        })
      });

      expect(result).toEqual(mockResponse);
    });

    test('should handle start session errors', async () => {
      fetch.mockResolvedValueOnce({
        ok: false,
        status: 409,
        json: jest.fn().mockResolvedValue({ message: 'Session already active' })
      });

      await expect(client.startSession('Test Session')).rejects.toThrow('A session is already active');
    });

    test('should end session successfully', async () => {
      const mockResponse = { success: true };

      fetch.mockResolvedValueOnce({
        ok: true,
        json: jest.fn().mockResolvedValue(mockResponse)
      });

      const result = await client.endSession();

      expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/session/end', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          instructor_id: 'test-user'
        })
      });

      expect(result).toEqual(mockResponse);
    });

    test('should get active session', async () => {
      const mockSession = {
        id: 'session-123',
        name: 'Active Session'
      };

      fetch.mockResolvedValueOnce({
        ok: true,
        json: jest.fn().mockResolvedValue(mockSession)
      });

      const result = await client.getActiveSession();

      expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/session/active');
      expect(result).toEqual(mockSession);
    });

    test('should return null for no active session', async () => {
      fetch.mockResolvedValueOnce({
        status: 204,
        ok: false
      });

      const result = await client.getActiveSession();
      expect(result).toBe(null);
    });
  });

  describe('Utility Methods', () => {
    test('should return correct connection status', () => {
      expect(client.connectionState).toBe('disconnected');
      expect(client.isConnected()).toBe(false);
    });

    test('should return session status', () => {
      expect(client.isSessionActive()).toBe(false);
      expect(client.getCurrentSession()).toBe(null);

      client.sessionActive = true;
      client.currentSession = { id: 'test', name: 'Test' };

      expect(client.isSessionActive()).toBe(true);
      expect(client.getCurrentSession()).toEqual({ id: 'test', name: 'Test' });
    });

    test('should store messages', () => {
      const messages = [
        { type: 'broadcast_to_students', content: { text: 'Announcement' } },
        { type: 'direct_message', content: { text: 'Direct message' } },
        { type: 'broadcast_to_students', content: { text: 'Another announcement' } }
      ];

      messages.forEach(msg => client._storeMessage(msg));

      expect(client.messages).toHaveLength(3);
      expect(client.messages.filter(m => m.type === 'broadcast_to_students')).toHaveLength(2);
      expect(client.messages.filter(m => m.type === 'direct_message')).toHaveLength(1);
    });

    test('should access message queue', () => {
      expect(client.messageQueue).toBeDefined();
      expect(Array.isArray(client.messageQueue)).toBe(true);
    });

    test('should check rate limiting', () => {
      expect(() => client._checkRateLimit()).not.toThrow();
      expect(() => client._trackRateLimit()).not.toThrow();
    });

    test('should manage debug mode', () => {
      const consoleSpy = jest.spyOn(console, 'log').mockImplementation();

      client.debug = true;
      client._log('test message');

      expect(consoleSpy).toHaveBeenCalledWith('[SwitchboardClient]', 'test message');

      client.debug = false;
      client._log('hidden message');

      expect(consoleSpy).toHaveBeenCalledTimes(1);

      consoleSpy.mockRestore();
    });
  });

  describe('System Message Events', () => {
    beforeEach(async () => {
      await client.connect();
    });

    test('should handle waiting_for_session event', () => {
      const message = {
        type: 'system',
        content: { event: 'waiting_for_session' }
      };

      client.ws.simulateMessage(message);

      expect(client.sessionActive).toBe(false);
      expect(client.currentSession).toBe(null);
      expect(client.historyDelivered).toBe(false);
      expect(mockHooks.onWaitingForSession).toHaveBeenCalledWith(message);
    });

    test('should handle session_active event', () => {
      const message = {
        type: 'system',
        content: {
          event: 'session_active',
          session_id: 'active-123',
          session_name: 'Active Session',
          started_by: 'instructor1',
          start_time: '2023-01-01T10:00:00Z'
        }
      };

      client.ws.simulateMessage(message);

      expect(client.sessionActive).toBe(true);
      expect(client.currentSession).toMatchObject({
        id: 'active-123',
        name: 'Active Session',
        startedBy: 'instructor1'
      });
      expect(mockHooks.onSessionActive).toHaveBeenCalledWith(client.currentSession, message);
    });

    test('should handle session_ended event', () => {
      // First set up an active session
      client.sessionActive = true;
      client.currentSession = { id: 'test', name: 'Test Session' };

      const message = {
        type: 'system',
        content: { event: 'session_ended' }
      };

      client.ws.simulateMessage(message);

      expect(client.sessionActive).toBe(false);
      expect(client.currentSession).toBe(null);
      expect(mockHooks.onSessionEnded).toHaveBeenCalled();
    });

    test('should handle history_delivered event', () => {
      const message = {
        type: 'system',
        content: { event: 'history_delivered' }
      };

      client.ws.simulateMessage(message);

      expect(client.historyDelivered).toBe(true);
      expect(mockHooks.onHistoryDelivered).toHaveBeenCalledWith(message);
    });
  });

  describe('Error Handling', () => {
    beforeEach(async () => {
      await client.connect();
    });

    test('should handle malformed JSON messages', () => {
      const consoleSpy = jest.spyOn(console, 'log').mockImplementation();
      client.debug = true;

      // Simulate malformed message
      if (client.ws.onmessage) {
        client.ws.onmessage({ data: 'invalid json' });
      }

      expect(consoleSpy).toHaveBeenCalledWith(
        '[SwitchboardClient]',
        'Failed to parse message:',
        expect.any(Error),
        'invalid json'
      );

      consoleSpy.mockRestore();
    });

    test('should handle WebSocket errors', () => {
      const error = new Error('WebSocket error');
      if (client.ws.onerror) {
        client.ws.onerror(error);
      }

      expect(mockHooks.onConnectionError).toHaveBeenCalledWith(error);
    });
  });
});