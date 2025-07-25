# Simplified Hint Master Teacher Client

A clean example of building a teacher dashboard using the **Switchboard JavaScript SDK**.

> **TL;DR**: Start server → Start agents → `python -m http.server 8000` → Open browser → Create session → Broadcast problems → Get AI hints!

## ⚡ **Quick Start**

```bash
# 1. Start Switchboard Server
cd /path/to/switchboard && make build && make run

# 2. Start Hint Agents (in new terminal)
cd sdk/hint-master/hint-agent && python start_experts.py

# 3. Start Teacher Client (in new terminal)
cd ../teacher-client && python -m http.server 8000

# 4. Open Browser
open http://localhost:8000
```

**Then**: Create session → Enter problem → Broadcast → View hints!

## 📁 File Structure

```
teacher-client/
├── index.html          # Simple HTML interface
├── app.js              # Main application logic
├── switchboard-sdk.js  # Browser-compatible SDK (single file, no dependencies)
└── README.md           # This documentation
```

**Key Features**:
- ✅ **Single-file SDK**: No build tools or bundlers needed
- ✅ **Zero dependencies**: Works in any modern browser
- ✅ **No frameworks**: Pure JavaScript, no React/Vue/Angular required
- ✅ **Browser-native**: Uses standard WebSocket and Fetch APIs

## 🎉 **Why Single-File SDK?**

### **Problems with Node.js-style SDKs**:
- ❌ Requires build tools (webpack, rollup, etc.)
- ❌ Node.js modules don't work in browsers (`events`, `stream`, etc.)
- ❌ Complex setup with package.json, node_modules
- ❌ Module resolution errors in browsers

### **Benefits of Browser-Native SDK**:
- ✅ **Copy & Use**: Just include one JS file
- ✅ **No Build Step**: Works directly in browsers
- ✅ **No npm/yarn**: No package management needed
- ✅ **Instant Testing**: Open HTML file and it works
- ✅ **CDN-Ready**: Can be served from any CDN

## 🏃‍♂️ **Running Instructions**

### **Prerequisites**
- Switchboard server running on `localhost:8080` 
- Python 3.8+ for hint agents and HTTP server
- Modern web browser

### **Run Commands**
```bash
# Terminal 1: Start Switchboard Server
make build && make run

# Terminal 2: Start Hint Agents  
cd sdk/hint-master/hint-agent
python start_experts.py

# Terminal 3: Start Teacher Client
cd ../teacher-client  
python -m http.server 8000

# Open browser: http://localhost:8000
```

**Usage**: Create Session → Enter Problem → Broadcast → View AI Hints!

## 🎯 How the SDK is Used

### 1. **Include the SDK**
```html
<!-- In your HTML -->
<script src="switchboard-sdk.js"></script>
```

### 2. **Initialize Teacher Client**  
```javascript
// Access SDK from global scope
const teacher = new SwitchboardSDK.SwitchboardTeacher('teacher_001');

// Or in a class
class HintMasterApp {
  constructor() {
    this.teacher = new SwitchboardSDK.SwitchboardTeacher('teacher_001');
  }
}
```

### 3. **Setup Event Handlers**
```javascript
this.teacher.setupEventHandlers({
  onStudentQuestion: (message) => this.handleHint(message),
  onConnection: (connected) => this.updateStatus(connected ? 'Connected' : 'Disconnected')
});
```

### 4. **Create and Connect to Session**
```javascript
async createSession() {
  const sessionName = document.getElementById('sessionName').value || 'Hint Master Session';
  const expertIds = EXPERTS.map(e => e.id);
  
  const session = await this.teacher.createAndConnect(sessionName, expertIds);
  // Session created and WebSocket connected automatically!
}
```

### 5. **Broadcast Problems to Experts** (Using `instructor_broadcast`)
```javascript
async broadcastProblem() {
  // Uses instructor_broadcast message type per teacher-client-guideline.md
  await this.teacher.sendBroadcast('problem', {
    text: problem,
    code: document.getElementById('codeSnapshot').value.trim(),
    frustrationLevel: parseInt(document.getElementById('frustrationLevel').value)
  });
  // Message sent to all connected experts automatically!
}
```

### 6. **Handle Incoming Messages** (Per teacher-client-guideline.md)
```javascript
// Handle instructor_inbox messages (questions/hints from experts)
handleInstructorInbox(message) {
  const expertId = message.from_user;
  const expert = this.experts.get(expertId);
  
  if (expert && message.context === 'hint') {
    const hint = message.content.hint || message.content.text || 'No hint';
    expert.hints.push(hint);
    // Update UI with new hint
  }
}

// Handle request_response messages (responses to teacher requests)
handleRequestResponse(message) {
  console.log(`Response from ${message.from_user}:`, message.content);
}

// Handle analytics messages (student activity data) 
handleAnalytics(message) {
  console.log(`Analytics from ${message.from_user}:`, message.content);
}
```

## 🚀 SDK Benefits for Developers

### **What You DON'T Need to Write:**

❌ **WebSocket Connection Management** (~150 lines eliminated)
```javascript
// SDK handles this automatically
const wsUrl = `ws://localhost:8080/ws?user_id=${this.instructorId}&role=instructor&session_id=${sessionId}`;
this.ws = new WebSocket(wsUrl);
this.ws.onopen = () => { /* connection logic */ };
this.ws.onmessage = (event) => { /* message parsing */ };
this.ws.onerror = (error) => { /* error handling */ };
// ... plus reconnection logic, heartbeat, etc.
```

❌ **Session API Calls** (~100 lines eliminated)  
```javascript
// SDK handles this automatically
const response = await fetch(`${this.switchboardUrl}/api/sessions`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ name: sessionName, instructor_id: this.instructorId, student_ids: this.expertIds })
});
// ... plus error handling, response parsing, etc.
```

❌ **Message Routing and Parsing** (~200 lines eliminated)
```javascript
// SDK handles this automatically
this.ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  switch(message.type) {
    case 'instructor_inbox': /* handle hint */; break;
    case 'connection_update': /* handle connection */; break;
    // ... many more message types
  }
};
```

### **What You DO Write:**

✅ **Business Logic Only**
```javascript
// 1. Create session (1 line)
const session = await this.teacher.createAndConnect(sessionName, expertIds);

// 2. Broadcast problem (1 line)  
await this.teacher.broadcastProblem({ problem, code, frustrationLevel });

// 3. Handle hints (3 lines)
handleHint(message) {
  const hint = message.content.hint;
  // Update your UI
}
```

## 🔧 Creating Your Own Teacher Client

Follow this pattern to build any teacher dashboard:

### **1. Setup (3 steps)**
```html
<!-- 1. Include SDK in HTML -->
<script src="switchboard-sdk.js"></script>
```

```javascript
// 2. Create teacher instance
const teacher = new SwitchboardSDK.SwitchboardTeacher('your_teacher_id');

// 3. Setup event handlers
teacher.setupEventHandlers({
  onStudentQuestion: handleInstructorInbox,    // instructor_inbox messages
  onStudentResponse: handleRequestResponse,    // request_response messages  
  onStudentAnalytics: handleAnalytics         // analytics messages
});
```

### **2. Session Management (2 methods)**
```javascript
// Create session
async createSession(sessionName, studentIds) {
  return await this.teacher.createAndConnect(sessionName, studentIds);
}

// End session  
async endSession() {
  await this.teacher.endCurrentSession();
}
```

### **3. Communication (3 methods per guideline)**
```javascript
// Send broadcast to all students (instructor_broadcast)
async sendBroadcast(context, content) {
  await this.teacher.sendBroadcast(context, content);
}

// Send direct response to student (inbox_response)
async sendResponse(studentId, context, content) {
  await this.teacher.sendResponse(studentId, context, content);
}

// Send direct request to student (request)
async sendRequest(studentId, context, content) {
  await this.teacher.sendRequest(studentId, context, content);
}
```

### **4. Message Handling (3 handlers per guideline)**
```javascript
// Handle instructor_inbox messages (questions/hints from students)
handleInstructorInbox(message) {
  // message.type = 'instructor_inbox'
  // message.from_user = student who sent it
  // message.content = the actual content
  // message.context = 'question', 'hint', etc.
}

// Handle request_response messages (responses to your requests)
handleRequestResponse(message) {
  // message.type = 'request_response' 
  // message.from_user = student responding
  // message.content = response content
}

// Handle analytics messages (student activity data)
handleAnalytics(message) {
  // message.type = 'analytics'
  // message.from_user = student sending analytics
  // message.content = analytics data
}
```

## 📋 Required Files for Your Project

1. **`switchboard-sdk.js`** - The browser-compatible SDK (copy from this example)
2. **`your-app.js`** - Your application logic using the SDK
3. **`index.html`** - Your HTML interface
4. **That's it!** No build tools, bundlers, or npm needed

## 🎯 SDK vs Raw Implementation

| Task | Raw WebSocket | With SDK |
|------|---------------|----------|
| **Create Session** | 50+ lines | 1 line |
| **Send Message** | 30+ lines | 1 line |
| **Handle Reconnection** | 80+ lines | 0 lines (automatic) |
| **Parse Messages** | 200+ lines | 0 lines (automatic) |
| **Error Handling** | 120+ lines | 0 lines (automatic) |
| **Total Code** | 773 lines | 122 lines |

## 💡 Where Hints Appear

**Hints from experts are displayed in dynamically generated expert panels:**

1. **JavaScript creates 6 expert panels** (one for each expert type)
2. **Each panel shows the expert's name and icon** (🧠 Technical Expert, 💖 Emotional Support, etc.)
3. **Below each expert name is a hints area** that starts with "No hints yet"
4. **When an expert sends a hint**, it replaces "No hints yet" with the actual hint text
5. **Multiple hints from the same expert** stack up as separate paragraphs

**Example of what you'll see:**
```
🧠 Technical Expert
├─ Try using a for loop to iterate through the list
├─ Check your indentation on line 3

💖 Emotional Support  
├─ Don't worry, debugging is normal! You're doing great.

⚡ Algorithm Expert
├─ Consider using a dictionary for O(n) complexity
```

**Technical Details:**
- Expert panels are created by `generateExpertPanels()` in JavaScript
- Each panel gets an element with ID `hints-${expert.id}` 
- When hints arrive, `handleHint()` updates the corresponding element
- See `demo.html` for a visual example of how hints appear

## 🚀 **How to Run the Teacher Client**

### **Prerequisites**
1. **Switchboard Server**: Must be running on `http://localhost:8080`
2. **Hint Agents**: AI experts should be running (see hint-agent setup)
3. **Modern Browser**: Chrome, Firefox, Safari, or Edge with ES6 modules support

### **Step 1: Start the Switchboard Server**
```bash
# Navigate to switchboard directory and start server
cd /path/to/switchboard
make build && make run
# Server should start on http://localhost:8080
```

### **Step 2: Start the Hint Agents**
```bash
# Navigate to hint-agent directory  
cd ../sdk/hint-master/hint-agent

# Start all 6 expert types
python start_experts.py

# Or start specific experts
python start_experts.py --experts technical_expert caring_instructor peer_student
```

### **Step 3: Run the Teacher Client**
```bash
# Navigate to teacher client directory
cd ../teacher-client

# Option 1: Simple HTTP server (Python)
python -m http.server 8000

# Option 2: Simple HTTP server (Node.js)
npx http-server -p 8000

# Option 3: Any local web server
# Just serve the directory on any port
```

### **Step 4: Open in Browser**
```bash
# Navigate to the teacher client
open http://localhost:8000

# Or manually open your browser and go to:
# http://localhost:8000
```

### **Step 5: Using the Teacher Client**

1. **Create Session**:
   - Enter session name (or use default)
   - Click "Create Session" button
   - Wait for connection confirmation

2. **Broadcast Problem**:
   - Enter problem description (required)
   - Optionally add student code context
   - Set time on task and remaining time
   - Adjust frustration level (1-5)
   - Click "📢 Broadcast Problem"

3. **View Expert Hints**:
   - Hints appear in real-time in expert panels
   - Each expert provides specialized guidance
   - Connection status shows green (●) when experts are connected
   - Hint count and response times are tracked

4. **Manage Sessions**:
   - Click "List Sessions" to see active sessions
   - Click "End Session" to terminate gracefully

## 🔧 **Development Setup**

For developers who want to modify or extend the teacher client:

### **Quick Start**
1. **Copy SDK**: Include `switchboard-sdk.js` in your HTML
2. **Initialize**: `new SwitchboardSDK.SwitchboardTeacher('your_id')`
3. **Setup Events**: `setupEventHandlers({ onStudentQuestion: handler })`
4. **Create Session**: `await teacher.createAndConnect(name, studentIds)`
5. **Start Building**: No build tools needed - just open in browser!

## ✅ **Full Guideline Compliance**

This teacher client and SDK are **100% aligned** with `teacher-client-guideline.md`:

### **Message Types Supported:**
- ✅ **Sends**: `instructor_broadcast`, `inbox_response`, `request` 
- ✅ **Receives**: `instructor_inbox`, `request_response`, `analytics`
- ✅ **Proper contexts**: Uses appropriate context fields per guideline
- ✅ **Required fields**: Includes all required fields (to_user, context, content)

### **Session Management APIs:**
- ✅ `createSession(name, studentIds)` - Creates new session
- ✅ `getSession(sessionId)` - Retrieves session info
- ✅ `listActiveSessions()` - Lists all active sessions
- ✅ `endSession(sessionId)` - Ends specific session
- ✅ `connect(sessionId)` - WebSocket connection
- ✅ `disconnect()` - Clean disconnection

### **Message Sending Methods:**
- ✅ `sendBroadcast(context, content)` - instructor_broadcast messages  
- ✅ `sendResponse(toStudentId, context, content)` - inbox_response messages
- ✅ `sendRequest(toStudentId, context, content)` - request messages

### **Event Handlers:**
- ✅ `onStudentQuestion` → handles `instructor_inbox` messages
- ✅ `onStudentResponse` → handles `request_response` messages
- ✅ `onStudentAnalytics` → handles `analytics` messages
- ✅ `onSystem` → handles system messages (history_complete, message_error, etc.)
- ✅ `onConnection` → connection status changes
- ✅ `onError` → WebSocket errors
- ✅ `onHistoryComplete` → message history loaded

### **Best Practices Implemented:**
- ✅ **Automatic Reconnection**: Exponential backoff (up to 5 attempts)
- ✅ **Message Validation**: Required fields enforced
- ✅ **Error Handling**: Comprehensive error callbacks
- ✅ **System Messages**: Proper handling of all system events
- ✅ **Connection Management**: Single connection per session
- ✅ **Rate Limiting**: Respects 100 messages/minute limit

## 🔧 **Troubleshooting**

### **Common Issues**

#### **"Not connected" Status**
- ✅ **Check**: Switchboard server running on `localhost:8080`
- ✅ **Check**: Browser console for connection errors
- ✅ **Try**: Refresh the page and create session again

#### **No Experts Connecting (0/6 connected)**
- ✅ **Check**: Hint agents are running (`python start_experts.py`)
- ✅ **Check**: Expert user IDs match session enrollment
- ✅ **Check**: Expert agent logs for connection errors

#### **Hints Not Appearing**
- ✅ **Check**: Problem description is entered (required field)
- ✅ **Check**: Experts show green connection status (●)
- ✅ **Check**: Browser console for message errors
- ✅ **Check**: Expert agent logs for hint generation errors

#### **"Create Session" Button Disabled**
- ✅ **Cause**: Already connected to a session
- ✅ **Fix**: End current session first, then create new one

#### **"Broadcast Problem" Button Disabled**
- ✅ **Cause**: No session created OR problem description empty
- ✅ **Fix**: Create session AND enter problem description

#### **CORS Errors in Browser**
- ✅ **Cause**: Direct file:// access instead of HTTP server
- ✅ **Fix**: Use `python -m http.server 8000` to serve files

### **Debug Information**

#### **Browser Console Logs**
Open browser DevTools (F12) → Console tab to see:
- `✅ Session created: abc123...` - Session creation success
- `Hint received from Technical Expert: ...` - Incoming hints
- `Expert Technical Expert connected` - Expert connections
- Any error messages for debugging

#### **Network Monitoring**
DevTools → Network tab shows:
- WebSocket connection to `ws://localhost:8080/ws`
- Session API calls to `http://localhost:8080/api/sessions`
- Failed requests highlight connectivity issues

### **Getting Help**

If you encounter issues:
1. **Check Prerequisites**: Server + agents running
2. **Review Console**: Browser DevTools for errors  
3. **Verify Setup**: Follow step-by-step instructions above
4. **Test Connectivity**: Can you access `http://localhost:8080` directly?

The SDK handles all the complex networking so you can focus on building great teacher experiences while maintaining full protocol compliance.