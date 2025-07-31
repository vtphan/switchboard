package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"switchboard/pkg/config"
	"switchboard/pkg/errors"
)

// SQLiteDatabaseManager implements DatabaseManager interface with single-writer pattern
type SQLiteDatabaseManager struct {
	db              *sql.DB
	writeChannel    chan writeRequest
	deadLetterQueue chan writeRequest
	metrics         *DatabaseMetrics
	stopCh          chan struct{}
	wg              sync.WaitGroup
	started         atomic.Bool
}

// writeRequest represents a database write operation request
type writeRequest struct {
	operation  string
	data       interface{}
	responseCh chan error
}

// DatabaseMetrics tracks database operation statistics
type DatabaseMetrics struct {
	SuccessfulWrites    int64
	FailedWrites        int64
	RetriedWrites       int64
	DeadLetterCount     int64
	PermanentLossCount  int64
}

// SQLiteOptions contains options for creating a SQLiteDatabaseManager
type SQLiteOptions struct {
	SkipTableInit bool // Skip table initialization (useful when tables are already created)
	SkipPragmas   bool // Skip PRAGMA configuration (useful in tests)
}

// NewSQLiteDatabaseManager creates a new SQLite database manager
func NewSQLiteDatabaseManager(db *sql.DB) (*SQLiteDatabaseManager, error) {
	return NewSQLiteDatabaseManagerWithOptions(db, SQLiteOptions{})
}

// NewSQLiteDatabaseManagerWithOptions creates a new SQLite database manager with custom options
func NewSQLiteDatabaseManagerWithOptions(db *sql.DB, opts SQLiteOptions) (*SQLiteDatabaseManager, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection cannot be nil")
	}

	manager := &SQLiteDatabaseManager{
		db:              db,
		writeChannel:    make(chan writeRequest, config.DatabaseWriteBuffer),
		deadLetterQueue: make(chan writeRequest, config.DeadLetterQueueSize),
		metrics:         &DatabaseMetrics{},
		stopCh:          make(chan struct{}),
	}

	// Apply SQLite configuration unless skipped
	if !opts.SkipPragmas {
		if err := manager.applySQLiteConfig(); err != nil {
			return nil, fmt.Errorf("failed to apply SQLite configuration: %w", err)
		}
	}

	// Initialize tables unless skipped
	if !opts.SkipTableInit {
		if err := manager.initializeTables(); err != nil {
			return nil, fmt.Errorf("failed to initialize tables: %w", err)
		}
	}

	return manager, nil
}

// Start initializes the single-writer pattern goroutines
func (s *SQLiteDatabaseManager) Start() error {
	if s.started.Swap(true) {
		// Already started, return without error (idempotent)
		return nil
	}

	// Start write loop goroutine
	s.wg.Add(1)
	go s.writeLoop()

	// Start dead letter queue processor
	s.wg.Add(1)
	go s.deadLetterProcessor()

	log.Printf("SQLiteDatabaseManager started with single-writer pattern")
	return nil
}

// Stop gracefully shuts down all goroutines
func (s *SQLiteDatabaseManager) Stop() error {
	if !s.started.Swap(false) {
		// Already stopped, return without error (idempotent)
		return nil
	}

	// Signal shutdown
	close(s.stopCh)

	// Wait for all goroutines to finish
	s.wg.Wait()

	log.Printf("SQLiteDatabaseManager stopped gracefully")
	return nil
}

// CreateSession creates a new session in the database
func (s *SQLiteDatabaseManager) CreateSession(session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	if err := session.Validate(); err != nil {
		return err
	}

	responseCh := make(chan error, 1)
	request := writeRequest{
		operation:  "create_session",
		data:       session,
		responseCh: responseCh,
	}

	select {
	case s.writeChannel <- request:
		return <-responseCh
	case <-s.stopCh:
		return fmt.Errorf("database manager is shutting down")
	}
}

// UpdateSession updates an existing session in the database
func (s *SQLiteDatabaseManager) UpdateSession(session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	if err := session.Validate(); err != nil {
		return err
	}

	responseCh := make(chan error, 1)
	request := writeRequest{
		operation:  "update_session",
		data:       session,
		responseCh: responseCh,
	}

	select {
	case s.writeChannel <- request:
		return <-responseCh
	case <-s.stopCh:
		return fmt.Errorf("database manager is shutting down")
	}
}

// GetActiveSession retrieves the currently active session
func (s *SQLiteDatabaseManager) GetActiveSession() (*Session, error) {
	query := `SELECT id, name, created_by, start_time, end_time, status 
	          FROM sessions WHERE status = 'active' ORDER BY start_time DESC LIMIT 1`

	row := s.db.QueryRow(query)

	var session Session
	var endTime sql.NullString
	err := row.Scan(&session.ID, &session.Name, &session.CreatedBy, 
		&session.StartTime, &endTime, &session.Status)

	if err == sql.ErrNoRows {
		return nil, nil // No active session found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}

	// Handle nullable end_time
	if endTime.Valid {
		if t, err := time.Parse(time.RFC3339, endTime.String); err == nil {
			session.EndTime = &t
		}
	}

	return &session, nil
}

// WriteMessage writes a single message to the database
func (s *SQLiteDatabaseManager) WriteMessage(msg *Message) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	if err := msg.Validate(); err != nil {
		return err
	}

	responseCh := make(chan error, 1)
	request := writeRequest{
		operation:  "write_message",
		data:       msg,
		responseCh: responseCh,
	}

	select {
	case s.writeChannel <- request:
		return <-responseCh
	case <-s.stopCh:
		return fmt.Errorf("database manager is shutting down")
	}
}

// WriteBatch writes multiple messages to the database in a single transaction
func (s *SQLiteDatabaseManager) WriteBatch(msgs []*Message) error {
	if len(msgs) == 0 {
		return nil // No messages to write
	}

	// Validate all messages
	for _, msg := range msgs {
		if msg == nil {
			return fmt.Errorf("message in batch cannot be nil")
		}
		if err := msg.Validate(); err != nil {
			return err
		}
	}

	responseCh := make(chan error, 1)
	request := writeRequest{
		operation:  "write_batch",
		data:       msgs,
		responseCh: responseCh,
	}

	select {
	case s.writeChannel <- request:
		return <-responseCh
	case <-s.stopCh:
		return fmt.Errorf("database manager is shutting down")
	}
}

// GetSessionMessages retrieves all messages for a given session
func (s *SQLiteDatabaseManager) GetSessionMessages(sessionID string) ([]*Message, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	query := `SELECT id, session_id, type, context, from_user, to_user, content, timestamp 
	          FROM messages WHERE session_id = ? ORDER BY timestamp ASC`

	rows, err := s.db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query session messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var messages []*Message
	for rows.Next() {
		var msg Message
		var toUser sql.NullString
		var contentJSON string

		err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Type, &msg.Context,
			&msg.FromUser, &toUser, &contentJSON, &msg.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}

		// Handle nullable to_user
		if toUser.Valid {
			msg.ToUser = &toUser.String
		}

		// Parse JSON content
		if err := json.Unmarshal([]byte(contentJSON), &msg.Content); err != nil {
			return nil, fmt.Errorf("failed to parse message content JSON: %w", err)
		}

		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating message rows: %w", err)
	}

	return messages, nil
}

// writeLoop is the single-writer goroutine that processes all database writes
func (s *SQLiteDatabaseManager) writeLoop() {
	defer s.wg.Done()

	for {
		select {
		case request := <-s.writeChannel:
			s.processWriteRequest(request)
		case <-s.stopCh:
			// Process remaining requests before shutdown
			s.drainWriteChannel()
			return
		}
	}
}

// processWriteRequest processes a single write request with smart retry logic
func (s *SQLiteDatabaseManager) processWriteRequest(request writeRequest) {
	var err error

	// First attempt
	err = s.executeWriteOperation(request)
	if err == nil {
		atomic.AddInt64(&s.metrics.SuccessfulWrites, 1)
		request.responseCh <- nil
		return
	}

	// Check if error should not be retried (business logic constraints, etc.)
	if s.isNonRetryableError(err) {
		atomic.AddInt64(&s.metrics.FailedWrites, 1)
		request.responseCh <- err
		return
	}

	// Retry logic with exponential backoff for retryable errors only
	for retry := 1; retry <= config.DatabaseMaxRetries; retry++ {
		atomic.AddInt64(&s.metrics.RetriedWrites, 1)
		backoffDuration := time.Duration(100*retry*retry) * time.Millisecond
		time.Sleep(backoffDuration)

		err = s.executeWriteOperation(request)
		if err == nil {
			atomic.AddInt64(&s.metrics.SuccessfulWrites, 1)
			request.responseCh <- nil
			return
		}

		// Check again if error became non-retryable
		if s.isNonRetryableError(err) {
			atomic.AddInt64(&s.metrics.FailedWrites, 1)
			request.responseCh <- err
			return
		}

		// Only log on the last retry attempt to reduce noise
		if retry == config.DatabaseMaxRetries {
			log.Printf("Database write failed after %d attempts: %v", retry+1, err)
		}
	}

	// All retries failed, send to dead letter queue only if error is potentially recoverable
	atomic.AddInt64(&s.metrics.FailedWrites, 1)
	
	// Don't send non-retryable errors to dead letter queue - they will never succeed
	if s.isNonRetryableError(err) {
		atomic.AddInt64(&s.metrics.PermanentLossCount, 1)
		request.responseCh <- err
		return
	}
	
	select {
	case s.deadLetterQueue <- request:
		atomic.AddInt64(&s.metrics.DeadLetterCount, 1)
		request.responseCh <- err
	default:
		// Dead letter queue is full, permanent loss
		atomic.AddInt64(&s.metrics.PermanentLossCount, 1)
		request.responseCh <- fmt.Errorf("write failed and dead letter queue full: %w", err)
	}
}

// isNonRetryableError determines if an error should not be retried
// These are typically business logic constraints or permanent failures
func (s *SQLiteDatabaseManager) isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	// Session management business logic errors should not be retried
	if err == errors.ErrSessionAlreadyActive || err == errors.ErrNoActiveSession {
		return true
	}
	
	// Other non-retryable database constraint violations
	errorStr := err.Error()
	nonRetryablePatterns := []string{
		"UNIQUE constraint failed",
		"FOREIGN KEY constraint failed", 
		"CHECK constraint failed",
		"NOT NULL constraint failed",
	}
	
	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}
	
	return false
}

// executeWriteOperation executes the actual database operation
func (s *SQLiteDatabaseManager) executeWriteOperation(request writeRequest) error {
	switch request.operation {
	case "create_session":
		return s.createSessionInDB(request.data.(*Session))
	case "update_session":
		return s.updateSessionInDB(request.data.(*Session))
	case "write_message":
		return s.writeMessageInDB(request.data.(*Message))
	case "write_batch":
		return s.writeBatchInDB(request.data.([]*Message))
	case "sync_barrier":
		// No-op operation used for synchronization
		return nil
	case "sync_barrier_with_checkpoint":
		// Force a WAL checkpoint to ensure all writes are visible to readers
		_, err := s.db.Exec("PRAGMA wal_checkpoint(FULL)")
		return err
	default:
		return fmt.Errorf("unknown write operation: %s", request.operation)
	}
}

// createSessionInDB creates a session in the database
func (s *SQLiteDatabaseManager) createSessionInDB(session *Session) error {
	query := `INSERT INTO sessions (id, name, created_by, start_time, end_time, status) 
	          VALUES (?, ?, ?, ?, ?, ?)`

	var endTime interface{}
	if session.EndTime != nil {
		endTime = session.EndTime.Format(time.RFC3339)
	}

	_, err := s.db.Exec(query, session.ID, session.Name, session.CreatedBy,
		session.StartTime.Format(time.RFC3339), endTime, session.Status)

	if err != nil {
		// Check for UNIQUE constraint violation on active session status
		// This is a business logic constraint, not a transient error
		if strings.Contains(err.Error(), "UNIQUE constraint failed: sessions.status") {
			return errors.ErrSessionAlreadyActive
		}
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// updateSessionInDB updates a session in the database
func (s *SQLiteDatabaseManager) updateSessionInDB(session *Session) error {
	query := `UPDATE sessions SET name = ?, created_by = ?, start_time = ?, end_time = ?, status = ? 
	          WHERE id = ?`

	var endTime interface{}
	if session.EndTime != nil {
		endTime = session.EndTime.Format(time.RFC3339)
	}

	result, err := s.db.Exec(query, session.Name, session.CreatedBy,
		session.StartTime.Format(time.RFC3339), endTime, session.Status, session.ID)

	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

// writeMessageInDB writes a single message to the database
func (s *SQLiteDatabaseManager) writeMessageInDB(msg *Message) error {
	query := `INSERT OR REPLACE INTO messages (id, session_id, type, context, from_user, to_user, content, timestamp) 
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	contentJSON, err := json.Marshal(msg.Content)
	if err != nil {
		return fmt.Errorf("failed to marshal message content: %w", err)
	}

	var toUser interface{}
	if msg.ToUser != nil {
		toUser = *msg.ToUser
	}

	_, err = s.db.Exec(query, msg.ID, msg.SessionID, msg.Type, msg.Context,
		msg.FromUser, toUser, string(contentJSON), msg.Timestamp.Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// writeBatchInDB writes multiple messages in a single transaction
func (s *SQLiteDatabaseManager) writeBatchInDB(msgs []*Message) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query := `INSERT OR REPLACE INTO messages (id, session_id, type, context, from_user, to_user, content, timestamp) 
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare batch statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, msg := range msgs {
		contentJSON, err := json.Marshal(msg.Content)
		if err != nil {
			return fmt.Errorf("failed to marshal message content: %w", err)
		}

		var toUser interface{}
		if msg.ToUser != nil {
			toUser = *msg.ToUser
		}

		_, err = stmt.Exec(msg.ID, msg.SessionID, msg.Type, msg.Context,
			msg.FromUser, toUser, string(contentJSON), msg.Timestamp.Format(time.RFC3339))
		if err != nil {
			return fmt.Errorf("failed to execute batch statement: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit batch transaction: %w", err)
	}

	return nil
}

// deadLetterProcessor handles failed writes from the dead letter queue
func (s *SQLiteDatabaseManager) deadLetterProcessor() {
	defer s.wg.Done()

	for {
		select {
		case request := <-s.deadLetterQueue:
			// Try to process dead letter request one more time
			err := s.executeWriteOperation(request)
			if err != nil {
				log.Printf("Dead letter processing failed permanently: %v", err)
				atomic.AddInt64(&s.metrics.PermanentLossCount, 1)
			} else {
				log.Printf("Dead letter request recovered successfully")
				atomic.AddInt64(&s.metrics.SuccessfulWrites, 1)
			}
		case <-s.stopCh:
			return
		}
	}
}

// drainWriteChannel processes remaining write requests during shutdown
func (s *SQLiteDatabaseManager) drainWriteChannel() {
	for {
		select {
		case request := <-s.writeChannel:
			s.processWriteRequest(request)
		default:
			return
		}
	}
}

// applySQLiteConfig applies SQLite optimization settings using the configuration constant
// This is graceful - it won't fail if PRAGMAs are already set or unsupported
func (s *SQLiteDatabaseManager) applySQLiteConfig() error {
	// Check current journal mode to avoid redundant PRAGMA execution
	var currentMode string
	err := s.db.QueryRow("PRAGMA journal_mode").Scan(&currentMode)
	if err == nil && currentMode == "wal" {
		log.Printf("SQLite already in WAL mode, skipping PRAGMA configuration")
		return nil
	}
	
	// Apply the SQLite configuration from config package
	// Split the configuration into individual statements and execute them
	statements := strings.Split(strings.TrimSpace(config.SQLiteConfiguration), "\n")
	
	for _, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		
		if _, err := s.db.Exec(statement); err != nil {
			// These are optimizations, not critical for functionality
			// Just log warnings for failed pragmas
			log.Printf("Warning: Failed to apply pragma '%s': %v (continuing anyway)", statement, err)
		}
	}
	
	return nil
}

// initializeTables creates database tables using the migrations
// This is idempotent and safe to call multiple times
func (s *SQLiteDatabaseManager) initializeTables() error {
	// Check if tables already exist before attempting to create them
	var tableCount int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='table' AND name IN ('sessions', 'messages')
	`).Scan(&tableCount)
	
	if err != nil {
		return fmt.Errorf("failed to check existing tables: %w", err)
	}
	
	// If both tables exist, skip initialization
	if tableCount >= 2 {
		log.Printf("Database tables already exist, skipping initialization")
		return nil
	}
	
	// Only create tables/indexes if they don't exist
	// This is safe to run even if partially executed before
	migrations := []string{
		// Sessions table
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL CHECK (length(name) >= 1 AND length(name) <= 200),
			created_by TEXT NOT NULL,
			start_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			end_time DATETIME,
			status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'ended'))
		)`,
		
		// Messages table
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			type TEXT NOT NULL CHECK (type IN ('broadcast_to_instructors', 'direct_message', 'broadcast_to_students')),
			context TEXT NOT NULL DEFAULT 'general' CHECK (length(context) >= 1 AND length(context) <= 50),
			from_user TEXT NOT NULL CHECK (length(from_user) >= 1 AND length(from_user) <= 50),
			to_user TEXT,
			content TEXT NOT NULL CHECK (length(content) <= 65536),
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		)`,
		
		// Indexes
		`CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_start_time ON sessions(start_time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_session_time ON messages(session_id, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_type_context ON messages(type, context)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_to_user ON messages(to_user) WHERE to_user IS NOT NULL`,
		
		// Unique constraint to prevent multiple active sessions
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_active_session 
		 ON sessions(status) 
		 WHERE status = 'active'`,
	}
	
	// Execute each migration statement individually for better error handling
	for i, migration := range migrations {
		if _, err := s.db.Exec(migration); err != nil {
			return fmt.Errorf("failed to execute migration %d: %w", i, err)
		}
	}
	
	log.Printf("Database tables initialized successfully")
	return nil
}

// GetMetrics returns the current database metrics
func (s *SQLiteDatabaseManager) GetMetrics() *DatabaseMetrics {
	return &DatabaseMetrics{
		SuccessfulWrites:    atomic.LoadInt64(&s.metrics.SuccessfulWrites),
		FailedWrites:        atomic.LoadInt64(&s.metrics.FailedWrites),
		RetriedWrites:       atomic.LoadInt64(&s.metrics.RetriedWrites),
		DeadLetterCount:     atomic.LoadInt64(&s.metrics.DeadLetterCount),
		PermanentLossCount:  atomic.LoadInt64(&s.metrics.PermanentLossCount),
	}
}

// WaitForPendingWrites waits for all pending database writes to complete
// This method is primarily intended for testing to ensure data consistency
func (s *SQLiteDatabaseManager) WaitForPendingWrites() error {
	if !s.started.Load() {
		return fmt.Errorf("database manager is not started")
	}

	// Send a barrier request that will also force a WAL checkpoint
	responseCh := make(chan error, 1)
	request := writeRequest{
		operation:  "sync_barrier_with_checkpoint",
		data:       nil,
		responseCh: responseCh,
	}

	select {
	case s.writeChannel <- request:
		return <-responseCh
	case <-s.stopCh:
		return fmt.Errorf("database manager is shutting down")
	}
}