package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"switchboard/internal/database"
	"switchboard/internal/hub"
	"switchboard/internal/router"
	"switchboard/internal/session"
	"switchboard/internal/websocket"
	dbconfig "switchboard/pkg/database"
)

// TestSystemInitialization tests that all components can be initialized together
func TestSystemInitialization(t *testing.T) {
	// Create temporary database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "integration_test.db")

	config := &dbconfig.Config{
		DatabasePath:    dbPath,
		MaxConnections:  10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: time.Minute,
	}

	// Initialize database manager
	dbManager, err := database.NewManager(config)
	require.NoError(t, err)
	defer func() {
		if err := dbManager.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()

	// Run migrations
	migrationManager := dbconfig.NewMigrationManager(dbManager.GetDB(), "../../migrations")
	err = migrationManager.ApplyMigrations()
	require.NoError(t, err)

	// Initialize session manager
	sessionManager := session.NewManager(dbManager)

	// Initialize registry
	registry := websocket.NewRegistry()

	// Initialize router
	router := router.NewRouter(registry, dbManager)

	// Initialize hub
	hub := hub.NewHub(registry, router)
	defer func() {
		if err := hub.Stop(); err != nil {
			t.Logf("Failed to stop hub: %v", err)
		}
	}()

	// Verify all components are properly initialized
	assert.NotNil(t, dbManager)
	assert.NotNil(t, sessionManager)
	assert.NotNil(t, registry)
	assert.NotNil(t, router)
	assert.NotNil(t, hub)

	// Test database health check
	err = dbManager.HealthCheck(context.Background())
	assert.NoError(t, err)
}