package api

import (
	"errors"
	"log/slog"
	"net/http"
	"purser/internal/app/errs"
	"purser/internal/app/library"
	"purser/internal/app/metadata"
	"purser/internal/domain"
	"purser/internal/ports"
	"time"

	"github.com/go-chi/chi/v5"
)

type libraryEntryHandler struct {
	svc *library.Service
}

func (h *libraryEntryHandler) routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Get("/{id}/children", h.children)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	r.Put("/{id}/people", h.addPerson)
	r.Delete("/{id}/people/{personId}", h.removePerson)
	r.Post("/{id}/tags", h.addTag)
	r.Delete("/{id}/tags/{tagId}", h.removeTag)
}

// ── Response types ────────────────────────────────────────────────────────────

type entryResponse struct {
	ID                string                `json:"id"`
	ContentType       string                `json:"contentType"`
	Kind              string                `json:"kind"`
	Name              string                `json:"name"`
	SortName          string                `json:"sortName"`
	Overview          string                `json:"overview"`
	ParentID          string                `json:"parentId,omitempty"`
	Monitored         bool                  `json:"monitored"`
	MonitorMode       string                `json:"monitorMode"`
	Status            string                `json:"status"`
	QualityProfileID  string                `json:"qualityProfileId,omitempty"`
	MetadataProfileID string                `json:"metadataProfileId,omitempty"`
	Path              string                `json:"path,omitempty"`
	ImageURL          string                `json:"imageUrl,omitempty"`
	ExternalIDs       []externalIDResponse  `json:"externalIds"`
	Tags              []tagResponse         `json:"tags"`
	People            []entryPersonResponse `json:"people"`
	Metadata          map[string]any        `json:"metadata,omitempty"`
	LockedFields      []string              `json:"lockedFields"`
	AddedAt           time.Time             `json:"addedAt"`
	UpdatedAt         time.Time             `json:"updatedAt"`
}

func toEntryResponse(e *domain.LibraryEntry) *entryResponse {
	r := &entryResponse{
		ID:                e.ID,
		ContentType:       string(e.ContentType),
		Kind:              string(e.Kind),
		Name:              e.Name,
		SortName:          e.SortName,
		Overview:          e.Overview,
		ParentID:          e.ParentID,
		Monitored:         e.Monitored,
		MonitorMode:       string(e.MonitorMode),
		Status:            string(e.Status),
		QualityProfileID:  e.QualityProfileID,
		MetadataProfileID: e.MetadataProfileID,
		Path:              e.Path,
		ImageURL:          imageURL("entries", e.ID, e.ImagePath),
		Metadata:          e.Metadata,
		AddedAt:           e.AddedAt,
		UpdatedAt:         e.UpdatedAt,
		ExternalIDs:       []externalIDResponse{},
		Tags:              []tagResponse{},
		People:            []entryPersonResponse{},
		LockedFields:      e.LockedFields,
	}
	if r.LockedFields == nil {
		r.LockedFields = []string{}
	}
	for _, id := range e.ExternalIDs {
		r.ExternalIDs = append(r.ExternalIDs, externalIDResponse{
			Source: string(id.Source),
			Value:  id.Value,
		})
	}
	for _, t := range e.Tags {
		r.Tags = append(r.Tags, tagResponse{ID: t.ID, Key: string(t.Key), Value: t.Value, Scope: string(t.Scope)})
	}
	for _, ep := range e.People {
		epr := entryPersonResponse{
			PersonID: ep.PersonID,
			Role:     ep.Role,
		}
		if !ep.StartDate.IsZero() {
			epr.StartDate = ep.StartDate.Format("2006-01-02")
		}
		if !ep.EndDate.IsZero() {
			epr.EndDate = ep.EndDate.Format("2006-01-02")
		}
		if ep.Person != nil {
			epr.Person = &personRefResponse{
				ID:       ep.Person.ID,
				Name:     ep.Person.Name,
				SortName: ep.Person.SortName,
				ImageURL: imageURL("people", ep.Person.ID, ep.Person.ImagePath),
			}
		}
		r.People = append(r.People, epr)
	}
	return r
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *libraryEntryHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r)
	q := r.URL.Query()

	entries, total, err := h.svc.ListEntries(r.Context(), ports.LibraryFilter{
		ContentType: domain.ContentType(q.Get("contentType")),
		Kind:        domain.Kind(q.Get("kind")),
		ParentID:    q.Get("parentId"),
		PersonID:    q.Get("personId"),
		Monitored:   boolPtr(r, "monitored"),
		Search:      q.Get("search"),
		TagKey:      domain.TagKey(q.Get("tag_key")),
		TagValue:    q.Get("tag_value"),
		Limit:       limit,
		Offset:      offset,
	})
	if handleErr(w, err) {
		return
	}

	resp := make([]*entryResponse, len(entries))
	for i, e := range entries {
		resp[i] = toEntryResponse(e)
	}
	writeJSON(w, http.StatusOK, page[*entryResponse]{Data: resp, Total: total, Limit: limit, Offset: offset})
}

func (h *libraryEntryHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := h.svc.GetEntry(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toEntryResponse(e))
}

func (h *libraryEntryHandler) children(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	entries, total, err := h.svc.ListChildren(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	resp := make([]*entryResponse, len(entries))
	for i, e := range entries {
		resp[i] = toEntryResponse(e)
	}
	writeJSON(w, http.StatusOK, page[*entryResponse]{Data: resp, Total: total, Limit: 200, Offset: 0})
}

type createEntryRequest struct {
	ContentType       string `json:"contentType"`
	Kind              string `json:"kind"`
	Name              string `json:"name"`
	SortName          string `json:"sortName"`
	Overview          string `json:"overview"`
	ParentID          string `json:"parentId"`
	Monitored         bool   `json:"monitored"`
	MonitorMode       string `json:"monitorMode"`
	QualityProfileID  string `json:"qualityProfileId"`
	MetadataProfileID string `json:"metadataProfileId"`
	Path              string `json:"path"`
	ExternalIDs       []struct {
		Source string `json:"source"`
		Value  string `json:"value"`
	} `json:"externalIds"`
	Metadata map[string]any `json:"metadata"`
}

func (h *libraryEntryHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createEntryRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	e := &domain.LibraryEntry{
		ContentType:       domain.ContentType(req.ContentType),
		Kind:              domain.Kind(req.Kind),
		Name:              req.Name,
		SortName:          req.SortName,
		Overview:          req.Overview,
		ParentID:          req.ParentID,
		Monitored:         req.Monitored,
		MonitorMode:       domain.MonitorMode(req.MonitorMode),
		QualityProfileID:  req.QualityProfileID,
		MetadataProfileID: req.MetadataProfileID,
		Path:              req.Path,
		Metadata:          req.Metadata,
	}
	for _, id := range req.ExternalIDs {
		e.ExternalIDs = append(e.ExternalIDs, domain.ExternalID{
			Source: domain.ExternalIDSource(id.Source),
			Value:  id.Value,
		})
	}

	if err := h.svc.CreateEntry(r.Context(), e); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, toEntryResponse(e))
}

type externalIDPatchItem struct {
	Source string `json:"source"`
	Value  string `json:"value"`
}

type patchEntryRequest struct {
	Name              *string                `json:"name"`
	SortName          *string                `json:"sortName"`
	Overview          *string                `json:"overview"`
	ParentID          *string                `json:"parentId"`
	Monitored         *bool                  `json:"monitored"`
	MonitorMode       *string                `json:"monitorMode"`
	Status            *string                `json:"status"`
	QualityProfileID  *string                `json:"qualityProfileId"`
	MetadataProfileID *string                `json:"metadataProfileId"`
	Path              *string                `json:"path"`
	LockedFields      *[]string              `json:"lockedFields"`
	ExternalIDs       *[]externalIDPatchItem `json:"externalIds"`
}

func applyEntryPatch(e *domain.LibraryEntry, req *patchEntryRequest) {
	if req.Name != nil {
		e.Name = *req.Name
	}
	if req.SortName != nil {
		e.SortName = *req.SortName
	}
	if req.Overview != nil {
		e.Overview = *req.Overview
	}
	if req.ParentID != nil {
		e.ParentID = *req.ParentID
	}
	if req.Monitored != nil {
		e.Monitored = *req.Monitored
	}
	if req.MonitorMode != nil {
		e.MonitorMode = domain.MonitorMode(*req.MonitorMode)
	}
	if req.Status != nil {
		e.Status = domain.EntryStatus(*req.Status)
	}
	if req.QualityProfileID != nil {
		e.QualityProfileID = *req.QualityProfileID
	}
	if req.MetadataProfileID != nil {
		e.MetadataProfileID = *req.MetadataProfileID
	}
	if req.Path != nil {
		e.Path = *req.Path
	}
	if req.LockedFields != nil {
		e.LockedFields = *req.LockedFields
	}
	if req.ExternalIDs != nil {
		e.ExternalIDs = nil
		for _, id := range *req.ExternalIDs {
			e.ExternalIDs = append(e.ExternalIDs, domain.ExternalID{
				Source: domain.ExternalIDSource(id.Source),
				Value:  id.Value,
			})
		}
	}
}

func (h *libraryEntryHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := h.svc.GetEntry(r.Context(), id)
	if handleErr(w, err) {
		return
	}

	var req patchEntryRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	applyEntryPatch(e, &req)

	if err := h.svc.SaveEntry(r.Context(), e); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toEntryResponse(e))
}

func (h *libraryEntryHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteEntry(r.Context(), id); handleErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type addEntryPersonRequest struct {
	PersonID  string `json:"personId"`
	Role      string `json:"role"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

func (h *libraryEntryHandler) addPerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req addEntryPersonRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	ep := domain.EntryPerson{PersonID: req.PersonID, Role: req.Role}
	if req.StartDate != "" {
		ep.StartDate, _ = time.Parse("2006-01-02", req.StartDate)
	}
	if req.EndDate != "" {
		ep.EndDate, _ = time.Parse("2006-01-02", req.EndDate)
	}
	if err := h.svc.SaveEntryPerson(r.Context(), id, ep); handleErr(w, err) {
		return
	}
	e, err := h.svc.GetEntry(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toEntryResponse(e))
}

func (h *libraryEntryHandler) removePerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	personID := chi.URLParam(r, "personId")
	role := r.URL.Query().Get("role")
	if err := h.svc.RemoveEntryPerson(r.Context(), id, personID, role); handleErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type addEntryTagRequest struct {
	TagID string `json:"tagId"`
}

func (h *libraryEntryHandler) addTag(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req addEntryTagRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.TagID == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "tagId is required")
		return
	}
	if err := h.svc.TagEntry(r.Context(), id, req.TagID); handleErr(w, err) {
		return
	}
	e, err := h.svc.GetEntry(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toEntryResponse(e))
}

func (h *libraryEntryHandler) removeTag(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tagID := chi.URLParam(r, "tagId")
	if err := h.svc.UntagEntry(r.Context(), id, tagID); handleErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Shared error mapper ───────────────────────────────────────────────────────

// handleErr maps service errors to HTTP responses. Returns true if an error was written.
func handleErr(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errs.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
		return true
	}
	if errors.Is(err, metadata.ErrUnknownJob) {
		writeError(w, http.StatusUnprocessableEntity, "UNKNOWN_COMMAND", err.Error())
		return true
	}
	if errors.Is(err, library.ErrInvalidStatusForUserUpdate) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_STATUS", err.Error())
		return true
	}
	if errors.Is(err, domain.ErrInvalidTransition) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_TRANSITION", err.Error())
		return true
	}
	if errs.IsValidation(err) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		return true
	}
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
	slog.Error("handler error", "error", err)
	return true
}
