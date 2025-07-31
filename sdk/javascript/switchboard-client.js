/**
 * Switchboard Client SDK
 * 
 * A comprehensive JavaScript library for connecting to Switchboard V4
 * educational communication system. Uses explicit protocol method names.
 * 
 * ARCHITECTURE: This SDK uses explicit message factories that map directly
 * to server protocol types. Any client can send any message type - filtering
 * happens only on the receiving side for educational privacy.
 * 
 * Usage:
 *   const client = new SwitchboardClient({
 *     userId: 'alice',
 *     role: 'student',
 *     wsUrl: 'ws://localhost:8080/ws'
 *   });
 *   
 *   await client.connect();
 *   
 *   // Explicit protocol mapping - factory method
 *   client.Message('broadcast_to_instructors', 'helpRequest')
 *     .withText("How do loops work?")
 *     .send();
 *   
 *   // Direct protocol methods - method names match wire protocol
 *   client.broadcast_to_instructors('codeSubmission')
 *     .withCode(solution)
 *     .send();
 */

class SwitchboardClient {
  constructor(options) {
    // Required options
    this.userId = options.userId;
    this.role = options.role; // 'student' or 'instructor'
    this.wsUrl = options.wsUrl;
    this.apiUrl = options.apiUrl || this.wsUrl.replace('ws:', 'http:').replace('/ws', '/api');
    
    // Validate role
    if (!['student', 'instructor'].includes(this.role)) {
      throw new Error('Role must be "student" or "instructor"');
    }
    
    // Event hooks
    this.hooks = options.hooks || {};
    
    // Connection state
    this.ws = null;
    this.connectionState = 'disconnected';
    this.sessionActive = false;
    this.currentSession = null;
    
    // Protocol constants
    this.maxMessageSize = 64 * 1024; // 64KB
    
    // Message queuing and storage
    this.messageQueue = [];
    this.queueMessages = options.queueMessages !== false;
    this.messages = [];
    this.maxStoredMessages = options.maxStoredMessages || 1000;
    
    // Rate limiting (client-side prevention)
    this.messagesSent = [];
    this.rateLimitWindow = 60000; // 1 minute
    this.rateLimitMax = 100; // 100 messages per minute
    
    // History tracking
    this.historyDelivered = false;
    
    // Debugging
    this.debug = options.debug || false;
    
    // Initialize search API
    this.search = new MessageSearchAPI(this);
  }
  
  // ========== CONNECTION MANAGEMENT ==========
  
  async connect() {
    if (this.connectionState === 'connected') {
      this._log('Already connected');
      return;
    }
    
    if (this.connectionState === 'connecting') {
      this._log('Connection already in progress');
      return;
    }
    
    this.connectionState = 'connecting';
    this.hooks.onConnecting?.();
    
    try {
      const url = `${this.wsUrl}?user_id=${encodeURIComponent(this.userId)}&role=${encodeURIComponent(this.role)}`;
      this._log('Connecting to:', url);
      
      this.ws = new WebSocket(url);
      
      await new Promise((resolve, reject) => {
        const timeout = setTimeout(() => {
          reject(new Error('Connection timeout'));
        }, 10000);
        
        this.ws.onopen = () => {
          clearTimeout(timeout);
          this._log('WebSocket connected');
          this.connectionState = 'connected';
          this.hooks.onConnected?.();
          this._flushMessageQueue();
          resolve();
        };
        
        this.ws.onerror = (error) => {
          clearTimeout(timeout);
          reject(error);
        };
        
        // Set up other handlers
        this._setupWebSocketHandlers();
      });
      
    } catch (error) {
      this.connectionState = 'disconnected';
      this.hooks.onConnectionError?.(error);
      throw error;
    }
  }
  
  disconnect() {
    if (this.ws) {
      this.ws.close(1000, 'Client disconnect');
      this.ws = null;
    }
    this.connectionState = 'disconnected';
    this.sessionActive = false;
    this.currentSession = null;
  }
  
  _setupWebSocketHandlers() {
    // onopen is handled in connect() method to avoid race condition
    
    this.ws.onclose = (event) => {
      this._log('WebSocket closed:', event.code, event.reason);
      this.connectionState = 'disconnected';
      this.hooks.onDisconnected?.(event.code, event.reason);
      
      // No automatic reconnection - user must manually reconnect
      this._log('Disconnected. Manual reconnection required.');
    };
    
    this.ws.onerror = (error) => {
      this._log('WebSocket error:', error);
      this.hooks.onConnectionError?.(error);
    };
    
    this.ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        this._handleMessage(message);
      } catch (error) {
        this._log('Failed to parse message:', error, event.data);
      }
    };
  }
  
  // ========== MESSAGE HANDLING ==========
  
  _handleMessage(message) {
    this._log('Received message:', message);
    
    if (message.type !== 'system' && message.type !== 'error') {
      this._storeMessage(message);
    }
    
    switch (message.type) {
      case 'broadcast_to_instructors':
        this.hooks.onBroadcastToInstructors?.(message);
        break;
      case 'direct_message':
        this.hooks.onDirectMessage?.(message);
        break;
      case 'broadcast_to_students':
        this.hooks.onBroadcastToStudents?.(message);
        break;
      case 'system':
        this._handleSystemMessage(message);
        break;
      case 'error':
        this._handleErrorMessage(message);
        break;
    }
    
    this.hooks.onMessage?.(message);
  }
  
  _handleSystemMessage(message) {
    const event = message.content?.event;
    
    switch (event) {
      case 'waiting_for_session':
        this.sessionActive = false;
        this.currentSession = null;
        this.historyDelivered = false;
        this.hooks.onWaitingForSession?.(message);
        break;
        
      case 'session_active':
        this.sessionActive = true;
        this.currentSession = {
          id: message.content.session_id,
          name: message.content.session_name,
          startedBy: message.content.started_by,
          startTime: new Date(message.content.start_time)
        };
        this.historyDelivered = false;
        this.hooks.onSessionActive?.(this.currentSession, message);
        break;
        
      case 'session_started':
        this.sessionActive = true;
        this.currentSession = {
          id: message.content.session_id,
          name: message.content.session_name,
          startedBy: message.content.started_by || message.content.instructor,
          startTime: new Date()
        };
        this.hooks.onSessionStarted?.(this.currentSession, message);
        break;
        
      case 'session_ended':
        this.sessionActive = false;
        const endedSession = this.currentSession;
        this.currentSession = null;
        this.hooks.onSessionEnded?.(endedSession, message);
        break;
        
      case 'history_delivered':
        this.historyDelivered = true;
        this.hooks.onHistoryDelivered?.(message);
        break;
        
      default:
        this.hooks.onSystemMessage?.(message);
    }
  }
  
  _handleErrorMessage(message) {
    this._log('Server error:', message.error, message.message);
    
    switch (message.error) {
      case 'rate_limit_exceeded':
        this.hooks.onRateLimited?.(message);
        break;
      case 'no_active_session':
        this.hooks.onNoActiveSession?.(message);
        break;
      case 'message_too_large':
        this.hooks.onMessageTooLarge?.(message);
        break;
    }
    
    this.hooks.onError?.(message);
  }
  
  _storeMessage(message) {
    this.messages.push(message);
    if (this.messages.length > this.maxStoredMessages) {
      this.messages = this.messages.slice(-this.maxStoredMessages);
    }
  }
  
  // ========== MESSAGE SENDING ==========
  
  sendMessage(type, content, toUser = null, context = 'general') {
    // Session validation (relaxed for certain contexts)
    if (!this.sessionActive && context !== 'system' && context !== 'announcement') {
      const error = new Error('Please wait for a session to start before sending messages');
      this.hooks.onError?.({ error: 'no_active_session', message: error.message });
      throw error;
    }
    
    // Rate limit check
    if (!this._checkRateLimit()) {
      const error = new Error('Rate limit exceeded (100 messages per minute)');
      this.hooks.onRateLimited?.({ error: 'rate_limit_exceeded', message: error.message });
      throw error;
    }
    
    // Build message
    const message = { type, context, content };
    if (toUser) message.to_user = toUser;
    
    // Size validation
    const messageStr = JSON.stringify(message);
    if (messageStr.length > this.maxMessageSize) {
      const error = new Error(`Message exceeds ${this.maxMessageSize} bytes`);
      this.hooks.onMessageTooLarge?.({ error: 'message_too_large', message: error.message });
      throw error;
    }
    
    // Send or queue
    if (this.connectionState === 'connected' && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(messageStr);
      this._trackRateLimit();
      this._log('Sent message:', message);
    } else if (this.queueMessages) {
      this.messageQueue.push(messageStr);
      this._log('Queued message:', message);
    } else {
      throw new Error('Not connected and message queuing disabled');
    }
    
    return message;
  }
  
  _flushMessageQueue() {
    if (this.messageQueue.length === 0) return;
    
    this._log(`Flushing ${this.messageQueue.length} queued messages`);
    
    for (const messageStr of this.messageQueue) {
      if (this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(messageStr);
        this._trackRateLimit();
      }
    }
    
    this.messageQueue = [];
  }
  
  _checkRateLimit() {
    const now = Date.now();
    const cutoff = now - this.rateLimitWindow;
    this.messagesSent = this.messagesSent.filter(time => time > cutoff);
    return this.messagesSent.length < this.rateLimitMax;
  }
  
  _trackRateLimit() {
    this.messagesSent.push(Date.now());
  }
  
  // ========== MESSAGE FACTORY METHODS ==========
  
  /**
   * Create a message with explicit type and semantic name
   * Maps directly to server protocol types for transparency
   * 
   * @param {string} type - Protocol message type: 'broadcast_to_instructors', 'direct_message', 'broadcast_to_students'  
   * @param {string} name - Semantic name: 'helpRequest', 'codeSubmission', 'announcement', etc.
   * @param {string} [toUser] - Required for direct_message type
   * @returns {MessageBuilder} - Fluent message builder
   */
  Message(type, name, toUser = null) {
    // Validate protocol message type
    const validTypes = ['broadcast_to_instructors', 'direct_message', 'broadcast_to_students'];
    if (!validTypes.includes(type)) {
      throw new Error(`Invalid message type. Must be one of: ${validTypes.join(', ')}`);
    }
    
    // Validate toUser for direct messages
    if (type === 'direct_message' && !toUser) {
      throw new Error('direct_message type requires toUser parameter');
    }
    
    if (type !== 'direct_message' && toUser) {
      throw new Error(`toUser parameter only valid for direct_message type, not ${type}`);
    }
    
    return new MessageBuilder(this, type, name, toUser);
  }
  
  // ========== PROTOCOL METHODS ==========
  
  /**
   * Send broadcast_to_instructors message
   * Method name matches exact protocol type for transparency
   * 
   * @param {string} name - Semantic name (helpRequest, codeSubmission, analytics, etc.)
   * @returns {MessageBuilder}
   */
  broadcast_to_instructors(name) {
    return this.Message('broadcast_to_instructors', name);
  }
  
  /**
   * Send broadcast_to_students message  
   * Method name matches exact protocol type for transparency
   * 
   * @param {string} name - Semantic name (announcement, instruction, etc.)
   * @returns {MessageBuilder}
   */
  broadcast_to_students(name) {
    return this.Message('broadcast_to_students', name);
  }
  
  /**
   * Send direct_message to specific user
   * Method name matches exact protocol type for transparency
   * 
   * @param {string} toUser - Recipient user ID
   * @param {string} name - Semantic name (response, helpRequest, general, etc.)
   * @returns {MessageBuilder}
   */
  direct_message(toUser, name) {
    return this.Message('direct_message', name, toUser);
  }
  
  // ========== SESSION MANAGEMENT (INSTRUCTORS) ==========
  
  async startSession(sessionName) {
    const url = `${this.apiUrl}/session/start`;
    this._log('Starting session with URL:', url);
    const response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: sessionName,
        instructor_id: this.userId
      })
    });
    
    if (!response.ok) {
      this._log('Session start failed:', response.status, response.statusText);
      let errorMessage = `HTTP ${response.status}: ${response.statusText}`;
      
      try {
        const error = await response.json();
        this._log('Error response body:', error);
        errorMessage = error.message || errorMessage;
      } catch (parseError) {
        this._log('Could not parse error response as JSON');
      }
      
      if (response.status === 409) {
        throw new Error('A session is already active. Please end the current session first.');
      }
      
      throw new Error(errorMessage);
    }
    
    const result = await response.json();
    this._log('Session started:', result);
    return result;
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
    
    const result = await response.json();
    this._log('Session ended:', result);
    return result;
  }
  
  async getActiveSession() {
    const response = await fetch(`${this.apiUrl}/session/active`);
    
    if (response.status === 204) {
      return null;
    }
    
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || `HTTP ${response.status}`);
    }
    
    return await response.json();
  }
  
  // ========== UTILITY METHODS ==========
  
  // ========== ESSENTIAL PUBLIC API (5 methods) ==========
  
  isConnected() {
    return this.connectionState === 'connected';
  }
  
  isSessionActive() {
    return this.sessionActive;
  }
  
  getCurrentSession() {
    return this.currentSession;
  }
  
  // ========== INTERNAL UTILITIES ==========
  
  _log(...args) {
    if (this.debug) {
      console.log('[SwitchboardClient]', ...args);
    }
  }
}

// ========== MESSAGE BUILDER CLASS ==========

class MessageBuilder {
  constructor(client, type, name, toUser = null) {
    this.client = client;
    this.type = type;
    this.name = name;
    this.toUser = toUser;
    this.content = { name };  // Include semantic name in content
    this.context = this.getDefaultContext(name);
  }
  
  /**
   * Auto-assign appropriate context based on semantic name
   * @private
   */
  getDefaultContext(name) {
    const contextMap = {
      // Questions and help
      'helpRequest': 'question',
      'question': 'question',
      'clarification': 'question',
      
      // Submissions and work
      'codeSubmission': 'submission',
      'submission': 'submission',
      'workSubmission': 'submission',
      
      // Analytics and tracking
      'codeSnapshot': 'analytics',
      'progressUpdate': 'analytics',
      'analytics': 'analytics',
      
      // Communications
      'announcement': 'announcement',
      'instruction': 'instruction',
      'response': 'response',
      
      // Technical
      'technicalIssue': 'request',
      'support': 'request',
      
      // General
      'general': 'general',
      'discovery': 'general',
      'sharing': 'general'
    };
    
    return contextMap[name] || 'general';
  }
  
  /**
   * Set the main text content
   */
  withText(text) {
    if (typeof text !== 'string') {
      throw new Error('Text must be a string');
    }
    this.content.text = text.trim();
    return this;
  }
  
  /**
   * Add code snippet
   */
  withCode(code, language = null) {
    this.content.code_snippet = code;
    if (language) this.content.language = language;
    return this;
  }
  
  /**
   * Add line number reference
   */
  withLineNumber(lineNumber) {
    if (typeof lineNumber === 'number' && lineNumber > 0) {
      this.content.line_number = lineNumber;
    }
    return this;
  }
  
  /**
   * Add arbitrary data to content
   */
  withData(data) {
    if (typeof data === 'object' && data !== null) {
      Object.assign(this.content, data);
    }
    return this;
  }
  
  /**
   * Override the default context
   */
  withContext(context) {
    this.context = context;
    return this;
  }
  
  /**
   * Mark as important (useful for announcements)
   */
  markAsImportant() {
    this.content.important = true;
    return this;
  }
  
  /**
   * Set urgency level
   */
  withUrgency(level) {
    const validLevels = ['low', 'medium', 'high', 'urgent'];
    if (validLevels.includes(level)) {
      this.content.urgency = level;
    }
    return this;
  }
  
  /**
   * Add tags for categorization
   */
  withTags(...tags) {
    this.content.tags = tags.filter(tag => typeof tag === 'string');
    return this;
  }
  
  /**
   * Reference another message
   */
  referencingMessage(messageId) {
    this.content.reference_message_id = messageId;
    return this;
  }
  
  /**
   * Send the message
   */
  send() {
    // Basic validation
    if (!this.content.text && !this.content.code_snippet && !this.content.data) {
      throw new Error(`Message '${this.name}' needs text, code, or data content`);
    }
    
    return this.client.sendMessage(this.type, this.content, this.toUser, this.context);
  }
}

// ========== CONSTANTS ==========

SwitchboardClient.MessageType = {
  BROADCAST_TO_INSTRUCTORS: 'broadcast_to_instructors',
  DIRECT_MESSAGE: 'direct_message', 
  BROADCAST_TO_STUDENTS: 'broadcast_to_students'
};

SwitchboardClient.MessageContext = {
  QUESTION: 'question',
  RESPONSE: 'response',
  ANNOUNCEMENT: 'announcement',
  INSTRUCTION: 'instruction',
  EMERGENCY: 'emergency',
  SUBMISSION: 'submission',
  ANALYTICS: 'analytics',
  REQUEST: 'request',
  PEER_HELP: 'peer_help',
  GENERAL: 'general'
};

SwitchboardClient.ConnectionState = {
  DISCONNECTED: 'disconnected',
  CONNECTING: 'connecting',
  CONNECTED: 'connected'
};

// ========== MESSAGE SEARCH API ==========

class MessageSearchAPI {
  constructor(client) {
    this.client = client;
  }
  
  /**
   * Search messages by text content
   */
  byText(searchText) {
    const text = searchText.toLowerCase();
    return this.client.messages.filter(msg => {
      const searchIn = [
        msg.content?.text || '',
        msg.content?.code_snippet || '',
        msg.from_user || ''
      ].join(' ').toLowerCase();
      
      return searchIn.includes(text);
    });
  }
  
  /**
   * Get messages by type
   */
  byType(messageType) {
    return this.client.messages.filter(msg => msg.type === messageType);
  }
  
  /**
   * Get messages by context
   */
  byContext(context) {
    return this.client.messages.filter(msg => msg.context === context);
  }
  
  /**
   * Get messages from specific user
   */
  byUser(userId) {
    return this.client.messages.filter(msg => msg.from_user === userId);
  }
  
  /**
   * Get recent messages (last N messages)
   */
  recent(count = 50) {
    return this.client.messages.slice(-count);
  }
}


// Export for both browser and Node.js
if (typeof module !== 'undefined' && module.exports) {
  module.exports = SwitchboardClient;
} else if (typeof window !== 'undefined') {
  window.SwitchboardClient = SwitchboardClient;
}