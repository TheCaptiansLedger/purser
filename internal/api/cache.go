package api

import (
	"net/http"
	"purser/pkg/cache"
)

type cacheHandler struct {
	c *cache.Cache
}

func (h *cacheHandler) stats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.c.Stats())
}
