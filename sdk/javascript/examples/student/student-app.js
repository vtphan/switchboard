/**
 * Student App using SwitchboardClient V2
 * 
 * Demonstrates the simplified 6-hook architecture and clean usage patterns.
 */

import SwitchboardClient from '../../switchboard-client.js';

class StudentAppV2 {
  constructor() {
    this.client = null;
    this.setupUI();
    this.initializeClient();
  }
  
  setupUI() {
    // Bind event handlers to existing HTML elements
    document.getElementById('connectBtn').onclick = () => this.connect();
    document.getElementById('disconnectBtn').onclick = () => this.disconnect();
    document.getElementById('askBtn').onclick = () => this.askQuestion();
    
    // Enable/disable ask button based on input
    document.getElementById('questionInput').oninput = () => this.updateAskButton();
  }
  
  initializeClient() {
    // Create client with the simplified V2 API
    this.client = new SwitchboardClient({
      userId: 'student-' + Math.random().toString(36).substr(2, 9),
      role: 'student',
      wsUrl: 'ws://localhost:8080/ws',
      
      // Only 4 message type hooks needed
      onBroadcastToStudents: (message) => this.handleAnnouncement(message),
      onDirectMessage: (message) => this.handleDirectMessage(message),
      onSystem: (message) => this.handleSystemMessage(message),
      
      // 2 state change hooks
      onConnectionChange: (state, error) => this.handleConnectionChange(state, error),
      onSessionChange: (session) => this.handleSessionChange(session)
    });
    
    console.log('Student client initialized with V2 API');
  }
  
  // Event handlers for the 6 hooks
  handleAnnouncement(message) {
    console.log('Received announcement:', message);
    this.addMessage({
      type: 'announcement',
      text: message.content.text,
      context: message.context,
      important: message.content.important,
      timestamp: new Date().toLocaleTimeString(),
      from: 'Instructor'
    });
  }
  
  handleDirectMessage(message) {
    console.log('Received direct message:', message);
    this.addMessage({
      type: 'direct',
      text: message.content.text,
      context: message.context,
      timestamp: new Date().toLocaleTimeString(),
      from: message.from_user || 'System'
    });
  }
  
  handleSystemMessage(message) {
    console.log('System message:', message);
    // System messages are handled internally by the client
    // This hook is optional for custom system message handling
  }
  
  handleConnectionChange(state, error) {
    console.log('Connection state changed:', state, error);
    
    const statusEl = document.getElementById('connectionStatus');
    const connectBtn = document.getElementById('connectBtn');
    const disconnectBtn = document.getElementById('disconnectBtn');
    const askBtn = document.getElementById('askBtn');
    
    statusEl.className = `status ${state}`;
    statusEl.textContent = state.charAt(0).toUpperCase() + state.slice(1);
    
    switch (state) {
      case 'connecting':
        connectBtn.disabled = true;
        disconnectBtn.disabled = true;
        break;
      case 'connected':
        connectBtn.disabled = true;
        disconnectBtn.disabled = false;
        this.updateAskButton();
        break;
      case 'disconnected':
        connectBtn.disabled = false;
        disconnectBtn.disabled = true;
        askBtn.disabled = true;
        break;
      case 'error':
        connectBtn.disabled = false;
        disconnectBtn.disabled = true;
        askBtn.disabled = true;
        this.addMessage({
          type: 'error',
          text: `Connection error: ${error.message}`,
          timestamp: new Date().toLocaleTimeString()
        });
        break;
    }
  }
  
  handleSessionChange(session) {
    console.log('Session state changed:', session);
    
    const statusEl = document.getElementById('sessionStatus');
    
    if (session.active) {
      statusEl.className = 'status active';
      statusEl.textContent = `Active: ${session.name}`;
      
      this.addMessage({
        type: 'system',
        text: `Session started: ${session.name}`,
        timestamp: new Date().toLocaleTimeString()
      });
    } else {
      statusEl.className = 'status inactive';
      statusEl.textContent = 'Waiting for session';
      
      this.addMessage({
        type: 'system',
        text: 'Session ended',
        timestamp: new Date().toLocaleTimeString()
      });
    }
    
    this.updateAskButton();
  }
  
  // UI methods
  async connect() {
    try {
      console.log('Connecting with V2 client...');
      await this.client.connect();
      console.log('Connected successfully!');
    } catch (error) {
      console.error('Connection failed:', error);
    }
  }
  
  disconnect() {
    console.log('Disconnecting...');
    this.client.disconnect();
  }
  
  askQuestion() {
    const questionText = document.getElementById('questionInput').value.trim();
    const context = document.getElementById('contextSelect').value;
    const urgent = document.getElementById('urgentCheck').checked;
    
    if (!questionText) return;
    
    try {
      // Use the new simplified message sending API
      const message = this.client.broadcastToInstructors({
        text: questionText,
        context: context,
        urgent: urgent,
        student_id: this.client.userId,
        timestamp: new Date().toISOString()
      });
      
      console.log('Question sent with V2 API:', message);
      
      // Show sent message in UI
      this.addMessage({
        type: 'sent',
        text: questionText,
        context: context,
        urgent: urgent,
        timestamp: new Date().toLocaleTimeString(),
        from: 'You'
      });
      
      // Clear input
      document.getElementById('questionInput').value = '';
      document.getElementById('urgentCheck').checked = false;
      this.updateAskButton();
      
    } catch (error) {
      console.error('Failed to send question:', error);
      this.addMessage({
        type: 'error',
        text: `Failed to send question: ${error.message}`,
        timestamp: new Date().toLocaleTimeString()
      });
    }
  }
  
  updateAskButton() {
    const askBtn = document.getElementById('askBtn');
    const questionText = document.getElementById('questionInput').value.trim();
    const canSend = (
      questionText.length > 0 &&
      this.client.isConnected() &&
      this.client.isSessionActive()
    );
    
    askBtn.disabled = !canSend;
    
    if (!this.client.isConnected()) {
      askBtn.textContent = 'Connect First';
    } else if (!this.client.isSessionActive()) {
      askBtn.textContent = 'Waiting for Session';
    } else {
      askBtn.textContent = 'Ask Question';
    }
  }
  
  addMessage(message) {
    const messagesList = document.getElementById('messagesList');
    const messageEl = document.createElement('div');
    messageEl.className = `message ${message.type}${message.urgent ? ' urgent' : ''}`;
    
    const contextBadge = message.context ? `<span class="context-badge">${message.context}</span>` : '';
    const urgentBadge = message.urgent ? '<span class="urgent-badge">URGENT</span>' : '';
    
    messageEl.innerHTML = `
      <div class="message-header">
        <span class="sender">${message.from || 'System'}</span>
        <span class="timestamp">${message.timestamp}</span>
        ${contextBadge}
        ${urgentBadge}
      </div>
      <div class="message-content">${message.text}</div>
    `;
    
    messagesList.appendChild(messageEl);
    messagesList.scrollTop = messagesList.scrollHeight;
  }
}

// Initialize the app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
  console.log('Initializing Switchboard Student App V2...');
  new StudentAppV2();
});