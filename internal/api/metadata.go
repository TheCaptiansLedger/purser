package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"purser/internal/app/metadata"
	"purser/internal/domain"
)

type metadataHandler struct {
	svc *metadata.Service
}

func (h *metadataHandler) routes(r chi.Router) {
	r.Get("/search", h.search)
	r.Post("/studios/import", h.importStudio)
	r.Post("/people/import", h.importPerson)
}

// ── Search ────────────────────────────────────────────────────────────────────

// GET /api/v1/metadata/search?kind=studio&q=bratty&contentType=adult&limit=20
func (h *metadataHandler) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind := q.Get("kind")
	query := q.Get("q")
	contentType := domain.ContentType(q.Get("contentType"))
	limit := 25
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 {
		limit = l
	}

	if query == "" {
		writeError(w, http.StatusBadRequest, "MISSING_QUERY", "q is required")
		return
	}

	switch kind {
	case "studio":
		studios, err := h.svc.SearchStudios(r.Context(), query, contentType, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SEARCH_ERROR", "search failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": toExternalStudioResponses(studios)})

	case "person":
		people, err := h.svc.SearchPeople(r.Context(), query, contentType, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SEARCH_ERROR", "search failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": toExternalPersonResponses(people)})

	default:
		writeError(w, http.StatusBadRequest, "INVALID_KIND", "kind must be studio or person")
	}
}

// ── Import studio ─────────────────────────────────────────────────────────────

type importStudioRequest struct {
	Source             string `json:"source"`
	ExternalID         string `json:"externalId"`
	Name               string `json:"name"`
	Overview           string `json:"overview"`
	ContentType        string `json:"contentType"`
	Monitored          bool   `json:"monitored"`
	MonitorMode        string `json:"monitorMode"`
	ParentExternalID   string `json:"parentExternalId"`
	ParentName         string `json:"parentName"`
	ParentImageURL     string `json:"parentImageUrl"`
	ParentWebsiteURL   string `json:"parentWebsiteUrl"`
	ImageURL           string `json:"imageUrl"`
	WebsiteURL         string `json:"websiteUrl"`
}

// POST /api/v1/metadata/studios/import
func (h *metadataHandler) importStudio(w http.ResponseWriter, r *http.Request) {
	var req importStudioRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	if req.Source == "" || req.ExternalID == "" || req.Name == "" || req.ContentType == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "source, externalId, name, and contentType are required")
		return
	}

	svcReq := &metadata.ImportStudioRequest{
		Source:           domain.ExternalIDSource(req.Source),
		ExternalID:       req.ExternalID,
		Name:             req.Name,
		Overview:         req.Overview,
		ContentType:      domain.ContentType(req.ContentType),
		Monitored:        req.Monitored,
		MonitorMode:      domain.MonitorMode(req.MonitorMode),
		ParentExternalID: req.ParentExternalID,
		ParentName:       req.ParentName,
		ParentImageURL:   req.ParentImageURL,
		ParentWebsiteURL: req.ParentWebsiteURL,
		ImageURL:         req.ImageURL,
		WebsiteURL:       req.WebsiteURL,
	}

	result, err := h.svc.ImportStudio(r.Context(), svcReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "IMPORT_ERROR", "import failed")
		return
	}

	resp := map[string]any{
		"studio": toEntryResponse(result.Studio),
	}
	if result.Network != nil {
		resp["network"] = toEntryResponse(result.Network)
	}
	writeJSON(w, http.StatusCreated, resp)
}

// ── Import person ─────────────────────────────────────────────────────────────

type importPersonRequest struct {
	Source      string         `json:"source"`
	ExternalID  string         `json:"externalId"`
	Name        string         `json:"name"`
	Aliases     []string       `json:"aliases"`
	Overview    string         `json:"overview"`
	Role        string         `json:"role"`
	Monitored   bool           `json:"monitored"`
	MonitorMode string         `json:"monitorMode"`
	Metadata    map[string]any `json:"metadata"`
}

// POST /api/v1/metadata/people/import
func (h *metadataHandler) importPerson(w http.ResponseWriter, r *http.Request) {
	var req importPersonRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	if req.Source == "" || req.ExternalID == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "source, externalId, and name are required")
		return
	}

	svcReq := &metadata.ImportPersonRequest{
		Source:      domain.ExternalIDSource(req.Source),
		ExternalID:  req.ExternalID,
		Name:        req.Name,
		Aliases:     req.Aliases,
		Overview:    req.Overview,
		Role:        domain.PersonRole(req.Role),
		Monitored:   req.Monitored,
		MonitorMode: domain.MonitorMode(req.MonitorMode),
		Metadata:    req.Metadata,
	}

	person, err := h.svc.ImportPerson(r.Context(), svcReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "IMPORT_ERROR", "import failed")
		return
	}
	writeJSON(w, http.StatusCreated, toPersonResponse(person))
}

// ── Response shapes ───────────────────────────────────────────────────────────

type externalStudioResponse struct {
	Source             string `json:"source"`
	ExternalID         string `json:"externalId"`
	Name               string `json:"name"`
	Overview           string `json:"overview,omitempty"`
	ImageURL           string `json:"imageUrl,omitempty"`
	WebsiteURL         string `json:"websiteUrl,omitempty"`
	ParentExternalID   string `json:"parentExternalId,omitempty"`
	ParentName         string `json:"parentName,omitempty"`
	ParentImageURL     string `json:"parentImageUrl,omitempty"`
	ParentWebsiteURL   string `json:"parentWebsiteUrl,omitempty"`
}

type externalPersonResponse struct {
	Source     string         `json:"source"`
	ExternalID string         `json:"externalId"`
	Name       string         `json:"name"`
	Aliases    []string       `json:"aliases,omitempty"`
	Overview   string         `json:"overview,omitempty"`
	ImageURL   string         `json:"imageUrl,omitempty"`
	Role       string         `json:"role,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

func toExternalStudioResponses(studios []*domain.ExternalStudio) []externalStudioResponse {
	out := make([]externalStudioResponse, len(studios))
	for i, s := range studios {
		out[i] = externalStudioResponse{
			Source:           string(s.Source),
			ExternalID:       s.ExternalID,
			Name:             s.Name,
			Overview:         s.Overview,
			ImageURL:         s.ImageURL,
			WebsiteURL:       s.WebsiteURL,
			ParentExternalID: s.ParentID,
			ParentName:       s.ParentName,
			ParentImageURL:   s.ParentImageURL,
			ParentWebsiteURL: s.ParentWebsiteURL,
		}
	}
	return out
}

func toExternalPersonResponses(people []*domain.ExternalPerson) []externalPersonResponse {
	out := make([]externalPersonResponse, len(people))
	for i, p := range people {
		out[i] = externalPersonResponse{
			Source:     string(p.Source),
			ExternalID: p.ExternalID,
			Name:       p.Name,
			Aliases:    p.Aliases,
			Overview:   p.Overview,
			ImageURL:   p.ImageURL,
			Role:       string(p.Role),
			Metadata:   p.Metadata,
		}
	}
	return out
}
