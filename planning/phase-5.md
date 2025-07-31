# Phase 5: HTTP API & System Integration

**Estimated Time:** 1 day  
**Dependencies:** Phase 1-4 (All previous phases)  
**Provides:** SessionAPI, WebSocketUpgrade, GracefulShutdown, SystemIntegration

## Phase Overview

Implements HTTP API endpoints for session management, integrates all system components, and establishes graceful shutdown procedures. This phase completes the Switchboard system with production-ready operational capabilities.

---

## Step 5.1: HTTP Session Management API (Estimated: 3h)

### EXACT REQUIREMENTS:
Read **exactly lines 637-675** of `/Users/vinhthuyphan/Apps/switchboard/docs/tech-specs.md` for complete HTTP API specifications. Implement RESTful endpoints with exact request/response formats.

### ARCHITECTURAL VALIDATION:
- Implementation in `web/api/handlers.go`
- Uses SessionLifecycle from Phase 2 for session operations
- No direct database or connection dependencies
- Standard HTTP patterns with proper status codes and JSON responses

### FUNCTIONAL VALIDATION:
- POST /api/session/start: Creates session with validation and returns session details
- POST /api/session/end: Ends session with instructor verification
- Error responses: Consistent JSON format with proper HTTP status codes
- Request validation: Validates required fields and constraints

### INTEGRATION CONTRACTS:
- HTTP server routes session API calls to these handlers
- SessionLifecycle manages actual session state and database persistence
- WebSocket connections receive session state changes via broadcast system

### MANDATORY INTERFACE:
```go
// SessionAPIHandler handles HTTP session management endpoints
type SessionAPIHandler struct {
    sessionLifecycle *SessionLifecycle
}

// POST /api/session/start
func (sah *SessionAPIHandler) StartSession(w http.ResponseWriter, r *http.Request)

// POST /api/session/end  
func (sah *SessionAPIHandler) EndSession(w http.ResponseWriter, r *http.Request)
```

### EXACT IMPLEMENTATION PATTERN:
```go
func NewSessionAPIHandler(sessionLifecycle *SessionLifecycle) *SessionAPIHandler {
    return &SessionAPIHandler{
        sessionLifecycle: sessionLifecycle,
    }
}

func (sah *SessionAPIHandler) StartSession(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    // Parse request body
    var request struct {
        Name         string `json:"name"`
        InstructorID string `json:"instructor_id"`
    }
    
    decoder := json.NewDecoder(r.Body)
    if err := decoder.Decode(&request); err != nil {
        writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid JSON format")
        return
    }
    
    // Validate required fields
    if request.Name == "" {
        writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Session name is required")
        return
    }
    
    if request.InstructorID == "" {
        writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Instructor ID is required")
        return
    }
    
    // Create session
    session, err := sah.sessionLifecycle.StartSession(request.Name, request.InstructorID)
    if err != nil {
        if err == ErrSessionAlreadyActive {
            writeErrorResponse(w, http.StatusConflict, "session_already_active", "A session is already active")
            return
        }
        
        writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to create session")
        return
    }
    
    // Return success response
    response := map[string]interface{}{
        "session": map[string]interface{}{
            "id":         session.ID,
            "name":       session.Name,
            "created_by": session.CreatedBy,
            "status":     session.Status,
            "start_time": session.StartTime.Format(time.RFC3339),
        },
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(response)
}

func (sah *SessionAPIHandler) EndSession(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    // Parse request body
    var request struct {
        InstructorID string `json:"instructor_id"`
    }
    
    decoder := json.NewDecoder(r.Body)
    if err := decoder.Decode(&request); err != nil {
        writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid JSON format")
        return
    }
    
    // Validate required fields
    if request.InstructorID == "" {
        writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Instructor ID is required")
        return
    }
    
    // End session
    session, err := sah.sessionLifecycle.EndSession(request.InstructorID)
    if err != nil {
        if err == ErrNoActiveSession {
            writeErrorResponse(w, http.StatusConflict, "no_active_session", "No active session to end")
            return
        }
        
        writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to end session")
        return
    }
    
    // Return success response
    response := map[string]interface{}{
        "session_id": session.ID,
        "status":     session.Status,
        "ended_at":   session.EndTime.Format(time.RFC3339),
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(response)
}

func writeErrorResponse(w http.ResponseWriter, statusCode int, errorType, message string) {
    response := map[string]interface{}{
        "error": map[string]interface{}{
            "type":    errorType,
            "message": message,
        },
        "timestamp": time.Now().Format(time.RFC3339),
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(response)
}
```

### SUCCESS CRITERIA:
- [ ] POST /api/session/start creates sessions with exact response format
- [ ] POST /api/session/end ends sessions with proper validation
- [ ] Error responses match exact format from tech specs
- [ ] HTTP status codes match REST conventions
- [ ] Request validation prevents invalid data
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `web/api/handlers.go`
- `web/api/handlers_test.go`

---

## Step 5.2: HTTP Server & Routing (Estimated: 2h)

### EXACT REQUIREMENTS:
Implement HTTP server with routing for API endpoints, WebSocket upgrade, and static file serving. Server must support graceful shutdown and proper request handling.

### ARCHITECTURAL VALIDATION:
- Implementation in `web/server.go`
- Uses standard library `net/http` with proper middleware patterns
- Integrates WebSocketHandler from Phase 4 and SessionAPI from Step 5.1
- Serves static test client from existing web/static directory

### FUNCTIONAL VALIDATION:
- HTTP routing: API endpoints and WebSocket upgrade properly routed
- Static files: Test client served from /web/static directory
- CORS handling: Proper headers for development and production
- Request logging: Basic access logging for debugging

### INTEGRATION CONTRACTS:
- Main application starts HTTP server with all integrated components
- Graceful shutdown stops HTTP server cleanly
- All endpoints accessible via standard HTTP methods

### MANDATORY INTERFACE:
```go
// HTTPServer manages HTTP routing and WebSocket upgrades
type HTTPServer struct {
    server           *http.Server
    sessionHandler   *SessionAPIHandler
    websocketHandler *WebSocketHandler
    mux              *http.ServeMux
}

// Start begins HTTP server
func (hs *HTTPServer) Start(port string) error

// Stop gracefully shuts down HTTP server
func (hs *HTTPServer) Stop(ctx context.Context) error
```

### EXACT IMPLEMENTATION PATTERN:
```go
func NewHTTPServer(sessionHandler *SessionAPIHandler, websocketHandler *WebSocketHandler) *HTTPServer {
    mux := http.NewServeMux()
    
    server := &HTTPServer{
        sessionHandler:   sessionHandler,
        websocketHandler: websocketHandler,
        mux:              mux,
    }
    
    // Setup routes
    server.setupRoutes()
    
    server.server = &http.Server{
        Handler:      server.mux,
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
        IdleTimeout:  120 * time.Second,
    }
    
    return server
}

func (hs *HTTPServer) setupRoutes() {
    // API routes
    hs.mux.HandleFunc("/api/session/start", hs.withCORS(hs.withLogging(hs.sessionHandler.StartSession)))
    hs.mux.HandleFunc("/api/session/end", hs.withCORS(hs.withLogging(hs.sessionHandler.EndSession)))
    
    // WebSocket route
    hs.mux.HandleFunc("/ws", hs.withLogging(func(w http.ResponseWriter, r *http.Request) {
        if err := hs.websocketHandler.HandleWebSocketUpgrade(w, r); err != nil {
            log.Printf("WebSocket upgrade error: %v", err)
        }
    }))
    
    // Static files for test client
    fileServer := http.FileServer(http.Dir("web/static/"))
    hs.mux.Handle("/", fileServer)
}

func (hs *HTTPServer) Start(port string) error {
    hs.server.Addr = ":" + port
    
    log.Printf("Starting HTTP server on port %s", port)
    
    if err := hs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        return fmt.Errorf("server start failed: %w", err)
    }
    
    return nil
}

func (hs *HTTPServer) Stop(ctx context.Context) error {
    log.Println("Shutting down HTTP server...")
    
    if err := hs.server.Shutdown(ctx); err != nil {
        return fmt.Errorf("server shutdown failed: %w", err)
    }
    
    log.Println("HTTP server stopped gracefully")
    return nil
}

// Middleware functions
func (hs *HTTPServer) withCORS(handler http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Basic CORS headers for development
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
        
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        
        handler(w, r)
    }
}

func (hs *HTTPServer) withLogging(handler http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Call handler
        handler(w, r)
        
        // Log request
        duration := time.Since(start)
        log.Printf("%s %s - %v", r.Method, r.URL.Path, duration)
    }
}
```

### SUCCESS CRITERIA:
- [ ] HTTP server starts and listens on specified port
- [ ] API endpoints accessible via HTTP requests
- [ ] WebSocket upgrade works at /ws endpoint
- [ ] Static test client served from root path
- [ ] CORS headers set correctly for development
- [ ] Request logging shows HTTP access patterns
- [ ] Graceful shutdown stops server cleanly
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `web/server.go`
- `web/server_test.go`

---

## Step 5.3: System Integration & Main Application (Estimated: 2h)

### EXACT REQUIREMENTS:
Integrate all system components in main application with proper initialization order, configuration loading, and dependency injection. Follow initialization sequence from tech specs.

### ARCHITECTURAL VALIDATION:
- Implementation in `cmd/server/main.go` (replace existing placeholder)
- Proper dependency injection order: Database → Session → Message → WebSocket → HTTP
- Configuration loading from existing `config/development.yaml`
- Error handling for all initialization steps

### FUNCTIONAL VALIDATION:
- Component initialization: All components initialize in correct dependency order
- Configuration: Load settings from YAML configuration files
- Database setup: Apply schema and start database manager
- Signal handling: Graceful shutdown on SIGINT/SIGTERM

### INTEGRATION CONTRACTS:
- All previous phases integrate through main application
- Application can be started with `make run` command
- Test client accessible at http://localhost:8080
- WebSocket connections work end-to-end

### MANDATORY INTERFACE:
```go
// Application represents the complete Switchboard system
type Application struct {
    config           *Config
    dbManager        DatabaseManager
    sessionManager   SessionManager
    sessionLifecycle *SessionLifecycle
    messageProcessor *MessageProcessor
    rateLimiter      *RateLimiter
    connectionRegistry *ConnectionRegistry
    websocketHandler *WebSocketHandler
    sessionAPIHandler *SessionAPIHandler
    httpServer       *HTTPServer
}

// Start initializes and starts all components
func (app *Application) Start() error

// Stop gracefully shuts down all components
func (app *Application) Stop(ctx context.Context) error
```

### EXACT IMPLEMENTATION PATTERN:
```go
// Replace existing cmd/server/main.go
package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    _ "github.com/mattn/go-sqlite3"
    "gopkg.in/yaml.v3"
    
    "switchboard/internal/database"
    "switchboard/internal/session"
    "switchboard/internal/message"
    "switchboard/internal/rate"
    "switchboard/internal/websocket"
    "switchboard/web"
)

type Config struct {
    Server struct {
        Host string `yaml:"host"`
        Port string `yaml:"port"`
        ReadTimeout  string `yaml:"read_timeout"`
        WriteTimeout string `yaml:"write_timeout"`
    } `yaml:"server"`
    
    Database struct {
        Path string `yaml:"path"`
    } `yaml:"database"`
    
    Logging struct {
        Level string `yaml:"level"`
        File  string `yaml:"file"`
    } `yaml:"logging"`
    
    WebSocket struct {
        CheckOrigin       bool   `yaml:"check_origin"`
        HandshakeTimeout string `yaml:"handshake_timeout"`
    } `yaml:"websocket"`
}

func main() {
    log.Println("Switchboard V4 starting...")
    
    // Load configuration
    config, err := loadConfig("config/development.yaml")
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }
    
    // Initialize application
    app, err := NewApplication(config)
    if err != nil {
        log.Fatalf("Failed to initialize application: %v", err)
    }
    
    // Start application
    if err := app.Start(); err != nil {
        log.Fatalf("Failed to start application: %v", err)
    }
    
    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    log.Println("Switchboard V4 running. Press Ctrl+C to shutdown.")
    <-sigChan
    
    // Graceful shutdown
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    log.Println("Shutting down...")
    if err := app.Stop(shutdownCtx); err != nil {
        log.Printf("Shutdown error: %v", err)
    } else {
        log.Println("Shutdown completed successfully")
    }
}

func NewApplication(config *Config) (*Application, error) {
    app := &Application{config: config}
    
    // Initialize database (Phase 1)
    db, err := sql.Open("sqlite3", config.Database.Path)
    if err != nil {
        return nil, fmt.Errorf("database connection failed: %w", err)
    }
    
    // Apply schema
    if err := applyDatabaseSchema(db); err != nil {
        return nil, fmt.Errorf("schema application failed: %w", err)
    }
    
    app.dbManager = database.NewSQLiteDatabaseManager(db)
    
    // Initialize session management (Phase 2)
    app.sessionManager = session.NewSessionManagerImpl(app.dbManager)
    app.sessionLifecycle = session.NewSessionLifecycle(app.sessionManager, app.dbManager)
    
    // Initialize message processing (Phase 3)
    app.rateLimiter = rate.NewRateLimiter()
    app.connectionRegistry = websocket.NewConnectionRegistry()
    
    messageRouter := message.NewMessageRouter(app.connectionRegistry)
    roleBasedFilter := &message.RoleBasedFilter{}
    broadcastSystem := websocket.NewBroadcastSystem(app.connectionRegistry, roleBasedFilter)
    
    app.messageProcessor = message.NewMessageProcessor(
        app.sessionManager,
        app.dbManager,
        app.rateLimiter,
        messageRouter,
        broadcastSystem,
    )
    
    // Initialize WebSocket handling (Phase 4)
    app.websocketHandler = websocket.NewWebSocketHandler(
        app.connectionRegistry,
        app.sessionManager,
        app.messageProcessor,
    )
    
    // Initialize HTTP API (Phase 5)
    app.sessionAPIHandler = web.NewSessionAPIHandler(app.sessionLifecycle)
    app.httpServer = web.NewHTTPServer(app.sessionAPIHandler, app.websocketHandler)
    
    return app, nil
}

func (app *Application) Start() error {
    log.Println("Starting application components...")
    
    // Start database manager
    if err := app.dbManager.Start(); err != nil {
        return fmt.Errorf("database manager start failed: %w", err)
    }
    
    // Start rate limiter
    app.rateLimiter.Start()
    
    // Start connection registry
    app.connectionRegistry.Start()
    
    // Start HTTP server (blocks until shutdown)
    go func() {
        port := app.config.Server.Port
        if port == "" {
            port = "8080"
        }
        
        if err := app.httpServer.Start(port); err != nil {
            log.Printf("HTTP server error: %v", err)
        }
    }()
    
    log.Printf("All components started successfully")
    log.Printf("Test client available at: http://localhost:%s", app.config.Server.Port)
    log.Printf("WebSocket endpoint: ws://localhost:%s/ws", app.config.Server.Port)
    
    return nil
}

func (app *Application) Stop(ctx context.Context) error {
    log.Println("Stopping application components...")
    
    // Stop HTTP server first
    if err := app.httpServer.Stop(ctx); err != nil {
        log.Printf("HTTP server shutdown error: %v", err)
    }
    
    // Stop connection registry (closes all WebSocket connections)
    app.connectionRegistry.Stop()
    
    // Stop rate limiter
    app.rateLimiter.Stop()
    
    // Stop database manager (flushes pending writes)
    if err := app.dbManager.Stop(); err != nil {
        log.Printf("Database manager shutdown error: %v", err)
    }
    
    log.Println("All components stopped")
    return nil
}

func loadConfig(configPath string) (*Config, error) {
    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, fmt.Errorf("config file read failed: %w", err)
    }
    
    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("config parse failed: %w", err)
    }
    
    return &config, nil
}

func applyDatabaseSchema(db *sql.DB) error {
    schema, err := os.ReadFile("internal/database/migrations.sql")
    if err != nil {
        return fmt.Errorf("schema file read failed: %w", err)
    }
    
    if _, err := db.Exec(string(schema)); err != nil {
        return fmt.Errorf("schema execution failed: %w", err)
    }
    
    return nil
}
```

### SUCCESS CRITERIA:
- [ ] All components initialize in correct dependency order
- [ ] Configuration loads from YAML file successfully
- [ ] Database schema applies without errors
- [ ] HTTP server starts and serves test client
- [ ] WebSocket connections work end-to-end
- [ ] Graceful shutdown completes within 10 seconds
- [ ] Application can be started with `make run`
- [ ] Coverage ≥85% statements

### FILES TO CREATE:
- `cmd/server/main.go` (replace existing)
- `cmd/server/config.go` (configuration loading)

---

## Step 5.4: Graceful Shutdown Implementation (Estimated: 1h)

### EXACT REQUIREMENTS:
Read **exactly lines 753-805** of `/Users/vinhthuyphan/Apps/switchboard/docs/tech-specs.md` for complete graceful shutdown algorithm. Implement multi-phase shutdown with proper timeouts.

### ARCHITECTURAL VALIDATION:
- Implementation integrated into main application and HTTP server
- Multi-phase shutdown: Stop accepting → Wait for messages → Close connections → Stop database → Wait for goroutines
- Timeout handling: Each phase has specific timeout from configuration constants

### FUNCTIONAL VALIDATION:
- Phase 1: Stop accepting new connections immediately
- Phase 2: Wait for in-flight message processing with timeout
- Phase 3: Close all WebSocket connections gracefully
- Phase 4: Stop database manager and flush pending writes
- Phase 5: Wait for all goroutines to finish with timeout

### INTEGRATION CONTRACTS:
- Signal handling triggers graceful shutdown
- All components support Stop() method with context cancellation
- Shutdown completes within total timeout (10 seconds)

### EXACT IMPLEMENTATION PATTERN:
```go
// Enhanced Stop method in main.go Application
func (app *Application) Stop(ctx context.Context) error {
    log.Println("Starting graceful shutdown...")
    
    // Phase 1: Stop accepting new connections
    log.Println("Phase 1: Stopping HTTP server...")
    if err := app.httpServer.Stop(ctx); err != nil {
        log.Printf("HTTP server shutdown error: %v", err)
    }
    
    // Phase 2: Wait for in-flight message processing (with timeout)
    log.Println("Phase 2: Waiting for message processing completion...")
    messageCtx, messageCancel := context.WithTimeout(ctx, MessageProcessingTimeout)
    defer messageCancel()
    
    // Wait for message processing to complete
    done := make(chan struct{})
    go func() {
        // Wait for message processor to finish current operations
        // This would require implementing a WaitGroup in MessageProcessor
        close(done)
    }()
    
    select {
    case <-done:
        log.Println("Message processing completed")
    case <-messageCtx.Done():
        log.Println("Message processing timeout - forcing continuation")
    }
    
    // Phase 3: Close all WebSocket connections
    log.Println("Phase 3: Closing WebSocket connections...")
    app.connectionRegistry.Stop() // Closes all connections and stops cleanup
    
    // Phase 4: Stop database manager
    log.Println("Phase 4: Stopping database manager...")
    if err := app.dbManager.Stop(); err != nil {
        log.Printf("Database manager shutdown error: %v", err)
    }
    
    // Phase 5: Stop remaining components
    log.Println("Phase 5: Stopping rate limiter...")
    app.rateLimiter.Stop()
    
    // Wait for all goroutines (with timeout)
    goroutineCtx, goroutineCancel := context.WithTimeout(ctx, GoroutineCleanupTimeout)
    defer goroutineCancel()
    
    goroutineDone := make(chan struct{})
    go func() {
        // In a full implementation, this would wait for a main WaitGroup
        // For now, we give a brief moment for cleanup
        time.Sleep(100 * time.Millisecond)
        close(goroutineDone)
    }()
    
    select {
    case <-goroutineDone:
        log.Println("All goroutines finished")
    case <-goroutineCtx.Done():
        log.Println("Goroutine cleanup timeout - forcing exit")
    }
    
    log.Println("Graceful shutdown completed")
    return nil
}

// Enhanced ConnectionRegistry.Stop for graceful connection closure
func (cr *ConnectionRegistry) Stop() {
    log.Println("Closing all WebSocket connections...")
    
    cr.mu.Lock()
    defer cr.mu.Unlock()
    
    // Send close message to all connections
    for userID, conn := range cr.connections {
        // Send WebSocket close frame with reason
        if wsConn := conn.conn; wsConn != nil {
            message := websocket.FormatCloseMessage(websocket.CloseGoingAway, "Server shutting down")
            wsConn.WriteMessage(websocket.CloseMessage, message)
        }
        
        // Close connection
        conn.Close()
        log.Printf("Closed connection for user: %s", userID)
    }
    
    // Clear connections map
    cr.connections = make(map[string]*Connection)
    
    // Stop cleanup ticker
    if cr.cleanupTicker != nil {
        cr.cleanupTicker.Stop()
    }
    
    // Signal cleanup goroutine to stop
    close(cr.stopCh)
    
    log.Printf("All WebSocket connections closed")
}
```

### SUCCESS CRITERIA:
- [ ] Shutdown phases execute in correct order
- [ ] Each phase respects timeout limits
- [ ] WebSocket connections receive close messages
- [ ] Database flushes pending writes before shutdown
- [ ] Total shutdown time within 10 seconds
- [ ] No goroutine leaks after shutdown
- [ ] Coverage ≥85% statements

### FILES TO MODIFY:
- `cmd/server/main.go` (enhanced Stop method)
- `internal/websocket/registry.go` (enhanced Stop method)

---

## Phase 5 Integration Tests

### Phase 5 Integration Test: Complete System End-to-End
```go
// Auto-generate: tests/integration/phase5_end_to_end_integration_test.go
func TestCompleteSystem_EndToEnd_Integration(t *testing.T) {
    // Start test application
    config := &Config{
        Server:   ServerConfig{Host: "localhost", Port: "0"}, // Random port
        Database: DatabaseConfig{Path: ":memory:"},
    }
    
    app, err := NewApplication(config)
    require.NoError(t, err)
    
    err = app.Start()
    require.NoError(t, err)
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        app.Stop(ctx)
    }()
    
    // Get actual server port
    serverPort := app.httpServer.server.Addr
    baseURL := fmt.Sprintf("http://localhost%s", serverPort)
    wsURL := fmt.Sprintf("ws://localhost%s/ws", strings.TrimPrefix(serverPort, ":"))
    
    // Test 1: Start session via HTTP API
    sessionReq := map[string]string{
        "name":          "End-to-End Test Session", 
        "instructor_id": "instructor_test",
    }
    reqBody, _ := json.Marshal(sessionReq)
    
    resp, err := http.Post(baseURL+"/api/session/start", "application/json", bytes.NewBuffer(reqBody))
    require.NoError(t, err)
    defer resp.Body.Close()
    
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
    
    var sessionResp map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&sessionResp)
    require.NoError(t, err)
    
    sessionData := sessionResp["session"].(map[string]interface{})
    sessionID := sessionData["id"].(string)
    
    // Test 2: Connect instructor via WebSocket
    instructorConn, _, err := websocket.DefaultDialer.Dial(wsURL+"?user_id=instructor_test&role=instructor", nil)
    require.NoError(t, err)
    defer instructorConn.Close()
    
    // Should receive session_active message
    var instructorMsg map[string]interface{}
    err = instructorConn.ReadJSON(&instructorMsg)
    require.NoError(t, err)
    
    content := instructorMsg["content"].(map[string]interface{})
    assert.Equal(t, "session_active", content["event"])
    assert.Equal(t, sessionID, content["session_id"])
    
    // Test 3: Connect student via WebSocket
    studentConn, _, err := websocket.DefaultDialer.Dial(wsURL+"?user_id=student_test&role=student", nil)
    require.NoError(t, err)
    defer studentConn.Close()
    
    // Should receive session_active message
    var studentMsg map[string]interface{}
    err = studentConn.ReadJSON(&studentMsg)
    require.NoError(t, err)
    
    content = studentMsg["content"].(map[string]interface{})
    assert.Equal(t, "session_active", content["event"])
    
    // Test 4: Student sends question to instructors
    questionMsg := map[string]interface{}{
        "type":    "broadcast_to_instructors",
        "context": "question",
        "content": map[string]interface{}{"text": "I have a question about the assignment"},
    }
    
    err = studentConn.WriteJSON(questionMsg)
    require.NoError(t, err)
    
    // Instructor should receive the question
    var receivedQuestion map[string]interface{}
    err = instructorConn.ReadJSON(&receivedQuestion) 
    require.NoError(t, err)
    
    assert.Equal(t, "broadcast_to_instructors", receivedQuestion["type"])
    assert.Equal(t, "student_test", receivedQuestion["from_user"])
    
    // Test 5: Instructor broadcasts announcement to students
    announcementMsg := map[string]interface{}{
        "type":    "broadcast_to_students",
        "context": "announcement",
        "content": map[string]interface{}{"text": "Remember the assignment is due tomorrow"},
    }
    
    err = instructorConn.WriteJSON(announcementMsg)
    require.NoError(t, err)
    
    // Student should receive the announcement
    var receivedAnnouncement map[string]interface{}
    err = studentConn.ReadJSON(&receivedAnnouncement)
    require.NoError(t, err)
    
    assert.Equal(t, "broadcast_to_students", receivedAnnouncement["type"])
    assert.Equal(t, "instructor_test", receivedAnnouncement["from_user"])
    
    // Test 6: End session via HTTP API
    endReq := map[string]string{"instructor_id": "instructor_test"}
    reqBody, _ = json.Marshal(endReq)
    
    resp, err = http.Post(baseURL+"/api/session/end", "application/json", bytes.NewBuffer(reqBody))
    require.NoError(t, err)
    defer resp.Body.Close()
    
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    
    var endResp map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&endResp)
    require.NoError(t, err)
    
    assert.Equal(t, "ended", endResp["status"])
    assert.Equal(t, sessionID, endResp["session_id"])
    
    // Test 7: Verify system state after session end
    time.Sleep(100 * time.Millisecond) // Allow message processing
    
    // New connections should receive waiting message
    newStudentConn, _, err := websocket.DefaultDialer.Dial(wsURL+"?user_id=new_student&role=student", nil)
    require.NoError(t, err)
    defer newStudentConn.Close()
    
    var waitingMsg map[string]interface{}
    err = newStudentConn.ReadJSON(&waitingMsg)
    require.NoError(t, err)
    
    content = waitingMsg["content"].(map[string]interface{})
    assert.Equal(t, "waiting_for_session", content["event"])
}
```

### Phase 5 Integration Test: Graceful Shutdown
```go
// Auto-generate: tests/integration/phase5_graceful_shutdown_integration_test.go
func TestGracefulShutdown_Integration(t *testing.T) {
    config := &Config{
        Server:   ServerConfig{Host: "localhost", Port: "0"},
        Database: DatabaseConfig{Path: ":memory:"},
    }
    
    app, err := NewApplication(config)
    require.NoError(t, err)
    
    err = app.Start()
    require.NoError(t, err)
    
    // Create some active connections
    serverPort := app.httpServer.server.Addr
    wsURL := fmt.Sprintf("ws://localhost%s/ws", strings.TrimPrefix(serverPort, ":"))
    
    // Connect multiple users
    connections := make([]*websocket.Conn, 3)
    for i := 0; i < 3; i++ {
        conn, _, err := websocket.DefaultDialer.Dial(
            fmt.Sprintf("%s?user_id=user%d&role=student", wsURL, i), nil)
        require.NoError(t, err)
        connections[i] = conn
    }
    
    // Start graceful shutdown
    shutdownStart := time.Now()
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    shutdownDone := make(chan error, 1)
    go func() {
        shutdownDone <- app.Stop(shutdownCtx)
    }()
    
    // Verify connections receive close messages
    for i, conn := range connections {
        _, _, err := conn.ReadMessage()
        if err != nil {
            // Should be a close error
            if websocket.IsCloseError(err, websocket.CloseGoingAway) {
                t.Logf("Connection %d closed gracefully", i)
            } else {
                t.Logf("Connection %d closed with error: %v", i, err)
            }
        }
        conn.Close()
    }
    
    // Wait for shutdown to complete
    select {
    case err := <-shutdownDone:
        shutdownDuration := time.Since(shutdownStart)
        assert.NoError(t, err)
        assert.Less(t, shutdownDuration, 10*time.Second, "Shutdown took too long")
        t.Logf("Graceful shutdown completed in %v", shutdownDuration)
        
    case <-shutdownCtx.Done():
        t.Fatal("Shutdown timeout exceeded")
    }
}
```

---

## Phase 5 Success Criteria

### ARCHITECTURAL VALIDATION ✓
- [ ] HTTP API uses SessionLifecycle interface from Phase 2
- [ ] Main application integrates all components with proper dependency injection
- [ ] Configuration loading from YAML files
- [ ] Graceful shutdown follows multi-phase pattern from tech specs

### FUNCTIONAL VALIDATION ✓
- [ ] Session API creates and ends sessions with exact response formats
- [ ] WebSocket connections work end-to-end with message processing
- [ ] Static test client serves and connects successfully
- [ ] Graceful shutdown completes within timeout limits

### TECHNICAL VALIDATION ✓
- [ ] Complete system passes end-to-end integration tests
- [ ] HTTP API returns proper status codes and error formats
- [ ] Graceful shutdown prevents data loss and connection leaks
- [ ] Application starts successfully with `make run`

### SYSTEM INTEGRATION ✓
- [ ] All 5 phases integrate successfully in complete application
- [ ] Session management works through HTTP API and WebSocket state
- [ ] Message processing pipeline works end-to-end with role-based filtering
- [ ] Connection management handles multiple concurrent users
- [ ] Database persistence works with batching and retry logic
- [ ] Test client provides functional testing interface

**Phase 5 completes the Switchboard V4 system with production-ready HTTP API, system integration, and operational capabilities.**