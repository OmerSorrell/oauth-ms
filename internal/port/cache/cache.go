// Package cache defines the bytes-level KV port the connector uses for state, sessions,
// and internal-service response caching. The in-memory adapter is wired by default and a
// Redis adapter can be dropped in without touching call sites.
package cache

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by adapters that prefer error-based absence signaling.
// Hot-path callers should branch on the (found bool) return instead.
var ErrNotFound = errors.New("cache: not found")

// Cache is the KV contract. Values are opaque bytes; typed wrappers (StateStore,
// SessionStore, cached InternalService) handle JSON serialization.
type Cache interface {
	Get(ctx context.Context, key string) (value []byte, found bool, err error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}
