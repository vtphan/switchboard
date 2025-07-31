/**
 * Switchboard Student Client Application
 * Demonstrates SDK integration for student users
 */

class StudentApp {
    constructor() {
        this.client = null;
        this.isConnected = false;
        this.questionsAsked = 0;
        this.messagesReceived = 0;
        
        // DOM elements
        this.elements = {
            studentId: document.getElementById('studentId'),
            connectBtn: document.getElementById('connectBtn'),
            disconnectBtn: document.getElementById('disconnectBtn'),
            statusDot: document.getElementById('statusDot'),
            statusText: document.getElementById('statusText'),
            sessionInfo: document.getElementById('sessionInfo'),
            questionText: document.getElementById('questionText'),
            codeSnippet: document.getElementById('codeSnippet'),
            askBtn: document.getElementById('askBtn'),
            messages: document.getElementById('messages'),
            questionsCount: document.getElementById('questionsCount'),
            messagesCount: document.getElementById('messagesCount')
        };
        
        this.setupEventListeners();
    }

    setupEventListeners() {
        this.elements.connectBtn.addEventListener('click', () => this.connect());
        this.elements.disconnectBtn.addEventListener('click', () => this.disconnect());
        this.elements.askBtn.addEventListener('click', () => this.askQuestion());
        
        // Allow Enter to submit question
        this.elements.questionText.addEventListener('keypress', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                this.askQuestion();
            }
        });
    }

    async connect() {
        const studentId = this.elements.studentId.value.trim();
        if (!studentId) {
            alert('Please enter a student ID');
            return;
        }

        try {
            // Initialize Switchboard client with comprehensive hooks
            this.client = new SwitchboardClient({
                userId: studentId,
                role: 'student',
                wsUrl: 'ws://localhost:8080/ws',
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
                        this.elements.askBtn.disabled = false;
                        this.addSystemMessage('✅ Connected to Switchboard');
                    },
                    
                    onDisconnected: (code, reason) => {
                        this.isConnected = false;
                        this.updateStatus('disconnected', 'Disconnected');
                        this.elements.connectBtn.disabled = false;
                        this.elements.disconnectBtn.disabled = true;
                        this.elements.askBtn.disabled = true;
                        this.addSystemMessage(`❌ Disconnected (${reason || 'Unknown reason'})`);
                    },
                    
                    onReconnecting: (attempt, delay) => {
                        this.updateStatus('connecting', `Reconnecting... (attempt ${attempt})`);
                        this.addSystemMessage(`🔄 Reconnecting in ${delay}ms (attempt ${attempt})`);
                    },
                    
                    onConnectionError: (error) => {
                        this.addErrorMessage(`Connection error: ${error.message || 'Unknown error'}`);
                    },

                    // Message events - key for students
                    onBroadcastToStudents: (message) => {
                        this.messagesReceived++;
                        this.updateStats();
                        
                        // Handle different types of announcements
                        if (message.content?.name === 'announcement') {
                            this.addAnnouncementMessage(message);
                        } else {
                            this.addInstructorMessage(message);
                        }
                    },
                    
                    onDirectMessage: (message) => {
                        this.messagesReceived++;
                        this.updateStats();
                        this.addDirectMessage(message);
                    },
                    
                    onMessage: (message) => {
                        // Log all messages for debugging
                        console.log('Received message:', message);
                    },

                    // Session events - important for students
                    onSessionStarted: (session) => {
                        this.elements.sessionInfo.textContent = `📚 Session: ${session.name}`;
                        this.addSystemMessage(`🎓 Session started: ${session.name}`);
                    },
                    
                    onSessionEnded: (session) => {
                        this.elements.sessionInfo.textContent = 'No active session';
                        this.addSystemMessage(`📚 Session ended: ${session.name}`);
                    },
                    
                    onWaitingForSession: () => {
                        this.elements.sessionInfo.textContent = 'Waiting for session to start...';
                        this.addSystemMessage('⏳ Waiting for instructor to start session');
                    },
                    
                    onSessionActive: (session) => {
                        this.elements.sessionInfo.textContent = `📚 Session: ${session.name}`;
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
                        this.addErrorMessage('⚠️ No active session. Wait for instructor to start class.');
                    },
                    
                    onMessageTooLarge: (error) => {
                        this.addErrorMessage('⚠️ Message is too large. Please shorten your message.');
                    }
                }
            });

            await this.client.connect();
            
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

    async askQuestion() {
        if (!this.client || !this.isConnected) {
            alert('Please connect first');
            return;
        }

        const questionText = this.elements.questionText.value.trim();
        const codeSnippet = this.elements.codeSnippet.value.trim();

        if (!questionText && !codeSnippet) {
            alert('Please enter a question or code snippet');
            return;
        }

        try {
            // Build question message using SDK's fluent API
            let messageBuilder = this.client.broadcast_to_instructors('helpRequest');
            
            if (questionText) {
                messageBuilder = messageBuilder.withText(questionText);
            }
            
            if (codeSnippet) {
                messageBuilder = messageBuilder.withCode(codeSnippet, 'javascript');
            }
            
            // Add helpful metadata
            messageBuilder = messageBuilder
                .withData({
                    studentId: this.elements.studentId.value,
                    timestamp: new Date().toISOString(),
                    hasCode: !!codeSnippet
                })
                .withTags('question', 'student-help');

            await messageBuilder.send();

            // Update UI
            this.questionsAsked++;
            this.updateStats();
            this.elements.questionText.value = '';
            this.elements.codeSnippet.value = '';
            
            this.addOwnMessage({
                type: 'question',
                content: { text: questionText, code_snippet: codeSnippet },
                timestamp: new Date().toISOString()
            });

        } catch (error) {
            this.addErrorMessage(`Failed to send question: ${error.message}`);
        }
    }

    // UI Update methods
    updateStatus(state, text) {
        this.elements.statusDot.className = `status-dot ${state}`;
        this.elements.statusText.textContent = text;
    }

    updateStats() {
        this.elements.questionsCount.textContent = this.questionsAsked;
        this.elements.messagesCount.textContent = this.messagesReceived;
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

    addAnnouncementMessage(message) {
        this.addMessage(message, 'announcement');
    }

    addInstructorMessage(message) {
        this.addMessage(message, 'other');
    }

    addDirectMessage(message) {
        this.addMessage(message, 'other');
    }

    addOwnMessage(message) {
        this.addMessage(message, 'own');
    }

    addMessage(message, messageClass = null) {
        const messageEl = document.createElement('div');
        
        // Determine message class
        if (!messageClass) {
            if (message.type === 'system') messageClass = 'system';
            else if (message.type === 'error') messageClass = 'error';
            else messageClass = 'other';
        }
        
        messageEl.className = `message ${messageClass}`;

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
            
            content = `
                <div class="message-header">
                    <span class="message-sender">${sender}</span>
                    <span class="message-time">${timeStr}</span>
                </div>
                <div class="message-body">
                    ${text}
                    ${code ? `<div class="code-block">${this.escapeHtml(code)}</div>` : ''}
                </div>
                <div class="message-meta">
                    <span class="message-type">${message.type}</span>
                    <span>${message.context || 'general'}</span>
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
    window.studentApp = new StudentApp();
    console.log('Student client loaded. Connect to start using Switchboard SDK.');
});