# Switchboard Teacher SDK Usage Guide

## Overview

The Switchboard Teacher SDK provides a seamless interface for managing educational sessions with automatic connection management and smart session transitions.

## Key Features

### ✅ Fixed in Latest Version

- **Seamless Session Management**: Create sessions without disconnection when connected to lobby  
- **Smart Connection Logic**: Automatically handles existing connections to prevent reconnection issues
- **Session Lifecycle Events**: Proper handling of session transitions with UI update events
- **Auto-Assignment**: Server automatically assigns teachers to active sessions or lobby
- **Connection Preservation**: WebSocket connections remain active throughout session lifecycle

## Quick Start

### Basic Setup

```javascript
// Create teacher instance
const teacher = new SwitchboardTeacher('teacher_001');

// Set up event handlers
teacher.onSessionEvent = (data) => {
  if (data.event === 'session_started') {
    console.log(`Session started: ${data.sessionName}`);
    // Update UI to session mode
  } else if (data.event === 'session_left') {
    console.log('Returned to lobby');  
    // Update UI to lobby mode
  }
};

// Connect (auto-assigned to active session or lobby)
await teacher.connect();
```

### Creating Sessions

```javascript
// Method 1: Create and connect (recommended)
const session = await teacher.createAndConnect('Math Class', [
  'student_001', 'student_002', 'student_003'
]);

// Method 2: Create then connect separately (if needed)
const session = await teacher.createSession('Math Class', ['student_001']);
await teacher.connect(); // Will auto-assign to the session
```

### Seamless Session Management

```javascript
// Start in lobby
await teacher.connect(); // Connected to lobby

// Create session - preserves existing connection
const session1 = await teacher.createAndConnect('Session 1', ['student1']);
console.log('In session 1, no disconnection occurred');

// End session - returns to lobby, still connected  
await teacher.endCurrentSession();
console.log('Back in lobby, still connected');

// Create another session immediately
const session2 = await teacher.createAndConnect('Session 2', ['student2']);
console.log('In session 2, seamless transition');
```

## Message Types

### Broadcasting to Students

```javascript
// Simple announcement
await teacher.announce('Welcome to class!');

// Structured announcement with context
await teacher.announce({
  text: 'Time for a quiz!',
  duration: 600,
  instructions: 'Please close all other applications'
}, 'quiz');

// Emergency broadcast
await teacher.emergency('Please evacuate the building immediately');
```

### Direct Student Communication

```javascript
// Ask specific student for information
await teacher.ask('student_001', 'Please share your current code', 'code');

// Respond to student question
await teacher.respond('student_001', {
  text: 'Great question! Here\'s the answer...',
  code_example: 'const result = array.map(x => x * 2);'
}, 'answer');

// Request with structured content
await teacher.requestCode('student_001', 'Please share your solution for problem 3');
```

### Common Teaching Actions

```javascript
// Welcome students
await teacher.welcomeStudents('Good morning! Ready to learn React?');

// Schedule break
await teacher.scheduleBreak(10, 'Take a break, we\'ll resume at 2:15 PM');

// Give feedback
await teacher.giveFeedback('student_001', 
  'Excellent work on the component!', 
  '// Here\'s a small improvement:\nconst [count, setCount] = useState(0);'
);
```

## Event Handling

### Session Events

```javascript
teacher.onSessionEvent = (data) => {
  switch (data.event) {
    case 'session_started':
      // User joined or created a session
      updateUI('session', data.sessionName);
      enableTeachingControls();
      break;
      
    case 'session_left':  
      // User returned to lobby after session ended
      updateUI('lobby');
      disableTeachingControls();
      clearSessionData();
      break;
  }
};
```

### Student Messages

```javascript
// Handle student questions
teacher.onStudentQuestion = (data) => {
  displayQuestion(data.studentId, data.question, data.context);
  if (data.context === 'urgent') {
    showUrgentNotification(data);
  }
};

// Handle student responses  
teacher.onStudentResponse = (data) => {
  displayResponse(data.studentId, data.response);
  if (data.context === 'code') {
    highlightCodeSubmission(data);
  }
};

// Handle analytics
teacher.onStudentAnalytics = (data) => {
  updateDashboard(data.studentId, data.data);
  if (data.context === 'error') {
    flagStudentNeedsHelp(data.studentId);
  }
};
```

### Connection Events

```javascript
teacher.onConnection = (connected) => {
  if (connected) {
    console.log('Connected to Switchboard');
    showConnectionStatus('Connected');
  } else {
    console.log('Disconnected from Switchboard');
    showConnectionStatus('Disconnected');
  }
};

teacher.onError = (error) => {
  console.error('Switchboard error:', error);
  showErrorMessage(error.message);
};
```

## Session Management Best Practices

### Connection Management

```javascript
// ✅ Good: Let server handle assignment
await teacher.connect(); // Auto-assigned to session or lobby

// ✅ Good: Preserve connections when creating sessions
if (teacher.isInLobby()) {
  const session = await teacher.createAndConnect('New Class', students);
}

// ❌ Avoid: Manual disconnection/reconnection  
// This is handled automatically now
```

### Error Handling

```javascript
try {
  const session = await teacher.createAndConnect('Math Class', studentIds);
} catch (error) {
  if (error.message.includes('Active session exists')) {
    // Handle conflict - ask user to end existing session first
    const shouldEnd = confirm('End existing session to create new one?');
    if (shouldEnd) {
      await teacher.endCurrentSession();
      const session = await teacher.createAndConnect('Math Class', studentIds);
    }
  } else {
    console.error('Failed to create session:', error);
  }
}
```

### Session State Checking

```javascript
// Check current state
if (teacher.isInSession()) {
  console.log(`Currently in session: ${teacher.getSessionId()}`);
  enableSessionControls();
} else if (teacher.isInLobby()) {
  console.log('Currently in lobby');
  enableSessionCreation();
}

// React to state changes
teacher.onSessionEvent = (data) => {
  // Update UI based on session state
  const isInSession = teacher.isInSession();
  toggleSessionControls(isInSession);
};
```

## Complete Example

```javascript
class TeacherApp {
  constructor() {
    this.teacher = new SwitchboardTeacher('teacher_001');
    this.setupEventHandlers();
  }

  setupEventHandlers() {
    // Session lifecycle
    this.teacher.onSessionEvent = (data) => {
      if (data.event === 'session_started') {
        this.onSessionStarted(data);
      } else if (data.event === 'session_left') {
        this.onSessionEnded();
      }
    };

    // Student interactions
    this.teacher.onStudentQuestion = (data) => {
      this.displayStudentQuestion(data);
    };

    this.teacher.onStudentResponse = (data) => {
      this.displayStudentResponse(data);  
    };

    // Connection status
    this.teacher.onConnection = (connected) => {
      this.updateConnectionStatus(connected);
    };
  }

  async start() {
    try {
      // Connect to lobby initially
      await this.teacher.connect();
      console.log('Connected to Switchboard');
      
      // Check if we should auto-join existing session
      if (this.teacher.isInSession()) {
        console.log('Auto-joined existing session');
      } else {
        console.log('In lobby - ready to create session');
      }
      
    } catch (error) {
      console.error('Failed to connect:', error);
    }
  }

  async createNewClass() {
    try {
      const studentIds = this.getSelectedStudents();
      const className = document.getElementById('className').value;
      
      // Create session - preserves existing connection
      const session = await this.teacher.createAndConnect(className, studentIds);
      console.log(`Created session: ${session.name}`);
      
    } catch (error) {
      this.handleSessionError(error);
    }
  }

  async endCurrentClass() {
    try {
      await this.teacher.endCurrentSession();
      console.log('Session ended, returned to lobby');
    } catch (error) {
      console.error('Failed to end session:', error);
    }
  }

  onSessionStarted(data) {
    // Update UI for session mode
    document.getElementById('sessionName').textContent = data.sessionName;
    document.getElementById('sessionControls').style.display = 'block';
    document.getElementById('lobbyControls').style.display = 'none';
  }

  onSessionEnded() {
    // Update UI for lobby mode  
    document.getElementById('sessionControls').style.display = 'none';
    document.getElementById('lobbyControls').style.display = 'block';
  }

  // ... other methods
}

// Initialize app
const app = new TeacherApp();
app.start();
```

## Migration from Old SDK

If upgrading from a previous version:

### Changes Made

1. **`startSession()` → `createAndConnect()`**: Method renamed for clarity
2. **Smart Connection Logic**: No need to disconnect/reconnect when switching sessions  
3. **Enhanced Event Handling**: Better session transition events
4. **Auto-Assignment**: Server handles session assignment automatically

### Update Your Code

```javascript
// Old way (caused disconnections)
await teacher.disconnect();
const session = await teacher.startSession('Class', students);

// New way (seamless)  
const session = await teacher.createAndConnect('Class', students);
```

The new SDK provides a much smoother experience with no connection interruptions during session management operations.