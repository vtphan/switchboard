# Switchboard SDK

Multi-language SDK collection for Switchboard V4 educational communication system. This repository provides client libraries for connecting to Switchboard servers across different programming languages.

## 🌍 Available SDKs

### JavaScript SDK
**Location**: [`./javascript/`](./javascript/)  
**Status**: ✅ Complete  
**Package**: [`switchboard-client`](https://www.npmjs.com/package/switchboard-client)

A focused JavaScript/TypeScript SDK with:
- Core WebSocket client for real-time messaging
- Message search and filtering capabilities
- Complete TypeScript definitions
- Browser and Node.js support
- Separate teacher/student example applications

**Quick Start**:
```bash
npm install switchboard-client
```

[📖 **JavaScript SDK Documentation →**](./javascript/README.md)

### Python SDK
**Location**: [`./python/`](./python/)  
**Status**: 🚧 Planned  

Python SDK for Switchboard V4 (coming soon).

## 🏗️ Architecture

All Switchboard SDKs follow a consistent architecture based on **explicit protocol mapping**:

- **Protocol Transparency**: Method names match exact WebSocket message types
- **No Role Restrictions**: Any client can send any message type (server handles filtering)
- **Semantic Flexibility**: Same protocol type supports multiple educational purposes
- **Educational Context**: Method names reflect classroom terminology

### Core Message Types

1. **`broadcast_to_instructors`** - Messages from students to all instructors
2. **`broadcast_to_students`** - Messages from instructors to all students  
3. **`direct_message`** - Private messages between specific users

### Example Usage Pattern (JavaScript)

```javascript
// Student asking for help
student.broadcast_to_instructors('helpRequest')
  .withText("How do loops work?")
  .withCode("for i := 0; i < 10; i++", 'go')
  .send();

// Instructor responding
instructor.direct_message('student123', 'response')
  .withText("Here's how loops work...")
  .referencingMessage('msg-456')
  .send();

// Class announcement
instructor.broadcast_to_students('announcement')
  .withText("Exercise 3 is due in 30 minutes")
  .markAsImportant()
  .send();
```

## 📁 Repository Structure

```
sdk/
├── README.md              # This overview (you are here)
├── javascript/            # JavaScript SDK
│   ├── README.md          # JavaScript SDK documentation
│   ├── package.json       # JavaScript package config
│   ├── src/               # Core client implementation
│   ├── examples/          # MVC example applications
│   │   ├── student/       # Student client example
│   │   ├── teacher/       # Teacher client example
│   │   └── shared-styles.css # Shared styling
│   ├── switchboard-client.js  # Standalone build
│   └── switchboard-client.d.ts # TypeScript definitions
├── student-client-sdk.md  # Student client specification
└── teacher-client-sdk.md  # Teacher client specification
```

## 🚀 Development

### Prerequisites

- Node.js 14+ (for JavaScript SDK)
- Python 3.8+ (for Python SDK, when available)

### JavaScript SDK Development

```bash
# Navigate to JavaScript SDK
cd javascript/

# Install dependencies
npm install

# Run tests
npm test

# Start development server (examples)
npm run dev

# Lint code
npm run lint
```

## 📋 Client Specifications

- **[Student Client SDK](./student-client-sdk.md)** - Complete specification for student client implementations
- **[Teacher Client SDK](./teacher-client-sdk.md)** - Complete specification for instructor client implementations

These specifications define the exact behavior, message filtering, and capabilities that each SDK should implement.

## 🌐 Protocol Overview

Switchboard V4 uses WebSocket communication with a simple 3-message-type protocol:

### Session Management
- **Single Active Session**: Only one session can be active at a time
- **Pre-connection Support**: Users can connect before sessions start
- **Complete History**: Late joiners receive full session history (role-filtered)
- **Message Gating**: No messages accepted without active session

### Real-time Features
- **WebSocket Heartbeat**: Automatic ping/pong for connection monitoring
- **Rate Limiting**: 100 messages per minute per user
- **Message Size Limit**: 64KB per message
- **Role-based Filtering**: Students see limited messages, instructors see all

### Educational Focus
- **Semantic Message Names**: `helpRequest`, `codeSubmission`, `announcement`, etc.
- **Rich Content Support**: Text, code snippets, structured data, tags
- **Context Awareness**: Messages automatically categorized by educational context
- **Reference Support**: Messages can reference other messages for threading

## 🤝 Contributing

We welcome contributions to any of the SDKs! Please see individual SDK directories for language-specific contribution guidelines.

### Adding a New Language SDK

1. Create a new directory: `{language}/`
2. Follow the established patterns from `javascript/`
3. Implement the client specifications from `student-client-sdk.md` and `teacher-client-sdk.md`
4. Add workspace configuration to root `package.json`
5. Update this README with the new SDK information

## 📄 License

MIT License - see [LICENSE](./LICENSE) file for details.

## 🔗 Related

- **[Switchboard V4 Server](../README.md)** - The Go-based server implementation
- **[Technical Specifications](../docs/tech-specs.md)** - Complete protocol documentation
- **[Architecture Documentation](../docs/)** - System design and patterns