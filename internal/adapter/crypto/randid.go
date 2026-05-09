// Package crypto holds small cryptographic primitives used across the connector
// (random IDs for sessions/state).
package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// RandomURLSafeID returns a 32-byte cryptographically random ID, base64url-encoded (43 chars).
//
// Suitable for session IDs and OAuth state values.
func RandomURLSafeID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto: rand.Read: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
