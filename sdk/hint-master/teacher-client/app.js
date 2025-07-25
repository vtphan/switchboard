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
    this.teacher = new SwitchboardSDK.SwitchboardTeacher('teacher_001');
    this.currentSession = null;
    this.experts = new Map();
    
    // Initialize experts
    EXPERTS.forEach(expert => {
      this.experts.set(expert.id, { ...expert, connected: false, hints: [] });
    });
    
    this.setupEventHandlers();
    this.generateExpertPanels();
  }

  setupEventHandlers() {
    // SDK event handlers - aligned with guideline message types
    this.teacher.setupEventHandlers({
      onStudentQuestion: (message) => this.handleInstructorInbox(message),
      onStudentResponse: (message) => this.handleRequestResponse(message), 
      onStudentAnalytics: (message) => this.handleAnalytics(message),
      onConnection: (connected) => this.updateStatus(connected ? 'Connected' : 'Disconnected'),
      onSystem: (message) => this.handleSystemMessage(message),
      onHistoryComplete: () => console.log('Message history loaded'),
      onError: (error) => this.handleError(error)
    });

    // UI event handlers
    document.getElementById('createSessionBtn').onclick = () => this.createSession();
    document.getElementById('listSessionsBtn').onclick = () => this.listSessions();
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
      return `<div class="hint-content">
        ${expertData.hints.map(h => `<p style="margin: 8px 0; line-height: 1.5;">${h}</p>`).join('')}
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
      this.updateStatus('Disconnected');
      
      // Refresh session list if it's visible
      const sessionList = document.getElementById('sessionList');
      if (sessionList.style.display === 'block') {
        this.listSessions();
      }
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
      await this.teacher.sendBroadcast('problem', {
        text: problem,
        code: document.getElementById('codeSnapshot').value.trim(),
        timeOnTask: parseInt(document.getElementById('timeOnTask').value) || 7,
        remainingTime: parseInt(document.getElementById('remainingTime').value) || 8,
        frustrationLevel: parseInt(document.getElementById('frustrationLevel').value) || 2
      });
      
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
      expert.hints.push(hint);
      
      const hintsDiv = document.getElementById(`hints-${expertId}`);
      if (hintsDiv) {
        hintsDiv.innerHTML = `
          <div class="hint-content">
            ${expert.hints.map(h => `<p style="margin: 8px 0; line-height: 1.5;">${h}</p>`).join('')}
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
      await this.teacher.sendResponse(expertId, context, content);
    } catch (error) {
      console.error('Failed to send response:', error);
    }
  }

  // Send direct request to a specific expert (request)
  async sendRequestToExpert(expertId, context, content) {
    try {
      await this.teacher.sendRequest(expertId, context, content);
    } catch (error) {
      console.error('Failed to send request:', error);
    }
  }

  async listSessions() {
    try {
      const sessions = await this.teacher.listActiveSessions();
      this.displaySessionList(sessions);
    } catch (error) {
      this.showBroadcastStatus('Failed to list sessions: ' + error.message, 'error');
    }
  }
  
  async selectSession(sessionId, sessionName) {
    try {
      // If already connected to a session, disconnect first
      if (this.currentSession) {
        this.teacher.disconnect();
        this.resetSessionUI();
      }
      
      this.updateStatus('Connecting to session...');
      
      // Connect to the selected session
      await this.teacher.connect(sessionId);
      
      // Update current session info
      this.currentSession = { id: sessionId, name: sessionName };
      
      // Update UI
      document.getElementById('currentSession').style.display = 'block';
      document.getElementById('currentSessionId').textContent = sessionId.substring(0, 8) + '...';
      document.getElementById('totalExperts').textContent = EXPERTS.length;
      document.getElementById('broadcastBtn').disabled = false;
      document.getElementById('endSessionBtn').disabled = false;
      document.getElementById('createSessionBtn').disabled = true;
      
      this.updateStatus('Connected');
      this.validateForm();
      
      // Refresh the session list to update button states
      this.listSessions();
      
    } catch (error) {
      this.updateStatus('Failed to connect to session: ' + error.message);
    }
  }

  displaySessionList(sessions) {
    const container = document.getElementById('sessionsContainer');
    const sessionList = document.getElementById('sessionList');
    
    if (sessions.length === 0) {
      container.innerHTML = '<p>No active sessions found.</p>';
    } else {
      container.innerHTML = sessions.map(session => {
        const isCurrentSession = this.currentSession && this.currentSession.id === session.id;
        const buttonText = isCurrentSession ? 'Current Session' : 'Join Session';
        const buttonClass = isCurrentSession ? 'secondary-btn' : 'primary-btn';
        const buttonDisabled = isCurrentSession ? 'disabled' : '';
        
        return `
          <div class="session-item ${isCurrentSession ? 'active' : ''}" data-session-id="${session.id}">
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <div>
                <strong>${session.name}</strong><br>
                <small>ID: ${session.id.substring(0, 8)}... | Students: ${session.student_ids ? session.student_ids.length : 0} | Connections: ${session.connection_count || 0}</small>
              </div>
              <button 
                class="${buttonClass}" 
                onclick="app.selectSession('${session.id}', '${session.name.replace(/'/g, "\\'")}')"
                ${buttonDisabled}
              >
                ${buttonText}
              </button>
            </div>
          </div>
        `;
      }).join('');
    }
    
    sessionList.style.display = 'block';
    // Don't auto-hide the session list anymore
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