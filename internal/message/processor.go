package message

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"switchboard/internal/database"
	"switchboard/internal/rate"
	"switchboard/internal/session"
	"switchboard/pkg/errors"
)

// MessageProcessor implements the core message processing pipeline
// as specified in tech specs lines 304-345 ProcessIncomingMessage algorithm
type MessageProcessor struct {
	sessionManager  session.SessionManager
	dbManager       database.DatabaseManager
	rateLimiter     *rate.RateLimiter
	router          MessageRouter
	broadcastSystem BroadcastSystemInterface
}

// NewMessageProcessor creates a new MessageProcessor instance
func NewMessageProcessor(
	sessionManager session.SessionManager,
	dbManager database.DatabaseManager,
	rateLimiter *rate.RateLimiter,
	router MessageRouter,
	broadcastSystem BroadcastSystemInterface,
) *MessageProcessor {
	return &MessageProcessor{
		sessionManager:  sessionManager,
		dbManager:       dbManager,
		rateLimiter:     rateLimiter,
		router:          router,
		broadcastSystem: broadcastSystem,
	}
}

// ProcessIncomingMessage processes an incoming message following the exact 8-step algorithm
// from tech specs lines 304-345. This is the core message processing pipeline.
func (mp *MessageProcessor) ProcessIncomingMessage(rawData []byte, senderID string) error {
	// Step 1: Atomic session state capture
	session := mp.sessionManager.GetActiveSession()
	if session == nil {
		log.Printf("MESSAGE REJECTED: No active session - message from %s rejected (message gating)", senderID)
		return errors.ErrNoActiveSession
	}
	
	log.Printf("MESSAGE ACCEPTED: Active session %s - processing message from %s", session.ID, senderID)

	// Step 2: Parse and validate message
	var message database.Message
	if err := json.Unmarshal(rawData, &message); err != nil {
		return fmt.Errorf("invalid message format: %w", err)
	}

	if err := validateMessage(&message); err != nil {
		return fmt.Errorf("message validation failed: %w", err)
	}

	// Step 3: Prepare message with session context
	message.ID = generateMessageID()
	message.Timestamp = time.Now()
	message.FromUser = senderID
	message.SessionID = session.ID

	if message.Context == "" {
		message.Context = database.ContextGeneral
	}

	// Step 4: Rate limiting check
	if !mp.rateLimiter.Allow(senderID) {
		return errors.ErrRateLimitExceeded
	}

	// Step 5: Route message and determine recipients
	recipients, err := mp.router.GetRecipients(&message)
	if err != nil {
		return fmt.Errorf("message routing failed: %w", err)
	}

	// Step 6: Real-time broadcast (immediate delivery)
	if err := mp.broadcastSystem.BroadcastMessage(&message, recipients); err != nil {
		log.Printf("Failed to broadcast message %s: %v", message.ID, err)
		// Continue processing - broadcast failure shouldn't block persistence
	}

	// Step 7: Asynchronous persistence
	go func() {
		if err := mp.dbManager.WriteMessage(&message); err != nil {
			log.Printf("Failed to persist message %s: %v", message.ID, err)
		}
	}()

	// Step 8: Return success to sender
	return nil
}

// ProcessIncomingMessageSync processes an incoming message with synchronous persistence
// This method is identical to ProcessIncomingMessage but waits for database persistence
// to complete before returning. It should only be used in tests where persistence
// verification is required immediately after processing.
func (mp *MessageProcessor) ProcessIncomingMessageSync(rawData []byte, senderID string) error {
	// Step 1: Atomic session state capture
	session := mp.sessionManager.GetActiveSession()
	if session == nil {
		return errors.ErrNoActiveSession
	}

	// Step 2: Parse and validate message
	var message database.Message
	if err := json.Unmarshal(rawData, &message); err != nil {
		return fmt.Errorf("invalid message format: %w", err)
	}

	if err := validateMessage(&message); err != nil {
		return fmt.Errorf("message validation failed: %w", err)
	}

	// Step 3: Prepare message with session context
	message.ID = generateMessageID()
	message.Timestamp = time.Now()
	message.FromUser = senderID
	message.SessionID = session.ID

	if message.Context == "" {
		message.Context = database.ContextGeneral
	}

	// Step 4: Rate limiting check
	if !mp.rateLimiter.Allow(senderID) {
		return errors.ErrRateLimitExceeded
	}

	// Step 5: Route message and determine recipients
	recipients, err := mp.router.GetRecipients(&message)
	if err != nil {
		return fmt.Errorf("message routing failed: %w", err)
	}

	// Step 6: Real-time broadcast (immediate delivery)
	if err := mp.broadcastSystem.BroadcastMessage(&message, recipients); err != nil {
		log.Printf("Failed to broadcast message %s: %v", message.ID, err)
		// Continue processing - broadcast failure shouldn't block persistence
	}

	// Step 7: Synchronous persistence (for testing)
	if err := mp.dbManager.WriteMessage(&message); err != nil {
		log.Printf("Failed to persist message %s: %v", message.ID, err)
		return fmt.Errorf("message persistence failed: %w", err)
	}

	// Step 8: Return success to sender
	return nil
}

// generateMessageID generates a cryptographically secure unique message ID
// using crypto/rand for high-entropy randomness as specified in tech specs.
// Returns a hex-encoded string of 16 random bytes (32 hex characters).
func generateMessageID() string {
	// Generate 16 bytes of cryptographically secure random data
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		// In the extremely unlikely case crypto/rand fails,
		// this would be a critical system failure. However,
		// crypto/rand.Read() is documented to only fail on
		// systems without a proper entropy source, which
		// should not occur in production environments.
		panic("crypto/rand failure: " + err.Error())
	}

	// Return hex-encoded string (32 characters)
	return hex.EncodeToString(bytes)
}
