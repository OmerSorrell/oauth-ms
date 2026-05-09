// Command internal-service is a tiny mock of the internal user-enrichment service
// described in the assignment: GET /api/users/{id} with X-Internal-API-Key.
//
// Sample data: a single happy-path user keyed by GitHub user ID. Override via env
// MOCK_USERS (JSON: {"<id>": {"id":"...","name":"...","license":"...","role":"..."}}).
package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

const apiKeyHeader = "X-Internal-API-Key"

type user struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	License string `json:"license"`
	Role    string `json:"role"`
}

type errBody struct {
	Error string `json:"error"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "mock-internal-service")

	addr := envOr("ADDR", ":8081")
	apiKey := envOr("API_KEY", "super-secret-key")

	users, err := loadUsers(os.Getenv("MOCK_USERS"))
	if err != nil {
		logger.Error("invalid MOCK_USERS env", "err", err)
		os.Exit(1)
	}
	if len(users) == 0 {
		// Default sample row so the connector demo works without configuring users.
		users["0"] = user{ID: "0", Name: "Demo User", License: "Pro", Role: "admin"}
	}

	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Get("/api/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(apiKeyHeader); got != apiKey {
			writeJSON(w, http.StatusUnauthorized, errBody{Error: "invalid api key"})
			return
		}
		id := chi.URLParam(r, "id")
		u, ok := users[id]
		if !ok {
			writeJSON(w, http.StatusNotFound, errBody{Error: "not found"})
			return
		}
		writeJSON(w, http.StatusOK, u)
	})

	srv := &http.Server{Addr: addr, Handler: r}
	logger.Info("listening", "addr", addr, "users", len(users))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("listen", "err", err)
		os.Exit(1)
	}
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func loadUsers(raw string) (map[string]user, error) {
	users := make(map[string]user)
	if raw == "" {
		return users, nil
	}
	if err := json.Unmarshal([]byte(raw), &users); err != nil {
		return nil, err
	}
	return users, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
