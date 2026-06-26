package api

import (
	"errors"
	"net/http"
	"purser/internal/app/errs"
	"purser/internal/ports"
)

type setupHandler struct {
	config ports.ConfigService
}

type setupStatusResponse struct {
	Complete bool `json:"complete"`
}

func (h *setupHandler) status(w http.ResponseWriter, r *http.Request) {
	val, err := h.config.Get(r.Context(), "setup_complete")
	if err != nil && !errors.Is(err, errs.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read setting")
		return
	}
	writeJSON(w, http.StatusOK, setupStatusResponse{Complete: val == "true"})
}

func (h *setupHandler) complete(w http.ResponseWriter, r *http.Request) {
	if err := h.config.Set(r.Context(), "setup_complete", "true"); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save setting")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
