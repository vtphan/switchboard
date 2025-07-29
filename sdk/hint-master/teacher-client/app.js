// Hint Master Teacher Client - Using Browser-Compatible SDK
// Uses global SwitchboardSDK from switchboard-sdk.js

// Expert configurations
const EXPERTS = [
  { id: 'technical_expert', name: 'Technical Expert', icon: '🧠' },
  { id: 'emotional_support_coach', name: 'Emotional Support', icon: '💖' },
  { id: 'algorithm_expert', name: 'Algorithm Expert', icon: '⚡' },
  { id: 'web_dev_expert', name: 'Web Dev Expert', icon: '🌐' },
  { id: 'caring_instructor', name: 'Caring Instructor', icon: '👩‍🏫' },
  { id: 'peer_student', name: 'Study Buddy', icon: '🎓' }
];

class HintMasterApp {
  constructor() {
    this.teacher = new SwitchboardTeacher('teacher_001');
    this.currentSession = null;
    this.experts = new Map();
    this.connectedUsers = new Map(); // LOBBY SYSTEM: Track online users
    
    // Initialize experts
    EXPERTS.forEach(expert => {
      this.experts.set(expert.id, { ...expert, connected: false, hints: [] });
    });
    
    this.setupEventHandlers();
    this.generateExpertPanels();
  }

  setupEventHandlers() {
    // SDK event handlers - aligned with guideline message types
    this.teacher.onStudentQuestion = (data) => this.handleInstructorInbox(data.fullMessage);
    this.teacher.onStudentResponse = (data) => this.handleRequestResponse(data.fullMessage);
    this.teacher.onStudentAnalytics = (data) => this.handleAnalytics(data.fullMessage);
    this.teacher.onConnection = (connected) => this.updateStatus(connected ? 'Connected' : 'Disconnected');
    this.teacher.onError = (error) => this.handleError(error);
    
    // LOBBY SYSTEM: Add presence and session management handlers
    this.teacher.onPresence = (data) => {
      if (data.event === 'user_connected') {
        this.connectedUsers.set(data.userId, data);
        this.updatePresenceDisplay();
        console.log(`👥 User connected: ${data.userId} (${data.role})`);
      } else if (data.event === 'user_disconnected') {
        this.connectedUsers.delete(data.userId);
        this.updatePresenceDisplay();
        console.log(`👥 User disconnected: ${data.userId}`);
      }
    };
    
    this.teacher.onSessionEvent = (data) => {
      if (data.event === 'session_left') {
        console.log(`🔄 Left session: ${data.sessionId}`);
        // Reset UI when session ends
        this.resetSessionUI();
        this.updateStatus('Connected to Lobby');
        // Clear experts list since they're no longer in session
        this.experts.forEach(expert => {
          expert.connected = false;
          expert.hints = [];
        });
        this.generateExpertPanels();
      } else if (data.event === 'session_started') {
        if (data.data.instructor_id === this.teacher.instructorId) {
          console.log(`🚀 My session started: ${data.sessionName}`);
          this.updateSessionUI(data.data);
        } else {
          console.log(`ℹ️ Other session started: ${data.sessionName}`);
        }
      }
    };

    // UI event handlers  
    document.getElementById('createSessionBtn').onclick = () => this.createSession();
    document.getElementById('connectBtn').onclick = () => this.connectToSession();
    document.getElementById('endSessionBtn').onclick = () => this.endSession();
    document.getElementById('broadcastBtn').onclick = () => this.broadcastProblem();
    
    // Form validation
    document.getElementById('problemDescription').oninput = () => this.validateForm();
    document.getElementById('codeSnapshot').oninput = () => this.validateForm();
    
    // Frustration level display
    document.getElementById('frustrationLevel').oninput = (e) => {
      document.getElementById('frustrationValue').textContent = e.target.value;
    };
  }

  getExpertHintsHtml(expertId) {
    const expertData = this.experts.get(expertId);
    if (expertData && expertData.hints && expertData.hints.length > 0) {
      // Display the single hint (latest one)
      return `<div class="hint-content">
        <p style="margin: 8px 0; line-height: 1.5;">${expertData.hints[0]}</p>
      </div>`;
    } else {
      return `<div class="no-hints">
        <p>No hints yet from this expert</p>
      </div>`;
    }
  }

  generateExpertPanels() {
    const grid = document.getElementById('expertsGrid');
    grid.innerHTML = '';
    
    // Only show connected experts, or show message if none connected
    const connectedExperts = EXPERTS.filter(expert => this.experts.get(expert.id)?.connected);
    
    if (connectedExperts.length === 0) {
      grid.innerHTML = `
        <div class="no-experts">
          <p>No experts connected yet</p>
          <small>Expert panels will appear when experts join the session</small>
        </div>
      `;
      return;
    }
    
    connectedExperts.forEach(expert => {
      const panel = document.createElement('div');
      panel.className = `expert-panel ${expert.id}`;
      panel.id = `panel-${expert.id}`;
      panel.innerHTML = `
        <div class="expert-header">
          <h3>${expert.icon} ${expert.name}</h3>
          <span class="connection-indicator connected" id="status-${expert.id}">● Connected</span>
        </div>
        <div class="latest-hint" id="hints-${expert.id}">
          ${this.getExpertHintsHtml(expert.id)}
        </div>
      `;
      grid.appendChild(panel);
    });
  }

  async createSession() {
    try {
      this.updateStatus('Creating session...');
      const sessionName = document.getElementById('sessionName').value || 'Hint Master Session';
      const expertIds = EXPERTS.map(e => e.id);
      
      const session = await this.teacher.createAndConnect(sessionName, expertIds);
      this.currentSession = session;
      
      document.getElementById('currentSession').style.display = 'block';
      document.getElementById('currentSessionId').textContent = session.id.substring(0, 8) + '...';
      document.getElementById('totalExperts').textContent = EXPERTS.length;
      document.getElementById('broadcastBtn').disabled = false;
      document.getElementById('endSessionBtn').disabled = false;
      document.getElementById('createSessionBtn').disabled = true;
      
      this.updateStatus('Connected');
      this.validateForm();
    } catch (error) {
      this.updateStatus('Error: ' + error.message);
    }
  }

  async endSession() {
    try {
      await this.teacher.endCurrentSession();
      this.currentSession = null;
      
      this.resetSessionUI();
      this.updateStatus('Connected to Lobby');
      
    } catch (error) {
      this.updateStatus('Error: ' + error.message);
    }
  }

  async broadcastProblem() {
    const problem = document.getElementById('problemDescription').value.trim();
    if (!problem) {
      alert('Please enter a problem description');
      return;
    }

    try {
      // Use instructor_broadcast message type as per guideline
      await this.teacher.announce({
        text: problem,
        code: document.getElementById('codeSnapshot').value.trim(),
        timeOnTask: parseInt(document.getElementById('timeOnTask').value) || 7,
        remainingTime: parseInt(document.getElementById('remainingTime').value) || 8,
        frustrationLevel: parseInt(document.getElementById('frustrationLevel').value) || 2
      }, 'problem');
      
      this.showBroadcastStatus('Problem broadcast to all experts', 'success');
    } catch (error) {
      this.showBroadcastStatus('Broadcast failed: ' + error.message, 'error');
    }
  }

  // Handle instructor_inbox messages (questions from students/experts)
  handleInstructorInbox(message) {
    const expertId = message.from_user;
    const expert = this.experts.get(expertId);
    
    // Mark expert as connected when we receive a message from them
    if (expert && !expert.connected) {
      this.updateExpertConnectionStatus(expertId, true);
    }
    
    if (expert && message.context === 'hint') {
      const hint = message.content.hint || message.content.text || 'No hint';
      
      // Store only the latest hint for each client
      expert.hints = [hint];
      
      const hintsDiv = document.getElementById(`hints-${expertId}`);
      if (hintsDiv) {
        hintsDiv.innerHTML = `
          <div class="hint-content">
            <p style="margin: 8px 0; line-height: 1.5;">${hint}</p>
          </div>
        `;
      }
      
      console.log(`Hint received from ${expert.name}: ${hint}`);
    } else if (expert && message.context === 'question') {
      // Handle questions from experts (if they ask clarifying questions)
      console.log(`Question from ${expert.name}: ${message.content.text}`);
    }
  }

  // Handle request_response messages (responses to teacher requests)
  handleRequestResponse(message) {
    const expertId = message.from_user;
    const expert = this.experts.get(expertId);
    
    if (expert) {
      console.log(`Response from ${expert.name}:`, message.content);
      // Could update UI to show expert responses
    }
  }

  // Handle analytics messages (student activity data)
  handleAnalytics(message) {
    const expertId = message.from_user;
    const expert = this.experts.get(expertId);
    
    if (expert && message.context === 'connection') {
      // Handle expert connection status
      const connected = message.content.event === 'connected';
      this.updateExpertConnectionStatus(expertId, connected);
      console.log(`Expert ${expert.name} ${connected ? 'connected' : 'disconnected'}`);
    } else if (expert) {
      console.log(`Analytics from ${expert.name}:`, message.content);
    }
  }

  // Send direct response to a specific expert (inbox_response)
  async sendResponseToExpert(expertId, context, content) {
    try {
      await this.teacher.respond(expertId, content, context);
    } catch (error) {
      console.error('Failed to send response:', error);
    }
  }

  // Send direct request to a specific expert (request)
  async sendRequestToExpert(expertId, context, content) {
    try {
      await this.teacher.ask(expertId, content, context);
    } catch (error) {
      console.error('Failed to send request:', error);
    }
  }

  async connectToSession() {
    try {
      this.updateStatus('Connecting...');
      
      // Auto-assignment connection - server decides where to place us
      await this.teacher.connect();
      
      // TIMING FIX: Wait a moment for server system messages to be processed
      await new Promise(resolve => setTimeout(resolve, 200));
      
      console.log(`🔍 DEBUG: Connection established. Session ID: ${this.teacher.getSessionId()}, isInSession: ${this.teacher.isInSession()}, isInLobby: ${this.teacher.isInLobby()}`);
      
      if (this.teacher.isInSession()) {
        // We're in an active session
        const sessionId = this.teacher.getSessionId();
        this.currentSession = { id: sessionId, name: 'Active Session' };
        
        // Update UI
        document.getElementById('currentSession').style.display = 'block';
        document.getElementById('currentSessionId').textContent = sessionId.substring(0, 8) + '...';
        document.getElementById('totalExperts').textContent = EXPERTS.length;
        document.getElementById('broadcastBtn').disabled = false;
        document.getElementById('endSessionBtn').disabled = false;
        document.getElementById('createSessionBtn').disabled = true;
        document.getElementById('connectBtn').disabled = true;
        
        this.updateStatus('Connected to Active Session');
        console.log(`✅ DEBUG: UI updated for active session: ${sessionId}`);
      } else if (this.teacher.isInLobby()) {
        // We're in lobby - no active session
        this.updateStatus('Connected to Lobby - No Active Session');
        document.getElementById('createSessionBtn').disabled = false;
        document.getElementById('connectBtn').disabled = true;
        console.log(`🏛️ DEBUG: UI updated for lobby connection`);
      } else {
        console.log(`❓ DEBUG: Unknown connection state - sessionId: ${this.teacher.getSessionId()}`);
      }
      
      this.validateForm();
      
    } catch (error) {
      this.updateStatus('Failed to connect: ' + error.message);
    }
  }


  validateForm() {
    const problem = document.getElementById('problemDescription').value.trim();
    const broadcastBtn = document.getElementById('broadcastBtn');
    
    const isValid = problem.length > 0 && this.currentSession !== null;
    broadcastBtn.disabled = !isValid;
  }

  updateExpertConnectionStatus(expertId, connected) {
    const expert = this.experts.get(expertId);
    if (expert) {
      const wasConnected = expert.connected;
      expert.connected = connected;
      
      // Only regenerate panels if connection status actually changed
      if (wasConnected !== connected) {
        this.generateExpertPanels();
        console.log(`Expert ${expert.name} ${connected ? 'connected' : 'disconnected'}`);
      }
      
      // Update connected count
      const connectedCount = Array.from(this.experts.values()).filter(e => e.connected).length;
      document.getElementById('connectedExperts').textContent = connectedCount;
    }
  }

  resetSessionUI() {
    document.getElementById('currentSession').style.display = 'none';
    document.getElementById('broadcastBtn').disabled = true;
    document.getElementById('endSessionBtn').disabled = true;
    document.getElementById('createSessionBtn').disabled = false;
    // Only enable connect button if we're not already connected
    document.getElementById('connectBtn').disabled = this.teacher.connected;
    this.currentSession = null;
  }

  handleSystemMessage(message) {
    console.log('System message:', message.content);
    
    if (message.content.event === 'session_ended') {
      this.updateStatus('Session ended by system');
      this.resetSessionUI();
    } else if (message.content.event === 'message_error') {
      this.updateStatus(`Message error: ${message.content.error}`);
    }
  }

  handleError(error) {
    console.error('Connection error:', error);
    this.updateStatus('Connection error - check console');
  }

  updateStatus(message) {
    const statusText = document.getElementById('statusText');
    const statusIndicator = document.getElementById('statusIndicator');
    const connectionStatus = document.getElementById('connectionStatus');
    
    if (statusText) {
      statusText.textContent = message;
    }
    
    // Update connection status styling and indicator
    if (message.toLowerCase().includes('connected') && !message.toLowerCase().includes('disconnected')) {
      if (connectionStatus) {
        connectionStatus.className = 'connection-status connected';
      }
      if (statusIndicator) {
        statusIndicator.textContent = '🟢';
      }
    } else if (message.toLowerCase().includes('connecting') || message.toLowerCase().includes('creating')) {
      if (connectionStatus) {
        connectionStatus.className = 'connection-status connecting';
      }
      if (statusIndicator) {
        statusIndicator.textContent = '🟡';
      }
    } else {
      if (connectionStatus) {
        connectionStatus.className = 'connection-status disconnected';
      }
      if (statusIndicator) {
        statusIndicator.textContent = '🔴';
      }
    }
  }
  
  // LOBBY SYSTEM: Presence management methods
  
  updatePresenceDisplay() {
    // Update UI to show who's online
    const onlineCount = this.connectedUsers.size;
    const onlineUsersElement = document.getElementById('onlineUsers');
    
    if (onlineUsersElement) {
      onlineUsersElement.textContent = onlineCount;
    }
    
    // Could add more detailed presence display here
    console.log(`👥 Online users: ${onlineCount}`);
  }
  
  updateSessionUI(sessionData) {
    // Update UI when session is created/started
    if (sessionData.session_id && sessionData.session_name) {
      this.currentSession = {
        id: sessionData.session_id,
        name: sessionData.session_name
      };
      
      // Update session display elements if they exist
      const sessionIdElement = document.getElementById('currentSessionId');
      if (sessionIdElement) {
        sessionIdElement.textContent = sessionData.session_id.substring(0, 8) + '...';
      }
    }
  }
  
  showBroadcastStatus(message, type = 'info') {
    const broadcastStatus = document.getElementById('broadcastStatus');
    if (broadcastStatus) {
      broadcastStatus.innerHTML = `<div class="broadcast-status ${type}">${message}</div>`;
      
      // Clear status after 5 seconds
      setTimeout(() => {
        broadcastStatus.innerHTML = '';
      }, 5000);
    }
  }
}

// Initialize when DOM loads
document.addEventListener('DOMContentLoaded', () => {
  window.app = new HintMasterApp();
});