# Switchboard Python SDK

A Python client library for connecting to Switchboard real-time educational messaging system. Provides high-level abstractions for both student and instructor clients.

## Installation

```bash
pip install switchboard-sdk
```

Or install from source:

```bash
cd sdk/python
pip install -e .
```

## Quick Start

### Student Client

```python
import asyncio
from switchboard_sdk import SwitchboardStudent

async def student_example():
    # Create student client
    student = SwitchboardStudent("student_001")
    
    # Connect to available session
    session = await student.connect_to_available_session()
    if not session:
        print("No available sessions found")
        return
    
    print(f"Connected to session: {session.name}")
    
    # Set up message handlers
    @student.on_instructor_response
    async def handle_response(message):
        print(f"Response from {message.from_user}: {message.content['text']}")
    
    @student.on_instructor_request  
    async def handle_request(message):
        print(f"Request from {message.from_user}: {message.content['text']}")
        # Respond to the request
        await student.respond_to_request({
            "text": "Here is my response",
            "data": "student work here"
        })
    
    # Ask a question
    await student.ask_question({
        "text": "I need help with React hooks",
        "code_context": "const [state, setState] = useState()",
        "urgency": "medium"
    })
    
    # Report progress
    await student.report_progress(
        completion_percentage=75,
        time_spent_minutes=30,
        current_topic="React Hooks"
    )
    
    # Keep connection alive
    try:
        await asyncio.sleep(3600)  # Run for 1 hour
    finally:
        await student.disconnect()

# Run the example
asyncio.run(student_example())
```

### Teacher Client

```python
import asyncio
from switchboard_sdk import SwitchboardTeacher

async def teacher_example():
    # Create teacher client  
    teacher = SwitchboardTeacher("teacher_001")
    
    # Create a new session
    session = await teacher.create_session(
        name="Python Workshop",
        student_ids=["student_001", "student_002", "student_003"]
    )
    
    # Connect to the session
    await teacher.connect(session.id)
    print(f"Created and connected to session: {session.name}")
    
    # Set up message handlers
    @teacher.on_student_question
    async def handle_question(message):
        print(f"Question from {message.from_user}: {message.content['text']}")
        
        # Respond to the student
        await teacher.respond_to_student(
            student_id=message.from_user,
            content={
                "text": "Great question! Here's the answer...",
                "code_example": "example_code_here"
            }
        )
    
    @teacher.on_student_analytics
    async def handle_analytics(message):
        print(f"Analytics from {message.from_user}: {message.content}")
    
    # Make an announcement
    await teacher.announce("Welcome to the Python Workshop!")
    
    # Request code from a specific student
    await teacher.request_code_from_student(
        student_id="student_001",
        prompt="Please share your current function implementation",
        requirements=["Include comments", "Handle edge cases"]
    )
    
    # Broadcast a problem for AI tutoring systems
    await teacher.broadcast_problem(
        problem="Students are struggling with list comprehensions",
        code="[x for x in range(10) if x % 2 == 0]",
        frustration_level=3
    )
    
    # Keep session running
    try:
        await asyncio.sleep(3600)  # Run for 1 hour
    finally:
        await teacher.end_current_session()

# Run the example
asyncio.run(teacher_example())
```

## AI Expert Bot Example

```python
import asyncio
from switchboard_sdk import SwitchboardStudent

class AIExpertBot(SwitchboardStudent):
    def __init__(self, expert_config):
        super().__init__(expert_config["user_id"])
        self.expert_name = expert_config["name"]
        self.expertise = expert_config["expertise"]
        
        # Set up message handlers
        self.on_instructor_broadcast(self.handle_problem_broadcast)
    
    async def handle_problem_broadcast(self, message):
        if message.context == "problem":
            # Generate hint using AI service
            hint = await self.generate_hint(message.content)
            
            # Send hint back to instructors
            await self.ask_question({
                "hint": hint,
                "expert": {
                    "name": self.expert_name,
                    "expertise": self.expertise
                },
                "problem_context": message.content
            }, context="hint")
    
    async def generate_hint(self, problem_data):
        # Integrate with your AI service here
        # This is where you'd call OpenAI, Anthropic, etc.
        return f"Hint for: {problem_data['problem']}"

# Usage
async def run_expert_bot():
    config = {
        "user_id": "technical_expert",
        "name": "Technical Expert",
        "expertise": "Python, JavaScript, React"
    }
    
    bot = AIExpertBot(config)
    session = await bot.connect_to_available_session()
    
    if session:
        print(f"Expert bot connected to {session.name}")
        await asyncio.sleep(3600)  # Keep running
    else:
        print("No sessions available")

asyncio.run(run_expert_bot())
```

## Features

### Student Client Features

- **Session Discovery**: Automatically find sessions you're enrolled in
- **Message Types**: Send questions, responses, and analytics
- **Convenience Methods**: 
  - `ask_question()` - Send questions to instructors
  - `respond_to_request()` - Respond to instructor requests
  - `report_progress()` - Send progress updates
  - `report_engagement()` - Send engagement metrics
  - `report_error()` - Report errors or problems

### Teacher Client Features

- **Session Management**: Create, connect to, and end sessions
- **Message Types**: Send responses, requests, and broadcasts
- **Convenience Methods**:
  - `announce()` - Make announcements
  - `request_code_from_student()` - Request code submissions
  - `provide_feedback()` - Give student feedback
  - `broadcast_problem()` - Send problems to AI tutoring systems

### Common Features

- **Automatic Reconnection**: Built-in reconnection with exponential backoff
- **Event Handlers**: Easy message handling with decorators
- **Type Safety**: Full type hints and dataclass models
- **Error Handling**: Comprehensive exception hierarchy
- **Connection Management**: Graceful connection and disconnection
- **Context Manager Support**: Use with `async with` statements

## API Reference

### SwitchboardStudent

#### Methods

- `find_available_sessions()` → `List[Session]`
- `connect_to_available_session()` → `Optional[Session]`
- `ask_question(content, context="question")`
- `respond_to_request(content, context="response")`
- `send_analytics(content, context="progress")`
- `report_progress(completion_percentage, time_spent_minutes, current_topic, ...)`
- `report_engagement(attention_level, confusion_level, participation_score)`
- `report_error(error_type, error_message, ...)`
- `request_help(topic, description, urgency="medium", ...)`

#### Event Handlers

- `@student.on_instructor_response`
- `@student.on_instructor_request`
- `@student.on_instructor_broadcast`
- `@student.on_system_message`

### SwitchboardTeacher

#### Methods

- `create_session(name, student_ids)` → `Session`
- `end_session(session_id)`
- `list_active_sessions()` → `List[Session]`
- `respond_to_student(student_id, content, context="answer")`
- `request_from_student(student_id, content, context="request")`
- `broadcast_to_students(content, context="announcement")`
- `announce(text, **kwargs)`
- `request_code_from_student(student_id, prompt, requirements=None, deadline=None)`
- `provide_feedback(student_id, feedback, code_example=None, resources=None)`
- `broadcast_problem(problem, code="", time_on_task=0, ...)`

#### Event Handlers

- `@teacher.on_student_question`
- `@teacher.on_student_response` 
- `@teacher.on_student_analytics`
- `@teacher.on_system_message`

## Error Handling

The SDK provides a comprehensive exception hierarchy:

```python
from switchboard_sdk import (
    SwitchboardError,         # Base exception
    ConnectionError,          # WebSocket connection issues
    AuthenticationError,      # Not enrolled in session
    SessionNotFoundError,     # Session doesn't exist
    MessageValidationError,   # Invalid message format  
    RateLimitError,          # Rate limit exceeded
    SessionEndedError,       # Session ended unexpectedly
    ReconnectionFailedError  # Auto-reconnection failed
)

try:
    await student.connect(session_id)
except AuthenticationError:
    print("Not enrolled in this session")
except ConnectionError as e:
    print(f"Connection failed: {e}")
```

## Configuration

The SDK supports various configuration options:

```python
# Custom server URL
client = SwitchboardStudent(
    user_id="student_001",
    server_url="https://my-switchboard.com"
)

# Reconnection settings
client = SwitchboardStudent(
    user_id="student_001", 
    max_reconnect_attempts=10,
    reconnect_delay=2.0  # Initial delay, increases exponentially
)
```

## Requirements

- Python 3.8+
- aiohttp>=3.8.0
- websockets>=11.0.0

## Development

```bash
# Install development dependencies
pip install -e ".[dev]"

# Run tests
pytest

# Format code
black switchboard_sdk/

# Type checking
mypy switchboard_sdk/
```

## License

MIT License - see LICENSE file for details.