// Package auth holds domain types for the OAuth flow and resulting user session.
package auth

import "time"

// Session is the server-side record bound to a session-id cookie.
//
// The browser sees only the random session ID; the access token never leaves the server.
// ProviderUserID is captured at callback time so /profile can fan out the provider call
// and the internal-service call truly in parallel.
type Session struct {
	ID                   string
	Provider             string
	AccessToken          string
	TokenType            string
	AccessTokenExpiresAt time.Time
	Scopes               []string
	ProviderUserID       string
	DisplayName          string
	CreatedAt            time.Time
}
