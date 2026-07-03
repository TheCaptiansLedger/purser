package scan

import (
	"context"
	"fmt"
	"log/slog"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Service orchestrates fingerprinting, identification, and the auto-import decision.
// It contains zero content-type switches and zero adapter name strings.
type Service struct {
	scanner        ports.FileScanner
	watcher        ports.FileWatcher
	fingerprinters []ports.FileFingerprinter
	identifiers    []ports.FileIdentifier
	items          ports.ItemRepository
	mediaFiles     ports.MediaFileRepository
	unmatched      ports.UnmatchedFileRepository
	notifier       ports.NotificationDispatcher
	threshold      float64
}

// New constructs a scan Service wired to the given ports.
func New(
	scanner ports.FileScanner,
	watcher ports.FileWatcher,
	fingerprinters []ports.FileFingerprinter,
	identifiers []ports.FileIdentifier,
	items ports.ItemRepository,
	mediaFiles ports.MediaFileRepository,
	unmatched ports.UnmatchedFileRepository,
	notifier ports.NotificationDispatcher,
	threshold float64,
) *Service {
	return &Service{
		scanner:        scanner,
		watcher:        watcher,
		fingerprinters: fingerprinters,
		identifiers:    identifiers,
		items:          items,
		mediaFiles:     mediaFiles,
		unmatched:      unmatched,
		notifier:       notifier,
		threshold:      threshold,
	}
}

// ScanRoots walks roots through the scanner and runs the full pipeline on each discovered file.
func (s *Service) ScanRoots(ctx context.Context, roots []string, filter ports.ScanFilter) error {
	_ = s.notifier.Dispatch(ctx, domain.NotificationEvent{Type: domain.NotifyScanStarted})

	ch, err := s.scanner.Scan(ctx, roots, filter)
	if err != nil {
		return fmt.Errorf("start scan: %w", err)
	}

	var discovered int
	for f := range ch {
		discovered++
		if err := s.processFile(ctx, f); err != nil {
			slog.WarnContext(ctx, "process file failed", "path", f.Path, "err", err)
		}
	}

	_ = s.notifier.Dispatch(ctx, domain.NotificationEvent{
		Type:    domain.NotifyScanComplete,
		Payload: map[string]int{"discovered": discovered},
	})
	return nil
}

// StartWatching watches roots for filesystem events and runs the pipeline on each one.
// Blocks until ctx is cancelled or the watcher channel closes.
func (s *Service) StartWatching(ctx context.Context, roots []string) error {
	ch, err := s.watcher.Watch(ctx, roots)
	if err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			s.handleWatchEvent(ctx, event)
		}
	}
}

func (s *Service) handleWatchEvent(ctx context.Context, event ports.WatchEvent) {
	switch event.Op {
	case ports.WatchCreated, ports.WatchModified:
		f := domain.ScannedFile{
			Path:         event.Path,
			ContentType:  event.ContentType,
			DiscoveredAt: time.Now().UTC(),
		}
		if err := s.processFile(ctx, f); err != nil {
			slog.WarnContext(ctx, "process watch event failed", "path", event.Path, "err", err)
		}
	case ports.WatchRemoved:
		s.handleRemoved(ctx, event.Path)
	}
}

// processFile is the single pipeline entry point for both ScanRoots and StartWatching.
func (s *Service) processFile(ctx context.Context, f domain.ScannedFile) error {
	// 1. Fingerprint: fan out to all fingerprinters that handle this content type.
	fp := &domain.Fingerprint{}
	for _, fpr := range s.fingerprinters {
		if !slices.Contains(fpr.ContentTypes(), f.ContentType) {
			continue
		}
		result, err := fpr.Fingerprint(ctx, f)
		if err != nil {
			slog.WarnContext(ctx, "fingerprinter failed", "path", f.Path, "err", err)
			continue
		}
		if result != nil {
			mergeFingerprint(fp, result)
		}
	}
	f.Fingerprint = fp

	// 2. Identify: fan out to all identifiers that handle this content type.
	var allCandidates []domain.MatchCandidate
	for _, id := range s.identifiers {
		if !slices.Contains(id.ContentTypes(), f.ContentType) {
			continue
		}
		candidates, err := id.Identify(ctx, f)
		if err != nil {
			slog.WarnContext(ctx, "identifier failed", "path", f.Path, "err", err)
			continue
		}
		allCandidates = append(allCandidates, candidates...)
	}
	allCandidates = deduplicateCandidates(allCandidates)

	// 3. Dispatch file discovered.
	_ = s.notifier.Dispatch(ctx, domain.NotificationEvent{
		Type:    domain.NotifyFileDiscovered,
		Payload: f.Path,
	})

	// 4. Duplicate check: skip if OSHash is already linked to a live item.
	if fp.OSHash != "" {
		existing, err := s.mediaFiles.GetByOSHash(ctx, fp.OSHash)
		if err == nil && existing.ItemID != "" {
			slog.DebugContext(ctx, "file already imported", "path", f.Path, "oshash", fp.OSHash)
			return nil
		}
		if err != nil && !errs.IsNotFound(err) {
			return fmt.Errorf("oshash lookup: %w", err)
		}
	}

	// 5. Decision: auto-import if best candidate meets the threshold.
	if len(allCandidates) > 0 && allCandidates[0].Confidence >= s.threshold {
		return s.autoImport(ctx, f, allCandidates[0])
	}
	return s.enqueueUnmatched(ctx, f, allCandidates)
}

func (s *Service) autoImport(ctx context.Context, f domain.ScannedFile, candidate domain.MatchCandidate) error {
	mf := &domain.MediaFile{
		ID:              uuid.New().String(),
		ItemID:          candidate.Item.ID,
		Path:            f.Path,
		Size:            f.Size,
		OSHash:          f.Fingerprint.OSHash,
		MatchConfidence: confidenceFromSource(candidate.Source),
		AddedAt:         time.Now().UTC(),
	}
	if f.Fingerprint.EmbeddedTags != nil {
		mf.Codec = f.Fingerprint.EmbeddedTags["codec"]
		mf.Container = f.Fingerprint.EmbeddedTags["container"]
		mf.Resolution = f.Fingerprint.EmbeddedTags["resolution"]
	}
	if err := s.mediaFiles.Save(ctx, mf); err != nil {
		return fmt.Errorf("save media file: %w", err)
	}

	candidate.Item.Status = domain.StatusImported
	candidate.Item.MediaFile = mf
	if err := s.items.Save(ctx, candidate.Item); err != nil {
		return fmt.Errorf("save item: %w", err)
	}

	_ = s.notifier.Dispatch(ctx, domain.NotificationEvent{
		Type: domain.NotifyAutoMatched,
		Payload: map[string]any{
			"path":       f.Path,
			"item_id":    candidate.Item.ID,
			"confidence": candidate.Confidence,
		},
	})
	return nil
}

func (s *Service) enqueueUnmatched(ctx context.Context, f domain.ScannedFile, candidates []domain.MatchCandidate) error {
	uf := &domain.UnmatchedFile{
		ID:           uuid.New().String(),
		Path:         f.Path,
		Size:         f.Size,
		ContentType:  f.ContentType,
		Fingerprint:  f.Fingerprint,
		Candidates:   candidates,
		DiscoveredAt: time.Now().UTC(),
		Status:       domain.UnmatchedPending,
	}
	if err := s.unmatched.Save(ctx, uf); err != nil {
		return fmt.Errorf("save unmatched file: %w", err)
	}

	payload := map[string]any{
		"path":         f.Path,
		"content_type": string(f.ContentType),
	}
	if len(candidates) > 0 {
		payload["top_candidate"] = candidates[0].Item.Title
	}
	_ = s.notifier.Dispatch(ctx, domain.NotificationEvent{
		Type:    domain.NotifyUnmatched,
		Payload: payload,
	})
	return nil
}

func (s *Service) handleRemoved(ctx context.Context, path string) {
	existing, err := s.mediaFiles.GetByPath(ctx, path)
	if errs.IsNotFound(err) {
		return
	}
	if err != nil {
		slog.WarnContext(ctx, "media file lookup failed on removal", "path", path, "err", err)
		return
	}
	slog.InfoContext(ctx, "media file removed from disk", "path", path, "item_id", existing.ItemID)
}

// ManualMatch resolves an unmatched file queue entry by linking it to a specific item.
func (s *Service) ManualMatch(ctx context.Context, unmatchedID, itemID string) error {
	uf, err := s.unmatched.Get(ctx, unmatchedID)
	if err != nil {
		return fmt.Errorf("get unmatched file: %w", err)
	}
	item, err := s.items.Get(ctx, itemID)
	if err != nil {
		return fmt.Errorf("get item: %w", err)
	}

	var osHash string
	if uf.Fingerprint != nil {
		osHash = uf.Fingerprint.OSHash
	}

	mf := &domain.MediaFile{
		ID:              uuid.New().String(),
		ItemID:          item.ID,
		Path:            uf.Path,
		Size:            uf.Size,
		OSHash:          osHash,
		MatchConfidence: domain.MatchManual,
		AddedAt:         time.Now().UTC(),
	}
	if err := s.mediaFiles.Save(ctx, mf); err != nil {
		return fmt.Errorf("save media file: %w", err)
	}

	item.Status = domain.StatusImported
	item.MediaFile = mf
	if err := s.items.Save(ctx, item); err != nil {
		return fmt.Errorf("save item: %w", err)
	}

	uf.Status = domain.UnmatchedMatched
	if err := s.unmatched.Save(ctx, uf); err != nil {
		return fmt.Errorf("update unmatched file status: %w", err)
	}

	_ = s.notifier.Dispatch(ctx, domain.NotificationEvent{
		Type: domain.NotifyAutoMatched,
		Payload: map[string]any{
			"path":    uf.Path,
			"item_id": item.ID,
			"manual":  true,
		},
	})
	return nil
}

// mergeFingerprint accumulates non-zero fields from src into dst.
func mergeFingerprint(dst, src *domain.Fingerprint) {
	if src.OSHash != "" {
		dst.OSHash = src.OSHash
	}
	if src.PHash != "" {
		dst.PHash = src.PHash
	}
	if src.AcoustID != "" {
		dst.AcoustID = src.AcoustID
	}
	if src.ISBN != "" {
		dst.ISBN = src.ISBN
	}
	if len(src.EmbeddedTags) > 0 {
		if dst.EmbeddedTags == nil {
			dst.EmbeddedTags = make(map[string]string)
		}
		for k, v := range src.EmbeddedTags {
			dst.EmbeddedTags[k] = v
		}
	}
}

// deduplicateCandidates sorts candidates by confidence desc and removes duplicate
// Item IDs, keeping the highest-scoring entry for each item.
func deduplicateCandidates(candidates []domain.MatchCandidate) []domain.MatchCandidate {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Confidence > candidates[j].Confidence
	})
	seen := make(map[string]struct{}, len(candidates))
	out := make([]domain.MatchCandidate, 0, len(candidates))
	for _, c := range candidates {
		if _, dup := seen[c.Item.ID]; dup {
			continue
		}
		seen[c.Item.ID] = struct{}{}
		out = append(out, c)
	}
	return out
}

// confidenceFromSource maps a match strategy name to a MatchConfidence level.
// Cryptographic/database-verified strategies produce MatchVerified; heuristic
// strategies produce MatchNameMatched.
func confidenceFromSource(source string) domain.MatchConfidence {
	switch source {
	case "oshash", "acoustid", "musicbrainz_tag":
		return domain.MatchVerified
	default:
		return domain.MatchNameMatched
	}
}
