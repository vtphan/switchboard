# Phase 5: API Layer and System Integration

## Overview
Implements HTTP API endpoints for session management, health monitoring, and system integration. Provides the final integration layer that brings together all components into a complete working system.

## Step 5.1: HTTP API Endpoints (Estimated: 2.5h)

### EXACT REQUIREMENTS (do not exceed scope):
- Implement exact API endpoints from specs: POST/DELETE/GET sessions, GET health
- JSON request/response handling with proper error codes
- Integration with SessionManager for session operations
- Health endpoint with system statistics and database status
- Proper HTTP status codes: 200, 201, 400, 404, 500, 503
- CORS handling for web client access

### ARCHITECTURAL VALIDATION REQUIREMENTS:
**Dependencies** (BLOCKING):
- Only import: pkg/interfaces, pkg/types, net/http, encoding/json, standard library
- No imports from: internal/websocket, internal/router, internal/database (use interfaces)
- Verify: `go mod graph | grep -E "cycle|circular"` returns empty

**Boundaries** (BLOCKING):
- Pure HTTP API layer - no business logic, just HTTP handling and JSON serialization
- Clean separation: HTTP handling vs business operations
- Interface usage: interact with components through interfaces only

**Integration Contracts** (BLOCKING):
- Uses SessionManager interface for all session operations
- Uses system components through interfaces for health checks
- Provides REST API that external clients can consume
- Error handling appropriate for HTTP responses

### FUNCTIONAL VALIDATION REQUIREMENTS:
**Core Behaviors** (BLOCKING):
- POST /api/sessions creates session with automatic duplicate student ID removal
- DELETE /api/sessions/{id} ends session and returns proper status
- GET /api/sessions/{id} returns session details with all fields
- GET /api/sessions lists only active sessions with connection counts
- GET /health returns system status with database connectivity check

**Error Handling** (BLOCKING):
- 400 Bad Request for invalid JSON or missing required fields
- 404 Not Found for non-existent sessions
- 500 Internal Server Error for database/system failures
- 503 Service Unavailable for health check failures
- Proper JSON error responses with descriptive messages

**Integration Contracts** (BLOCKING):
- Session operations match SessionManager interface exactly
- Health checks validate all system components
- Response formats match API specification exactly
- HTTP status codes align with standard REST practices

### TECHNICAL VALIDATION REQUIREMENTS:
**Performance** (WARNING):
- API endpoints respond in <100ms for typical operations
- JSON parsing and serialization efficient for session data
- Health checks complete quickly (<500ms)

**Security** (WARNING):
- Input validation prevents injection attacks
- Proper error handling doesn't leak sensitive information
- CORS configuration appropriate for deployment environment

### MANDATORY IMPLEMENTATION (implement exactly):
```go
package api

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strings"
    "time"
    
    "github.com/gorilla/mux"
    "github.com/switchboard/pkg/interfaces"
    "github.com/switchboard/pkg/types"
)

// Server implements the HTTP API server
type Server struct {
    sessionManager interfaces.SessionManager
    dbManager      interfaces.DatabaseManager
    registry       ConnectionRegistry // For health stats
    router         *mux.Router
}

// ConnectionRegistry interface for health statistics
type ConnectionRegistry interface {
    GetStats() map[string]int
}

// NewServer creates a new API server
func NewServer(sessionManager interfaces.SessionManager, dbManager interfaces.DatabaseManager, registry ConnectionRegistry) *Server {
    server := &Server{
        sessionManager: sessionManager,
        dbManager:      dbManager,
        registry:       registry,
        router:         mux.NewRouter(),
    }
    
    server.setupRoutes()
    return server
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
    // API routes
    api := s.router.PathPrefix("/api").Subrouter()
    api.HandleFunc("/sessions", s.createSession).Methods("POST")
    api.HandleFunc("/sessions", s.listSessions).Methods("GET")
    api.HandleFunc("/sessions/{id}", s.getSession).Methods("GET")
    api.HandleFunc("/sessions/{id}", s.endSession).Methods("DELETE")
    
    // Health check
    s.router.HandleFunc("/health", s.healthCheck).Methods("GET")
    
    // CORS middleware
    s.router.Use(s.corsMiddleware)
    
    // JSON content type middleware
    api.Use(s.jsonMiddleware)
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    s.router.ServeHTTP(w, r)
}
```

### CRITICAL API ENDPOINTS (must follow):
```go
// CreateSessionRequest represents the request body for creating a session
type CreateSessionRequest struct {
    Name       string   `json:"name"`
    StudentIDs []string `json:"student_ids"`
}

// CreateSessionResponse represents the response for creating a session
type CreateSessionResponse struct {
    SessionID string    `json:"session_id"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
}

// createSession handles POST /api/sessions
func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
    var req CreateSessionRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.sendError(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    // Validate required fields
    if req.Name == "" {
        s.sendError(w, "Missing required field: name", http.StatusBadRequest)
        return
    }
    
    if len(req.StudentIDs) == 0 {
        s.sendError(w, "Missing required field: student_ids", http.StatusBadRequest)
        return
    }
    
    // Extract instructor ID from request context (set by auth middleware)
    instructorID := r.Header.Get("X-User-ID")
    if instructorID == "" {
        s.sendError(w, "Missing instructor ID", http.StatusBadRequest)
        return
    }
    
    // Create session (duplicate student IDs will be removed automatically)
    ctx := context.Background()
    session, err := s.sessionManager.CreateSession(ctx, req.Name, instructorID, req.StudentIDs)
    if err != nil {
        log.Printf("Failed to create session: %v", err)
        s.sendError(w, "Failed to create session", http.StatusInternalServerError)
        return
    }
    
    // Build response
    response := CreateSessionResponse{
        SessionID: session.ID,
        Status:    session.Status,
        CreatedAt: session.StartTime,
    }
    
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(response)
}

// getSession handles GET /api/sessions/{id}
func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    sessionID := vars["id"]
    
    if sessionID == "" {
        s.sendError(w, "Missing session ID", http.StatusBadRequest)
        return
    }
    
    ctx := context.Background()
    session, err := s.sessionManager.GetSession(ctx, sessionID)
    if err != nil {
        if err == interfaces.ErrSessionNotFound {
            s.sendError(w, "Session not found", http.StatusNotFound)
        } else {
            log.Printf("Failed to get session %s: %v", sessionID, err)
            s.sendError(w, "Internal server error", http.StatusInternalServerError)
        }
        return
    }
    
    json.NewEncoder(w).Encode(session)
}

// endSession handles DELETE /api/sessions/{id}
func (s *Server) endSession(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    sessionID := vars["id"]
    
    if sessionID == "" {
        s.sendError(w, "Missing session ID", http.StatusBadRequest)
        return
    }
    
    ctx := context.Background()
    
    // Get session first to build response
    session, err := s.sessionManager.GetSession(ctx, sessionID)
    if err != nil {
        if err == interfaces.ErrSessionNotFound {
            s.sendError(w, "Session not found", http.StatusNotFound)
        } else {
            log.Printf("Failed to get session %s: %v", sessionID, err)
            s.sendError(w, "Internal server error", http.StatusInternalServerError)
        }
        return
    }
    
    if session.Status == "ended" {
        s.sendError(w, "Session already ended", http.StatusBadRequest)
        return
    }
    
    // End the session
    if err := s.sessionManager.EndSession(ctx, sessionID); err != nil {
        log.Printf("Failed to end session %s: %v", sessionID, err)
        s.sendError(w, "Failed to end session", http.StatusInternalServerError)
        return
    }
    
    // Build response
    now := time.Now()
    response := map[string]interface{}{
        "session_id": sessionID,
        "status":     "ended",
        "ended_at":   now,
    }
    
    json.NewEncoder(w).Encode(response)
}

// ListSessionsResponse represents the response for listing sessions
type ListSessionsResponse struct {
    Sessions   []SessionSummary `json:"sessions"`
    TotalCount int              `json:"total_count"`
}

// SessionSummary represents a session in the list response
type SessionSummary struct {
    SessionID        string    `json:"session_id"`
    Name             string    `json:"name"`
    CreatedBy        string    `json:"created_by"`
    Status           string    `json:"status"`
    CreatedAt        time.Time `json:"created_at"`
    ConnectedClients int       `json:"connected_clients"`
}

// listSessions handles GET /api/sessions
func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
    ctx := context.Background()
    sessions, err := s.sessionManager.ListActiveSessions(ctx)
    if err != nil {
        log.Printf("Failed to list sessions: %v", err)
        s.sendError(w, "Failed to list sessions", http.StatusInternalServerError)
        return
    }
    
    // Get connection statistics
    stats := s.registry.GetStats()
    
    // Build session summaries
    summaries := make([]SessionSummary, len(sessions))
    for i, session := range sessions {
        summaries[i] = SessionSummary{
            SessionID:        session.ID,
            Name:             session.Name,
            CreatedBy:        session.CreatedBy,
            Status:           session.Status,
            CreatedAt:        session.StartTime,
            ConnectedClients: s.getSessionConnectionCount(session.ID),
        }
    }
    
    response := ListSessionsResponse{
        Sessions:   summaries,
        TotalCount: len(summaries),
    }
    
    json.NewEncoder(w).Encode(response)
}

// getSessionConnectionCount returns the number of connections for a session
func (s *Server) getSessionConnectionCount(sessionID string) int {
    // This would need to be implemented by the registry
    // For now, return 0 as placeholder
    return 0
}
```

### HEALTH CHECK IMPLEMENTATION (implement exactly):
```go
// HealthResponse represents the health check response
type HealthResponse struct {
    Status           string                 `json:"status"`
    Timestamp        time.Time              `json:"timestamp"`
    UptimeSeconds    int64                  `json:"uptime_seconds"`
    ActiveSessions   int                    `json:"active_sessions"`
    TotalConnections int                    `json:"total_connections"`
    DatabaseStatus   string                 `json:"database_status"`
    Details          map[string]interface{} `json:"details,omitempty"`
}

var startTime = time.Now()

// healthCheck handles GET /health
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    response := HealthResponse{
        Status:        "healthy",
        Timestamp:     time.Now(),
        UptimeSeconds: int64(time.Since(startTime).Seconds()),
    }
    
    // Check database health
    if err := s.dbManager.HealthCheck(ctx); err != nil {
        response.Status = "unhealthy"
        response.DatabaseStatus = "disconnected"
        response.Details = map[string]interface{}{
            "database_error": err.Error(),
        }
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(response)
        return
    }
    response.DatabaseStatus = "connected"
    
    // Get session statistics
    sessions, err := s.sessionManager.ListActiveSessions(ctx)
    if err != nil {
        response.ActiveSessions = 0
    } else {
        response.ActiveSessions = len(sessions)
    }
    
    // Get connection statistics
    stats := s.registry.GetStats()
    response.TotalConnections = stats["total_connections"]
    
    json.NewEncoder(w).Encode(response)
}

// Error response helper
type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message"`
}

// sendError sends a JSON error response
func (s *Server) sendError(w http.ResponseWriter, message string, statusCode int) {
    w.WriteHeader(statusCode)
    response := ErrorResponse{
        Error:   http.StatusText(statusCode),
        Message: message,
    }
    json.NewEncoder(w).Encode(response)
}

// CORS middleware
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-ID")
        
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}

// JSON middleware
func (s *Server) jsonMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        next.ServeHTTP(w, r)
    })
}
```

### SUCCESS CRITERIA:
**Architectural** (BLOCKING):
- [ ] No circular dependencies: clean HTTP layer using interfaces
- [ ] Boundary separation: pure HTTP handling, no business logic
- [ ] Interface usage: all operations through SessionManager/DatabaseManager
- [ ] Proper middleware: CORS, JSON content-type, error handling

**Functional** (BLOCKING):
- [ ] All API endpoints match specification exactly
- [ ] HTTP status codes correct for each error scenario
- [ ] JSON serialization works for all request/response types
- [ ] Health check validates system components properly

**Technical** (WARNING):
- [ ] API responses in <100ms for typical operations
- [ ] Proper error handling without information leakage
- [ ] CORS configuration appropriate for web clients
- [ ] JSON parsing robust against malformed input

### INTEGRATION CONTRACTS:
**What SessionManager provides:**
- CreateSession(), GetSession(), EndSession(), ListActiveSessions() operations
- Error types that map to appropriate HTTP status codes
- Fast operations suitable for HTTP response times

**What clients expect:**
- REST API following OpenAPI specification
- Consistent JSON response formats
- Proper HTTP status codes for different scenarios
- Health endpoint for monitoring and load balancing

### READ ONLY THESE REFERENCES:
- switchboard-tech-specs.md lines 404-488 (API endpoints specification)
- switchboard-tech-specs.md lines 489-507 (health check endpoint)
- pkg/interfaces/session.go and pkg/interfaces/database.go (interface definitions)

### IGNORE EVERYTHING ELSE
Do not read sections about WebSocket handling, message routing, or internal component implementation. Focus only on HTTP API requirements.

### FILES TO CREATE:
- internal/api/server.go (HTTP API server implementation)
- internal/api/handlers.go (HTTP handler implementations)
- internal/api/middleware.go (CORS and JSON middleware)
- internal/api/server_test.go (comprehensive API endpoint tests)

---

## Step 5.2: Main Application Integration (Estimated: 2h)

### EXACT REQUIREMENTS (do not exceed scope):
- Main application entry point that initializes all components in correct order
- Configuration management for database, HTTP server, and system settings
- Graceful shutdown handling with proper resource cleanup
- Component lifecycle management and dependency injection
- Error handling and logging configuration
- Integration of all phases into working system

### ARCHITECTURAL VALIDATION REQUIREMENTS:
**Dependencies** (BLOCKING):
- Only import: all internal packages, pkg packages, standard library, external dependencies
- Final integration point - can import from all previous phases
- Verify: all components integrate correctly without circular dependencies

**Boundaries** (BLOCKING):
- Application orchestration layer - coordinates component initialization and shutdown
- Clean separation: main app coordination vs component implementation
- Dependency injection: components receive dependencies through constructors

**Integration Contracts** (BLOCKING):
- All components initialized in correct dependency order
- Configuration passed to components appropriately
- Shutdown coordination ensures proper cleanup
- Error handling appropriate for system-level failures

### FUNCTIONAL VALIDATION REQUIREMENTS:
**Core Behaviors** (BLOCKING):
- Database initialized first with schema validation
- Session manager loaded with active sessions from database
- WebSocket registry and router initialized with dependencies
- HTTP API server starts with all components wired together
- Graceful shutdown stops components in reverse dependency order

**Error Handling** (BLOCKING):
- Configuration validation prevents startup with invalid settings
- Component initialization failures stop application startup
- Database connection issues handled appropriately
- Shutdown errors logged but don't prevent cleanup

**Integration Contracts** (BLOCKING):
- All interfaces satisfied with concrete implementations
- Component lifecycle managed consistently
- Resource cleanup prevents leaks on shutdown
- System ready for production deployment

### TECHNICAL VALIDATION REQUIREMENTS:
**Startup** (WARNING):
- Application startup completes in <5 seconds
- Configuration validation comprehensive and clear
- Component initialization order prevents race conditions

**Shutdown** (WARNING):
- Graceful shutdown completes in <10 seconds
- All resources cleaned up properly
- No hanging goroutines after shutdown

### MANDATORY IMPLEMENTATION (implement exactly):
```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/switchboard/internal/api"
    "github.com/switchboard/internal/database"
    "github.com/switchboard/internal/hub"
    "github.com/switchboard/internal/router"
    "github.com/switchboard/internal/session"
    "github.com/switchboard/internal/websocket"
    dbconfig "github.com/switchboard/pkg/database"
)

// Config holds application configuration
type Config struct {
    Database *dbconfig.Config `json:"database"`
    HTTP     *HTTPConfig      `json:"http"`
    System   *SystemConfig    `json:"system"`
}

// HTTPConfig holds HTTP server configuration
type HTTPConfig struct {
    Address      string        `json:"address"`
    Port         int           `json:"port"`
    ReadTimeout  time.Duration `json:"read_timeout"`
    WriteTimeout time.Duration `json:"write_timeout"`
}

// SystemConfig holds system-level configuration
type SystemConfig struct {
    LogLevel    string `json:"log_level"`
    Environment string `json:"environment"`
}

// DefaultConfig returns production-ready application configuration
func DefaultConfig() *Config {
    return &Config{
        Database: dbconfig.DefaultConfig(),
        HTTP: &HTTPConfig{
            Address:      "0.0.0.0",
            Port:         8080,
            ReadTimeout:  10 * time.Second,
            WriteTimeout: 10 * time.Second,
        },
        System: &SystemConfig{
            LogLevel:    "info",
            Environment: "production",
        },
    }
}

// Application holds all system components
type Application struct {
    config         *Config
    dbManager      *database.Manager
    sessionManager *session.Manager
    registry       *websocket.Registry
    messageRouter  *router.Router
    messageHub     *hub.Hub
    apiServer      *api.Server
    httpServer     *http.Server
}

func main() {
    if err := run(); err != nil {
        log.Fatalf("Application failed: %v", err)
    }
}

func run() error {
    // Load configuration
    config := DefaultConfig()
    
    // Initialize application
    app, err := NewApplication(config)
    if err != nil {
        return fmt.Errorf("failed to initialize application: %w", err)
    }
    
    // Start application
    if err := app.Start(); err != nil {
        return fmt.Errorf("failed to start application: %w", err)
    }
    
    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    log.Println("Application started successfully")
    <-sigChan
    log.Println("Shutdown signal received")
    
    // Graceful shutdown
    if err := app.Stop(); err != nil {
        return fmt.Errorf("failed to stop application gracefully: %w", err)
    }
    
    log.Println("Application stopped successfully")
    return nil
}
```

### CRITICAL INITIALIZATION SEQUENCE (must follow):
```go
// NewApplication creates and initializes all components
func NewApplication(config *Config) (*Application, error) {
    app := &Application{config: config}
    
    log.Println("Initializing application components...")
    
    // Step 1: Initialize database manager
    log.Println("Initializing database manager...")
    dbManager, err := database.NewManager(config.Database)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize database manager: %w", err)
    }
    app.dbManager = dbManager
    
    // Step 2: Validate database schema
    ctx := context.Background()
    if err := dbManager.HealthCheck(ctx); err != nil {
        return nil, fmt.Errorf("database health check failed: %w", err)
    }
    
    // Step 3: Initialize session manager
    log.Println("Initializing session manager...")
    sessionManager := session.NewManager(dbManager)
    if err := sessionManager.LoadActiveSessions(ctx); err != nil {
        return nil, fmt.Errorf("failed to load active sessions: %w", err)
    }
    app.sessionManager = sessionManager
    
    // Step 4: Initialize WebSocket registry
    log.Println("Initializing WebSocket registry...")
    registry := websocket.NewRegistry()
    app.registry = registry
    
    // Step 5: Initialize message router
    log.Println("Initializing message router...")
    messageRouter := router.NewRouter(registry, dbManager)
    app.messageRouter = messageRouter
    
    // Step 6: Initialize message hub
    log.Println("Initializing message hub...")
    messageHub := hub.NewHub(registry, messageRouter)
    app.messageHub = messageHub
    
    // Step 7: Initialize API server
    log.Println("Initializing API server...")
    apiServer := api.NewServer(sessionManager, dbManager, registry)
    app.apiServer = apiServer
    
    // Step 8: Initialize HTTP server
    httpAddr := fmt.Sprintf("%s:%d", config.HTTP.Address, config.HTTP.Port)
    httpServer := &http.Server{
        Addr:         httpAddr,
        Handler:      apiServer,
        ReadTimeout:  config.HTTP.ReadTimeout,
        WriteTimeout: config.HTTP.WriteTimeout,
    }
    app.httpServer = httpServer
    
    log.Println("All components initialized successfully")
    return app, nil
}

// Start starts all application components
func (app *Application) Start() error {
    log.Println("Starting application components...")
    
    // Start message hub first
    ctx := context.Background()
    if err := app.messageHub.Start(ctx); err != nil {
        return fmt.Errorf("failed to start message hub: %w", err)
    }
    
    // Start HTTP server
    go func() {
        log.Printf("Starting HTTP server on %s", app.httpServer.Addr)
        if err := app.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("HTTP server failed: %v", err)
        }
    }()
    
    // Add WebSocket endpoint to HTTP server
    websocketHandler := websocket.NewHandler(app.registry, app.sessionManager, app.dbManager)
    http.HandleFunc("/ws", websocketHandler.HandleWebSocket)
    
    return nil
}

// Stop gracefully stops all application components
func (app *Application) Stop() error {
    log.Println("Stopping application components...")
    
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    // Stop HTTP server first
    log.Println("Stopping HTTP server...")
    if err := app.httpServer.Shutdown(ctx); err != nil {
        log.Printf("HTTP server shutdown error: %v", err)
    }
    
    // Stop message hub
    log.Println("Stopping message hub...")
    if err := app.messageHub.Stop(); err != nil {
        log.Printf("Message hub stop error: %v", err)
    }
    
    // Close all WebSocket connections
    log.Println("Closing WebSocket connections...")
    // This would require adding a CloseAll method to the registry
    
    // Close database manager last
    log.Println("Closing database manager...")
    if err := app.dbManager.Close(); err != nil {
        log.Printf("Database manager close error: %v", err)
    }
    
    log.Println("All components stopped")
    return nil
}
```

### CONFIGURATION MANAGEMENT (implement exactly):
```go
// LoadConfigFromFile loads configuration from JSON file
func LoadConfigFromFile(filename string) (*Config, error) {
    if filename == "" {
        return DefaultConfig(), nil
    }
    
    file, err := os.Open(filename)
    if err != nil {
        if os.IsNotExist(err) {
            log.Printf("Config file %s not found, using defaults", filename)
            return DefaultConfig(), nil
        }
        return nil, fmt.Errorf("failed to open config file: %w", err)
    }
    defer file.Close()
    
    var config Config
    if err := json.NewDecoder(file).Decode(&config); err != nil {
        return nil, fmt.Errorf("failed to parse config file: %w", err)
    }
    
    // Validate configuration
    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }
    
    return &config, nil
}

// Validate validates the application configuration
func (c *Config) Validate() error {
    if c.Database == nil {
        return fmt.Errorf("database configuration required")
    }
    
    if c.HTTP == nil {
        return fmt.Errorf("HTTP configuration required")
    }
    
    if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
        return fmt.Errorf("invalid HTTP port: %d", c.HTTP.Port)
    }
    
    if c.HTTP.ReadTimeout <= 0 {
        return fmt.Errorf("HTTP read timeout must be positive")
    }
    
    if c.HTTP.WriteTimeout <= 0 {
        return fmt.Errorf("HTTP write timeout must be positive")
    }
    
    return nil
}

// Environment configuration
func LoadConfigFromEnv() *Config {
    config := DefaultConfig()
    
    // Database configuration from environment
    if dbPath := os.Getenv("SWITCHBOARD_DB_PATH"); dbPath != "" {
        config.Database.DatabasePath = dbPath
    }
    
    // HTTP configuration from environment
    if httpAddr := os.Getenv("SWITCHBOARD_HTTP_ADDRESS"); httpAddr != "" {
        config.HTTP.Address = httpAddr
    }
    
    if httpPort := os.Getenv("SWITCHBOARD_HTTP_PORT"); httpPort != "" {
        if port, err := strconv.Atoi(httpPort); err == nil {
            config.HTTP.Port = port
        }
    }
    
    // System configuration from environment
    if logLevel := os.Getenv("SWITCHBOARD_LOG_LEVEL"); logLevel != "" {
        config.System.LogLevel = logLevel
    }
    
    if env := os.Getenv("SWITCHBOARD_ENVIRONMENT"); env != "" {
        config.System.Environment = env
    }
    
    return config
}
```

### SUCCESS CRITERIA:
**Architectural** (BLOCKING):
- [ ] All components initialized in correct dependency order
- [ ] No circular dependencies in final integration
- [ ] Clean separation: main app coordination vs component implementation
- [ ] Dependency injection through constructors

**Functional** (BLOCKING):
- [ ] Application starts successfully with all components working
- [ ] Configuration validation prevents invalid startup
- [ ] Graceful shutdown cleans up all resources
- [ ] WebSocket and HTTP endpoints both functional

**Technical** (WARNING):
- [ ] Application startup completes in <5 seconds
- [ ] Graceful shutdown completes in <10 seconds
- [ ] Configuration management flexible and robust
- [ ] Error handling appropriate for production deployment

### INTEGRATION CONTRACTS:
**What all components provide:**
- Clean initialization through constructors
- Proper resource cleanup on shutdown
- Interface-based integration without tight coupling
- Error handling suitable for system-level coordination

**What production deployment expects:**
- Single binary with all functionality
- Configuration through files or environment variables
- Graceful shutdown handling for container orchestration
- Health checks and monitoring endpoints

### READ ONLY THESE REFERENCES:
- All previous phase implementations for integration requirements
- switchboard-tech-specs.md lines 399-403 (startup recovery)
- switchboard-tech-specs.md lines 677-681 (performance considerations)

### IGNORE EVERYTHING ELSE
Do not read detailed implementation sections. Focus only on system integration and application lifecycle.

### FILES TO CREATE:
- cmd/switchboard/main.go (main application entry point)
- internal/config/config.go (configuration management)
- Makefile (build and deployment targets)
- README.md (setup and deployment instructions)