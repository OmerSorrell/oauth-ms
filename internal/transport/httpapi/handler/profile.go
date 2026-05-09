package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/OmerSorrell/oauth-ms/internal/port/internalsvc"
	"github.com/OmerSorrell/oauth-ms/internal/transport/httpapi/middleware"
	"github.com/OmerSorrell/oauth-ms/internal/transport/httpapi/response"
	profileuc "github.com/OmerSorrell/oauth-ms/internal/usecase/profile"
)

// Profile handles GET /profile.
type Profile struct {
	log *slog.Logger
	uc  *profileuc.UseCase
}

func NewProfile(log *slog.Logger, uc *profileuc.UseCase) *Profile {
	return &Profile{log: log, uc: uc}
}

func (h *Profile) Get(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "session required")
		return
	}
	np, err := h.uc.GetProfile(r.Context(), sess)
	if err != nil {
		if errors.Is(err, internalsvc.ErrUserNotFound) {
			response.WriteError(w, http.StatusNotFound, "internal user not found")
			return
		}
		h.log.Error("profile failed", "err", err)
		response.WriteError(w, http.StatusBadGateway, "profile failed")
		return
	}
	response.WriteJSON(w, http.StatusOK, np)
}
