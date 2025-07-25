# Switchboard SDKs

This directory contains client SDKs for the Switchboard real-time educational messaging system in multiple programming languages.

## Available SDKs

### Python SDK (`./python/`)
- **Target**: Server-side applications, AI bots, data analysis tools
- **Features**: Async/await support, comprehensive error handling, type hints
- **Use Cases**: AI tutoring bots, analytics processors, server integrations
- **Installation**: `pip install -e ./python`

### JavaScript/TypeScript SDK (`./javascript/`)
- **Target**: Web applications, Node.js services, React components
- **Features**: Universal (browser + Node.js), TypeScript support, React hooks
- **Use Cases**: Student/teacher dashboards, real-time classroom apps
- **Installation**: `npm install ./javascript`

## Quick Comparison

| Feature | Python SDK | JavaScript SDK |
|---------|------------|----------------|
| **Environment** | Server-side | Universal (browser + Node.js) |
| **Type Safety** | Type hints | Full TypeScript |
| **React Support** | ❌ | ✅ React hooks |
| **AI Integration** | ✅ Excellent | ✅ Good |
| **Performance** | High | High |
| **Learning Curve** | Low | Low-Medium |

## Architecture Overview

Both SDKs follow the same architectural patterns:

```
┌─────────────────┐
│   Client Apps   │
│  (Your Code)    │
└─────────────────┘
         │
┌─────────────────┐
│  Switchboard    │
│      SDK        │
└─────────────────┘
         │
┌─────────────────┐
│   WebSocket +   │
│   HTTP APIs     │
└─────────────────┘
         │
┌─────────────────┐
│  Switchboard    │
│     Server      │
└─────────────────┘
```

### Common Features

Both SDKs provide:

1. **Role-Based Clients**
   - `SwitchboardStudent` - For student applications
   - `SwitchboardTeacher` - For instructor applications

2. **Session Management**
   - Session discovery and connection
   - Automatic enrollment validation
   - Graceful disconnection handling

3. **Message Types**
   - Student messages: `instructor_inbox`, `request_response`, `analytics`
   - Teacher messages: `inbox_response`, `request`, `instructor_broadcast`
   - System messages: Connection status, errors, notifications

4. **Connection Reliability**
   - Automatic reconnection with exponential backoff
   - Connection state management
   - Error handling and recovery

5. **Event-Driven Architecture**
   - Message type handlers
   - Connection state callbacks
   - Error event handling

## Quick Start Examples

### Python Student Bot

```python
from switchboard_sdk import SwitchboardStudent

async def main():
    student = SwitchboardStudent("student_001")
    
    # Set up handlers
    @student.on_instructor_broadcast
    async def handle_broadcast(message):
        print(f"Announcement: {message.content['text']}")
    
    # Connect and interact
    session = await student.connect_to_available_session()
    await student.ask_question({
        "text": "I need help with this problem",
        "urgency": "medium"
    })
```

### JavaScript Teacher Dashboard

```javascript
import { SwitchboardTeacher } from '@switchboard/sdk';

const teacher = new SwitchboardTeacher('teacher_001');

teacher.setupEventHandlers({
  onStudentQuestion: async (message) => {
    console.log(`Question: ${message.content.text}`);
    await teacher.respondToStudent(message.from_user, {
      text: "Great question! Here's the answer..."
    });
  }
});

const session = await teacher.createAndConnect('Math Class', ['student_001']);
await teacher.announce('Welcome to class!');
```

### React Student Component

```jsx
import { useSwitchboardStudent } from '@switchboard/sdk';

function StudentApp() {
  const { client, connected, messages } = useSwitchboardStudent('student_001');
  
  const askQuestion = async () => {
    await client.askQuestion({
      text: "How do I solve this?",
      urgency: "high"
    });
  };
  
  return (
    <div>
      <div>Status: {connected ? 'Connected' : 'Disconnected'}</div>
      <button onClick={askQuestion}>Ask Question</button>
      <div>Messages: {messages.length}</div>
    </div>
  );
}
```

## Use Case Examples

### 1. AI Tutoring Bot (Python)
```python
class AITutorBot(SwitchboardStudent):
    async def on_instructor_broadcast(self, message):
        if message.context == "problem":
            hint = await self.generate_ai_hint(message.content)
            await self.ask_question({"hint": hint}, "hint")
```

### 2. Real-time Dashboard (React)
```jsx
function TeacherDashboard() {
  const { messages, createSession } = useSwitchboardTeacher('teacher_001');
  
  const analytics = messages.filter(m => m.type === 'analytics');
  
  return <AnalyticsChart data={analytics} />;
}
```

### 3. Classroom Management (JavaScript)
```javascript
const teacher = new SwitchboardTeacher('teacher_001');

// Broadcast problem to AI tutors
await teacher.broadcastProblem({
  problem: "Debug this code",
  code: "function broken() { return x.undefined; }",
  frustrationLevel: 4
});
```

## Message Flow Examples

### Student Question → Teacher Response
```
Student App (Python)          Teacher App (React)
     │                              │
     │ ── ask_question() ──→ instructor_inbox ──→ │
     │                              │
     │ ←── inbox_response ←── respond_to_student() ─── │
     │                              │
```

### Problem Broadcast → AI Hints
```
Teacher App                AI Bot (Python)         
     │                        │
     │ ── broadcast_problem() ──→ instructor_broadcast ──→ │
     │                        │
     │ ←── instructor_inbox ←── ask_question(hint) ─── │
     │                        │
```

## Examples Directory

The `examples/` directory contains complete, runnable examples:

- **`python-student-bot.py`** - AI expert bot implementation
- **`javascript-teacher-app.js`** - Interactive teacher CLI application  
- **`react-student-component.jsx`** - Full-featured React student dashboard
- **`expert-config.json`** - Configuration for AI bots

## Getting Started

1. **Choose your SDK** based on your application needs
2. **Install dependencies** using the package manager
3. **Review the examples** for your chosen language
4. **Read the SDK-specific documentation** in each directory
5. **Start with a simple connection** and add features incrementally

## Migration from Raw WebSocket

If you're currently using raw WebSocket connections, the SDKs provide:

- **80% less boilerplate code** (from ~500 lines to ~50-100 lines)
- **Built-in error handling** and reconnection logic
- **Type safety** and validation
- **Event-driven architecture** instead of manual message parsing
- **Session management** helpers
- **Rate limiting** compliance

### Before (Raw WebSocket)
```javascript
// 50+ lines of connection, reconnection, message parsing, error handling...
const ws = new WebSocket('ws://localhost:8080/ws?user_id=student&role=student&session_id=123');
ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  if (message.type === 'instructor_broadcast') {
    // Handle message...
  }
  // More manual parsing...
};
```

### After (SDK)
```javascript
// 5-10 lines with built-in reliability
const student = new SwitchboardStudent('student_001');
student.onInstructorBroadcast((message) => {
  // Handle message...
});
await student.connectToAvailableSession();
```

## Contributing

When contributing to the SDKs:

1. **Maintain API consistency** between languages
2. **Follow language conventions** (naming, async patterns, etc.)
3. **Add comprehensive tests** for new features
4. **Update documentation** and examples
5. **Ensure compatibility** with the Switchboard server protocol

## Support

- **Documentation**: See README files in each SDK directory
- **Examples**: Complete examples in `./examples/`
- **Issues**: Report issues in the main Switchboard repository
- **Protocol**: Refer to `../docs/switchboard-tech-specs.md` for server protocol details