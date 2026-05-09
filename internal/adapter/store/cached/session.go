// Package cached implements StateStore and SessionStore on top of a port.Cache.
//
// Records are JSON-encoded so the same code works against an in-memory or Redis backend.
package cached

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/OmerSorrell/oauth-ms/internal/domain/auth"
	"github.com/OmerSorrell/oauth-ms/internal/port/cache"
	"github.com/OmerSorrell/oauth-ms/internal/port/store"
)

const sessionKeyPrefix = "session:"

// SessionStore persists post-callback sessions over a port.Cache.
type SessionStore struct {
	cache cache.Cache
}

func NewSessionStore(c cache.Cache) *SessionStore {
	return &SessionStore{cache: c}
}

func (s *SessionStore) Put(ctx context.Context, sess auth.Session, ttl time.Duration) error {
	b, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("cached.SessionStore: marshal: %w", err)
	}
	if err := s.cache.Set(ctx, sessionKeyPrefix+sess.ID, b, ttl); err != nil {
		return fmt.Errorf("cached.SessionStore: set: %w", err)
	}
	return nil
}

func (s *SessionStore) Get(ctx context.Context, sid string) (auth.Session, error) {
	b, ok, err := s.cache.Get(ctx, sessionKeyPrefix+sid)
	if err != nil {
		return auth.Session{}, fmt.Errorf("cached.SessionStore: get: %w", err)
	}
	if !ok {
		return auth.Session{}, store.ErrSessionNotFound
	}
	var sess auth.Session
	if err := json.Unmarshal(b, &sess); err != nil {
		return auth.Session{}, fmt.Errorf("cached.SessionStore: unmarshal: %w", err)
	}
	return sess, nil
}

func (s *SessionStore) Delete(ctx context.Context, sid string) error {
	if err := s.cache.Delete(ctx, sessionKeyPrefix+sid); err != nil {
		return fmt.Errorf("cached.SessionStore: delete: %w", err)
	}
	return nil
}

var _ store.SessionStore = (*SessionStore)(nil)
