// Package httpapi composes chi router with handlers + middleware.
package httpapi

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/OmerSorrell/oauth-ms/internal/port/store"
	"github.com/OmerSorrell/oauth-ms/internal/transport/httpapi/handler"
	"github.com/OmerSorrell/oauth-ms/internal/transport/httpapi/middleware"
)

// RouterDeps bundles everything NewRouter needs. The container builds it.
type RouterDeps struct {
	Logger         *slog.Logger
	SessionStore   store.SessionStore
	AuthHandler    *handler.Auth
	ProfileHandler *handler.Profile
	StaticFS       fs.FS // optional: web/ embedded; when nil, no static files served
}

// NewRouter wires the chi router. Routes:
//
//	GET  /healthz, /readyz                 — liveness/readiness
//	GET  /auth/{provider}                  — start the OAuth flow
//	GET  /auth/{provider}/callback         — finish the OAuth flow
//	POST /logout                           — clear the server-side session and cookie
//	GET  /profile                          — aggregated profile (requires session)
//	GET  /*                                — embedded UI (when StaticFS is set)
func NewRouter(d RouterDeps) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(middleware.Recovery(d.Logger))
	r.Use(middleware.Logging(d.Logger))
	r.Use(middleware.WithSession(d.SessionStore))

	r.Get("/healthz", handler.Health)
	r.Get("/readyz", handler.Health)

	r.Route("/auth", func(r chi.Router) {
		r.Get("/{provider}", d.AuthHandler.Start)
		r.Get("/{provider}/callback", d.AuthHandler.Callback)
	})
	r.Post("/logout", d.AuthHandler.Logout)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireSession)
		r.Get("/profile", d.ProfileHandler.Get)
	})

	if d.StaticFS != nil {
		r.Handle("/*", http.FileServer(http.FS(d.StaticFS)))
	}

	return r
}
