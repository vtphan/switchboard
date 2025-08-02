/**
 * Jest Setup File
 * Global test configuration and mocks for JavaScript SDK tests
 */

// Mock console methods to reduce noise in tests
global.console = {
  ...console,
  log: jest.fn(),
  warn: jest.fn(),
  error: jest.fn(),
  debug: jest.fn(),
  info: jest.fn()
};

// Mock global fetch for HTTP requests
global.fetch = jest.fn(() =>
  Promise.resolve({
    ok: true,
    status: 200,
    json: () => Promise.resolve({}),
    text: () => Promise.resolve('')
  })
);

// Mock WebSocket globally (can be overridden in individual tests)
global.WebSocket = jest.fn();

// Mock DOM globals for browser environment simulation
global.window = {
  ...global.window,
  addEventListener: jest.fn(),
  removeEventListener: jest.fn(),
  location: {
    hostname: 'localhost',
    port: '8080',
    protocol: 'http:'
  }
};

global.document = {
  ...global.document,
  getElementById: jest.fn(),
  createElement: jest.fn(),
  addEventListener: jest.fn()
};

// Mock alert for UI tests
global.alert = jest.fn();

// Clean up mocks between tests
beforeEach(() => {
  jest.clearAllMocks();
  global.fetch.mockClear();
  global.console.log.mockClear();
  if (global.console.warn && global.console.warn.mockClear) {
    global.console.warn.mockClear();
  }
  if (global.console.error && global.console.error.mockClear) {
    global.console.error.mockClear();
  }
  if (global.console.info && global.console.info.mockClear) {
    global.console.info.mockClear();
  }
  global.alert.mockClear();
});

// Test utilities
global.TestUtils = {
  // Create a mock WebSocket with common behavior
  createMockWebSocket: (options = {}) => {
    return class MockWebSocket {
      constructor(url) {
        this.url = url;
        this.readyState = options.readyState || MockWebSocket.OPEN;
        this.sentMessages = [];
        this.onopen = null;
        this.onclose = null;
        this.onerror = null;
        this.onmessage = null;

        if (options.autoConnect !== false) {
          setTimeout(() => {
            if (this.onopen) this.onopen();
          }, options.connectionDelay || 10);
        }
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

      simulateMessage(data) {
        if (this.onmessage) {
          this.onmessage({ data: JSON.stringify(data) });
        }
      }

      static get CONNECTING() { return 0; }
      static get OPEN() { return 1; }
      static get CLOSING() { return 2; }
      static get CLOSED() { return 3; }
    };
  },

  // Create a mock DOM element
  createMockElement: (tagName = 'div') => {
    return {
      tagName,
      value: '',
      textContent: '',
      innerHTML: '',
      disabled: false,
      style: {},
      className: '',
      children: [],
      eventListeners: {},
      addEventListener: jest.fn(),
      removeEventListener: jest.fn(),
      appendChild: jest.fn(),
      focus: jest.fn(),
      click: jest.fn()
    };
  },

  // Wait for async operations
  waitFor: (ms = 10) => new Promise(resolve => setTimeout(resolve, ms)),

  // Generate test message
  createTestMessage: (type, content = {}) => ({
    type,
    from_user: 'test-user',
    content: { text: 'Test message', ...content },
    timestamp: new Date().toISOString()
  })
};

// Custom Jest matchers
expect.extend({
  toBeValidMessage(received) {
    const requiredFields = ['type', 'content'];
    const validTypes = ['broadcast_to_instructors', 'direct_message', 'broadcast_to_students', 'system', 'error'];
    
    const pass = requiredFields.every(field => received.hasOwnProperty(field)) &&
                 validTypes.includes(received.type) &&
                 typeof received.content === 'object';

    if (pass) {
      return {
        message: () => `Expected ${JSON.stringify(received)} not to be a valid message`,
        pass: true
      };
    } else {
      return {
        message: () => `Expected ${JSON.stringify(received)} to be a valid message with type and content`,
        pass: false
      };
    }
  },

  toHaveBeenCalledWithMessage(received, expectedType, expectedContent) {
    const calls = received.mock.calls;
    const pass = calls.some(call => {
      const message = call[0];
      return message && 
             message.type === expectedType && 
             (expectedContent ? JSON.stringify(message.content).includes(JSON.stringify(expectedContent)) : true);
    });

    if (pass) {
      return {
        message: () => `Expected mock not to have been called with message type ${expectedType}`,
        pass: true
      };
    } else {
      return {
        message: () => `Expected mock to have been called with message type ${expectedType}`,
        pass: false
      };
    }
  }
});