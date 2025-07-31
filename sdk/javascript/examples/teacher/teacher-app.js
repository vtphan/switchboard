/**
 * Switchboard Teacher Client Application
 * Demonstrates SDK integration for instructor users
 */

class TeacherApp {
    constructor() {
        this.client = null;
        this.isConnected = false;
        this.activeSession = null;
        this.questionsReceived = 0;
        this.responsesSent = 0;
        this.activeStudents = new Set();
        this.selectedStudent = null; // For quick response
        
        // DOM elements
        this.elements = {
            teacherId: document.getElementById('teacherId'),
            connectBtn: document.getElementById('connectBtn'),
            disconnectBtn: document.getElementById('disconnectBtn'),
            statusDot: document.getElementById('statusDot'),
            statusText: document.getElementById('statusText'),
            sessionInfo: document.getElementById('sessionInfo'),
            sessionDetails: document.getElementById('sessionDetails'),
            sessionName: document.getElementById('sessionName'),
            startSessionBtn: document.getElementById('startSessionBtn'),
            endSessionBtn: document.getElementById('endSessionBtn'),
            startSessionGroup: document.getElementById('startSessionGroup'),
            endSessionGroup: document.getElementById('endSessionGroup'),
            activeSessionName: document.getElementById('activeSessionName'),
            activeSessionId: document.getElementById('activeSessionId'),
            announcementText: document.getElementById('announcementText'),
            announceBtn: document.getElementById('announceBtn'),
            importantBtn: document.getElementById('importantBtn'),
            messages: document.getElementById('messages'),
            responseToStudent: document.getElementById('responseToStudent'),
            responseText: document.getElementById('responseText'),
            responseBtn: document.getElementById('responseBtn'),
            clearResponseBtn: document.getElementById('clearResponseBtn'),
            studentsCount: document.getElementById('studentsCount'),
            questionsCount: document.getElementById('questionsCount'),
            responsesCount: document.getElementById('responsesCount')
        };
        
        this.setupEventListeners();
    }

    setupEventListeners() {
        this.elements.connectBtn.addEventListener('click', () => this.connect());
        this.elements.disconnectBtn.addEventListener('click', () => this.disconnect());
        this.elements.startSessionBtn.addEventListener('click', () => this.startSession());
        this.elements.endSessionBtn.addEventListener('click', () => this.endSession());
        this.elements.announceBtn.addEventListener('click', () => this.sendAnnouncement());
        this.elements.importantBtn.addEventListener('click', () => this.sendAnnouncement(true));
        this.elements.responseBtn.addEventListener('click', () => this.sendResponse());
        this.elements.clearResponseBtn.addEventListener('click', () => this.clearResponse());
        
        // Allow Enter to submit announcement
        this.elements.announcementText.addEventListener('keypress', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                this.sendAnnouncement();
            }
        });
        
        // Allow Enter to submit response
        this.elements.responseText.addEventListener('keypress', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                this.sendResponse();
            }
        });
    }

    async connect() {
        const teacherId = this.elements.teacherId.value.trim();
        if (!teacherId) {
            alert('Please enter a teacher ID');
            return;
        }

        try {
            console.log('Creating SwitchboardClient with userId:', teacherId);
            // Initialize Switchboard client with instructor-focused hooks
            this.client = new SwitchboardClient({
                userId: teacherId,
                role: 'instructor',
                wsUrl: 'ws://localhost:8080/ws',
                debug: true, // Enable debug logging
                hooks: {
                    // Connection events
                    onConnecting: () => {
                        this.updateStatus('connecting', 'Connecting...');
                        this.elements.connectBtn.disabled = true;
                    },
                    
                    onConnected: () => {
                        this.isConnected = true;
                        this.updateStatus('connected', 'Connected');
                        this.elements.connectBtn.disabled = true;
                        this.elements.disconnectBtn.disabled = false;
                        this.elements.startSessionBtn.disabled = false;
                        this.elements.announceBtn.disabled = false;
                        this.elements.importantBtn.disabled = false;
                        this.addSystemMessage('✅ Connected to Switchboard');
                    },
                    
                    onDisconnected: (code, reason) => {
                        this.isConnected = false;
                        this.updateStatus('disconnected', 'Disconnected');
                        this.elements.connectBtn.disabled = false;
                        this.elements.disconnectBtn.disabled = true;
                        this.disableSessionControls();
                        this.addSystemMessage(`❌ Disconnected (${reason || 'Unknown reason'})`);
                    },
                    
                    onReconnecting: (attempt, delay) => {
                        this.updateStatus('connecting', `Reconnecting... (attempt ${attempt})`);
                        this.addSystemMessage(`🔄 Reconnecting in ${delay}ms (attempt ${attempt})`);
                    },
                    
                    onConnectionError: (error) => {
                        this.addErrorMessage(`Connection error: ${error.message || 'Unknown error'}`);
                    },

                    // Message events - critical for teachers
                    onBroadcastToInstructors: (message) => {
                        this.questionsReceived++;
                        this.activeStudents.add(message.from_user);
                        this.updateStats();
                        
                        // Handle different types of student messages
                        if (message.content?.name === 'helpRequest') {
                            this.addStudentQuestion(message);
                        } else if (message.content?.name === 'codeSnapshot') {
                            this.addStudentCodeUpdate(message);
                        } else {
                            this.addStudentMessage(message);
                        }
                    },
                    
                    onDirectMessage: (message) => {
                        this.addDirectMessage(message);
                    },
                    
                    onMessage: (message) => {
                        // Log all messages for debugging
                        console.log('Received message:', message);
                    },

                    // Session events - essential for instructors
                    onSessionStarted: (session) => {
                        console.log('onSessionStarted hook called with:', session);
                        this.activeSession = session;
                        this.updateSessionUI(session);
                        this.addSystemMessage(`🎓 Session started: ${session.name}`);
                    },
                    
                    onSessionEnded: (session) => {
                        this.activeSession = null;
                        this.updateSessionUI(null);
                        this.addSystemMessage(`📚 Session ended: ${session.name}`);
                    },
                    
                    onWaitingForSession: () => {
                        this.elements.sessionInfo.textContent = 'Ready to start session';
                    },
                    
                    onSessionActive: (session) => {
                        this.activeSession = session;
                        this.updateSessionUI(session);
                        this.addSystemMessage(`🎓 Joined active session: ${session.name}`);
                    },
                    
                    onHistoryDelivered: () => {
                        this.addSystemMessage('📋 Message history loaded');
                    },

                    // Error handling
                    onError: (error) => {
                        this.addErrorMessage(`Error: ${error.message || 'Unknown error'}`);
                    },
                    
                    onRateLimited: (error) => {
                        this.addErrorMessage('⚠️ Sending messages too quickly. Please slow down.');
                    },
                    
                    onNoActiveSession: (error) => {
                        this.addErrorMessage('⚠️ Please start a session before sending messages.');
                    },
                    
                    onMessageTooLarge: (error) => {
                        this.addErrorMessage('⚠️ Message is too large. Please shorten your message.');
                    }
                }
            });

            console.log('Attempting to connect...');
            await this.client.connect();
            console.log('Connection attempt completed');
            
        } catch (error) {
            this.addErrorMessage(`Failed to connect: ${error.message}`);
            this.elements.connectBtn.disabled = false;
        }
    }

    disconnect() {
        if (this.client) {
            this.client.disconnect();
            this.client = null;
        }
    }

    async startSession() {
        if (!this.client || !this.isConnected) {
            alert('Please connect first');
            return;
        }

        const sessionName = this.elements.sessionName.value.trim();
        if (!sessionName) {
            alert('Please enter a session name');
            return;
        }

        try {
            console.log('Attempting to start session:', sessionName);
            const result = await this.client.startSession(sessionName);
            console.log('Session start result:', result);
            
            // Manually update UI since WebSocket notification might not come
            if (result && result.id) {
                console.log('Manually updating session UI');
                this.activeSession = result;
                this.updateSessionUI(result);
                this.addSystemMessage(`🎓 Session started: ${result.name}`);
            }
        } catch (error) {
            if (error.message.includes('409') || error.message.includes('already active')) {
                this.addErrorMessage('⚠️ A session is already active. Please end the current session first or refresh the page.');
            } else {
                this.addErrorMessage(`Failed to start session: ${error.message}`);
            }
        }
    }

    async endSession() {
        if (!this.client || !this.activeSession) {
            return;
        }

        try {
            console.log('Attempting to end session');
            const result = await this.client.endSession();
            console.log('Session end result:', result);
            
            // Manually update UI since WebSocket notification might not come
            const endedSession = this.activeSession;
            this.activeSession = null;
            this.updateSessionUI(null);
            this.addSystemMessage(`📚 Session ended: ${endedSession.name}`);
        } catch (error) {
            this.addErrorMessage(`Failed to end session: ${error.message}`);
        }
    }

    async sendAnnouncement(important = false) {
        if (!this.client || !this.isConnected) {
            alert('Please connect first');
            return;
        }

        const text = this.elements.announcementText.value.trim();
        if (!text) {
            alert('Please enter an announcement');
            return;
        }

        try {
            let messageBuilder = this.client.broadcast_to_students('announcement')
                .withText(text)
                .withData({
                    teacherId: this.elements.teacherId.value,
                    timestamp: new Date().toISOString()
                });

            if (important) {
                messageBuilder = messageBuilder.markAsImportant();
            }

            await messageBuilder.send();

            // Update UI
            this.elements.announcementText.value = '';
            this.addOwnMessage({
                type: 'announcement',
                content: { text, important },
                timestamp: new Date().toISOString()
            });

        } catch (error) {
            this.addErrorMessage(`Failed to send announcement: ${error.message}`);
        }
    }

    async sendResponse() {
        if (!this.client || !this.isConnected || !this.selectedStudent) {
            alert('Please select a student to respond to');
            return;
        }

        const responseText = this.elements.responseText.value.trim();
        if (!responseText) {
            alert('Please enter a response');
            return;
        }

        try {
            await this.client.direct_message(this.selectedStudent, 'response')
                .withText(responseText)
                .withData({
                    teacherId: this.elements.teacherId.value,
                    timestamp: new Date().toISOString(),
                    responseType: 'direct'
                })
                .send();

            // Update UI
            this.responsesSent++;
            this.updateStats();
            this.clearResponse();
            
            this.addOwnMessage({
                type: 'response',
                content: { text: `Response to ${this.selectedStudent}: ${responseText}` },
                timestamp: new Date().toISOString()
            });

        } catch (error) {
            this.addErrorMessage(`Failed to send response: ${error.message}`);
        }
    }

    clearResponse() {
        this.selectedStudent = null;
        this.elements.responseToStudent.value = '';
        this.elements.responseText.value = '';
        this.elements.responseBtn.disabled = true;
    }

    selectStudentForResponse(studentId) {
        this.selectedStudent = studentId;
        this.elements.responseToStudent.value = studentId;
        this.elements.responseBtn.disabled = false;
        this.elements.responseText.focus();
    }

    updateSessionUI(session) {
        console.log('updateSessionUI called with:', session);
        console.log('DOM elements check:', {
            startSessionGroup: !!this.elements.startSessionGroup,
            endSessionGroup: !!this.elements.endSessionGroup,
            activeSessionName: !!this.elements.activeSessionName,
            activeSessionId: !!this.elements.activeSessionId
        });
        
        if (session) {
            // Session is active
            this.elements.sessionInfo.textContent = `📚 Active Session: ${session.name}`;
            this.elements.sessionDetails.textContent = `Session ID: ${session.id || 'Unknown'}`;
            this.elements.sessionDetails.style.display = 'block';
            
            // Show session details in the management area
            this.elements.activeSessionName.textContent = session.name;
            this.elements.activeSessionId.textContent = session.id || 'Unknown';
            
            // Switch to end session UI
            this.elements.startSessionGroup.style.display = 'none';
            this.elements.endSessionGroup.style.display = 'flex';
        } else {
            // No active session
            this.elements.sessionInfo.textContent = 'No active session';
            this.elements.sessionDetails.style.display = 'none';
            
            // Switch to start session UI
            this.elements.startSessionGroup.style.display = 'flex';
            this.elements.endSessionGroup.style.display = 'none';
            this.elements.startSessionBtn.disabled = !this.isConnected;
        }
    }

    disableSessionControls() {
        this.elements.startSessionBtn.disabled = true;
        this.elements.endSessionBtn.disabled = true;
        this.elements.announceBtn.disabled = true;
        this.elements.importantBtn.disabled = true;
        
        // Reset to start session UI when disconnected
        this.updateSessionUI(null);
    }

    // UI Update methods
    updateStatus(state, text) {
        this.elements.statusDot.className = `status-dot ${state}`;
        this.elements.statusText.textContent = text;
    }

    updateStats() {
        this.elements.studentsCount.textContent = this.activeStudents.size;
        this.elements.questionsCount.textContent = this.questionsReceived;
        this.elements.responsesCount.textContent = this.responsesSent;
    }

    // Message display methods
    addSystemMessage(text) {
        this.addMessage({
            type: 'system',
            content: { text },
            timestamp: new Date().toISOString()
        });
    }

    addErrorMessage(text) {
        this.addMessage({
            type: 'error',
            content: { text },
            timestamp: new Date().toISOString()
        });
    }

    addStudentQuestion(message) {
        this.addMessage(message, 'question', true);
    }

    addStudentCodeUpdate(message) {
        this.addMessage(message, 'other');
    }

    addStudentMessage(message) {
        this.addMessage(message, 'other');
    }

    addDirectMessage(message) {
        this.addMessage(message, 'other');
    }

    addOwnMessage(message) {
        this.addMessage(message, 'own');
    }

    addMessage(message, messageClass = null, isClickable = false) {
        const messageEl = document.createElement('div');
        
        // Determine message class
        if (!messageClass) {
            if (message.type === 'system') messageClass = 'system';
            else if (message.type === 'error') messageClass = 'error';
            else messageClass = 'other';
        }
        
        messageEl.className = `message ${messageClass}`;

        // Make question messages clickable for quick response
        if (isClickable && message.from_user) {
            messageEl.style.cursor = 'pointer';
            messageEl.title = 'Click to respond to this student';
            messageEl.addEventListener('click', () => {
                this.selectStudentForResponse(message.from_user);
            });
        }

        // Format timestamp
        const timestamp = new Date(message.timestamp);
        const timeStr = timestamp.toLocaleTimeString();

        // Build message content
        let content = '';
        
        if (message.type === 'system' || message.type === 'error') {
            content = `<div class="message-body">${message.content.text}</div>`;
        } else {
            const sender = message.from_user || 'You';
            const text = message.content?.text || '';
            const code = message.content?.code_snippet;
            const important = message.content?.important;
            
            content = `
                <div class="message-header">
                    <span class="message-sender">${sender}${important ? ' ⚠️' : ''}</span>
                    <span class="message-time">${timeStr}</span>
                </div>
                <div class="message-body">
                    ${text}
                    ${code ? `<div class="code-block">${this.escapeHtml(code)}</div>` : ''}
                </div>
                <div class="message-meta">
                    <span class="message-type">${message.type}</span>
                    <span>${message.context || 'general'}${isClickable ? ' • Click to respond' : ''}</span>
                </div>
            `;
        }

        messageEl.innerHTML = content;
        this.elements.messages.appendChild(messageEl);
        this.elements.messages.scrollTop = this.elements.messages.scrollHeight;
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Initialize the application when the page loads
window.addEventListener('DOMContentLoaded', () => {
    window.teacherApp = new TeacherApp();
    console.log('Teacher client loaded. Connect and start a session to begin.');
});