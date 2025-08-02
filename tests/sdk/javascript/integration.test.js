/**
 * Integration Tests for JavaScript SDK Client Examples
 * Tests the StudentApp and TeacherApp examples to ensure they work correctly with the SDK
 */

const SwitchboardClient = require('../../../sdk/javascript/switchboard-client.js');

// Mock DOM elements for testing
class MockElement {
  constructor(tagName = 'div') {
    this.tagName = tagName;
    this.value = '';
    this.textContent = '';
    this.innerHTML = '';
    this.disabled = false;
    this.style = {};
    this.className = '';
    this.children = [];
    this.eventListeners = {};
    this.scrollTop = 0;
    this.scrollHeight = 100;
  }

  addEventListener(event, handler) {
    if (!this.eventListeners[event]) {
      this.eventListeners[event] = [];
    }
    this.eventListeners[event].push(handler);
  }

  removeEventListener(event, handler) {
    if (this.eventListeners[event]) {
      const index = this.eventListeners[event].indexOf(handler);
      if (index > -1) {
        this.eventListeners[event].splice(index, 1);
      }
    }
  }

  dispatchEvent(event) {
    if (this.eventListeners[event.type]) {
      this.eventListeners[event.type].forEach(handler => handler(event));
    }
  }

  appendChild(child) {
    this.children.push(child);
    return child;
  }

  focus() {
    // Mock focus behavior
  }

  click() {
    this.dispatchEvent({ type: 'click' });
  }

  keypress(key, options = {}) {
    this.dispatchEvent({ 
      type: 'keypress', 
      key, 
      shiftKey: options.shiftKey || false,
      preventDefault: jest.fn()
    });
  }
}

// Mock global DOM
global.document = {
  getElementById: jest.fn((id) => new MockElement()),
  createElement: jest.fn((tagName) => new MockElement(tagName))
};

global.window = {
  addEventListener: jest.fn()
};

global.console = {
  log: jest.fn()
};

global.alert = jest.fn();

// Mock WebSocket
class MockWebSocket {
  constructor(url) {
    this.url = url;
    this.readyState = MockWebSocket.OPEN;
    this.sentMessages = [];
    this.onopen = null;
    this.onclose = null;
    this.onerror = null;
    this.onmessage = null;

    setTimeout(() => {
      if (this.onopen) this.onopen();
    }, 10);
  }

  send(data) {
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

  static get OPEN() { return 1; }
  static get CLOSED() { return 3; }
}

global.WebSocket = MockWebSocket;
global.fetch = jest.fn();

// Import the client applications
// Note: In a real test environment, you would import these from their actual files
// For this test, we'll create simplified versions that capture the essential functionality

class StudentApp {
  constructor() {
    this.client = null;
    this.isConnected = false;
    this.questionsAsked = 0;
    this.messagesReceived = 0;
    
    this.elements = {
      studentId: new MockElement('input'),
      connectBtn: new MockElement('button'),
      disconnectBtn: new MockElement('button'),
      statusDot: new MockElement('div'),
      statusText: new MockElement('span'),
      sessionInfo: new MockElement('div'),
      questionText: new MockElement('textarea'),
      codeSnippet: new MockElement('textarea'),
      askBtn: new MockElement('button'),
      messages: new MockElement('div'),
      questionsCount: new MockElement('span'),
      messagesCount: new MockElement('span')
    };

    this.setupEventListeners();
  }

  setupEventListeners() {
    this.elements.connectBtn.addEventListener('click', () => this.connect());
    this.elements.disconnectBtn.addEventListener('click', () => this.disconnect());
    this.elements.askBtn.addEventListener('click', () => this.askQuestion());
  }

  async connect() {
    const studentId = this.elements.studentId.value.trim();
    if (!studentId) {
      alert('Please enter a student ID');
      return;
    }

    this.client = new SwitchboardClient({
      userId: studentId,
      role: 'student',
      wsUrl: 'ws://localhost:8080/ws',
      hooks: {
        onConnected: () => {
          this.isConnected = true;
          this.updateStatus('connected', 'Connected');
        },
        onDisconnected: () => {
          this.isConnected = false;
          this.updateStatus('disconnected', 'Disconnected');
        },
        onBroadcastToStudents: (message) => {
          this.messagesReceived++;
          this.updateStats();
        },
        onSessionStarted: (session) => {
          this.elements.sessionInfo.textContent = `📚 Session: ${session.name}`;
        },
        onSessionEnded: (session) => {
          this.elements.sessionInfo.textContent = 'No active session';
        }
      }
    });

    await this.client.connect();
  }

  disconnect() {
    if (this.client) {
      this.client.disconnect();
      this.client = null;
    }
  }

  async askQuestion() {
    if (!this.client || !this.isConnected) {
      alert('Please connect first');
      return;
    }

    const questionText = this.elements.questionText.value.trim();
    const codeSnippet = this.elements.codeSnippet.value.trim();

    if (!questionText && !codeSnippet) {
      alert('Please enter a question or code snippet');
      return;
    }

    let messageBuilder = this.client.broadcast_to_instructors('helpRequest');
    
    if (questionText) {
      messageBuilder = messageBuilder.withText(questionText);
    }
    
    if (codeSnippet) {
      messageBuilder = messageBuilder.withCode(codeSnippet, 'javascript');
    }

    await messageBuilder.send();

    this.questionsAsked++;
    this.updateStats();
    this.elements.questionText.value = '';
    this.elements.codeSnippet.value = '';
  }

  updateStatus(state, text) {
    this.elements.statusDot.className = `status-dot ${state}`;
    this.elements.statusText.textContent = text;
  }

  updateStats() {
    this.elements.questionsCount.textContent = this.questionsAsked;
    this.elements.messagesCount.textContent = this.messagesReceived;
  }
}

class TeacherApp {
  constructor() {
    this.client = null;
    this.isConnected = false;
    this.activeSession = null;
    this.questionsReceived = 0;
    this.responsesSent = 0;
    this.selectedStudent = null;

    this.elements = {
      teacherId: new MockElement('input'),
      connectBtn: new MockElement('button'),
      disconnectBtn: new MockElement('button'),
      statusDot: new MockElement('div'),
      statusText: new MockElement('span'),
      sessionInfo: new MockElement('div'),
      sessionName: new MockElement('input'),
      startSessionBtn: new MockElement('button'),
      endSessionBtn: new MockElement('button'),
      announcementText: new MockElement('textarea'),
      announceBtn: new MockElement('button'),
      responseToStudent: new MockElement('input'),
      responseText: new MockElement('textarea'),
      responseBtn: new MockElement('button'),
      questionsCount: new MockElement('span'),
      responsesCount: new MockElement('span')
    };

    this.setupEventListeners();
  }

  setupEventListeners() {
    this.elements.connectBtn.addEventListener('click', () => this.connect());
    this.elements.disconnectBtn.addEventListener('click', () => this.disconnect());
    this.elements.startSessionBtn.addEventListener('click', () => this.startSession());
    this.elements.endSessionBtn.addEventListener('click', () => this.endSession());
    this.elements.announceBtn.addEventListener('click', () => this.sendAnnouncement());
    this.elements.responseBtn.addEventListener('click', () => this.sendResponse());
  }

  async connect() {
    const teacherId = this.elements.teacherId.value.trim();
    if (!teacherId) {
      alert('Please enter a teacher ID');
      return;
    }

    this.client = new SwitchboardClient({
      userId: teacherId,
      role: 'instructor',
      wsUrl: 'ws://localhost:8080/ws',
      hooks: {
        onConnected: () => {
          this.isConnected = true;
          this.updateStatus('connected', 'Connected');
        },
        onDisconnected: () => {
          this.isConnected = false;
          this.updateStatus('disconnected', 'Disconnected');
        },
        onBroadcastToInstructors: (message) => {
          this.questionsReceived++;
          this.updateStats();
        },
        onSessionStarted: (session) => {
          this.activeSession = session;
          this.elements.sessionInfo.textContent = `📚 Active Session: ${session.name}`;
        },
        onSessionEnded: (session) => {
          this.activeSession = null;
          this.elements.sessionInfo.textContent = 'No active session';
        }
      }
    });

    await this.client.connect();
  }

  disconnect() {
    if (this.client) {
      this.client.disconnect();
      this.client = null;
    }
  }

  async startSession() {
    if (!this.client || !this.isConnected) {
      alert('Please connect first');
      return;
    }

    const sessionName = this.elements.sessionName.value.trim();
    if (!sessionName) {
      alert('Please enter a session name');
      return;
    }

    const result = await this.client.startSession(sessionName);
    this.activeSession = result;
  }

  async endSession() {
    if (!this.client || !this.activeSession) {
      return;
    }

    await this.client.endSession();
    this.activeSession = null;
  }

  async sendAnnouncement() {
    if (!this.client || !this.isConnected) {
      alert('Please connect first');
      return;
    }

    const text = this.elements.announcementText.value.trim();
    if (!text) {
      alert('Please enter an announcement');
      return;
    }

    await this.client.broadcast_to_students('announcement')
      .withText(text)
      .send();

    this.elements.announcementText.value = '';
  }

  async sendResponse() {
    if (!this.client || !this.isConnected || !this.selectedStudent) {
      alert('Please select a student to respond to');
      return;
    }

    const responseText = this.elements.responseText.value.trim();
    if (!responseText) {
      alert('Please enter a response');
      return;
    }

    await this.client.direct_message(this.selectedStudent, 'response')
      .withText(responseText)
      .send();

    this.responsesSent++;
    this.updateStats();
    this.elements.responseText.value = '';
  }

  updateStatus(state, text) {
    this.elements.statusDot.className = `status-dot ${state}`;
    this.elements.statusText.textContent = text;
  }

  updateStats() {
    this.elements.questionsCount.textContent = this.questionsReceived;
    this.elements.responsesCount.textContent = this.responsesSent;
  }

  selectStudentForResponse(studentId) {
    this.selectedStudent = studentId;
    this.elements.responseToStudent.value = studentId;
  }
}

describe('Integration Tests - Client Examples', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    global.alert.mockClear();
    fetch.mockClear();
  });

  describe('StudentApp Integration', () => {
    let studentApp;

    beforeEach(() => {
      studentApp = new StudentApp();
    });

    test('should initialize with correct default state', () => {
      expect(studentApp.client).toBe(null);
      expect(studentApp.isConnected).toBe(false);
      expect(studentApp.questionsAsked).toBe(0);
      expect(studentApp.messagesReceived).toBe(0);
    });

    test('should handle connection flow', async () => {
      studentApp.elements.studentId.value = 'student-123';
      
      await studentApp.connect();

      expect(studentApp.client).toBeTruthy();
      expect(studentApp.client.userId).toBe('student-123');
      expect(studentApp.client.role).toBe('student');
      expect(studentApp.isConnected).toBe(true);
      expect(studentApp.elements.statusText.textContent).toBe('Connected');
    });

    test('should validate student ID before connecting', async () => {
      studentApp.elements.studentId.value = '';
      
      await studentApp.connect();

      expect(global.alert).toHaveBeenCalledWith('Please enter a student ID');
      expect(studentApp.client).toBe(null);
    });

    test('should handle disconnection', async () => {
      studentApp.elements.studentId.value = 'student-123';
      await studentApp.connect();

      studentApp.disconnect();

      expect(studentApp.client).toBe(null);
      expect(studentApp.isConnected).toBe(false);
      expect(studentApp.elements.statusText.textContent).toBe('Disconnected');
    });

    test('should ask question with text only', async () => {
      studentApp.elements.studentId.value = 'student-123';
      await studentApp.connect();
      studentApp.client.sessionActive = true;

      studentApp.elements.questionText.value = 'I need help with loops';
      await studentApp.askQuestion();

      expect(studentApp.client.ws.sentMessages).toHaveLength(1);
      const sentMessage = JSON.parse(studentApp.client.ws.sentMessages[0]);
      expect(sentMessage.content.text).toBe('I need help with loops');
      expect(studentApp.questionsAsked).toBe(1);
      expect(studentApp.elements.questionText.value).toBe('');
    });

    test('should ask question with code only', async () => {
      studentApp.elements.studentId.value = 'student-123';
      await studentApp.connect();
      studentApp.client.sessionActive = true;

      studentApp.elements.codeSnippet.value = 'for(let i=0; i<10; i++) console.log(i);';
      await studentApp.askQuestion();

      expect(studentApp.client.ws.sentMessages).toHaveLength(1);
      const sentMessage = JSON.parse(studentApp.client.ws.sentMessages[0]);
      expect(sentMessage.content.code_snippet).toBe('for(let i=0; i<10; i++) console.log(i);');
      expect(sentMessage.content.language).toBe('javascript');
    });

    test('should ask question with both text and code', async () => {
      studentApp.elements.studentId.value = 'student-123';
      await studentApp.connect();
      studentApp.client.sessionActive = true;

      studentApp.elements.questionText.value = 'This loop is not working';
      studentApp.elements.codeSnippet.value = 'for(let i=0; i<=10; i--) console.log(i);';
      await studentApp.askQuestion();

      expect(studentApp.client.ws.sentMessages).toHaveLength(1);
      const sentMessage = JSON.parse(studentApp.client.ws.sentMessages[0]);
      expect(sentMessage.content.text).toBe('This loop is not working');
      expect(sentMessage.content.code_snippet).toBe('for(let i=0; i<=10; i--) console.log(i);');
    });

    test('should validate question content before sending', async () => {
      studentApp.elements.studentId.value = 'student-123';
      await studentApp.connect();

      await studentApp.askQuestion();

      expect(global.alert).toHaveBeenCalledWith('Please enter a question or code snippet');
      expect(studentApp.client.ws.sentMessages).toHaveLength(0);
    });

    test('should handle session events', async () => {
      studentApp.elements.studentId.value = 'student-123';
      await studentApp.connect();

      // Simulate session started
      const sessionStartedMessage = {
        type: 'system',
        content: {
          event: 'session_started',
          session_id: 'session-123',
          session_name: 'Math Class',
          started_by: 'teacher-1'
        }
      };

      studentApp.client.ws.simulateMessage(sessionStartedMessage);
      expect(studentApp.elements.sessionInfo.textContent).toBe('📚 Session: Math Class');

      // Simulate session ended
      const sessionEndedMessage = {
        type: 'system',
        content: { event: 'session_ended' }
      };

      studentApp.client.ws.simulateMessage(sessionEndedMessage);
      expect(studentApp.elements.sessionInfo.textContent).toBe('No active session');
    });

    test('should handle broadcast messages from instructors', async () => {
      studentApp.elements.studentId.value = 'student-123';
      await studentApp.connect();

      const broadcastMessage = {
        type: 'broadcast_to_students',
        from_user: 'teacher-1',
        content: { text: 'Please submit your homework' },
        timestamp: new Date().toISOString()
      };

      studentApp.client.ws.simulateMessage(broadcastMessage);

      expect(studentApp.messagesReceived).toBe(1);
      expect(studentApp.elements.messagesCount.textContent).toBe('1');
    });
  });

  describe('TeacherApp Integration', () => {
    let teacherApp;

    beforeEach(() => {
      teacherApp = new TeacherApp();
    });

    test('should initialize with correct default state', () => {
      expect(teacherApp.client).toBe(null);
      expect(teacherApp.isConnected).toBe(false);
      expect(teacherApp.activeSession).toBe(null);
      expect(teacherApp.questionsReceived).toBe(0);
      expect(teacherApp.responsesSent).toBe(0);
    });

    test('should handle connection flow', async () => {
      teacherApp.elements.teacherId.value = 'teacher-456';
      
      await teacherApp.connect();

      expect(teacherApp.client).toBeTruthy();
      expect(teacherApp.client.userId).toBe('teacher-456');
      expect(teacherApp.client.role).toBe('instructor');
      expect(teacherApp.isConnected).toBe(true);
      expect(teacherApp.elements.statusText.textContent).toBe('Connected');
    });

    test('should validate teacher ID before connecting', async () => {
      teacherApp.elements.teacherId.value = '';
      
      await teacherApp.connect();

      expect(global.alert).toHaveBeenCalledWith('Please enter a teacher ID');
      expect(teacherApp.client).toBe(null);
    });

    test('should start session successfully', async () => {
      teacherApp.elements.teacherId.value = 'teacher-456';
      await teacherApp.connect();

      fetch.mockResolvedValueOnce({
        ok: true,
        json: jest.fn().mockResolvedValue({
          id: 'session-789',
          name: 'Physics Class',
          instructor_id: 'teacher-456'
        })
      });

      teacherApp.elements.sessionName.value = 'Physics Class';
      await teacherApp.startSession();

      expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/session/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: 'Physics Class',
          instructor_id: 'teacher-456'
        })
      });

      expect(teacherApp.activeSession).toMatchObject({
        id: 'session-789',
        name: 'Physics Class'
      });
    });

    test('should validate session name before starting', async () => {
      teacherApp.elements.teacherId.value = 'teacher-456';
      await teacherApp.connect();

      teacherApp.elements.sessionName.value = '';
      await teacherApp.startSession();

      expect(global.alert).toHaveBeenCalledWith('Please enter a session name');
      expect(fetch).not.toHaveBeenCalled();
    });

    test('should end session successfully', async () => {
      teacherApp.elements.teacherId.value = 'teacher-456';
      await teacherApp.connect();
      teacherApp.activeSession = { id: 'session-789', name: 'Physics Class' };

      fetch.mockResolvedValueOnce({
        ok: true,
        json: jest.fn().mockResolvedValue({ success: true })
      });

      await teacherApp.endSession();

      expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/session/end', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          instructor_id: 'teacher-456'
        })
      });

      expect(teacherApp.activeSession).toBe(null);
    });

    test('should send announcement successfully', async () => {
      teacherApp.elements.teacherId.value = 'teacher-456';
      await teacherApp.connect();
      teacherApp.client.sessionActive = true;

      teacherApp.elements.announcementText.value = 'Class starts in 5 minutes';
      await teacherApp.sendAnnouncement();

      expect(teacherApp.client.ws.sentMessages).toHaveLength(1);
      const sentMessage = JSON.parse(teacherApp.client.ws.sentMessages[0]);
      expect(sentMessage.type).toBe('broadcast_to_students');
      expect(sentMessage.content.text).toBe('Class starts in 5 minutes');
      expect(teacherApp.elements.announcementText.value).toBe('');
    });

    test('should validate announcement text', async () => {
      teacherApp.elements.teacherId.value = 'teacher-456';
      await teacherApp.connect();

      teacherApp.elements.announcementText.value = '';
      await teacherApp.sendAnnouncement();

      expect(global.alert).toHaveBeenCalledWith('Please enter an announcement');
      expect(teacherApp.client.ws.sentMessages).toHaveLength(0);
    });

    test('should send direct response to student', async () => {
      teacherApp.elements.teacherId.value = 'teacher-456';
      await teacherApp.connect();
      teacherApp.client.sessionActive = true;

      teacherApp.selectStudentForResponse('student-123');
      teacherApp.elements.responseText.value = 'Good question! The answer is...';
      await teacherApp.sendResponse();

      expect(teacherApp.client.ws.sentMessages).toHaveLength(1);
      const sentMessage = JSON.parse(teacherApp.client.ws.sentMessages[0]);
      expect(sentMessage.type).toBe('direct_message');
      expect(sentMessage.to_user).toBe('student-123');
      expect(sentMessage.content.text).toBe('Good question! The answer is...');
      expect(teacherApp.responsesSent).toBe(1);
      expect(teacherApp.elements.responseText.value).toBe('');
    });

    test('should validate student selection for response', async () => {
      teacherApp.elements.teacherId.value = 'teacher-456';
      await teacherApp.connect();

      teacherApp.elements.responseText.value = 'Some response';
      await teacherApp.sendResponse();

      expect(global.alert).toHaveBeenCalledWith('Please select a student to respond to');
      expect(teacherApp.client.ws.sentMessages).toHaveLength(0);
    });

    test('should handle student questions', async () => {
      teacherApp.elements.teacherId.value = 'teacher-456';
      await teacherApp.connect();

      const questionMessage = {
        type: 'broadcast_to_instructors',
        from_user: 'student-123',
        content: { 
          name: 'helpRequest',
          text: 'I dont understand this concept'
        },
        timestamp: new Date().toISOString()
      };

      teacherApp.client.ws.simulateMessage(questionMessage);

      expect(teacherApp.questionsReceived).toBe(1);
      expect(teacherApp.elements.questionsCount.textContent).toBe('1');
    });

    test('should handle session events', async () => {
      teacherApp.elements.teacherId.value = 'teacher-456';
      await teacherApp.connect();

      // Simulate session started event
      const sessionStartedMessage = {
        type: 'system',
        content: {
          event: 'session_started',
          session_id: 'session-456',
          session_name: 'Biology Class',
          started_by: 'teacher-456'
        }
      };

      teacherApp.client.ws.simulateMessage(sessionStartedMessage);
      
      expect(teacherApp.activeSession).toMatchObject({
        id: 'session-456',
        name: 'Biology Class'
      });
      expect(teacherApp.elements.sessionInfo.textContent).toBe('📚 Active Session: Biology Class');
    });
  });

  describe('Student-Teacher Interaction Flow', () => {
    let studentApp, teacherApp;

    beforeEach(() => {
      studentApp = new StudentApp();
      teacherApp = new TeacherApp();
    });

    test('should simulate complete interaction workflow', async () => {
      // Setup both clients
      studentApp.elements.studentId.value = 'student-999';
      teacherApp.elements.teacherId.value = 'teacher-999';

      await Promise.all([
        studentApp.connect(),
        teacherApp.connect()
      ]);

      // Teacher starts session
      fetch.mockResolvedValueOnce({
        ok: true,
        json: jest.fn().mockResolvedValue({
          id: 'session-workflow',
          name: 'Integration Test',
          instructor_id: 'teacher-999'
        })
      });

      teacherApp.elements.sessionName.value = 'Integration Test';
      await teacherApp.startSession();

      // Simulate session started for both clients
      const sessionMessage = {
        type: 'system',
        content: {
          event: 'session_started',
          session_id: 'session-workflow',
          session_name: 'Integration Test',
          started_by: 'teacher-999'
        }
      };

      studentApp.client.ws.simulateMessage(sessionMessage);
      teacherApp.client.ws.simulateMessage(sessionMessage);

      // Student asks question
      studentApp.client.sessionActive = true;
      studentApp.elements.questionText.value = 'How do I fix this loop?';
      studentApp.elements.codeSnippet.value = 'for(let i=0; i<=10; i--) console.log(i);';
      await studentApp.askQuestion();

      // Simulate question reaching teacher
      const questionMessage = {
        type: 'broadcast_to_instructors',
        from_user: 'student-999',
        content: {
          name: 'helpRequest',
          text: 'How do I fix this loop?',
          code_snippet: 'for(let i=0; i<=10; i--) console.log(i);',
          language: 'javascript'
        },
        timestamp: new Date().toISOString()
      };

      teacherApp.client.ws.simulateMessage(questionMessage);

      // Teacher responds
      teacherApp.client.sessionActive = true;
      teacherApp.selectStudentForResponse('student-999');
      teacherApp.elements.responseText.value = 'Change i-- to i++ to fix the infinite loop';
      await teacherApp.sendResponse();

      // Simulate response reaching student
      const responseMessage = {
        type: 'direct_message',
        from_user: 'teacher-999',
        to_user: 'student-999',
        content: {
          name: 'response',
          text: 'Change i-- to i++ to fix the infinite loop'
        },
        timestamp: new Date().toISOString()
      };

      studentApp.client.ws.simulateMessage(responseMessage);

      // Teacher makes announcement
      teacherApp.elements.announcementText.value = 'Remember to submit homework by Friday';
      await teacherApp.sendAnnouncement();

      // Simulate announcement reaching student
      const announcementMessage = {
        type: 'broadcast_to_students',
        from_user: 'teacher-999',
        content: {
          name: 'announcement',
          text: 'Remember to submit homework by Friday'
        },
        timestamp: new Date().toISOString()
      };

      studentApp.client.ws.simulateMessage(announcementMessage);

      // Verify the complete interaction
      expect(studentApp.questionsAsked).toBe(1);
      expect(studentApp.messagesReceived).toBe(2); // Response + announcement
      expect(teacherApp.questionsReceived).toBe(1);
      expect(teacherApp.responsesSent).toBe(1);

      // Verify messages were sent correctly
      expect(studentApp.client.ws.sentMessages).toHaveLength(1); // Question
      expect(teacherApp.client.ws.sentMessages).toHaveLength(2); // Response + announcement

      const studentMessage = JSON.parse(studentApp.client.ws.sentMessages[0]);
      expect(studentMessage.type).toBe('broadcast_to_instructors');
      expect(studentMessage.content.text).toBe('How do I fix this loop?');

      const teacherMessages = teacherApp.client.ws.sentMessages.map(msg => JSON.parse(msg));
      expect(teacherMessages[0].type).toBe('direct_message');
      expect(teacherMessages[0].to_user).toBe('student-999');
      expect(teacherMessages[1].type).toBe('broadcast_to_students');
    });
  });
});