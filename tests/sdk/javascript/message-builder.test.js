/**
 * Unit Tests for MessageBuilder Class
 * Tests the fluent API for building and sending messages
 */

const SwitchboardClient = require('../../../sdk/javascript/switchboard-client.js');

// Mock WebSocket for testing
class MockWebSocket {
  constructor(url) {
    this.url = url;
    this.readyState = MockWebSocket.OPEN;
    this.sentMessages = [];
  }
  
  send(data) {
    this.sentMessages.push(data);
  }
  
  close() {
    this.readyState = MockWebSocket.CLOSED;
  }
  
  static get OPEN() { return 1; }
  static get CLOSED() { return 3; }
}

global.WebSocket = MockWebSocket;

describe('MessageBuilder', () => {
  let client;
  let mockHooks;

  beforeEach(() => {
    mockHooks = {
      onError: jest.fn(),
      onRateLimited: jest.fn(),
      onMessageTooLarge: jest.fn(),
      onNoActiveSession: jest.fn()
    };

    client = new SwitchboardClient({
      userId: 'test-user',
      role: 'student',
      wsUrl: 'ws://localhost:8080/ws',
      hooks: mockHooks
    });

    // Mock connected state
    client.connectionState = 'connected';
    client.ws = new MockWebSocket('ws://localhost:8080/ws');
    client.sessionActive = true;
  });

  describe('Basic Message Building', () => {
    test('should create message builder with correct type and name', () => {
      const builder = client.broadcast_to_instructors('helpRequest');
      
      expect(builder.type).toBe('broadcast_to_instructors');
      expect(builder.name).toBe('helpRequest');
      expect(builder.content.name).toBe('helpRequest');
    });

    test('should set default context based on message name', () => {
      const helpBuilder = client.broadcast_to_instructors('helpRequest');
      expect(helpBuilder.context).toBe('question');

      const announcementBuilder = client.broadcast_to_students('announcement');
      expect(announcementBuilder.context).toBe('announcement');

      const submissionBuilder = client.broadcast_to_instructors('codeSubmission');
      expect(submissionBuilder.context).toBe('submission');

      const analyticsBuilder = client.broadcast_to_instructors('progressUpdate');
      expect(analyticsBuilder.context).toBe('analytics');

      const generalBuilder = client.broadcast_to_instructors('general');
      expect(generalBuilder.context).toBe('general');
    });

    test('should handle unknown message names with general context', () => {
      const builder = client.broadcast_to_instructors('unknownMessageType');
      expect(builder.context).toBe('general');
    });
  });

  describe('Content Building Methods', () => {
    test('should add text content', () => {
      const builder = client.broadcast_to_instructors('helpRequest')
        .withText('I need help with this problem');

      expect(builder.content.text).toBe('I need help with this problem');
    });

    test('should trim text content', () => {
      const builder = client.broadcast_to_instructors('helpRequest')
        .withText('  I need help  ');

      expect(builder.content.text).toBe('I need help');
    });

    test('should validate text is a string', () => {
      expect(() => {
        client.broadcast_to_instructors('helpRequest')
          .withText(123);
      }).toThrow('Text must be a string');
    });

    test('should add code snippet', () => {
      const code = 'function test() { return "hello"; }';
      const builder = client.broadcast_to_instructors('codeSubmission')
        .withCode(code, 'javascript');

      expect(builder.content.code_snippet).toBe(code);
      expect(builder.content.language).toBe('javascript');
    });

    test('should add code without language', () => {
      const code = 'SELECT * FROM users;';
      const builder = client.broadcast_to_instructors('codeSubmission')
        .withCode(code);

      expect(builder.content.code_snippet).toBe(code);
      expect(builder.content.language).toBeUndefined();
    });

    test('should add line number', () => {
      const builder = client.broadcast_to_instructors('helpRequest')
        .withLineNumber(42);

      expect(builder.content.line_number).toBe(42);
    });

    test('should ignore invalid line numbers', () => {
      const builder = client.broadcast_to_instructors('helpRequest')
        .withLineNumber(-1);

      expect(builder.content.line_number).toBeUndefined();
    });

    test('should add custom data', () => {
      const customData = {
        studentLevel: 'beginner',
        lessonId: 'lesson-123',
        difficulty: 3
      };

      const builder = client.broadcast_to_instructors('helpRequest')
        .withData(customData);

      expect(builder.content.studentLevel).toBe('beginner');
      expect(builder.content.lessonId).toBe('lesson-123');
      expect(builder.content.difficulty).toBe(3);
    });

    test('should ignore null or non-object data', () => {
      const builder = client.broadcast_to_instructors('helpRequest')
        .withData(null)
        .withData('string')
        .withData(123);

      // Should only have the name from initialization
      expect(Object.keys(builder.content)).toEqual(['name']);
    });

    test('should override default context', () => {
      const builder = client.broadcast_to_instructors('helpRequest')
        .withContext('urgent');

      expect(builder.context).toBe('urgent');
    });
  });

  describe('Message Metadata', () => {
    test('should mark message as important', () => {
      const builder = client.broadcast_to_students('announcement')
        .markAsImportant();

      expect(builder.content.important).toBe(true);
    });

    test('should set urgency level', () => {
      const builder = client.broadcast_to_instructors('helpRequest')
        .withUrgency('high');

      expect(builder.content.urgency).toBe('high');
    });

    test('should ignore invalid urgency levels', () => {
      const builder = client.broadcast_to_instructors('helpRequest')
        .withUrgency('invalid');

      expect(builder.content.urgency).toBeUndefined();
    });

    test('should add tags', () => {
      const builder = client.broadcast_to_instructors('helpRequest')
        .withTags('javascript', 'debugging', 'beginner');

      expect(builder.content.tags).toEqual(['javascript', 'debugging', 'beginner']);
    });

    test('should filter non-string tags', () => {
      const builder = client.broadcast_to_instructors('helpRequest')
        .withTags('valid', 123, null, 'another-valid', undefined);

      expect(builder.content.tags).toEqual(['valid', 'another-valid']);
    });

    test('should reference another message', () => {
      const builder = client.direct_message('instructor1', 'response')
        .referencingMessage('msg-123');

      expect(builder.content.reference_message_id).toBe('msg-123');
    });
  });

  describe('Method Chaining', () => {
    test('should support fluent interface', () => {
      const builder = client.broadcast_to_instructors('helpRequest')
        .withText('I need help with loops')
        .withCode('for (let i = 0; i < 10; i++) { }', 'javascript')
        .withLineNumber(15)
        .withUrgency('medium')
        .withTags('loops', 'javascript')
        .markAsImportant()
        .withData({ studentId: 'student-123' });

      expect(builder.content).toMatchObject({
        name: 'helpRequest',
        text: 'I need help with loops',
        code_snippet: 'for (let i = 0; i < 10; i++) { }',
        language: 'javascript',
        line_number: 15,
        urgency: 'medium',
        tags: ['loops', 'javascript'],
        important: true,
        studentId: 'student-123'
      });
    });

    test('should allow overriding previous values', () => {
      const builder = client.broadcast_to_instructors('helpRequest')
        .withText('First text')
        .withText('Second text')
        .withUrgency('low')
        .withUrgency('high');

      expect(builder.content.text).toBe('Second text');
      expect(builder.content.urgency).toBe('high');
    });
  });

  describe('Message Sending', () => {
    test('should send message with text content', () => {
      client.broadcast_to_instructors('helpRequest')
        .withText('I need help')
        .send();

      expect(client.ws.sentMessages).toHaveLength(1);
      const sentMessage = JSON.parse(client.ws.sentMessages[0]);
      
      expect(sentMessage).toMatchObject({
        type: 'broadcast_to_instructors',
        context: 'question',
        content: {
          name: 'helpRequest',
          text: 'I need help'
        }
      });
    });

    test('should send message with code content', () => {
      client.broadcast_to_instructors('codeSubmission')
        .withCode('console.log("test");', 'javascript')
        .send();

      expect(client.ws.sentMessages).toHaveLength(1);
      const sentMessage = JSON.parse(client.ws.sentMessages[0]);
      
      expect(sentMessage.content).toMatchObject({
        name: 'codeSubmission',
        code_snippet: 'console.log("test");',
        language: 'javascript'
      });
    });

    test('should send message with custom data only', () => {
      client.broadcast_to_instructors('progressUpdate')
        .withData({ progress: 75, lesson: 'arrays' })
        .send();

      expect(client.ws.sentMessages).toHaveLength(1);
      const sentMessage = JSON.parse(client.ws.sentMessages[0]);
      
      expect(sentMessage.content).toMatchObject({
        name: 'progressUpdate',
        progress: 75,
        lesson: 'arrays'
      });
    });

    test('should send direct message with recipient', () => {
      client.direct_message('instructor1', 'question')
        .withText('Can you help me?')
        .send();

      expect(client.ws.sentMessages).toHaveLength(1);
      const sentMessage = JSON.parse(client.ws.sentMessages[0]);
      
      expect(sentMessage).toMatchObject({
        type: 'direct_message',
        to_user: 'instructor1',
        content: {
          name: 'question',
          text: 'Can you help me?'
        }
      });
    });

    test('should require content before sending', () => {
      expect(() => {
        client.broadcast_to_instructors('helpRequest').send();
      }).toThrow("Message 'helpRequest' needs text, code, or data content");
    });

    test('should accept message with only data content', () => {
      expect(() => {
        client.broadcast_to_instructors('progressUpdate')
          .withData({ progress: 50 })
          .send();
      }).not.toThrow();
    });

    test('should handle sending errors gracefully', () => {
      // Simulate no active session
      client.sessionActive = false;

      expect(() => {
        client.broadcast_to_instructors('helpRequest')
          .withText('I need help')
          .send();
      }).toThrow('Please wait for a session to start');

      expect(mockHooks.onError).toHaveBeenCalled();
    });
  });

  describe('Context Mapping', () => {
    test('should map question-related names to question context', () => {
      expect(client.broadcast_to_instructors('helpRequest').context).toBe('question');
      expect(client.broadcast_to_instructors('question').context).toBe('question');
      expect(client.broadcast_to_instructors('clarification').context).toBe('question');
    });

    test('should map submission-related names to submission context', () => {
      expect(client.broadcast_to_instructors('codeSubmission').context).toBe('submission');
      expect(client.broadcast_to_instructors('submission').context).toBe('submission');
      expect(client.broadcast_to_instructors('workSubmission').context).toBe('submission');
    });

    test('should map analytics-related names to analytics context', () => {
      expect(client.broadcast_to_instructors('codeSnapshot').context).toBe('analytics');
      expect(client.broadcast_to_instructors('progressUpdate').context).toBe('analytics');
      expect(client.broadcast_to_instructors('analytics').context).toBe('analytics');
    });

    test('should map instructor messages to appropriate contexts', () => {
      expect(client.broadcast_to_students('announcement').context).toBe('announcement');
      expect(client.broadcast_to_students('instruction').context).toBe('instruction');
      expect(client.direct_message('student1', 'response').context).toBe('response');
    });

    test('should map support-related names to request context', () => {
      expect(client.broadcast_to_instructors('technicalIssue').context).toBe('request');
      expect(client.broadcast_to_instructors('support').context).toBe('request');
    });

    test('should map general interaction names to general context', () => {
      expect(client.broadcast_to_students('general').context).toBe('general');
      expect(client.broadcast_to_students('discovery').context).toBe('general');
      expect(client.broadcast_to_students('sharing').context).toBe('general');
    });
  });

  describe('Integration with Client Validation', () => {
    test('should respect client rate limiting', () => {
      // Fill rate limit
      for (let i = 0; i < 100; i++) {
        client.messagesSent.push(Date.now());
      }

      expect(() => {
        client.broadcast_to_instructors('helpRequest')
          .withText('This should be rate limited')
          .send();
      }).toThrow('Rate limit exceeded');

      expect(mockHooks.onRateLimited).toHaveBeenCalled();
    });

    test('should respect message size limits', () => {
      const largeText = 'x'.repeat(client.maxMessageSize);

      expect(() => {
        client.broadcast_to_instructors('helpRequest')
          .withText(largeText)
          .send();
      }).toThrow('Message exceeds');

      expect(mockHooks.onMessageTooLarge).toHaveBeenCalled();
    });

    test('should handle disconnected state', () => {
      client.connectionState = 'disconnected';
      client.ws.readyState = MockWebSocket.CLOSED;

      // Should queue message instead of throwing
      client.broadcast_to_instructors('helpRequest')
        .withText('Queued message')
        .send();

      expect(client.messageQueue).toHaveLength(1);
      expect(client.ws.sentMessages).toHaveLength(0);
    });
  });

  describe('Complex Message Examples', () => {
    test('should build comprehensive help request', () => {
      const message = client.broadcast_to_instructors('helpRequest')
        .withText('My loop is not working correctly. It seems to run infinitely.')
        .withCode(`
for (let i = 0; i <= 10; i--) {
  console.log(i);
}`, 'javascript')
        .withLineNumber(25)
        .withUrgency('high')
        .withTags('loops', 'javascript', 'infinite-loop', 'debugging')
        .markAsImportant()
        .withData({
          studentId: 'student-123',
          lesson: 'javascript-basics',
          attemptCount: 3,
          timeSpent: 1800 // 30 minutes
        })
        .send();

      expect(client.ws.sentMessages).toHaveLength(1);
      const sentMessage = JSON.parse(client.ws.sentMessages[0]);
      
      expect(sentMessage).toMatchObject({
        type: 'broadcast_to_instructors',
        context: 'question',
        content: {
          name: 'helpRequest',
          text: 'My loop is not working correctly. It seems to run infinitely.',
          code_snippet: expect.stringContaining('for (let i = 0; i <= 10; i--)'),
          language: 'javascript',
          line_number: 25,
          urgency: 'high',
          tags: ['loops', 'javascript', 'infinite-loop', 'debugging'],
          important: true,
          studentId: 'student-123',
          lesson: 'javascript-basics',
          attemptCount: 3,
          timeSpent: 1800
        }
      });
    });

    test('should build instructor announcement', () => {
      const instructorClient = new SwitchboardClient({
        userId: 'instructor-1',
        role: 'instructor',
        wsUrl: 'ws://localhost:8080/ws'
      });
      
      instructorClient.connectionState = 'connected';
      instructorClient.ws = new MockWebSocket('ws://localhost:8080/ws');
      instructorClient.sessionActive = true;

      instructorClient.broadcast_to_students('announcement')
        .withText('Please submit your assignments by 5 PM today.')
        .markAsImportant()
        .withUrgency('medium')
        .withTags('deadline', 'assignment')
        .withData({
          deadline: '2023-12-15T17:00:00Z',
          assignmentId: 'hw-week-12'
        })
        .send();

      expect(instructorClient.ws.sentMessages).toHaveLength(1);
      const sentMessage = JSON.parse(instructorClient.ws.sentMessages[0]);
      
      expect(sentMessage).toMatchObject({
        type: 'broadcast_to_students',
        context: 'announcement',
        content: {
          name: 'announcement',
          text: 'Please submit your assignments by 5 PM today.',
          important: true,
          urgency: 'medium',
          tags: ['deadline', 'assignment'],
          deadline: '2023-12-15T17:00:00Z',
          assignmentId: 'hw-week-12'
        }
      });
    });

    test('should build code review response', () => {
      const instructorClient = new SwitchboardClient({
        userId: 'instructor-1',
        role: 'instructor',
        wsUrl: 'ws://localhost:8080/ws'
      });
      
      instructorClient.connectionState = 'connected';
      instructorClient.ws = new MockWebSocket('ws://localhost:8080/ws');
      instructorClient.sessionActive = true;

      instructorClient.direct_message('student-123', 'response')
        .withText('Good start! However, you have an infinite loop on line 25.')
        .withCode('for (let i = 0; i < 10; i++) { // Fixed: changed i-- to i++', 'javascript')
        .referencingMessage('msg-456')
        .withData({
          reviewType: 'code-review',
          fixes: ['infinite-loop'],
          grade: null
        })
        .send();

      expect(instructorClient.ws.sentMessages).toHaveLength(1);
      const sentMessage = JSON.parse(instructorClient.ws.sentMessages[0]);
      
      expect(sentMessage).toMatchObject({
        type: 'direct_message',
        to_user: 'student-123',
        context: 'response',
        content: {
          name: 'response',
          text: 'Good start! However, you have an infinite loop on line 25.',
          code_snippet: expect.stringContaining('for (let i = 0; i < 10; i++)'),
          language: 'javascript',
          reference_message_id: 'msg-456',
          reviewType: 'code-review',
          fixes: ['infinite-loop'],
          grade: null
        }
      });
    });
  });
});