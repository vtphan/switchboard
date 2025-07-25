// AI Programming Mentorship - Switchboard Teacher Client Application
// Direct Switchboard integration without wrapper classes

class TeacherApp {
  constructor() {
    // Switchboard connection settings
    this.switchboardUrl = 'http://localhost:8080';
    this.instructorId = 'teacher_001';
    this.ws = null;
    this.connected = false;
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
    
    // Session and expert management
    this.currentSession = null;
    this.expertIds = [
      'technical_expert',
      'emotional_support_coach', 
      'debugging_guru',
      'learning_coach',
      'architecture_expert'
    ];
    
    // UI state
    this.hintsReceived = [];
    this.lastBroadcastTime = null;
    
    // Consolidated expert state
    this.experts = new Map();
    this.initializeExperts();
    
    this.initializeEventListeners();
  }

  initializeEventListeners() {
    // Session management
    document.getElementById('createSessionBtn').addEventListener('click', () => this.createSession());
    document.getElementById('listSessionsBtn').addEventListener('click', () => this.listSessions());
    document.getElementById('endSessionBtn').addEventListener('click', () => this.endSession());

    // Problem configuration
    document.getElementById('broadcastBtn').addEventListener('click', () => this.broadcastProblem());
    document.getElementById('frustrationLevel').addEventListener('input', this.updateFrustrationValue);

    // Form validation
    const requiredFields = ['problemDescription', 'codeSnapshot'];
    requiredFields.forEach(fieldId => {
      document.getElementById(fieldId).addEventListener('input', this.validateForm);
    });

  }

  initializeExperts() {
    // Initialize consolidated expert state
    this.expertIds.forEach(id => {
      this.experts.set(id, {
        id: id,
        name: this.formatExpertName(id),
        connected: false,
        lastSeen: null,
        hintCount: 0,
        hintHistory: [],
        averageResponseTime: 0,
        responseTimes: []
      });
      
      // Initialize UI state - start all as disconnected
      this.updateExpertPanelStatus(id, false);
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

  // Session Management - Direct Switchboard integration
  async createSession() {
    try {
      this.updateConnectionStatus('connecting', 'Creating session...');
      const sessionName = document.getElementById('sessionName').value || 'AI Programming Mentorship Session';
      
      // Create session via Switchboard API
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
      this.currentSession = data.session;
      console.log(`✅ Session created: ${data.session.id}`);
      
      // Connect to the session via WebSocket
      await this.connectToSession(data.session.id);
      
      this.updateSessionDisplay(data.session);
      this.showMainContent();
      this.updateConnectionStatus('connected', `Connected to session: ${data.session.id.substring(0, 8)}...`);
      
      console.log('✅ Session created and connected:', data.session.id);
    } catch (error) {
      console.error('❌ Failed to create session:', error);
      this.updateConnectionStatus('error', 'Failed to create session');
      alert('Failed to create session: ' + error.message);
    }
  }

  async listSessions() {
    try {
      const response = await fetch(`${this.switchboardUrl}/api/sessions`);
      if (!response.ok) {
        throw new Error(`Failed to list sessions: ${response.statusText}`);
      }
      
      const data = await response.json();
      const activeSessions = data.sessions.filter(s => s.status === 'active');
      this.displaySessionList(activeSessions);
    } catch (error) {
      console.error('❌ Failed to list sessions:', error);
      alert('Failed to list sessions: ' + error.message);
    }
  }

  async endSession() {
    if (!this.currentSession) return;
    
    try {
      // End session via Switchboard API
      const response = await fetch(`${this.switchboardUrl}/api/sessions/${this.currentSession.id}`, {
        method: 'DELETE'
      });

      if (response.ok) {
        console.log(`✅ Session ${this.currentSession.id} ended`);
      }
      
      // Don't disconnect immediately - let the session_ended system message handle cleanup
      // The server will send a session_ended message which will trigger proper cleanup
      console.log('⏳ Waiting for session_ended system message to complete cleanup...');
      
      console.log('✅ Session ended successfully');
    } catch (error) {
      console.error('❌ Failed to end session:', error);
      alert('Failed to end session: ' + error.message);
    }
  }

  async connectToExistingSession(sessionId) {
    try {
      this.updateConnectionStatus('connecting', 'Connecting to session...');
      
      await this.connectToSession(sessionId);
      
      // Get session details
      const response = await fetch(`${this.switchboardUrl}/api/sessions`);
      if (response.ok) {
        const data = await response.json();
        const session = data.sessions.find(s => s.id === sessionId);
        
        if (session) {
          this.currentSession = session;
          this.updateSessionDisplay(session);
          this.showMainContent();
          this.updateConnectionStatus('connected', `Connected to session: ${sessionId.substring(0, 8)}...`);
        }
      }
    } catch (error) {
      console.error('❌ Failed to connect to session:', error);
      this.updateConnectionStatus('error', 'Failed to connect');
      alert('Failed to connect to session: ' + error.message);
    }
  }

  // WebSocket Connection Management
  async connectToSession(sessionId) {
    try {
      const wsUrl = `ws://localhost:8080/ws?user_id=${this.instructorId}&role=instructor&session_id=${sessionId}`;
      
      this.ws = new WebSocket(wsUrl);

      return new Promise((resolve, reject) => {
        this.ws.onopen = () => {
          this.connected = true;
          this.reconnectAttempts = 0;
          console.log(`🔗 Connected to session: ${sessionId}`);
          resolve();
        };

        this.ws.onmessage = (event) => {
          console.log('🔍 DEBUG: Raw WebSocket message received:', event.data);
          try {
            const message = JSON.parse(event.data);
            console.log('🔍 DEBUG: Parsed message:', message);
            this.handleMessage(message);
          } catch (error) {
            console.error('❌ Failed to parse WebSocket message:', error, event.data);
          }
        };

        this.ws.onclose = () => {
          this.connected = false;
          console.log('🔌 WebSocket connection closed');
          if (this.currentSession) {
            this.attemptReconnection();
          }
        };

        this.ws.onerror = (error) => {
          console.error('❌ WebSocket error:', error);
          this.connected = false;
          reject(error);
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

  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.connected = false;
  }

  async attemptReconnection() {
    this.reconnectAttempts++;
    const delay = Math.pow(2, this.reconnectAttempts) * 1000; // Exponential backoff
    
    console.log(`🔄 Reconnecting in ${delay/1000}s (attempt ${this.reconnectAttempts})`);
    
    setTimeout(async () => {
      try {
        await this.connectToSession(this.currentSession.id);
        this.updateConnectionStatus('connected', 'Connected to session');
      } catch (error) {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
          this.attemptReconnection();
        } else {
          this.updateConnectionStatus('error', 'Failed to reconnect');
        }
      }
    }, delay);
  }

  // Message Handling - Direct Switchboard protocol
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
        if (message.context === 'connection') {
          this.handleExpertConnection(message);
        }
        break;
        
      case 'system':
        this.handleSystemMessage(message);
        break;
        
      default:
        console.log('📨 Received message:', message);
    }
  }

  handleHintReceived(message) {
    // Robust hint object construction with fallbacks
    const expertName = message.content?.expert?.name || message.from_user || 'Unknown Expert';
    const expertExpertise = message.content?.expert?.expertise || 'General';
    const hintContent = message.content?.hint || 'No hint content';
    
    const hint = {
      type: 'hint_received',
      expert: {
        name: expertName,
        expertise: expertExpertise,
      },
      hint: hintContent,
      timestamp: message.timestamp || new Date().toISOString(),
      problemContext: message.content?.problemContext
    };

    console.log(`💡 Hint from ${hint.expert.name}: ${hint.hint.substring(0, 50)}...`);
    
    this.hintsReceived.push(hint);
    
    // Determine expert ID from the hint
    const expertId = this.getExpertIdFromHint(hint);
    
    if (expertId) {
      this.displayHintInExpertPanel(expertId, hint);
      this.updateExpertMetrics(expertId, hint);
    } else {
      console.warn('⚠️ Could not map hint to expert panel:', hint.expert.name);
    }
  }

  handleExpertError(message) {
    console.log(`⚠️ Expert error from ${message.from_user}: ${message.content.error}`);
  }

  handleSystemMessage(message) {
    const event = message.content.event;
    
    switch (event) {
      case 'history_complete':
        console.log('📚 Message history loaded');
        this.handleHistoryLoaded();
        break;
        
      case 'message_error':
        console.error('❌ Message error:', message.content.error);
        break;
        
      case 'session_ended':
        console.log('🛑 Session ended by system');
        this.disconnect();
        this.currentSession = null;
        this.hideMainContent();
        this.updateConnectionStatus('disconnected', 'Session ended');
        this.resetExpertStatus(); // Reset expert status when session actually ends
        break;
    }
  }

  // UI Updates
  updateSessionDisplay(session) {
    document.getElementById('currentSessionId').textContent = session.id;
    document.getElementById('enrolledExperts').textContent = '5';
    document.getElementById('connectedExperts').textContent = this.getConnectedExpertCount();
    
    document.getElementById('currentSession').style.display = 'block';
    document.getElementById('endSessionBtn').disabled = false;
  }

  getConnectedExpertCount() {
    let count = 0;
    this.experts.forEach(expert => {
      if (expert.connected) count++;
    });
    return count;
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
    document.getElementById('mainContent').style.display = 'flex';
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
    
    if (this.experts.has(expertId)) {
      const expert = this.experts.get(expertId);
      
      if (event === 'connected' || event === 'session_joined') {
        expert.connected = true;
        expert.lastSeen = new Date();
      } else if (event === 'disconnected') {
        expert.connected = false;
      }
      
      this.updateExpertPanelStatus(expertId, expert.connected);
      this.updateConnectedExpertsCount();
    }
  }

  updateExpertPanelStatus(expertId, connected) {
    const statusElement = document.getElementById(`status-${expertId}`);
    const panelElement = document.getElementById(`panel-${expertId}`);
    
    if (statusElement) {
      if (connected) {
        statusElement.className = 'connection-status connected';
      } else {
        statusElement.className = 'connection-status disconnected';
      }
    }
    
    if (panelElement) {
      if (connected) {
        panelElement.classList.remove('disconnected');
      } else {
        panelElement.classList.add('disconnected');
      }
    }
  }

  updateConnectedExpertsCount() {
    const countElement = document.getElementById('connectedExperts');
    if (countElement) {
      countElement.textContent = this.getConnectedExpertCount();
    }
  }

  resetExpertStatus() {
    this.experts.forEach((expert, expertId) => {
      expert.connected = false;
      expert.lastSeen = null;
      this.updateExpertPanelStatus(expertId, false);
    });
    this.updateConnectedExpertsCount();
  }

  // Problem Broadcasting
  async broadcastProblem() {
    if (!this.connected) {
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
      
      // Send problem broadcast via WebSocket
      const message = {
        type: 'instructor_broadcast',
        context: 'problem',
        content: problemData,
        timestamp: new Date().toISOString()
      };

      this.ws.send(JSON.stringify(message));
      
      this.displayBroadcastConfirmation(problemData);
      this.clearHintsFromPanels(); // Clear previous hints for new problem
      
      console.log(`📢 Problem broadcasted to ${this.expertIds.length} experts`);
    } catch (error) {
      console.error('❌ Failed to broadcast problem:', error);
      alert('Failed to broadcast problem: ' + error.message);
    }
  }

  displayBroadcastConfirmation(problemData) {
    const statusElement = document.getElementById('broadcastStatus');
    statusElement.innerHTML = `
      <div class="broadcast-success">
        ✅ Problem broadcasted to ${this.getConnectedExpertCount()} connected experts
        <div class="broadcast-time">Sent at ${new Date().toLocaleTimeString()}</div>
      </div>
    `;
    
    // Clear after 5 seconds
    setTimeout(() => {
      statusElement.innerHTML = '';
    }, 5000);
  }



  getExpertIdFromHint(hint) {
    // Map expert names to IDs (both display names and user IDs)
    const nameToId = {
      // Display names
      'Technical Expert': 'technical_expert',
      'Emotional Support Coach': 'emotional_support_coach',
      'Debugging Guru': 'debugging_guru',
      'Learning Coach': 'learning_coach',
      'Architecture Expert': 'architecture_expert',
      // User IDs (fallback)
      'technical_expert': 'technical_expert',
      'emotional_support_coach': 'emotional_support_coach',
      'debugging_guru': 'debugging_guru',
      'learning_coach': 'learning_coach',
      'architecture_expert': 'architecture_expert'
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
    const expert = this.experts.get(expertId);
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
    const expert = this.experts.get(expertId);
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
    const expert = this.experts.get(expertId);
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
      const expert = this.experts.get(expertId);
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
    console.log('✅ Session history loaded');
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