package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"switchboard/internal/app"
	"switchboard/internal/config"
)


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
	application, err := app.NewApplication(cfg)
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
		if err := application.Start(ctx); err != nil {
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
		
		if err := application.Stop(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown error: %w", err)
		}
		
		return nil
	}
}