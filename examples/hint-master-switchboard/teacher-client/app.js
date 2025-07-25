// AI Programming Mentorship - Switchboard Teacher Client Application
// Integrates with Switchboard instead of custom WebSocket server

class TeacherApp {
  constructor() {
    this.client = new TeacherSwitchboardClient();
    this.currentSession = null;
    this.connectedExperts = new Set();
    this.hintsReceived = [];
    this.expertStatusMap = new Map();
    
    this.initializeEventListeners();
    this.initializeExpertStatusTracking();
  }

  initializeEventListeners() {
    // Session management
    document.getElementById('createSessionBtn').addEventListener('click', () => this.createSession());
    document.getElementById('listSessionsBtn').addEventListener('click', () => this.listSessions());
    document.getElementById('endSessionBtn').addEventListener('click', () => this.endSession());

    // Problem configuration
    document.getElementById('broadcastBtn').addEventListener('click', () => this.broadcastProblem());
    document.getElementById('frustrationLevel').addEventListener('input', this.updateFrustrationValue);
    document.getElementById('togglePreview').addEventListener('click', this.toggleCodePreview);

    // Form validation
    const requiredFields = ['problemDescription', 'codeSnapshot'];
    requiredFields.forEach(fieldId => {
      document.getElementById(fieldId).addEventListener('input', this.validateForm);
    });

    // Switchboard client event handlers
    this.client.onHint((hint) => this.handleHintReceived(hint));
    this.client.onExpertConnectionChange((message) => this.handleExpertConnection(message));
    this.client.onError((error) => this.handleError(error));
    
    // Set up client event handlers for UI updates
    this.client.onHistoryLoaded = () => this.handleHistoryLoaded();
    this.client.onReconnected = () => this.updateConnectionStatus('connected', 'Connected to session');
    this.client.onReconnectionFailed = () => this.updateConnectionStatus('error', 'Failed to reconnect');
  }

  initializeExpertStatusTracking() {
    // Initialize expert status map
    const expertNames = [
      'technical_expert',
      'emotional_support_coach', 
      'debugging_guru',
      'learning_coach',
      'architecture_expert'
    ];

    expertNames.forEach(id => {
      this.expertStatusMap.set(id, {
        name: this.formatExpertName(id),
        connected: false,
        lastSeen: null,
        hintCount: 0,
        hintHistory: [],
        averageResponseTime: 0,
        responseTimes: []
      });
    });
  }

  formatExpertName(expertId) {
    const names = {
      'technical_expert': 'Technical Expert',
      'emotional_support_coach': 'Emotional Support Coach',
      'debugging_guru': 'Debugging Guru', 
      'learning_coach': 'Learning Coach',
      'architecture_expert': 'Architecture Expert'
    };
    return names[expertId] || expertId;
  }

  // Session Management
  async createSession() {
    try {
      this.updateConnectionStatus('connecting', 'Creating session...');
      const sessionName = document.getElementById('sessionName').value || 'AI Programming Mentorship Session';
      
      const session = await this.client.createSession(sessionName);
      this.currentSession = session;
      
      // Connect to the session
      await this.client.connectToSession(session.id);
      
      this.updateSessionDisplay(session);
      this.showMainContent();
      this.updateConnectionStatus('connected', `Connected to session: ${session.id.substring(0, 8)}...`);
      
      console.log('✅ Session created and connected:', session.id);
    } catch (error) {
      console.error('❌ Failed to create session:', error);
      this.updateConnectionStatus('error', 'Failed to create session');
      alert('Failed to create session: ' + error.message);
    }
  }

  async listSessions() {
    try {
      const sessions = await this.client.listActiveSessions();
      this.displaySessionList(sessions);
    } catch (error) {
      console.error('❌ Failed to list sessions:', error);
      alert('Failed to list sessions: ' + error.message);
    }
  }

  async endSession() {
    if (!this.currentSession) return;
    
    try {
      await this.client.endSession();
      await this.client.disconnect();
      
      this.currentSession = null;
      this.hideMainContent();
      this.updateConnectionStatus('disconnected', 'Session ended');
      this.resetExpertStatus();
      
      console.log('✅ Session ended successfully');
    } catch (error) {
      console.error('❌ Failed to end session:', error);
      alert('Failed to end session: ' + error.message);
    }
  }

  async connectToExistingSession(sessionId) {
    try {
      this.updateConnectionStatus('connecting', 'Connecting to session...');
      
      await this.client.connectToSession(sessionId);
      
      // Get session details
      const sessions = await this.client.listActiveSessions();
      const session = sessions.find(s => s.id === sessionId);
      
      if (session) {
        this.currentSession = session;
        this.updateSessionDisplay(session);
        this.showMainContent();
        this.updateConnectionStatus('connected', `Connected to session: ${sessionId.substring(0, 8)}...`);
      }
    } catch (error) {
      console.error('❌ Failed to connect to session:', error);
      this.updateConnectionStatus('error', 'Failed to connect');
      alert('Failed to connect to session: ' + error.message);
    }
  }

  // UI Updates
  updateSessionDisplay(session) {
    document.getElementById('currentSessionId').textContent = session.id;
    document.getElementById('currentSessionName').textContent = session.name;
    document.getElementById('currentSessionCreated').textContent = new Date(session.start_time).toLocaleString();
    document.getElementById('enrolledExperts').textContent = '5';
    document.getElementById('connectedExperts').textContent = this.connectedExperts.size;
    
    document.getElementById('currentSession').style.display = 'block';
    document.getElementById('endSessionBtn').disabled = false;
  }

  displaySessionList(sessions) {
    const container = document.getElementById('sessionsContainer');
    const sessionList = document.getElementById('sessionList');
    
    if (sessions.length === 0) {
      container.innerHTML = '<p class="no-sessions">No active sessions found.</p>';
    } else {
      container.innerHTML = sessions.map(session => `
        <div class="session-item ${session.id === this.currentSession?.id ? 'active' : ''}" 
             onclick="app.connectToExistingSession('${session.id}')">
          <div><strong>${session.name}</strong></div>
          <div>ID: ${session.id.substring(0, 8)}...</div>
          <div>Created: ${new Date(session.start_time).toLocaleString()}</div>
          <div>Students: ${session.student_ids.length}</div>
        </div>
      `).join('');
    }
    
    sessionList.style.display = 'block';
  }

  showMainContent() {
    document.getElementById('mainContent').style.display = 'grid';
    this.validateForm();
  }

  hideMainContent() {
    document.getElementById('mainContent').style.display = 'none';
    document.getElementById('currentSession').style.display = 'none';
    document.getElementById('sessionList').style.display = 'none';
    document.getElementById('endSessionBtn').disabled = true;
  }

  updateConnectionStatus(status, message) {
    const statusElement = document.getElementById('connectionStatus');
    const indicatorElement = document.getElementById('statusIndicator');
    const textElement = document.getElementById('statusText');
    
    statusElement.className = `connection-status ${status}`;
    textElement.textContent = message;
    
    const indicators = {
      connected: '🟢',
      connecting: '🟡', 
      disconnected: '🔴',
      error: '❌'
    };
    
    indicatorElement.textContent = indicators[status] || '🔴';
  }

  // Expert Status Management
  handleExpertConnection(message) {
    const expertId = message.from_user;
    const event = message.content.event;
    
    if (this.expertStatusMap.has(expertId)) {
      const expert = this.expertStatusMap.get(expertId);
      
      if (event === 'connected' || event === 'session_joined') {
        expert.connected = true;
        expert.lastSeen = new Date();
        this.connectedExperts.add(expertId);
      } else if (event === 'disconnected') {
        expert.connected = false;
        this.connectedExperts.delete(expertId);
      }
      
      this.updateExpertPanelStatus(expertId, expert.connected);
      this.updateConnectedExpertsCount();
    }
  }

  updateExpertPanelStatus(expertId, connected) {
    const statusElement = document.getElementById(`status-${expertId}`);
    if (statusElement) {
      if (connected) {
        statusElement.className = 'connection-status connected';
      } else {
        statusElement.className = 'connection-status disconnected';
      }
    }
  }

  updateConnectedExpertsCount() {
    const countElement = document.getElementById('connectedExperts');
    if (countElement) {
      countElement.textContent = this.connectedExperts.size;
    }
  }

  resetExpertStatus() {
    this.connectedExperts.clear();
    this.expertStatusMap.forEach((expert, expertId) => {
      expert.connected = false;
      expert.lastSeen = null;
      this.updateExpertPanelStatus(expertId, false);
    });
    this.updateConnectedExpertsCount();
  }

  // Problem Broadcasting
  async broadcastProblem() {
    if (!this.client.connected) {
      alert('Not connected to a session');
      return;
    }

    const problemData = {
      problem: document.getElementById('problemDescription').value,
      code: document.getElementById('codeSnapshot').value,
      timeOnTask: parseInt(document.getElementById('timeOnTask').value) || 15,
      remainingTime: parseInt(document.getElementById('remainingTime').value) || 30,
      frustrationLevel: parseInt(document.getElementById('frustrationLevel').value) || 2
    };

    try {
      // Track broadcast time for response time calculation
      this.lastBroadcastTime = new Date();
      
      this.client.broadcastProblem(problemData);
      this.displayBroadcastConfirmation(problemData);
      this.updateProblemSummary(problemData);
      this.clearHintsFromPanels(); // Clear previous hints for new problem
      
      console.log('📢 Problem broadcasted to experts');
    } catch (error) {
      console.error('❌ Failed to broadcast problem:', error);
      alert('Failed to broadcast problem: ' + error.message);
    }
  }

  displayBroadcastConfirmation(problemData) {
    const statusElement = document.getElementById('broadcastStatus');
    statusElement.innerHTML = `
      <div class="broadcast-success">
        ✅ Problem broadcasted to ${this.connectedExperts.size} connected experts
        <div class="broadcast-time">Sent at ${new Date().toLocaleTimeString()}</div>
      </div>
    `;
    
    // Clear after 5 seconds
    setTimeout(() => {
      statusElement.innerHTML = '';
    }, 5000);
  }

  updateProblemSummary(problemData) {
    document.getElementById('summaryTimeOnTask').textContent = problemData.timeOnTask;
    document.getElementById('summaryRemainingTime').textContent = problemData.remainingTime;
    document.getElementById('summaryFrustration').textContent = problemData.frustrationLevel;
    document.getElementById('summaryTimestamp').textContent = new Date().toLocaleString();
    
    document.getElementById('problemSummary').style.display = 'block';
  }

  // Hint Management
  handleHintReceived(hint) {
    this.hintsReceived.push(hint);
    
    // Determine expert ID from the hint
    const expertId = this.getExpertIdFromHint(hint);
    if (expertId) {
      this.displayHintInExpertPanel(expertId, hint);
      this.updateExpertMetrics(expertId, hint);
    }
    
    console.log(`💡 Hint received from ${hint.expert.name}`);
  }

  getExpertIdFromHint(hint) {
    // Map expert names to IDs
    const nameToId = {
      'Technical Expert': 'technical_expert',
      'Emotional Support Coach': 'emotional_support_coach',
      'Debugging Guru': 'debugging_guru',
      'Learning Coach': 'learning_coach',
      'Architecture Expert': 'architecture_expert'
    };
    
    return nameToId[hint.expert.name] || null;
  }

  displayHintInExpertPanel(expertId, hint) {
    // Hide "no hints" message
    const emptyState = document.getElementById(`empty-${expertId}`);
    if (emptyState) {
      emptyState.style.display = 'none';
    }

    // Show latest hint section
    const latestHint = document.getElementById(`latest-${expertId}`);
    if (latestHint) {
      latestHint.style.display = 'block';
      
      // Update content
      document.getElementById(`timestamp-${expertId}`).textContent = this.formatTime(hint.timestamp);
      document.getElementById(`content-${expertId}`).textContent = hint.hint;
    }

    // Add to history
    this.addToExpertHistory(expertId, hint);
  }

  addToExpertHistory(expertId, hint) {
    const expert = this.expertStatusMap.get(expertId);
    if (expert) {
      // Move current hint to history if it exists
      if (expert.hintHistory.length > 0 || document.getElementById(`latest-${expertId}`).style.display !== 'none') {
        const currentContent = document.getElementById(`content-${expertId}`).textContent;
        const currentTimestamp = document.getElementById(`timestamp-${expertId}`).textContent;
        
        if (currentContent && currentContent !== hint.hint) {
          expert.hintHistory.unshift({
            content: currentContent,
            timestamp: currentTimestamp
          });
        }
      }

      // Add new hint
      expert.hintHistory.unshift({
        content: hint.hint,
        timestamp: this.formatTime(hint.timestamp)
      });

      // Keep only last 10 hints in history
      if (expert.hintHistory.length > 10) {
        expert.hintHistory = expert.hintHistory.slice(0, 10);
      }

      this.updateExpertHistoryDisplay(expertId);
    }
  }

  updateExpertHistoryDisplay(expertId) {
    const expert = this.expertStatusMap.get(expertId);
    if (!expert || expert.hintHistory.length === 0) return;

    const historyList = document.getElementById(`history-list-${expertId}`);
    const historyCount = document.getElementById(`history-count-${expertId}`);
    
    if (historyList && historyCount) {
      historyCount.textContent = expert.hintHistory.length;
      
      historyList.innerHTML = expert.hintHistory.map(hint => `
        <div class="history-item">
          <span class="history-timestamp">${hint.timestamp}</span>
          ${this.formatHintContent(hint.content)}
        </div>
      `).join('');
    }
  }

  updateExpertMetrics(expertId, hint) {
    const expert = this.expertStatusMap.get(expertId);
    if (expert) {
      expert.hintCount++;
      
      // Calculate response time (if we have broadcast time)
      if (this.lastBroadcastTime) {
        const responseTime = (new Date(hint.timestamp) - this.lastBroadcastTime) / 1000;
        expert.responseTimes.push(responseTime);
        
        // Keep only last 5 response times for average
        if (expert.responseTimes.length > 5) {
          expert.responseTimes = expert.responseTimes.slice(-5);
        }
        
        expert.averageResponseTime = expert.responseTimes.reduce((a, b) => a + b, 0) / expert.responseTimes.length;
      }

      // Update UI
      document.getElementById(`count-${expertId}`).textContent = `${expert.hintCount} hints`;
      if (expert.averageResponseTime > 0) {
        document.getElementById(`time-${expertId}`).textContent = `~${Math.round(expert.averageResponseTime)}s response`;
      }
    }
  }

  formatHintContent(content) {
    // Simple formatting for hint content
    return content.replace(/\n/g, '<br>').replace(/`([^`]+)`/g, '<code>$1</code>');
  }

  formatTime(timestamp) {
    return new Date(timestamp).toLocaleTimeString();
  }

  clearHintsFromPanels() {
    // Clear hints from all expert panels
    const expertIds = [
      'technical_expert',
      'emotional_support_coach', 
      'debugging_guru',
      'learning_coach',
      'architecture_expert'
    ];

    expertIds.forEach(expertId => {
      // Hide latest hint
      const latestHint = document.getElementById(`latest-${expertId}`);
      if (latestHint) {
        latestHint.style.display = 'none';
      }
      
      // Show empty state
      const emptyState = document.getElementById(`empty-${expertId}`);
      if (emptyState) {
        emptyState.style.display = 'block';
      }
      
      // Reset expert metrics
      const expert = this.expertStatusMap.get(expertId);
      if (expert) {
        expert.hintCount = 0;
        expert.hintHistory = [];
        expert.responseTimes = [];
        expert.averageResponseTime = 0;
        
        // Update UI
        document.getElementById(`count-${expertId}`).textContent = '0 hints';
        document.getElementById(`time-${expertId}`).textContent = '~0s response';
        document.getElementById(`history-count-${expertId}`).textContent = '0';
        
        const historyList = document.getElementById(`history-list-${expertId}`);
        if (historyList) {
          historyList.innerHTML = '';
        }
      }
    });
    
    this.hintsReceived = [];
  }

  // Form Validation and UI Helpers
  validateForm() {
    const problem = document.getElementById('problemDescription').value.trim();
    const code = document.getElementById('codeSnapshot').value.trim();
    const broadcastBtn = document.getElementById('broadcastBtn');
    
    const isValid = problem.length > 0 && code.length > 0;
    broadcastBtn.disabled = !isValid;
  }

  updateFrustrationValue() {
    const slider = document.getElementById('frustrationLevel');
    const valueDisplay = document.getElementById('frustrationValue');
    valueDisplay.textContent = slider.value;
  }

  toggleCodePreview() {
    const textarea = document.getElementById('codeSnapshot');
    const preview = document.getElementById('codePreview');
    const button = document.getElementById('togglePreview');
    
    if (preview.style.display === 'none') {
      // Show preview
      const code = textarea.value;
      
      document.getElementById('highlightedCode').textContent = code;
      document.getElementById('highlightedCode').className = 'language-javascript';
      
      if (typeof hljs !== 'undefined') {
        hljs.highlightAll();
      }
      
      textarea.style.display = 'none';
      preview.style.display = 'block';
      button.textContent = '✏️ Edit';
    } else {
      // Show textarea
      textarea.style.display = 'block';
      preview.style.display = 'none';
      button.textContent = '👁️ Preview';
    }
  }

  // Copy hint to clipboard
  copyHint(expertId) {
    const hintContent = document.getElementById(`content-${expertId}`);
    if (hintContent && hintContent.textContent) {
      navigator.clipboard.writeText(hintContent.textContent).then(() => {
        // Show temporary feedback
        const copyBtn = document.querySelector(`[onclick="copyHint('${expertId}')"]`);
        if (copyBtn) {
          const originalText = copyBtn.textContent;
          copyBtn.textContent = '✅';
          setTimeout(() => {
            copyBtn.textContent = originalText;
          }, 1000);
        }
      }).catch(err => {
        console.error('Failed to copy hint:', err);
        alert('Failed to copy hint to clipboard');
      });
    }
  }

  // History and System Message Handling
  handleHistoryLoaded() {
    document.getElementById('sessionHistory').style.display = 'block';
    const historyContainer = document.getElementById('historyContainer');
    historyContainer.innerHTML = '<p>✅ Session history loaded. Previous messages will appear above as you interact.</p>';
  }

  handleError(error) {
    console.error('Expert error:', error);
    // Could display expert errors in the UI if needed
  }
}

// Initialize the application when the page loads
let app;
document.addEventListener('DOMContentLoaded', () => {
  app = new TeacherApp();
  console.log('🎓 Switchboard Teacher App initialized');
});

// Global function for copy buttons in HTML
function copyHint(expertId) {
  if (app) {
    app.copyHint(expertId);
  }
}