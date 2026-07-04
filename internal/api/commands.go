package api

import (
	"net/http"
	"purser/internal/app/metadata"
	"purser/internal/app/scan"
	"purser/internal/config"
	"purser/internal/domain"

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
		job, err := h.scanSvc.SubmitScanAllRootsJob(r.Context(), modulesFromConfig(h.cfg))
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

// modulesFromConfig builds per-content-type ScanModules from the enabled modules in config.
// Each module carries its explicit ContentType so the scanner uses the correct extension filter.
func modulesFromConfig(cfg *config.Config) []scan.Module {
	type entry struct {
		mc config.ModuleConfig
		ct domain.ContentType
	}
	all := []entry{
		{cfg.Modules.Movies, domain.ContentTypeMovie},
		{cfg.Modules.TV, domain.ContentTypeTV},
		{cfg.Modules.Music, domain.ContentTypeMusic},
		{cfg.Modules.Books, domain.ContentTypeBook},
		{cfg.Modules.AfterDark, domain.ContentTypeAdult},
		{cfg.Modules.JAV, domain.ContentTypeJAV},
	}
	var modules []scan.Module
	for _, e := range all {
		if e.mc.Enabled && len(e.mc.Roots) > 0 {
			modules = append(modules, scan.Module{ContentType: e.ct, Roots: e.mc.Roots})
		}
	}
	return modules
}
