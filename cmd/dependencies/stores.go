package dependencies

import (
	"log/slog"

	"github.com/OmerSorrell/oauth-ms/internal/adapter/store/cached"
	"github.com/OmerSorrell/oauth-ms/internal/port/cache"
	"github.com/OmerSorrell/oauth-ms/internal/port/store"
)

func provideStateStore(_ *slog.Logger, c cache.Cache) store.StateStore {
	return cached.NewStateStore(c)
}

func provideSessionStore(_ *slog.Logger, c cache.Cache) store.SessionStore {
	return cached.NewSessionStore(c)
}
