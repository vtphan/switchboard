/**
 * Teacher App using SwitchboardClient V2
 * 
 * Demonstrates instructor session management and the simplified message sending API.
 */

import SwitchboardClient from '../../switchboard-client.js';

class TeacherAppV2 {
  constructor() {
    this.client = null;
    this.setupUI();
    this.initializeClient();
  }
  
  setupUI() {
    // Bind event handlers to existing HTML elements
    document.getElementById('connectBtn').onclick = () => this.connect();
    document.getElementById('disconnectBtn').onclick = () => this.disconnect();
    document.getElementById('startSessionBtn').onclick = () => this.startSession();
    document.getElementById('endSessionBtn').onclick = () => this.endSession();
    document.getElementById('announceBtn').onclick = () => this.sendAnnouncement();
    
    // Update button states on input
    document.getElementById('announcementInput').oninput = () => this.updateSendButton();
    document.getElementById('sessionNameInput').oninput = () => this.updateSessionButtons();
  }
  
  initializeClient() {
    this.client = new SwitchboardClient({
      userId: 'instructor-' + Math.random().toString(36).substr(2, 9),
      role: 'instructor',
      wsUrl: 'ws://localhost:8080/ws',
      apiUrl: 'http://localhost:8080/api',
      
      // 4 message type hooks
      onBroadcastToInstructors: (message) => this.handleStudentQuestion(message),
      onDirectMessage: (message) => this.handleDirectMessage(message),
      onSystem: (message) => this.handleSystemMessage(message),
      
      // 2 state change hooks
      onConnectionChange: (state, error) => this.handleConnectionChange(state, error),
      onSessionChange: (session) => this.handleSessionChange(session)
    });
    
    console.log('Teacher client initialized with V2 API');
  }
  
  // Event handlers for the 6 hooks
  handleStudentQuestion(message) {
    console.log('Received student question:', message);
    this.addQuestion({
      id: Date.now(),
      text: message.content.text,
      context: message.context,
      urgent: message.content.urgent,
      studentId: message.content.student_id || message.from_user || 'Unknown',
      timestamp: new Date().toLocaleTimeString()
    });
  }
  
  handleDirectMessage(message) {
    console.log('Received direct message:', message);
    // Handle direct messages if needed
  }
  
  handleSystemMessage(message) {
    console.log('System message:', message);
    // System messages handled internally
  }
  
  handleConnectionChange(state, error) {
    console.log('Connection state changed:', state, error);
    
    const statusEl = document.getElementById('connectionStatus');
    const connectBtn = document.getElementById('connectBtn');
    const disconnectBtn = document.getElementById('disconnectBtn');
    
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
        this.updateSessionButtons();
        this.updateSendButton();
        break;
      case 'disconnected':
        connectBtn.disabled = false;
        disconnectBtn.disabled = true;
        this.disableAllControls();
        break;
      case 'error':
        connectBtn.disabled = false;
        disconnectBtn.disabled = true;
        this.disableAllControls();
        break;
    }
  }
  
  handleSessionChange(session) {
    console.log('Session state changed:', session);
    
    const statusEl = document.getElementById('sessionStatus');
    const startBtn = document.getElementById('startSessionBtn');
    const endBtn = document.getElementById('endSessionBtn');
    
    if (session.active) {
      statusEl.className = 'status active';
      statusEl.textContent = `Active: ${session.name}`;
      startBtn.disabled = true;
      endBtn.disabled = false;
    } else {
      statusEl.className = 'status inactive';
      statusEl.textContent = 'No session';
      endBtn.disabled = true;
      this.updateSessionButtons();
    }
    
    this.updateSendButton();
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
  
  async startSession() {
    const sessionName = document.getElementById('sessionNameInput').value.trim();
    if (!sessionName) return;
    
    try {
      console.log('Starting session:', sessionName);
      const result = await this.client.startSession(sessionName);
      console.log('Session started:', result);
    } catch (error) {
      console.error('Failed to start session:', error);
      alert(`Failed to start session: ${error.message}`);
    }
  }
  
  async endSession() {
    try {
      console.log('Ending session...');
      const result = await this.client.endSession();
      console.log('Session ended:', result);
    } catch (error) {
      console.error('Failed to end session:', error);
      alert(`Failed to end session: ${error.message}`);
    }
  }
  
  sendAnnouncement() {
    const announcementText = document.getElementById('announcementInput').value.trim();
    const context = document.getElementById('announcementContextSelect').value;
    const important = document.getElementById('importantCheck').checked;
    const tagsText = document.getElementById('tagsInput').value.trim();
    
    if (!announcementText) return;
    
    try {
      // Build message object with all properties
      const messageContent = {
        text: announcementText,
        context: context,
        important: important,
        instructor_id: this.client.userId,
        timestamp: new Date().toISOString()
      };
      
      // Add tags if provided
      if (tagsText) {
        messageContent.tags = tagsText.split(',').map(tag => tag.trim()).filter(tag => tag);
      }
      
      // Use the new simplified message sending API
      const message = this.client.broadcastToStudents(messageContent);
      
      console.log('Announcement sent with V2 API:', message);
      
      // Show protocol structure in demo section
      this.showProtocolDemo(message);
      
      // Clear inputs
      document.getElementById('announcementInput').value = '';
      document.getElementById('importantCheck').checked = false;
      document.getElementById('tagsInput').value = '';
      this.updateSendButton();
      
    } catch (error) {
      console.error('Failed to send announcement:', error);
      alert(`Failed to send announcement: ${error.message}`);
    }
  }
  
  showProtocolDemo(message) {
    const demoEl = document.getElementById('protocolDemo');
    demoEl.innerHTML = `
      <h4>Last Sent Message Protocol Structure:</h4>
      <pre class="protocol-structure">${JSON.stringify(message, null, 2)}</pre>
      <div class="protocol-analysis">
        <p><strong>✅ Protocol Compliance Verified:</strong></p>
        <ul>
          <li>Context field: <code>${message.context}</code> (separate field)</li>
          <li>Content keys: <code>${Object.keys(message.content).join(', ')}</code></li>
          <li>Context in content: ${message.content.context === undefined ? '✅ No (correct)' : '❌ Yes (double-nested!)'}</li>
        </ul>
      </div>
    `;
  }
  
  updateSessionButtons() {
    const startBtn = document.getElementById('startSessionBtn');
    const sessionName = document.getElementById('sessionNameInput').value.trim();
    
    startBtn.disabled = !(
      this.client.isConnected() &&
      !this.client.isSessionActive() &&
      sessionName.length > 0
    );
  }
  
  updateSendButton() {
    const announceBtn = document.getElementById('announceBtn');
    const announcementText = document.getElementById('announcementInput').value.trim();
    
    const canSend = (
      announcementText.length > 0 &&
      this.client.isConnected() &&
      this.client.isSessionActive()
    );
    
    announceBtn.disabled = !canSend;
    
    if (!this.client.isConnected()) {
      announceBtn.textContent = 'Connect First';
    } else if (!this.client.isSessionActive()) {
      announceBtn.textContent = 'Start Session First';
    } else {
      announceBtn.textContent = 'Send Announcement';
    }
  }
  
  disableAllControls() {
    document.getElementById('startSessionBtn').disabled = true;
    document.getElementById('endSessionBtn').disabled = true;
    document.getElementById('announceBtn').disabled = true;
  }
  
  addQuestion(question) {
    const questionsList = document.getElementById('questionsList');
    const questionEl = document.createElement('div');
    questionEl.className = `message question${question.urgent ? ' urgent' : ''}`;
    
    const contextBadge = `<span class="context-badge">${question.context}</span>`;
    const urgentBadge = question.urgent ? '<span class="urgent-badge">URGENT</span>' : '';
    
    questionEl.innerHTML = `
      <div class="message-header">
        <span class="sender">${question.studentId}</span>
        <span class="timestamp">${question.timestamp}</span>
        ${contextBadge}
        ${urgentBadge}
      </div>
      <div class="message-content">${question.text}</div>
      <div class="message-actions">
        <button class="btn small" onclick="this.closest('.message').style.opacity='0.5'">Mark as Read</button>
      </div>
    `;
    
    questionsList.appendChild(questionEl);
    questionsList.scrollTop = questionsList.scrollHeight;
  }
}

// Initialize the app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
  console.log('Initializing Switchboard Teacher App V2...');
  new TeacherAppV2();
});