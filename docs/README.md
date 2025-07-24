# Switchboard - Real-Time Educational Communication System

A real-time communication component that facilitates structured communication between students and instructors within session-based contexts. Built with Go using WebSocket technology for immediate message delivery.

## Overview

Switchboard supports various communication patterns including:
- **instructor_inbox** - Student questions/messages to all instructors
- **inbox_response** - Instructor responses to specific students  
- **request** - Instructor requests for information from students
- **request_response** - Student responses to instructor requests
- **analytics** - Student analytics/activity data to instructors
- **instructor_broadcast** - Instructor announcements to all students

## Architecture

- **Session-Centric**: All communication occurs within defined educational sessions
- **Real-Time**: WebSocket-based for immediate message delivery
- **Persistent**: All messages stored for history replay and audit
- **Go Concurrency**: Leverages goroutines and channels for scalable concurrent processing
- **Immutable Sessions**: Sessions cannot be modified after creation

## Key Features

- WebSocket-based real-time communication
- Session-based message routing with role validation
- Complete message history with replay capability
- Rate limiting (100 messages per minute per client)
- SQLite persistence with concurrent access safety
- Health monitoring and metrics
- Comprehensive validation and testing framework

## Directory Structure

```
switchboard/
├── cmd/server/                 # Main application entry point
├── internal/
│   ├── api/                   # HTTP API handlers (sessions, health)
│   ├── websocket/             # WebSocket hub and client management
│   ├── database/              # Database operations and models
│   ├── session/               # Session management logic
│   └── config/                # Configuration management
├── pkg/
│   ├── models/                # Shared data structures
│   └── interfaces/            # Interface definitions
├── migrations/                # SQL schema files
├── planning/                  # Implementation phases and discoveries
└── tests/                     # Integration and load tests
```

## Quick Start

### Prerequisites
- Go 1.24.5 or later
- SQLite3

### Installation

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd switchboard
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Install development tools:
   ```bash
   make install-tools
   ```

4. Set up database:
   ```bash
   make migrate-up
   ```

5. Build and run:
   ```bash
   make build
   make run
   ```

## Development

### Build Commands
- `make build` - Build the application
- `make run` - Build and run the application  
- `make dev` - Run in development mode

### Testing Commands
- `make test` - Run all tests
- `make test-race` - Run tests with race detection
- `make coverage` - Generate test coverage report
- `make benchmark` - Run performance benchmarks
- `make load-test` - Run load tests for real-time components

### Validation Commands (Required for TDD)
- `make validate` - Run all validation checks
- `make lint` - Run static analysis
- `make security` - Run security analysis
- `make vulnerability` - Check for vulnerabilities

### Leak Detection
- `make leak-test` - Check for memory leaks
- `make goroutine-test` - Check for goroutine leaks

## API Endpoints

### Session Management
- `POST /api/sessions` - Create new session
- `DELETE /api/sessions/{id}` - End session
- `GET /api/sessions/{id}` - Get session details
- `GET /api/sessions` - List active sessions

### WebSocket Connection
- `ws://localhost:8080/ws?user_id={id}&role={role}&session_id={session_id}`

### Health Check
- `GET /health` - System health status

## Message Types and Routing

All message types are available in every session:

| Type | From | To | Description |
|------|------|----|-----------| 
| `instructor_inbox` | Student | All Instructors | Student questions/messages |
| `inbox_response` | Instructor | Specific Student | Response to student message |
| `request` | Instructor | Specific Student | Request for information |
| `request_response` | Student | All Instructors | Response to instructor request |
| `analytics` | Student | All Instructors | Analytics/activity data |
| `instructor_broadcast` | Instructor | All Students | Announcements/instructions |

## Implementation Methodology

This project uses validation-driven TDD with specialized validation for real-time systems:

- **Architectural Validation**: Dependencies, interface compliance
- **Functional Validation**: TDD with 85%+ coverage
- **Real-Time Validation**: Race detection, resource cleanup, performance
- **Quality Validation**: Static analysis, documentation

## Security

- Role-based message routing with session membership validation
- Rate limiting (100 messages per minute per client)
- Input validation and size limits (64KB max message content)
- Complete audit trail of all communications
- Session isolation preventing cross-session data leakage

## Performance Targets

- <1ms message routing
- <100ms connection handling
- Support for classroom-scale concurrent users (20-50 per session)
- Resource leak prevention with automatic cleanup

## License

[Add license information]

## Development Status

This project is under active development using a phase-based implementation approach. See the `planning/` directory for detailed implementation phases and progress tracking.