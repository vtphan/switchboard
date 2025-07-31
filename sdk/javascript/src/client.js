/**
 * Switchboard Client SDK - Core Client
 * 
 * Pure WebSocket client with explicit protocol methods.
 * No built-in helpers or UI components - those are imported separately.
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
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = options.maxReconnectAttempts || 10;
    
    // Protocol constants
    this.maxMessageSize = 64 * 1024; // 64KB
    this.reconnectBaseDelay = 1000;
    this.maxReconnectDelay = 30000;
    
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
  }
  
  // ========== CONNECTION MANAGEMENT ==========
  
  async connect() {
    if (this.connectionState === 'connected') {
      this.log('Already connected');
      return;
    }
    
    if (this.connectionState === 'connecting') {
      this.log('Connection already in progress');
      return;
    }
    
    this.connectionState = 'connecting';
    this.hooks.onConnecting?.();
    
    try {
      const url = `${this.wsUrl}?user_id=${encodeURIComponent(this.userId)}&role=${encodeURIComponent(this.role)}`;
      this.log('Connecting to:', url);
      
      this.ws = new WebSocket(url);
      this.setupWebSocketHandlers();
      
      await new Promise((resolve, reject) => {
        const timeout = setTimeout(() => {
          reject(new Error('Connection timeout'));
        }, 10000);
        
        this.ws.onopen = () => {
          clearTimeout(timeout);
          resolve();
        };
        
        this.ws.onerror = (error) => {
          clearTimeout(timeout);
          reject(error);
        };
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
    this.reconnectAttempts = 0;
  }
  
  setupWebSocketHandlers() {
    this.ws.onopen = () => {
      this.log('WebSocket connected');
      this.connectionState = 'connected';
      this.reconnectAttempts = 0;
      this.hooks.onConnected?.();
      this.flushMessageQueue();
    };
    
    this.ws.onclose = (event) => {
      this.log('WebSocket closed:', event.code, event.reason);
      this.connectionState = 'disconnected';
      this.hooks.onDisconnected?.(event.code, event.reason);
      
      if (event.code !== 1000 && this.reconnectAttempts < this.maxReconnectAttempts) {
        this.scheduleReconnect();
      }
    };
    
    this.ws.onerror = (error) => {
      this.log('WebSocket error:', error);
      this.hooks.onConnectionError?.(error);
    };
    
    this.ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        this.handleMessage(message);
      } catch (error) {
        this.log('Failed to parse message:', error, event.data);
      }
    };
  }
  
  scheduleReconnect() {
    const delay = Math.min(
      this.reconnectBaseDelay * Math.pow(2, this.reconnectAttempts),
      this.maxReconnectDelay
    );
    
    this.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts + 1})`);
    this.hooks.onReconnecting?.(this.reconnectAttempts + 1, delay);
    
    setTimeout(() => {
      this.reconnectAttempts++;
      this.connect().catch(error => {
        this.log('Reconnection failed:', error);
      });
    }, delay);
  }
  
  // ========== MESSAGE HANDLING ==========
  
  handleMessage(message) {
    this.log('Received message:', message);
    
    if (message.type !== 'system' && message.type !== 'error') {
      this.storeMessage(message);
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
        this.handleSystemMessage(message);
        break;
      case 'error':
        this.handleErrorMessage(message);
        break;
    }
    
    this.hooks.onMessage?.(message);
  }
  
  handleSystemMessage(message) {
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
  
  handleErrorMessage(message) {
    this.log('Server error:', message.error, message.message);
    
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
  
  storeMessage(message) {
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
    if (!this.checkRateLimit()) {
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
      this.trackRateLimit();
      this.log('Sent message:', message);
    } else if (this.queueMessages) {
      this.messageQueue.push(messageStr);
      this.log('Queued message:', message);
    } else {
      throw new Error('Not connected and message queuing disabled');
    }
    
    return message;
  }
  
  flushMessageQueue() {
    if (this.messageQueue.length === 0) return;
    
    this.log(`Flushing ${this.messageQueue.length} queued messages`);
    
    for (const messageStr of this.messageQueue) {
      if (this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(messageStr);
        this.trackRateLimit();
      }
    }
    
    this.messageQueue = [];
  }
  
  checkRateLimit() {
    const now = Date.now();
    const cutoff = now - this.rateLimitWindow;
    this.messagesSent = this.messagesSent.filter(time => time > cutoff);
    return this.messagesSent.length < this.rateLimitMax;
  }
  
  trackRateLimit() {
    this.messagesSent.push(Date.now());
  }
  
  // ========== MESSAGE FACTORY METHODS ==========
  
  /**
   * Create a message with explicit type and semantic name
   */
  Message(type, name, toUser = null) {
    const validTypes = ['broadcast_to_instructors', 'direct_message', 'broadcast_to_students'];
    if (!validTypes.includes(type)) {
      throw new Error(`Invalid message type. Must be one of: ${validTypes.join(', ')}`);
    }
    
    if (type === 'direct_message' && !toUser) {
      throw new Error('direct_message type requires toUser parameter');
    }
    
    if (type !== 'direct_message' && toUser) {
      throw new Error(`toUser parameter only valid for direct_message type, not ${type}`);
    }
    
    return new MessageBuilder(this, type, name, toUser);
  }
  
  // ========== PROTOCOL METHODS ==========
  
  broadcast_to_instructors(name) {
    return this.Message('broadcast_to_instructors', name);
  }
  
  broadcast_to_students(name) {
    return this.Message('broadcast_to_students', name);
  }
  
  direct_message(toUser, name) {
    return this.Message('direct_message', name, toUser);
  }
  
  // ========== SESSION MANAGEMENT (INSTRUCTORS) ==========
  
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
    
    const result = await response.json();
    this.log('Session started:', result);
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
    this.log('Session ended:', result);
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
  
  getMessages(filterType = null, limit = null) {
    let messages = this.messages;
    
    if (filterType) {
      messages = messages.filter(msg => msg.type === filterType);
    }
    
    if (limit) {
      messages = messages.slice(-limit);
    }
    
    return messages;
  }
  
  clearMessages() {
    this.messages = [];
  }
  
  getRateLimitStatus() {
    const now = Date.now();
    const cutoff = now - this.rateLimitWindow;
    const recentMessages = this.messagesSent.filter(time => time > cutoff);
    
    return {
      messagesSent: recentMessages.length,
      maxMessages: this.rateLimitMax,
      windowMs: this.rateLimitWindow,
      canSend: recentMessages.length < this.rateLimitMax
    };
  }
  
  setDebug(enabled) {
    this.debug = enabled;
  }
  
  log(...args) {
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
    this.content = { name };
    this.context = this.getDefaultContext(name);
  }
  
  getDefaultContext(name) {
    const contextMap = {
      'helpRequest': 'question',
      'question': 'question',
      'clarification': 'question',
      'codeSubmission': 'submission',
      'submission': 'submission',
      'workSubmission': 'submission',
      'codeSnapshot': 'analytics',
      'progressUpdate': 'analytics',
      'analytics': 'analytics',
      'announcement': 'announcement',
      'instruction': 'instruction',
      'response': 'response',
      'technicalIssue': 'request',
      'support': 'request',
      'general': 'general',
      'discovery': 'general',
      'sharing': 'general'
    };
    
    return contextMap[name] || 'general';
  }
  
  withText(text) {
    if (typeof text !== 'string') {
      throw new Error('Text must be a string');
    }
    this.content.text = text.trim();
    return this;
  }
  
  withCode(code, language = null) {
    this.content.code_snippet = code;
    if (language) this.content.language = language;
    return this;
  }
  
  withLineNumber(lineNumber) {
    if (typeof lineNumber === 'number' && lineNumber > 0) {
      this.content.line_number = lineNumber;
    }
    return this;
  }
  
  withData(data) {
    if (typeof data === 'object' && data !== null) {
      Object.assign(this.content, data);
    }
    return this;
  }
  
  withContext(context) {
    this.context = context;
    return this;
  }
  
  markAsImportant() {
    this.content.important = true;
    return this;
  }
  
  withUrgency(level) {
    const validLevels = ['low', 'medium', 'high', 'urgent'];
    if (validLevels.includes(level)) {
      this.content.urgency = level;
    }
    return this;
  }
  
  withTags(...tags) {
    this.content.tags = tags.filter(tag => typeof tag === 'string');
    return this;
  }
  
  referencingMessage(messageId) {
    this.content.reference_message_id = messageId;
    return this;
  }
  
  send() {
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

// Export for both browser and Node.js
if (typeof module !== 'undefined' && module.exports) {
  module.exports = SwitchboardClient;
} else if (typeof window !== 'undefined') {
  window.SwitchboardClient = SwitchboardClient;
}

export default SwitchboardClient;