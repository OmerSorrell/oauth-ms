// Package handler holds the chi handlers for /auth, /profile, /logout, /healthz.
package handler

import (
	"net/http"

	"github.com/OmerSorrell/oauth-ms/internal/transport/httpapi/response"
)

// Health is shared by /healthz and /readyz. The skeleton has no dependency probes;
// extend with cache/internal-service pings if needed.
func Health(w http.ResponseWriter, _ *http.Request) {
	response.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
