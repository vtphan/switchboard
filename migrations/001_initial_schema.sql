-- Version 001: Initial schema
-- Creates the foundational tables for sessions and messages
-- ARCHITECTURAL DISCOVERY: Schema exactly matches pkg/types struct definitions
-- to ensure seamless JSON marshaling and database operations

-- Sessions table
-- FUNCTIONAL DISCOVERY: student_ids stored as JSON array enables flexible
-- session membership without requiring separate junction table
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_by TEXT NOT NULL,
    student_ids TEXT NOT NULL, -- JSON array of strings
    start_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    end_time DATETIME,
    status TEXT NOT NULL DEFAULT 'active',
    CHECK (status IN ('active', 'ended')),
    CHECK (length(name) >= 1 AND length(name) <= 200)
);

-- Messages table  
-- TECHNICAL DISCOVERY: All 6 message types constrained at database level
-- to prevent invalid message types from entering the system
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    type TEXT NOT NULL,
    context TEXT NOT NULL DEFAULT 'general',
    from_user TEXT NOT NULL,
    to_user TEXT, -- NULL for broadcasts
    content TEXT NOT NULL, -- JSON, max 64KB enforced by application
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    CHECK (type IN ('instructor_inbox', 'inbox_response', 'request', 'request_response', 'analytics', 'instructor_broadcast')),
    CHECK (length(context) >= 1 AND length(context) <= 50)
);

-- Performance indexes
-- ARCHITECTURAL DISCOVERY: Compound indexes optimized for specific query patterns
-- from WebSocket history replay and session management operations

-- Session status lookup for active session listing
CREATE INDEX idx_sessions_status ON sessions(status);

-- Session ownership lookup for instructor access control
CREATE INDEX idx_sessions_created_by ON sessions(created_by);

-- Message history retrieval ordered by timestamp (most common query)
CREATE INDEX idx_messages_session_time ON messages(session_id, timestamp);

-- Message type filtering for role-based message access
CREATE INDEX idx_messages_session_type ON messages(session_id, type);

-- Direct message recipient lookup (only for non-broadcast messages)
CREATE INDEX idx_messages_to_user ON messages(to_user) WHERE to_user IS NOT NULL;