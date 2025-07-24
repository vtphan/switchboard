package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
	
	// ARCHITECTURAL DISCOVERY: Import SQLite driver but only reference in connection string
	_ "github.com/mattn/go-sqlite3"
	"switchboard/pkg/interfaces"
	"switchboard/pkg/types"
	dbconfig "switchboard/pkg/database"
)

// Manager implements the DatabaseManager interface
type Manager struct {
	db           *sql.DB
	config       *dbconfig.Config
	writeChannel chan writeOperation  // TECHNICAL: Single-writer pattern for SQLite
	shutdown     chan struct{}
	wg           sync.WaitGroup
	closed       bool
	mu           sync.RWMutex  // TECHNICAL: Protect closed status
}

// writeOperation represents a database write operation
type writeOperation struct {
	operation func(*sql.DB) error
	result    chan error
}

// NewManager creates a new database manager
func NewManager(config *dbconfig.Config) (*Manager, error) {
	// ARCHITECTURAL DISCOVERY: SQLite connection string includes optimizations from Phase 1
	// Open database connection with SQLite-specific optimizations
	db, err := sql.Open("sqlite3", config.DatabasePath+"?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	// FUNCTIONAL DISCOVERY: Connection pool configuration critical for concurrent reads
	// Configure connection pool for concurrent read access
	db.SetMaxOpenConns(config.MaxConnections)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	
	// Apply SQLite optimizations from Phase 1 configuration
	if err := applySQLiteOptimizations(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to apply SQLite optimizations: %w", err)
	}
	
	manager := &Manager{
		db:           db,
		config:       config,
		writeChannel: make(chan writeOperation, 100), // TECHNICAL: Buffer for write operations prevents blocking
		shutdown:     make(chan struct{}),
	}
	
	// ARCHITECTURAL DISCOVERY: Single-writer goroutine prevents SQLite write contention
	// Start single-writer goroutine - critical for SQLite performance
	manager.wg.Add(1)
	go manager.writeLoop()
	
	return manager, nil
}

// writeLoop processes all write operations in a single goroutine
func (m *Manager) writeLoop() {
	defer m.wg.Done()
	
	for {
		select {
		case op := <-m.writeChannel:
			// FUNCTIONAL DISCOVERY: Retry logic exactly once after 5 seconds as specified
			err := op.operation(m.db)
			if err != nil {
				log.Printf("Database write failed, retrying in 5 seconds: %v", err)
				time.Sleep(5 * time.Second)
				err = op.operation(m.db) // Retry once
				if err != nil {
					log.Printf("Database write failed after retry: %v", err)
				}
			}
			op.result <- err
			
		case <-m.shutdown:
			log.Println("Database write loop shutting down")
			return
		}
	}
}

// executeWrite queues a write operation and waits for completion
func (m *Manager) executeWrite(operation func(*sql.DB) error) error {
	// TECHNICAL DISCOVERY: Check if manager is closed before attempting write
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return fmt.Errorf("database manager is closed")
	}
	m.mu.RUnlock()
	
	result := make(chan error, 1)
	
	select {
	case m.writeChannel <- writeOperation{operation: operation, result: result}:
		return <-result
	case <-time.After(30 * time.Second):
		return fmt.Errorf("write operation timeout")
	case <-m.shutdown:
		return fmt.Errorf("database manager is shutting down")
	}
}

// CreateSession creates a new session in the database
func (m *Manager) CreateSession(ctx context.Context, session *types.Session) error {
	return m.executeWrite(func(db *sql.DB) error {
		// FUNCTIONAL DISCOVERY: Transaction support essential for atomic session operations
		// Begin transaction for atomic session creation
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }() // TECHNICAL: Always rollback unless commit succeeds
		
		// TECHNICAL DISCOVERY: JSON serialization for student IDs maintains schema flexibility
		// Serialize student IDs to JSON for database storage
		studentIDsJSON, err := json.Marshal(session.StudentIDs)
		if err != nil {
			return fmt.Errorf("failed to marshal student IDs: %w", err)
		}
		
		// Insert session with all required fields
		query := `
			INSERT INTO sessions (id, name, created_by, student_ids, start_time, status)
			VALUES (?, ?, ?, ?, ?, ?)
		`
		_, err = tx.ExecContext(ctx, query,
			session.ID,
			session.Name,
			session.CreatedBy,
			string(studentIDsJSON),
			session.StartTime,
			session.Status,
		)
		if err != nil {
			return fmt.Errorf("failed to insert session: %w", err)
		}
		
		// FUNCTIONAL DISCOVERY: Commit required for transaction completion
		// Commit transaction - session creation is atomic
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit session creation: %w", err)
		}
		
		return nil
	})
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	// ARCHITECTURAL DISCOVERY: Read operations can be concurrent - no need for writeChannel
	query := `
		SELECT id, name, created_by, student_ids, start_time, end_time, status
		FROM sessions
		WHERE id = ?
	`
	
	row := m.db.QueryRowContext(ctx, query, sessionID)
	
	var session types.Session
	var studentIDsJSON string
	var endTime sql.NullTime
	
	err := row.Scan(
		&session.ID,
		&session.Name,
		&session.CreatedBy,
		&studentIDsJSON,
		&session.StartTime,
		&endTime,
		&session.Status,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// FUNCTIONAL DISCOVERY: Return specific error type for session not found
			return nil, interfaces.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to query session: %w", err)
	}
	
	// TECHNICAL DISCOVERY: JSON deserialization restores student ID slice
	// Deserialize student IDs from JSON storage
	if err := json.Unmarshal([]byte(studentIDsJSON), &session.StudentIDs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal student IDs: %w", err)
	}
	
	// FUNCTIONAL DISCOVERY: Handle nullable end_time field properly
	// Handle nullable end_time
	if endTime.Valid {
		session.EndTime = &endTime.Time
	}
	
	return &session, nil
}

// UpdateSession updates an existing session
func (m *Manager) UpdateSession(ctx context.Context, session *types.Session) error {
	return m.executeWrite(func(db *sql.DB) error {
		// FUNCTIONAL DISCOVERY: Update only end_time and status during session termination
		query := `
			UPDATE sessions
			SET end_time = ?, status = ?
			WHERE id = ?
		`
		
		_, err := db.ExecContext(ctx, query,
			session.EndTime,
			session.Status,
			session.ID,
		)
		if err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}
		
		return nil
	})
}

// ListActiveSessions returns all active sessions
func (m *Manager) ListActiveSessions(ctx context.Context) ([]*types.Session, error) {
	// ARCHITECTURAL DISCOVERY: Read operations concurrent, ordered by start_time DESC for recency
	query := `
		SELECT id, name, created_by, student_ids, start_time, end_time, status
		FROM sessions
		WHERE status = 'active'
		ORDER BY start_time DESC
	`
	
	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var sessions []*types.Session
	
	for rows.Next() {
		var session types.Session
		var studentIDsJSON string
		var endTime sql.NullTime
		
		err := rows.Scan(
			&session.ID,
			&session.Name,
			&session.CreatedBy,
			&studentIDsJSON,
			&session.StartTime,
			&endTime,
			&session.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session row: %w", err)
		}
		
		// TECHNICAL DISCOVERY: JSON deserialization for each session in list
		// Deserialize student IDs
		if err := json.Unmarshal([]byte(studentIDsJSON), &session.StudentIDs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal student IDs: %w", err)
		}
		
		// Handle nullable end_time
		if endTime.Valid {
			session.EndTime = &endTime.Time
		}
		
		sessions = append(sessions, &session)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating session rows: %w", err)
	}
	
	return sessions, nil
}

// StoreMessage stores a message in the database
func (m *Manager) StoreMessage(ctx context.Context, message *types.Message) error {
	return m.executeWrite(func(db *sql.DB) error {
		// TECHNICAL DISCOVERY: JSON serialization for message content enables flexible payloads
		// Serialize message content to JSON
		contentJSON, err := json.Marshal(message.Content)
		if err != nil {
			return fmt.Errorf("failed to marshal message content: %w", err)
		}
		
		// FUNCTIONAL DISCOVERY: Handle nullable to_user field for different message types
		query := `
			INSERT INTO messages (id, session_id, type, context, from_user, to_user, content, timestamp)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`
		
		_, err = db.ExecContext(ctx, query,
			message.ID,
			message.SessionID,
			message.Type,
			message.Context,
			message.FromUser,
			message.ToUser,
			string(contentJSON),
			message.Timestamp,
		)
		if err != nil {
			return fmt.Errorf("failed to insert message: %w", err)
		}
		
		return nil
	})
}

// GetSessionHistory retrieves all messages for a session
func (m *Manager) GetSessionHistory(ctx context.Context, sessionID string) ([]*types.Message, error) {
	// FUNCTIONAL DISCOVERY: Order by timestamp ASC for chronological message history
	query := `
		SELECT id, session_id, type, context, from_user, to_user, content, timestamp
		FROM messages
		WHERE session_id = ?
		ORDER BY timestamp ASC
	`
	
	rows, err := m.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query session history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	
	var messages []*types.Message
	
	for rows.Next() {
		var message types.Message
		var contentJSON string
		var toUser sql.NullString
		
		err := rows.Scan(
			&message.ID,
			&message.SessionID,
			&message.Type,
			&message.Context,
			&message.FromUser,
			&toUser,
			&contentJSON,
			&message.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		
		// FUNCTIONAL DISCOVERY: Handle nullable to_user field for broadcast vs targeted messages
		// Handle nullable to_user
		if toUser.Valid {
			message.ToUser = &toUser.String
		}
		
		// TECHNICAL DISCOVERY: JSON deserialization restores message content structure
		// Deserialize message content
		if err := json.Unmarshal([]byte(contentJSON), &message.Content); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message content: %w", err)
		}
		
		messages = append(messages, &message)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating message rows: %w", err)
	}
	
	return messages, nil
}

// HealthCheck validates database connectivity
func (m *Manager) HealthCheck(ctx context.Context) error {
	// FUNCTIONAL DISCOVERY: Health check validates both connectivity and basic operations
	// Test database connectivity
	if err := m.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	
	// Test read operation to verify database is accessible
	_, err := m.db.QueryContext(ctx, "SELECT COUNT(*) FROM sessions LIMIT 1")
	if err != nil {
		return fmt.Errorf("database read test failed: %w", err)
	}
	
	return nil
}

// GetDB returns the underlying database connection for migrations
func (m *Manager) GetDB() *sql.DB {
	return m.db
}

// Close shuts down the database manager
func (m *Manager) Close() error {
	// TECHNICAL DISCOVERY: Prevent multiple close operations
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil // Already closed
	}
	m.closed = true
	m.mu.Unlock()
	
	// ARCHITECTURAL DISCOVERY: Graceful shutdown requires careful goroutine coordination
	// Signal shutdown to writeLoop
	close(m.shutdown)
	m.wg.Wait() // Wait for write loop to finish processing
	
	// Close database connection
	if err := m.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}
	
	return nil
}

// applySQLiteOptimizations applies performance optimizations
func applySQLiteOptimizations(db *sql.DB) error {
	// TECHNICAL DISCOVERY: SQLite pragmas from Phase 1 configuration for classroom performance
	pragmas := []string{
		"PRAGMA journal_mode = WAL",          // Write-Ahead Logging for concurrency
		"PRAGMA synchronous = NORMAL",        // Balance safety and performance
		"PRAGMA cache_size = -64000",         // 64MB cache for classroom scale
		"PRAGMA temp_store = MEMORY",         // Use memory for temporary tables
		"PRAGMA foreign_keys = ON",           // Ensure referential integrity
		"PRAGMA busy_timeout = 5000",         // 5 second timeout for write coordination
	}
	
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to execute pragma %s: %w", pragma, err)
		}
	}
	
	return nil
}