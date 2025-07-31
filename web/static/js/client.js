let ws = null;
let userId = '';
let role = '';

function connect() {
    userId = document.getElementById('userId').value;
    role = document.getElementById('role').value;
    
    const wsUrl = `ws://localhost:8080/ws?user_id=${userId}&role=${role}`;
    ws = new WebSocket(wsUrl);
    
    ws.onopen = function() {
        document.getElementById('status').textContent = 'Connected';
        addMessage('system', 'Connected to Switchboard');
    };
    
    ws.onmessage = function(event) {
        const message = JSON.parse(event.data);
        addMessage('received', JSON.stringify(message, null, 2));
    };
    
    ws.onclose = function() {
        document.getElementById('status').textContent = 'Disconnected';
        addMessage('system', 'Disconnected from Switchboard');
    };
    
    ws.onerror = function(error) {
        addMessage('error', 'WebSocket error: ' + error);
    };
}

function disconnect() {
    if (ws) {
        ws.close();
        ws = null;
    }
}

function startSession() {
    const sessionName = document.getElementById('sessionName').value;
    fetch('/api/session/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            name: sessionName,
            instructor_id: userId
        })
    })
    .then(response => response.json())
    .then(data => addMessage('api', 'Session started: ' + JSON.stringify(data)))
    .catch(error => addMessage('error', 'Session start error: ' + error));
}

function endSession() {
    fetch('/api/session/end', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            instructor_id: userId
        })
    })
    .then(response => response.json())
    .then(data => addMessage('api', 'Session ended: ' + JSON.stringify(data)))
    .catch(error => addMessage('error', 'Session end error: ' + error));
}

function sendMessage() {
    if (!ws) {
        addMessage('error', 'Not connected');
        return;
    }
    
    const messageType = document.getElementById('messageType').value;
    const toUser = document.getElementById('toUser').value;
    const content = document.getElementById('messageContent').value;
    
    const message = {
        type: messageType,
        context: 'general',
        content: { text: content }
    };
    
    if (messageType === 'direct_message' && toUser) {
        message.to_user = toUser;
    }
    
    ws.send(JSON.stringify(message));
    addMessage('sent', JSON.stringify(message, null, 2));
}

function addMessage(type, content) {
    const messages = document.getElementById('messages');
    const messageDiv = document.createElement('div');
    messageDiv.className = 'message';
    messageDiv.innerHTML = `<strong>[${type}]</strong> ${content}`;
    messages.appendChild(messageDiv);
    messages.scrollTop = messages.scrollHeight;
}

function clearMessages() {
    document.getElementById('messages').innerHTML = '';
}