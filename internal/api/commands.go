package api

import (
	"context"
	"net/http"
	"purser/internal/app/metadata"
	"purser/internal/ports"

	"github.com/go-chi/chi/v5"
)

type commandsHandler struct {
	metaSvc  *metadata.Service
	jobQueue ports.JobQueue
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

	switch req.Name {
	case "RefreshStudio":
		if req.EntryID == "" {
			writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "entryId is required")
			return
		}
		entryID := req.EntryID
		svc := h.metaSvc
		job, err := h.jobQueue.Submit(r.Context(), "RefreshStudio", map[string]any{"entry_id": entryID},
			func(ctx context.Context, p ports.ProgressReporter) error {
				return svc.RefreshStudio(ctx, entryID, p)
			})
		if handleErr(w, err) {
			return
		}
		writeJSON(w, http.StatusAccepted, jobToResponse(job))

	case "RefreshArtist":
		if req.EntryID == "" {
			writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "entryId is required")
			return
		}
		entryID := req.EntryID
		svc := h.metaSvc
		job, err := h.jobQueue.Submit(r.Context(), "RefreshArtist", map[string]any{"entry_id": entryID},
			func(ctx context.Context, p ports.ProgressReporter) error {
				return svc.RefreshArtist(ctx, entryID, p)
			})
		if handleErr(w, err) {
			return
		}
		writeJSON(w, http.StatusAccepted, jobToResponse(job))

	default:
		writeError(w, http.StatusBadRequest, "UNKNOWN_COMMAND", "unknown command: "+req.Name)
	}
}
