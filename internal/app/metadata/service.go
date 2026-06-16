package metadata

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/google/uuid"

	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/media"
	"purser/internal/ports"
)

// Service fans out metadata searches to all registered sources and handles
// importing search results into the library as domain entities.
type Service struct {
	sources     []ports.MetadataSource
	entries     ports.LibraryEntryRepository
	people      ports.PersonRepository
	externalIDs ports.ExternalIDRepository
	mediaPath   string
}

func New(
	sources []ports.MetadataSource,
	entries ports.LibraryEntryRepository,
	people ports.PersonRepository,
	externalIDs ports.ExternalIDRepository,
	mediaPath string,
) *Service {
	return &Service{
		sources:     sources,
		entries:     entries,
		people:      people,
		externalIDs: externalIDs,
		mediaPath:   mediaPath,
	}
}

// ── Search ────────────────────────────────────────────────────────────────────

// SearchStudios queries all sources that serve contentType and merges the results.
// Errors from individual sources are logged but do not abort the fan-out.
func (s *Service) SearchStudios(ctx context.Context, query string, contentType domain.ContentType, limit int) ([]*domain.ExternalStudio, error) {
	type result struct {
		studios []*domain.ExternalStudio
	}
	ch := make(chan result, len(s.sources))

	var wg sync.WaitGroup
	for _, src := range s.sources {
		if !servesContentType(src, contentType) {
			continue
		}
		wg.Add(1)
		go func(src ports.MetadataSource) {
			defer wg.Done()
			studios, err := src.SearchStudios(ctx, query, limit)
			if err != nil {
				slog.Warn("metadata search studios failed", "source", src.Name(), "error", err)
				return
			}
			ch <- result{studios}
		}(src)
	}

	go func() { wg.Wait(); close(ch) }()

	var all []*domain.ExternalStudio
	for r := range ch {
		all = append(all, r.studios...)
	}
	return all, nil
}

// SearchPeople queries all sources that serve contentType and merges the results.
func (s *Service) SearchPeople(ctx context.Context, query string, contentType domain.ContentType, limit int) ([]*domain.ExternalPerson, error) {
	type result struct {
		people []*domain.ExternalPerson
	}
	ch := make(chan result, len(s.sources))

	var wg sync.WaitGroup
	for _, src := range s.sources {
		if contentType != "" && !servesContentType(src, contentType) {
			continue
		}
		wg.Add(1)
		go func(src ports.MetadataSource) {
			defer wg.Done()
			people, err := src.SearchPeople(ctx, query, limit)
			if err != nil {
				slog.Warn("metadata search people failed", "source", src.Name(), "error", err)
				return
			}
			ch <- result{people}
		}(src)
	}

	go func() { wg.Wait(); close(ch) }()

	var all []*domain.ExternalPerson
	for r := range ch {
		all = append(all, r.people...)
	}
	return all, nil
}

// ── Import ────────────────────────────────────────────────────────────────────

// ImportStudioRequest carries the (user-reviewed) studio data to persist.
type ImportStudioRequest struct {
	Source      domain.ExternalIDSource
	ExternalID  string
	Name        string
	Overview    string
	ContentType domain.ContentType
	Monitored   bool
	MonitorMode domain.MonitorMode
	ImageURL         string
	WebsiteURL       string
	ParentExternalID string // parent's ID within the same source
	ParentName       string
	ParentImageURL   string
	ParentWebsiteURL string
}

// ImportStudioResult holds the persisted studio and, if applicable, its network.
type ImportStudioResult struct {
	Studio  *domain.LibraryEntry
	Network *domain.LibraryEntry // nil if no parent was specified or it already existed
}

// ImportStudio persists an ExternalStudio as a library entry. If the studio
// has a parent network, that network is looked up or created first.
// The operation is idempotent: if an entry with the same external ID already
// exists, it is returned without modification.
func (s *Service) ImportStudio(ctx context.Context, req *ImportStudioRequest) (*ImportStudioResult, error) {
	src := string(req.Source)

	// Idempotency: return existing studio if already imported.
	if id, err := s.externalIDs.FindEntity(ctx, "library_entry", src, req.ExternalID); err == nil {
		entry, err := s.entries.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return &ImportStudioResult{Studio: entry}, nil
	}

	res := &ImportStudioResult{}

	// Resolve or create the parent network.
	var parentID string
	if req.ParentExternalID != "" {
		if id, err := s.externalIDs.FindEntity(ctx, "library_entry", src, req.ParentExternalID); err == nil {
			parentID = id
		} else if errors.Is(err, errs.ErrNotFound) {
			networkID := uuid.New().String()
			var networkImagePath string
			if req.ParentImageURL != "" && s.mediaPath != "" {
				networkImagePath = s.fetchImage(ctx, req.ParentImageURL, "entries", networkID)
			}
			networkMeta := map[string]any{}
			if req.ParentWebsiteURL != "" {
				networkMeta["website_url"] = req.ParentWebsiteURL
			}
			network := &domain.LibraryEntry{
				ID:          networkID,
				ContentType: req.ContentType,
				Kind:        domain.KindNetwork,
				Name:        req.ParentName,
				SortName:    req.ParentName,
				Status:      domain.EntryStatusActive,
				Monitored:   false,
				MonitorMode: domain.MonitorNone,
				ImagePath:   networkImagePath,
				Metadata:    networkMeta,
				ExternalIDs: []domain.ExternalID{
					{Source: req.Source, Value: req.ParentExternalID},
				},
			}
			if err := s.entries.Save(ctx, network); err != nil {
				return nil, err
			}
			parentID = network.ID
			res.Network = network
		} else {
			return nil, err
		}
	} else if req.ParentName != "" {
		// Source gave us a name but no external ID — create a name-only network.
		networkID := uuid.New().String()
		var networkImagePath string
		if req.ParentImageURL != "" && s.mediaPath != "" {
			networkImagePath = s.fetchImage(ctx, req.ParentImageURL, "entries", networkID)
		}
		networkMeta := map[string]any{}
		if req.ParentWebsiteURL != "" {
			networkMeta["website_url"] = req.ParentWebsiteURL
		}
		network := &domain.LibraryEntry{
			ID:          networkID,
			ContentType: req.ContentType,
			Kind:        domain.KindNetwork,
			Name:        req.ParentName,
			SortName:    req.ParentName,
			Status:      domain.EntryStatusActive,
			Monitored:   false,
			MonitorMode: domain.MonitorNone,
			ImagePath:   networkImagePath,
			Metadata:    networkMeta,
		}
		if err := s.entries.Save(ctx, network); err != nil {
			return nil, err
		}
		parentID = network.ID
		res.Network = network
	}

	monitorMode := req.MonitorMode
	if monitorMode == "" {
		monitorMode = domain.MonitorLatest
	}

	studioID := uuid.New().String()

	var imagePath string
	if req.ImageURL != "" && s.mediaPath != "" {
		imagePath = s.fetchImage(ctx, req.ImageURL, "entries", studioID)
	}

	meta := map[string]any{}
	if req.WebsiteURL != "" {
		meta["website_url"] = req.WebsiteURL
	}

	studio := &domain.LibraryEntry{
		ID:          studioID,
		ContentType: req.ContentType,
		Kind:        domain.KindStudio,
		Name:        req.Name,
		SortName:    req.Name,
		Overview:    req.Overview,
		ParentID:    parentID,
		Status:      domain.EntryStatusActive,
		Monitored:   req.Monitored,
		MonitorMode: monitorMode,
		ImagePath:   imagePath,
		Metadata:    meta,
		ExternalIDs: []domain.ExternalID{
			{Source: req.Source, Value: req.ExternalID},
		},
	}
	if err := s.entries.Save(ctx, studio); err != nil {
		return nil, err
	}
	res.Studio = studio
	return res, nil
}

// ImportPersonRequest carries the (user-reviewed) person data to persist.
type ImportPersonRequest struct {
	Source      domain.ExternalIDSource
	ExternalID  string
	Name        string
	Aliases     []string
	Overview    string
	Role        domain.PersonRole
	Monitored   bool
	MonitorMode domain.MonitorMode
	ImageURL    string
	Metadata    map[string]any
}

// ImportPerson persists an ExternalPerson as a people record.
// Idempotent: returns the existing record if already imported.
func (s *Service) ImportPerson(ctx context.Context, req *ImportPersonRequest) (*domain.Person, error) {
	src := string(req.Source)

	if id, err := s.externalIDs.FindEntity(ctx, "person", src, req.ExternalID); err == nil {
		return s.people.Get(ctx, id)
	}

	monitorMode := req.MonitorMode
	if monitorMode == "" {
		monitorMode = domain.MonitorNone
	}

	personID := uuid.New().String()

	var imagePath string
	if req.ImageURL != "" && s.mediaPath != "" {
		imagePath = s.fetchImage(ctx, req.ImageURL, "people", personID)
	}

	p := &domain.Person{
		ID:          personID,
		Name:        req.Name,
		SortName:    req.Name,
		Overview:    req.Overview,
		Aliases:     req.Aliases,
		Monitored:   req.Monitored,
		MonitorMode: monitorMode,
		ImagePath:   imagePath,
		Metadata:    req.Metadata,
		ExternalIDs: []domain.ExternalID{
			{Source: req.Source, Value: req.ExternalID},
		},
	}
	if err := s.people.Save(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func servesContentType(src ports.MetadataSource, contentType domain.ContentType) bool {
	return slices.Contains(src.ContentTypes(), contentType)
}

// fetchImage downloads url to the entity's image location and returns the file
// extension (e.g. ".jpg"). Returns "" on error (non-fatal; logged only).
func (s *Service) fetchImage(ctx context.Context, url, entityType, entityID string) string {
	destBase := media.ImagePath(s.mediaPath, entityType, entityID, "")
	if err := os.MkdirAll(filepath.Dir(destBase), 0o755); err != nil {
		slog.Warn("failed to create image dir", "error", err)
		return ""
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Warn("failed to build image request", "url", url, "error", err)
		return ""
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("failed to fetch image", "url", url, "error", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("unexpected status fetching image", "url", url, "status", resp.StatusCode)
		return ""
	}

	ext := extFromContentType(resp.Header.Get("Content-Type"))
	f, err := os.Create(destBase + ext)
	if err != nil {
		slog.Warn("failed to create image file", "path", destBase+ext, "error", err)
		return ""
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		slog.Warn("failed to write image", "path", destBase+ext, "error", err)
		_ = os.Remove(destBase + ext)
		return ""
	}

	return ext
}

func extFromContentType(ct string) string {
	switch {
	case strings.Contains(ct, "jpeg"):
		return ".jpg"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "svg"):
		return ".svg"
	default:
		return ".jpg"
	}
}
