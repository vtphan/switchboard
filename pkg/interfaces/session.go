package interfaces

import (
	"context"
	"switchboard/pkg/types"
)

// SessionManager handles session lifecycle operations
// ARCHITECTURAL DISCOVERY: Context-first design pattern ensures proper
// cancellation and timeout handling across all session operations
type SessionManager interface {
	// CreateSession creates a new session
	// FUNCTIONAL DISCOVERY: Student ID list validation occurs at manager level
	// to ensure consistent business rules across different creation paths
	CreateSession(ctx context.Context, name string, createdBy string, studentIDs []string) (*types.Session, error)

	// GetSession retrieves a session by ID
	// ARCHITECTURAL DISCOVERY: Returns pointer to enable efficient caching
	// and reduce memory allocation for frequently accessed sessions
	GetSession(ctx context.Context, sessionID string) (*types.Session, error)

	// EndSession ends an active session
	// FUNCTIONAL DISCOVERY: Session termination updates both database and cache
	// atomically to prevent stale session access after termination
	EndSession(ctx context.Context, sessionID string) error

	// ListActiveSessions returns all active sessions
	// TECHNICAL DISCOVERY: Returns slice of pointers for memory efficiency
	// when dealing with classroom-scale session counts (10-50 sessions)
	ListActiveSessions(ctx context.Context) ([]*types.Session, error)

	// ValidateSessionMembership checks if user can join session
	// ARCHITECTURAL DISCOVERY: Role-based validation abstracted to interface
	// enables different validation strategies (cache-first, database-only, etc.)
	ValidateSessionMembership(sessionID, userID, role string) error
}