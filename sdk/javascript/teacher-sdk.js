/**
 * Switchboard Teacher SDK - Single File
 * Zero dependencies, minimal boilerplate, focus on teaching logic
 */

(function(global) {
  'use strict';

  class SwitchboardTeacher {
    constructor(instructorId, options = {}) {
      this.instructorId = instructorId;
      this.serverUrl = options.serverUrl || 'http://localhost:8080';
      this.ws = null;
      this.currentSessionId = null;
      this.connected = false;
      this.reconnectAttempts = 0;
      this.maxReconnectAttempts = 5;
      
      // Event handlers - developers focus on these
      this.onStudentQuestion = null;
      this.onStudentResponse = null;
      this.onStudentAnalytics = null;
      this.onConnection = null;
      this.onError = null;
      this.onPresence = null;
      this.onSessionEvent = null;
    }

    // ==================== SIMPLE SESSION FLOW ====================
    
    /**
     * Create session and auto-connect - one line to start teaching
     */
    async createAndConnect(sessionName, studentIds) {
      try {
        const session = await this.createSession(sessionName, studentIds);
        
        // Only connect if we're not already connected
        if (!this.connected) {
          await this.connect();
        } else {
          // If already connected, the server auto-transition will handle moving us to the new session
          // Wait a moment for the transition to complete
          await new Promise(resolve => setTimeout(resolve, 200));
          console.log('🎯 DEBUG: Already connected, waiting for server transition to new session');
        }
        
        return session;
      } catch (error) {
        this._handleError(error);
        throw error;
      }
    }

    /**
     * End current session and disconnect
     */
    async endCurrentSession() {
      // SIMPLIFIED: Single endpoint to end active session - no need to get ID first
      try {
        console.log('🚩 DEBUG: Ending active session via DELETE /api/sessions/active');
        
        const response = await fetch(`${this.serverUrl}/api/sessions/active`, {
          method: 'DELETE'
        });
        
        // IDEMPOTENCY: Both 200 (success) and 404 (no session) are valid responses
        if (!response.ok && response.status !== 404) {
          const errorText = await response.text();
          console.error('Session end failed:', {
            status: response.status,
            statusText: response.statusText,
            body: errorText
          });
          throw new Error(`Failed to end session: ${response.statusText}`);
        }
        
        const responseData = await response.json();
        console.log(`✅ DEBUG: ${responseData.message || 'Session ended successfully'}`);
        
        // Always ensure client state is set to lobby after operation
        this.currentSessionId = 'lobby';
        
        // RACE CONDITION FIX: Add small delay to ensure session cleanup completes
        // This prevents conflicts when immediately creating a new session
        await new Promise(resolve => setTimeout(resolve, 100));
        
      } catch (error) {
        this._handleError(error);
        throw error;
      }
    }

    /**
     * Connect to lobby for presence awareness (no session needed)
     */
    async connectToLobby() {
      await this.connect('lobby');
    }

    // ==================== MESSAGE SENDING (SIMPLIFIED) ====================
    
    /**
     * Broadcast message to all students - most common action
     */
    announce(message, context = 'announcement') {
      const content = typeof message === 'string' ? { text: message } : message;
      return this._sendMessage('instructor_broadcast', context, content);
    }

    /**
     * Ask a specific student for something
     */
    ask(studentId, message, context = 'request') {
      const content = typeof message === 'string' ? { text: message } : message;
      return this._sendMessage('request', context, content, studentId);
    }

    /**
     * Respond to a student's question
     */
    respond(studentId, message, context = 'answer') {
      const content = typeof message === 'string' ? { text: message } : message;
      return this._sendMessage('inbox_response', context, content, studentId);
    }

    // ==================== COMMON TEACHING ACTIONS ====================

    /**
     * Start class with welcome message
     */
    welcomeStudents(message = 'Welcome to class! Please introduce yourselves.') {
      return this.announce(message, 'welcome');
    }

    /**
     * Schedule a break
     */
    scheduleBreak(minutes, message = null) {
      const content = {
        text: message || `We'll take a ${minutes}-minute break.`,
        break_duration: minutes * 60,
        resume_time: new Date(Date.now() + minutes * 60 * 1000).toLocaleTimeString()
      };
      return this.announce(content, 'break');
    }

    /**
     * Ask for code from a student
     */
    requestCode(studentId, prompt = 'Please share your current code') {
      return this.ask(studentId, { text: prompt, type: 'code' }, 'code');
    }

    /**
     * Give feedback to a student
     */
    giveFeedback(studentId, feedback, codeExample = null) {
      const content = { text: feedback };
      if (codeExample) content.code_example = codeExample;
      return this.respond(studentId, content, 'feedback');
    }

    /**
     * Emergency broadcast
     */
    emergency(message) {
      return this.announce({ text: message, priority: 'high' }, 'emergency');
    }

    // ==================== SESSION MANAGEMENT ====================

    async createSession(name, studentIds) {
      const response = await fetch(`${this.serverUrl}/api/sessions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: name,
          instructor_id: this.instructorId,
          student_ids: studentIds
        })
      });

      if (!response.ok) {
        const errorText = await response.text();
        console.error('Session creation failed:', {
          status: response.status,
          statusText: response.statusText,
          body: errorText
        });
        throw new Error(`Failed to create session: ${response.statusText}`);
      }

      const data = await response.json();
      return data.session;
    }

    async getSession(sessionId) {
      const response = await fetch(`${this.serverUrl}/api/sessions/${sessionId}`);
      if (!response.ok) {
        throw new Error(`Failed to get session: ${response.statusText}`);
      }
      return await response.json();
    }

    async getActiveSession() {
      const response = await fetch(`${this.serverUrl}/api/sessions/active`);
      if (!response.ok) {
        if (response.status === 404) {
          return null; // No active session
        }
        throw new Error(`Failed to get active session: ${response.statusText}`);
      }
      const data = await response.json();
      return data.session;
    }

    async endSession(sessionId) {
      const response = await fetch(`${this.serverUrl}/api/sessions/${sessionId}`, {
        method: 'DELETE'
      });
      if (!response.ok) {
        const errorText = await response.text();
        console.error('Session end failed:', {
          status: response.status,
          statusText: response.statusText,
          body: errorText
        });
        throw new Error(`Failed to end session: ${response.statusText}`);
      }
      console.log('✅ DEBUG: Session ended successfully');
    }

    // ==================== CONNECTION MANAGEMENT ====================

    async connect() {
      // Auto-assignment: don't specify session, let server decide
      const wsUrl = `ws://localhost:8080/ws?user_id=${this.instructorId}&role=instructor`;
      
      return new Promise((resolve, reject) => {
        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
          this.connected = true;
          this.reconnectAttempts = 0;
          // Server will tell us which session we're in via system message
          this.currentSessionId = 'lobby'; // Default until server confirms
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

    disconnect() {
      if (this.ws) {
        this.ws.close();
        this.ws = null;
      }
      this.connected = false;
      this.currentSessionId = null;
    }

    // ==================== STATE CHECKING ====================

    isInLobby() {
      return this.connected && (this.currentSessionId === 'lobby' || this.currentSessionId === null);
    }

    isInSession() {
      return this.connected && this.currentSessionId && this.currentSessionId !== 'lobby';
    }

    getSessionId() {
      return this.isInSession() ? this.currentSessionId : null;
    }

    // ==================== INTERNAL METHODS ====================

    _sendMessage(type, context, content, toUser = null) {
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

    _handleMessage(message) {
      // Handle system messages (lobby events, connection status, etc.)
      if (message.type === 'system') {
        this._handleSystemMessage(message);
        return;
      }

      // Route to user-defined handlers
      switch (message.type) {
        case 'instructor_inbox':
          if (this.onStudentQuestion) {
            this.onStudentQuestion({
              studentId: message.from_user,
              question: message.content.text || message.content,
              context: message.context,
              fullMessage: message
            });
          }
          break;

        case 'request_response':
          if (this.onStudentResponse) {
            this.onStudentResponse({
              studentId: message.from_user,
              response: message.content,
              context: message.context,
              fullMessage: message
            });
          }
          break;

        case 'analytics':
          if (this.onStudentAnalytics) {
            this.onStudentAnalytics({
              studentId: message.from_user,
              data: message.content,
              context: message.context,
              fullMessage: message
            });
          }
          break;
      }
    }

    _handleSystemMessage(message) {
      const event = message.context || message.content?.event;
      
      // Handle lobby system events
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

      if (event === 'session_started') {
        // Update current session when we join/create a session
        this.currentSessionId = message.content?.session_id;
        console.log(`🎆 DEBUG: Server assigned us to session: ${this.currentSessionId}`);
        if (this.onSessionEvent) {
          this.onSessionEvent({
            event: event,
            sessionId: message.content?.session_id,
            sessionName: message.content?.session_name,
            data: message.content
          });
        }
      } else if (event === 'session_transition') {
        // Update current session when we're transitioned to a new session
        this.currentSessionId = message.content?.session_id;
        console.log(`🔄 DEBUG: Server transitioned us to session: ${this.currentSessionId}`);
        if (this.onSessionEvent) {
          this.onSessionEvent({
            event: 'session_started', // Treat as session_started for UI purposes
            sessionId: message.content?.session_id,
            sessionName: message.content?.session_name,
            data: message.content
          });
        }
      } else if (event === 'lobby_assigned') {
        // Update to lobby when server assigns us to lobby
        this.currentSessionId = 'lobby';
        console.log('📝 DEBUG: Server assigned us to lobby');
      } else if (event === 'session_left') {
        // Return to lobby when session ends
        this.currentSessionId = 'lobby';
        if (this.onSessionEvent) {
          this.onSessionEvent({
            event: event,
            sessionId: message.content?.session_id,
            sessionName: message.content?.session_name,
            data: message.content
          });
        }
      }

      if (event === 'history_complete') {
        // Session history loaded - ready to start
        console.log('Session history loaded, ready to teach!');
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
          this.connect().catch(console.error);
        }
      }, delay);
    }
  }

  // Export to global scope for easy script tag usage
  global.SwitchboardTeacher = SwitchboardTeacher;

  // Support CommonJS/ES6 modules
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = SwitchboardTeacher;
  }

})(typeof window !== 'undefined' ? window : this);