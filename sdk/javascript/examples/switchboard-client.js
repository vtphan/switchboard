/**
 * Switchboard Client SDK V2 - Simplified Client
 * 
 * Redesigned with 6 hooks (4 message types + 2 state changes) for clean, intuitive API.
 * Key improvements:
 * - Simple object-based message sending (no builder pattern)
 * - Direct mapping to protocol message types
 * - Fixed context field handling to prevent double-nesting
 * - Hidden complexity with smart defaults
 */

class SwitchboardClient {
  constructor(config) {
    // Validation
    if (!config.userId) throw new Error('userId required');
    if (!['student', 'instructor'].includes(config.role)) {
      throw new Error('role must be "student" or "instructor"');
    }
    
    // Configuration
    this.userId = config.userId;
    this.role = config.role;
    this.wsUrl = config.wsUrl || 'ws://localhost:8080/ws';
    this.apiUrl = config.apiUrl || this.wsUrl.replace('ws:', 'http:').replace('/ws', '/api');
    
    // 4 core message handlers (all optional)
    this.messageHandlers = {
      broadcast_to_instructors: config.onBroadcastToInstructors || null,
      broadcast_to_students: config.onBroadcastToStudents || null,
      direct_message: config.onDirectMessage || null,
      system: config.onSystem || null
    };
    
    // 2 state change handlers (optional with defaults)
    this.onConnectionChange = config.onConnectionChange || this._defaultConnectionHandler;
    this.onSessionChange = config.onSessionChange || this._defaultSessionHandler;
    
    // Internal state
    this.ws = null;
    this.connectionState = 'disconnected';
    this.sessionActive = false;
    this.currentSession = null;
    this.messageQueue = [];
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = config.maxReconnectAttempts || 10;
    
    // Protocol constants
    this.maxMessageSize = 64 * 1024; // 64KB
    this.reconnectBaseDelay = 1000;
    this.maxReconnectDelay = 30000;
    
    // Rate limiting (client-side prevention)
    this.messagesSent = [];
    this.rateLimitWindow = 60000; // 1 minute
    this.rateLimitMax = 100; // 100 messages per minute
  }
  
  // ========== DEFAULT HANDLERS ==========
  
  _defaultConnectionHandler(state, error) {
    if (state === 'error' && error) {
      console.error('Switchboard connection error:', error);
    }
  }
  
  _defaultSessionHandler(session) {
    // No-op by default
  }
  
  // ========== CONNECTION MANAGEMENT ==========
  
  async connect() {
    if (this.connectionState === 'connected') return;
    if (this.connectionState === 'connecting') return;
    
    this._setConnectionState('connecting');
    
    try {
      const url = `${this.wsUrl}?user_id=${encodeURIComponent(this.userId)}&role=${this.role}`;
      this.ws = new WebSocket(url);
      
      this._setupWebSocketHandlers();
      
      await new Promise((resolve, reject) => {
        const timeout = setTimeout(() => reject(new Error('Connection timeout')), 10000);
        
        const originalOnOpen = this.ws.onopen;
        const originalOnError = this.ws.onerror;
        
        this.ws.onopen = () => {
          clearTimeout(timeout);
          this._onWebSocketOpen();
          resolve();
        };
        
        this.ws.onerror = (error) => {
          clearTimeout(timeout);
          reject(error);
        };
      });
      
    } catch (error) {
      this._setConnectionState('error', error);
      throw error;
    }
  }
  
  disconnect() {
    if (this.ws) {
      // Clear handlers before closing to prevent null reference errors
      this.ws.onopen = null;
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.onmessage = null;
      
      this.ws.close(1000, 'Client disconnect');
      this.ws = null;
    }
    this._setConnectionState('disconnected');
    this.reconnectAttempts = 0;
  }
  
  _setConnectionState(state, error = null) {
    this.connectionState = state;
    this.onConnectionChange(state, error);
  }
  
  _onWebSocketOpen() {
    this._setConnectionState('connected');
    this.reconnectAttempts = 0;
    this._flushMessageQueue();
  }
  
  // ========== WEBSOCKET HANDLERS ==========
  
  _setupWebSocketHandlers() {
    this.ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        this._routeMessage(message);
      } catch (error) {
        console.error('Failed to parse message:', error);
      }
    };
    
    this.ws.onclose = (event) => {
      this._setConnectionState('disconnected');
      if (event.code !== 1000 && this.reconnectAttempts < this.maxReconnectAttempts) {
        this._scheduleReconnect();
      }
    };
    
    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  }
  
  _routeMessage(message) {
    // Handle system messages internally first
    if (message.type === 'system') {
      this._handleSystemMessage(message);
    }
    
    // Route to appropriate handler
    const handler = this.messageHandlers[message.type];
    if (handler) {
      handler(message);
    }
  }
  
  _handleSystemMessage(message) {
    const event = message.content?.event;
    
    switch (event) {
      case 'session_started':
        this._setSession({
          active: true,
          id: message.content.session_id,
          name: message.content.session_name,
          startedBy: message.content.started_by || message.content.instructor
        });
        break;
        
      case 'session_ended':
        this._setSession({ active: false });
        break;
        
      case 'session_active':
        this._setSession({
          active: true,
          id: message.content.session_id,
          name: message.content.session_name,
          startedBy: message.content.started_by
        });
        break;
        
      case 'waiting_for_session':
        this._setSession({ active: false });
        break;
    }
    
    // Still pass to handler if provided
    if (this.messageHandlers.system) {
      this.messageHandlers.system(message);
    }
  }
  
  _setSession(session) {
    this.sessionActive = session.active;
    this.currentSession = session.active ? session : null;
    this.onSessionChange(session);
  }
  
  // ========== MESSAGE SENDING ==========
  
  broadcastToInstructors(content) {
    return this._send('broadcast_to_instructors', content);
  }
  
  broadcastToStudents(content) {
    return this._send('broadcast_to_students', content);
  }
  
  directMessage(toUser, content) {
    return this._send('direct_message', content, toUser);
  }
  
  _send(type, content, toUser = null) {
    // Support both string and object content
    if (typeof content === 'string') {
      content = { text: content };
    }
    
    // Extract context from content (CRITICAL FIX: don't double-nest)
    const context = content.context || 'general';
    
    // Create clean content object without metadata
    const cleanContent = { ...content };
    delete cleanContent.context; // Remove context from content to avoid double-nesting
    
    // Validate session (unless it's a system context)
    if (!this.sessionActive && context !== 'system') {
      throw new Error('No active session');
    }
    
    // Check rate limit
    if (!this._checkRateLimit()) {
      throw new Error('Rate limit exceeded');
    }
    
    // Build message with correct protocol structure
    const message = {
      type,
      context,           // Context as separate field (database schema requirement)
      content: cleanContent  // Clean content without metadata duplication
    };
    
    if (toUser) {
      message.to_user = toUser;
    }
    
    // Size check
    const messageStr = JSON.stringify(message);
    if (messageStr.length > this.maxMessageSize) {
      throw new Error('Message too large');
    }
    
    // Send or queue
    if (this.connectionState === 'connected' && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(messageStr);
      this._trackRateLimit();
    } else {
      this.messageQueue.push(messageStr);
    }
    
    return message;
  }
  
  // ========== RATE LIMITING ==========
  
  _checkRateLimit() {
    const now = Date.now();
    const cutoff = now - this.rateLimitWindow;
    this.messagesSent = this.messagesSent.filter(time => time > cutoff);
    return this.messagesSent.length < this.rateLimitMax;
  }
  
  _trackRateLimit() {
    this.messagesSent.push(Date.now());
  }
  
  // ========== SESSION MANAGEMENT ==========
  
  async startSession(sessionName) {
    const response = await fetch(`${this.apiUrl}/session/start`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: sessionName,
        instructor_id: this.userId
      })
    });
    
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || `HTTP ${response.status}`);
    }
    
    return await response.json();
  }
  
  async endSession() {
    const response = await fetch(`${this.apiUrl}/session/end`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        instructor_id: this.userId
      })
    });
    
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || `HTTP ${response.status}`);
    }
    
    return await response.json();
  }
  
  // NOTE: getActiveSession() method removed - not needed in single-session architecture
  // Session state is maintained internally via WebSocket system messages:
  // - this.sessionActive (boolean) - true when session is active  
  // - this.currentSession (object) - current session details or null
  // - onSessionChange hook - called when session state changes
  //
  // Usage: if (client.sessionActive) { console.log(client.currentSession.name); }
  
  // ========== AUTO-RECONNECTION ==========
  
  _scheduleReconnect() {
    const delay = Math.min(
      this.reconnectBaseDelay * Math.pow(2, this.reconnectAttempts),
      this.maxReconnectDelay
    );
    
    setTimeout(() => {
      this.reconnectAttempts++;
      this.connect().catch(error => {
        console.error('Reconnection failed:', error);
      });
    }, delay);
  }
  
  _flushMessageQueue() {
    while (this.messageQueue.length > 0 && this.ws.readyState === WebSocket.OPEN) {
      const messageStr = this.messageQueue.shift();
      this.ws.send(messageStr);
      this._trackRateLimit();
    }
  }
  
  // ========== UTILITY METHODS ==========
  
  getConnectionStatus() {
    return this.connectionState;
  }
  
  isConnected() {
    return this.connectionState === 'connected';
  }
  
  isSessionActive() {
    return this.sessionActive;
  }
  
  getCurrentSession() {
    return this.currentSession;
  }
}

// Export for both browser and Node.js
if (typeof module !== 'undefined' && module.exports) {
  module.exports = SwitchboardClient;
} else if (typeof window !== 'undefined') {
  window.SwitchboardClient = SwitchboardClient;
}

export default SwitchboardClient;