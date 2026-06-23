package api

import (
	"net/http"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"

	"github.com/go-chi/chi/v5"
)

type tagHandler struct {
	repo ports.TagRepository
}

func (h *tagHandler) routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Delete("/{id}", h.delete)
}

func (h *tagHandler) list(w http.ResponseWriter, r *http.Request) {
	f := ports.TagFilter{
		Scope: domain.TagScope(r.URL.Query().Get("scope")),
		Key:   r.URL.Query().Get("key"),
	}
	if ct := r.URL.Query().Get("contentType"); ct != "" {
		for _, s := range strings.Split(ct, ",") {
			if s = strings.TrimSpace(s); s != "" {
				f.ContentTypes = append(f.ContentTypes, domain.ContentType(s))
			}
		}
	}
	tags, err := h.repo.List(r.Context(), f)
	if handleErr(w, err) {
		return
	}

	resp := make([]tagResponse, len(tags))
	for i, t := range tags {
		resp[i] = tagResponse{ID: t.ID, Key: t.Key, Value: t.Value, Scope: string(t.Scope)}
	}
	writeJSON(w, http.StatusOK, page[tagResponse]{Data: resp, Total: len(resp), Limit: len(resp)})
}

type createTagRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Scope string `json:"scope"`
}

func (h *tagHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createTagRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.Value == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "value is required")
		return
	}

	t := &domain.Tag{
		Key:   req.Key,
		Value: req.Value,
		Scope: domain.TagScope(req.Scope),
	}
	if err := h.repo.Save(r.Context(), t); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, tagResponse{ID: t.ID, Key: t.Key, Value: t.Value, Scope: string(t.Scope)})
}

func (h *tagHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(r.Context(), id); handleErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
