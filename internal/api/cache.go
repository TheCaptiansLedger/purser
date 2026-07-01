package api

import (
	"net/http"
	"purser/pkg/cache"

	"github.com/go-chi/chi/v5"
)

type cacheHandler struct {
	caches []*cache.Cache
}

type cacheStatsResponse struct {
	Caches []cache.Stats `json:"caches"`
}

func (h *cacheHandler) routes(r chi.Router) {
	r.Get("/stats", h.stats)
	r.Delete("/{name}", h.flush)
}

func (h *cacheHandler) stats(w http.ResponseWriter, _ *http.Request) {
	stats := make([]cache.Stats, len(h.caches))
	for i, c := range h.caches {
		stats[i] = c.Stats()
	}
	writeJSON(w, http.StatusOK, cacheStatsResponse{Caches: stats})
}

func (h *cacheHandler) flush(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	for _, c := range h.caches {
		if c.Name() == name {
			c.Flush()
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", "no cache named: "+name)
}
