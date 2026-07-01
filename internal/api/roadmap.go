package api

import (
	"net/http"
	"purser/internal/ports"

	"github.com/go-chi/chi/v5"
)

type roadmapHandler struct {
	gh ports.GitHubProxy
}

func (h *roadmapHandler) routes(r chi.Router) {
	r.Get("/issues", h.issues)
	r.Get("/releases", h.releases)
	r.Get("/contributors", h.contributors)
}

func (h *roadmapHandler) issues(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state != "open" && state != "closed" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "state must be open or closed")
		return
	}
	label := r.URL.Query().Get("labels")
	data, err := h.gh.Issues(r.Context(), state, label)
	if err != nil {
		writeError(w, http.StatusBadGateway, "GITHUB_ERROR", err.Error())
		return
	}
	writeRaw(w, data)
}

func (h *roadmapHandler) releases(w http.ResponseWriter, r *http.Request) {
	data, err := h.gh.Releases(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "GITHUB_ERROR", err.Error())
		return
	}
	writeRaw(w, data)
}

func (h *roadmapHandler) contributors(w http.ResponseWriter, r *http.Request) {
	head := r.URL.Query().Get("head")
	if head == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "head is required")
		return
	}
	base := r.URL.Query().Get("base")
	data, err := h.gh.Contributors(r.Context(), base, head)
	if err != nil {
		writeError(w, http.StatusBadGateway, "GITHUB_ERROR", err.Error())
		return
	}
	writeRaw(w, data)
}

// writeRaw writes a pre-encoded JSON payload directly without re-encoding.
// data is already validated JSON received from a trusted internal proxy
// (the GitHub adapter), not user-supplied content, so XSS is not applicable here.
func writeRaw(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data) //nolint:gosec
}
