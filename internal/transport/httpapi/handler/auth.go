package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/OmerSorrell/oauth-ms/internal/port/provider"
	"github.com/OmerSorrell/oauth-ms/internal/port/store"
	"github.com/OmerSorrell/oauth-ms/internal/transport/httpapi/middleware"
	"github.com/OmerSorrell/oauth-ms/internal/transport/httpapi/response"
	authuc "github.com/OmerSorrell/oauth-ms/internal/usecase/auth"
)

// StateCookieName is the short-lived cookie paired with StateStore for CSRF defense in depth.
const StateCookieName = "oauth_state"

// CookieOptions are the security flags applied to both the oauth_state and session_id cookies.
type CookieOptions struct {
	Secure   bool
	SameSite http.SameSite
	Domain   string
}

// AuthConfig configures the auth handler.
type AuthConfig struct {
	Cookies       CookieOptions
	StateTTL      time.Duration
	SessionTTL    time.Duration
	PostLoginPath string // where to redirect after a successful callback (e.g., "/")
}

// Auth handles the OAuth start/callback/logout endpoints.
type Auth struct {
	log *slog.Logger
	uc  *authuc.UseCase
	cfg AuthConfig
}

func NewAuth(log *slog.Logger, uc *authuc.UseCase, cfg AuthConfig) *Auth {
	if cfg.PostLoginPath == "" {
		cfg.PostLoginPath = "/"
	}
	return &Auth{log: log, uc: uc, cfg: cfg}
}

// Start handles GET /auth/{provider}.
func (h *Auth) Start(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	res, err := h.uc.StartFlow(r.Context(), providerName)
	if err != nil {
		if errors.Is(err, provider.ErrUnknownProvider) {
			response.WriteError(w, http.StatusNotFound, "unknown provider")
			return
		}
		h.log.Error("auth start failed", "provider", providerName, "err", err)
		response.WriteError(w, http.StatusInternalServerError, "auth start failed")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     StateCookieName,
		Value:    res.State,
		Path:     "/auth",
		HttpOnly: true,
		Secure:   h.cfg.Cookies.Secure,
		SameSite: h.cfg.Cookies.SameSite,
		Domain:   h.cfg.Cookies.Domain,
		MaxAge:   int(h.cfg.StateTTL.Seconds()),
	})
	http.Redirect(w, r, res.AuthorizationURL, http.StatusFound)
}

// Callback handles GET /auth/{provider}/callback.
func (h *Auth) Callback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	q := r.URL.Query()
	if errParam := q.Get("error"); errParam != "" {
		h.clearStateCookie(w)
		response.WriteError(w, http.StatusBadRequest, errParam)
		return
	}

	state := q.Get("state")
	code := q.Get("code")

	var stateCookie string
	if c, err := r.Cookie(StateCookieName); err == nil {
		stateCookie = c.Value
	}
	h.clearStateCookie(w)

	out, err := h.uc.HandleCallback(r.Context(), authuc.CallbackInput{
		Provider:    providerName,
		State:       state,
		StateCookie: stateCookie,
		Code:        code,
	})
	if err != nil {
		switch {
		case errors.Is(err, authuc.ErrStateMismatch), errors.Is(err, authuc.ErrProviderMismatch):
			response.WriteError(w, http.StatusBadRequest, "invalid state")
		case errors.Is(err, store.ErrStateNotFound):
			response.WriteError(w, http.StatusBadRequest, "state expired or already used")
		case errors.Is(err, provider.ErrUnknownProvider):
			response.WriteError(w, http.StatusNotFound, "unknown provider")
		default:
			h.log.Error("auth callback failed", "provider", providerName, "err", err)
			response.WriteError(w, http.StatusBadGateway, "callback failed")
		}
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    out.SessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.Cookies.Secure,
		SameSite: h.cfg.Cookies.SameSite,
		Domain:   h.cfg.Cookies.Domain,
		MaxAge:   int(h.cfg.SessionTTL.Seconds()),
	})
	http.Redirect(w, r, h.cfg.PostLoginPath, http.StatusFound)
}

// Logout handles POST /logout.
func (h *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	var sid string
	if c, err := r.Cookie(middleware.SessionCookieName); err == nil {
		sid = c.Value
	}
	if err := h.uc.Logout(r.Context(), sid); err != nil {
		h.log.Error("logout failed", "err", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.Cookies.Secure,
		SameSite: h.cfg.Cookies.SameSite,
		Domain:   h.cfg.Cookies.Domain,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Auth) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     StateCookieName,
		Value:    "",
		Path:     "/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.Cookies.Secure,
		SameSite: h.cfg.Cookies.SameSite,
		Domain:   h.cfg.Cookies.Domain,
	})
}
