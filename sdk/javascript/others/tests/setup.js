// Jest setup file for SwitchboardClient tests

// Mock console methods to reduce noise in tests
global.console = {
  ...console,
  // Suppress expected error logs during tests
  error: jest.fn(),
  warn: jest.fn(),
  log: jest.fn()
};

// Add custom matchers if needed
expect.extend({
  toBeValidMessage(received) {
    const pass = (
      received &&
      typeof received.type === 'string' &&
      typeof received.context === 'string' &&
      typeof received.content === 'object' &&
      received.content !== null
    );
    
    return {
      message: () =>
        pass
          ? `expected ${JSON.stringify(received)} not to be a valid message`
          : `expected ${JSON.stringify(received)} to be a valid message with type, context, and content`,
      pass,
    };
  },
});