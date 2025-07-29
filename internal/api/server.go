package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	sessionpkg "switchboard/internal/session"
	"switchboard/internal/websocket"
	"switchboard/pkg/interfaces"
	"switchboard/pkg/types"
)

// Registry interface to avoid tight coupling to websocket.Registry implementation
type Registry interface {
	GetSessionConnections(sessionID string) []*websocket.Connection
	GetStats() map[string]int
	BroadcastToUsers(userIDs []string, message interface{})
	// Auto-transition methods for Phase 4
	GetLobbyConnections() []*websocket.Connection
	TransitionUserToSession(userID, sessionID string) error
	TransitionUserToLobby(userID string) error
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
	sessionID := strings.Split(path, "/")[0]
	
	log.Printf("DEBUG: handleSessionByID - path: %s, sessionID: %s, method: %s", path, sessionID, r.Method)
	
	if sessionID == "" || sessionID == "api" {
		s.sendError(w, "Session ID required", http.StatusBadRequest)
		return
	}
	
	switch r.Method {
	case http.MethodGet:
		// SPECIAL CASE: Handle /api/sessions/active to get the current active session
		if sessionID == "active" {
			log.Printf("🔍 DEBUG: API request for active session")
			s.getActiveSession(w, r)
		} else {
			log.Printf("🔍 DEBUG: API request for specific session: %s", sessionID)
			s.getSession(w, r, sessionID)
		}
	case http.MethodDelete:
		// SPECIAL CASE: Handle /api/sessions/active to end the current active session
		if sessionID == "active" {
			log.Printf("🚩 DEBUG: API request to end active session")
			s.endActiveSession(w, r)
		} else {
			s.endSession(w, r, sessionID)
		}
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

// ActiveSessionConflictResponse represents the 409 error response
// FUNCTIONAL DISCOVERY: Detailed error response guides teacher workflow
type ActiveSessionConflictResponse struct {
	Error         string         `json:"error"`
	Message       string         `json:"message"`
	ActiveSession *types.Session `json:"active_session"`
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
		// ARCHITECTURAL DISCOVERY: Handle single session enforcement at API layer
		if errors.Is(err, sessionpkg.ErrActiveSessionExists) {
			s.sendActiveSessionConflictError(w, r.Context())
			return
		}
		if strings.Contains(err.Error(), "validation") {
			s.sendError(w, err.Error(), http.StatusBadRequest)
		} else {
			s.sendError(w, "Failed to create session", http.StatusInternalServerError)
		}
		return
	}
	
	// AUTO-TRANSITION: Move enrolled students from lobby to session
	lobbyConnections := s.registry.GetLobbyConnections()
	var transitionedUsers []string
	
	for _, conn := range lobbyConnections {
		if conn == nil {
			continue
		}
		
		userID := conn.GetUserID()
		role := conn.GetRole()
		
		// Auto-transition logic consistent with auto-assignment (Phase 3)
		if role == "instructor" {
			// Instructors have universal access to active sessions
			if err := s.registry.TransitionUserToSession(userID, session.ID); err != nil {
				log.Printf("WARNING: Failed to auto-transition instructor %s to session %s: %v", userID, session.ID, err)
			} else {
				transitionedUsers = append(transitionedUsers, userID)
				log.Printf("🏫 DEBUG: Auto-transitioned instructor %s from lobby to session %s", userID, session.ID)
			}
		} else if role == "student" {
			// Check if student is enrolled in this session
			for _, enrolledStudentID := range session.StudentIDs {
				if enrolledStudentID == userID {
					// Transition student from lobby to session
					if err := s.registry.TransitionUserToSession(userID, session.ID); err != nil {
						log.Printf("WARNING: Failed to auto-transition student %s to session %s: %v", userID, session.ID, err)
					} else {
						transitionedUsers = append(transitionedUsers, userID)
						log.Printf("🎓 DEBUG: Auto-transitioned student %s from lobby to session %s", userID, session.ID)
					}
					break
				}
			}
		}
	}
	
	// Send transition notifications to users who were moved
	if len(transitionedUsers) > 0 {
		transitionMsg := map[string]interface{}{
			"type":    "system",
			"context": "session_transition",
			"content": map[string]interface{}{
				"session_id":   session.ID,
				"session_name": session.Name,
				"reason":       "Auto-transitioned from lobby to session",
				"timestamp":    time.Now(),
			},
		}
		s.registry.BroadcastToUsers(transitionedUsers, transitionMsg)
		log.Printf("Auto-transitioned %d students from lobby to session %s", len(transitionedUsers), session.ID)
	}

	// LOBBY SYSTEM: Broadcast session_started to enrolled users
	sessionStartedMsg := map[string]interface{}{
		"type":    "system",
		"context": "session_started",
		"content": map[string]interface{}{
			"session_id":   session.ID,
			"session_name": session.Name,
			"instructor_id": session.CreatedBy,
			"student_ids":  session.StudentIDs,
			"timestamp":    time.Now(),
		},
	}
	
	// Create list of all session participants (instructor + students)
	allParticipants := make([]string, 0, len(session.StudentIDs)+1)
	allParticipants = append(allParticipants, session.CreatedBy) // instructor
	allParticipants = append(allParticipants, session.StudentIDs...) // students
	
	// Broadcast to all enrolled participants who are currently connected
	s.registry.BroadcastToUsers(allParticipants, sessionStartedMsg)
	log.Printf("Broadcast session_started for session %s to %d participants", session.ID, len(allParticipants))
	
	// FUNCTIONAL DISCOVERY: Return 201 Created with session data
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(CreateSessionResponse{Session: session}); err != nil {
		log.Printf("Failed to encode session creation response: %v", err)
	}
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
	
	if err := json.NewEncoder(w).Encode(SessionResponse{
		Session:         session,
		ConnectionCount: connectionCount,
	}); err != nil {
		log.Printf("Failed to encode session response: %v", err)
	}
}

// FUNCTIONAL DISCOVERY: DELETE /api/sessions/{id} - End session
func (s *Server) endSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	log.Printf("🚩 DEBUG: Attempting to end session - sessionID: %s", sessionID)
	
	// Get session information first to find all participants
	session, err := s.sessionManager.GetSession(r.Context(), sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.sendError(w, "Session not found", http.StatusNotFound)
		} else {
			s.sendError(w, "Failed to get session", http.StatusInternalServerError)
		}
		return
	}
	
	// LOBBY SYSTEM: Send session_left to all session participants (like session_started)
	sessionLeftMsg := map[string]interface{}{
		"type":    "system",
		"context": "session_left",
		"content": map[string]interface{}{
			"session_id":   sessionID,
			"session_name": session.Name,
			"reason":       "Session ended by instructor",
			"timestamp":    time.Now(),
		},
	}
	
	// Create list of all session participants (instructor + students)
	allParticipants := make([]string, 0, len(session.StudentIDs)+1)
	allParticipants = append(allParticipants, session.CreatedBy) // instructor
	allParticipants = append(allParticipants, session.StudentIDs...) // students
	
	// Broadcast to all enrolled participants who are currently connected (regardless of their current session)
	s.registry.BroadcastToUsers(allParticipants, sessionLeftMsg)
	log.Printf("📢 DEBUG: Broadcast session_left for session %s to %d participants", sessionID, len(allParticipants))
	
	// CRITICAL FIX: Transition all participants from session back to lobby
	// This ensures WebSocket connections are properly updated when session ends
	for _, userID := range allParticipants {
		if err := s.registry.TransitionUserToLobby(userID); err != nil {
			log.Printf("WARNING: Failed to transition user %s to lobby: %v", userID, err)
		}
	}
	log.Printf("🏛️ DEBUG: Transitioned all participants from session %s to lobby", sessionID)
	
	err = s.sessionManager.EndSession(r.Context(), sessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.sendError(w, "Session not found", http.StatusNotFound)
		} else {
			s.sendError(w, "Failed to end session", http.StatusInternalServerError)
		}
		return
	}
	log.Printf("✅ DEBUG: Session ended successfully via API - sessionID: %s", sessionID)
	
	// FUNCTIONAL DISCOVERY: Return simple success response
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "Session ended successfully"}); err != nil {
		log.Printf("Failed to encode session end response: %v", err)
	}
}

// FUNCTIONAL DISCOVERY: GET /api/sessions/active - Get the current active session
func (s *Server) getActiveSession(w http.ResponseWriter, r *http.Request) {
	session, err := s.sessionManager.GetActiveSession(r.Context())
	if err != nil {
		s.sendError(w, "Failed to get active session", http.StatusInternalServerError)
		return
	}
	
	if session == nil {
		// No active session - return 404 as expected by client
		s.sendError(w, "No active session", http.StatusNotFound)
		return
	}
	
	// Include current connection count from registry
	connections := s.registry.GetSessionConnections(session.ID)
	connectionCount := len(connections)
	
	if err := json.NewEncoder(w).Encode(SessionResponse{
		Session:         session,
		ConnectionCount: connectionCount,
	}); err != nil {
		log.Printf("Failed to encode active session response: %v", err)
	}
	
	log.Printf("✅ DEBUG: Returned active session via API - sessionID: %s", session.ID)
}

// FUNCTIONAL DISCOVERY: DELETE /api/sessions/active - End the current active session
// ARCHITECTURAL DISCOVERY: Simplifies client by removing need to track session IDs
func (s *Server) endActiveSession(w http.ResponseWriter, r *http.Request) {
	// Get the current active session
	activeSession, err := s.sessionManager.GetActiveSession(r.Context())
	if err != nil {
		log.Printf("❌ DEBUG: Failed to get active session: %v", err)
		s.sendError(w, "Failed to get active session", http.StatusInternalServerError)
		return
	}
	
	// IDEMPOTENCY: If no active session, return success
	if activeSession == nil {
		log.Printf("✅ DEBUG: No active session to end - idempotent success")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"message": "No active session to end"}); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
		return
	}
	
	// Delegate to existing endSession logic
	log.Printf("🎯 DEBUG: Found active session %s, delegating to endSession", activeSession.ID)
	s.endSession(w, r, activeSession.ID)
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
	
	if err := json.NewEncoder(w).Encode(ListSessionsResponse{Sessions: sessionsWithConnections}); err != nil {
		log.Printf("Failed to encode sessions list response: %v", err)
	}
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
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode health check response: %v", err)
	}
}

// FUNCTIONAL DISCOVERY: Consistent error response format
func (s *Server) sendError(w http.ResponseWriter, message string, code int) {
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(ErrorResponse{
		Error:   http.StatusText(code),
		Code:    code,
		Message: message,
	}); err != nil {
		log.Printf("Failed to encode error response: %v", err)
	}
}

// sendActiveSessionConflictError sends a 409 response with active session details
// FUNCTIONAL DISCOVERY: Detailed error response guides teacher workflow
func (s *Server) sendActiveSessionConflictError(w http.ResponseWriter, ctx context.Context) {
	// Get the active session details
	activeSession, err := s.sessionManager.GetActiveSession(ctx)
	if err != nil {
		// Fallback to generic error if we can't get session details
		s.sendError(w, "Cannot create session: active session exists", http.StatusConflict)
		return
	}
	
	if activeSession == nil {
		// This shouldn't happen, but handle gracefully
		s.sendError(w, "Cannot create session: active session exists", http.StatusConflict)
		return
	}
	
	// Create detailed conflict response
	message := fmt.Sprintf("Cannot create new session. Active session '%s' must be ended first.", activeSession.Name)
	
	w.WriteHeader(http.StatusConflict)
	if err := json.NewEncoder(w).Encode(ActiveSessionConflictResponse{
		Error:         "Active session exists",
		Message:       message,
		ActiveSession: activeSession,
	}); err != nil {
		log.Printf("Failed to encode active session conflict response: %v", err)
	}
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