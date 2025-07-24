package interfaces

// Connection represents a WebSocket client connection interface
// ARCHITECTURAL DISCOVERY: Pure abstraction without implementation details
// ensures clean boundaries between WebSocket infrastructure and business logic
type Connection interface {
	// WriteJSON sends a JSON message to the client (thread-safe)
	// FUNCTIONAL DISCOVERY: Thread-safety requirement documented in interface
	// to ensure all implementations use single-writer pattern to prevent races
	WriteJSON(v interface{}) error

	// Close closes the connection and cleans up resources
	// ARCHITECTURAL DISCOVERY: Resource cleanup abstracted to interface level
	// allows different connection implementations (WebSocket, mock, etc.)
	Close() error

	// GetUserID returns the connected user's ID
	// FUNCTIONAL DISCOVERY: User identification needed for message routing
	// and session validation throughout the system
	GetUserID() string

	// GetRole returns the user's role ("student" or "instructor")
	// FUNCTIONAL DISCOVERY: Role-based access control requires role information
	// to be accessible at connection level for permission checks
	GetRole() string

	// GetSessionID returns the session ID this connection belongs to
	// ARCHITECTURAL DISCOVERY: Session scoping at connection level enables
	// efficient message routing and session-based cleanup
	GetSessionID() string

	// IsAuthenticated returns true if connection is authenticated
	// FUNCTIONAL DISCOVERY: Authentication state tracking prevents unauthorized
	// access and enables proper connection lifecycle management
	IsAuthenticated() bool

	// SetCredentials sets user credentials after authentication
	// TECHNICAL DISCOVERY: Separate authentication step allows WebSocket
	// upgrade before credential validation, improving connection establishment
	SetCredentials(userID, role, sessionID string) error
}