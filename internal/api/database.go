package api

import (
	"net/http"
	"purser/internal/ports"

	"github.com/go-chi/chi/v5"
)

type databaseHandler struct {
	store ports.StorageAdminPort
}

func (h *databaseHandler) routes(r chi.Router, shutdownFn func()) {
	r.Get("/stats", h.stats)
	r.Get("/backup", h.backup)
	r.Post("/restore", func(w http.ResponseWriter, r *http.Request) {
		h.restore(w, r, shutdownFn)
	})
}

type restoreResponse struct {
	Message     string                  `json:"message"`
	Collections []ports.CollectionStats `json:"collections"`
	TotalRows   int64                   `json:"total_rows"`
}

func (h *databaseHandler) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STATS_ERROR", "failed to collect database stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *databaseHandler) backup(w http.ResponseWriter, r *http.Request) {
	ct, filename := h.store.BackupMeta()
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)

	if err := h.store.Backup(r.Context(), w); err != nil {
		// Headers already sent; we can only log at this point.
		return
	}
}

func (h *databaseHandler) restore(w http.ResponseWriter, r *http.Request, shutdownFn func()) {
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)

	if err := r.ParseMultipartForm(32 << 20); err != nil { //nolint:gosec
		writeError(w, http.StatusBadRequest, "PARSE_ERROR", "failed to parse upload")
		return
	}

	file, _, err := r.FormFile("database")
	if err != nil {
		writeError(w, http.StatusBadRequest, "FILE_MISSING", "database file is required")
		return
	}
	defer func() { _ = file.Close() }()

	stats, err := h.store.Restore(r.Context(), file, shutdownFn)
	if err != nil {
		writeError(w, http.StatusBadRequest, "RESTORE_ERROR", err.Error())
		return
	}

	var totalRows int64
	for _, c := range stats.Collections {
		totalRows += c.Count
	}
	writeJSON(w, http.StatusOK, restoreResponse{
		Message:     "Database restored successfully. Server is restarting.",
		Collections: stats.Collections,
		TotalRows:   totalRows,
	})
}
