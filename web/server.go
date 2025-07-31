package web

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"switchboard/internal/websocket"
	"switchboard/web/api"
)

// HTTPServer manages HTTP routing and WebSocket upgrades
// Implementation follows exact pattern from phase-5.md lines 212-322
type HTTPServer struct {
	server           *http.Server
	sessionHandler   *api.SessionAPIHandler
	websocketHandler *websocket.WebSocketHandler
	mux              *http.ServeMux
}

// NewHTTPServer creates a new HTTPServer instance with all required components.
// Follows exact implementation pattern from phase-5.md lines 231-251.
func NewHTTPServer(sessionHandler *api.SessionAPIHandler, websocketHandler *websocket.WebSocketHandler) *HTTPServer {
	mux := http.NewServeMux()

	server := &HTTPServer{
		sessionHandler:   sessionHandler,
		websocketHandler: websocketHandler,
		mux:              mux,
	}

	// Setup routes
	server.setupRoutes()

	server.server = &http.Server{
		Handler:      server.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return server
}

// GetHandler returns the HTTP handler for testing purposes
func (hs *HTTPServer) GetHandler() http.Handler {
	return hs.mux
}

// setupRoutes configures all HTTP routes with proper middleware chain.
// Follows exact routing pattern from phase-5.md lines 253-268.
func (hs *HTTPServer) setupRoutes() {
	// API routes with CORS and logging middleware
	hs.mux.HandleFunc("/api/session/start", hs.withCORS(hs.withLogging(hs.sessionHandler.StartSession)))
	hs.mux.HandleFunc("/api/session/end", hs.withCORS(hs.withLogging(hs.sessionHandler.EndSession)))

	// WebSocket route with logging middleware only
	hs.mux.HandleFunc("/ws", hs.withLogging(func(w http.ResponseWriter, r *http.Request) {
		if err := hs.websocketHandler.HandleWebSocketUpgrade(w, r); err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
		}
	}))

	// Static files for test client
	fileServer := http.FileServer(http.Dir("web/static/"))
	hs.mux.Handle("/", fileServer)
}

// Start begins HTTP server on specified port.
// Follows exact implementation pattern from phase-5.md lines 270-280.
func (hs *HTTPServer) Start(port string) error {
	hs.server.Addr = ":" + port

	log.Printf("Starting HTTP server on port %s", port)

	if err := hs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server start failed: %w", err)
	}

	return nil
}

// Stop gracefully shuts down HTTP server with context timeout.
// Follows exact implementation pattern from phase-5.md lines 282-291.
func (hs *HTTPServer) Stop(ctx context.Context) error {
	log.Println("Shutting down HTTP server...")

	if err := hs.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	log.Println("HTTP server stopped gracefully")
	return nil
}

// withCORS adds CORS headers and handles OPTIONS requests.
// Follows exact middleware pattern from phase-5.md lines 294-308.
func (hs *HTTPServer) withCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Basic CORS headers for development
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler(w, r)
	}
}

// withLogging adds request logging with method, path, and duration.
// Follows exact middleware pattern from phase-5.md lines 310-321.
func (hs *HTTPServer) withLogging(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Call handler
		handler(w, r)

		// Log request
		duration := time.Since(start)
		log.Printf("%s %s - %v", r.Method, r.URL.Path, duration)
	}
}