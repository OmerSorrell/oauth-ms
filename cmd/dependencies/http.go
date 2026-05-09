package dependencies

import (
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/OmerSorrell/oauth-ms/internal/config"
	"github.com/OmerSorrell/oauth-ms/internal/port/store"
	"github.com/OmerSorrell/oauth-ms/internal/transport/httpapi"
	"github.com/OmerSorrell/oauth-ms/internal/transport/httpapi/handler"
	authuc "github.com/OmerSorrell/oauth-ms/internal/usecase/auth"
	profileuc "github.com/OmerSorrell/oauth-ms/internal/usecase/profile"
)

func provideHandlers(log *slog.Logger, authUC *authuc.UseCase, profileUC *profileuc.UseCase, cfg *config.Config) (*handler.Auth, *handler.Profile) {
	authH := handler.NewAuth(log, authUC, handler.AuthConfig{
		Cookies: handler.CookieOptions{
			Secure:   cfg.Auth.CookieSecure,
			SameSite: parseSameSite(cfg.Auth.CookieSameSite),
			Domain:   cfg.Auth.CookieDomain,
		},
		StateTTL:      cfg.Auth.StateTTL,
		SessionTTL:    cfg.Auth.SessionTTL,
		PostLoginPath: cfg.Auth.PostLoginPath,
	})
	profH := handler.NewProfile(log, profileUC)
	return authH, profH
}

func parseSameSite(s string) http.SameSite {
	switch strings.ToLower(s) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func provideRouter(log *slog.Logger, sessions store.SessionStore, authH *handler.Auth, profH *handler.Profile, staticFS fs.FS) http.Handler {
	return httpapi.NewRouter(httpapi.RouterDeps{
		Logger:         log,
		SessionStore:   sessions,
		AuthHandler:    authH,
		ProfileHandler: profH,
		StaticFS:       staticFS,
	})
}

func provideHTTPServer(_ *slog.Logger, h http.Handler, cfg *config.Config) *http.Server {
	return &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      h,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}
}
