package dependencies

import (
	"log/slog"

	"github.com/OmerSorrell/oauth-ms/internal/config"
	"github.com/OmerSorrell/oauth-ms/internal/port/internalsvc"
	"github.com/OmerSorrell/oauth-ms/internal/port/provider"
	"github.com/OmerSorrell/oauth-ms/internal/port/store"
	authuc "github.com/OmerSorrell/oauth-ms/internal/usecase/auth"
	profileuc "github.com/OmerSorrell/oauth-ms/internal/usecase/profile"
)

func createAuthUseCase(log *slog.Logger, reg provider.Registry, states store.StateStore, sessions store.SessionStore, cfg *config.Config) *authuc.UseCase {
	return authuc.New(log, reg, states, sessions, authuc.Config{
		StateTTL:   cfg.Auth.StateTTL,
		SessionTTL: cfg.Auth.SessionTTL,
		BaseURL:    cfg.Server.BaseURL,
	})
}

func createProfileUseCase(log *slog.Logger, reg provider.Registry, svc internalsvc.InternalService) *profileuc.UseCase {
	return profileuc.New(log, reg, svc)
}
