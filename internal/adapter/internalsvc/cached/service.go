// Package cached wraps an internalsvc.InternalService with a TTL cache and
// singleflight, so repeated /profile calls for the same user collapse to one
// upstream request and warm hits return immediately.
package cached

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/OmerSorrell/oauth-ms/internal/domain/profile"
	"github.com/OmerSorrell/oauth-ms/internal/port/cache"
	"github.com/OmerSorrell/oauth-ms/internal/port/internalsvc"
)

const keyPrefix = "internal:user:"

// Service decorates an upstream InternalService with caching and request coalescing.
type Service struct {
	upstream internalsvc.InternalService
	cache    cache.Cache
	ttl      time.Duration
	sf       singleflight.Group
}

func New(upstream internalsvc.InternalService, c cache.Cache, ttl time.Duration) *Service {
	return &Service{upstream: upstream, cache: c, ttl: ttl}
}

func (s *Service) GetUser(ctx context.Context, userID string) (profile.User, error) {
	key := keyPrefix + userID
	if b, ok, err := s.cache.Get(ctx, key); err == nil && ok {
		var u profile.User
		if err := json.Unmarshal(b, &u); err == nil {
			return u, nil
		}
	}

	v, err, _ := s.sf.Do(key, func() (any, error) {
		u, err := s.upstream.GetUser(ctx, userID)
		if err != nil {
			return profile.User{}, err
		}
		if b, mErr := json.Marshal(u); mErr == nil {
			_ = s.cache.Set(ctx, key, b, s.ttl)
		}
		return u, nil
	})
	if err != nil {
		if errors.Is(err, internalsvc.ErrUserNotFound) || errors.Is(err, internalsvc.ErrUnauthorized) {
			return profile.User{}, err
		}
		return profile.User{}, fmt.Errorf("internalsvc cached: %w", err)
	}
	return v.(profile.User), nil
}

var _ internalsvc.InternalService = (*Service)(nil)
