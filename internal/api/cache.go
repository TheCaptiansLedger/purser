package api

import (
	"net/http"
	"purser/pkg/cache"
)

type cacheHandler struct {
	caches []*cache.Cache
}

type cacheStatsResponse struct {
	Caches []cache.Stats `json:"caches"`
}

func (h *cacheHandler) stats(w http.ResponseWriter, _ *http.Request) {
	stats := make([]cache.Stats, len(h.caches))
	for i, c := range h.caches {
		stats[i] = c.Stats()
	}
	writeJSON(w, http.StatusOK, cacheStatsResponse{Caches: stats})
}
