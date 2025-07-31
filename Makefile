.PHONY: setup build run test clean dev

# Setup development environment
setup:
	go mod tidy
	go mod download
	mkdir -p db logs

# Build the application
build:
	go build -o bin/switchboard cmd/server/main.go

# Run in development mode  
run: build
	./bin/switchboard

# Run all tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f db/switchboard.db*

# Development mode with auto-reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air

# Initialize database
init-db:
	sqlite3 db/switchboard.db < internal/database/migrations.sql

# Load test data
load-fixtures:
	sqlite3 db/switchboard.db < db/fixtures/test_data.sql

# Database shell
db-shell:
	sqlite3 db/switchboard.db

# Format code
fmt:
	go fmt ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Build for production
build-prod:
	CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o bin/switchboard-prod cmd/server/main.go