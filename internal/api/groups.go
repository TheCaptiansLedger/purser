package api

import (
	"net/http"
	"purser/internal/app/library"
	"purser/internal/domain"
	"purser/internal/ports"

	"github.com/go-chi/chi/v5"
)

type groupHandler struct {
	svc *library.Service
}

func (h *groupHandler) routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Get("/{id}/deletion-preview", h.deletionPreview)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	r.Post("/{id}/tags", h.addTag)
	r.Delete("/{id}/tags/{tagId}", h.removeTag)
}

// ── Response types ────────────────────────────────────────────────────────────

type groupResponse struct {
	ID             string               `json:"id"`
	LibraryEntryID string               `json:"libraryEntryId"`
	Title          string               `json:"title"`
	SortName       string               `json:"sortName"`
	Number         int                  `json:"number"`
	Year           int                  `json:"year"`
	Overview       string               `json:"overview"`
	Monitored      bool                 `json:"monitored"`
	MonitorMode    string               `json:"monitorMode"`
	CoverURL       string               `json:"coverUrl,omitempty"`
	ExternalIDs    []externalIDResponse `json:"externalIds"`
	Tags           []tagResponse        `json:"tags"`
	Metadata       map[string]any       `json:"metadata,omitempty"`
	LockedFields   []string             `json:"lockedFields"`
}

func toGroupResponse(g *domain.Group) *groupResponse {
	r := &groupResponse{
		ID:             g.ID,
		LibraryEntryID: g.LibraryEntryID,
		Title:          g.Title,
		SortName:       g.SortName,
		Number:         g.Number,
		Year:           g.Year,
		Overview:       g.Overview,
		Monitored:      g.Monitored,
		MonitorMode:    string(g.MonitorMode),
		CoverURL:       imageURL("groups", g.ID, g.CoverPath),
		Metadata:       g.Metadata,
		ExternalIDs:    []externalIDResponse{},
		Tags:           []tagResponse{},
		LockedFields:   g.LockedFields,
	}
	if r.LockedFields == nil {
		r.LockedFields = []string{}
	}
	for _, id := range g.ExternalIDs {
		r.ExternalIDs = append(r.ExternalIDs, externalIDResponse{
			Source: string(id.Source),
			Value:  id.Value,
		})
	}
	for _, t := range g.Tags {
		r.Tags = append(r.Tags, tagResponse{ID: t.ID, Key: string(t.Key), Value: t.Value, Scope: string(t.Scope)})
	}
	return r
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *groupHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	groups, err := h.svc.ListGroups(r.Context(), ports.GroupFilter{
		LibraryEntryID: q.Get("libraryEntryId"),
		Monitored:      boolPtr(r, "monitored"),
		TagKey:         domain.TagKey(q.Get("tag_key")),
		TagValue:       q.Get("tag_value"),
	})
	if handleErr(w, err) {
		return
	}
	resp := make([]*groupResponse, len(groups))
	for i, g := range groups {
		resp[i] = toGroupResponse(g)
	}
	writeJSON(w, http.StatusOK, page[*groupResponse]{
		Data:  resp,
		Total: len(resp),
		Limit: 200,
	})
}

func (h *groupHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	g, err := h.svc.GetGroup(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toGroupResponse(g))
}

type createGroupRequest struct {
	LibraryEntryID string `json:"libraryEntryId"`
	Title          string `json:"title"`
	SortName       string `json:"sortName"`
	Number         int    `json:"number"`
	Year           int    `json:"year"`
	Overview       string `json:"overview"`
	Monitored      bool   `json:"monitored"`
	MonitorMode    string `json:"monitorMode"`
	ExternalIDs    []struct {
		Source string `json:"source"`
		Value  string `json:"value"`
	} `json:"externalIds"`
	Metadata map[string]any `json:"metadata"`
}

func (h *groupHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createGroupRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	g := &domain.Group{
		LibraryEntryID: req.LibraryEntryID,
		Title:          req.Title,
		SortName:       req.SortName,
		Number:         req.Number,
		Year:           req.Year,
		Overview:       req.Overview,
		Monitored:      req.Monitored,
		MonitorMode:    domain.MonitorMode(req.MonitorMode),
		Metadata:       req.Metadata,
	}
	for _, id := range req.ExternalIDs {
		g.ExternalIDs = append(g.ExternalIDs, domain.ExternalID{
			Source: domain.ExternalIDSource(id.Source),
			Value:  id.Value,
		})
	}

	if err := h.svc.CreateGroup(r.Context(), g); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, toGroupResponse(g))
}

type patchGroupRequest struct {
	Title        *string   `json:"title"`
	SortName     *string   `json:"sortName"`
	Number       *int      `json:"number"`
	Year         *int      `json:"year"`
	Overview     *string   `json:"overview"`
	Monitored    *bool     `json:"monitored"`
	MonitorMode  *string   `json:"monitorMode"`
	LockedFields *[]string `json:"lockedFields"`
}

func (h *groupHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	g, err := h.svc.GetGroup(r.Context(), id)
	if handleErr(w, err) {
		return
	}

	var req patchGroupRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if req.Title != nil {
		g.Title = *req.Title
	}
	if req.SortName != nil {
		g.SortName = *req.SortName
	}
	if req.Number != nil {
		g.Number = *req.Number
	}
	if req.Year != nil {
		g.Year = *req.Year
	}
	if req.Overview != nil {
		g.Overview = *req.Overview
	}
	if req.Monitored != nil {
		g.Monitored = *req.Monitored
	}
	if req.MonitorMode != nil {
		g.MonitorMode = domain.MonitorMode(*req.MonitorMode)
	}
	if req.LockedFields != nil {
		g.LockedFields = *req.LockedFields
	}

	if err := h.svc.SaveGroup(r.Context(), g); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toGroupResponse(g))
}

func (h *groupHandler) deletionPreview(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	impact, err := h.svc.DeletionImpactOfGroup(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, impact)
}

func (h *groupHandler) delete(w http.ResponseWriter, r *http.Request) {
	if !requireDeleteConfirm(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteGroup(r.Context(), id); handleErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type addGroupTagRequest struct {
	TagID string `json:"tagId"`
}

func (h *groupHandler) addTag(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req addGroupTagRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if req.TagID == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "tagId is required")
		return
	}
	if err := h.svc.AddGroupTag(r.Context(), id, req.TagID); handleErr(w, err) {
		return
	}
	g, err := h.svc.GetGroup(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toGroupResponse(g))
}

func (h *groupHandler) removeTag(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tagID := chi.URLParam(r, "tagId")
	if err := h.svc.RemoveGroupTag(r.Context(), id, tagID); handleErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
