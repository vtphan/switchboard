package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"switchboard/internal/session"
	"switchboard/pkg/errors"
)

// SessionAPIHandler handles HTTP session management endpoints following the exact
// implementation pattern specified in phase-5.md lines 35-174.
type SessionAPIHandler struct {
	sessionLifecycle session.SessionLifecycleInterface
}

// NewSessionAPIHandler creates a new SessionAPIHandler instance with required dependencies
func NewSessionAPIHandler(sessionLifecycle session.SessionLifecycleInterface) *SessionAPIHandler {
	return &SessionAPIHandler{
		sessionLifecycle: sessionLifecycle,
	}
}

// StartSession handles POST /api/session/start requests to create and activate a new session
// following the exact API specification from tech specs lines 640-657.
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
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid JSON format")
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
		if err == errors.ErrSessionAlreadyActive {
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
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// EndSession handles POST /api/session/end requests to end the current active session
// following the exact API specification from tech specs lines 662-672.
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
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid JSON format")
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
		if err == errors.ErrNoActiveSession {
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
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// writeErrorResponse writes a standardized error response following the exact format
// specified in the tech specs.
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
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}