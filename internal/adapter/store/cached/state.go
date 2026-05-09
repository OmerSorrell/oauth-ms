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

const stateKeyPrefix = "oauth:state:"

// StateStore persists OAuth state+PKCE records over a port.Cache.
type StateStore struct {
	cache cache.Cache
}

func NewStateStore(c cache.Cache) *StateStore {
	return &StateStore{cache: c}
}

func (s *StateStore) Put(ctx context.Context, rec auth.StateRecord, ttl time.Duration) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("cached.StateStore: marshal: %w", err)
	}
	if err := s.cache.Set(ctx, stateKeyPrefix+rec.State, b, ttl); err != nil {
		return fmt.Errorf("cached.StateStore: set: %w", err)
	}
	return nil
}

// Consume reads then deletes the state record.
//
// On the in-memory cache this is a two-step (get, delete) sequence: under concurrent
// callbacks for the same state value, the second caller observes ErrStateNotFound.
// A Redis adapter can use GETDEL for true atomicity.
func (s *StateStore) Consume(ctx context.Context, state string) (auth.StateRecord, error) {
	key := stateKeyPrefix + state
	b, ok, err := s.cache.Get(ctx, key)
	if err != nil {
		return auth.StateRecord{}, fmt.Errorf("cached.StateStore: get: %w", err)
	}
	if !ok {
		return auth.StateRecord{}, store.ErrStateNotFound
	}
	if err := s.cache.Delete(ctx, key); err != nil {
		return auth.StateRecord{}, fmt.Errorf("cached.StateStore: delete: %w", err)
	}
	var rec auth.StateRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		return auth.StateRecord{}, fmt.Errorf("cached.StateStore: unmarshal: %w", err)
	}
	return rec, nil
}

var _ store.StateStore = (*StateStore)(nil)
