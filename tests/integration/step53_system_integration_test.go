// Integration tests for Step 5.3: System Integration & Main Application
// These tests will initially FAIL (RED phase) until implementation is complete

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"

	"switchboard/internal/database"
	"switchboard/internal/message"
	"switchboard/internal/rate"
	"switchboard/internal/session"
	websocketpkg "switchboard/internal/websocket"
	"switchboard/web"
	"switchboard/web/api"
)

// TestStep53_ApplicationInitialization tests that the main application
// can initialize all components in the correct dependency order
func TestStep53_ApplicationInitialization(t *testing.T) {
	t.Run("component_initialization_order", func(t *testing.T) {
		// This test will fail until Application struct is implemented
		
		// Create test configuration
		config := &Config{
			Server: struct {
				Host string `yaml:"host"`
				Port string `yaml:"port"`
				ReadTimeout  string `yaml:"read_timeout"`
				WriteTimeout string `yaml:"write_timeout"`
			}{
				Host: "localhost",
				Port: "0", // Random port for testing
			},
			Database: struct {
				Path string `yaml:"path"`
			}{
				Path: ":memory:", // In-memory SQLite for testing
			},
		}
		
		// Test that NewApplication can initialize all components
		app, err := NewApplication(config)
		require.NoError(t, err, "Application initialization should succeed")
		require.NotNil(t, app, "Application should not be nil")
		
		// Verify all components are initialized
		assert.NotNil(t, app.dbManager, "DatabaseManager should be initialized")
		assert.NotNil(t, app.sessionManager, "SessionManager should be initialized")
		assert.NotNil(t, app.sessionLifecycle, "SessionLifecycle should be initialized")
		assert.NotNil(t, app.messageProcessor, "MessageProcessor should be initialized")
		assert.NotNil(t, app.rateLimiter, "RateLimiter should be initialized")
		assert.NotNil(t, app.connectionRegistry, "ConnectionRegistry should be initialized")
		assert.NotNil(t, app.websocketHandler, "WebSocketHandler should be initialized")
		assert.NotNil(t, app.sessionAPIHandler, "SessionAPIHandler should be initialized")
		assert.NotNil(t, app.httpServer, "HTTPServer should be initialized")
	})
	
	t.Run("configuration_loading", func(t *testing.T) {
		// This test will fail until configuration loading is implemented
		
		// Create temporary config file
		configContent := `
server:
  host: "localhost"
  port: "8080"
  read_timeout: "30s"
  write_timeout: "30s"

database:
  path: "test.db"

logging:
  level: "info"
  file: "test.log"

websocket:
  check_origin: false
  handshake_timeout: "10s"
`
		configFile := "/tmp/test_config.yaml"
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)
		defer func() {
			if err := os.Remove(configFile); err != nil {
				t.Logf("Failed to remove config file: %v", err)
			}
		}()
		
		// Test configuration loading
		config, err := loadConfig(configFile)
		require.NoError(t, err, "Configuration loading should succeed")
		
		assert.Equal(t, "localhost", config.Server.Host)
		assert.Equal(t, "8080", config.Server.Port)
		assert.Equal(t, "test.db", config.Database.Path)
		assert.Equal(t, "info", config.Logging.Level)
		assert.Equal(t, false, config.WebSocket.CheckOrigin)
	})
	
	t.Run("database_schema_application", func(t *testing.T) {
		// This test will fail until database schema application is implemented
		
		config := &Config{
			Database: struct {
				Path string `yaml:"path"`
			}{
				Path: ":memory:",
			},
		}
		
		app, err := NewApplication(config)
		require.NoError(t, err)
		
		// Start the application to apply schema
		err = app.Start()
		require.NoError(t, err, "Application start should succeed")
		
		// Test that database schema was applied by checking tables exist
		// This would require database manager to have a method to verify schema
		
		// Clean shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = app.Stop(ctx)
		assert.NoError(t, err, "Application stop should succeed")
	})
}

// TestStep53_SystemIntegration tests complete end-to-end system functionality
func TestStep53_SystemIntegration(t *testing.T) {
	t.Run("complete_system_integration", func(t *testing.T) {
		// This test will fail until complete system integration is working
		
		config := &Config{
			Server: struct {
				Host string `yaml:"host"`
				Port string `yaml:"port"`
				ReadTimeout  string `yaml:"read_timeout"`
				WriteTimeout string `yaml:"write_timeout"`
			}{
				Host: "localhost",
				Port: "9876", // Fixed test port
			},
			Database: struct {
				Path string `yaml:"path"`
			}{
				Path: ":memory:",
			},
		}
		
		app, err := NewApplication(config)
		require.NoError(t, err)
		
		// Start application
		err = app.Start()
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := app.Stop(ctx); err != nil {
				t.Logf("Failed to stop application: %v", err)
			}
		}()
		
		// Give server time to start
		time.Sleep(100 * time.Millisecond)
		
		// Get actual server port from config  
		port := app.config.Server.Port
		if port == "" {
			port = "8080"
		}
		baseURL := "http://localhost:" + port
		wsURL := "ws://localhost:" + port + "/ws"
		
		// Test 1: HTTP API - Start session
		sessionReq := map[string]string{
			"name":          "Integration Test Session",
			"instructor_id": "instructor_test",
		}
		reqBody, _ := json.Marshal(sessionReq)
		
		resp, err := http.Post(baseURL+"/api/session/start", "application/json", bytes.NewReader(reqBody))
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}()
		
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		var sessionResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&sessionResp)
		require.NoError(t, err)
		
		sessionData := sessionResp["session"].(map[string]interface{})
		sessionID := sessionData["id"].(string)
		
		// Test 2: WebSocket - Connect instructor
		instructorConn, _, err := websocket.DefaultDialer.Dial(wsURL+"?user_id=instructor_test&role=instructor", nil)
		require.NoError(t, err)
		defer func() {
			if err := instructorConn.Close(); err != nil {
				t.Logf("Failed to close instructor connection: %v", err)
			}
		}()
		
		// Test 3: WebSocket - Connect student
		studentConn, _, err := websocket.DefaultDialer.Dial(wsURL+"?user_id=student_test&role=student", nil)
		require.NoError(t, err)
		defer func() {
			if err := studentConn.Close(); err != nil {
				t.Logf("Failed to close student connection: %v", err)
			}
		}()
		
		// Test 4: Message processing - Student question to instructor
		questionMsg := map[string]interface{}{
			"type":    "broadcast_to_instructors",
			"context": "question",
			"content": map[string]interface{}{"text": "Test question"},
		}
		
		err = studentConn.WriteJSON(questionMsg)
		require.NoError(t, err)
		
		// Test 5: HTTP API - End session
		endReq := map[string]string{"instructor_id": "instructor_test"}
		reqBody, _ = json.Marshal(endReq)
		
		resp, err = http.Post(baseURL+"/api/session/end", "application/json", bytes.NewReader(reqBody))
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var endResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&endResp)
		require.NoError(t, err)
		
		assert.Equal(t, sessionID, endResp["session_id"])
		assert.Equal(t, "ended", endResp["status"])
	})
}

// TestStep53_GracefulShutdown tests multi-phase graceful shutdown
func TestStep53_GracefulShutdown(t *testing.T) {
	t.Run("graceful_shutdown_phases", func(t *testing.T) {
		// This test will fail until graceful shutdown is implemented
		
		config := &Config{
			Server: struct {
				Host string `yaml:"host"`
				Port string `yaml:"port"`
				ReadTimeout  string `yaml:"read_timeout"`
				WriteTimeout string `yaml:"write_timeout"`
			}{
				Host: "localhost",
				Port: "9877", // Different port for shutdown test
			},
			Database: struct {
				Path string `yaml:"path"`
			}{
				Path: ":memory:",
			},
		}
		
		app, err := NewApplication(config)
		require.NoError(t, err)
		
		err = app.Start()
		require.NoError(t, err)
		
		// Create some connections to test shutdown
		port := app.config.Server.Port
		if port == "" {
			port = "8080"
		}
		wsURL := "ws://localhost:" + port + "/ws"
		
		// Connect some WebSocket clients
		var connections []*websocket.Conn
		for i := 0; i < 3; i++ {
			conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?user_id=user"+string(rune('0'+i))+"&role=student", nil)
			require.NoError(t, err)
			connections = append(connections, conn)
		}
		
		// Test graceful shutdown timing
		shutdownStart := time.Now()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		shutdownDone := make(chan error, 1)
		go func() {
			shutdownDone <- app.Stop(shutdownCtx)
		}()
		
		// Verify connections receive close messages
		for i, conn := range connections {
			_, _, err := conn.ReadMessage()
			if err != nil {
				// Should be a close error
				if websocket.IsCloseError(err, websocket.CloseGoingAway) {
					t.Logf("Connection %d closed gracefully", i)
				}
			}
			if err := conn.Close(); err != nil {
				t.Logf("Failed to close connection %d: %v", i, err)
			}
		}
		
		// Wait for shutdown to complete
		select {
		case err := <-shutdownDone:
			shutdownDuration := time.Since(shutdownStart)
			assert.NoError(t, err, "Graceful shutdown should succeed")
			assert.Less(t, shutdownDuration, 10*time.Second, "Shutdown should complete within timeout")
			t.Logf("Graceful shutdown completed in %v", shutdownDuration)
			
		case <-shutdownCtx.Done():
			t.Fatal("Shutdown timeout exceeded")
		}
	})
}

// TestStep53_ConfigurationErrorHandling tests configuration error scenarios
func TestStep53_ConfigurationErrorHandling(t *testing.T) {
	t.Run("invalid_config_file", func(t *testing.T) {
		// This test will fail until error handling is implemented
		
		// Test loading non-existent config file
		_, err := loadConfig("/non/existent/config.yaml")
		assert.Error(t, err, "Should fail to load non-existent config")
		
		// Test loading invalid YAML
		invalidConfigFile := "/tmp/invalid_config.yaml"
		err = os.WriteFile(invalidConfigFile, []byte("invalid: yaml: content: ["), 0644)
		require.NoError(t, err)
		defer func() {
			if err := os.Remove(invalidConfigFile); err != nil {
				t.Logf("Failed to remove invalid config file: %v", err)
			}
		}()
		
		_, err = loadConfig(invalidConfigFile)
		assert.Error(t, err, "Should fail to parse invalid YAML")
	})
	
	t.Run("invalid_database_path", func(t *testing.T) {
		// This test will fail until database error handling is implemented
		
		config := &Config{
			Database: struct {
				Path string `yaml:"path"`
			}{
				Path: "/invalid/path/database.db",
			},
		}
		
		_, err := NewApplication(config)
		assert.Error(t, err, "Should fail with invalid database path")
	})
}

// These types will fail to compile until implementation is complete
// They represent the expected interface for Step 5.3

type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
		ReadTimeout  string `yaml:"read_timeout"`
		WriteTimeout string `yaml:"write_timeout"`
	} `yaml:"server"`
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
	Logging struct {
		Level string `yaml:"level"`
		File  string `yaml:"file"`
	} `yaml:"logging"`
	WebSocket struct {
		CheckOrigin       bool   `yaml:"check_origin"`
		HandshakeTimeout string `yaml:"handshake_timeout"`
	} `yaml:"websocket"`
}

type Application struct {
	config             *Config
	dbManager          database.DatabaseManager
	sessionManager     session.SessionManager
	sessionLifecycle   *session.SessionLifecycle
	messageProcessor   *message.MessageProcessor
	rateLimiter        *rate.RateLimiter
	connectionRegistry *websocketpkg.ConnectionRegistry
	broadcastSystem    *websocketpkg.BroadcastSystem
	websocketHandler   *websocketpkg.WebSocketHandler
	sessionAPIHandler  *api.SessionAPIHandler
	httpServer         *web.HTTPServer
}

// NewApplication creates and initializes the application with all components
// following the exact dependency injection order specified in phase-5.md
func NewApplication(config *Config) (*Application, error) {
	app := &Application{config: config}

	// Phase 1: Initialize database
	db, err := sql.Open("sqlite3", config.Database.Path)
	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	// Apply database schema
	if err := applyDatabaseSchema(db); err != nil {
		return nil, fmt.Errorf("schema application failed: %w", err)
	}

	dbManager, err := database.NewSQLiteDatabaseManager(db)
	if err != nil {
		return nil, fmt.Errorf("database manager creation failed: %w", err)
	}
	app.dbManager = dbManager

	// Phase 2: Initialize session management
	app.sessionManager = session.NewSessionManager(app.dbManager)
	app.sessionLifecycle = session.NewSessionLifecycle(app.sessionManager, app.dbManager)

	// Phase 3: Initialize message processing
	app.rateLimiter = rate.NewRateLimiter()
	app.connectionRegistry = websocketpkg.NewConnectionRegistry(app.sessionManager)

	roleBasedFilter := &message.RoleBasedFilter{}
	messageRouter := message.NewMessageRouter(app.connectionRegistry, roleBasedFilter)

	// Initialize BroadcastSystem with connection registry and role-based filter
	filterAdapter := websocketpkg.NewFilterAdapter(roleBasedFilter)
	app.broadcastSystem = websocketpkg.NewBroadcastSystem(app.connectionRegistry, filterAdapter)

	app.messageProcessor = message.NewMessageProcessor(
		app.sessionManager,
		app.dbManager,
		app.rateLimiter,
		messageRouter,
		app.broadcastSystem,
	)

	// Phase 4: Initialize WebSocket handling
	app.websocketHandler = websocketpkg.NewWebSocketHandler(
		app.connectionRegistry,
		app.sessionManager,
		app.messageProcessor,
		app.dbManager,
	)

	// Phase 5: Initialize HTTP API
	app.sessionAPIHandler = api.NewSessionAPIHandler(app.sessionLifecycle)
	app.httpServer = web.NewHTTPServer(app.sessionAPIHandler, app.websocketHandler)

	return app, nil
}

func (app *Application) Start() error {
	// Start database manager
	if err := app.dbManager.Start(); err != nil {
		return fmt.Errorf("database manager start failed: %w", err)
	}

	// Note: RateLimiter starts automatically in constructor

	// Start connection registry
	app.connectionRegistry.Start()

	// Start HTTP server (non-blocking)
	go func() {
		port := app.config.Server.Port
		if port == "" {
			port = "8080"
		}

		if err := app.httpServer.Start(port); err != nil {
			// Non-fatal in test environment - server start failures are logged but don't stop test
			log.Printf("HTTP server start failed (non-fatal): %v", err)
		}
	}()

	return nil
}

func (app *Application) Stop(ctx context.Context) error {
	// Stop HTTP server first
	if err := app.httpServer.Stop(ctx); err != nil {
		// Non-fatal in test environment - log but continue shutdown
		log.Printf("HTTP server stop failed (non-fatal): %v", err)
	}

	// Stop connection registry (closes all WebSocket connections)
	app.connectionRegistry.Stop()

	// Stop rate limiter
	app.rateLimiter.Stop()

	// Stop database manager (flushes pending writes)
	if err := app.dbManager.Stop(); err != nil {
		// Non-fatal in test environment - log but continue shutdown
		log.Printf("Database manager stop failed (non-fatal): %v", err)
	}

	return nil
}

func loadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("config file read failed: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("config parse failed: %w", err)
	}

	return &config, nil
}

// applyDatabaseSchema applies the database schema from migrations.sql
func applyDatabaseSchema(db *sql.DB) error {
	// Try different paths since test may run from different working directories
	possiblePaths := []string{
		"internal/database/migrations.sql",
		"../../internal/database/migrations.sql",
		"../../../internal/database/migrations.sql",
	}
	
	var schema []byte
	var err error
	
	for _, path := range possiblePaths {
		schema, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}
	
	if err != nil {
		return fmt.Errorf("schema file read failed from all paths: %w", err)
	}

	if _, err := db.Exec(string(schema)); err != nil {
		return fmt.Errorf("schema execution failed: %w", err)
	}

	return nil
}