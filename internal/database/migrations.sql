-- Switchboard V4 Database Schema
-- Ultra-simple sessions and messages tables

-- Sessions table
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK (length(name) >= 1 AND length(name) <= 200),
    created_by TEXT NOT NULL,
    start_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    end_time DATETIME,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'ended'))
);

-- Messages table  
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('broadcast_to_instructors', 'direct_message', 'broadcast_to_students')),
    context TEXT NOT NULL DEFAULT 'general' CHECK (length(context) >= 1 AND length(context) <= 50),
    from_user TEXT NOT NULL CHECK (length(from_user) >= 1 AND length(from_user) <= 50),
    to_user TEXT,
    content TEXT NOT NULL CHECK (length(content) <= 65536),
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_start_time ON sessions(start_time DESC);
CREATE INDEX IF NOT EXISTS idx_messages_session_time ON messages(session_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_type_context ON messages(type, context);
CREATE INDEX IF NOT EXISTS idx_messages_to_user ON messages(to_user) WHERE to_user IS NOT NULL;

-- Unique constraint to prevent multiple active sessions
-- This partial index ensures only one session can be active at a time
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_active_session 
ON sessions(status) 
WHERE status = 'active';

-- SQLite optimization pragmas
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = -64000;
PRAGMA wal_autocheckpoint = 1000;