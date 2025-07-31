#!/bin/bash

# Switchboard V4 Test Runner
# Provides convenient commands for running different test suites

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function definitions
run_unit_tests() {
    print_status "Running unit tests..."
    go test -v ./internal/... ./pkg/... ./web/...
    print_success "Unit tests completed"
}

run_integration_tests() {
    print_status "Running integration tests..."
    go test -v ./tests/integration/...
    print_success "Integration tests completed"
}

run_workflow_tests() {
    print_status "Running workflow tests..."
    go test -v ./tests/workflows/...
    print_success "Workflow tests completed"
}

run_edge_case_tests() {
    print_status "Running edge case tests..."
    go test -v ./tests/edge_cases/...
    print_success "Edge case tests completed"
}

run_performance_tests() {
    print_status "Running performance tests (this may take several minutes)..."
    go test -v ./tests/performance/... -timeout=10m
    print_success "Performance tests completed"
}

run_all_tests() {
    print_status "Running complete test suite..."
    echo "This will run all tests including performance tests and may take 15+ minutes"
    read -p "Continue? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        run_unit_tests
        run_integration_tests
        run_workflow_tests
        run_edge_case_tests
        run_performance_tests
        print_success "All tests completed successfully!"
    else
        print_warning "Test run cancelled"
        exit 0
    fi
}

run_quick_tests() {
    print_status "Running quick test suite (unit + integration)..."
    run_unit_tests
    run_integration_tests
    print_success "Quick tests completed"
}

run_with_race_detection() {
    print_status "Running tests with race detection..."
    go test -race -v ./internal/... ./pkg/... ./web/... ./tests/workflows/... ./tests/edge_cases/...
    print_success "Race detection tests completed"
}

run_coverage_report() {
    print_status "Generating coverage report..."
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    print_success "Coverage report generated: coverage.html"
}

run_benchmarks() {
    print_status "Running benchmark tests..."
    go test -bench=. -benchmem ./tests/performance/...
    print_success "Benchmarks completed"
}

show_help() {
    echo "Switchboard V4 Test Runner"
    echo ""
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  unit          Run unit tests only"
    echo "  integration   Run integration tests only"
    echo "  workflows     Run workflow tests only"
    echo "  edge-cases    Run edge case tests only"
    echo "  performance   Run performance tests only"
    echo "  quick         Run unit and integration tests (fast)"
    echo "  all           Run complete test suite (slow)"
    echo "  race          Run tests with race detection"
    echo "  coverage      Generate test coverage report"
    echo "  benchmarks    Run benchmark tests"
    echo "  help          Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 quick                 # Fast test run for development"
    echo "  $0 workflows             # Test complete user workflows"
    echo "  $0 performance           # Test system under load"
    echo "  $0 race                  # Check for race conditions"
    echo ""
    echo "Performance Test Notes:"
    echo "  - Performance tests simulate realistic classroom loads"
    echo "  - Tests include 30-100 concurrent users"
    echo "  - May consume significant CPU and memory during execution"
    echo "  - Recommended to run on dedicated test environments"
}

# Main script logic
case "${1:-help}" in
    unit)
        run_unit_tests
        ;;
    integration)
        run_integration_tests
        ;;
    workflows)
        run_workflow_tests
        ;;
    edge-cases)
        run_edge_case_tests
        ;;
    performance)
        run_performance_tests
        ;;
    quick)
        run_quick_tests
        ;;
    all)
        run_all_tests
        ;;
    race)
        run_with_race_detection
        ;;
    coverage)
        run_coverage_report
        ;;
    benchmarks)
        run_benchmarks
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        print_error "Unknown command: $1"
        echo ""
        show_help
        exit 1
        ;;
esac