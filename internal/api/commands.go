package api

import (
	"net/http"
	"purser/internal/app/metadata"

	"github.com/go-chi/chi/v5"
)

type commandsHandler struct {
	metaSvc *metadata.Service
}

func (h *commandsHandler) routes(r chi.Router) {
	r.Post("/", h.submit)
}

type commandRequest struct {
	Name    string `json:"name"`
	EntryID string `json:"entryId"`
}

func (h *commandsHandler) submit(w http.ResponseWriter, r *http.Request) {
	var req commandRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	job, err := h.metaSvc.SubmitRefreshJob(r.Context(), req.Name, req.EntryID)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusAccepted, jobToResponse(job))
}
