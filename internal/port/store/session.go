package store

import (
	"context"
	"errors"
	"time"

	"github.com/OmerSorrell/oauth-ms/internal/domain/auth"
)

// ErrSessionNotFound is returned when the session ID is unknown or expired.
var ErrSessionNotFound = errors.New("session: not found")

// SessionStore persists post-callback sessions keyed by a random session ID.
//
// The session ID is what's set in the browser cookie; the access token only ever
// lives server-side, addressable by that ID.
type SessionStore interface {
	Put(ctx context.Context, s auth.Session, ttl time.Duration) error
	Get(ctx context.Context, sessionID string) (auth.Session, error)
	Delete(ctx context.Context, sessionID string) error
}
