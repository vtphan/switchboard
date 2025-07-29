# Revised SDK Usage Examples - Auto-Assignment Architecture

## Overview

The SDKs have been updated to align with the single-session architecture where:
- **Students** connect without specifying a session and are auto-assigned to the active session (if enrolled) or lobby
- **Teachers** connect without specifying a session and are auto-assigned to the active session (if exists) or lobby
- **Lobby** is the default state when no active session exists or student is not enrolled

## JavaScript/TypeScript SDK

### Student Usage

```typescript
import { SwitchboardStudent } from '@switchboard/sdk';

// Create student client
const student = new SwitchboardStudent('student123', {
  serverUrl: 'http://localhost:8080'
});

// Simple connection - server decides where to place student
await student.connect();

// Check where we ended up
if (student.isInSession()) {
  console.log(`Joined session: ${student.getSessionId()}`);
  
  // Ask question to instructors
  await student.askQuestion({
    text: "How do I solve this problem?",
    topic: "arrays"
  });
  
} else if (student.isInLobby()) {
  console.log("Waiting in lobby for session to start");
}

// Set up event handlers
student.setupEventHandlers({
  onInstructorResponse: (message) => {
    console.log('Instructor answered:', message.content);
  },
  onInstructorBroadcast: (message) => {
    console.log('Announcement:', message.content);
  },
  onSystemMessage: (message) => {
    if (message.content.event === 'presence_update') {
      console.log('Someone joined/left');
    }
  }
});

// Report progress analytics
await student.reportProgress({
  completionPercentage: 75,
  timeSpentMinutes: 45,
  currentTopic: "Data Structures",
  exercisesCompleted: 8,
  exercisesTotal: 10
});
```

### Teacher Usage

```typescript
import { SwitchboardTeacher } from '@switchboard/sdk';

// Create teacher client
const teacher = new SwitchboardTeacher('instructor456', {
  serverUrl: 'http://localhost:8080'
});

// Connect - will join active session if exists, otherwise lobby
await teacher.connect();

if (teacher.isInLobby()) {
  console.log("No active session - creating one");
  
  // Create new session
  const session = await teacher.createAndConnect(
    "JavaScript Basics",
    ["student123", "student456", "student789"]
  );
  
  console.log(`Created session: ${session.id}`);
  
  // Students will auto-join when they connect
  await teacher.announce("Welcome to class! Let's begin.");
}

// Set up event handlers
teacher.setupEventHandlers({
  onStudentQuestion: async (message) => {
    console.log(`Question from ${message.from_user}:`, message.content);
    
    // Respond to the student
    await teacher.respondToStudent(
      message.from_user,
      { text: "Great question! Here's the answer..." },
      "answer"
    );
  },
  onStudentAnalytics: (message) => {
    console.log(`Analytics from ${message.from_user}:`, message.content);
  }
});

// Broadcast to all students
await teacher.giveInstruction("Please open exercise 3");

// Request code from specific student
await teacher.requestCodeFromStudent({
  studentId: "student123",
  prompt: "Please share your solution",
  requirements: ["Include comments", "Handle edge cases"]
});

// End session
await teacher.endCurrentSession();
console.log("Session ended, back in lobby");
```

## Python SDK

### Student Usage

```python
import asyncio
from switchboard_sdk import SwitchboardStudent

async def student_example():
    # Create student client
    student = SwitchboardStudent('student123', server_url='http://localhost:8080')
    
    # Simple connection - server decides where to place student
    await student.connect()
    
    # Check where we ended up
    if student.is_in_session():
        print(f"Joined session: {student.get_session_id()}")
        
        # Ask question to instructors
        await student.ask_question({
            "text": "How do I solve this problem?",
            "topic": "arrays"
        })
        
    elif student.is_in_lobby():
        print("Waiting in lobby for session to start")
    
    # Set up event handlers
    student.on_instructor_response(lambda msg: print(f"Instructor answered: {msg.content}"))
    student.on_instructor_broadcast(lambda msg: print(f"Announcement: {msg.content}"))
    
    # Report progress analytics
    await student.report_progress(
        completion_percentage=75,
        time_spent_minutes=45,
        current_topic="Data Structures",
        exercises_completed=8,
        exercises_total=10
    )
    
    # Keep connection alive
    await asyncio.sleep(10)
    await student.disconnect()

asyncio.run(student_example())
```

### Teacher Usage

```python
import asyncio
from switchboard_sdk import SwitchboardTeacher

async def teacher_example():
    # Create teacher client
    teacher = SwitchboardTeacher('instructor456', server_url='http://localhost:8080')
    
    # Connect - will join active session if exists, otherwise lobby
    await teacher.connect()
    
    if teacher.is_in_lobby():
        print("No active session - creating one")
        
        # Create new session
        session = await teacher.create_and_connect(
            "Python Basics",
            ["student123", "student456", "student789"]
        )
        
        print(f"Created session: {session.id}")
        
        # Students will auto-join when they connect
        await teacher.announce("Welcome to class! Let's begin.")
    
    # Set up event handlers
    async def handle_question(message):
        print(f"Question from {message.from_user}: {message.content}")
        
        # Respond to the student
        await teacher.respond_to_student(
            message.from_user,
            {"text": "Great question! Here's the answer..."},
            "answer"
        )
    
    teacher.on_student_question(handle_question)
    teacher.on_student_analytics(lambda msg: print(f"Analytics: {msg.content}"))
    
    # Broadcast to all students
    await teacher.give_instruction("Please open exercise 3")
    
    # Request code from specific student
    await teacher.request_code_from_student(
        student_id="student123",
        prompt="Please share your solution",
        requirements=["Include comments", "Handle edge cases"]
    )
    
    # Keep session running
    await asyncio.sleep(10)
    
    # End session
    await teacher.end_current_session()
    print("Session ended, back in lobby")
    
    await teacher.disconnect()

asyncio.run(teacher_example())
```

## Key Changes from Previous Version

### What Changed:
1. **No session discovery** - Students don't search for sessions
2. **Auto-assignment is default** - Just call `connect()` without parameters
3. **Single active session** - Methods changed from plural to singular (`getActiveSession()` not `listActiveSessions()`)
4. **Lobby is automatic** - No explicit lobby connection needed
5. **Simplified flow** - Connect → Check state → Start working

### Migration Guide:

**Old way:**
```typescript
// DON'T do this anymore
const sessions = await student.findAvailableSessions();
await student.connect(sessions[0].id);
```

**New way:**
```typescript
// DO this instead
await student.connect();
if (student.isInSession()) {
  // Ready to work
}
```

This architecture is much simpler and matches the server's auto-assignment behavior perfectly!