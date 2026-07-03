package api

import (
	"net/http"
	"purser/internal/app/metadata"
	"purser/internal/app/scan"
	"purser/internal/config"

	"github.com/go-chi/chi/v5"
)

type commandsHandler struct {
	metaSvc *metadata.Service
	scanSvc *scan.Service
	cfg     *config.Config
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
	case "ScanLibrary":
		job, err := h.scanSvc.SubmitScanLibraryJob(r.Context(), req.EntryID)
		if handleErr(w, err) {
			return
		}
		writeJSON(w, http.StatusAccepted, jobToResponse(job))
	case "ScanAllRoots":
		job, err := h.scanSvc.SubmitScanAllRootsJob(r.Context(), enabledRoots(h.cfg))
		if handleErr(w, err) {
			return
		}
		writeJSON(w, http.StatusAccepted, jobToResponse(job))
	default:
		job, err := h.metaSvc.SubmitRefreshJob(r.Context(), req.Name, req.EntryID)
		if handleErr(w, err) {
			return
		}
		writeJSON(w, http.StatusAccepted, jobToResponse(job))
	}
}

func enabledRoots(cfg *config.Config) []string {
	modules := []config.ModuleConfig{
		cfg.Modules.Movies,
		cfg.Modules.TV,
		cfg.Modules.Music,
		cfg.Modules.Books,
		cfg.Modules.AfterDark,
		cfg.Modules.JAV,
	}
	var roots []string
	for _, m := range modules {
		if m.Enabled {
			roots = append(roots, m.Roots...)
		}
	}
	return roots
}
