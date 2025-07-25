package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"switchboard/pkg/interfaces"
	"switchboard/pkg/types"
	"switchboard/internal/websocket"
)

// Registry interface to avoid tight coupling to websocket.Registry implementation
type Registry interface {
	GetSessionConnections(sessionID string) []*websocket.Connection
	GetStats() map[string]int
}

// ARCHITECTURAL DISCOVERY: HTTP API layer serves as pure interface between external clients and internal components
// Clean separation - no business logic, only HTTP handling and JSON serialization
type Server struct {
	sessionManager interfaces.SessionManager
	dbManager      interfaces.DatabaseManager
	registry       Registry
	router         *http.ServeMux
}

// FUNCTIONAL DISCOVERY: Constructor initializes all dependencies and sets up routing
// Dependency injection pattern maintains architectural boundaries
func NewServer(sessionManager interfaces.SessionManager, dbManager interfaces.DatabaseManager, registry Registry) *Server {
	s := &Server{
		sessionManager: sessionManager,
		dbManager:      dbManager,
		registry:       registry,
		router:         http.NewServeMux(),
	}
	
	s.setupRoutes()
	return s
}

// ARCHITECTURAL DISCOVERY: Route setup follows REST conventions with proper middleware
// CORS and JSON middleware applied to all routes for web client compatibility
func (s *Server) setupRoutes() {
	// Apply middleware to all routes
	s.router.Handle("/api/sessions", s.corsMiddleware(s.jsonMiddleware(http.HandlerFunc(s.handleSessions))))
	s.router.Handle("/api/sessions/", s.corsMiddleware(s.jsonMiddleware(http.HandlerFunc(s.handleSessionByID))))
	s.router.Handle("/health", s.corsMiddleware(s.jsonMiddleware(http.HandlerFunc(s.healthCheck))))
}

// FUNCTIONAL DISCOVERY: Implement http.Handler interface for integration with standard HTTP server
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// FUNCTIONAL DISCOVERY: Handle sessions collection endpoints (POST /api/sessions, GET /api/sessions)
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createSession(w, r)
	case http.MethodGet:
		s.listSessions(w, r)
	case http.MethodOptions:
		// CORS preflight handled by middleware
		w.WriteHeader(http.StatusOK)
	default:
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// FUNCTIONAL DISCOVERY: Handle individual session endpoints (GET /api/sessions/{id}, DELETE /api/sessions/{id})
func (s *Server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if path == "" {
		s.sendError(w, "Session ID required", http.StatusBadRequest)
		return
	}
	
	sessionID := strings.Split(path, "/")[0]
	if sessionID == "" {
		s.sendError(w, "Invalid session ID", http.StatusBadRequest)
		return
	}
	
	switch r.Method {
	case http.MethodGet:
		s.getSession(w, r, sessionID)
	case http.MethodDelete:
		s.endSession(w, r, sessionID)
	case http.MethodOptions:
		// CORS preflight handled by middleware
		w.WriteHeader(http.StatusOK)
	default:
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Request/Response types for JSON serialization
type CreateSessionRequest struct {
	Name         string   `json:"name"`
	InstructorID string   `json:"instructor_id"`
	StudentIDs   []string `json:"student_ids"`
}

type CreateSessionResponse struct {
	Session *types.Session `json:"session"`
}

type SessionResponse struct {
	Session         *types.Session `json:"session"`
	ConnectionCount int           `json:"connection_count"`
}

type ListSessionsResponse struct {
	Sessions []SessionWithConnections `json:"sessions"`
}

type SessionWithConnections struct {
	*types.Session
	ConnectionCount int `json:"connection_count"`
}

type HealthResponse struct {
	Status      string                 `json:"status"`
	Timestamp   time.Time             `json:"timestamp"`
	Database    string                `json:"database"`
	Connections map[string]int        `json:"connections"`
	System      map[string]interface{} `json:"system"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// FUNCTIONAL DISCOVERY: POST /api/sessions - Create new session with duplicate student ID removal
func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	// FUNCTIONAL DISCOVERY: Validate required fields
	if req.Name == "" {
		s.sendError(w, "Session name is required", http.StatusBadRequest)
		return
	}
	if req.InstructorID == "" {
		s.sendError(w, "Instructor ID is required", http.StatusBadRequest)
		return
	}
	if len(req.StudentIDs) == 0 {
		s.sendError(w, "At least one student ID is required", http.StatusBadRequest)
		return
	}
	
	// FUNCTIONAL DISCOVERY: Create session through SessionManager (handles duplicate removal)
	session, err := s.sessionManager.CreateSession(r.Context(), req.Name, req.InstructorID, req.StudentIDs)
	if err != nil {
		if strings.Contains(err.Error(), "validation") {
			s.sendError(w, err.Error(), http.StatusBadRequest)
		} else {
			s.sendError(w, "Failed to create session", http.StatusInternalServerError)
		}
		return
	}
	
	// FUNCTIONAL DISCOVERY: Return 201 Created with session data
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(CreateSessionResponse{Session: session})
}

// FUNCTIONAL DISCOVERY: GET /api/sessions/{id} - Get session details with connection count
func (s *Server) getSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	session, err := s.sessionManager.GetSession(r.Context(), sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.sendError(w, "Session not found", http.StatusNotFound)
		} else {
			s.sendError(w, "Failed to get session", http.StatusInternalServerError)
		}
		return
	}
	
	// FUNCTIONAL DISCOVERY: Include current connection count from registry
	connections := s.registry.GetSessionConnections(sessionID)
	connectionCount := len(connections)
	
	json.NewEncoder(w).Encode(SessionResponse{
		Session:         session,
		ConnectionCount: connectionCount,
	})
}

// FUNCTIONAL DISCOVERY: DELETE /api/sessions/{id} - End session
func (s *Server) endSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	log.Printf("DEBUG: endSession() called for sessionID: %s", sessionID)
	
	// Notify all connected clients before ending the session
	connections := s.registry.GetSessionConnections(sessionID)
	log.Printf("DEBUG: GetSessionConnections() returned %d connections for session %s", len(connections), sessionID)
	
	if len(connections) > 0 {
		sessionEndedMsg := map[string]interface{}{
			"type":    "system",
			"context": "session_ended",
			"content": map[string]interface{}{
				"event":  "session_ended",
				"reason": "Session ended by instructor",
			},
		}
		
		log.Printf("DEBUG: Prepared session_ended message: %+v", sessionEndedMsg)
		
		// Send session_ended message to all connected clients
		successCount := 0
		for i, conn := range connections {
			log.Printf("DEBUG: Attempting to send session_ended to connection %d", i)
			if err := conn.WriteJSON(sessionEndedMsg); err != nil {
				log.Printf("ERROR: Failed to send session_ended to client %d: %v", i, err)
			} else {
				successCount++
				log.Printf("DEBUG: Successfully sent session_ended to connection %d", i)
			}
		}
		log.Printf("SUCCESS: Sent session_ended message to %d/%d connected clients", successCount, len(connections))
	} else {
		log.Printf("WARNING: No connections found for session %s - cannot send session_ended message", sessionID)
	}
	
	err := s.sessionManager.EndSession(r.Context(), sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.sendError(w, "Session not found", http.StatusNotFound)
		} else if strings.Contains(err.Error(), "already ended") {
			s.sendError(w, "Session already ended", http.StatusBadRequest)
		} else {
			s.sendError(w, "Failed to end session", http.StatusInternalServerError)
		}
		return
	}
	
	// FUNCTIONAL DISCOVERY: Return simple success response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Session ended successfully"})
}

// FUNCTIONAL DISCOVERY: GET /api/sessions - List active sessions with connection counts
func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.sessionManager.ListActiveSessions(r.Context())
	if err != nil {
		s.sendError(w, "Failed to list sessions", http.StatusInternalServerError)
		return
	}
	
	// FUNCTIONAL DISCOVERY: Enhance with connection counts from registry
	sessionsWithConnections := make([]SessionWithConnections, len(sessions))
	for i, session := range sessions {
		connections := s.registry.GetSessionConnections(session.ID)
		sessionsWithConnections[i] = SessionWithConnections{
			Session:         session,
			ConnectionCount: len(connections),
		}
	}
	
	json.NewEncoder(w).Encode(ListSessionsResponse{Sessions: sessionsWithConnections})
}

// FUNCTIONAL DISCOVERY: GET /health - System health check with component validation
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	
	status := "healthy"
	dbStatus := "healthy"
	
	// FUNCTIONAL DISCOVERY: Check database connectivity
	if err := s.dbManager.HealthCheck(ctx); err != nil {
		status = "unhealthy"
		dbStatus = fmt.Sprintf("error: %v", err)
	}
	
	// FUNCTIONAL DISCOVERY: Get connection statistics from registry
	connectionStats := s.registry.GetStats()
	
	// FUNCTIONAL DISCOVERY: Include system information
	systemInfo := map[string]interface{}{
		"goroutines": "not_implemented", // Would use runtime.NumGoroutine() in production
		"memory":     "not_implemented", // Would use runtime.MemStats in production
		"uptime":     "not_implemented", // Would track application start time
	}
	
	response := HealthResponse{
		Status:      status,
		Timestamp:   time.Now(),
		Database:    dbStatus,
		Connections: connectionStats,
		System:      systemInfo,
	}
	
	// FUNCTIONAL DISCOVERY: Return 503 if any component is unhealthy
	if status == "unhealthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	
	json.NewEncoder(w).Encode(response)
}

// FUNCTIONAL DISCOVERY: Consistent error response format
func (s *Server) sendError(w http.ResponseWriter, message string, code int) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   http.StatusText(code),
		Code:    code,
		Message: message,
	})
}

// ARCHITECTURAL DISCOVERY: CORS middleware enables web client access
// Allows all origins in development - would be restricted in production
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// FUNCTIONAL DISCOVERY: Set CORS headers for web client compatibility
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		
		// FUNCTIONAL DISCOVERY: Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// FUNCTIONAL DISCOVERY: JSON middleware ensures proper content-type headers
func (s *Server) jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// FUNCTIONAL DISCOVERY: Set JSON content type for all API responses
		w.Header().Set("Content-Type", "application/json")
		
		next.ServeHTTP(w, r)
	})
}