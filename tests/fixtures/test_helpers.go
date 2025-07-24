package fixtures

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"switchboard/internal/database"  
	"switchboard/internal/session"
	pkgdatabase "switchboard/pkg/database"
	"switchboard/pkg/types"
)

// TestSession represents a test session with automatic cleanup
type TestSession struct {
	SessionID    string
	Session      *types.Session
	DatabasePath string
	DbManager    *database.Manager
	SessionMgr   *session.Manager
	cleanup      func() error
}

// SetupCleanSession creates a test session with complete cleanup capability
func SetupCleanSession(t *testing.T, name string, instructorID string, studentIDs []string) *TestSession {
	// Create temporary database file with unique test identifier
	tmpDir := os.TempDir()
	testID := fmt.Sprintf("%s_%d_%d", t.Name(), time.Now().UnixNano(), os.Getpid())
	// Replace invalid filename characters
	dbPath := filepath.Join(tmpDir, fmt.Sprintf("switchboard_test_%x.db", []byte(testID)))
	
	// Initialize database manager
	dbConfig := &pkgdatabase.Config{
		DatabasePath:    dbPath,
		MaxConnections:  10,
		ConnMaxLifetime: 30 * time.Second,
		ConnMaxIdleTime: 10 * time.Second,
		MigrationsPath:  "../../migrations", // Relative to test location
	}
	
	dbManager, err := database.NewManager(dbConfig)
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}
	
	// Apply database migrations
	migrationManager := pkgdatabase.NewMigrationManager(dbManager.GetDB(), dbConfig.MigrationsPath)
	if err := migrationManager.ApplyMigrations(); err != nil {
		dbManager.Close()
		os.Remove(dbPath)
		t.Fatalf("Failed to apply migrations: %v", err)
	}
	
	// Initialize session manager
	sessionMgr := session.NewManager(dbManager)
	if err := sessionMgr.LoadActiveSessions(context.Background()); err != nil {
		t.Fatalf("Failed to load active sessions: %v", err)
	}
	
	// Create the test session
	session, err := sessionMgr.CreateSession(context.Background(), name, instructorID, studentIDs)
	if err != nil {
		dbManager.Close()
		os.Remove(dbPath)
		t.Fatalf("Failed to create test session: %v", err)
	}
	
	testSession := &TestSession{
		SessionID:    session.ID,
		Session:      session,
		DatabasePath: dbPath,
		DbManager:    dbManager,
		SessionMgr:   sessionMgr,
		cleanup: func() error {
			// Close database connections
			if err := dbManager.Close(); err != nil {
				return fmt.Errorf("failed to close database: %w", err)
			}
			
			// Remove temporary database file
			if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove database file: %w", err)
			}
			
			return nil
		},
	}
	
	// Register cleanup with testing framework
	t.Cleanup(func() {
		if err := testSession.CleanupAll(); err != nil {
			t.Errorf("Test cleanup failed: %v", err)
		}
	})
	
	return testSession
}

// CleanupAll performs complete cleanup of database, connections, and temporary files
func (ts *TestSession) CleanupAll() error {
	if ts.cleanup != nil {
		return ts.cleanup()
	}
	return nil
}

// GetMessageCount returns the number of messages in the test session
func (ts *TestSession) GetMessageCount() (int, error) {
	ctx := context.Background()
	messages, err := ts.DbManager.GetSessionHistory(ctx, ts.SessionID)
	if err != nil {
		return 0, err
	}
	return len(messages), nil
}

// ValidateMessageFlow compares expected vs actual message sequences
func ValidateMessageFlow(t *testing.T, expected, actual []*types.Message) {
	if len(expected) != len(actual) {
		t.Fatalf("Message count mismatch: expected %d, got %d", len(expected), len(actual)) 
	}
	
	for i, expectedMsg := range expected {
		actualMsg := actual[i]
		
		// Compare relevant fields (ignore server-generated fields like ID, timestamp)
		if expectedMsg.Type != actualMsg.Type {
			t.Errorf("Message %d type mismatch: expected %s, got %s", i, expectedMsg.Type, actualMsg.Type)
		}
		
		if expectedMsg.Context != actualMsg.Context {
			t.Errorf("Message %d context mismatch: expected %s, got %s", i, expectedMsg.Context, actualMsg.Context)
		}
		
		if expectedMsg.FromUser != actualMsg.FromUser {
			t.Errorf("Message %d from_user mismatch: expected %s, got %s", i, expectedMsg.FromUser, actualMsg.FromUser)
		}
		
		if (expectedMsg.ToUser == nil && actualMsg.ToUser != nil) || 
		   (expectedMsg.ToUser != nil && actualMsg.ToUser == nil) ||
		   (expectedMsg.ToUser != nil && actualMsg.ToUser != nil && *expectedMsg.ToUser != *actualMsg.ToUser) {
			expectedStr := "<nil>"
			actualStr := "<nil>" 
			if expectedMsg.ToUser != nil {
				expectedStr = *expectedMsg.ToUser
			}
			if actualMsg.ToUser != nil {
				actualStr = *actualMsg.ToUser
			}
			t.Errorf("Message %d to_user mismatch: expected %s, got %s", i, expectedStr, actualStr)
		}
	}
}

// WaitForCondition waits for a condition to be met with timeout
func WaitForCondition(condition func() bool, timeout time.Duration, interval time.Duration) bool {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(interval)
	}
	
	return false
}

// AssertEventuallyTrue waits for a condition with testing context
func AssertEventuallyTrue(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	if !WaitForCondition(condition, timeout, 10*time.Millisecond) {
		t.Fatalf("Condition not met within %v: %s", timeout, message)
	}
}

// CreateTempDatabase creates a temporary database file for testing
func CreateTempDatabase(t *testing.T) (string, func()) {
	tmpDir := os.TempDir()
	dbPath := filepath.Join(tmpDir, fmt.Sprintf("switchboard_test_%d.db", time.Now().UnixNano()))
	
	cleanup := func() {
		if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
			t.Errorf("Failed to remove temporary database: %v", err)
		}
	}
	
	t.Cleanup(cleanup)
	return dbPath, cleanup
}

// ValidateNoResourceLeaks checks for common resource leaks
func ValidateNoResourceLeaks(t *testing.T, beforeStats, afterStats map[string]int) {
	// Check connection count hasn't increased
	beforeConns := beforeStats["total_connections"]
	afterConns := afterStats["total_connections"]
	
	if afterConns > beforeConns {
		t.Errorf("Potential connection leak: before=%d, after=%d", beforeConns, afterConns)
	}
	
	// Add other resource validations as needed
}

// SimulateNetworkDelay adds realistic network delay to test timing
func SimulateNetworkDelay() {
	// Simulate typical classroom network latency (5-50ms)
	delay := time.Duration(5+time.Now().UnixNano()%45) * time.Millisecond
	time.Sleep(delay)
}

// GenerateUniqueUserID creates a unique user ID for testing
func GenerateUniqueUserID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}