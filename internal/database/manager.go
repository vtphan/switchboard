package database

// DatabaseManager provides unified interface for all database operations
// This interface defines the contract for database operations supporting
// the single-writer pattern and batched operations as specified in tech specs.
type DatabaseManager interface {
	// Session operations
	CreateSession(session *Session) error
	UpdateSession(session *Session) error
	GetActiveSession() (*Session, error)
	
	// Message operations  
	WriteMessage(msg *Message) error
	WriteBatch(msgs []*Message) error
	GetSessionMessages(sessionID string) ([]*Message, error)
	
	// Lifecycle
	Start() error
	Stop() error
	
	// Synchronization for testing
	WaitForPendingWrites() error
}