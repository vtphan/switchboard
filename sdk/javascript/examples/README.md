# Switchboard JavaScript SDK Examples

Professional, separated client examples demonstrating real-world SDK integration patterns for educational communication systems.

## 🏗️ Architecture

These examples follow **MVC separation of concerns**:
- **HTML**: Pure structure with semantic markup
- **CSS**: Clean, responsive styling (shared across clients)  
- **JavaScript**: Focused SDK integration demonstrations

## 🚀 Quick Start

### 1. Start the Switchboard Server
```bash
# From the project root directory
cd ../../../
make run
```
The server will start on `http://localhost:8080`

### 2. Open the Clients

**Teacher Client (start first):**
- Open `teacher/index.html` in your browser
- Or navigate to: `http://localhost:8080/sdk/javascript/examples/teacher/`

**Student Client(s):**
- Open `student/index.html` in another browser tab/window
- Or navigate to: `http://localhost:8080/sdk/javascript/examples/student/`
- Open multiple student clients to simulate a classroom

### 3. Typical Workflow
1. **Teacher**: Connect → Start Session
2. **Students**: Connect (will join active session automatically)
3. **Students**: Ask questions, receive announcements
4. **Teacher**: See questions, send responses and announcements
5. **Teacher**: End session when done

## 📁 Example Structure

```
examples/
├── README.md                 # This guide
├── shared-styles.css         # Clean, professional styling
├── student/
│   ├── index.html           # Student interface structure
│   └── student-app.js       # SDK integration for students
└── teacher/
    ├── index.html           # Teacher interface structure  
    └── teacher-app.js       # SDK integration for instructors
```

## 🎓 Student Client Features

**SDK Integration Demonstrations:**
- Client initialization with hooks
- Connection management and auto-reconnection
- Question submission with code snippets
- Real-time announcement reception
- Session lifecycle handling

**Key Hooks Showcased:**
- `onBroadcastToStudents` - Receive announcements
- `onSessionStarted/Ended` - React to session changes
- `onConnected/Disconnected` - Handle connection states
- `onError/RateLimited` - Error handling patterns

**UI Features:**
- Clean message display with timestamps
- Code snippet formatting
- Connection status indicators
- Question submission form
- Live statistics

## 🎓 Teacher Client Features

**SDK Integration Demonstrations:**
- Session management (start/end sessions)
- Broadcasting announcements to all students
- Receiving and responding to student questions
- Direct messaging capabilities
- Advanced error handling

**Key Hooks Showcased:**
- `onBroadcastToInstructors` - Receive student questions
- `onSessionStarted/Ended` - Session lifecycle management
- Session management hooks
- Student interaction patterns

**UI Features:**
- Session control interface
- Student question queue with click-to-respond
- Announcement broadcasting
- Quick response system
- Live classroom analytics

## 💡 SDK Patterns Demonstrated

### Client Initialization
```javascript
this.client = new SwitchboardClient({
    userId: 'student_alice',
    role: 'student',
    wsUrl: 'ws://localhost:8080/ws',
    hooks: {
        onConnected: () => { /* Update UI */ },
        onBroadcastToStudents: (message) => { /* Handle announcements */ },
        // ... more hooks
    }
});
```

### Message Building (Fluent API)
```javascript
// Student asking question with code
client.broadcast_to_instructors('helpRequest')
    .withText("How do loops work?")
    .withCode("for i := 0; i < 10; i++", 'go')
    .withTags('question', 'loops')
    .send();

// Teacher announcement
client.broadcast_to_students('announcement')
    .withText("Take a 10-minute break")
    .markAsImportant()
    .send();
```

### Hook-Driven UI Updates
```javascript
onBroadcastToStudents: (message) => {
    this.messagesReceived++;
    this.updateStats();
    
    if (message.content?.name === 'announcement') {
        this.addAnnouncementMessage(message);
    }
}
```

### Session Management
```javascript
// Instructor starts session
await this.client.startSession('Introduction to JavaScript');

// Students automatically join when connecting
// Session info updates via onSessionActive hook
```

## 🛠️ Technical Features

### Connection Management
- Auto-reconnection with exponential backoff
- Connection state indicators
- Graceful error handling
- Message queuing during disconnection

### Real-Time Updates
- WebSocket-based messaging
- Event-driven UI updates using hooks
- Live statistics and status indicators
- Automatic message history on connect

### Message Types
- `broadcast_to_instructors` - Student questions to all teachers
- `broadcast_to_students` - Teacher announcements to all students
- `direct_message` - Private conversations

### Error Handling
- Rate limiting protection
- Message size validation
- Connection error recovery
- User-friendly error messages

## 🎨 Customization

### Styling
- Modify `shared-styles.css` for visual customization
- CSS variables for easy theme changes
- Responsive design included
- Role-specific color schemes

### Functionality
- Copy JS patterns to your own applications
- Extend with additional SDK features
- Integrate with your preferred frameworks
- Customize message types and contexts

## 🔧 Development Tips

### Debugging
- Open browser developer tools to see WebSocket messages
- Check console for SDK debug information
- Use network tab to monitor connection health

### Testing Multiple Clients
- Open multiple browser tabs/windows
- Use different browser profiles for separate sessions
- Test with various network conditions

### Framework Integration
These vanilla JS patterns translate easily to:
- **React**: Use hooks for state management, effects for SDK events
- **Vue**: Use reactive data and watchers for SDK integration  
- **Angular**: Use services for SDK client, observables for events

## 🆘 Troubleshooting

**Connection Issues:**
- Ensure Switchboard server is running on localhost:8080
- Check for firewall or port conflicts
- Verify browser WebSocket support

**Session Problems:**
- Teacher must start session before students can send messages
- Only instructors can start/end sessions
- Students automatically join active sessions on connect

**Message Delivery:**
- Check connection status indicators
- Verify session is active
- Monitor browser console for errors

**Performance:**
- Modern browsers recommended (Chrome, Firefox, Safari, Edge)
- Test with realistic number of concurrent students
- Monitor memory usage with developer tools

---

## 🚀 Next Steps

1. **Study the Code**: Examine the JS files to understand SDK integration patterns
2. **Customize**: Modify styling and functionality for your needs
3. **Integrate**: Use these patterns in your own applications with React, Vue, etc.
4. **Scale**: Test with multiple concurrent users and sessions

**Questions?** Check the main SDK documentation at `../README.md` or create an issue in the repository.