// Package dependencies builds the connector's full dependency graph from a Config.
//
// Layout follows the auth-service container-provider convention: one Provide() per
// process, factories grouped by category in sibling files (cache.go, stores.go,
// services.go, providers.go, usecase.go, http.go, logging.go).
package dependencies

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/OmerSorrell/oauth-ms/internal/config"
	"github.com/OmerSorrell/oauth-ms/web"
)

// Container is the entry-point command's view of the wired graph.
//
// Only fields entry-point code needs are exported; intermediates stay local to Provide().
type Container struct {
	Server *http.Server
	Logger *slog.Logger
	Config *config.Config

	cleanups []func()
}

// Close runs cleanups in LIFO order. Idempotent; safe to defer in main.
func (c *Container) Close() {
	for i := len(c.cleanups) - 1; i >= 0; i-- {
		c.cleanups[i]()
	}
	c.cleanups = nil
}

// ContainerProvider wires the graph. One Provide call per process.
type ContainerProvider struct{}

func NewContainerProvider() *ContainerProvider { return &ContainerProvider{} }

// Provide constructs the full dependency graph top-down with logged + wrapped errors.
func (p *ContainerProvider) Provide(ctx context.Context, cfg *config.Config) (context.Context, *Container, error) {
	logger := provideLogger(cfg)
	c := &Container{Logger: logger, Config: cfg}

	cacheImpl, stopCache, err := provideCache(ctx, logger, cfg)
	if err != nil {
		return ctx, nil, fmt.Errorf("dependencies: cache: %w", err)
	}
	c.cleanups = append(c.cleanups, stopCache)

	stateStore := provideStateStore(logger, cacheImpl)
	sessionStore := provideSessionStore(logger, cacheImpl)

	internalHTTP, err := provideInternalServiceHTTPClient(logger, cfg)
	if err != nil {
		return ctx, nil, fmt.Errorf("dependencies: internal http: %w", err)
	}
	internalSvc := provideCachedInternalService(logger, internalHTTP, cacheImpl, cfg)

	githubProv, err := provideGithubProvider(logger, cfg)
	if err != nil {
		return ctx, nil, fmt.Errorf("dependencies: github provider: %w", err)
	}

	registry, err := provideProviderRegistry(logger, githubProv)
	if err != nil {
		return ctx, nil, fmt.Errorf("dependencies: registry: %w", err)
	}

	authUC := createAuthUseCase(logger, registry, stateStore, sessionStore, cfg)
	profileUC := createProfileUseCase(logger, registry, internalSvc)

	authH, profileH := provideHandlers(logger, authUC, profileUC, cfg)
	router := provideRouter(logger, sessionStore, authH, profileH, web.FS())
	c.Server = provideHTTPServer(logger, router, cfg)

	logger.Info("container ready",
		"cache", cfg.Cache.Driver,
		"providers", registry.Names(),
		"base_url", cfg.Server.BaseURL,
	)
	return ctx, c, nil
}
