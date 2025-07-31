# Switchboard JavaScript Client SDK

A comprehensive, single-file JavaScript library for connecting to Switchboard V4 educational communication system. Uses explicit protocol method names for complete transparency.

## 🏗️ Architecture

This SDK uses **explicit protocol mapping** where method names match server protocol types exactly:

- **Protocol Transparency**: Method names = exact wire protocol types
- **No Role Restrictions**: Any client can send any message type (server handles filtering)
- **Semantic Flexibility**: Same protocol type supports multiple purposes via semantic names
- **Educational Context**: Names provide educational meaning while preserving protocol clarity

## 📁 Package Structure

```
javascript/
├── README.md              # This guide
├── package.json           # NPM package configuration
├── src/
│   ├── client.js          # Core WebSocket client
│   └── helpers.js         # Non-opinionated helper functions
├── ui/
│   ├── index.js           # UI package entry point
│   ├── styles.css         # Complete CSS theme system
│   └── components/
│       ├── ui.js          # Rich UI components
│       └── search.js      # Advanced search API
├── examples/              # Usage examples
│   ├── basic-integration.html      # Layer 1 (helpers)
│   ├── advanced-ui.html            # Layer 2 (UI components)
│   └── layer-comparison.html       # Side-by-side comparison
├── index.js               # Package entry point
├── switchboard-client.js  # Standalone build
└── switchboard-client.d.ts # TypeScript definitions
```

## Quick Start

### Installation

**For Development (from repository)**:
```bash
# Clone the repository
git clone https://github.com/switchboard/sdk.git
cd sdk/javascript

# Install dependencies
npm install

# Build the SDK
npm run build

# Run tests
npm test

# Start development mode
npm run dev
```

**npm/yarn**:
```bash
npm install switchboard-client
# or
yarn add switchboard-client
```

**CDN (Browser)**:
```html
<!-- Core SDK only -->
<script src="https://unpkg.com/switchboard-client@latest/dist/switchboard-client.umd.js"></script>

<!-- With UI components -->
<script src="https://unpkg.com/switchboard-client@latest/dist/ui/switchboard-ui.umd.js"></script>
<link rel="stylesheet" href="https://unpkg.com/switchboard-client@latest/ui/styles.css">
```

**ES Modules**:
```javascript
// Core SDK only
import SwitchboardClient from 'switchboard-client';

// With non-opinionated helpers
import { SwitchboardClient, helpers } from 'switchboard-client';

// Full UI components
import { SwitchboardClient, SwitchboardUI } from 'switchboard-client/ui';
import 'switchboard-client/ui/styles.css';

// Local development (if cloning repository)
import SwitchboardClient from './javascript/src/client.js';
import { SwitchboardUI } from './javascript/ui/index.js';
```

**CommonJS**:
```javascript
// Core SDK
const SwitchboardClient = require('switchboard-client');

// With helpers
const { SwitchboardClient, helpers } = require('switchboard-client');

// Full UI (requires additional setup for CSS)
const { SwitchboardClient, SwitchboardUI } = require('switchboard-client/ui');
```

### Basic Usage

#### Student Client
```javascript
const student = new SwitchboardClient({
  userId: 'alice_student',
  role: 'student',
  wsUrl: 'ws://localhost:8080/ws',
  hooks: {
    onSessionStarted: (session) => {
      console.log(`Class started: ${session.name}`);
    },
    onBroadcastToStudents: (message) => {
      console.log(`Announcement: ${message.content.text}`);
    }
  }
});

await student.connect();

// Send question to instructors
student.broadcast_to_instructors('helpRequest')
  .withText("How do loops work?")
  .withCode("for i := 0; i < 10; i++")
  .send();

// Send direct message to instructor
student.direct_message('prof_smith', 'question')
  .withText("Can I submit this late?")
  .send();
```

#### Instructor Client
```javascript
const instructor = new SwitchboardClient({
  userId: 'prof_smith',
  role: 'instructor',
  wsUrl: 'ws://localhost:8080/ws',
  hooks: {
    onBroadcastToInstructors: (message) => {
      console.log(`Student question: ${message.content.text}`);
    }
  }
});

await instructor.connect();

// Start a session (instructors only)
await instructor.startSession('Exercise 3: For Loops');

// Send announcement to all students
instructor.broadcast_to_students('announcement')
  .withText("Take a 10-minute break")
  .markAsImportant()
  .send();

// Respond to student privately
instructor.direct_message('alice_student', 'response')
  .withText("Great question! Here's the answer...")
  .referencingMessage('msg-123')
  .send();
```

## API Reference

### Core Methods

#### Protocol Methods (Any Client)

```javascript
// Send to all instructors
client.broadcast_to_instructors(semanticName)
  .withText(text)
  .withCode(code, language)
  .withData(data)
  .send();

// Send to all students  
client.broadcast_to_students(semanticName)
  .withText(text)
  .markAsImportant()
  .send();

// Send direct message
client.direct_message(toUser, semanticName)
  .withText(text)
  .referencingMessage(messageId)
  .send();
```

#### Factory Method (Most Explicit)

```javascript
// Most explicit - shows exact protocol type
client.Message('broadcast_to_instructors', 'helpRequest')
  .withText("Question text")
  .send();
```

### Message Builder Methods

```javascript
messageBuilder
  .withText(text)                    // Main message text
  .withCode(code, language)          // Code snippet with optional language
  .withLineNumber(number)            // Line number reference
  .withData(object)                  // Additional structured data
  .withContext(context)              // Override default context
  .withTags(...tags)                 // Add categorization tags
  .withUrgency('low'|'medium'|'high'|'urgent')  // Set urgency level
  .markAsImportant()                 // Mark as important
  .referencingMessage(messageId)     // Reference another message
  .send();                           // Send the message
```

### Session Management (Instructors Only)

```javascript
// Start session
const session = await instructor.startSession('Session Name');

// End session
await instructor.endSession();

// Get active session
const session = await instructor.getActiveSession();
```

### Connection Management

```javascript
// Connect
await client.connect();

// Disconnect
client.disconnect();

// Connection state
const connected = client.isConnected();
const sessionActive = client.isSessionActive();
const session = client.getCurrentSession();
```

### Message History

```javascript
// Get all messages
const messages = client.getMessages();

// Get messages by type
const questions = client.getMessages('broadcast_to_instructors');

// Get recent messages
const recent = client.getMessages(null, 10);

// Clear message history
client.clearMessages();
```

### Rate Limiting

```javascript
// Check rate limit status
const status = client.getRateLimitStatus();
console.log(`${status.messagesSent}/${status.maxMessages} messages sent`);
```

## Message Types & Contexts

### Protocol Types
- `broadcast_to_instructors` - Messages to all instructors
- `broadcast_to_students` - Messages to all students  
- `direct_message` - Private messages to specific users

### Semantic Names (Examples)
- `helpRequest` - Student asking for help
- `codeSubmission` - Code or work submission
- `announcement` - Important class announcements
- `response` - Replies to questions
- `analytics` - Performance or progress data
- `technicalIssue` - Technical problems
- `discovery` - Learning insights to share

### Auto-Assigned Contexts
The SDK automatically assigns appropriate contexts based on semantic names:
- `helpRequest` → `question`
- `codeSubmission` → `submission`  
- `announcement` → `announcement`
- `analytics` → `analytics`

## How Messages Are Translated to WebSocket Protocol

The SDK provides a fluent API that translates directly to WebSocket JSON messages. Here's exactly how your code becomes a server message:

### Example: Student Code Progress Update

**Your Code:**
```javascript
const workInProgress = student.broadcast_to_instructors('codeSnapshot')
  .withText("Current progress on exercise 3")
  .withCode(currentCode, 'go')
  .withData({
    exercise: 3, 
    progress: 0.6,
    nextSteps: ['implement error handling', 'add tests']
  })
  .send();
```

**Actual WebSocket Message Sent:**
```json
{
  "type": "broadcast_to_instructors",
  "context": "analytics",
  "content": {
    "name": "codeSnapshot",
    "text": "Current progress on exercise 3",
    "code_snippet": "// actual code here...",
    "language": "go",
    "exercise": 3,
    "progress": 0.6,
    "nextSteps": ["implement error handling", "add tests"]
  }
}
```

**What Instructors Receive (Server-Enhanced):**
```json
{
  "id": "msg-789",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "broadcast_to_instructors",
  "context": "analytics",
  "from_user": "alice_student",
  "to_user": null,
  "content": {
    "name": "codeSnapshot",
    "text": "Current progress on exercise 3",
    "code_snippet": "// actual code here...",
    "language": "go",
    "exercise": 3,
    "progress": 0.6,
    "nextSteps": ["implement error handling", "add tests"]
  },
  "timestamp": "2025-01-15T14:35:30Z"
}
```

### Translation Process:

1. **Method → Type**: `broadcast_to_instructors()` → `"type": "broadcast_to_instructors"`
2. **Semantic Name → Content**: `'codeSnapshot'` → `"name": "codeSnapshot"` in content
3. **Auto-Context**: 'codeSnapshot' automatically assigned `"context": "analytics"`
4. **Builder Methods → Content**: All `.withX()` calls merge into single `content` object
5. **Server Enhancement**: Server adds `id`, `session_id`, `from_user`, `timestamp`

### Simple Example: Ask a Question

**Your Code:**
```javascript
student.broadcast_to_instructors('helpRequest')
  .withText("How do loops work?")
  .send();
```

**WebSocket Message:**
```json
{
  "type": "broadcast_to_instructors",
  "context": "question",
  "content": {
    "name": "helpRequest",
    "text": "How do loops work?"
  }
}
```

### Key Points:

- **Protocol Transparency**: Method names match exact message types
- **No Client-Side Filtering**: Any role can send any message type
- **Server-Side Processing**: Server validates, routes, and filters based on recipient role
- **Content Preservation**: All data from builder methods preserved in `content` object
- **Automatic Metadata**: Server adds session and timing information

## Event Hooks

```javascript
const client = new SwitchboardClient({
  // ... options
  hooks: {
    // Connection events
    onConnecting: () => console.log('Connecting...'),
    onConnected: () => console.log('Connected!'),
    onDisconnected: (code, reason) => console.log('Disconnected'),
    
    // Message events  
    onMessage: (message) => console.log('Any message received'),
    onBroadcastToInstructors: (message) => console.log('Question received'),
    onBroadcastToStudents: (message) => console.log('Announcement received'),
    onDirectMessage: (message) => console.log('Direct message received'),
    
    // Session events
    onSessionStarted: (session) => console.log('Session started'),
    onSessionEnded: (session) => console.log('Session ended'),
    onSessionActive: (session) => console.log('Joined active session'),
    onWaitingForSession: () => console.log('Waiting for session'),
    onHistoryDelivered: () => console.log('History complete'),
    
    // Error events
    onError: (error) => console.error('Error:', error.message),
    onRateLimited: (error) => console.warn('Rate limited'),
    onNoActiveSession: (error) => console.warn('No active session'),
    onMessageTooLarge: (error) => console.error('Message too large'),
    onConnectionError: (error) => console.error('Connection error')
  }
});
```

## Usage Examples

### Student Examples

#### 1. Help Requests (Multiple Ways)

```javascript
// Factory method - most explicit
const helpRequest = student.Message('broadcast_to_instructors', 'helpRequest')
  .withText("I'm stuck on this loop")
  .withCode("for i := 0; i < 10; i++")
  .withLineNumber(42)
  .withUrgency('medium')
  .send();

// Direct protocol method - clean and explicit
const helpRequest2 = student.broadcast_to_instructors('helpRequest')
  .withText("How do nested loops work?")
  .withTags('loops', 'nested', 'golang')
  .send();

// Different semantic purpose, same protocol type
const technicalIssue = student.broadcast_to_instructors('technicalIssue')
  .withText("My IDE keeps crashing")
  .withData({ 
    browser: 'Chrome',
    os: 'macOS',
    timestamp: new Date().toISOString()
  })
  .send();
```

#### 2. Code Submissions

```javascript
// Final submission
const submission = student.broadcast_to_instructors('codeSubmission')
  .withText("Here's my solution for exercise 3")
  .withCode(finalSolutionCode, 'go')
  .withData({
    exercise: 3,
    confidence: 4,
    timeSpent: 45,
    attempts: 3
  })
  .markAsImportant() // Final submission
  .send();

// Work in progress
const workInProgress = student.broadcast_to_instructors('codeSnapshot')
  .withText("Current progress on exercise 3")
  .withCode(currentCode, 'go')
  .withData({
    exercise: 3, 
    progress: 0.6,
    nextSteps: ['implement error handling', 'add tests']
  })
  .send();
```

#### 3. Direct Messages

```javascript
// Private help request to instructor
const privateHelp = student.direct_message('prof_smith', 'helpRequest')
  .withText("Can you help me understand pointers?")
  .withCode("var ptr *int = &x")
  .send();

// Peer communication
const peerHelp = student.direct_message('alice_student', 'question')
  .withText("Did you figure out exercise 2?")
  .withData({ exercise: 2 })
  .send();

// Response to instructor
const response = student.direct_message('prof_smith', 'response')
  .withText("Thank you! That makes sense now.")
  .referencingMessage('msg-456')
  .send();
```

#### 4. Analytics and Tracking

```javascript
// Auto-tracking code snapshots
setInterval(() => {
  student.broadcast_to_instructors('codeSnapshot')
    .withCode(editor.getValue(), 'go')
    .withData({
      exercise: currentExercise,
      progress: calculateProgress(),
      linesChanged: getChangedLines(),
      timestamp: Date.now()
    })
    .send();
}, 30000); // Every 30 seconds

// Progress milestone
const milestone = student.broadcast_to_instructors('progressUpdate')
  .withText("Completed part 1 of exercise 3!")
  .withData({
    exercise: 3,
    part: 1,
    totalParts: 4,
    achievement: 'first_working_solution'
  })
  .send();
```

### Instructor Examples

#### 1. Announcements

```javascript
// Class announcement
const announcement = instructor.broadcast_to_students('announcement')
  .withText("Exercise 3 is due in 30 minutes")
  .markAsImportant()
  .withData({
    dueTime: new Date(Date.now() + 30 * 60000),
    exercise: 3,
    submissionUrl: '/submit/exercise3'
  })
  .send();

// Break announcement
const breakTime = instructor.broadcast_to_students('announcement')
  .withText("Take a 10-minute break. We'll resume at 2:30 PM")
  .withData({
    breakDuration: 10 * 60 * 1000,
    resumeTime: '2:30 PM'
  })
  .send();
```

#### 2. Instructions

```javascript
// Technical instruction
const instruction = instructor.broadcast_to_students('instruction')
  .withText("Please open your terminal and run 'go mod init exercise3'")
  .withCode("go mod init exercise3", "bash")
  .withData({
    step: 1,
    totalSteps: 5,
    exercise: 3
  })
  .send();

// Conceptual instruction
const concept = instructor.broadcast_to_students('instruction')
  .withText("Remember: loops iterate over data structures")
  .withData({
    concept: 'loops',
    examples: ['for', 'while', 'range']
  })
  .send();
```

#### 3. Responses to Students

```javascript
// Response to student question
const response = instructor.direct_message('alice_student', 'response')
  .withText("Great question! Here's how pointers work...")
  .withCode(`
    var x int = 42
    var ptr *int = &x
    fmt.Println(*ptr) // prints 42
  `, 'go')
  .referencingMessage('msg-123') // Student's original question
  .send();

// Feedback on submission
const feedback = instructor.direct_message('bob_student', 'feedback')
  .withText("Nice work! Consider adding error handling here:")
  .withCode("if err != nil { return err }", 'go')
  .withData({
    grade: 'A-',
    points: 85,
    suggestions: ['error handling', 'add comments']
  })
  .send();
```

#### 4. Analytics to Other Instructors

```javascript
// Class analytics
const classStats = instructor.broadcast_to_instructors('analytics')
  .withText("Current class progress on exercise 3")
  .withData({
    totalStudents: 25,
    submitted: 18,
    inProgress: 5,
    notStarted: 2,
    averageTime: 35,
    commonErrors: ['off-by-one', 'nil pointer'],
    helpRequests: 12
  })
  .send();

// Struggling student alert
const alert = instructor.broadcast_to_instructors('alert')
  .withText("Student needs additional support")
  .withData({
    studentId: 'struggling_student',
    issues: ['multiple help requests', 'no progress in 30 min'],
    recommendedAction: 'direct intervention'
  })
  .withUrgency('high')
  .send();
```

### Advanced Usage Patterns

#### 1. Message Chains with References

```javascript
// Student asks question
const question = student.broadcast_to_instructors('helpRequest')
  .withText("How do I handle errors in Go?")
  .withCode("result, err := someFunction()")
  .send();

// Teacher responds (assuming they get the message ID somehow)
const response = teacher.direct_message('student123', 'response')
  .withText("Use the error return value like this:")
  .withCode(`
    result, err := someFunction()
    if err != nil {
        log.Fatal(err)
    }
  `)
  .referencingMessage(question.id)
  .send();

// Student follows up
const followUp = student.direct_message('prof_smith', 'clarification')
  .withText("What if I want to handle the error gracefully instead of Fatal?")
  .referencingMessage(response.id)
  .send();
```

#### 2. Automated Code Tracking

```javascript
class CodeTracker {
  constructor(student, exerciseNumber) {
    this.student = student;
    this.exerciseNumber = exerciseNumber;
    this.lastSnapshot = '';
    this.snapshotInterval = null;
  }
  
  startTracking() {
    this.snapshotInterval = setInterval(() => {
      const currentCode = editor.getValue();
      
      // Only send if code changed significantly
      if (this.hasSignificantChange(currentCode)) {
        this.student.broadcast_to_instructors('codeSnapshot')
          .withCode(currentCode, 'go')
          .withData({
            exercise: this.exerciseNumber,
            progress: this.calculateProgress(currentCode),
            changesSince: this.getChangesSince(currentCode),
            timestamp: Date.now()
          })
          .send();
          
        this.lastSnapshot = currentCode;
      }
    }, 30000);
  }
  
  stopTracking() {
    if (this.snapshotInterval) {
      clearInterval(this.snapshotInterval);
    }
  }
  
  hasSignificantChange(newCode) {
    const changes = this.getChangesSince(newCode);
    return changes.linesAdded > 2 || changes.linesRemoved > 1;
  }
  
  // ... other methods
}

// Usage
const tracker = new CodeTracker(student, 3);
tracker.startTracking();
```

#### 3. Multi-Purpose Message Types

```javascript
// Same protocol type (broadcast_to_instructors) for different purposes

// 1. Question
student.broadcast_to_instructors('helpRequest')
  .withText("How do interfaces work?")
  .send();

// 2. Submission  
student.broadcast_to_instructors('codeSubmission')
  .withCode(solution)
  .send();

// 3. Analytics
student.broadcast_to_instructors('progressUpdate')
  .withData({ completed: true, timeSpent: 45 })
  .send();

// 4. Discovery sharing
student.broadcast_to_instructors('discovery')
  .withText("I found a cool pattern!")
  .withCode("type Stringer interface { String() string }")
  .send();

// 5. Technical issue
student.broadcast_to_instructors('technicalIssue')
  .withText("My terminal is not responding")
  .send();
```

## Error Handling

```javascript
try {
  student.broadcast_to_instructors('helpRequest')
    .withText("") // This will fail - empty text
    .send();
} catch (error) {
  console.error('Message failed:', error.message);
  // "Message 'helpRequest' needs text, code, or data content"
}

// Handle rate limiting
client.hooks.onRateLimited = (error) => {
  showNotification('Please slow down - sending messages too quickly');
  disableSendButton(10000); // Disable for 10 seconds
};

// Handle connection errors
client.hooks.onConnectionError = (error) => {
  showErrorDialog('Connection failed. Please check your internet connection.');
};
```

## Best Practices

### 1. Use Descriptive Semantic Names
```javascript
// Good - clear educational purpose
student.broadcast_to_instructors('helpRequest')
student.broadcast_to_instructors('codeSubmission')
instructor.broadcast_to_students('announcement')

// Avoid - too generic
student.broadcast_to_instructors('message')
student.broadcast_to_instructors('data')
```

### 2. Include Rich Context
```javascript
// Good - rich contextual information
student.broadcast_to_instructors('helpRequest')
  .withText("I'm getting a segmentation fault")
  .withCode(problematicCode)
  .withLineNumber(42)
  .withData({
    exercise: 3,
    attempt: 2,
    errorMessage: "segmentation fault (core dumped)"
  })
  .withTags('debugging', 'segfault', 'pointers')
  .send();
```

### 3. Use Appropriate Message Types
```javascript
// Public question - broadcast_to_instructors
student.broadcast_to_instructors('helpRequest')
  .withText("General question about loops")
  .send();

// Private question - direct_message  
student.direct_message('prof_smith', 'helpRequest')
  .withText("Personal question about my grade")
  .send();

// Class-wide info - broadcast_to_students
instructor.broadcast_to_students('announcement')
  .withText("Important class update")
  .send();
```

## Configuration

```javascript
const client = new SwitchboardClient({
  userId: 'user123',                    // Required: unique user identifier
  role: 'student',                      // Required: 'student' or 'instructor'
  wsUrl: 'ws://localhost:8080/ws',      // Required: WebSocket URL
  apiUrl: 'http://localhost:8080/api',  // Optional: API URL (auto-derived)
  
  // Optional configuration
  queueMessages: true,                  // Queue messages when disconnected
  maxStoredMessages: 1000,              // Maximum stored message history
  debug: false,                         // Enable debug logging
  
  hooks: {
    // Event handlers (see Event Hooks section)
  }
});
```

## Protocol Compliance

This SDK implements the complete Switchboard V4 protocol:

- ✅ **WebSocket Protocol**: Automatic ping/pong heartbeat handling
- ✅ **Message Types**: All 3 protocol types supported
- ✅ **Rate Limiting**: Client-side validation (100 messages/minute)
- ✅ **Message Size**: 64KB limit with validation
- ✅ **Session Management**: Full session lifecycle support
- ✅ **Error Handling**: Complete error code coverage
- ✅ **Connection Management**: Clean connect/disconnect with message queuing
- ✅ **Role Filtering**: Receive-side filtering (send-side transparent)

## Browser Support

- Chrome 16+
- Firefox 11+  
- Safari 7+
- Edge 12+
- IE 10+ (with WebSocket polyfill)

## Node.js Support

- Node.js 14+
- Requires `ws` package for WebSocket support

```bash
npm install ws
```

## Advanced Features

### Search/Filter API

The SDK includes powerful message search and filtering capabilities:

```javascript
// Search for recent questions
const recentQuestions = client.search
  .where('type', 'broadcast_to_instructors')
  .where('context', 'question')
  .since('1h ago')
  .orderBy('timestamp', 'desc')
  .get();

// Find messages with code snippets
const codeMessages = client.search
  .having('content.code_snippet')
  .containing('loop')
  .get();

// Group messages by type and count
const messageStats = client.search
  .groupBy('type')
  .having(messages => messages.length > 5)
  .get();

// Complex queries
const urgentQuestions = client.search
  .where('type', 'broadcast_to_instructors')
  .where('content.urgency', 'urgent')
  .notReferenced()  // Questions without responses
  .since('30m ago')
  .orderBy('timestamp', 'desc')
  .limit(10)
  .get();
```

**Available Search Methods:**
- `where(field, value)` - Filter by field value
- `containing(text)` - Text search in message content
- `having(field)` / `notHaving(field)` - Check field existence
- `since(time)` / `before(time)` - Time range filtering
- `between(start, end)` - Time range
- `referencedBy(messageId)` - Find replies to a message
- `notReferenced()` - Find messages without replies
- `groupBy(field)` - Group results by field
- `orderBy(field, direction)` - Sort results
- `limit(count)` / `offset(count)` - Pagination
- `count()` / `first()` / `last()` - Result methods

### Message Helpers

Built-in utilities for working with messages:

```javascript
// Format messages for display
const formatted = client.helpers.formatMessage(message);
console.log(formatted.timeAgo);  // "5m ago"
console.log(formatted.excerpt);  // Truncated text

// Check connection status
const status = client.helpers.getConnectionStatus();
console.log(status.canSendMessages);  // true/false

// Group messages into conversation threads
const threads = client.helpers.groupMessageThreads(messages);
threads.forEach(thread => {
  console.log(`Thread started by: ${thread.original.from_user}`);
  console.log(`Responses: ${thread.responses.length}`);
  console.log(`Resolved: ${thread.isResolved}`);
});

// Parse code from messages
const codeInfo = client.helpers.parseCode(message.content);
if (codeInfo) {
  console.log(`Language: ${codeInfo.language}`);
  console.log(`Complete: ${codeInfo.isComplete}`);
}

// Extract @mentions
const mentions = client.helpers.extractMentions(message.content.text);
mentions.forEach(username => console.log(`Mentioned: ${username}`));
```

### Optional UI Components

For rapid development, use the optional UI library (requires separate import):

```html
<script src="javascript/switchboard-client.js"></script>
<script src="javascript/ui/switchboard-ui.js"></script>
```

```javascript
// Create UI components
const client = new SwitchboardClient(options);
const ui = new SwitchboardUI(client, {
  theme: 'light',  // 'light', 'dark', 'auto', or custom theme object
  animations: true,
  sounds: false
});

// Message list with rich features
const messageList = ui.createMessageList('#messages', {
  showAvatars: true,
  enableReactions: true,
  codeHighlighting: 'prism',  // 'prism', 'highlight.js', or false
  bubbles: true,
  threadLines: false
});

// Smart message input
const input = ui.createMessageInput('#input', {
  codeButton: true,
  autoComplete: true,
  characterLimit: true,
  templates: [
    { name: 'Question', preview: 'I have a question about...' },
    { name: 'Code Help', preview: 'Can you help with this code?' }
  ]
});

// Connection status badge
const badge = ui.createStatusBadge('#status', {
  showUserCount: true,
  showSessionInfo: true,
  pulseOnActivity: true
});

// Complete dashboard
const dashboard = ui.createDashboard('#app', {
  layout: 'student',  // 'student' or 'instructor'
  components: ['messageList', 'activeQuestions', 'studentRoster']
});

// Enable browser notifications
ui.enableNotifications({
  desktop: true,
  sounds: true,
  vibration: true
});

// Theme management
ui.setTheme('dark');
ui.setTheme({
  colors: {
    'bg-primary': '#1a1a1a',
    'accent': '#00d4aa'
  }
});
```

**UI Component Features:**
- **Message List**: Avatar support, code highlighting, reactions, threading
- **Message Input**: Templates, code insertion, auto-complete, drag-drop
- **Status Badge**: Live connection status, session info, activity pulse
- **Dashboard**: Role-specific layouts with analytics and controls
- **Notifications**: Desktop notifications, sound effects, vibration
- **Theming**: Light/dark/auto themes with custom color support

### Non-Opinionated Helpers

For custom UIs, use the lightweight helper functions:

```javascript
// Import non-opinionated helpers
import { helpers } from './javascript/ui/index.js';

// Create basic message elements
const messageEl = helpers.createMessageElement(message, {
  className: 'my-message',
  currentUser: 'alice'
});

// Format messages without styling
const formatted = helpers.formatMessage(message, {
  includeFormatted: true,
  currentUser: 'alice'
});

// Create status indicators
const status = helpers.createStatusIndicator('connected');

// Group messages by time
const groups = helpers.groupMessagesByTime(messages, 'hour');

// Extract mentions and other content
const mentions = helpers.extractMentions(messageText);
const elements = helpers.parseMessageContent(message.content);
```

## TypeScript Support

Full TypeScript definitions are included:

```typescript
import SwitchboardClient from './javascript/src/client.js';
import type { Message, MessageType, Session } from './javascript/switchboard-client';

const client = new SwitchboardClient({
  userId: 'alice',
  role: 'student',
  wsUrl: 'ws://localhost:8080/ws'
});

// Type-safe message building
const message: MessageBuilder = client.broadcast_to_instructors('helpRequest')
  .withText('How do loops work?')
  .withCode('for i := 0; i < 10; i++', 'go')
  .withTags('loops', 'golang');

// Type-safe search API
const results: Message[] = client.search
  .where('type', 'broadcast_to_instructors' as MessageType)
  .containing('loop')
  .get();

// Optional UI components with types
import { SwitchboardUI } from './ui/switchboard-ui';
const ui = new SwitchboardUI(client);
```

**TypeScript Features:**
- Complete type definitions for all SDK methods
- Generic types for search results and message content
- Interface definitions for all events and options
- Optional UI component types (separate module)

## License

MIT License - see LICENSE file for details.