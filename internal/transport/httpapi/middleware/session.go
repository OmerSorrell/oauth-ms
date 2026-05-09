package middleware

import (
	"context"
	"net/http"

	domauth "github.com/OmerSorrell/oauth-ms/internal/domain/auth"
	"github.com/OmerSorrell/oauth-ms/internal/port/store"
	"github.com/OmerSorrell/oauth-ms/internal/transport/httpapi/response"
)

// SessionCookieName is the browser-visible cookie holding only the random session ID.
// The access token never leaves the server.
const SessionCookieName = "session_id"

type sessionCtxKey struct{}

// WithSession reads the session_id cookie and (if present and valid) attaches the
// resolved Session to the request context. Missing/invalid cookies pass through
// untouched so handlers can decide whether to require a session.
func WithSession(s store.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(SessionCookieName)
			if err != nil || c.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			sess, err := s.Get(r.Context(), c.Value)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), sessionCtxKey{}, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionFromContext returns the resolved Session, if any, for the current request.
func SessionFromContext(ctx context.Context) (domauth.Session, bool) {
	s, ok := ctx.Value(sessionCtxKey{}).(domauth.Session)
	return s, ok
}

// RequireSession is a route-scoped middleware that 401s requests without a session.
func RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := SessionFromContext(r.Context()); !ok {
			response.WriteError(w, http.StatusUnauthorized, "session required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
