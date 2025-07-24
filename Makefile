# Switchboard Makefile
# Build and validation commands for validation-driven TDD approach

.PHONY: build test test-race lint vet clean run dev validate coverage benchmark help

# Build commands
build:
	go build -o bin/switchboard cmd/server/*.go

run: build
	./bin/switchboard

dev:
	go run cmd/server/*.go

# Test commands  
test:
	go test ./...

test-race:
	go test -race ./...

test-verbose:
	go test -v ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out

# Validation commands (required by validation-driven TDD)
lint:
	golangci-lint run ./...

vulnerability:
	govulncheck ./...

vet:
	go vet ./...

# Performance testing
benchmark:
	go test -bench=. -benchmem ./...

# Load testing for real-time components
load-test:
	go test -run=TestHighLoad -timeout=10m ./internal/websocket

# Resource leak detection
leak-test:
	go test -run=TestResourceCleanup -memprofile=mem.prof ./...
	go tool pprof -top mem.prof

# Goroutine leak detection
goroutine-test:
	go test -run=TestGoroutineLeaks -trace=trace.out ./...
	go tool trace trace.out

# Comprehensive validation (blocking validations for TDD)
validate: vet lint test test-race coverage
	@echo "All validation checks passed"

# Development tools installation
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest

# Database commands
migrate-up:
	sqlite3 switchboard.db < migrations/001_initial.sql

migrate-down:
	rm -f switchboard.db

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f mem.prof trace.out
	rm -f switchboard.db

# Help
help:
	@echo "Available commands:"
	@echo "  build          - Build the application"
	@echo "  run            - Build and run the application"
	@echo "  dev            - Run in development mode"
	@echo "  test           - Run all tests"
	@echo "  test-race      - Run tests with race detection"
	@echo "  coverage       - Generate test coverage report"
	@echo "  lint           - Run static analysis"
	@echo "  vulnerability  - Check for vulnerabilities"
	@echo "  validate       - Run all validation checks"
	@echo "  benchmark      - Run performance benchmarks"
	@echo "  load-test      - Run load tests for real-time components"
	@echo "  leak-test      - Check for memory leaks"
	@echo "  goroutine-test - Check for goroutine leaks"
	@echo "  install-tools  - Install development tools"
	@echo "  clean          - Clean build artifacts"