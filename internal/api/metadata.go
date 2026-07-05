package api

import (
	"net/http"
	"purser/internal/app/errs"
	"purser/internal/app/metadata"
	"purser/internal/domain"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type metadataHandler struct {
	svc *metadata.Service
}

func (h *metadataHandler) routes(r chi.Router) {
	r.Get("/search", h.search)
	r.Get("/discography", h.discography)
	r.Post("/entries/import", h.importEntry)
	r.Post("/people/import", h.importPerson)
	r.Post("/albums/import", h.importAlbum)
	r.Post("/items/import", h.importItem)
}

// ── Search ────────────────────────────────────────────────────────────────────

func parseLimit(s string, dflt int) int {
	if l, err := strconv.Atoi(s); err == nil && l > 0 {
		return l
	}
	return dflt
}

// GET /api/v1/metadata/search?kind=studio|person|track&q=...&contentType=...&limit=...
func (h *metadataHandler) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind := q.Get("kind")
	query := q.Get("q")
	contentType := domain.ContentType(q.Get("contentType"))
	limit := parseLimit(q.Get("limit"), 25)

	switch kind {
	case "studio":
		if query == "" {
			writeError(w, http.StatusBadRequest, "MISSING_QUERY", "q is required")
			return
		}
		studios, err := h.svc.SearchStudios(r.Context(), query, contentType, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SEARCH_ERROR", "search failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": toExternalStudioResponses(studios)})

	case "person":
		if query == "" {
			writeError(w, http.StatusBadRequest, "MISSING_QUERY", "q is required")
			return
		}
		role := domain.PersonRole(q.Get("role"))
		people, err := h.svc.SearchPeople(r.Context(), query, role, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SEARCH_ERROR", "search failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": toExternalPersonResponses(people)})

	case "track":
		source := domain.ExternalIDSource(q.Get("source"))
		groupExtID := q.Get("groupExternalId")
		if source == "" || contentType == "" || groupExtID == "" {
			writeError(w, http.StatusBadRequest, "MISSING_PARAMS", "source, contentType, and groupExternalId are required")
			return
		}
		req := &metadata.SearchTracksRequest{
			Source:      source,
			ContentType: contentType,
			GroupExtID:  groupExtID,
			Query:       query,
			Limit:       limit,
		}
		tracks, err := h.svc.SearchTracks(r.Context(), req)
		if err != nil {
			if errs.IsValidation(err) {
				writeError(w, http.StatusBadRequest, "SEARCH_ERROR", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "SEARCH_ERROR", "search failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": toExternalTrackResponses(tracks)})

	default:
		writeError(w, http.StatusBadRequest, "INVALID_KIND", "kind must be studio, person, or track")
	}
}

// ── Import entry ──────────────────────────────────────────────────────────────

type importEntryRequest struct {
	Source           string   `json:"source"`
	ExternalID       string   `json:"externalId"`
	Name             string   `json:"name"`
	Overview         string   `json:"overview"`
	ContentType      string   `json:"contentType"`
	Monitored        bool     `json:"monitored"`
	MonitorMode      string   `json:"monitorMode"`
	AutoImport       *bool    `json:"autoImport"` // nil = omitted → defaults to true
	ParentExternalID string   `json:"parentExternalId"`
	ParentName       string   `json:"parentName"`
	ParentImageURL   string   `json:"parentImageUrl"`
	ParentWebsiteURL string   `json:"parentWebsiteUrl"`
	ImageURL         string   `json:"imageUrl"`
	WebsiteURL       string   `json:"websiteUrl"`
	AlbumFilter      []string `json:"albumFilter"`
}

// POST /api/v1/metadata/entries/import
func (h *metadataHandler) importEntry(w http.ResponseWriter, r *http.Request) {
	var req importEntryRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	if req.Source == "" || req.ExternalID == "" || req.Name == "" || req.ContentType == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "source, externalId, name, and contentType are required")
		return
	}

	ct := domain.ContentType(req.ContentType)
	if ct.ParentEntryKind() == "" {
		writeError(w, http.StatusBadRequest, "INVALID_CONTENT_TYPE", "contentType "+req.ContentType+" has no known entry kind")
		return
	}

	autoImport := req.AutoImport == nil || *req.AutoImport

	svcReq := &metadata.ImportEntryRequest{
		Source:           domain.ExternalIDSource(req.Source),
		ExternalID:       req.ExternalID,
		Name:             req.Name,
		Overview:         req.Overview,
		ContentType:      ct,
		Monitored:        req.Monitored,
		MonitorMode:      domain.MonitorMode(req.MonitorMode),
		AutoImport:       autoImport,
		ParentExternalID: req.ParentExternalID,
		ParentName:       req.ParentName,
		ParentImageURL:   req.ParentImageURL,
		ParentWebsiteURL: req.ParentWebsiteURL,
		ImageURL:         req.ImageURL,
		WebsiteURL:       req.WebsiteURL,
		AlbumFilter:      req.AlbumFilter,
	}

	result, err := h.svc.ImportEntry(r.Context(), svcReq)
	if err != nil {
		handleErr(w, err)
		return
	}

	resp := map[string]any{
		"entry": toEntryResponse(result.Entry),
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

// ── Import album ──────────────────────────────────────────────────────────────

type importAlbumRequest struct {
	Source         string   `json:"source"`
	ExternalID     string   `json:"externalId"`
	LibraryEntryID string   `json:"libraryEntryId"`
	Title          string   `json:"title"`
	Year           int      `json:"year"`
	Monitored      bool     `json:"monitored"`
	MonitorMode    string   `json:"monitorMode"`
	PrimaryType    string   `json:"primaryType,omitempty"`
	SecondaryTypes []string `json:"secondaryTypes,omitempty"`
}

// POST /api/v1/metadata/albums/import
func (h *metadataHandler) importAlbum(w http.ResponseWriter, r *http.Request) {
	var req importAlbumRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	if req.Source == "" || req.ExternalID == "" || req.LibraryEntryID == "" || req.Title == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "source, externalId, libraryEntryId, and title are required")
		return
	}

	svcReq := &metadata.ImportAlbumRequest{
		Source:         domain.ExternalIDSource(req.Source),
		ExternalID:     req.ExternalID,
		LibraryEntryID: req.LibraryEntryID,
		Title:          req.Title,
		Year:           req.Year,
		Monitored:      req.Monitored,
		MonitorMode:    domain.MonitorMode(req.MonitorMode),
		PrimaryType:    req.PrimaryType,
		SecondaryTypes: req.SecondaryTypes,
	}

	group, err := h.svc.ImportAlbum(r.Context(), svcReq)
	if err != nil {
		if errs.IsValidation(err) {
			writeError(w, http.StatusBadRequest, "UNKNOWN_SOURCE", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "IMPORT_ERROR", "import failed")
		return
	}
	writeJSON(w, http.StatusCreated, toGroupResponse(group))
}

// ── Import item ───────────────────────────────────────────────────────────────

type importItemRequest struct {
	Source          string `json:"source"`
	ExternalID      string `json:"externalId"`
	ContentType     string `json:"contentType"`
	AlbumExternalID string `json:"albumExternalId,omitempty"`
	AlbumTitle      string `json:"albumTitle,omitempty"`
	Monitored       bool   `json:"monitored"`
}

// POST /api/v1/metadata/items/import
func (h *metadataHandler) importItem(w http.ResponseWriter, r *http.Request) {
	var req importItemRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	if req.Source == "" || req.ExternalID == "" || req.ContentType == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "source, externalId, and contentType are required")
		return
	}

	svcReq := &metadata.ImportItemRequest{
		Source:          domain.ExternalIDSource(req.Source),
		ExternalID:      req.ExternalID,
		ContentType:     domain.ContentType(req.ContentType),
		AlbumExternalID: req.AlbumExternalID,
		AlbumTitle:      req.AlbumTitle,
		Monitored:       req.Monitored,
	}

	result, err := h.svc.ImportItem(r.Context(), svcReq)
	if err != nil {
		if errs.IsValidation(err) {
			writeError(w, http.StatusBadRequest, "IMPORT_ERROR", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "IMPORT_ERROR", "import failed")
		return
	}

	resp := map[string]any{
		"item": toItemResponse(result.Item),
	}
	if result.Entry != nil {
		resp["entry"] = toEntryResponse(result.Entry)
	}
	if result.Network != nil {
		resp["network"] = toEntryResponse(result.Network)
	}
	if result.Album != nil {
		resp["album"] = toGroupResponse(result.Album)
	}
	writeJSON(w, http.StatusCreated, resp)
}

// ── Discography ───────────────────────────────────────────────────────────────

// GET /api/v1/metadata/discography?source=mbz&contentType=music&externalId={mbid}&page=1&pageSize=50
func (h *metadataHandler) discography(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	source := domain.ExternalIDSource(q.Get("source"))
	externalID := q.Get("externalId")
	contentType := domain.ContentType(q.Get("contentType"))
	if source == "" || externalID == "" || contentType == "" {
		writeError(w, http.StatusBadRequest, "MISSING_PARAMS", "source, contentType, and externalId are required")
		return
	}
	if !contentType.Valid() {
		writeError(w, http.StatusBadRequest, "INVALID_CONTENT_TYPE", "invalid contentType")
		return
	}
	page := 1
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 0 {
		page = p
	}
	pageSize := 50
	if ps, err := strconv.Atoi(q.Get("pageSize")); err == nil && ps > 0 {
		pageSize = ps
	}
	groups, total, err := h.svc.FetchArtistDiscography(r.Context(), source, contentType, externalID, page, pageSize)
	if err != nil {
		if errs.IsValidation(err) {
			writeError(w, http.StatusBadRequest, "UNKNOWN_SOURCE", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "FETCH_ERROR", "failed to fetch discography")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"results":  toExternalGroupResponses(groups),
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// ── Response shapes ───────────────────────────────────────────────────────────

type externalGroupResponse struct {
	Source         string   `json:"source"`
	ExternalID     string   `json:"externalId"`
	Title          string   `json:"title"`
	Year           int      `json:"year,omitempty"`
	PrimaryType    string   `json:"primaryType,omitempty"`
	SecondaryTypes []string `json:"secondaryTypes,omitempty"`
}

type externalStudioResponse struct {
	Source           string `json:"source"`
	ExternalID       string `json:"externalId"`
	Name             string `json:"name"`
	Overview         string `json:"overview,omitempty"`
	ImageURL         string `json:"imageUrl,omitempty"`
	WebsiteURL       string `json:"websiteUrl,omitempty"`
	ParentExternalID string `json:"parentExternalId,omitempty"`
	ParentName       string `json:"parentName,omitempty"`
	ParentImageURL   string `json:"parentImageUrl,omitempty"`
	ParentWebsiteURL string `json:"parentWebsiteUrl,omitempty"`
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

type externalTrackResponse struct {
	Source         string `json:"source"`
	ExternalID     string `json:"externalId"`
	Title          string `json:"title"`
	Sequence       string `json:"sequence,omitempty"`
	RuntimeSeconds int    `json:"runtimeSeconds,omitempty"`
}

func toExternalTrackResponses(tracks []*domain.ExternalItem) []externalTrackResponse {
	out := make([]externalTrackResponse, len(tracks))
	for i, t := range tracks {
		out[i] = externalTrackResponse{
			Source:         string(t.Source),
			ExternalID:     t.ExternalID,
			Title:          t.Title,
			Sequence:       t.Sequence,
			RuntimeSeconds: t.RuntimeSecs,
		}
	}
	return out
}

func toExternalGroupResponses(groups []*domain.ExternalGroup) []externalGroupResponse {
	out := make([]externalGroupResponse, len(groups))
	for i, g := range groups {
		out[i] = externalGroupResponse{
			Source:         string(g.Source),
			ExternalID:     g.ExternalID,
			Title:          g.Title,
			Year:           g.Year,
			PrimaryType:    g.PrimaryType,
			SecondaryTypes: g.SecondaryTypes,
		}
	}
	return out
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
