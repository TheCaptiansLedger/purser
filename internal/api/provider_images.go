package api

import (
	"net/http"
	"purser/internal/app/errs"
	"purser/internal/app/metadata"
	"purser/internal/domain"

	"github.com/go-chi/chi/v5"
)

type providerImagesHandler struct {
	svc *metadata.Service
}

func (h *providerImagesHandler) forEntry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	images, err := h.svc.FetchImagesForEntry(r.Context(), id)
	if err != nil {
		if errs.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "library entry not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "FETCH_ERROR", "failed to fetch images")
		return
	}
	writeJSON(w, http.StatusOK, toProviderImageResponses(images))
}

func (h *providerImagesHandler) forGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	images, err := h.svc.FetchImagesForGroup(r.Context(), id)
	if err != nil {
		if errs.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "group not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "FETCH_ERROR", "failed to fetch images")
		return
	}
	writeJSON(w, http.StatusOK, toProviderImageResponses(images))
}

func (h *providerImagesHandler) forItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	images, err := h.svc.FetchImagesForItem(r.Context(), id)
	if err != nil {
		if errs.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "FETCH_ERROR", "failed to fetch images")
		return
	}
	writeJSON(w, http.StatusOK, toProviderImageResponses(images))
}

func (h *providerImagesHandler) forPerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	images, err := h.svc.FetchImagesForPerson(r.Context(), id)
	if err != nil {
		if errs.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "person not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "FETCH_ERROR", "failed to fetch images")
		return
	}
	writeJSON(w, http.StatusOK, toProviderImageResponses(images))
}

type providerImageResponse struct {
	URL    string `json:"url"`
	Type   string `json:"type"`
	Source string `json:"source"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func toProviderImageResponses(images []domain.ExternalImage) []providerImageResponse {
	out := make([]providerImageResponse, len(images))
	for i, img := range images {
		out[i] = providerImageResponse{
			URL:    img.URL,
			Type:   string(img.Type),
			Source: img.Source,
			Width:  img.Width,
			Height: img.Height,
		}
	}
	return out
}
