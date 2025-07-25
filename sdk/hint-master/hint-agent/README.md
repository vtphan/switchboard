# AI Hint Agent

AI expert agents that provide programming hints using the Switchboard Python SDK and Google's Gemini API.

## Overview

This implementation recreates the original JavaScript hint-master expert using the Switchboard Python SDK, providing:

- **Async Python implementation** using the Switchboard SDK
- **Multiple expert personalities** with different specializations
- **Gemini API integration** for AI-generated hints
- **Robust error handling** and reconnection logic
- **Multi-expert management** for running multiple agents simultaneously

## Features

### Core Functionality
- **Problem Detection**: Listens for instructor problem broadcasts
- **AI Hint Generation**: Uses Gemini API to generate contextual programming hints
- **Expert Personalities**: Different experts with specialized knowledge and response styles
- **Automatic Session Discovery**: Finds and connects to available sessions
- **Graceful Error Handling**: Handles API failures and connection issues

### Expert Types
- **Technical Expert**: Python programming, debugging, best practices
- **Emotional Support Coach**: Learning psychology, motivation, stress reduction
- **Algorithm Specialist**: Algorithms, data structures, computational thinking
- **Web Development Expert**: HTML, CSS, JavaScript, frameworks
- **Caring Instructor**: Teaching-focused approach with Socratic method and deep learning
- **Study Buddy**: Peer-to-peer learning with relatable, casual communication style

## Quick Start

### Prerequisites
1. Python 3.8+
2. Switchboard server running on localhost:8080
3. Google Gemini API key

### Installation
```bash
# Install dependencies
pip install -r requirements.txt

# Install Switchboard Python SDK (if not already installed)
pip install -e ../../python
```

### Configuration
1. Copy and edit a configuration file:
```bash
cp configs/technical-expert.json configs/my-expert.json
```

2. Update the Gemini API key:
```json
{
  "gemini_api_key": "your_actual_gemini_api_key_here"
}
```

### Running Single Expert
```bash
# Start a single expert
python hint_agent.py configs/technical-expert.json
```

### Running Multiple Experts
```bash
# Start all configured experts
python start_experts.py

# Start specific experts
python start_experts.py --experts technical-expert caring-instructor peer-student

# Use custom config directory
python start_experts.py --config-dir /path/to/configs
```

## Configuration

### Expert Configuration Format
```json
{
  "user_id": "unique_expert_id",
  "name": "Expert Display Name",
  "expertise": "Area of Expertise",
  "description": "Brief description of the expert",
  "response_style": "helpful|encouraging|analytical|practical",
  "switchboard_url": "http://localhost:8080",
  "gemini_api_key": "your_gemini_api_key",
  "gemini_model": "gemini-2.0-flash",
  "gemini_temperature": 0.7,
  "gemini_max_output_tokens": 200,
  "prompt_template": "You are {name}, an AI expert specialized in {expertise}. Your response style is {response_style}.\n\nA student is working on this problem:\nProblem: {problem}\n\n{code_context}Student status:\n- Time spent: {time_on_task} minutes\n- Frustration level: {frustration_level}/5 (1=calm, 5=very frustrated)\n\nProvide a helpful hint that guides them toward the solution."
}
```

### Configuration Fields

- **user_id**: Unique identifier for the expert (must be enrolled in sessions)
- **name**: Display name shown to instructors
- **expertise**: Area of specialization (used in prompts)
- **description**: Brief description of expert capabilities
- **response_style**: Tone and approach for responses
- **switchboard_url**: Switchboard server URL
- **gemini_api_key**: Your Google Gemini API key
- **gemini_model**: Gemini model to use (e.g., "gemini-2.0-flash", "gemini-1.5-flash")
- **gemini_temperature**: Creativity level (0.0-2.0)
- **gemini_max_output_tokens**: Maximum tokens in response
- **prompt_template**: Customizable prompt template with placeholders for dynamic content

### Prompt Template Variables

The `prompt_template` field supports the following variables:

- **{name}**: Expert's display name
- **{expertise}**: Expert's area of expertise
- **{response_style}**: Expert's response style
- **{problem}**: The student's problem description
- **{code_context}**: Code context (if provided by student)
- **{time_on_task}**: Minutes the student has spent on the problem
- **{frustration_level}**: Student's frustration level (1-5)

## Usage Flow

1. **Startup**: Expert agent connects to Switchboard as a student
2. **Session Discovery**: Finds sessions where the expert is enrolled
3. **Problem Listening**: Waits for instructor problem broadcasts
4. **Hint Generation**: Uses Gemini API to generate contextual hints
5. **Hint Delivery**: Sends hints back to instructors via `instructor_inbox`

## Message Flow

```
Teacher broadcasts problem → Expert receives broadcast → 
Gemini generates hint → Expert sends hint to instructors
```

### Message Types Handled
- **instructor_broadcast** (context: problem) → Generate and send hint
- **instructor_broadcast** (context: status) → Log status update
- **inbox_response** → Log instructor response
- **system** → Handle connection events

### Message Types Sent
- **instructor_inbox** (context: hint) → Send generated hints
- **instructor_inbox** (context: error) → Send error notifications
- **analytics** (context: connection) → Send connection status

## Implementation Differences from Original

### Improvements
- **SDK Integration**: Uses Switchboard Python SDK instead of raw WebSocket
- **Async/Await**: Modern Python async patterns
- **Type Hints**: Full type safety
- **Configuration-Driven**: All expert behavior configured via JSON files
- **Configurable Prompts**: Custom prompt templates per expert type
- **No Hard-coded Logic**: Agent is free of assumptions and configurations
- **Valid Gemini Parameters**: Only uses supported API parameters
- **Multi-Expert Management**: Run multiple experts simultaneously
- **Configuration Validation**: Validates all required fields before startup

### Code Reduction
- **~550 lines** → **~350 lines** (37% reduction)
- **Built-in reconnection** via SDK
- **Session management** handled by SDK
- **Message parsing** abstracted by SDK
- **No max hint length limits** - let Gemini API control output length
- **Expert-specific prompts** aligned with each expert's profile

## Monitoring and Debugging

### Logs
Each expert provides detailed logging:
- Connection status and session discovery
- Problem processing and hint generation
- Gemini API calls and responses
- Error handling and recovery

### Statistics
Each expert tracks:
- Messages processed
- Hints generated
- Connection uptime
- API call success/failure rates

### Multi-Expert Output
The expert manager shows:
- Process status for each expert
- Failed expert restart attempts
- Aggregate statistics

## Error Handling

### API Errors
- **401/403**: Invalid API key → Notify instructors
- **429**: Rate limiting → Notify instructors  
- **500/503**: Service unavailable → Notify instructors
- **Network errors**: Retry with exponential backoff

### Connection Errors
- **WebSocket disconnection**: Automatic reconnection via SDK
- **Session ended**: Discover and connect to new sessions
- **No sessions available**: Poll every 30 seconds

### Configuration Errors
- **Missing fields**: Validation on startup
- **Invalid JSON**: Clear error messages
- **Placeholder API keys**: Warning with instructions

## Customization

### Adding New Expert Types
1. Create a new configuration file in `configs/`
2. Customize the expert's personality and capabilities
3. Adjust Gemini parameters for the expert's style
4. Test with specific problem types

### Custom Prompt Templates
The `_build_prompt()` method can be customized to:
- Add domain-specific context
- Adjust response formatting
- Include expert-specific instructions
- Handle different problem types

### Response Post-Processing
Hints can be post-processed to:
- Add formatting (code blocks, lists)
- Validate hint quality
- Add expert signature/branding
- Integrate with external APIs

## Expert Response Examples

Different expert types provide distinct approaches to the same problem:

### Problem: "Find the longest palindromic substring"
**Frustration Level: 3/5** | **Time: 8 minutes**

**Technical Expert** (helpful, direct):
```
"For palindrome detection, consider using the expand-around-centers approach. 
Start with each character as a potential center and expand outwards while 
characters match. This gives you O(n²) time complexity with O(1) space."
```

**Caring Instructor** (nurturing, Socratic):
```
"I can see you've been thinking about this for 8 minutes - that shows good 
persistence! Let me ask you: what makes a palindrome special? If you were 
checking by hand, how would you verify if 'racecar' is a palindrome? 
That intuition might guide your algorithm approach."
```

**Study Buddy** (relatable, peer-to-peer):
```
"Ugh, I remember this problem! I spent like 20 minutes trying to reverse 
every substring 😅. Then my classmate showed me this neat trick where you 
treat each letter as the 'middle' of a palindrome and just expand outward. 
Way less confusing than what I was doing!"
```

**Emotional Support Coach** (encouraging, supportive):
```
"I notice you're at a 3/5 frustration level - that's completely normal for 
this type of problem. Take a deep breath. Remember, every expert programmer 
has wrestled with string algorithms. You're not behind; you're exactly 
where you should be in your learning journey."
```

**Algorithm Specialist** (analytical, methodical):
```
"This problem has several algorithmic approaches: brute force O(n³), 
dynamic programming O(n²), or Manacher's algorithm O(n). Given your time 
investment, I'd recommend the expand-around-centers approach as it balances 
efficiency with conceptual clarity."
```

## Integration with Hint-Master System

This Python implementation integrates seamlessly with the existing hint-master architecture:

- **Teacher Client**: Broadcasts problems using JavaScript teacher client
- **Python Experts**: Multiple AI experts provide hints using this implementation
- **Student Interface**: Receives hints through the same WebSocket connections

The experts appear as regular students in the Switchboard system, making them invisible to the underlying infrastructure while providing AI-powered assistance to instructors.