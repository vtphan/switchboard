/**
 * Switchboard SDK - Browser-Compatible Single File
 * No external dependencies, works in any modern browser
 */

(function(global) {
  'use strict';

  // Simple EventEmitter implementation for browsers
  class EventEmitter {
    constructor() {
      this.events = {};
    }

    on(event, listener) {
      if (!this.events[event]) {
        this.events[event] = [];
      }
      this.events[event].push(listener);
    }

    emit(event, ...args) {
      if (!this.events[event]) return;
      this.events[event].forEach(listener => listener(...args));
    }

    off(event, listenerToRemove) {
      if (!this.events[event]) return;
      this.events[event] = this.events[event].filter(listener => listener !== listenerToRemove);
    }
  }

  // Base Switchboard Client
  class SwitchboardClient extends EventEmitter {
    constructor(userId, role) {
      super();
      this.userId = userId;
      this.role = role;
      this.baseUrl = 'http://localhost:8080';
      this.ws = null;
      this.currentSessionId = null;
      this.connected = false;
      this.reconnectAttempts = 0;
      this.maxReconnectAttempts = 5;
      this.messageHandlers = new Map();
    }

    async createSession(name, studentIds) {
      const response = await fetch(`${this.baseUrl}/api/sessions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: name,
          instructor_id: this.userId,
          student_ids: studentIds
        })
      });

      if (!response.ok) {
        throw new Error(`Failed to create session: ${response.statusText}`);
      }

      const data = await response.json();
      return data.session;
    }

    async getSession(sessionId) {
      const response = await fetch(`${this.baseUrl}/api/sessions/${sessionId}`);
      if (!response.ok) {
        throw new Error(`Failed to get session: ${response.statusText}`);
      }
      const data = await response.json();
      return data;
    }

    async listActiveSessions() {
      const response = await fetch(`${this.baseUrl}/api/sessions`);
      if (!response.ok) {
        throw new Error(`Failed to list sessions: ${response.statusText}`);
      }
      const data = await response.json();
      return data.sessions || [];
    }

    async endSession(sessionId) {
      const response = await fetch(`${this.baseUrl}/api/sessions/${sessionId}`, {
        method: 'DELETE'
      });
      if (!response.ok) {
        throw new Error(`Failed to end session: ${response.statusText}`);
      }
    }

    async connect(sessionId) {
      this.currentSessionId = sessionId;
      const wsUrl = `ws://localhost:8080/ws?user_id=${this.userId}&role=${this.role}&session_id=${sessionId}`;
      
      return new Promise((resolve, reject) => {
        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
          this.connected = true;
          this.reconnectAttempts = 0;
          this.emit('connection', true);
          resolve();
        };

        this.ws.onmessage = (event) => {
          try {
            const message = JSON.parse(event.data);
            this.handleMessage(message);
          } catch (error) {
            console.error('Failed to parse message:', error);
          }
        };

        this.ws.onerror = (error) => {
          this.emit('error', error);
          reject(error);
        };

        this.ws.onclose = () => {
          this.connected = false;
          this.emit('connection', false);
          this.attemptReconnect();
        };
      });
    }

    disconnect() {
      if (this.ws) {
        this.ws.close();
        this.ws = null;
      }
      this.connected = false;
      this.currentSessionId = null;
    }

    attemptReconnect() {
      if (this.reconnectAttempts >= this.maxReconnectAttempts) {
        console.error('Max reconnection attempts reached');
        return;
      }

      this.reconnectAttempts++;
      const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);

      setTimeout(() => {
        if (this.currentSessionId && !this.connected) {
          this.connect(this.currentSessionId).catch(console.error);
        }
      }, delay);
    }

    sendMessage(type, context, content, toUser = null) {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        throw new Error('Not connected to session');
      }

      const message = {
        type: type,
        context: context,
        content: content
      };

      if (toUser) {
        message.to_user = toUser;
      }

      this.ws.send(JSON.stringify(message));
    }

    handleMessage(message) {
      // Route to specific handlers based on message type
      this.emit('message', message);
      
      // Handle system messages
      if (message.type === 'system') {
        this.handleSystemMessage(message);
        return;
      }
      
      if (this.messageHandlers.has(message.type)) {
        this.messageHandlers.get(message.type)(message);
      }
    }

    handleSystemMessage(message) {
      if (message.content.event === 'history_complete') {
        this.emit('historyComplete');
      } else if (message.content.event === 'message_error') {
        this.emit('messageError', message.content);
      } else if (message.content.event === 'session_started') {
        this.emit('sessionStarted', message.content);
      } else if (message.content.event === 'session_ended') {
        this.emit('sessionEnded', message.content);
      }
      
      // Always emit generic system event
      this.emit('system', message);
    }

    setupEventHandlers(handlers) {
      // Override in subclasses
    }
  }

  // Teacher Client
  class SwitchboardTeacher extends SwitchboardClient {
    constructor(instructorId) {
      super(instructorId, 'instructor');
    }

    async createAndConnect(sessionName, studentIds) {
      const session = await this.createSession(sessionName, studentIds);
      await this.connect(session.id);
      return session;
    }

    async endCurrentSession() {
      if (this.currentSessionId) {
        await this.endSession(this.currentSessionId);
        this.disconnect();
      }
    }

    sendBroadcast(context, content) {
      return this.sendMessage('instructor_broadcast', context, content);
    }

    sendResponse(toStudentId, context, content) {
      return this.sendMessage('inbox_response', context, content, toStudentId);
    }

    sendRequest(toStudentId, context, content) {
      return this.sendMessage('request', context, content, toStudentId);
    }

    setupEventHandlers(handlers) {
      // Map internal message types to handler functions
      this.messageHandlers.set('instructor_inbox', (message) => {
        if (handlers.onStudentQuestion) {
          handlers.onStudentQuestion(message);
        }
      });

      this.messageHandlers.set('request_response', (message) => {
        if (handlers.onStudentResponse) {
          handlers.onStudentResponse(message);
        }
      });

      this.messageHandlers.set('analytics', (message) => {
        if (handlers.onStudentAnalytics) {
          handlers.onStudentAnalytics(message);
        }
      });

      if (handlers.onConnection) {
        this.on('connection', handlers.onConnection);
      }

      if (handlers.onError) {
        this.on('error', handlers.onError);
      }

      if (handlers.onSystem) {
        this.on('system', handlers.onSystem);
      }

      if (handlers.onHistoryComplete) {
        this.on('historyComplete', handlers.onHistoryComplete);
      }
    }

    // Convenience methods matching guideline patterns
    onStudentQuestion(handler) {
      this.messageHandlers.set('instructor_inbox', handler);
    }

    onStudentResponse(handler) {
      this.messageHandlers.set('request_response', handler);
    }

    onStudentAnalytics(handler) {
      this.messageHandlers.set('analytics', handler);
    }
  }

  // Student Client
  class SwitchboardStudent extends SwitchboardClient {
    constructor(studentId) {
      super(studentId, 'student');
    }

    async joinSession(sessionId) {
      await this.connect(sessionId);
    }

    sendQuestion(context, content) {
      return this.sendMessage('instructor_inbox', context, content);
    }

    sendResponse(context, content) {
      return this.sendMessage('request_response', context, content);
    }

    sendAnalytics(context, content) {
      return this.sendMessage('analytics', context, content);
    }

    setupEventHandlers(handlers) {
      // Map internal message types to handler functions
      this.messageHandlers.set('inbox_response', (message) => {
        if (handlers.onInstructorResponse) {
          handlers.onInstructorResponse(message);
        }
      });

      this.messageHandlers.set('request', (message) => {
        if (handlers.onInstructorRequest) {
          handlers.onInstructorRequest(message);
        }
      });

      this.messageHandlers.set('instructor_broadcast', (message) => {
        if (handlers.onInstructorBroadcast) {
          handlers.onInstructorBroadcast(message);
        }
      });

      if (handlers.onConnection) {
        this.on('connection', handlers.onConnection);
      }

      if (handlers.onError) {
        this.on('error', handlers.onError);
      }
    }
  }

  // Export to global scope
  global.SwitchboardSDK = {
    SwitchboardTeacher: SwitchboardTeacher,
    SwitchboardStudent: SwitchboardStudent,
    SwitchboardClient: SwitchboardClient
  };

  // Also support ES6 modules if available
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = global.SwitchboardSDK;
  }

})(typeof window !== 'undefined' ? window : this);