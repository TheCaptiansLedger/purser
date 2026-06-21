package api

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	fs "purser/internal/adapters/fs"
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

	slog.Debug("image.get", "entity_type", entityType, "entity_id", entityID)

	base := filepath.Clean(h.basePath)
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp", ".svg"} {
		candidate := fs.ImagePath(h.basePath, entityType, entityID, ext)
		if !strings.HasPrefix(filepath.Clean(candidate), base) {
			http.NotFound(w, r)
			return
		}
		slog.Debug("image.candidate", "path", candidate)
		if _, err := os.Stat(candidate); err == nil { //nolint:gosec
			slog.Debug("image.served", "path", candidate)
			http.ServeFile(w, r, candidate) //nolint:gosec
			return
		}
	}

	slog.Debug("image.not_found", "entity_type", entityType, "entity_id", entityID)
	http.NotFound(w, r)
}

// imageURL returns the API path for an entity's image, or empty string if no image is stored.
func imageURL(entityType, entityID, imagePath string) string {
	if imagePath == "" {
		return ""
	}
	return "/api/v1/images/" + entityType + "/" + entityID
}
