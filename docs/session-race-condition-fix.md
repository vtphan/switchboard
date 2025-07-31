# Session Creation Race Condition Fix

## Problem Description

The original implementation had a race condition in session creation where multiple goroutines could create sessions simultaneously. The issue occurred because:

1. In-memory state was checked and set first
2. Database persistence happened second
3. Multiple goroutines could pass the in-memory check before any database writes occurred

## Solution: Database-First Approach

The fix implements a database-first approach with the following key changes:

### 1. Database Constraint
Added a unique partial index to the database schema:
```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_active_session 
ON sessions(status) 
WHERE status = 'active';
```

This ensures only one session can have `status = 'active'` at the database level.

### 2. Reversed Operation Order

**Before (Race Condition Present):**
1. Check in-memory state
2. Set in-memory state
3. Write to database (could fail if another goroutine already wrote)
4. Rollback in-memory state on failure

**After (Race Condition Fixed):**
1. Write to database (atomic operation, constraint enforced)
2. Update in-memory state only after successful database write
3. Handle any in-memory sync issues with logging

### 3. Implementation Details

The `SessionLifecycle.StartSession` method now:
- Attempts database insertion first
- The database constraint prevents multiple active sessions
- Updates in-memory state only after successful database operation
- Treats database as the source of truth

```go
// Step 4: Database-first atomic creation
err := sl.dbManager.CreateSession(session)
if err != nil {
    // Database constraint prevented creation
    return nil, errors.ErrSessionAlreadyActive
}

// Step 5: Update in-memory state after successful database write
err = sl.sessionManager.SetActiveSession(session)
```

### 4. Error Handling Improvements

- Proper error propagation when database operations fail
- Recovery mechanism for in-memory/database state mismatches
- Comprehensive logging for debugging
- Clear distinction between expected errors (session already active) and unexpected errors

## Testing

The fix includes comprehensive tests:

1. **Extreme Concurrency Test**: 100 goroutines attempting simultaneous session creation
2. **Rapid Cycling Test**: Multiple create/end cycles with concurrent operations
3. **Database Constraint Test**: Direct verification of constraint enforcement
4. **Recovery Test**: Handling of in-memory/database state mismatches

## Performance Impact

- Minimal performance impact as database operations were already required
- Better consistency guarantees outweigh the slight reordering overhead
- SQLite's WAL mode ensures good concurrent read performance

## Backward Compatibility

The fix maintains full backward compatibility:
- Same public API for SessionLifecycle
- Same error types returned
- Existing code continues to work without modifications