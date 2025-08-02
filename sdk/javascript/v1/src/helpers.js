/**
 * Switchboard Client SDK - Non-Opinionated Helpers
 * 
 * Pure utility functions for message formatting, connection status,
 * and data manipulation. No DOM dependencies or styling assumptions.
 */

/**
 * Format a message for display with computed properties
 */
export function formatMessage(message, options = {}) {
  const currentUser = options.currentUser;
  const isOwn = currentUser && message.from_user === currentUser;
  const timestamp = new Date(message.timestamp);
  
  return {
    id: message.id,
    type: message.type,
    context: message.context,
    displayName: message.from_user,
    timeAgo: getRelativeTime(timestamp),
    timestamp: formatTimestamp(timestamp),
    isOwn,
    excerpt: getExcerpt(message.content?.text || ''),
    hasCode: !!message.content?.code_snippet,
    hasData: Object.keys(message.content || {}).length > 2,
    urgency: message.content?.urgency || 'normal',
    tags: message.content?.tags || [],
    important: !!message.content?.important
  };
}

/**
 * Get connection status with computed properties
 */
export function getConnectionStatus(client) {
  if (!client) {
    return {
      state: 'disconnected',
      sessionActive: false,
      sessionName: null,
      sessionId: null,
      canSendMessages: false,
      isReconnecting: false,
      reconnectAttempt: 0
    };
  }
  
  return {
    state: client.connectionState,
    sessionActive: client.sessionActive,
    sessionName: client.currentSession?.name || null,
    sessionId: client.currentSession?.id || null,
    canSendMessages: client.isConnected() && client.isSessionActive(),
    isReconnecting: client.connectionState === 'connecting' && client.reconnectAttempts > 0,
    reconnectAttempt: client.reconnectAttempts
  };
}

/**
 * Group messages into conversation threads
 */
export function groupMessageThreads(messages) {
  const threads = [];
  const messageMap = new Map();
  const threadMap = new Map();
  
  // Build message map
  messages.forEach(msg => {
    messageMap.set(msg.id, msg);
  });
  
  // Find root messages and build threads
  messages.forEach(msg => {
    const refId = msg.content?.reference_message_id;
    
    if (!refId || !messageMap.has(refId)) {
      // This is a root message
      if (!threadMap.has(msg.id)) {
        threadMap.set(msg.id, {
          original: msg,
          responses: [],
          participants: new Set([msg.from_user]),
          lastActivity: new Date(msg.timestamp)
        });
      }
    } else {
      // This is a response
      let rootId = refId;
      let rootMsg = messageMap.get(rootId);
      
      // Find the root of the thread
      while (rootMsg?.content?.reference_message_id && 
             messageMap.has(rootMsg.content.reference_message_id)) {
        rootId = rootMsg.content.reference_message_id;
        rootMsg = messageMap.get(rootId);
      }
      
      if (!threadMap.has(rootId)) {
        threadMap.set(rootId, {
          original: rootMsg,
          responses: [],
          participants: new Set([rootMsg.from_user]),
          lastActivity: new Date(rootMsg.timestamp)
        });
      }
      
      const thread = threadMap.get(rootId);
      thread.responses.push(msg);
      thread.participants.add(msg.from_user);
      
      const msgTime = new Date(msg.timestamp);
      if (msgTime > thread.lastActivity) {
        thread.lastActivity = msgTime;
      }
    }
  });
  
  // Convert to array and add computed properties
  threadMap.forEach(thread => {
    threads.push({
      original: thread.original,
      responses: thread.responses.sort((a, b) => 
        new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
      ),
      participants: Array.from(thread.participants),
      lastActivity: thread.lastActivity,
      isResolved: thread.responses.some(r => 
        r.context === 'response' && r.from_user !== thread.original.from_user
      )
    });
  });
  
  return threads.sort((a, b) => 
    b.lastActivity.getTime() - a.lastActivity.getTime()
  );
}

/**
 * Format timestamp for display
 */
export function formatTimestamp(timestamp) {
  const date = timestamp instanceof Date ? timestamp : new Date(timestamp);
  const now = new Date();
  const isToday = date.toDateString() === now.toDateString();
  
  if (isToday) {
    return date.toLocaleTimeString('en-US', { 
      hour: 'numeric', 
      minute: '2-digit',
      hour12: true 
    });
  }
  
  return date.toLocaleDateString('en-US', { 
    month: 'short', 
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit'
  });
}

/**
 * Get relative time string
 */
export function getRelativeTime(timestamp) {
  const date = timestamp instanceof Date ? timestamp : new Date(timestamp);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  
  const minutes = Math.floor(diff / 60000);
  const hours = Math.floor(diff / 3600000);
  const days = Math.floor(diff / 86400000);
  
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes}m ago`;
  if (hours < 24) return `${hours}h ago`;
  if (days < 30) return `${days}d ago`;
  
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

/**
 * Check if message belongs to current user
 */
export function isOwnMessage(message, currentUser) {
  return message.from_user === currentUser;
}

/**
 * Extract @mentions from text
 */
export function extractMentions(text) {
  if (!text) return [];
  
  const mentions = [];
  const regex = /@(\w+)/g;
  let match;
  
  while ((match = regex.exec(text)) !== null) {
    mentions.push(match[1]);
  }
  
  return [...new Set(mentions)]; // Remove duplicates
}

/**
 * Parse code content from message
 */
export function parseCode(content) {
  if (!content?.code_snippet) return null;
  
  return {
    snippet: content.code_snippet,
    language: content.language || null,
    lineNumber: content.line_number || null,
    isComplete: !content.code_snippet.includes('...')
  };
}

/**
 * Group messages by time period
 */
export function groupMessagesByTime(messages, period = 'day') {
  const groups = {};
  
  messages.forEach(msg => {
    const date = new Date(msg.timestamp);
    let key;
    
    switch (period) {
      case 'hour':
        key = `${date.toDateString()} ${date.getHours()}:00`;
        break;
      case 'day':
        key = date.toDateString();
        break;
      case 'week':
        const week = Math.floor(date.getDate() / 7);
        key = `${date.getFullYear()}-${date.getMonth()}-W${week}`;
        break;
      default:
        key = date.toDateString();
    }
    
    if (!groups[key]) groups[key] = [];
    groups[key].push(msg);
  });
  
  return groups;
}

/**
 * Create a basic message element (no styling)
 */
export function createMessageElement(message, options = {}) {
  const div = document.createElement('div');
  div.className = options.className || 'message';
  div.dataset.messageId = message.id;
  
  const formatted = formatMessage(message, options);
  
  const content = document.createElement('div');
  content.className = 'message-content';
  content.textContent = formatted.excerpt;
  
  const meta = document.createElement('div');
  meta.className = 'message-meta';
  meta.innerHTML = `
    <span class="message-user">${formatted.displayName}</span>
    <span class="message-time">${formatted.timeAgo}</span>
  `;
  
  div.appendChild(meta);
  div.appendChild(content);
  
  return div;
}

/**
 * Create a connection status indicator (no styling)
 */
export function createStatusIndicator(state, text = null) {
  const indicator = document.createElement('div');
  indicator.className = 'connection-status';
  indicator.dataset.state = state;
  
  const states = {
    'disconnected': { text: 'Disconnected', symbol: '○' },
    'connecting': { text: 'Connecting...', symbol: '◐' },
    'connected': { text: 'Connected', symbol: '●' }
  };
  
  const stateInfo = states[state] || states.disconnected;
  const displayText = text || stateInfo.text;
  
  indicator.innerHTML = `
    <span class="status-symbol">${stateInfo.symbol}</span>
    <span class="status-text">${displayText}</span>
  `;
  
  return indicator;
}

/**
 * Parse message content into structured elements
 */
export function parseMessageContent(content) {
  const elements = [];
  
  if (content.text) {
    elements.push({ type: 'text', value: content.text });
  }
  
  if (content.code_snippet) {
    elements.push({
      type: 'code',
      value: content.code_snippet,
      language: content.language
    });
  }
  
  if (content.tags) {
    elements.push({
      type: 'tags',
      value: content.tags
    });
  }
  
  return elements;
}

/**
 * Create a typing indicator element
 */
export function createTypingIndicator(users = []) {
  const div = document.createElement('div');
  div.className = 'typing-indicator';
  
  if (users.length === 0) {
    div.style.display = 'none';
  } else if (users.length === 1) {
    div.textContent = `${users[0]} is typing...`;
  } else if (users.length === 2) {
    div.textContent = `${users[0]} and ${users[1]} are typing...`;
  } else {
    div.textContent = `${users[0]} and ${users.length - 1} others are typing...`;
  }
  
  return div;
}

// ========== PRIVATE HELPER FUNCTIONS ==========

function getExcerpt(text, maxLength = 100) {
  if (!text || text.length <= maxLength) return text;
  
  const truncated = text.substring(0, maxLength);
  const lastSpace = truncated.lastIndexOf(' ');
  
  return (lastSpace > 0 ? truncated.substring(0, lastSpace) : truncated) + '...';
}