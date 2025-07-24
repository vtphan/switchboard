# Comprehensive Classroom Simulation Testing

## Overview
This test suite provides comprehensive validation of the Switchboard system under realistic classroom conditions. Tests are organized in progressive subphases from basic functionality to complex edge cases and performance validation.

## Test Structure

### Subphases
1. **Foundation** (`foundation_test.go`) - Basic infrastructure and message type validation
2. **Core Workflows** (`core_workflows_test.go`) - Primary classroom interaction patterns  
3. **Advanced Scenarios** (`advanced_scenarios_test.go`) - Complex multi-user interactions
4. **Edge Cases** (`edge_cases_test.go`) - Error conditions and system limits
5. **Performance** (`performance_test.go`) - Load and stress testing

### Test Infrastructure
- **`fixtures/test_helpers.go`** - Database cleanup and test utilities
- **`fixtures/classroom_data.go`** - Realistic test data generation
- **`fixtures/test_client.go`** - WebSocket test client implementation
- **`fixtures/scenario_runner.go`** - Test scenario orchestration

## Message Types Tested

All 6 message types from the technical specification are comprehensively tested:

| Message Type | From | To | Test Contexts |
|--------------|------|----|--------------| 
| `instructor_inbox` | Student | All Instructors | `"question"`, `"help_request"`, `"clarification"`, `"technical_issue"` |
| `inbox_response` | Instructor | Specific Student | `"answer"`, `"guidance"`, `"follow_up"` |
| `request` | Instructor | Specific Student | `"code"`, `"execution_output"`, `"explanation"`, `"screenshot"` |
| `request_response` | Student | All Instructors | `"code_submission"`, `"output_results"`, `"explanation"` |
| `analytics` | Student | All Instructors | `"engagement"`, `"progress"`, `"performance"`, `"errors"` |
| `instructor_broadcast` | Instructor | All Students | `"announcement"`, `"instruction"`, `"emergency"` |

## Running Tests

### Prerequisites
- Switchboard application must be built: `make build`
- Database must be accessible (tests create temporary databases)
- All dependencies installed: `go mod tidy`

### Execution Commands

**Run All Tests:**
```bash
go test ./tests/scenarios/... -v
```

**Run Specific Subphase:**
```bash
go test ./tests/scenarios/ -run TestFoundation -v
go test ./tests/scenarios/ -run TestCoreWorkflows -v
go test ./tests/scenarios/ -run TestAdvancedScenarios -v
go test ./tests/scenarios/ -run TestEdgeCases -v
go test ./tests/scenarios/ -run TestPerformance -v
```

**Run with Race Detection:**
```bash
go test ./tests/scenarios/... -race -v
```

**Run with Coverage:**
```bash
go test ./tests/scenarios/... -cover -coverprofile=classroom_coverage.out -v
```

### Test Database Management

Each test scenario:
1. Creates a temporary SQLite database
2. Initializes the schema
3. Runs the test scenario
4. Cleans up all data (database, connections, registry)
5. Removes temporary files

This ensures complete isolation between tests and prevents data pollution.

## Test Scenarios

### Subphase 1: Foundation (2 hours estimated)
- Database integration and schema validation
- Basic message type sending/receiving for all 6 types
- Role-based permission enforcement
- Single connection scenarios with bidirectional communication

### Subphase 2: Core Workflows (3 hours estimated)
- **Complete Q&A Session**: Question broadcast → Student responses → Instructor answers
- **Code Review Session**: Code requests → Submissions → Follow-up guidance
- **Real-time Analytics**: Student analytics → Instructor monitoring → Instruction adjustment
- **Multi-Context Communication**: All message types with various contexts simultaneously

### Subphase 3: Advanced Scenarios (2.5 hours estimated)
- **Concurrent Multi-Session**: 3 parallel classroom sessions with isolation validation
- **Instructor Collaboration**: Multiple instructors in single session
- **Connection Replacement**: Disconnect/reconnect during active messaging
- **Large Class Session**: 50-student classroom scale testing

### Subphase 4: Edge Cases (2 hours estimated)
- **Rate Limiting**: 100 messages/minute enforcement
- **Invalid Message Handling**: Malformed JSON, oversized content, invalid types
- **Session State Edge Cases**: Ended sessions, unauthorized access, invalid IDs
- **Database Failure Simulation**: Write failures, retry logic, recovery
- **Concurrent Connection Management**: Race conditions, cleanup validation

### Subphase 5: Performance (1.5 hours estimated)
- **Message Throughput**: 1000+ messages/second processing validation
- **Connection Scalability**: Progressive load testing (10, 25, 50 users)
- **Memory and Resource Usage**: Long-running session leak detection

## Success Criteria

### Performance Targets
- **Message Processing**: >1000 messages/second throughput
- **Connection Setup**: <20ms average connection time
- **Message Routing**: <10ms average routing time
- **Session Validation**: <1ms using in-memory cache
- **Resource Usage**: <5.2KB memory per session

### Functional Requirements
- All 6 message types work correctly in all defined contexts
- Role-based permissions enforced (students vs instructors)
- Session isolation maintained under concurrent load
- Error conditions handled gracefully without service disruption
- Resource cleanup prevents memory/connection leaks

### Reliability Requirements
- Database write failures handled with retry logic
- Connection failures don't affect other users
- System shutdown gracefully cleans up all resources
- Rate limiting protects system without breaking functionality

## Troubleshooting

### Common Issues

**Database Lock Errors:**
```bash
# Ensure no other tests are running
pkill -f "go test"
# Clean up any stale database files
rm -f /tmp/switchboard_test_*.db
```

**Port Conflicts:**
```bash
# Tests use dynamic port allocation, but check for conflicts
lsof -i :8080-8090
```

**Memory Issues:**
```bash
# Monitor memory usage during large tests
go test ./tests/scenarios/ -run TestLargeClass -memprofile=mem.prof -v
```

### Debug Logging

Enable detailed logging for test debugging:
```bash
export SWITCHBOARD_LOG_LEVEL=debug
go test ./tests/scenarios/ -run TestSpecificScenario -v
```

## Contributing

When adding new test scenarios:

1. **Follow the existing pattern**: Use `TestSession` for cleanup, realistic data generation
2. **Add comprehensive contexts**: Test all relevant context variations for message types
3. **Include timing validation**: Ensure realistic classroom interaction timing
4. **Validate cleanup**: Verify no resources leak after test completion
5. **Document expectations**: Clear success criteria and failure conditions

## Integration with CI/CD

These tests are designed for:
- **Development validation**: Run before committing changes
- **CI pipeline integration**: Automated validation on pull requests
- **Production readiness**: Comprehensive validation before deployment
- **Performance regression detection**: Baseline performance monitoring

Total estimated execution time: **11 hours** for complete suite (can be parallelized)
Individual subphase execution: **1.5-3 hours** per subphase