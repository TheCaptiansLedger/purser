package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"purser/internal/app/errs"
	"purser/internal/app/library"
	"purser/internal/app/people"
	"purser/internal/domain"
	"purser/internal/ports"

	"github.com/go-chi/chi/v5"
)

type entityImagesHandler struct {
	repo      ports.ImageRepository
	libSvc    *library.Service
	peopleSvc *people.Service
}

type imageItemResponse struct {
	ID        string `json:"id"`
	ImageType string `json:"imageType"`
	URL       string `json:"url"`
	Source    string `json:"source"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Selected  bool   `json:"selected"`
}

type imageListResponse struct {
	Images []imageItemResponse `json:"images"`
}

func (h *entityImagesHandler) routesFor(entityType string) func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/", h.list(entityType))
		r.Put("/{imageType}/selected", h.setSelection(entityType))
		r.Delete("/{imageType}/selected", h.clearSelection(entityType))
	}
}

func (h *entityImagesHandler) list(entityType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		ctx := r.Context()

		var filter *domain.ImageType
		if t := r.URL.Query().Get("type"); t != "" {
			it := domain.ImageType(t)
			filter = &it
		}

		images, err := h.repo.List(ctx, entityType, id, filter)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list images")
			slog.Error("list images", "error", err)
			return
		}

		typesPresent := map[domain.ImageType]struct{}{}
		for _, img := range images {
			typesPresent[img.ImageType] = struct{}{}
		}
		selectedByType := map[domain.ImageType]string{}
		for it := range typesPresent {
			sel, err := h.repo.GetSelection(ctx, entityType, id, it)
			if err == nil && sel != nil {
				selectedByType[it] = sel.ID
			}
		}

		result := make([]imageItemResponse, len(images))
		for i, img := range images {
			result[i] = imageItemResponse{
				ID:        img.ID,
				ImageType: string(img.ImageType),
				URL:       img.URL,
				Source:    img.Source,
				Width:     img.Width,
				Height:    img.Height,
				Selected:  selectedByType[img.ImageType] == img.ID,
			}
		}
		writeJSON(w, http.StatusOK, imageListResponse{Images: result})
	}
}

func (h *entityImagesHandler) setSelection(entityType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		imageTypeStr := chi.URLParam(r, "imageType")
		ctx := r.Context()

		ct, err := h.contentTypeForEntity(ctx, entityType, id)
		if handleErr(w, err) {
			return
		}

		it := domain.ImageType(imageTypeStr)
		if !imageTypeApplicable(it, entityType, string(ct)) {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_IMAGE_TYPE",
				fmt.Sprintf("image type %q is not valid for this entity", imageTypeStr))
			return
		}

		var body struct {
			ImageID string `json:"image_id"`
		}
		if err := decode(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
			return
		}

		images, err := h.repo.List(ctx, entityType, id, &it)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to verify image")
			slog.Error("verify image for selection", "error", err)
			return
		}
		var found *domain.StoredImage
		for i := range images {
			if images[i].ID == body.ImageID {
				found = &images[i]
				break
			}
		}
		if found == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "image not found for this entity and type")
			return
		}

		if err := h.repo.SetSelection(ctx, entityType, id, it, body.ImageID); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to set selection")
			slog.Error("set image selection", "error", err)
			return
		}

		writeJSON(w, http.StatusOK, imageItemResponse{
			ID:        found.ID,
			ImageType: string(found.ImageType),
			URL:       found.URL,
			Source:    found.Source,
			Width:     found.Width,
			Height:    found.Height,
			Selected:  true,
		})
	}
}

func (h *entityImagesHandler) clearSelection(entityType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		imageTypeStr := chi.URLParam(r, "imageType")
		ctx := r.Context()

		if err := h.repo.ClearSelection(ctx, entityType, id, domain.ImageType(imageTypeStr)); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to clear selection")
			slog.Error("clear image selection", "error", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// contentTypeForEntity returns the content type for an entity, performing a DB lookup.
// Groups do not carry content type directly — it is inherited from the parent library entry.
// For person entities, content type is not applicable; an empty string is returned.
func (h *entityImagesHandler) contentTypeForEntity(ctx context.Context, entityType, entityID string) (domain.ContentType, error) {
	switch entityType {
	case "library_entry":
		e, err := h.libSvc.GetEntry(ctx, entityID)
		if err != nil {
			return "", err
		}
		return e.ContentType, nil
	case "group":
		g, err := h.libSvc.GetGroup(ctx, entityID)
		if err != nil {
			return "", err
		}
		parent, err := h.libSvc.GetEntry(ctx, g.LibraryEntryID)
		if err != nil {
			return "", err
		}
		return parent.ContentType, nil
	case "item":
		item, err := h.libSvc.GetItem(ctx, entityID)
		if err != nil {
			return "", err
		}
		return item.ContentType, nil
	case "person":
		if _, err := h.peopleSvc.GetPerson(ctx, entityID); err != nil {
			return "", err
		}
		return "", nil
	default:
		return "", errs.ErrNotFound
	}
}

func imageTypeApplicable(it domain.ImageType, entityType, contentType string) bool {
	for _, t := range domain.ApplicableImageTypes(contentType, entityType) {
		if t == it {
			return true
		}
	}
	return false
}
