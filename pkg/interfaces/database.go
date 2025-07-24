package interfaces

import (
	"context"
	"switchboard/pkg/types"
)

// DatabaseManager handles all database operations
// ARCHITECTURAL DISCOVERY: Single interface for all persistence operations
// enables consistent transaction handling and connection management
type DatabaseManager interface {
	// Session operations
	// FUNCTIONAL DISCOVERY: Separate Create vs Update methods enable different
	// validation and error handling for new sessions vs existing modifications

	// CreateSession creates a new session in the database
	// TECHNICAL DISCOVERY: Takes Session pointer to avoid large struct copying
	// and enable efficient field validation before persistence
	CreateSession(ctx context.Context, session *types.Session) error

	// GetSession retrieves a session by ID from the database
	// ARCHITECTURAL DISCOVERY: Context enables query timeout and cancellation
	// for database operations that may block during high load
	GetSession(ctx context.Context, sessionID string) (*types.Session, error)

	// UpdateSession updates an existing session (primarily for ending sessions)
	// FUNCTIONAL DISCOVERY: Session pointer enables partial updates while
	// maintaining data integrity through validation
	UpdateSession(ctx context.Context, session *types.Session) error

	// ListActiveSessions returns all active sessions from the database
	// TECHNICAL DISCOVERY: Returns slice of pointers for memory efficiency
	// when loading multiple sessions for cache initialization
	ListActiveSessions(ctx context.Context) ([]*types.Session, error)

	// Message operations
	// ARCHITECTURAL DISCOVERY: Message operations grouped with session operations
	// in single interface to enable transaction coordination

	// StoreMessage persists a message to the database
	// FUNCTIONAL DISCOVERY: Message storage must complete before routing
	// to ensure audit trail and message history integrity
	StoreMessage(ctx context.Context, message *types.Message) error

	// GetSessionHistory retrieves all messages for a session
	// TECHNICAL DISCOVERY: Returns message slice ordered by timestamp
	// for efficient history replay during WebSocket connection setup
	GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error)

	// Health and lifecycle operations
	// ARCHITECTURAL DISCOVERY: Health checking and lifecycle management
	// grouped with data operations for comprehensive database status

	// HealthCheck verifies database connectivity and basic operations
	// FUNCTIONAL DISCOVERY: Context enables health check timeout to prevent
	// hanging health checks from blocking application startup
	HealthCheck(ctx context.Context) error

	// Close closes the database connection and cleans up resources
	// TECHNICAL DISCOVERY: Synchronous close ensures all pending operations
	// complete before application shutdown
	Close() error
}