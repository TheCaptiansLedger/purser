package api

import (
	"net/http"
	"purser/internal/app/people"
	"purser/internal/domain"
	"purser/internal/ports"
	"time"

	"github.com/go-chi/chi/v5"
)

type peopleHandler struct {
	svc *people.Service
}

func (h *peopleHandler) routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)
}

// ── Response types ────────────────────────────────────────────────────────────

type personResponse struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	SortName     string               `json:"sortName"`
	Overview     string               `json:"overview"`
	Monitored    bool                 `json:"monitored"`
	MonitorMode  string               `json:"monitorMode"`
	ImageURL     string               `json:"imageUrl,omitempty"`
	Aliases      []string             `json:"aliases"`
	ExternalIDs  []externalIDResponse `json:"externalIds"`
	Metadata     map[string]any       `json:"metadata,omitempty"`
	LockedFields []string             `json:"lockedFields"`
	AddedAt      time.Time            `json:"addedAt"`
}

func toPersonResponse(p *domain.Person) *personResponse {
	r := &personResponse{
		ID:           p.ID,
		Name:         p.Name,
		SortName:     p.SortName,
		Overview:     p.Overview,
		Monitored:    p.Monitored,
		MonitorMode:  string(p.MonitorMode),
		ImageURL:     imageURL("people", p.ID, p.ImagePath),
		Metadata:     p.Metadata,
		LockedFields: p.LockedFields,
		AddedAt:      p.AddedAt,
		Aliases:      []string{},
		ExternalIDs:  []externalIDResponse{},
	}
	if r.LockedFields == nil {
		r.LockedFields = []string{}
	}
	if len(p.Aliases) > 0 {
		r.Aliases = p.Aliases
	}
	for _, id := range p.ExternalIDs {
		r.ExternalIDs = append(r.ExternalIDs, externalIDResponse{
			Source: string(id.Source),
			Value:  id.Value,
		})
	}
	return r
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *peopleHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r)
	q := r.URL.Query()

	people, total, err := h.svc.ListPeople(r.Context(), ports.PersonFilter{
		ContentType: domain.ContentType(q.Get("contentType")),
		Monitored:   boolPtr(r, "monitored"),
		Role:        domain.PersonRole(q.Get("role")),
		Search:      q.Get("search"),
		Limit:       limit,
		Offset:      offset,
	})
	if handleErr(w, err) {
		return
	}

	resp := make([]*personResponse, len(people))
	for i, p := range people {
		resp[i] = toPersonResponse(p)
	}
	writeJSON(w, http.StatusOK, page[*personResponse]{Data: resp, Total: total, Limit: limit, Offset: offset})
}

func (h *peopleHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.svc.GetPerson(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toPersonResponse(p))
}

type createPersonRequest struct {
	Name        string   `json:"name"`
	SortName    string   `json:"sortName"`
	Overview    string   `json:"overview"`
	Monitored   bool     `json:"monitored"`
	MonitorMode string   `json:"monitorMode"`
	Aliases     []string `json:"aliases"`
	ExternalIDs []struct {
		Source string `json:"source"`
		Value  string `json:"value"`
	} `json:"externalIds"`
	Metadata map[string]any `json:"metadata"`
}

func (h *peopleHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createPersonRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	p := &domain.Person{
		Name:        req.Name,
		SortName:    req.SortName,
		Overview:    req.Overview,
		Monitored:   req.Monitored,
		MonitorMode: domain.MonitorMode(req.MonitorMode),
		Aliases:     req.Aliases,
		Metadata:    req.Metadata,
	}
	for _, id := range req.ExternalIDs {
		p.ExternalIDs = append(p.ExternalIDs, domain.ExternalID{
			Source: domain.ExternalIDSource(id.Source),
			Value:  id.Value,
		})
	}

	if err := h.svc.CreatePerson(r.Context(), p); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, toPersonResponse(p))
}

type patchPersonRequest struct {
	Name         *string   `json:"name"`
	SortName     *string   `json:"sortName"`
	Overview     *string   `json:"overview"`
	Monitored    *bool     `json:"monitored"`
	MonitorMode  *string   `json:"monitorMode"`
	Aliases      []string  `json:"aliases"`
	LockedFields *[]string `json:"lockedFields"`
}

func (h *peopleHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.svc.GetPerson(r.Context(), id)
	if handleErr(w, err) {
		return
	}

	var req patchPersonRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.SortName != nil {
		p.SortName = *req.SortName
	}
	if req.Overview != nil {
		p.Overview = *req.Overview
	}
	if req.Monitored != nil {
		p.Monitored = *req.Monitored
	}
	if req.MonitorMode != nil {
		p.MonitorMode = domain.MonitorMode(*req.MonitorMode)
	}
	if req.Aliases != nil {
		p.Aliases = req.Aliases
	}
	if req.LockedFields != nil {
		p.LockedFields = *req.LockedFields
	}

	if err := h.svc.SavePerson(r.Context(), p); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toPersonResponse(p))
}

func (h *peopleHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeletePerson(r.Context(), id); handleErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
