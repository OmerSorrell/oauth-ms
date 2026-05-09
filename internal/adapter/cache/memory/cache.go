// Package memory provides an in-memory port.Cache backed by jellydator/ttlcache.
//
// Swap to Redis later by writing a sibling adapter that satisfies port.Cache; the
// stores and the cached internal-service decorator have no knowledge of the backend.
package memory

import (
	"context"
	"time"

	"github.com/jellydator/ttlcache/v3"

	"github.com/OmerSorrell/oauth-ms/internal/port/cache"
)

// Cache is the in-memory implementation of port.Cache.
type Cache struct {
	c *ttlcache.Cache[string, []byte]
}

// New constructs a Cache with periodic janitor goroutine started.
//
// TTL is set per-key on Set; reads do NOT extend TTL (so OAuth state expires deterministically).
func New() *Cache {
	c := ttlcache.New[string, []byte](
		ttlcache.WithDisableTouchOnHit[string, []byte](),
	)
	go c.Start()
	return &Cache{c: c}
}

func (m *Cache) Get(_ context.Context, key string) ([]byte, bool, error) {
	item := m.c.Get(key)
	if item == nil {
		return nil, false, nil
	}
	return item.Value(), true, nil
}

func (m *Cache) Set(_ context.Context, key string, val []byte, ttl time.Duration) error {
	m.c.Set(key, val, ttl)
	return nil
}

func (m *Cache) Delete(_ context.Context, key string) error {
	m.c.Delete(key)
	return nil
}

// Stop terminates the janitor goroutine. Call from main on shutdown.
func (m *Cache) Stop() { m.c.Stop() }

var _ cache.Cache = (*Cache)(nil)
