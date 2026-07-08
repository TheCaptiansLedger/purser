package api

import (
	"net/http"
	"purser/internal/domain"
	"purser/internal/ports"
	"time"

	"github.com/go-chi/chi/v5"
)

type musicReleaseHandler struct {
	releases ports.MusicReleaseRepository
}

func (h *musicReleaseHandler) routes(r chi.Router) {
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	r.Get("/{id}/tracks", h.listTracksByRelease)
}

func toMusicReleaseResponse(r *domain.MusicRelease) *musicReleaseResponse {
	date := ""
	if !r.Date.IsZero() {
		date = r.Date.Format("2006-01-02")
	}
	resp := &musicReleaseResponse{
		ID:             r.ID,
		GroupID:        r.GroupID,
		LibraryEntryID: r.LibraryEntryID,
		Title:          r.Title,
		Country:        r.Country,
		Date:           date,
		Label:          r.Label,
		CatalogNumber:  r.CatalogNumber,
		Barcode:        r.Barcode,
		Format:         r.Format,
		MediumCount:    r.MediumCount,
		TrackCount:     r.TrackCount,
		IsDefault:      r.IsDefault,
		Monitored:      r.Monitored,
		Status:         string(r.Status),
		CoverURL:       imageURL("music-releases", r.ID, r.CoverPath),
		AddedAt:        r.AddedAt,
		UpdatedAt:      r.UpdatedAt,
		ExternalIDs:    []externalIDResponse{},
	}
	for _, id := range r.ExternalIDs {
		resp.ExternalIDs = append(resp.ExternalIDs, externalIDResponse{
			Source: string(id.Source),
			Value:  id.Value,
		})
	}
	return resp
}

type createMusicReleaseRequest struct {
	GroupID        string `json:"groupId"`
	LibraryEntryID string `json:"libraryEntryId"`
	Title          string `json:"title"`
	Country        string `json:"country"`
	Date           string `json:"date"`
	Label          string `json:"label"`
	CatalogNumber  string `json:"catalogNumber"`
	Barcode        string `json:"barcode"`
	Format         string `json:"format"`
	MediumCount    int    `json:"mediumCount"`
	TrackCount     int    `json:"trackCount"`
	IsDefault      bool   `json:"isDefault"`
	Status         string `json:"status"`
	ExternalIDs    []struct {
		Source string `json:"source"`
		Value  string `json:"value"`
	} `json:"externalIds"`
}

func (h *musicReleaseHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createMusicReleaseRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	rel := &domain.MusicRelease{
		GroupID:        req.GroupID,
		LibraryEntryID: req.LibraryEntryID,
		Title:          req.Title,
		Country:        req.Country,
		Label:          req.Label,
		CatalogNumber:  req.CatalogNumber,
		Barcode:        req.Barcode,
		Format:         req.Format,
		MediumCount:    req.MediumCount,
		TrackCount:     req.TrackCount,
		IsDefault:      req.IsDefault,
		Status:         domain.ReleaseStatus(req.Status),
	}
	if req.Date != "" {
		if t, err := time.Parse("2006-01-02", req.Date); err == nil {
			rel.Date = t.UTC()
		}
	}
	for _, id := range req.ExternalIDs {
		rel.ExternalIDs = append(rel.ExternalIDs, domain.ExternalID{
			Source: domain.ExternalIDSource(id.Source),
			Value:  id.Value,
		})
	}
	if err := h.releases.Save(r.Context(), rel); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, toMusicReleaseResponse(rel))
}

func (h *musicReleaseHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rel, err := h.releases.Get(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toMusicReleaseResponse(rel))
}

type patchMusicReleaseRequest struct {
	Status    *string `json:"status"`
	Monitored *bool   `json:"monitored"`
}

func (h *musicReleaseHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rel, err := h.releases.Get(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	var req patchMusicReleaseRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.Status != nil {
		rel.Status = domain.ReleaseStatus(*req.Status)
	}
	if req.Monitored != nil {
		rel.Monitored = *req.Monitored
	}
	rel.UpdatedAt = time.Now().UTC()
	if err := h.releases.Save(r.Context(), rel); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toMusicReleaseResponse(rel))
}

func (h *musicReleaseHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.releases.Delete(r.Context(), id); handleErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listTracksByRelease is a stub — full track loading is implemented in Task 18 (#410).
// Returns an empty array so callers get a valid response rather than 404.
func (h *musicReleaseHandler) listTracksByRelease(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []*domain.Item{})
}

func (h *musicReleaseHandler) listByGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	releases, err := h.releases.ListByGroup(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	resp := make([]*musicReleaseResponse, 0, len(releases))
	for _, rel := range releases {
		resp = append(resp, toMusicReleaseResponse(rel))
	}
	writeJSON(w, http.StatusOK, resp)
}
