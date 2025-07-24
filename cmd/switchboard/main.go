package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"switchboard/internal/api"
	"switchboard/internal/config"
	"switchboard/internal/database"
	"switchboard/internal/hub"
	"switchboard/internal/router"
	"switchboard/internal/session"
	"switchboard/internal/websocket"
	pkgdatabase "switchboard/pkg/database"
)

// ARCHITECTURAL DISCOVERY: Application struct coordinates all system components
// Clean dependency injection pattern with proper initialization order
type Application struct {
	config        *config.Config
	dbManager     *database.Manager
	sessionManager *session.Manager
	registry      *websocket.Registry
	messageRouter *router.Router
	messageHub    *hub.Hub
	apiServer     *api.Server
	httpServer    *http.Server
}

// FUNCTIONAL DISCOVERY: Component initialization follows strict dependency order
// Database → Session → Registry → Router → Hub → API → HTTP
func NewApplication(cfg *config.Config) (*Application, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	
	// ARCHITECTURAL DISCOVERY: Validate configuration before component initialization
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// STEP 1: Initialize database manager (foundation layer)
	dbConfig := &pkgdatabase.Config{
		DatabasePath:    cfg.Database.Path,
		MaxConnections:  10,
		ConnMaxLifetime: cfg.Database.Timeout,
		ConnMaxIdleTime: cfg.Database.Timeout / 3,
		MigrationsPath:  "./migrations",
	}
	
	dbManager, err := database.NewManager(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database manager: %w", err)
	}
	
	// STEP 2: Initialize session manager with database dependency
	sessionManager := session.NewManager(dbManager)
	if err := sessionManager.LoadActiveSessions(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to load active sessions: %w", err)
	}
	
	// STEP 3: Initialize WebSocket registry for connection tracking
	registry := websocket.NewRegistry()
	
	// STEP 4: Initialize message router with dependencies
	messageRouter := router.NewRouter(registry, dbManager)
	
	// STEP 5: Initialize message hub for coordination
	messageHub := hub.NewHub(registry, messageRouter)
	
	// STEP 6: Initialize API server with all business dependencies
	apiServer := api.NewServer(sessionManager, dbManager, registry)
	
	// STEP 7: Initialize WebSocket handler
	wsHandler := websocket.NewHandler(registry, sessionManager, dbManager)
	
	// STEP 8: Setup HTTP server with both API and WebSocket endpoints
	mux := http.NewServeMux()
	mux.Handle("/api/", apiServer)
	mux.Handle("/health", apiServer)
	mux.HandleFunc("/ws", wsHandler.HandleWebSocket)
	
	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		Handler:      mux,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}
	
	return &Application{
		config:         cfg,
		dbManager:      dbManager,
		sessionManager: sessionManager,
		registry:       registry,
		messageRouter:  messageRouter,
		messageHub:     messageHub,
		apiServer:      apiServer,
		httpServer:     httpServer,
	}, nil
}

// FUNCTIONAL DISCOVERY: Startup coordination ensures all components ready before serving
// Hub starts first to handle messages, then HTTP server accepts connections
func (app *Application) Start(ctx context.Context) error {
	log.Printf("Starting Switchboard application on %s", app.httpServer.Addr)
	
	// STEP 1: Start message hub (background message processing)
	if err := app.messageHub.Start(ctx); err != nil {
		return fmt.Errorf("failed to start message hub: %w", err)
	}
	
	// STEP 2: Start HTTP server (accepts connections)
	serverErrCh := make(chan error, 1)
	go func() {
		if err := app.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()
	
	// FUNCTIONAL DISCOVERY: Verify server is ready before returning
	select {
	case err := <-serverErrCh:
		// Cleanup on startup failure
		app.messageHub.Stop()
		return err
	case <-time.After(100 * time.Millisecond):
		// Server started successfully
		log.Printf("Switchboard application started successfully")
		return nil
	case <-ctx.Done():
		// Context cancelled during startup
		app.messageHub.Stop()
		return ctx.Err()
	}
}

// FUNCTIONAL DISCOVERY: Shutdown coordination ensures proper resource cleanup
// Reverse dependency order: HTTP → Hub → Database
func (app *Application) Stop(ctx context.Context) error {
	log.Printf("Shutting down Switchboard application")
	
	// STEP 1: Stop accepting new connections
	if err := app.httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	
	// STEP 2: Stop message processing
	if err := app.messageHub.Stop(); err != nil {
		log.Printf("Message hub shutdown error: %v", err)
	}
	
	// STEP 3: Close database connections
	if err := app.dbManager.Close(); err != nil {
		log.Printf("Database shutdown error: %v", err)
	}
	
	log.Printf("Switchboard application shutdown complete")
	return nil
}

// FUNCTIONAL DISCOVERY: Main entry point with comprehensive error handling and signal management
// Graceful shutdown on SIGINT/SIGTERM ensures proper resource cleanup
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// ARCHITECTURAL DISCOVERY: Separate run function enables testing and error handling
// Signal handling ensures graceful shutdown in production environments
func run() error {
	// STEP 1: Load configuration with precedence (file > env > defaults)
	configPath := os.Getenv("SWITCHBOARD_CONFIG_FILE")
	cfg := config.LoadConfigWithPrecedence(configPath)
	
	// STEP 2: Create application with configuration
	app, err := NewApplication(cfg)
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}
	
	// STEP 3: Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	
	// STEP 4: Start application in background
	appErrCh := make(chan error, 1)
	go func() {
		if err := app.Start(ctx); err != nil {
			appErrCh <- err
		}
	}()
	
	// STEP 5: Wait for shutdown signal or application error
	select {
	case err := <-appErrCh:
		// Application startup/runtime error
		return fmt.Errorf("application error: %w", err)
	case sig := <-signalCh:
		// Graceful shutdown requested
		log.Printf("Received signal %v, shutting down gracefully", sig)
		
		// FUNCTIONAL DISCOVERY: Timeout context prevents hanging shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		
		if err := app.Stop(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown error: %w", err)
		}
		
		return nil
	}
}