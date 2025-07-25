# Switchboard JavaScript/TypeScript SDK

A comprehensive JavaScript/TypeScript client library for Switchboard real-time educational messaging system. Works in both Node.js and browser environments with full TypeScript support and React hooks.

## Installation

```bash
npm install @switchboard/sdk
```

For Node.js usage, you'll also need WebSocket support:

```bash
npm install ws node-fetch
```

## Quick Start

### Student Client (Node.js)

```javascript
import { SwitchboardStudent } from '@switchboard/sdk';

const student = new SwitchboardStudent('student_001');

// Set up event handlers
student.setupEventHandlers({
  onInstructorResponse: (message) => {
    console.log(`Response from ${message.from_user}: ${message.content.text}`);
  },
  
  onInstructorRequest: async (message) => {
    console.log(`Request from ${message.from_user}: ${message.content.text}`);
    
    // Respond to the request
    await student.respondToRequest({
      text: "Here is my response",
      code: "const result = 42;"
    });
  },
  
  onInstructorBroadcast: (message) => {
    console.log(`Announcement: ${message.content.text}`);
  },
  
  onConnection: (connected) => {
    console.log(`Connection status: ${connected ? 'Connected' : 'Disconnected'}`);
  },
  
  onError: (error) => {
    console.error('Error:', error.message);
  }
});

// Connect to available session
const session = await student.connectToAvailableSession();
if (session) {
  console.log(`Connected to: ${session.name}`);
  
  // Ask a question
  await student.askQuestion({
    text: "I need help with React hooks",
    code_context: "const [state, setState] = useState()",
    urgency: "medium"
  });
  
  // Report progress
  await student.reportProgress({
    completionPercentage: 75,
    timeSpentMinutes: 30,
    currentTopic: "React Hooks"
  });
}
```

### Teacher Client (Node.js)

```javascript
import { SwitchboardTeacher } from '@switchboard/sdk';

const teacher = new SwitchboardTeacher('teacher_001');

// Set up event handlers
teacher.setupEventHandlers({
  onStudentQuestion: async (message) => {
    console.log(`Question from ${message.from_user}: ${message.content.text}`);
    
    // Respond to the student
    await teacher.respondToStudent(message.from_user, {
      text: "Great question! Here's the answer...",
      code_example: "useEffect(() => { /* ... */ }, []);"
    });
  },
  
  onStudentAnalytics: (message) => {
    console.log(`Analytics from ${message.from_user}:`, message.content);
  }
});

// Create and connect to a session
const session = await teacher.createAndConnect('Python Workshop', [
  'student_001', 'student_002', 'student_003'
]);

console.log(`Created session: ${session.name}`);

// Make an announcement
await teacher.announce("Welcome to the Python Workshop!");

// Request code from a specific student
await teacher.requestCodeFromStudent({
  studentId: 'student_001',
  prompt: "Please share your current function implementation",
  requirements: ["Include comments", "Handle edge cases"]
});

// Broadcast a problem for AI tutoring systems
await teacher.broadcastProblem({
  problem: "Students are struggling with list comprehensions",
  code: "[x for x in range(10) if x % 2 == 0]",
  frustrationLevel: 3
});
```

### React Student Component

```jsx
import React from 'react';
import { useSwitchboardStudent } from '@switchboard/sdk';

function StudentApp() {
  const {
    client,
    connected,
    session,
    messages,
    connect,
    disconnect,
    sendMessage,
    error
  } = useSwitchboardStudent('student_001', {
    serverUrl: 'http://localhost:8080',
    autoConnect: true,
    sessionId: 'your-session-id'
  });

  const handleAskQuestion = async () => {
    if (client) {
      await client.askQuestion({
        text: "I need help with this problem",
        urgency: "medium"
      });
    }
  };

  const handleReportProgress = async () => {
    if (client) {
      await client.reportProgress({
        completionPercentage: 80,
        timeSpentMinutes: 45,
        currentTopic: "JavaScript Promises"
      });
    }
  };

  if (error) {
    return <div>Error: {error.message}</div>;
  }

  return (
    <div>
      <h1>Student Dashboard</h1>
      
      <div>
        Status: {connected ? 'Connected' : 'Disconnected'}
        {session && <span> - Session: {session.name}</span>}
      </div>
      
      <div>
        <button onClick={handleAskQuestion} disabled={!connected}>
          Ask Question
        </button>
        <button onClick={handleReportProgress} disabled={!connected}>
          Report Progress
        </button>
      </div>
      
      <div>
        <h3>Messages ({messages.length})</h3>
        {messages.map((message, index) => (
          <div key={index}>
            <strong>{message.type}</strong> from {message.from_user}: 
            {message.content.text}
          </div>
        ))}
      </div>
    </div>
  );
}

export default StudentApp;
```

### React Teacher Component

```jsx
import React, { useState } from 'react';
import { useSwitchboardTeacher } from '@switchboard/sdk';

function TeacherApp() {
  const {
    client,
    connected,
    session,
    messages,
    connect,
    disconnect,
    createSession,
    endSession,
    error
  } = useSwitchboardTeacher('teacher_001');

  const [sessionName, setSessionName] = useState('');
  const [studentIds, setStudentIds] = useState('student_001,student_002');

  const handleCreateSession = async () => {
    try {
      const newSession = await createSession(sessionName, studentIds.split(','));
      await connect(newSession.id);
    } catch (err) {
      console.error('Failed to create session:', err);
    }
  };

  const handleAnnouncement = async () => {
    if (client) {
      await client.announce("Welcome to today's lesson!");
    }
  };

  const handleBroadcastProblem = async () => {
    if (client) {
      await client.broadcastProblem({
        problem: "Debug this code snippet",
        code: "function broken() { return undefined.property; }",
        frustrationLevel: 2
      });
    }
  };

  return (
    <div>
      <h1>Teacher Dashboard</h1>
      
      {!session ? (
        <div>
          <h3>Create Session</h3>
          <input 
            value={sessionName}
            onChange={(e) => setSessionName(e.target.value)}
            placeholder="Session name"
          />
          <input 
            value={studentIds}
            onChange={(e) => setStudentIds(e.target.value)}
            placeholder="Student IDs (comma-separated)"
          />
          <button onClick={handleCreateSession}>Create & Connect</button>
        </div>
      ) : (
        <div>
          <div>
            Connected to: {session.name} 
            ({session.student_ids.length} students)
          </div>
          
          <div>
            <button onClick={handleAnnouncement}>
              Make Announcement
            </button>
            <button onClick={handleBroadcastProblem}>
              Broadcast Problem
            </button>
            <button onClick={() => endSession(session.id)}>
              End Session
            </button>
          </div>
        </div>
      )}
      
      <div>
        <h3>Messages ({messages.length})</h3>
        {messages.map((message, index) => (
          <div key={index}>
            <strong>{message.type}</strong> from {message.from_user}: 
            {JSON.stringify(message.content)}
          </div>
        ))}
      </div>
      
      {error && <div>Error: {error.message}</div>}
    </div>
  );
}

export default TeacherApp;
```

### AI Expert Bot (Node.js)

```javascript
import { SwitchboardStudent } from '@switchboard/sdk';

class AIExpertBot extends SwitchboardStudent {
  constructor(config) {
    super(config.userId);
    this.expertName = config.name;
    this.expertise = config.expertise;
    
    this.setupEventHandlers({
      onInstructorBroadcast: this.handleProblemBroadcast.bind(this)
    });
  }
  
  async handleProblemBroadcast(message) {
    if (message.context === 'problem') {
      console.log(`Processing problem: ${message.content.problem}`);
      
      // Generate hint using your AI service
      const hint = await this.generateHint(message.content);
      
      // Send hint back to instructors
      await this.askQuestion({
        hint: hint,
        expert: {
          name: this.expertName,
          expertise: this.expertise
        },
        problem_context: message.content
      }, 'hint');
    }
  }
  
  async generateHint(problemData) {
    // Integrate with OpenAI, Anthropic, Google AI, etc.
    return `AI-generated hint for: ${problemData.problem}`;
  }
}

// Usage
const bot = new AIExpertBot({
  userId: 'technical_expert',
  name: 'Technical Expert',
  expertise: 'JavaScript, React, Node.js'
});

const session = await bot.connectToAvailableSession();
if (session) {
  console.log(`Expert bot connected to ${session.name}`);
}
```

## Browser Usage

### Via CDN

```html
<script src="https://unpkg.com/@switchboard/sdk/dist/index.umd.js"></script>
<script>
  const { SwitchboardStudent, MessageType } = SwitchboardSDK;
  
  const student = new SwitchboardStudent('student_001');
  
  student.onMessage(MessageType.INSTRUCTOR_BROADCAST, (message) => {
    console.log('Broadcast:', message.content.text);
  });
  
  // Connect and use the client
  student.connectToAvailableSession().then(session => {
    if (session) {
      console.log('Connected to:', session.name);
    }
  });
</script>
```

### ES Modules

```html
<script type="module">
  import { SwitchboardStudent } from 'https://unpkg.com/@switchboard/sdk/dist/index.esm.js';
  
  const student = new SwitchboardStudent('student_001');
  // ... use the client
</script>
```

## API Reference

### SwitchboardStudent

#### Methods

- `findAvailableSessions()` → `Promise<Session[]>`
- `connectToAvailableSession()` → `Promise<Session | null>`
- `askQuestion(content, context?)` → `Promise<void>`
- `respondToRequest(content, context?)` → `Promise<void>`
- `sendAnalytics(content, context?)` → `Promise<void>`
- `reportProgress(options)` → `Promise<void>`
- `reportEngagement(options)` → `Promise<void>`
- `reportError(options)` → `Promise<void>`
- `requestHelp(options)` → `Promise<void>`

#### Event Handlers

- `onInstructorResponse(handler)`
- `onInstructorRequest(handler)`
- `onInstructorBroadcast(handler)`
- `onSystemMessage(handler)`
- `setupEventHandlers(handlers)` - Convenient way to set up multiple handlers

### SwitchboardTeacher

#### Methods

- `createSession(name, studentIds)` → `Promise<Session>`
- `endSession(sessionId)` → `Promise<void>`
- `listActiveSessions()` → `Promise<Session[]>`
- `respondToStudent(studentId, content, context?)` → `Promise<void>`
- `requestFromStudent(studentId, content, context?)` → `Promise<void>`
- `broadcastToStudents(content, context?)` → `Promise<void>`
- `announce(text, additionalContent?)` → `Promise<void>`
- `requestCodeFromStudent(options)` → `Promise<void>`
- `provideFeedback(options)` → `Promise<void>`
- `broadcastProblem(options)` → `Promise<void>`
- `createAndConnect(sessionName, studentIds)` → `Promise<Session>`

#### Event Handlers

- `onStudentQuestion(handler)`
- `onStudentResponse(handler)`
- `onStudentAnalytics(handler)`
- `onSystemMessage(handler)`
- `setupEventHandlers(handlers)` - Convenient way to set up multiple handlers

### React Hooks

#### useSwitchboardStudent(userId, options?)

Returns an object with:
- `client` - SwitchboardStudent instance
- `connected` - Connection status
- `session` - Current session object
- `messages` - Array of received messages
- `connect(sessionId)` - Connect function
- `disconnect()` - Disconnect function
- `sendMessage(message)` - Send message function
- `error` - Last error that occurred

#### useSwitchboardTeacher(userId, options?)

Similar to student hook, plus:
- `createSession(name, studentIds)` - Create session function
- `endSession(sessionId)` - End session function
- `listActiveSessions()` - List sessions function

#### Helper Hooks

- `useMessagesByType(messages, messageType)` - Filter messages by type
- `useLatestMessage(messages, messageType)` - Get latest message of type
- `useConnectionStatus(client)` - Get connection status with live updates

## Configuration

```javascript
const client = new SwitchboardStudent('student_001', {
  serverUrl: 'https://my-switchboard.com',
  maxReconnectAttempts: 10,
  reconnectDelay: 2000,
  heartbeatInterval: 30000
});
```

## Error Handling

The SDK provides comprehensive error handling:

```javascript
import { 
  SwitchboardError,
  ConnectionError,
  AuthenticationError,
  SessionNotFoundError,
  MessageValidationError,
  RateLimitError,
  SessionEndedError,
  ReconnectionFailedError
} from '@switchboard/sdk';

try {
  await student.connect(sessionId);
} catch (error) {
  if (error instanceof AuthenticationError) {
    console.log('Not enrolled in this session');
  } else if (error instanceof ConnectionError) {
    console.log('Connection failed:', error.message);
  }
}
```

## TypeScript Support

The SDK is written in TypeScript and provides full type definitions:

```typescript
import { 
  SwitchboardStudent, 
  MessageType, 
  IncomingMessage,
  Session 
} from '@switchboard/sdk';

const student = new SwitchboardStudent('student_001');

student.onMessage(MessageType.INSTRUCTOR_RESPONSE, (message: IncomingMessage) => {
  // message is fully typed
  console.log(message.content.text);
});
```

## Requirements

- **Node.js**: 14.0.0 or higher
- **Browser**: Modern browsers with WebSocket support
- **Dependencies**: 
  - `ws` (Node.js WebSocket support)
  - `node-fetch` (Node.js HTTP requests)
  - `react` (optional, for React hooks)

## Development

```bash
# Install dependencies
npm install

# Build the SDK
npm run build

# Run tests
npm test

# Lint code
npm run lint

# Type check
npm run type-check
```

## License

MIT License - see LICENSE file for details.