# Switchboard JavaScript SDK

Single-file SDKs for teachers and students. Zero dependencies, minimal boilerplate.

## Quick Start

### Teacher SDK

```html
<script src="teacher-sdk.js"></script>
<script>
const teacher = new SwitchboardTeacher('teacher_001');

// Start a session in one line
teacher.startSession('My Class', ['student1', 'student2']).then(() => {
  teacher.welcomeStudents();
});

// Handle student questions
teacher.onStudentQuestion = (data) => {
  console.log(`${data.studentId} asked: ${data.question}`);
  teacher.respond(data.studentId, 'Great question! Here is the answer...');
};
</script>
```

### Student SDK

```html
<script src="student-sdk.js"></script>
<script>
const student = new SwitchboardStudent('student_001');

// Connect - server auto-assigns to active session or lobby!
student.connect();

// Ask questions easily
student.askQuestion('How do I use React hooks?');

// Handle teacher messages
student.onTeacherMessage = (data) => {
  console.log('Teacher said:', data.message);
};

student.onTeacherRequest = (data) => {
  console.log('Teacher wants:', data.request);
  student.respond('Here is my response!');
};

// Handle automatic session assignments
student.onSessionEvent = (data) => {
  if (data.event === 'session_assigned') {
    console.log(`Server assigned me to: ${data.sessionName}`);
  } else if (data.event === 'moved_to_lobby') {
    console.log('Session ended, back in lobby');
  }
};
</script>
```

## Common Teacher Actions

```javascript
// Teaching essentials
teacher.announce('Welcome to class!');
teacher.scheduleBreak(10);
teacher.requestCode('student1', 'Share your component');
teacher.giveFeedback('student1', 'Great work!', 'const example = "code";');
teacher.emergency('Please save your work now!');

// Session management
teacher.stopSession();
```

## Common Student Actions

```javascript
// Learning essentials - work automatically in lobby or assigned session
student.askQuestion('I need help with this error');
student.needHelp('My code is not working', 'const broken = code;');
student.shareCode('const working = "code";', 'This works now!');
student.taskCompleted('Exercise 1');
student.updateProgress(75, 1800, 2, 'React Hooks');

// Check current status
console.log(student.getStatus()); // { connected: true, currentSession: 'lobby' }
```

## Event Handlers

### Teacher Events
```javascript
teacher.onStudentQuestion = (data) => { /* Handle questions */ };
teacher.onStudentResponse = (data) => { /* Handle responses */ };
teacher.onStudentAnalytics = (data) => { /* Handle progress data */ };
teacher.onConnection = (connected) => { /* Connection status */ };
teacher.onPresence = (data) => { /* Student online/offline */ };
```

### Student Events
```javascript
student.onTeacherMessage = (data) => { /* Teacher responses */ };
student.onTeacherRequest = (data) => { /* Teacher requests */ };
student.onTeacherBroadcast = (data) => { /* Announcements */ };
student.onConnection = (connected) => { /* Connection status */ };
```

## Server Configuration

Default: `http://localhost:8080`

Custom server:
```javascript
const teacher = new SwitchboardTeacher('teacher_001', {
  serverUrl: 'https://your-server.com'
});
```

## Usage in Node.js

```javascript
const SwitchboardTeacher = require('./teacher-sdk.js');
const teacher = new SwitchboardTeacher('teacher_001');
```

## Usage in React

```javascript
import SwitchboardTeacher from './teacher-sdk.js';

function TeacherComponent() {
  const [teacher] = useState(() => new SwitchboardTeacher('teacher_001'));
  
  useEffect(() => {
    teacher.onStudentQuestion = (data) => {
      setQuestions(prev => [...prev, data]);
    };
  }, []);
  
  return <div>Teaching Interface</div>;
}
```

That's it! Focus on your teaching/learning logic, not the infrastructure.