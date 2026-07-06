package api

import (
	"net/http"
	"purser/internal/domain"
	"purser/internal/ports"
	"time"

	"github.com/go-chi/chi/v5"
)

type musicQueueHandler struct {
	queue ports.MusicScanGroupRepository
}

func (h *musicQueueHandler) routes(r chi.Router) {
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Delete("/{id}", h.dismiss)
}

func toTagSummaryResponse(s domain.MusicTagSummary) musicTagSummaryResponse {
	durations := make([]int64, len(s.TrackDurations))
	for i, d := range s.TrackDurations {
		durations[i] = d.Milliseconds()
	}
	titles := s.TrackTitles
	if titles == nil {
		titles = []string{}
	}
	isrcs := s.ISRCs
	if isrcs == nil {
		isrcs = []string{}
	}
	return musicTagSummaryResponse{
		AlbumArtist:      s.AlbumArtist,
		AlbumTitle:       s.AlbumTitle,
		Year:             s.Year,
		Barcode:          s.Barcode,
		Label:            s.Label,
		CatalogNumber:    s.CatalogNumber,
		TotalTracks:      s.TotalTracks,
		TotalDiscs:       s.TotalDiscs,
		MBZReleaseID:     s.MBZReleaseID,
		TrackTitles:      titles,
		ISRCs:            isrcs,
		TrackDurationsMS: durations,
	}
}

type tagsRequestFields struct {
	AlbumArtist      string   `json:"albumArtist"`
	AlbumTitle       string   `json:"albumTitle"`
	Year             int      `json:"year"`
	Barcode          string   `json:"barcode"`
	Label            string   `json:"label"`
	CatalogNumber    string   `json:"catalogNumber"`
	TotalTracks      int      `json:"totalTracks"`
	TotalDiscs       int      `json:"totalDiscs"`
	MBZReleaseID     string   `json:"mbzReleaseId"`
	TrackTitles      []string `json:"trackTitles"`
	ISRCs            []string `json:"isrcs"`
	TrackDurationsMS []int64  `json:"trackDurationsMs"`
}

func reqTagsToSummary(req tagsRequestFields) domain.MusicTagSummary {
	durations := make([]time.Duration, len(req.TrackDurationsMS))
	for i, ms := range req.TrackDurationsMS {
		durations[i] = time.Duration(ms) * time.Millisecond
	}
	return domain.MusicTagSummary{
		AlbumArtist:    req.AlbumArtist,
		AlbumTitle:     req.AlbumTitle,
		Year:           req.Year,
		Barcode:        req.Barcode,
		Label:          req.Label,
		CatalogNumber:  req.CatalogNumber,
		TotalTracks:    req.TotalTracks,
		TotalDiscs:     req.TotalDiscs,
		MBZReleaseID:   req.MBZReleaseID,
		TrackTitles:    req.TrackTitles,
		TrackDurations: durations,
		ISRCs:          req.ISRCs,
	}
}

func toSignalsResponse(s domain.MusicConfidenceSignals) musicConfidenceSignalsResponse {
	return musicConfidenceSignalsResponse{
		Barcode:       s.Barcode,
		ISRC:          s.ISRC,
		RGNameFuzzy:   s.RGNameFuzzy,
		TrackCount:    s.TrackCount,
		TrackTitleSet: s.TrackTitleSet,
		Duration:      s.Duration,
		AcoustID:      s.AcoustID,
	}
}

func toCandidateResponse(c domain.MusicReleaseCandidate) musicReleaseCandidateResponse {
	return musicReleaseCandidateResponse{
		ArtistMBID:         c.ArtistMBID,
		ArtistName:         c.ArtistName,
		ReleaseGroupMBID:   c.ReleaseGroupMBID,
		ReleaseGroupTitle:  c.ReleaseGroupTitle,
		ReleaseGroupType:   c.ReleaseGroupType,
		ReleaseMBID:        c.ReleaseMBID,
		ReleaseTitle:       c.ReleaseTitle,
		ReleaseDate:        c.ReleaseDate,
		ReleaseLabel:       c.ReleaseLabel,
		ReleaseCountry:     c.ReleaseCountry,
		ReleaseBarcode:     c.ReleaseBarcode,
		ReleaseFormat:      c.ReleaseFormat,
		ReleaseMediumCount: c.ReleaseMediumCount,
		ReleaseTrackCount:  c.ReleaseTrackCount,
		OverallConfidence:  c.OverallConfidence,
		Signals:            toSignalsResponse(c.Signals),
	}
}

func toMusicScanGroupResponse(g *domain.MusicScanGroup) *musicScanGroupResponse {
	candidates := make([]musicReleaseCandidateResponse, 0, len(g.Candidates))
	for _, c := range g.Candidates {
		candidates = append(candidates, toCandidateResponse(c))
	}
	return &musicScanGroupResponse{
		ID:           g.ID,
		FolderPath:   g.FolderPath,
		TotalTracks:  g.TotalTracks,
		TotalDiscs:   g.TotalDiscs,
		Status:       string(g.Status),
		Tags:         toTagSummaryResponse(g.Tags),
		Candidates:   candidates,
		DiscoveredAt: g.DiscoveredAt,
	}
}

type createMusicQueueRequest struct {
	FolderPath  string            `json:"folderPath"`
	TotalTracks int               `json:"totalTracks"`
	TotalDiscs  int               `json:"totalDiscs"`
	Status      string            `json:"status"`
	Tags        tagsRequestFields `json:"tags"`
	Candidates  []struct {
		ArtistMBID         string  `json:"artistMbid"`
		ArtistName         string  `json:"artistName"`
		ReleaseGroupMBID   string  `json:"releaseGroupMbid"`
		ReleaseGroupTitle  string  `json:"releaseGroupTitle"`
		ReleaseGroupType   string  `json:"releaseGroupType"`
		ReleaseMBID        string  `json:"releaseMbid"`
		ReleaseTitle       string  `json:"releaseTitle"`
		ReleaseDate        string  `json:"releaseDate"`
		ReleaseLabel       string  `json:"releaseLabel"`
		ReleaseCountry     string  `json:"releaseCountry"`
		ReleaseBarcode     string  `json:"releaseBarcode"`
		ReleaseFormat      string  `json:"releaseFormat"`
		ReleaseMediumCount int     `json:"releaseMediumCount"`
		ReleaseTrackCount  int     `json:"releaseTrackCount"`
		OverallConfidence  float64 `json:"overallConfidence"`
		Signals            struct {
			Barcode       float64 `json:"barcode"`
			ISRC          float64 `json:"isrc"`
			RGNameFuzzy   float64 `json:"rgNameFuzzy"`
			TrackCount    float64 `json:"trackCount"`
			TrackTitleSet float64 `json:"trackTitleSet"`
			Duration      float64 `json:"duration"`
			AcoustID      float64 `json:"acoustid"`
		} `json:"signals"`
	} `json:"candidates"`
}

func (h *musicQueueHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createMusicQueueRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	g := &domain.MusicScanGroup{
		FolderPath:   req.FolderPath,
		TotalTracks:  req.TotalTracks,
		TotalDiscs:   req.TotalDiscs,
		Status:       domain.UnmatchedStatus(req.Status),
		DiscoveredAt: time.Now().UTC(),
		Tags:         reqTagsToSummary(req.Tags),
	}
	if g.Status == "" {
		g.Status = domain.UnmatchedPending
	}
	for _, c := range req.Candidates {
		g.Candidates = append(g.Candidates, domain.MusicReleaseCandidate{
			ArtistMBID:         c.ArtistMBID,
			ArtistName:         c.ArtistName,
			ReleaseGroupMBID:   c.ReleaseGroupMBID,
			ReleaseGroupTitle:  c.ReleaseGroupTitle,
			ReleaseGroupType:   c.ReleaseGroupType,
			ReleaseMBID:        c.ReleaseMBID,
			ReleaseTitle:       c.ReleaseTitle,
			ReleaseDate:        c.ReleaseDate,
			ReleaseLabel:       c.ReleaseLabel,
			ReleaseCountry:     c.ReleaseCountry,
			ReleaseBarcode:     c.ReleaseBarcode,
			ReleaseFormat:      c.ReleaseFormat,
			ReleaseMediumCount: c.ReleaseMediumCount,
			ReleaseTrackCount:  c.ReleaseTrackCount,
			OverallConfidence:  c.OverallConfidence,
			Signals: domain.MusicConfidenceSignals{
				Barcode:       c.Signals.Barcode,
				ISRC:          c.Signals.ISRC,
				RGNameFuzzy:   c.Signals.RGNameFuzzy,
				TrackCount:    c.Signals.TrackCount,
				TrackTitleSet: c.Signals.TrackTitleSet,
				Duration:      c.Signals.Duration,
				AcoustID:      c.Signals.AcoustID,
			},
		})
	}
	if err := h.queue.Save(r.Context(), g); handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, toMusicScanGroupResponse(g))
}

func (h *musicQueueHandler) list(w http.ResponseWriter, r *http.Request) {
	status := domain.UnmatchedStatus(r.URL.Query().Get("status"))
	if status == "" {
		status = domain.UnmatchedPending
	}
	groups, err := h.queue.List(r.Context(), status)
	if handleErr(w, err) {
		return
	}
	resp := make([]*musicScanGroupResponse, 0, len(groups))
	for _, g := range groups {
		resp = append(resp, toMusicScanGroupResponse(g))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *musicQueueHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	g, err := h.queue.Get(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, toMusicScanGroupResponse(g))
}

func (h *musicQueueHandler) dismiss(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	g, err := h.queue.Get(r.Context(), id)
	if handleErr(w, err) {
		return
	}
	g.Status = domain.UnmatchedDismissed
	if err := h.queue.Save(r.Context(), g); handleErr(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
