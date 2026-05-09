package dependencies

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/OmerSorrell/oauth-ms/internal/adapter/cache/memory"
	"github.com/OmerSorrell/oauth-ms/internal/config"
	"github.com/OmerSorrell/oauth-ms/internal/port/cache"
)

// provideCache returns the configured cache implementation and its cleanup func.
//
// Today: in-memory only. The "redis" driver is wired as an explicit error to make
// it obvious where to add a redis adapter without anyone silently downgrading.
func provideCache(_ context.Context, log *slog.Logger, cfg *config.Config) (cache.Cache, func(), error) {
	switch cfg.Cache.Driver {
	case "memory", "":
		m := memory.New()
		log.Info("cache: in-memory adapter")
		return m, m.Stop, nil
	case "redis":
		return nil, func() {}, fmt.Errorf("redis driver not implemented (skeleton): write internal/adapter/cache/redis and wire it here")
	default:
		return nil, func() {}, fmt.Errorf("unknown cache driver %q", cfg.Cache.Driver)
	}
}
