// Package middleware contains chi-compatible HTTP middleware for the connector:
// panic recovery, request logging, and session-cookie ↔ context plumbing.
package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/OmerSorrell/oauth-ms/internal/transport/httpapi/response"
)

// Recovery turns a panic into a 500 with a logged stack trace.
func Recovery(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered",
						"panic", rec,
						"path", r.URL.Path,
						"stack", string(debug.Stack()),
					)
					response.WriteError(w, http.StatusInternalServerError, "internal error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
