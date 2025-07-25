#!/usr/bin/env node

// Teacher Switchboard Client for AI Programming Mentorship Demo
// Connects to Switchboard instead of creating custom WebSocket server

class TeacherSwitchboardClient {
  constructor(switchboardUrl = 'http://localhost:8080', instructorId = 'teacher_001') {
    this.switchboardUrl = switchboardUrl;
    this.instructorId = instructorId;
    this.ws = null;
    this.currentSessionId = null;
    this.connected = false;
    this.messageHandlers = new Map();
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
    
    // AI Expert IDs that will be enrolled in sessions
    this.expertIds = [
      'technical_expert',
      'emotional_support_coach', 
      'debugging_guru',
      'learning_coach',
      'architecture_expert'
    ];
  }

  // Session Management - Replace custom WebSocket server entirely
  async createSession(sessionName = 'AI Programming Mentorship Session') {
    try {
      const response = await fetch(`${this.switchboardUrl}/api/sessions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: sessionName,
          instructor_id: this.instructorId,
          student_ids: this.expertIds
        })
      });

      if (!response.ok) {
        throw new Error(`Failed to create session: ${response.statusText}`);
      }

      const data = await response.json();
      console.log(`✅ Session created: ${data.session.id}`);
      return data.session;
    } catch (error) {
      console.error('❌ Failed to create session:', error);
      throw error;
    }
  }

  async listActiveSessions() {
    try {
      const response = await fetch(`${this.switchboardUrl}/api/sessions`);
      if (!response.ok) {
        throw new Error(`Failed to list sessions: ${response.statusText}`);
      }
      
      const data = await response.json();
      return data.sessions.filter(s => s.status === 'active');
    } catch (error) {
      console.error('❌ Failed to list sessions:', error);
      throw error;
    }
  }

  async endSession(sessionId = this.currentSessionId) {
    if (!sessionId) return;
    
    try {
      const response = await fetch(`${this.switchboardUrl}/api/sessions/${sessionId}`, {
        method: 'DELETE'
      });

      if (response.ok) {
        console.log(`✅ Session ${sessionId} ended`);
      }
    } catch (error) {
      console.error('❌ Failed to end session:', error);
    }
  }

  // WebSocket Connection - Simple client instead of server
  async connectToSession(sessionId) {
    try {
      const wsUrl = `ws://localhost:8080/ws?user_id=${this.instructorId}&role=instructor&session_id=${sessionId}`;
      
      this.ws = new WebSocket(wsUrl);
      this.currentSessionId = sessionId;

      return new Promise((resolve, reject) => {
        this.ws.onopen = () => {
          this.connected = true;
          this.reconnectAttempts = 0;
          console.log(`🔗 Connected to session: ${sessionId}`);
          resolve();
        };

        this.ws.onmessage = (event) => {
          try {
            const message = JSON.parse(event.data);
            this.handleMessage(message);
          } catch (error) {
            console.error('❌ Failed to parse message:', error);
          }
        };

        this.ws.onclose = (event) => {
          this.connected = false;
          console.log('⚠️ Connection closed');
          
          if (event.code !== 1000 && this.reconnectAttempts < this.maxReconnectAttempts) {
            this.attemptReconnection();
          }
        };

        this.ws.onerror = (error) => {
          console.error('❌ WebSocket error:', error);
          if (!this.connected) {
            reject(error);
          }
        };

        // Connection timeout
        setTimeout(() => {
          if (!this.connected) {
            reject(new Error('Connection timeout'));
          }
        }, 10000);
      });
    } catch (error) {
      console.error('❌ Failed to connect to session:', error);
      throw error;
    }
  }

  async attemptReconnection() {
    this.reconnectAttempts++;
    const delay = Math.pow(2, this.reconnectAttempts) * 1000; // Exponential backoff
    
    console.log(`🔄 Reconnecting in ${delay/1000}s (attempt ${this.reconnectAttempts})`);
    
    setTimeout(async () => {
      try {
        await this.connectToSession(this.currentSessionId);
        this.onReconnected?.();
      } catch (error) {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
          this.attemptReconnection();
        } else {
          this.onReconnectionFailed?.();
        }
      }
    }, delay);
  }

  // Message Handling - Simplified to handle Switchboard message types
  handleMessage(message) {
    switch (message.type) {
      case 'instructor_inbox':
        if (message.context === 'hint') {
          this.handleHintReceived(message);
        } else if (message.context === 'error') {
          this.handleExpertError(message);
        }
        break;
        
      case 'analytics':
        this.handleExpertAnalytics(message);
        break;
        
      case 'system':
        this.handleSystemMessage(message);
        break;
        
      default:
        console.log('📨 Received message:', message);
    }

    // Trigger any registered message handlers
    const handler = this.messageHandlers.get(message.type);
    if (handler) {
      handler(message);
    }
  }

  handleHintReceived(message) {
    const hint = {
      type: 'hint_received',
      expert: {
        name: message.content.expert?.name || message.from_user,
        expertise: message.content.expert?.expertise || 'General',
      },
      hint: message.content.hint,
      timestamp: message.timestamp,
      problemContext: message.content.problemContext
    };

    console.log(`💡 Hint from ${hint.expert.name}: ${hint.hint.substring(0, 50)}...`);
    
    // Notify web client
    this.onHintReceived?.(hint);
  }

  handleExpertError(message) {
    console.log(`⚠️ Expert error from ${message.from_user}: ${message.content.error}`);
    this.onExpertError?.(message);
  }

  handleExpertAnalytics(message) {
    if (message.context === 'connection') {
      console.log(`📊 Expert connection: ${message.from_user} - ${message.content.event}`);
      this.onExpertConnection?.(message);
    }
  }

  handleSystemMessage(message) {
    const event = message.content.event;
    
    switch (event) {
      case 'history_complete':
        console.log('📚 Message history loaded');
        this.onHistoryLoaded?.();
        break;
        
      case 'message_error':
        console.error('❌ Message error:', message.content.error);
        break;
    }
  }

  // Message Sending - Single simple broadcast instead of complex loops
  broadcastProblem(problemData) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('Not connected to session');
    }

    const message = {
      type: 'instructor_broadcast',
      context: 'problem',
      content: {
        problem: problemData.problem,
        code: problemData.code,
        timeOnTask: problemData.timeOnTask,
        remainingTime: problemData.remainingTime, 
        frustrationLevel: problemData.frustrationLevel,
        timestamp: new Date().toISOString()
      }
    };

    this.ws.send(JSON.stringify(message));
    
    console.log(`📢 Problem broadcasted to ${this.expertIds.length} experts`);
    
    // Notify web client
    this.onProblemBroadcasted?.(problemData);
  }

  sendDirectResponse(toExpertId, context, content) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('Not connected to session');
    }

    const message = {
      type: 'inbox_response',
      context: context,
      content: content,
      to_user: toExpertId
    };

    this.ws.send(JSON.stringify(message));
  }

  sendStatusUpdate(statusMessage) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      return;
    }

    const message = {
      type: 'instructor_broadcast',
      context: 'status',
      content: {
        message: statusMessage,
        timestamp: new Date().toISOString()
      }
    };

    this.ws.send(JSON.stringify(message));
  }

  // Event Handlers - For web client integration
  onMessage(handler) {
    this.messageHandlers.set('*', handler);
  }

  onHint(handler) {
    this.onHintReceived = handler;
  }

  onExpertConnectionChange(handler) {
    this.onExpertConnection = handler;
  }

  onError(handler) {
    this.onExpertError = handler;
  }

  // Cleanup
  async disconnect() {
    if (this.ws) {
      this.ws.close(1000, 'User disconnected');
      this.ws = null;
    }
    this.connected = false;
    this.currentSessionId = null;
  }

  // Status
  getStatus() {
    return {
      connected: this.connected,
      sessionId: this.currentSessionId,
      expertIds: this.expertIds,
      reconnectAttempts: this.reconnectAttempts
    };
  }
}

// Export for use as module
if (typeof module !== 'undefined' && module.exports) {
  module.exports = TeacherSwitchboardClient;
}

// For browser use
if (typeof window !== 'undefined') {
  window.TeacherSwitchboardClient = TeacherSwitchboardClient;
}