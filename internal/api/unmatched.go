package api

import (
	"net/http"
	"purser/internal/app/scan"
	"purser/internal/domain"
	"purser/internal/ports"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type unmatchedHandler struct {
	scanSvc *scan.Service
}

func (h *unmatchedHandler) routes(r chi.Router) {
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Post("/{id}/match", h.manualMatch)
	r.Post("/{id}/scrape", h.rescrape)
	r.Post("/{id}/dismiss", h.dismiss)
}

// ── Response types ────────────────────────────────────────────────────────────

type fingerprintResponse struct {
	OSHash       string            `json:"oshash,omitempty"`
	PHash        string            `json:"phash,omitempty"`
	AcoustID     string            `json:"acoust_id,omitempty"`
	EmbeddedTags map[string]string `json:"embedded_tags,omitempty"`
	ISBN         string            `json:"isbn,omitempty"`
}

type externalParentResponse struct {
	Source         string `json:"source"`
	ExternalID     string `json:"external_id"`
	Name           string `json:"name"`
	ParentID       string `json:"parent_id,omitempty"`
	ParentName     string `json:"parent_name,omitempty"`
	ImageURL       string `json:"image_url,omitempty"`
	ParentImageURL string `json:"parent_image_url,omitempty"`
}

type externalCandidateResponse struct {
	Source          string                  `json:"source"`
	ExternalID      string                  `json:"external_id"`
	Title           string                  `json:"title"`
	ContentType     string                  `json:"content_type"`
	ParentKind      string                  `json:"parent_kind,omitempty"`
	ImageURL        string                  `json:"image_url,omitempty"`
	Overview        string                  `json:"overview,omitempty"`
	Date            string                  `json:"date,omitempty"`
	RuntimeSeconds  int                     `json:"runtime_seconds,omitempty"`
	GroupExternalID string                  `json:"group_external_id,omitempty"`
	Parent          *externalParentResponse `json:"parent,omitempty"`
}

type matchCandidateResponse struct {
	ItemID            string                     `json:"item_id"`
	ItemTitle         string                     `json:"item_title,omitempty"`
	Confidence        float64                    `json:"confidence"`
	Source            string                     `json:"source"`
	SourceLabel       string                     `json:"source_label"`
	SourceDescription string                     `json:"source_description,omitempty"`
	External          *externalCandidateResponse `json:"external,omitempty"`
}

type unmatchedFileResponse struct {
	ID           string                   `json:"id"`
	Path         string                   `json:"path"`
	Size         int64                    `json:"size"`
	ContentType  string                   `json:"content_type"`
	DiscoveredAt time.Time                `json:"discovered_at"`
	Status       string                   `json:"status"`
	Fingerprint  *fingerprintResponse     `json:"fingerprint"`
	Candidates   []matchCandidateResponse `json:"candidates"`
}

type unmatchedGroupResponse struct {
	GroupID                 string                  `json:"group_id"`
	GroupTitle              string                  `json:"group_title"`
	Files                   []unmatchedFileResponse `json:"files"`
	BestCandidateConfidence float64                 `json:"best_candidate_confidence"`
}

type unmatchedListResponse struct {
	Items     any    `json:"items"`
	Total     int    `json:"total"`
	GroupedBy string `json:"grouped_by,omitempty"`
}

type rescrapeResponse struct {
	Candidates []matchCandidateResponse `json:"candidates"`
}

// ── Conversion helpers ────────────────────────────────────────────────────────

func fingerprintToResponse(fp *domain.Fingerprint) *fingerprintResponse {
	if fp == nil {
		return &fingerprintResponse{}
	}
	return &fingerprintResponse{
		OSHash:       fp.OSHash,
		PHash:        fp.PHash,
		AcoustID:     fp.AcoustID,
		EmbeddedTags: fp.EmbeddedTags,
		ISBN:         fp.ISBN,
	}
}

func candidatesToResponse(cs []domain.MatchCandidate, fp *domain.Fingerprint) []matchCandidateResponse {
	out := make([]matchCandidateResponse, 0, len(cs))
	for _, c := range cs {
		itemID, itemTitle := "", ""
		if c.Item != nil {
			itemID = c.Item.ID
			itemTitle = c.Item.Title
		}
		ms := domain.MatchSource(c.Source)
		resp := matchCandidateResponse{
			ItemID:            itemID,
			ItemTitle:         itemTitle,
			Confidence:        c.Confidence,
			Source:            c.Source,
			SourceLabel:       ms.Label(),
			SourceDescription: ms.Description(),
		}
		if c.ExternalItem != nil {
			var date string
			if !c.ExternalItem.Date.IsZero() {
				date = c.ExternalItem.Date.Format("2006-01-02")
			}
			ext := &externalCandidateResponse{
				Source:          string(c.ExternalItem.Source),
				ExternalID:      c.ExternalItem.ExternalID,
				Title:           c.ExternalItem.Title,
				ContentType:     string(c.ExternalItem.ContentType),
				ParentKind:      c.ExternalItem.ContentType.ParentEntryKind(),
				ImageURL:        c.ExternalItem.ImageURL,
				Overview:        c.ExternalItem.Overview,
				Date:            date,
				RuntimeSeconds:  c.ExternalItem.RuntimeSecs,
				GroupExternalID: c.ExternalItem.GroupExternalID,
			}
			// For music stubs stored before the MBZ recording lookup was available,
			// enrich title/runtime/artist from the file's embedded tags at response time.
			if fp != nil && c.ExternalItem.ContentType == domain.ContentTypeMusic {
				if ext.Title == "" {
					ext.Title = fp.EmbeddedTags["title"]
				}
				if ext.RuntimeSeconds == 0 {
					if ms, err := strconv.Atoi(fp.EmbeddedTags["duration_ms"]); err == nil {
						ext.RuntimeSeconds = ms / 1000
					}
				}
			}
			if c.ExternalItem.Studio != nil {
				ext.Parent = &externalParentResponse{
					Source:         string(c.ExternalItem.Studio.Source),
					ExternalID:     c.ExternalItem.Studio.ExternalID,
					Name:           c.ExternalItem.Studio.Name,
					ParentID:       c.ExternalItem.Studio.ParentID,
					ParentName:     c.ExternalItem.Studio.ParentName,
					ImageURL:       c.ExternalItem.Studio.ImageURL,
					ParentImageURL: c.ExternalItem.Studio.ParentImageURL,
				}
			} else if fp != nil && c.ExternalItem.ContentType == domain.ContentTypeMusic {
				// Synthesise a parent from embedded tags if the stub has no artist.
				artist := fp.EmbeddedTags["artist"]
				artistID := fp.EmbeddedTags["musicbrainz_artist_id"]
				if artist != "" && artistID != "" {
					ext.Parent = &externalParentResponse{
						Source:     string(domain.SourceMusicBrainz),
						ExternalID: artistID,
						Name:       artist,
					}
				}
			}
			resp.External = ext
		}
		out = append(out, resp)
	}
	return out
}

func unmatchedToResponse(uf *domain.UnmatchedFile) unmatchedFileResponse {
	return unmatchedFileResponse{
		ID:           uf.ID,
		Path:         uf.Path,
		Size:         uf.Size,
		ContentType:  string(uf.ContentType),
		DiscoveredAt: uf.DiscoveredAt,
		Status:       string(uf.Status),
		Fingerprint:  fingerprintToResponse(uf.Fingerprint),
		Candidates:   candidatesToResponse(uf.Candidates, uf.Fingerprint),
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *unmatchedHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	contentType := domain.ContentType(q.Get("contentType"))
	status := domain.UnmatchedStatus(q.Get("status"))
	groupBy := q.Get("groupBy")

	if status == "" {
		status = domain.UnmatchedPending
	}

	filter := ports.UnmatchedFilter{
		ContentType: contentType,
		Status:      status,
	}

	if groupBy == "album" {
		groups, err := h.scanSvc.ListUnmatchedGrouped(r.Context(), filter)
		if handleErr(w, err) {
			return
		}
		items := make([]unmatchedGroupResponse, 0, len(groups))
		for _, g := range groups {
			files := make([]unmatchedFileResponse, 0, len(g.Files))
			for _, f := range g.Files {
				files = append(files, unmatchedToResponse(f))
			}
			items = append(items, unmatchedGroupResponse{
				GroupID:                 g.GroupID,
				GroupTitle:              g.GroupTitle,
				Files:                   files,
				BestCandidateConfidence: g.BestCandidateConfidence,
			})
		}
		writeJSON(w, http.StatusOK, unmatchedListResponse{
			Items:     items,
			Total:     len(items),
			GroupedBy: "album",
		})
		return
	}

	files, err := h.scanSvc.ListUnmatched(r.Context(), filter)
	if handleErr(w, err) {
		return
	}
	items := make([]unmatchedFileResponse, 0, len(files))
	for _, f := range files {
		items = append(items, unmatchedToResponse(f))
	}
	writeJSON(w, http.StatusOK, unmatchedListResponse{Items: items, Total: len(items)})
}

func (h *unmatchedHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	uf, err := h.scanSvc.GetUnmatched(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, unmatchedToResponse(uf))
}

type manualMatchRequest struct {
	ItemID string `json:"item_id"`
}

func (h *unmatchedHandler) manualMatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req manualMatchRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	if req.ItemID == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "item_id is required")
		return
	}
	if err := h.scanSvc.ManualMatch(r.Context(), id, req.ItemID); handleErr(w, err) {
		return
	}
	uf, err := h.scanSvc.GetUnmatched(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, unmatchedToResponse(uf))
}

type rescrapeRequest struct {
	Query string `json:"query"`
}

func (h *unmatchedHandler) rescrape(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req rescrapeRequest
	_ = decode(r, &req) // body is optional
	candidates, err := h.scanSvc.Rescrape(r.Context(), id, req.Query)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, rescrapeResponse{Candidates: candidatesToResponse(candidates, nil)})
}

func (h *unmatchedHandler) dismiss(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.scanSvc.Dismiss(r.Context(), id); handleErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusOK)
}
