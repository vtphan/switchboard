/**
 * Switchboard Student SDK - Single File
 * Zero dependencies, minimal boilerplate, focus on learning logic
 */

(function(global) {
  'use strict';

  class SwitchboardStudent {
    constructor(studentId, options = {}) {
      this.studentId = studentId;
      this.serverUrl = options.serverUrl || 'http://localhost:8080';
      this.ws = null;
      this.currentSessionId = null;
      this.connected = false;
      this.reconnectAttempts = 0;
      this.maxReconnectAttempts = 5;
      
      // Event handlers - developers focus on these
      this.onTeacherMessage = null;
      this.onTeacherRequest = null;
      this.onTeacherBroadcast = null;
      this.onConnection = null;
      this.onError = null;
      this.onPresence = null;
      this.onSessionEvent = null;
    }

    // ==================== SIMPLE LEARNING FLOW ====================
    
    /**
     * Connect and become available for learning - one line to start
     * Students connect to lobby and become visible to teachers
     */
    async connect() {
      try {
        await this._connectToLobby();
        return { connected: true, status: 'available' };
      } catch (error) {
        this._handleError(error);
        throw error;
      }
    }

    /**
     * Disconnect from the system
     */
    disconnect() {
      this._disconnect();
    }

    /**
     * Check current status - lobby, assigned to session, etc.
     */
    getStatus() {
      return {
        connected: this.connected,
        currentSession: this.currentSessionId,
        studentId: this.studentId
      };
    }

    // ==================== MESSAGE SENDING (SIMPLIFIED) ====================
    
    /**
     * Ask teacher a question - most common student action
     */
    askQuestion(question, context = 'question') {
      const content = typeof question === 'string' ? { text: question } : question;
      return this._sendMessage('instructor_inbox', context, content);
    }

    /**
     * Respond to teacher's request
     */
    respond(response, context = 'response') {
      const content = typeof response === 'string' ? { text: response } : response;
      return this._sendMessage('request_response', context, content);
    }

    /**
     * Send analytics/progress data
     */
    sendProgress(data, context = 'progress') {
      const content = typeof data === 'object' ? data : { value: data };
      return this._sendMessage('analytics', context, content);
    }

    // ==================== COMMON STUDENT ACTIONS ====================

    /**
     * Ask for help with something specific
     */
    needHelp(problem, codeContext = null) {
      const content = { text: `I need help: ${problem}` };
      if (codeContext) content.code_context = codeContext;
      return this.askQuestion(content, 'help');
    }

    /**
     * Share code with teacher
     */
    shareCode(code, explanation = null) {
      const content = { code: code };
      if (explanation) content.explanation = explanation;
      return this.respond(content, 'code');
    }

    /**
     * Report completion of a task
     */
    taskCompleted(taskName, details = null) {
      const content = { 
        text: `Completed: ${taskName}`,
        task: taskName,
        completion_time: new Date().toISOString()
      };
      if (details) content.details = details;
      return this.sendProgress(content, 'completion');
    }

    /**
     * Report an error or problem
     */
    reportError(error, code = null) {
      const content = { 
        text: `Error encountered: ${error}`,
        error: error,
        timestamp: new Date().toISOString()
      };
      if (code) content.code_context = code;
      return this.askQuestion(content, 'error');
    }

    /**
     * Send learning analytics
     */
    updateProgress(completionPercent, timeSpent, errorsCount = 0, topic = null) {
      return this.sendProgress({
        completion_percentage: completionPercent,
        time_spent: timeSpent,
        errors_encountered: errorsCount,
        topic: topic,
        timestamp: new Date().toISOString()
      });
    }

    /**
     * Simple acknowledgment
     */
    acknowledge(message = 'Got it!') {
      return this.respond({ text: message }, 'acknowledgment');
    }

    // ==================== SESSION DISCOVERY ====================

    async findAvailableSessions() {
      const response = await fetch(`${this.serverUrl}/api/sessions`);
      if (!response.ok) {
        throw new Error(`Failed to list sessions: ${response.statusText}`);
      }
      const data = await response.json();
      return data.sessions || [];
    }

    async getSessionInfo(sessionId) {
      const response = await fetch(`${this.serverUrl}/api/sessions/${sessionId}`);
      if (!response.ok) {
        throw new Error(`Failed to get session: ${response.statusText}`);
      }
      return await response.json();
    }

    // ==================== CONNECTION MANAGEMENT ====================

    async _connectToLobby() {
      // Connect without session_id - server will auto-assign to active session or lobby
      const wsUrl = `ws://localhost:8080/ws?user_id=${this.studentId}&role=student`;
      
      return new Promise((resolve, reject) => {
        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
          this.connected = true;
          this.reconnectAttempts = 0;
          console.log('Connected to Switchboard - server will assign you automatically!');
          this._handleConnection(true);
          resolve();
        };

        this.ws.onmessage = (event) => {
          try {
            const message = JSON.parse(event.data);
            this._handleMessage(message);
          } catch (error) {
            console.error('Failed to parse message:', error);
          }
        };

        this.ws.onerror = (error) => {
          this._handleError(error);
          reject(error);
        };

        this.ws.onclose = () => {
          this.connected = false;
          this._handleConnection(false);
          this._attemptReconnect();
        };
      });
    }

    _disconnect() {
      if (this.ws) {
        this.ws.close();
        this.ws = null;
      }
      this.connected = false;
      this.currentSessionId = null;
    }

    // ==================== INTERNAL METHODS ====================

    _sendMessage(type, context, content) {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        throw new Error('Not connected to session');
      }

      const message = {
        type: type,
        context: context,
        content: content
      };

      this.ws.send(JSON.stringify(message));
    }

    _handleMessage(message) {
      // Handle system messages (lobby events, connection status, etc.)
      if (message.type === 'system') {
        this._handleSystemMessage(message);
        return;
      }

      // Route to user-defined handlers
      switch (message.type) {
        case 'inbox_response':
          if (this.onTeacherMessage) {
            this.onTeacherMessage({
              message: message.content.text || message.content,
              context: message.context,
              fullMessage: message
            });
          }
          break;

        case 'request':
          if (this.onTeacherRequest) {
            this.onTeacherRequest({
              request: message.content.text || message.content,
              context: message.context,
              instructions: message.content.instructions,
              fullMessage: message
            });
          }
          break;

        case 'instructor_broadcast':
          if (this.onTeacherBroadcast) {
            this.onTeacherBroadcast({
              message: message.content.text || message.content,
              context: message.context,
              data: message.content,
              fullMessage: message
            });
          }
          break;
      }
    }

    _handleSystemMessage(message) {
      const event = message.context || message.content?.event;
      
      // Handle session assignment - server auto-assigned us upon connection
      if (event === 'session_started' && message.content?.student_ids?.includes(this.studentId)) {
        console.log(`Auto-assigned to session: ${message.content.session_name}`);
        this.currentSessionId = message.content.session_id;
        
        if (this.onSessionEvent) {
          this.onSessionEvent({
            event: 'session_assigned',
            sessionId: message.content.session_id,
            sessionName: message.content.session_name,
            instructorId: message.content.instructor_id,
            data: message.content
          });
        }
      }

      // Handle being moved back to lobby when session ends
      if (event === 'session_left') {
        console.log(`Session ended, moved to lobby`);
        this.currentSessionId = 'lobby';
        
        if (this.onSessionEvent) {
          this.onSessionEvent({
            event: 'moved_to_lobby',
            reason: message.content?.reason || 'session_ended',
            data: message.content
          });
        }
      }

      // Handle lobby presence events (see other students/teachers online)
      if (event === 'user_connected' || event === 'user_disconnected' || event === 'presence_update') {
        if (this.onPresence) {
          this.onPresence({
            event: event,
            userId: message.content?.user_id,
            role: message.content?.role,
            sessionId: message.content?.session_id,
            timestamp: message.content?.timestamp || message.timestamp
          });
        }
      }

      if (event === 'history_complete') {
        // History loaded - we know where we are now (lobby or session)
        const status = this.currentSessionId === 'lobby' ? 'in lobby' : `in session ${this.currentSessionId}`;
        console.log(`Ready to learn! Currently ${status}`);
      }
    }

    _handleConnection(connected) {
      if (this.onConnection) {
        this.onConnection(connected);
      }
    }

    _handleError(error) {
      if (this.onError) {
        this.onError(error);
      } else {
        console.error('Switchboard Error:', error);
      }
    }

    _attemptReconnect() {
      if (this.reconnectAttempts >= this.maxReconnectAttempts) {
        console.error('Max reconnection attempts reached');
        return;
      }

      this.reconnectAttempts++;
      const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);

      setTimeout(() => {
        if (!this.connected) {
          console.log(`Attempting to reconnect (${this.reconnectAttempts}/${this.maxReconnectAttempts})...`);
          this._connectToLobby().catch(console.error);
        }
      }, delay);
    }
  }

  // Export to global scope for easy script tag usage
  global.SwitchboardStudent = SwitchboardStudent;

  // Support CommonJS/ES6 modules
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = SwitchboardStudent;
  }

})(typeof window !== 'undefined' ? window : this);