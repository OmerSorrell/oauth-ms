package auth

import "time"

// StateRecord pairs a CSRF state value with its PKCE verifier and originating provider.
//
// The record is consumed (atomically read-and-deleted) on callback so a state value
// can never be replayed.
type StateRecord struct {
	State        string
	CodeVerifier string
	Provider     string
	RedirectURI  string
	CreatedAt    time.Time
}
