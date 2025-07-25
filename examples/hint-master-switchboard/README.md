# AI Programming Mentorship - Switchboard Integration

This is a **dramatically simplified** version of the AI Programming Mentorship demo that uses the Switchboard real-time messaging system instead of a custom WebSocket implementation. 

## Key Improvements

- **Automatic message persistence** with full history replay
- **Production-ready connection management** with automatic reconnection
- **Session-based architecture** with proper lifecycle management
- **Built-in rate limiting** and error handling
- **Zero server infrastructure** to manage (teacher client is just static files)

## Architecture Overview

## Prerequisites

1. **Switchboard Server Running**
   ```bash
   cd /path/to/switchboard
   make run
   ```
   
2. **Node.js Dependencies**
   ```bash
   npm install
   ```

3. **Gemini API Key**
   ```bash
   ./setup-api-keys.sh YOUR_GEMINI_API_KEY [gemini-model]
   ```

## Quick Start

### 1. Start the Teacher Dashboard
```bash
npm run teacher
# Opens http://localhost:3000
```

### 2. Start All AI Experts
```bash
npm run start-all-experts
# Starts 5 AI experts that connect to Switchboard
```

### 3. Use the System
1. Open http://localhost:3000 in your browser
2. Click "Create New Session" (automatically enrolls all 5 AI experts)
3. Fill out the problem form and click "Broadcast to AI Experts"
4. Watch real-time hints appear from all connected experts

## The 5 AI Experts

Each expert connects to Switchboard as a **student** with a unique personality:

1. **Technical Expert** (`technical_expert`) - Algorithms and optimization
2. **Emotional Support Coach** (`emotional_support_coach`) - Motivation and confidence
3. **Debugging Guru** (`debugging_guru`) - Error detection and debugging strategies  
4. **Learning Coach** (`learning_coach`) - Conceptual understanding and teaching
5. **Architecture Expert** (`architecture_expert`) - Design patterns and code structure

## How It Works

### Session Management (Teacher)
```javascript
// Create session with all 5 AI experts enrolled
const session = await client.createSession("AI Mentorship Session");
// Experts are automatically enrolled with these user IDs:
// ["technical_expert", "emotional_support_coach", "debugging_guru", 
//  "learning_coach", "architecture_expert"]
```

### Expert Discovery (AI Experts)
```javascript
// Each expert discovers sessions they're enrolled in
const sessions = await fetch('/api/sessions').then(r => r.json());
const mySessions = sessions.filter(s => s.student_ids.includes(myUserId));
// Connect to first available session
await connectToSession(mySessions[0].id);
```

### Message Flow
1. **Teacher broadcasts problem** → `instructor_broadcast` with context `"problem"`
2. **AI experts receive problem** → Generate hints using Gemini API
3. **Experts send hints** → `instructor_inbox` with context `"hint"`
4. **Teacher receives hints** → Display in real-time dashboard

## Message History & Late Joining

**Key Advantage**: When new experts join a session, they automatically receive complete message history:

- All previous problem broadcasts
- All previous hints from other experts
- Full context for generating relevant hints

This enables experts to provide better, more contextual hints even when joining late.

## File Structure

```
hint-master-switchboard/
├── teacher-client/
│   ├── index.html              # Teacher dashboard with session management
│   ├── app.js                  # UI logic and Switchboard integration
│   ├── switchboard-client.js   # Switchboard client (~50 lines)
│   ├── style.css              # Updated styles
│   └── server.js              # Simple static file server (~20 lines)
├── student-client/
│   ├── switchboard-expert.js  # Expert Switchboard client (~200 lines)
│   ├── start-all-experts.sh   # Startup script
│   └── experts/               # AI expert configurations
│       ├── technical-expert.json
│       ├── emotional-support.json
│       ├── debugging-guru.json
│       ├── learning-coach.json
│       └── architecture-expert.json
├── package.json               # Dependencies and scripts
├── setup-api-keys.sh         # API key setup script
└── README.md                 # This file
```

## Configuration

### Expert Configuration Example
```json
{
  "expert_profile": {
    "name": "Technical Expert",
    "user_id": "technical_expert",
    "expertise": "algorithms, optimization",
    "personality": "direct, analytical"
  },
  "switchboard": {
    "server_url": "http://localhost:8080",
    "role": "student"
  },
  "gemini_config": {
    "api_key": "your_key_here",
    "model": "gemini-2.0-flash",
    "max_tokens": 150,
    "temperature": 0.7
  }
}
```

## Development Commands

```bash
# Start teacher dashboard only
npm run teacher

# Start single expert
npm run expert experts/technical-expert.json

# Check if Switchboard is running
npm run health

# Update all API keys
./setup-api-keys.sh YOUR_API_KEY gemini-2.0-flash
```

## Troubleshooting

### "Switchboard server not running"
- Make sure Switchboard is running on port 8080: `cd /path/to/switchboard && make run`

### "No sessions found" (Experts)
- Create a session from the teacher dashboard first
- Session automatically enrolls all 5 expert user IDs

### "Invalid API key" (Experts)
- Run `./setup-api-keys.sh YOUR_ACTUAL_API_KEY`
- Make sure your Gemini API key is valid

### Experts not connecting
- Check Switchboard server logs for connection errors
- Verify expert user IDs match the session enrollment

## Comparison with Original

| Feature | Original | Switchboard |
|---------|----------|-------------|
| **Lines of Code** | 777 | 80 |
| **Message Persistence** | ❌ | ✅ |
| **History Replay** | ❌ | ✅ |
| **Session Management** | ❌ | ✅ |
| **Connection Recovery** | Manual | Automatic |
| **Rate Limiting** | Manual | Built-in |
| **Multi-Session Support** | ❌ | ✅ |
| **Server Management** | Complex | None |

## Benefits of Switchboard Integration

1. **Simplicity**: 90% reduction in code complexity
2. **Reliability**: Production-ready connection handling
3. **Persistence**: All messages saved automatically
4. **History**: Late-joining experts get full context
5. **Scalability**: Built-in rate limiting and error handling
6. **Maintainability**: No custom server infrastructure to maintain

This demo showcases how Switchboard dramatically simplifies real-time application development while adding enterprise-grade features that would be extremely complex to implement manually.