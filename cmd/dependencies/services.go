package dependencies

import (
	"log/slog"

	cachedinternal "github.com/OmerSorrell/oauth-ms/internal/adapter/internalsvc/cached"
	"github.com/OmerSorrell/oauth-ms/internal/adapter/internalsvc/httpclient"
	"github.com/OmerSorrell/oauth-ms/internal/config"
	"github.com/OmerSorrell/oauth-ms/internal/port/cache"
	"github.com/OmerSorrell/oauth-ms/internal/port/internalsvc"
)

func provideInternalServiceHTTPClient(_ *slog.Logger, cfg *config.Config) (internalsvc.InternalService, error) {
	return httpclient.New(httpclient.Config{
		BaseURL: cfg.Internal.BaseURL,
		APIKey:  cfg.Internal.APIKey,
		Timeout: cfg.Internal.HTTPTimeout,
	})
}

func provideCachedInternalService(_ *slog.Logger, upstream internalsvc.InternalService, c cache.Cache, cfg *config.Config) internalsvc.InternalService {
	return cachedinternal.New(upstream, c, cfg.Internal.CacheTTL)
}
