// Package store defines typed persistence ports (sessions, OAuth state) used by the
// auth use case. Adapters live under internal/adapter/store/* and are typically
// thin wrappers over a port.Cache.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/OmerSorrell/oauth-ms/internal/domain/auth"
)

// ErrStateNotFound is returned when the state was never stored, or has been consumed/expired.
var ErrStateNotFound = errors.New("state: not found")

// StateStore persists OAuth state+PKCE records for the duration of an in-flight auth flow.
//
// Consume MUST be atomic (get-and-delete) so a state value can never be replayed.
type StateStore interface {
	Put(ctx context.Context, rec auth.StateRecord, ttl time.Duration) error
	Consume(ctx context.Context, state string) (auth.StateRecord, error)
}
