// Package response holds tiny helpers for writing JSON success/error envelopes
// that the handlers share.
package response

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorBody is the canonical error envelope.
type ErrorBody struct {
	Error string `json:"error"`
}

// WriteJSON writes status + JSON-encoded body. body may be nil for a status-only response.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("response: encode failed", "err", err)
	}
}

// WriteError writes a JSON ErrorBody at the given status.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, ErrorBody{Error: msg})
}
