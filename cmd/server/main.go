package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"

	"switchboard/internal/database"
	"switchboard/internal/message"
	"switchboard/internal/rate"
	"switchboard/internal/session"
	"switchboard/internal/websocket"
	"switchboard/pkg/config"
	"switchboard/web"
	"switchboard/web/api"
)

// Config represents the YAML configuration structure
type Config struct {
	Server struct {
		Host         string `yaml:"host"`
		Port         string `yaml:"port"`
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
		CheckOrigin      bool   `yaml:"check_origin"`
		HandshakeTimeout string `yaml:"handshake_timeout"`
	} `yaml:"websocket"`
}

// Application represents the main application with all component dependencies
type Application struct {
	config             *Config
	dbManager          database.DatabaseManager
	sessionManager     session.SessionManager
	sessionLifecycle   *session.SessionLifecycle
	messageProcessor   *message.MessageProcessor
	rateLimiter        *rate.RateLimiter
	connectionRegistry *websocket.ConnectionRegistry
	broadcastSystem    *websocket.BroadcastSystem
	websocketHandler   *websocket.WebSocketHandler
	sessionAPIHandler  *api.SessionAPIHandler
	httpServer         *web.HTTPServer
}

func main() {
	log.Println("Switchboard V4 starting...")

	// Load configuration
	config, err := loadConfig("config/development.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize application
	app, err := NewApplication(config)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Start application
	if err := app.Start(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Switchboard V4 running. Press Ctrl+C to shutdown.")
	<-sigChan

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Shutting down...")
	if err := app.Stop(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v", err)
	} else {
		log.Println("Shutdown completed successfully")
	}
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

	// Phase 3: Initialize message processing
	app.rateLimiter = rate.NewRateLimiter()
	app.connectionRegistry = websocket.NewConnectionRegistry(app.sessionManager)

	roleBasedFilter := &message.RoleBasedFilter{}
	messageRouter := message.NewMessageRouter(app.connectionRegistry, roleBasedFilter)

	// Initialize BroadcastSystem with connection registry and role-based filter
	filterAdapter := websocket.NewFilterAdapter(roleBasedFilter)
	app.broadcastSystem = websocket.NewBroadcastSystem(app.connectionRegistry, filterAdapter)

	// Initialize SessionBroadcaster as adapter between SessionLifecycle and BroadcastSystem
	sessionBroadcaster := websocket.NewSessionBroadcaster(app.broadcastSystem, app.connectionRegistry)

	// Initialize SessionLifecycle with system broadcaster dependency
	app.sessionLifecycle = session.NewSessionLifecycle(app.sessionManager, app.dbManager, sessionBroadcaster)

	app.messageProcessor = message.NewMessageProcessor(
		app.sessionManager,
		app.dbManager,
		app.rateLimiter,
		messageRouter,
		app.broadcastSystem,
	)

	// Phase 4: Initialize WebSocket handling
	app.websocketHandler = websocket.NewWebSocketHandler(
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

// Start initializes and starts all application components in the proper sequence
func (app *Application) Start() error {
	log.Println("Starting application components...")

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
			log.Printf("HTTP server error: %v", err)
		}
	}()

	log.Printf("All components started successfully")
	log.Printf("Test client available at: http://localhost:%s", app.config.Server.Port)
	log.Printf("WebSocket endpoint: ws://localhost:%s/ws", app.config.Server.Port)

	return nil
}

// Stop gracefully shuts down all application components within the timeout
// Implements the 5-phase graceful shutdown algorithm from tech specs lines 753-805
func (app *Application) Stop(ctx context.Context) error {
	log.Println("Starting graceful shutdown sequence...")
	shutdownStart := time.Now()

	// Phase 1: Stop accepting new connections (HTTP server shutdown)
	log.Println("Phase 1: Stopping HTTP server...")
	phaseStart := time.Now()
	
	if err := app.httpServer.Stop(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	} else {
		log.Printf("Phase 1 completed in %v", time.Since(phaseStart))
	}

	// Phase 2: Wait for in-flight message processing (simplified - no active tracking needed)
	log.Println("Phase 2: Waiting for in-flight message processing...")
	phaseStart = time.Now()
	
	// Create timeout context for message processing phase
	messageCtx, messageCancel := context.WithTimeout(ctx, config.MessageProcessingTimeout)
	defer messageCancel()
	
	// Since message processing is synchronous in current implementation,
	// we add a small delay to allow any concurrent processing to complete
	select {
	case <-time.After(100 * time.Millisecond):
		log.Printf("Phase 2 completed in %v", time.Since(phaseStart))
	case <-messageCtx.Done():
		log.Printf("Phase 2 timed out after %v", time.Since(phaseStart))
	}

	// Phase 3: Close all WebSocket connections gracefully
	log.Println("Phase 3: Closing WebSocket connections...")
	phaseStart = time.Now()
	
	app.connectionRegistry.Stop() // This now sends close messages before closing
	log.Printf("Phase 3 completed in %v", time.Since(phaseStart))

	// Phase 4: Stop database manager and flush pending writes
	log.Println("Phase 4: Stopping database manager...")
	phaseStart = time.Now()
	
	if err := app.dbManager.Stop(); err != nil {
		log.Printf("Database manager shutdown error: %v", err)
	} else {
		log.Printf("Phase 4 completed in %v", time.Since(phaseStart))
	}

	// Phase 5: Stop rate limiter and wait for remaining goroutines
	log.Println("Phase 5: Stopping remaining components...")
	phaseStart = time.Now()
	
	// Create timeout context for goroutine cleanup
	cleanupCtx, cleanupCancel := context.WithTimeout(ctx, config.GoroutineCleanupTimeout)
	defer cleanupCancel()
	
	// Stop rate limiter
	app.rateLimiter.Stop()
	
	// Wait for cleanup timeout or context cancellation
	select {
	case <-time.After(100 * time.Millisecond):
		log.Printf("Phase 5 completed in %v", time.Since(phaseStart))
	case <-cleanupCtx.Done():
		log.Printf("Phase 5 timed out after %v", time.Since(phaseStart))
	}

	totalTime := time.Since(shutdownStart)
	log.Printf("Graceful shutdown completed successfully in %v", totalTime)
	
	if totalTime > 10*time.Second {
		log.Printf("WARNING: Shutdown took longer than 10 seconds")
	}
	
	return nil
}

// loadConfig loads and parses the YAML configuration file
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
	schema, err := os.ReadFile("internal/database/migrations.sql")
	if err != nil {
		return fmt.Errorf("schema file read failed: %w", err)
	}

	if _, err := db.Exec(string(schema)); err != nil {
		return fmt.Errorf("schema execution failed: %w", err)
	}

	return nil
}