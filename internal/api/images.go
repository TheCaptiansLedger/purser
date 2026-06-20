package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

type imageHandler struct {
	basePath string
}

func (h *imageHandler) routes(r chi.Router) {
	r.Get("/{entityType}/{entityID}", h.get)
}

func (h *imageHandler) get(w http.ResponseWriter, r *http.Request) {
	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityID")

	switch entityType {
	case "people", "entries", "items", "groups":
	default:
		http.NotFound(w, r)
		return
	}

	// Reject any path traversal attempts.
	if strings.ContainsAny(entityID, "/\\") || strings.Contains(entityID, "..") {
		http.NotFound(w, r)
		return
	}

	shard := entityID
	if len(entityID) >= 2 {
		shard = entityID[:2]
	}
	base := filepath.Clean(h.basePath)
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp", ".svg"} {
		candidate := filepath.Join(h.basePath, entityType, shard, entityID+ext)
		if !strings.HasPrefix(filepath.Clean(candidate), base) {
			http.NotFound(w, r)
			return
		}
		if _, err := os.Stat(candidate); err == nil { //nolint:gosec
			http.ServeFile(w, r, candidate) //nolint:gosec
			return
		}
	}

	http.NotFound(w, r)
}

// imageURL returns the API path for an entity's image, or empty string if no image is stored.
func imageURL(entityType, entityID, imagePath string) string {
	if imagePath == "" {
		return ""
	}
	return "/api/v1/images/" + entityType + "/" + entityID
}
