# Switchboard User Journeys

This document outlines the key user journeys for teachers and students using the Switchboard real-time educational communication system. These journeys reflect the implemented single-session architecture with auto-assignment and lobby system.

## Table of Contents

- [Overview](#overview)
- [Teacher User Journeys](#teacher-user-journeys)
- [Student User Journeys](#student-user-journeys)
- [System Messages and Events](#system-messages-and-events)
- [Error Scenarios](#error-scenarios)
- [Advanced Workflows](#advanced-workflows)

## Overview

### Key System Concepts

- **Single Session Architecture**: Only one session can be active at any time
- **Auto-Assignment**: Users automatically assigned to appropriate session or lobby
- **Lobby System**: Persistent connections for users not in active sessions
- **Message Types**: 6 distinct communication patterns with role-based permissions
- **Real-Time Communication**: WebSocket-based with <1ms message routing

### Connection States

```
LOBBY STATE (Default)
├─ All users start here
├─ Persistent presence awareness
├─ Session notifications
└─ Auto-transition when sessions become available

SESSION STATE (Active Participation)
├─ Full message communication
├─ History replay on join
├─ Role-based message permissions
└─ Return to lobby when session ends
```

---

## Teacher User Journeys

### Journey 1: Starting a New Class Session

**Context**: Teacher wants to begin a new lesson with enrolled students.

#### Steps:
1. **Connection Setup**
   ```
   Teacher connects: ws://host/ws?user_id=prof_smith&role=instructor
   → Auto-assigned to lobby (no active session)
   → Receives presence_update events for all connected users
   ```

2. **Session Creation Attempt**
   ```
   POST /api/sessions
   {
     "name": "Mathematics - Chapter 5: Calculus",
     "instructor_id": "prof_smith", 
     "student_ids": ["alice", "bob", "charlie", "diana"]
   }
   ```

3. **Successful Creation**
   ```
   HTTP 201 Created
   {
     "session": {
       "id": "session-abc123",
       "name": "Mathematics - Chapter 5: Calculus",
       "created_by": "prof_smith",
       "student_ids": ["alice", "bob", "charlie", "diana"],
       "status": "active",
       "start_time": "2025-01-28T10:00:00Z"
     }
   }
   ```

4. **Auto-Transition and Notifications**
   ```
   → Teacher automatically transitioned from lobby to session
   → Connected students (alice, bob) auto-transitioned to session
   → Offline students (charlie, diana) will auto-join when they connect
   → All users receive session_started system message
   ```

5. **Session Ready State**
   ```
   → Teacher can now send all message types
   → Students receive message history (empty for new session)
   → Real-time communication begins
   ```

**Expected Outcome**: Active teaching session with automatic student assignment and real-time communication ready.

---

### Journey 2: Managing Multiple Session Attempts (Conflict Resolution)

**Context**: Teacher tries to create a session while another is already active.

#### Steps:
1. **Conflict Detection**
   ```
   POST /api/sessions (while "Previous Class" is active)
   
   HTTP 409 Conflict
   {
     "error": "Active session exists",
     "message": "Cannot create new session. Active session 'Previous Class' must be ended first.",
     "active_session": {
       "id": "session-xyz789",
       "name": "Previous Class", 
       "created_by": "prof_jones",
       "status": "active"
     }
   }
   ```

2. **Resolution Workflow**
   ```
   Option A: End existing session first
   DELETE /api/sessions/session-xyz789
   → HTTP 200 OK
   → All users in session return to lobby
   → session_left messages sent to participants
   
   Option B: Wait for existing session to end
   → Monitor session status via GET /api/sessions
   → Proceed when no active sessions exist
   ```

3. **Retry Session Creation**
   ```
   POST /api/sessions (after conflict resolved)
   → HTTP 201 Created
   → Normal session creation workflow proceeds
   ```

**Expected Outcome**: Clear guidance for resolving session conflicts with helpful error messages.

---

### Journey 3: Conducting Interactive Teaching

**Context**: Teacher conducting live instruction with real-time student interaction.

#### Steps:
1. **Class Announcement**
   ```
   Send instructor_broadcast message:
   {
     "type": "instructor_broadcast",
     "context": "announcement",
     "content": {
       "message": "Welcome to today's calculus lesson. We'll cover derivatives.",
       "urgency": "normal"
     }
   }
   → Delivered to all students in session (alice, bob, charlie, diana)
   ```

2. **Individual Student Request**
   ```
   Send request message to specific student:
   {
     "type": "request",
     "context": "code_review",
     "to_user": "alice",
     "content": {
       "task": "Please share your derivative calculation for problem #3",
       "deadline": "5 minutes"
     }
   }
   → Delivered only to alice
   ```

3. **Receiving Student Questions**
   ```
   Receive instructor_inbox message from bob:
   {
     "type": "instructor_inbox", 
     "context": "question",
     "from_user": "bob",
     "content": {
       "question": "I'm confused about the chain rule. Can you explain?",
       "priority": "medium"
     }
   }
   → Received by all instructors in session
   ```

4. **Providing Individual Help**
   ```
   Send inbox_response to bob:
   {
     "type": "inbox_response",
     "context": "explanation", 
     "to_user": "bob",
     "content": {
       "answer": "The chain rule applies when you have a function inside another function...",
       "resources": ["textbook_page_45", "online_tutorial_link"]
     }
   }
   → Delivered only to bob
   ```

5. **Monitoring Student Analytics**
   ```
   Receive analytics messages from students:
   {
     "type": "analytics",
     "from_user": "charlie",
     "content": {
       "event": "problem_completed",
       "problem_id": "derivative_1",
       "time_spent": 180,
       "attempts": 2,
       "accuracy": 0.85
     }
   }
   → Helps teacher track student progress
   ```

**Expected Outcome**: Fluid real-time teaching with multiple communication patterns for different instructional needs.

---

### Journey 4: Ending Class Session

**Context**: Teacher concludes the lesson and ends the session.

#### Steps:
1. **Final Class Message**
   ```
   Send instructor_broadcast:
   {
     "type": "instructor_broadcast",
     "context": "announcement",
     "content": {
       "message": "Great work today! Homework is posted. See you next class.",
       "action_required": "check_homework"
     }
   }
   ```

2. **Session Termination**
   ```
   DELETE /api/sessions/session-abc123
   
   HTTP 200 OK
   {
     "session_id": "session-abc123",
     "status": "ended",
     "ended_at": "2025-01-28T11:00:00Z"
   }
   ```

3. **Automatic User Transitions**
   ```
   → All session participants receive session_left message
   → Students automatically return to lobby state (maintain connection)
   → Teacher returns to lobby state
   → Session marked as ended in database
   ```

4. **Post-Session State**
   ```
   → All users remain connected in lobby
   → Presence updates continue
   → Users ready for next session
   → Message history preserved for ended session
   ```

**Expected Outcome**: Clean session termination with seamless transition to lobby state for all participants.

---

## Student User Journeys

### Journey 1: Joining Class (Auto-Assignment)

**Context**: Student connects and is automatically assigned to their enrolled session.

#### Steps:
1. **Initial Connection**
   ```
   Student connects: ws://host/ws?user_id=alice&role=student
   → Auto-assignment logic evaluates enrollment
   → Alice enrolled in active session "Mathematics - Chapter 5"
   → Automatically assigned to session (not lobby)
   ```

2. **Session Integration**
   ```
   → Receives complete message history for session
   → Gets current session state and participant list
   → Can immediately participate in ongoing lesson
   ```

3. **Participation Ready**
   ```
   → Can send student-authorized message types:
     - instructor_inbox (questions to all instructors)
     - request_response (responses to teacher requests)
     - analytics (learning data to instructors)
   → Receives instructor broadcasts and direct messages
   ```

**Expected Outcome**: Seamless entry into active learning session without manual session discovery.

---

### Journey 2: Waiting in Lobby (No Active Session)

**Context**: Student connects when no session is active or they're not enrolled in current session.

#### Steps:
1. **Lobby Assignment**
   ```
   Student connects: ws://host/ws?user_id=eve&role=student
   → No active session OR not enrolled in active session
   → Automatically assigned to lobby
   → Maintains persistent connection
   ```

2. **Lobby Experience**
   ```
   → Receives presence_update events for all connected users
   → Sees who's online (students and instructors)
   → Waits for session notifications
   → Cannot send session-specific messages
   ```

3. **Session Notification**
   ```
   When session created with eve in student_ids:
   → Receives session_started system message:
   {
     "type": "system",
     "content": {
       "event": "session_started",
       "session_id": "session-def456",
       "session_name": "Biology Lab",
       "student_ids": ["eve", "frank", "grace"]
     }
   }
   ```

4. **Auto-Transition to Session**
   ```
   → Automatically transitioned from lobby to session
   → Receives session history and current state
   → Can immediately participate in lesson
   ```

**Expected Outcome**: Persistent connection with automatic session joining when available.

---

### Journey 3: Active Learning Participation

**Context**: Student actively participating in a live teaching session.

#### Steps:
1. **Receiving Instruction**
   ```
   Receive instructor_broadcast from teacher:
   {
     "type": "instructor_broadcast",
     "context": "instruction",
     "from_user": "prof_smith",
     "content": {
       "topic": "Photosynthesis Process",
       "instruction": "Open your lab materials and prepare for the experiment"
     }
   }
   ```

2. **Asking Questions**
   ```
   Send instructor_inbox message:
   {
     "type": "instructor_inbox",
     "context": "question",
     "content": {
       "question": "Should we use distilled water or tap water for this experiment?",
       "urgency": "high"
     }
   }
   → Delivered to all instructors in session
   ```

3. **Responding to Teacher Requests**
   ```
   Receive request from teacher:
   {
     "type": "request",
     "context": "lab_data",
     "from_user": "prof_smith",
     "content": {
       "request": "Please share your pH readings from test tube A"
     }
   }
   
   Send request_response:
   {
     "type": "request_response", 
     "context": "lab_data",
     "content": {
       "ph_reading": 7.2,
       "test_tube": "A",
       "timestamp": "2025-01-28T10:30:00Z"
     }
   }
   → Delivered to all instructors
   ```

4. **Automatic Analytics Sharing**
   ```
   Send analytics message (automated by client):
   {
     "type": "analytics",
     "context": "engagement",
     "content": {
       "event": "experiment_step_completed",
       "step": "ph_measurement",
       "time_spent": 120,
       "success": true
     }
   }
   → Helps instructor track learning progress
   ```

**Expected Outcome**: Rich interactive learning experience with multiple communication channels.

---

### Journey 4: Session Transition (Class Ends)

**Context**: Student experiences session ending and transition back to lobby.

#### Steps:
1. **Session End Notification**
   ```
   Receive session_left system message:
   {
     "type": "system",
     "content": {
       "event": "session_left",
       "session_id": "session-def456",
       "reason": "Session ended by instructor"
     }
   }
   ```

2. **Automatic Lobby Return**
   ```
   → Connection maintained (no disconnection)
   → Automatically returned to lobby state
   → Can no longer send session-specific messages
   → Continues receiving presence updates
   ```

3. **Post-Session State**
   ```
   → Remains connected and ready for next session
   → Sees other students and instructors in lobby
   → Waits for next session notification
   → Previous session history preserved
   ```

**Expected Outcome**: Seamless transition maintaining connection readiness for future sessions.

---

### Journey 5: Connection Recovery (Reconnection)

**Context**: Student reconnects after network interruption or client restart.

#### Steps:
1. **Reconnection with Auto-Assignment**
   ```
   Student reconnects: ws://host/ws?user_id=alice&role=student
   → Auto-assignment checks current session state
   → If active session exists and alice enrolled → rejoin session
   → If no active session → assign to lobby
   ```

2. **Session State Recovery**
   ```
   If rejoining active session:
   → Receives complete message history since session start
   → Gets current session participants
   → Seamlessly resumes participation
   
   If joining lobby:
   → Receives current presence information
   → Waits for session notifications
   ```

3. **Connection Replacement Handling**
   ```
   If alice already connected from another device:
   → Previous connection receives connection_replaced message
   → Previous connection gracefully closes
   → New connection becomes active
   → No service interruption
   ```

**Expected Outcome**: Robust reconnection with automatic state recovery and connection management.

---

## System Messages and Events

### Presence Updates
```json
{
  "type": "system",
  "content": {
    "event": "presence_update",
    "user_id": "alice",
    "role": "student", 
    "session_id": "session-abc123"  // or "lobby" or null for disconnect
  },
  "timestamp": "2025-01-28T10:00:00Z"
}
```

### Session Events
```json
{
  "type": "system",
  "content": {
    "event": "session_started",
    "session_id": "session-abc123",
    "session_name": "Mathematics - Chapter 5",
    "student_ids": ["alice", "bob", "charlie"]
  },
  "timestamp": "2025-01-28T10:00:00Z" 
}

{
  "type": "system",
  "content": {
    "event": "session_left",
    "session_id": "session-abc123", 
    "reason": "Session ended by instructor"
  },
  "timestamp": "2025-01-28T11:00:00Z"
}
```

### Connection Events
```json
{
  "type": "system",
  "content": {
    "event": "connection_replaced",
    "reason": "New connection established"
  },
  "timestamp": "2025-01-28T10:05:00Z"
}
```

---

## Error Scenarios

### Rate Limiting
**Scenario**: Student sends too many messages too quickly.

```
→ Messages beyond 100/minute dropped
→ Client continues to function normally
→ No disconnection occurs
→ Rate limit resets each minute
```

### Invalid Message Types
**Scenario**: Student attempts to send instructor-only message type.

```
→ Message rejected with validation error
→ Connection remains active
→ Client should handle gracefully
→ Error logged server-side
```

### Session Not Found
**Scenario**: User tries to connect to non-existent session.

```
→ WebSocket connection closed with error
→ Client receives 404 error
→ Should retry with auto-assignment
→ Fallback to lobby connection
```

### Network Interruption
**Scenario**: Client loses network connectivity.

```
→ WebSocket connection times out (120s)
→ Server cleans up connection resources
→ Other users notified of disconnection
→ Client should reconnect automatically
```

---

## Advanced Workflows

### Multi-Device Access
**Teacher with laptop and tablet**:
1. Connect from laptop → Active connection established
2. Connect from tablet → Laptop connection replaced gracefully  
3. Laptop receives connection_replaced message
4. Tablet becomes active connection
5. No service interruption

### Late Arrival to Session
**Student joins mid-session**:
1. Connect with auto-assignment
2. Receive complete session history from start
3. See current session state and participants
4. Immediately participate in ongoing activities
5. No context loss despite late arrival

### Instructor Handoff
**Teaching assistant takes over**:
1. Original instructor ends session
2. All participants return to lobby
3. TA creates new session with same students
4. Students auto-transition to new session
5. Continuous teaching experience maintained

### Emergency Session Termination
**System maintenance or emergency**:
1. Admin triggers session end via API
2. All participants receive session_left message
3. Users return to lobby automatically
4. System can be restarted without data loss
5. Sessions can be recreated after maintenance

### Batch Student Operations
**Mass enrollment changes**:
1. Teacher creates session with initial student list
2. Some students auto-transition from lobby
3. Additional students added via session update (if implemented)
4. New students auto-join when they connect
5. Removed students transition back to lobby

---

## Performance Expectations

### Response Times
- **Auto-assignment**: < 10ms for 95% of connections
- **Message routing**: < 1ms average latency
- **Session creation**: < 100ms including database write
- **History replay**: < 200ms for typical session history
- **Presence updates**: < 50ms broadcast to all users

### Reliability Targets
- **Connection success rate**: > 99%
- **Message delivery**: > 99.9% within session
- **Auto-assignment accuracy**: 100% based on enrollment
- **Session state consistency**: 100% across all participants
- **Resource cleanup**: 100% on disconnection

### Scalability Limits
- **Concurrent users per session**: 50+ students + instructors
- **Messages per minute**: 5,000+ (classroom scale)
- **Session history size**: Limited by client memory and network
- **Presence updates**: Real-time for up to 100 concurrent users
- **Database performance**: Batched writes for optimal throughput

---

This comprehensive guide covers the major user journeys and system behaviors for both teachers and students using Switchboard. Each journey reflects the implemented single-session architecture with auto-assignment, providing clear expectations for user experience and system behavior.