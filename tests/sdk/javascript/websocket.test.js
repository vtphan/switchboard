/**
 * WebSocket Connection and Messaging Tests
 * Tests WebSocket-specific functionality including connection management, 
 * reconnection logic, message queueing, and error handling
 */

const SwitchboardClient = require('../../../sdk/javascript/switchboard-client.js');

// Advanced WebSocket Mock for testing various connection scenarios
class AdvancedMockWebSocket {
  constructor(url) {
    this.url = url;
    this.readyState = AdvancedMockWebSocket.CONNECTING;
    this.onopen = null;
    this.onclose = null;
    this.onerror = null;
    this.onmessage = null;
    this.sentMessages = [];
    this.closeCode = null;
    this.closeReason = null;
    
    // Behaviors for testing
    this.shouldFailConnection = false;
    this.connectionDelay = 10;
    this.shouldTimeout = false;
    
    this.setupConnection();
  }
  
  setupConnection() {
    if (this.shouldTimeout) {
      // Don't complete connection for timeout tests
      return;
    }
    
    setTimeout(() => {
      if (this.shouldFailConnection) {
        this.readyState = AdvancedMockWebSocket.CLOSED;
        if (this.onerror) this.onerror(new Error('Connection failed'));
        return;
      }
      
      this.readyState = AdvancedMockWebSocket.OPEN;
      if (this.onopen) this.onopen();
    }, this.connectionDelay);
  }
  
  send(data) {
    if (this.readyState !== AdvancedMockWebSocket.OPEN) {
      throw new Error('WebSocket is not open');
    }
    this.sentMessages.push(data);
  }
  
  close(code = 1000, reason = '') {
    this.closeCode = code;
    this.closeReason = reason;
    this.readyState = AdvancedMockWebSocket.CLOSED;
    if (this.onclose) {
      this.onclose({ code, reason });
    }
  }
  
  // Test helper methods
  simulateMessage(data) {
    if (this.onmessage) {
      this.onmessage({ data: JSON.stringify(data) });
    }
  }
  
  simulateError(error) {
    if (this.onerror) {
      this.onerror(error);
    }
  }
  
  simulateUnexpectedClose(code = 1006, reason = 'Connection lost') {
    this.readyState = AdvancedMockWebSocket.CLOSED;
    if (this.onclose) {
      this.onclose({ code, reason });
    }
  }
  
  static get CONNECTING() { return 0; }
  static get OPEN() { return 1; }
  static get CLOSING() { return 2; }
  static get CLOSED() { return 3; }
}

// Mock timers for testing reconnection logic
const mockTimers = {
  timeouts: [],
  setTimeout: jest.fn((callback, delay) => {
    const id = Math.random();
    mockTimers.timeouts.push({ id, callback, delay });
    return id;
  }),
  clearTimeout: jest.fn((id) => {
    mockTimers.timeouts = mockTimers.timeouts.filter(timer => timer.id !== id);
  }),
  triggerTimeouts: () => {
    const timeouts = [...mockTimers.timeouts];
    mockTimers.timeouts = [];
    timeouts.forEach(timer => timer.callback());
  },
  clear: () => {
    mockTimers.timeouts = [];
  }
};

global.setTimeout = mockTimers.setTimeout;
global.clearTimeout = mockTimers.clearTimeout;
global.WebSocket = AdvancedMockWebSocket;

describe('WebSocket Connection and Messaging', () => {
  let client;
  let mockHooks;

  beforeEach(() => {
    mockHooks = {
      onConnecting: jest.fn(),
      onConnected: jest.fn(),
      onDisconnected: jest.fn(),
      onReconnecting: jest.fn(),
      onConnectionError: jest.fn(),
      onMessage: jest.fn(),
      onError: jest.fn()
    };

    client = new SwitchboardClient({
      userId: 'test-user',
      role: 'student',
      wsUrl: 'ws://localhost:8080/ws',
      hooks: mockHooks,
      maxReconnectAttempts: 3
    });

    mockTimers.clear();
    jest.clearAllMocks();
  });

  afterEach(() => {
    if (client && client.ws) {
      client.disconnect();
    }
    mockTimers.clear();
  });

  describe('Connection Establishment', () => {
    test('should establish WebSocket connection successfully', async () => {
      await client.connect();

      expect(mockHooks.onConnecting).toHaveBeenCalled();
      expect(mockHooks.onConnected).toHaveBeenCalled();
      expect(client.connectionState).toBe('connected');
      expect(client.ws).toBeTruthy();
      expect(client.ws.url).toBe('ws://localhost:8080/ws?user_id=test-user&role=student');
    });

    test('should handle connection timeout', async () => {
      global.WebSocket = class extends AdvancedMockWebSocket {
        constructor(url) {
          super(url);
          this.shouldTimeout = true;
        }
      };

      await expect(client.connect()).rejects.toThrow('Connection timeout');
      expect(mockHooks.onConnectionError).toHaveBeenCalled();
      expect(client.connectionState).toBe('disconnected');
    });

    test('should handle connection failure', async () => {
      global.WebSocket = class extends AdvancedMockWebSocket {
        constructor(url) {
          super(url);
          this.shouldFailConnection = true;
        }
      };

      await expect(client.connect()).rejects.toThrow();
      expect(mockHooks.onConnectionError).toHaveBeenCalled();
      expect(client.connectionState).toBe('disconnected');
    });

    test('should not connect if already connected', async () => {
      await client.connect();
      mockHooks.onConnecting.mockClear();

      await client.connect();
      expect(mockHooks.onConnecting).not.toHaveBeenCalled();
    });

    test('should not connect if already connecting', async () => {
      const connectPromise1 = client.connect();
      const connectPromise2 = client.connect();

      await Promise.all([connectPromise1, connectPromise2]);

      expect(mockHooks.onConnecting).toHaveBeenCalledTimes(1);
    });

    test('should encode URL parameters correctly', async () => {
      const specialClient = new SwitchboardClient({
        userId: 'user with spaces',
        role: 'student',
        wsUrl: 'ws://localhost:8080/ws',
        hooks: mockHooks
      });

      await specialClient.connect();

      expect(specialClient.ws.url).toBe('ws://localhost:8080/ws?user_id=user%20with%20spaces&role=student');
      specialClient.disconnect();
    });
  });

  describe('Connection Management', () => {
    test('should handle normal disconnection', async () => {
      await client.connect();

      client.disconnect();

      expect(client.ws.closeCode).toBe(1000);
      expect(client.ws.closeReason).toBe('Client disconnect');
      expect(client.connectionState).toBe('disconnected');
      expect(client.sessionActive).toBe(false);
      expect(client.currentSession).toBe(null);
      expect(client.reconnectAttempts).toBe(0);
    });

    test('should handle unexpected disconnection', async () => {
      await client.connect();

      client.ws.simulateUnexpectedClose(1006, 'Network error');

      expect(mockHooks.onDisconnected).toHaveBeenCalledWith(1006, 'Network error');
      expect(client.connectionState).toBe('disconnected');
    });

    test('should setup WebSocket event handlers correctly', async () => {
      await client.connect();

      expect(client.ws.onopen).toBeTruthy();
      expect(client.ws.onclose).toBeTruthy();
      expect(client.ws.onerror).toBeTruthy();
      expect(client.ws.onmessage).toBeTruthy();
    });

    test('should handle WebSocket errors', async () => {
      await client.connect();

      const error = new Error('WebSocket error');
      client.ws.simulateError(error);

      expect(mockHooks.onConnectionError).toHaveBeenCalledWith(error);
    });
  });

  describe('Reconnection Logic', () => {
    test('should attempt reconnection after unexpected disconnection', async () => {
      await client.connect();

      client.ws.simulateUnexpectedClose(1006, 'Connection lost');

      expect(mockHooks.onDisconnected).toHaveBeenCalled();
      expect(mockTimers.timeouts).toHaveLength(1);
      expect(mockHooks.onReconnecting).toHaveBeenCalledWith(1, 1000);
    });

    test('should not reconnect after normal closure', async () => {
      await client.connect();

      client.ws.close(1000, 'Normal closure');

      expect(mockTimers.timeouts).toHaveLength(0);
      expect(mockHooks.onReconnecting).not.toHaveBeenCalled();
    });

    test('should use exponential backoff for reconnection delays', async () => {
      await client.connect();

      // First reconnection attempt
      client.ws.simulateUnexpectedClose(1006);
      expect(mockHooks.onReconnecting).toHaveBeenCalledWith(1, 1000);

      // Simulate failed reconnection
      mockTimers.triggerTimeouts();
      global.WebSocket = class extends AdvancedMockWebSocket {
        constructor(url) {
          super(url);
          this.shouldFailConnection = true;
        }
      };

      // Wait for connection attempt to fail
      await new Promise(resolve => setTimeout(resolve, 20));

      // Second reconnection attempt should have double delay
      expect(mockHooks.onReconnecting).toHaveBeenCalledWith(2, 2000);
    });

    test('should cap reconnection delay at maximum', async () => {
      client.reconnectAttempts = 10; // High number to test cap
      await client.connect();

      client.ws.simulateUnexpectedClose(1006);

      // Should be capped at maxReconnectDelay (30000ms)
      const lastCall = mockHooks.onReconnecting.mock.calls[mockHooks.onReconnecting.mock.calls.length - 1];
      expect(lastCall[1]).toBe(30000);
    });

    test('should stop reconnecting after max attempts', async () => {
      client.maxReconnectAttempts = 2;
      await client.connect();

      // Simulate multiple failed reconnections
      for (let i = 0; i < 3; i++) {
        client.ws.simulateUnexpectedClose(1006);
        mockTimers.triggerTimeouts();
        
        global.WebSocket = class extends AdvancedMockWebSocket {
          constructor(url) {
            super(url);
            this.shouldFailConnection = true;
          }
        };
        
        await new Promise(resolve => setTimeout(resolve, 20));
      }

      // Should stop after maxReconnectAttempts
      expect(client.reconnectAttempts).toBe(2);
    });

    test('should reset reconnect attempts on successful connection', async () => {
      await client.connect();

      client.reconnectAttempts = 5;
      
      // Simulate successful reconnection
      client.ws.onopen();

      expect(client.reconnectAttempts).toBe(0);
    });
  });

  describe('Message Queueing', () => {
    test('should queue messages when disconnected', () => {
      client.sessionActive = true;
      client.connectionState = 'disconnected';

      client.sendMessage('broadcast_to_instructors', { text: 'Queued message' });

      expect(client.messageQueue).toHaveLength(1);
      expect(client.messageQueue[0]).toContain('Queued message');
    });

    test('should not queue messages when queueMessages is disabled', () => {
      client.queueMessages = false;
      client.sessionActive = true;
      client.connectionState = 'disconnected';

      expect(() => {
        client.sendMessage('broadcast_to_instructors', { text: 'Message' });
      }).toThrow('Not connected and message queuing disabled');

      expect(client.messageQueue).toHaveLength(0);
    });

    test('should flush message queue on connection', async () => {
      client.sessionActive = true;
      client.connectionState = 'disconnected';

      // Queue some messages
      client.sendMessage('broadcast_to_instructors', { text: 'Message 1' });
      client.sendMessage('broadcast_to_instructors', { text: 'Message 2' });
      client.sendMessage('broadcast_to_instructors', { text: 'Message 3' });

      expect(client.messageQueue).toHaveLength(3);

      await client.connect();

      expect(client.messageQueue).toHaveLength(0);
      expect(client.ws.sentMessages).toHaveLength(3);
    });

    test('should handle queue flushing with some messages failing', async () => {
      client.sessionActive = true;
      client.connectionState = 'disconnected';

      // Queue messages
      client.sendMessage('broadcast_to_instructors', { text: 'Message 1' });
      client.sendMessage('broadcast_to_instructors', { text: 'Message 2' });

      await client.connect();

      // Simulate connection closing during flush
      client.ws.readyState = AdvancedMockWebSocket.CLOSED;
      client.flushMessageQueue();

      // Should clear queue even if messages couldn't be sent
      expect(client.messageQueue).toHaveLength(0);
    });
  });

  describe('Message Parsing and Handling', () => {
    beforeEach(async () => {
      await client.connect();
    });

    test('should parse and handle valid JSON messages', () => {
      const message = {
        type: 'broadcast_to_students',
        from_user: 'instructor1',
        content: { text: 'Hello students' },
        timestamp: new Date().toISOString()
      };

      client.ws.simulateMessage(message);

      expect(mockHooks.onMessage).toHaveBeenCalledWith(message);
      expect(client.messages).toContain(message);
    });

    test('should handle malformed JSON gracefully', () => {
      const consoleSpy = jest.spyOn(console, 'log').mockImplementation();
      client.setDebug(true);

      // Simulate malformed message
      if (client.ws.onmessage) {
        client.ws.onmessage({ data: 'invalid json {' });
      }

      expect(consoleSpy).toHaveBeenCalledWith(
        '[SwitchboardClient]',
        'Failed to parse message:',
        expect.any(SyntaxError),
        'invalid json {'
      );

      consoleSpy.mockRestore();
    });

    test('should handle empty messages', () => {
      if (client.ws.onmessage) {
        client.ws.onmessage({ data: '' });
      }

      // Should not crash or call hooks
      expect(mockHooks.onMessage).not.toHaveBeenCalled();
    });

    test('should handle null message data', () => {
      if (client.ws.onmessage) {
        client.ws.onmessage({ data: null });
      }

      // Should not crash
      expect(mockHooks.onMessage).not.toHaveBeenCalled();
    });
  });

  describe('Rate Limiting Integration', () => {
    beforeEach(async () => {
      await client.connect();
      client.sessionActive = true;
    });

    test('should enforce rate limiting on WebSocket sends', () => {
      // Fill rate limit window
      for (let i = 0; i < 100; i++) {
        client.messagesSent.push(Date.now());
      }

      expect(() => {
        client.sendMessage('broadcast_to_instructors', { text: 'Rate limited' });
      }).toThrow('Rate limit exceeded');

      expect(client.ws.sentMessages).toHaveLength(0);
    });

    test('should track rate limit correctly for sent messages', () => {
      const initialTime = Date.now();
      
      client.sendMessage('broadcast_to_instructors', { text: 'Message 1' });
      client.sendMessage('broadcast_to_instructors', { text: 'Message 2' });

      expect(client.messagesSent).toHaveLength(2);
      expect(client.messagesSent[0]).toBeGreaterThanOrEqual(initialTime);
      expect(client.messagesSent[1]).toBeGreaterThanOrEqual(initialTime);
    });

    test('should not track rate limit for queued messages', () => {
      client.connectionState = 'disconnected';
      client.ws.readyState = AdvancedMockWebSocket.CLOSED;

      client.sendMessage('broadcast_to_instructors', { text: 'Queued message' });

      expect(client.messagesSent).toHaveLength(0);
    });

    test('should clean up old rate limit entries', () => {
      const oldTime = Date.now() - 120000; // 2 minutes ago
      const recentTime = Date.now() - 30000; // 30 seconds ago

      client.messagesSent = [oldTime, recentTime];

      const status = client.getRateLimitStatus();

      expect(status.messagesSent).toBe(1); // Only recent message should count
    });
  });

  describe('Connection State Management', () => {
    test('should track connection state transitions', async () => {
      expect(client.connectionState).toBe('disconnected');

      const connectPromise = client.connect();
      expect(client.connectionState).toBe('connecting');

      await connectPromise;
      expect(client.connectionState).toBe('connected');

      client.disconnect();
      expect(client.connectionState).toBe('disconnected');
    });

    test('should handle multiple connection attempts correctly', async () => {
      const promise1 = client.connect();
      const promise2 = client.connect();
      const promise3 = client.connect();

      await Promise.all([promise1, promise2, promise3]);

      expect(client.connectionState).toBe('connected');
      expect(mockHooks.onConnected).toHaveBeenCalledTimes(1);
    });

    test('should handle connection failure state transition', async () => {
      global.WebSocket = class extends AdvancedMockWebSocket {
        constructor(url) {
          super(url);
          this.shouldFailConnection = true;
        }
      };

      try {
        await client.connect();
      } catch (error) {
        // Expected to fail
      }

      expect(client.connectionState).toBe('disconnected');
    });
  });

  describe('WebSocket Protocol Compliance', () => {
    test('should send messages only when connection is open', async () => {
      await client.connect();
      client.sessionActive = true;

      // Ensure connection is open
      expect(client.ws.readyState).toBe(AdvancedMockWebSocket.OPEN);

      client.sendMessage('broadcast_to_instructors', { text: 'Test message' });

      expect(client.ws.sentMessages).toHaveLength(1);
    });

    test('should not send messages when connection is closing', async () => {
      await client.connect();
      client.sessionActive = true;

      client.ws.readyState = AdvancedMockWebSocket.CLOSING;

      client.sendMessage('broadcast_to_instructors', { text: 'Should queue' });

      expect(client.ws.sentMessages).toHaveLength(0);
      expect(client.messageQueue).toHaveLength(1);
    });

    test('should handle WebSocket send errors', async () => {
      await client.connect();
      client.sessionActive = true;

      // Mock WebSocket to throw error on send
      client.ws.send = jest.fn(() => {
        throw new Error('Send failed');
      });

      expect(() => {
        client.sendMessage('broadcast_to_instructors', { text: 'Test message' });
      }).toThrow('Send failed');
    });

    test('should properly close WebSocket with code and reason', () => {
      client.ws = new AdvancedMockWebSocket('ws://test');
      client.disconnect();

      expect(client.ws.closeCode).toBe(1000);
      expect(client.ws.closeReason).toBe('Client disconnect');
    });
  });

  describe('Connection Resilience', () => {
    test('should handle rapid connect/disconnect cycles', async () => {
      for (let i = 0; i < 5; i++) {
        await client.connect();
        expect(client.connectionState).toBe('connected');
        
        client.disconnect();
        expect(client.connectionState).toBe('disconnected');
      }
    });

    test('should handle network interruption simulation', async () => {
      await client.connect();

      // Simulate network interruption
      client.ws.simulateUnexpectedClose(1006, 'Network unreachable');
      expect(mockHooks.onDisconnected).toHaveBeenCalled();

      // Simulate successful reconnection
      mockTimers.triggerTimeouts();
      await new Promise(resolve => setTimeout(resolve, 20));

      expect(mockHooks.onReconnecting).toHaveBeenCalled();
    });

    test('should maintain message ordering during reconnection', async () => {
      await client.connect();
      client.sessionActive = true;

      // Send some messages
      client.sendMessage('broadcast_to_instructors', { text: 'Message 1' });
      client.sendMessage('broadcast_to_instructors', { text: 'Message 2' });

      // Simulate disconnection
      client.connectionState = 'disconnected';
      client.ws.readyState = AdvancedMockWebSocket.CLOSED;

      // Queue more messages
      client.sendMessage('broadcast_to_instructors', { text: 'Message 3' });
      client.sendMessage('broadcast_to_instructors', { text: 'Message 4' });

      // Reconnect
      await client.connect();

      // Check message ordering
      const allMessages = client.ws.sentMessages.map(msg => JSON.parse(msg));
      expect(allMessages[0].content.text).toBe('Message 1');
      expect(allMessages[1].content.text).toBe('Message 2');
      expect(allMessages[2].content.text).toBe('Message 3');
      expect(allMessages[3].content.text).toBe('Message 4');
    });
  });

  describe('Performance and Memory', () => {
    test('should limit stored messages to prevent memory leaks', async () => {
      await client.connect();
      client.maxStoredMessages = 10;

      // Send more messages than the limit
      for (let i = 0; i < 15; i++) {
        const message = {
          type: 'broadcast_to_students',
          content: { text: `Message ${i}` },
          timestamp: new Date().toISOString()
        };
        client.ws.simulateMessage(message);
      }

      expect(client.messages).toHaveLength(10);
      expect(client.messages[0].content.text).toBe('Message 5'); // Oldest kept
      expect(client.messages[9].content.text).toBe('Message 14'); // Newest
    });

    test('should clean up event listeners on disconnect', () => {
      client.ws = new AdvancedMockWebSocket('ws://test');
      
      client.disconnect();

      expect(client.ws).toBe(null);
    });

    test('should handle high-frequency messages without dropping', async () => {
      await client.connect();

      const messageCount = 100;
      const messages = [];

      for (let i = 0; i < messageCount; i++) {
        const message = {
          type: 'broadcast_to_students',
          content: { text: `Rapid message ${i}` },
          timestamp: new Date().toISOString()
        };
        messages.push(message);
        client.ws.simulateMessage(message);
      }

      expect(client.messages).toHaveLength(messageCount);
      expect(mockHooks.onMessage).toHaveBeenCalledTimes(messageCount);
    });
  });
});