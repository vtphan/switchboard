package session

import (
	"crypto/rand"
	"encoding/hex"
)

// generateSessionID generates a cryptographically secure unique session ID
// using crypto/rand for high-entropy randomness as specified in tech specs.
// Returns a hex-encoded string of 16 random bytes (32 hex characters).
func generateSessionID() string {
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