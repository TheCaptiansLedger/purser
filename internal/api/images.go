package api

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"purser/internal/app/library"
	"purser/internal/app/people"
	"purser/internal/ports"
	"strings"

	"github.com/go-chi/chi/v5"
)

// ── Image serve handler ───────────────────────────────────────────────────────

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
	case "people", "entries", "entry-banners", "items", "groups":
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
	for _, ext := range imageExtensions {
		candidate := imagePath(h.basePath, entityType, entityID, ext)
		if !strings.HasPrefix(filepath.Clean(candidate), base) {
			http.NotFound(w, r)
			return
		}
		slog.Debug("image.candidate", "path", candidate)
		if _, err := os.Stat(candidate); err == nil { //nolint:gosec
			slog.Debug("image.served", "path", candidate)
			w.Header().Set("Cache-Control", "no-cache")
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

// ── Image set/clear handler ───────────────────────────────────────────────────

const maxImageUploadBytes = 10 << 20 // 10 MB

// imageExtensions is the canonical list of extensions for stored images.
// Must stay aligned with imageUploadAllowedTypes; SVG is excluded from direct
// upload (XSS risk) but included here because it can arrive via URL download.
var imageExtensions = []string{".jpg", ".jpeg", ".png", ".webp", ".gif", ".svg"}

// imageUploadAllowedTypes maps detected MIME types to file extensions.
// SVG is intentionally excluded: it can embed scripts and is served directly by
// the UI, which makes it an XSS vector if user-supplied.
var imageUploadAllowedTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

type entityImageSetHandler struct {
	libSvc     *library.Service
	peopleSvc  *people.Service
	downloader ports.ImageDownloader
	mediaPath  string
}

func newEntityImageSetHandler(libSvc *library.Service, peopleSvc *people.Service, mediaPath string, downloader ports.ImageDownloader) *entityImageSetHandler {
	return &entityImageSetHandler{
		libSvc:     libSvc,
		peopleSvc:  peopleSvc,
		downloader: downloader,
		mediaPath:  mediaPath,
	}
}

// ── Library entries ───────────────────────────────────────────────────────────

func (h *entityImageSetHandler) setEntryImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImageUploadBytes)
	id := chi.URLParam(r, "id")
	if !safeEntityID(id) {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "invalid entity id")
		return
	}
	e, err := h.libSvc.GetEntry(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	ext, ok := h.saveImage(r, "entries", id)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "IMAGE_ERROR", "failed to save image")
		return
	}
	removeEntityImagesExcept(h.mediaPath, "entries", id, ext)
	e.ImagePath = ext
	if err := h.libSvc.SaveEntry(r.Context(), e); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toEntryResponse(e))
}

func (h *entityImageSetHandler) clearEntryImage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !safeEntityID(id) {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "invalid entity id")
		return
	}
	e, err := h.libSvc.GetEntry(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	removeEntityImages(h.mediaPath, "entries", id)
	e.ImagePath = ""
	if err := h.libSvc.SaveEntry(r.Context(), e); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toEntryResponse(e))
}

// ── Library entry banners ─────────────────────────────────────────────────────

func (h *entityImageSetHandler) setEntryBanner(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImageUploadBytes)
	id := chi.URLParam(r, "id")
	if !safeEntityID(id) {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "invalid entity id")
		return
	}
	e, err := h.libSvc.GetEntry(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	ext, ok := h.saveImage(r, "entry-banners", id)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "IMAGE_ERROR", "failed to save banner image")
		return
	}
	removeEntityImagesExcept(h.mediaPath, "entry-banners", id, ext)
	bannerURL := imageURL("entry-banners", id, ext)
	e.BannerURL = &bannerURL
	if err := h.libSvc.SaveEntry(r.Context(), e); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toEntryResponse(e))
}

func (h *entityImageSetHandler) clearEntryBanner(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !safeEntityID(id) {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "invalid entity id")
		return
	}
	e, err := h.libSvc.GetEntry(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	removeEntityImages(h.mediaPath, "entry-banners", id)
	e.BannerURL = nil
	if err := h.libSvc.SaveEntry(r.Context(), e); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toEntryResponse(e))
}

// ── Groups ────────────────────────────────────────────────────────────────────

func (h *entityImageSetHandler) setGroupImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImageUploadBytes)
	id := chi.URLParam(r, "id")
	if !safeEntityID(id) {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "invalid entity id")
		return
	}
	g, err := h.libSvc.GetGroup(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	ext, ok := h.saveImage(r, "groups", id)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "IMAGE_ERROR", "failed to save image")
		return
	}
	removeEntityImagesExcept(h.mediaPath, "groups", id, ext)
	g.CoverPath = ext
	if err := h.libSvc.SaveGroup(r.Context(), g); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toGroupResponse(g))
}

func (h *entityImageSetHandler) clearGroupImage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !safeEntityID(id) {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "invalid entity id")
		return
	}
	g, err := h.libSvc.GetGroup(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	removeEntityImages(h.mediaPath, "groups", id)
	g.CoverPath = ""
	if err := h.libSvc.SaveGroup(r.Context(), g); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toGroupResponse(g))
}

// ── Items ─────────────────────────────────────────────────────────────────────

func (h *entityImageSetHandler) setItemImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImageUploadBytes)
	id := chi.URLParam(r, "id")
	if !safeEntityID(id) {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "invalid entity id")
		return
	}
	item, err := h.libSvc.GetItem(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	ext, ok := h.saveImage(r, "items", id)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "IMAGE_ERROR", "failed to save image")
		return
	}
	removeEntityImagesExcept(h.mediaPath, "items", id, ext)
	item.CoverPath = ext
	if err := h.libSvc.SaveItem(r.Context(), item); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toItemResponse(item))
}

func (h *entityImageSetHandler) clearItemImage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !safeEntityID(id) {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "invalid entity id")
		return
	}
	item, err := h.libSvc.GetItem(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	removeEntityImages(h.mediaPath, "items", id)
	item.CoverPath = ""
	if err := h.libSvc.SaveItem(r.Context(), item); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toItemResponse(item))
}

// ── People ────────────────────────────────────────────────────────────────────

func (h *entityImageSetHandler) setPersonImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImageUploadBytes)
	id := chi.URLParam(r, "id")
	if !safeEntityID(id) {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "invalid entity id")
		return
	}
	p, err := h.peopleSvc.GetPerson(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	ext, ok := h.saveImage(r, "people", id)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "IMAGE_ERROR", "failed to save image")
		return
	}
	removeEntityImagesExcept(h.mediaPath, "people", id, ext)
	p.ImagePath = ext
	if err := h.peopleSvc.SavePerson(r.Context(), p); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toPersonResponse(p))
}

func (h *entityImageSetHandler) clearPersonImage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !safeEntityID(id) {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "invalid entity id")
		return
	}
	p, err := h.peopleSvc.GetPerson(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	removeEntityImages(h.mediaPath, "people", id)
	p.ImagePath = ""
	if err := h.peopleSvc.SavePerson(r.Context(), p); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toPersonResponse(p))
}

// ── Shared I/O helpers ────────────────────────────────────────────────────────

func (h *entityImageSetHandler) saveImage(r *http.Request, entityType, id string) (string, bool) {
	if strings.Contains(r.Header.Get("Content-Type"), "multipart") {
		return h.saveUploadedImage(r, entityType, id)
	}
	return h.saveURLImage(r, entityType, id)
}

func (h *entityImageSetHandler) saveURLImage(r *http.Request, entityType, id string) (string, bool) {
	var req struct {
		URL string `json:"url"`
	}
	if err := decode(r, &req); err != nil || req.URL == "" {
		return "", false
	}
	if !validImageURL(req.URL) {
		slog.Warn("rejected image URL: invalid scheme or host", "url", req.URL)
		return "", false
	}
	ext := h.downloader.Download(r.Context(), req.URL, entityType, id)
	return ext, ext != ""
}

func (h *entityImageSetHandler) saveUploadedImage(r *http.Request, entityType, id string) (string, bool) {
	if err := r.ParseMultipartForm(4 << 20); err != nil { //nolint:gosec // G120: body size bounded by MaxBytesReader set in each handler
		slog.Warn("multipart parse failed", "error", err)
		return "", false
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		return "", false
	}
	defer func() { _ = file.Close() }()

	// Detect content type from file bytes — never trust the client-supplied header.
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", false
	}
	head = head[:n]
	mime := strings.Split(http.DetectContentType(head), ";")[0]
	ext, allowed := imageUploadAllowedTypes[mime]
	if !allowed {
		slog.Warn("rejected upload: unsupported content type", "mime", mime)
		return "", false
	}

	destBase := imagePath(h.mediaPath, entityType, id, "")
	if err := os.MkdirAll(filepath.Dir(destBase), 0o750); err != nil { //nolint:gosec // G703: path validated by safeEntityID + HasPrefix check below
		slog.Warn("failed to create image dir", "error", err)
		return "", false
	}
	dest := filepath.Clean(destBase + ext)
	if !strings.HasPrefix(dest, filepath.Clean(h.mediaPath)) {
		slog.Warn("image path escapes media dir", "dest", dest)
		return "", false
	}

	if err := atomicWriteImage(filepath.Dir(dest), dest, head, file); err != nil {
		slog.Warn("failed to write image", "error", err)
		return "", false
	}
	return ext, true
}

// atomicWriteImage writes head + rest to a temp file in dir, then renames to dest.
// The rename is atomic on the same filesystem, preventing partial file exposure.
func atomicWriteImage(dir, dest string, head []byte, rest io.Reader) error {
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err := tmp.Write(head); err != nil {
		return err
	}
	if _, err := io.Copy(tmp, rest); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return err
	}
	committed = true
	return nil
}

// removeEntityImages deletes all known image extension variants for the entity.
func removeEntityImages(mediaPath, entityType, id string) {
	for _, ext := range imageExtensions {
		_ = os.Remove(imagePath(mediaPath, entityType, id, ext)) //nolint:gosec // G703: id validated by safeEntityID before reaching this call
	}
}

// removeEntityImagesExcept deletes all extension variants for the entity except keepExt.
// Called after a new image is saved so stale files from previous uploads or downloads
// cannot shadow the new one in the extension-ordered serve loop.
func removeEntityImagesExcept(mediaPath, entityType, id, keepExt string) {
	for _, ext := range imageExtensions {
		if ext == keepExt {
			continue
		}
		_ = os.Remove(imagePath(mediaPath, entityType, id, ext)) //nolint:gosec // G703: id validated by safeEntityID before reaching this call
	}
}

// validImageURL returns true only for http/https URLs with a non-empty host.
// This prevents SSRF via file://, ftp://, or bare-scheme URLs.
func validImageURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// imagePath returns the on-disk path for an entity image using the sharded layout:
// {base}/{entityType}/{id[0:2]}/{id}{ext}
func imagePath(base, entityType, id, ext string) string {
	shard := id
	if len(id) >= 2 {
		shard = id[:2]
	}
	return filepath.Join(base, entityType, shard, id+ext)
}

// safeEntityID rejects IDs containing path separators or double-dots.
func safeEntityID(id string) bool {
	return id != "" && !strings.ContainsAny(id, "/\\") && !strings.Contains(id, "..")
}
