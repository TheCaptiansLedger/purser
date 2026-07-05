package scan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Module describes a content-type module's roots for a scan-all job.
type Module struct {
	ContentType domain.ContentType
	Roots       []string
}

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
	jobs           ports.JobQueue
	entries        ports.LibraryEntryRepository
	groups         ports.GroupRepository
	thumbnailCache ports.ThumbnailCache
	upgradeMode    map[domain.ContentType]string
}

// New constructs a scan Service wired to the given ports.
// upgradeMode maps each ContentType to "auto" or "queue" (default when empty).
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
	jobs ports.JobQueue,
	entries ports.LibraryEntryRepository,
	groups ports.GroupRepository,
	thumbnailCache ports.ThumbnailCache,
	upgradeMode map[domain.ContentType]string,
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
		jobs:           jobs,
		entries:        entries,
		groups:         groups,
		thumbnailCache: thumbnailCache,
		upgradeMode:    upgradeMode,
	}
}

// ScanRoots walks roots through the scanner and runs the full pipeline on each discovered file.
func (s *Service) ScanRoots(ctx context.Context, roots []string, filter ports.ScanFilter) error {
	_ = s.notifier.Dispatch(ctx, domain.NotificationEvent{Type: domain.NotifyScanStarted})

	s.pruneResolvedUnmatched(ctx)

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

// StartWatching watches module roots for filesystem events and runs the pipeline
// on each file. ContentType is resolved from the module root prefix, not from
// the file extension, so adult and JAV files are never misclassified as movie/TV.
// Blocks until ctx is cancelled or the watcher channel closes.
func (s *Service) StartWatching(ctx context.Context, modules []Module) error {
	rootTypes := buildRootContentTypes(modules)
	roots := make([]string, 0, len(rootTypes))
	for root := range rootTypes {
		roots = append(roots, root)
	}

	slog.InfoContext(ctx, "watcher: starting", "roots", roots)
	ch, err := s.watcher.Watch(ctx, roots)
	if err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			wg.Add(1)
			go func(e ports.WatchEvent) {
				defer wg.Done()
				s.handleWatchEvent(ctx, e, rootTypes)
			}(event)
		}
	}
}

// buildRootContentTypes returns a map of root path → ContentType built from the
// module list. Each root is stored with a trailing slash so HasPrefix matching
// never confuses /media/content/jav with /media/content/jav-extra.
func buildRootContentTypes(modules []Module) map[string]domain.ContentType {
	m := make(map[string]domain.ContentType)
	for _, mod := range modules {
		for _, root := range mod.Roots {
			key := strings.TrimRight(root, "/") + "/"
			m[key] = mod.ContentType
		}
	}
	return m
}

// contentTypeForPath returns the ContentType for a file path by finding the
// longest matching module root prefix. Returns "" if no module owns the path.
func contentTypeForPath(path string, rootTypes map[string]domain.ContentType) domain.ContentType {
	best := ""
	var ct domain.ContentType
	for root, t := range rootTypes {
		if strings.HasPrefix(path, root) && len(root) > len(best) {
			best = root
			ct = t
		}
	}
	return ct
}

// watchFileTimeout caps how long a single file's fingerprinting + identification may take.
// Network-backed identifiers (AcoustID, MusicBrainz, StashDB) can hang without this bound.
const watchFileTimeout = 5 * time.Minute

func (s *Service) handleWatchEvent(ctx context.Context, event ports.WatchEvent, rootTypes map[string]domain.ContentType) {
	switch event.Op {
	case ports.WatchCreated, ports.WatchModified:
		ct := contentTypeForPath(event.Path, rootTypes)
		if ct == "" {
			slog.DebugContext(ctx, "watcher: ignoring event: path not under any watched root", "path", event.Path)
			return
		}
		slog.InfoContext(ctx, "watcher: queuing file for processing", "path", event.Path, "content_type", ct)
		f := domain.ScannedFile{
			Path:         event.Path,
			Size:         event.Size,
			ContentType:  ct,
			DiscoveredAt: time.Now().UTC(),
		}
		fileCtx, cancel := context.WithTimeout(ctx, watchFileTimeout)
		defer cancel()
		if err := s.processFile(fileCtx, f); err != nil {
			slog.WarnContext(ctx, "watcher: process file failed", "path", event.Path, "err", err)
		}
	case ports.WatchRemoved:
		s.handleRemoved(ctx, event.Path)
	}
}

// processFile is the single pipeline entry point for both ScanRoots and StartWatching.
func (s *Service) processFile(ctx context.Context, f domain.ScannedFile) error {
	// 1. Path idempotency: same path + same size means nothing changed.
	if done, err := s.checkPathIdempotency(ctx, &f); err != nil || done {
		return err
	}
	// 2. Fingerprint + identify.
	f.Fingerprint = s.collectFingerprints(ctx, f)
	candidates := s.collectCandidates(ctx, f)

	_ = s.notifier.Dispatch(ctx, domain.NotificationEvent{
		Type:    domain.NotifyFileDiscovered,
		Payload: f.Path,
	})

	// 3. Hash-state resolution: skip or update path for known content.
	if done, err := s.resolveHashState(ctx, f); err != nil || done {
		return err
	}
	// 4. Upgrade / duplicate: item already has a file at this quality or better.
	if done, err := s.handleExistingItemFile(ctx, f, candidates); err != nil || done {
		return err
	}
	// 5. Association decision: auto-import when confidence meets threshold.
	// Candidates are sorted by descending confidence; pick the first one that
	// links to a library item. AcoustID can return multiple MBIDs for the same
	// recording (different releases), only some of which are in the library.
	for _, c := range candidates {
		if c.Confidence >= s.threshold && c.Item != nil {
			return s.autoImport(ctx, f, c)
		}
	}
	if len(candidates) == 0 {
		slog.WarnContext(ctx, "no candidates found for file, queuing unmatched", "path", f.Path)
	}
	return s.enqueueUnmatched(ctx, f, candidates, "")
}

func (s *Service) checkPathIdempotency(ctx context.Context, f *domain.ScannedFile) (bool, error) {
	existing, err := s.mediaFiles.GetByPath(ctx, f.Path)
	if errs.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("path lookup: %w", err)
	}
	if existing.Size == f.Size {
		s.cleanUnmatchedAtPath(ctx, f.Path)
		return true, nil // unchanged file
	}
	// Same path, different size: file replaced in-place. Drop the stale record.
	if delErr := s.mediaFiles.Delete(ctx, existing.ID); delErr != nil {
		return false, fmt.Errorf("delete replaced media file: %w", delErr)
	}
	return false, nil
}

func (s *Service) collectFingerprints(ctx context.Context, f domain.ScannedFile) *domain.Fingerprint {
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
	return fp
}

func (s *Service) collectCandidates(ctx context.Context, f domain.ScannedFile) []domain.MatchCandidate {
	var all []domain.MatchCandidate
	for _, ider := range s.identifiers {
		if !slices.Contains(ider.ContentTypes(), f.ContentType) {
			continue
		}
		got, err := ider.Identify(ctx, f)
		if err != nil {
			slog.WarnContext(ctx, "identifier failed", "path", f.Path, "err", err)
			continue
		}
		all = append(all, got...)
	}
	return deduplicateCandidates(all)
}

// resolveHashState handles the case where content with the same OSHash is already
// tracked: skips (same path) or updates the path record (file moved/renamed).
func (s *Service) resolveHashState(ctx context.Context, f domain.ScannedFile) (bool, error) {
	if f.Fingerprint == nil || f.Fingerprint.OSHash == "" {
		return false, nil
	}
	byHash, err := s.mediaFiles.GetByOSHash(ctx, f.Fingerprint.OSHash)
	if errs.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("oshash lookup: %w", err)
	}
	if byHash.Path == f.Path {
		return true, nil // same content, same path — already processed
	}
	// Different path — treat as a moved/renamed file: update the record in-place.
	byHash.Path = f.Path
	if saveErr := s.mediaFiles.Save(ctx, byHash); saveErr != nil {
		return false, fmt.Errorf("update path for moved file: %w", saveErr)
	}
	return true, nil
}

// handleExistingItemFile checks whether the best candidate's item already owns a
// media file. If it does, the new file is either an upgrade or a duplicate and is
// handled accordingly. Returns (true, nil) when the file has been fully handled.
func (s *Service) handleExistingItemFile(ctx context.Context, f domain.ScannedFile, candidates []domain.MatchCandidate) (bool, error) {
	if len(candidates) == 0 || candidates[0].Item == nil {
		return false, nil
	}
	existingMF, err := s.mediaFiles.GetByItemID(ctx, candidates[0].Item.ID)
	if errs.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("item media file lookup: %w", err)
	}
	incoming := qualityFromResolution(embeddedTag(f.Fingerprint, "resolution"))
	upgradeMode := s.upgradeMode[f.ContentType]
	if upgradeMode == "" {
		upgradeMode = "queue"
	}
	if incoming != "" && qualityRank(incoming) > qualityRank(existingMF.Quality) {
		if upgradeMode == "auto" {
			return true, s.applyUpgrade(ctx, f, candidates[0], existingMF.ID)
		}
		_ = s.notifier.Dispatch(ctx, domain.NotificationEvent{
			Type:    domain.NotifyUpgradeQueued,
			Payload: map[string]any{"path": f.Path, "item_id": candidates[0].Item.ID},
		})
	}
	return true, s.enqueueUnmatched(ctx, f, candidates, existingMF.ID)
}

func (s *Service) applyUpgrade(ctx context.Context, f domain.ScannedFile, candidate domain.MatchCandidate, oldID string) error {
	if delErr := s.mediaFiles.Delete(ctx, oldID); delErr != nil {
		return fmt.Errorf("delete old media file for upgrade: %w", delErr)
	}
	_ = s.notifier.Dispatch(ctx, domain.NotificationEvent{
		Type:    domain.NotifyUpgradeApplied,
		Payload: map[string]any{"path": f.Path, "item_id": candidate.Item.ID},
	})
	return s.autoImport(ctx, f, candidate)
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
		mf.Quality = qualityFromResolution(mf.Resolution)
	}
	if err := s.mediaFiles.Save(ctx, mf); err != nil {
		return fmt.Errorf("save media file: %w", err)
	}

	candidate.Item.Status = domain.StatusImported
	candidate.Item.MediaFile = mf
	if err := s.items.Save(ctx, candidate.Item); err != nil {
		return fmt.Errorf("save item: %w", err)
	}

	s.cleanUnmatchedAtPath(ctx, f.Path)

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

func (s *Service) enqueueUnmatched(ctx context.Context, f domain.ScannedFile, candidates []domain.MatchCandidate, duplicateOf string) error {
	existing, err := s.unmatched.List(ctx, ports.UnmatchedFilter{
		Path:   f.Path,
		Status: domain.UnmatchedPending,
	})
	if err != nil {
		return fmt.Errorf("check unmatched file: %w", err)
	}

	var uf *domain.UnmatchedFile
	if len(existing) > 0 {
		uf = existing[0]
		uf.Fingerprint = f.Fingerprint
		uf.Candidates = candidates
		uf.Size = f.Size
		uf.DuplicateOf = duplicateOf
	} else {
		uf = &domain.UnmatchedFile{
			ID:           uuid.New().String(),
			Path:         f.Path,
			Size:         f.Size,
			ContentType:  f.ContentType,
			Fingerprint:  f.Fingerprint,
			Candidates:   candidates,
			DiscoveredAt: time.Now().UTC(),
			Status:       domain.UnmatchedPending,
			DuplicateOf:  duplicateOf,
		}
	}

	// Cache a thumbnail for queue display (once per entry; skip re-fetch on update).
	if s.thumbnailCache != nil && uf.ThumbnailPath == "" {
		for _, c := range candidates {
			if c.ExternalItem != nil && c.ExternalItem.ImageURL != "" {
				uf.ThumbnailPath = s.thumbnailCache.Store(ctx, c.ExternalItem.ImageURL, uf.ID)
				if uf.ThumbnailPath != "" {
					break
				}
			}
		}
	}

	if err := s.unmatched.Save(ctx, uf); err != nil {
		return fmt.Errorf("save unmatched file: %w", err)
	}

	payload := map[string]any{
		"path":         f.Path,
		"content_type": string(f.ContentType),
	}
	if len(candidates) > 0 {
		c := candidates[0]
		switch {
		case c.Item != nil:
			payload["top_candidate"] = c.Item.Title
		case c.ExternalItem != nil:
			payload["top_candidate"] = c.ExternalItem.Title
		}
	}
	_ = s.notifier.Dispatch(ctx, domain.NotificationEvent{
		Type:    domain.NotifyUnmatched,
		Payload: payload,
	})
	if len(existing) == 0 {
		_ = s.notifier.Dispatch(ctx, domain.NotificationEvent{
			Type:    domain.NotifyNewUnmatched,
			Payload: payload,
		})
	}
	return nil
}

func (s *Service) handleRemoved(ctx context.Context, path string) {
	mf, err := s.mediaFiles.GetByPath(ctx, path)
	if errs.IsNotFound(err) {
		s.cleanUnmatchedAtPath(ctx, path)
		return
	}
	if err != nil {
		slog.WarnContext(ctx, "media file lookup failed on removal", "path", path, "err", err)
		return
	}
	if delErr := s.mediaFiles.Delete(ctx, mf.ID); delErr != nil {
		slog.WarnContext(ctx, "failed to delete media file on removal", "path", path, "err", delErr)
	}
	if mf.ItemID != "" {
		item, itemErr := s.items.Get(ctx, mf.ItemID)
		if itemErr == nil {
			item.Status = domain.StatusMissing
			item.MediaFile = nil
			if saveErr := s.items.Save(ctx, item); saveErr != nil {
				slog.WarnContext(ctx, "failed to update item status for removed file", "path", path, "err", saveErr)
			}
		}
		_ = s.notifier.Dispatch(ctx, domain.NotificationEvent{
			Type:    domain.NotifyFileMissing,
			Payload: map[string]any{"path": path, "item_id": mf.ItemID},
		})
	}
	s.cleanUnmatchedAtPath(ctx, path)
}

func (s *Service) cleanUnmatchedAtPath(ctx context.Context, path string) {
	pending, err := s.unmatched.List(ctx, ports.UnmatchedFilter{Path: path, Status: domain.UnmatchedPending})
	if err != nil {
		return
	}
	for _, uf := range pending {
		_ = s.unmatched.Delete(ctx, uf.ID)
	}
}

// pruneResolvedUnmatched deletes pending unmatched records whose path already
// has a media file. This catches stale records created before the file was
// matched via a code path that didn't clean up the queue (e.g. a scan that ran
// after an external import, or a match done through a non-ManualMatch route).
func (s *Service) pruneResolvedUnmatched(ctx context.Context) {
	pending, err := s.unmatched.List(ctx, ports.UnmatchedFilter{Status: domain.UnmatchedPending})
	if err != nil {
		return
	}
	for _, uf := range pending {
		if _, err := s.mediaFiles.GetByPath(ctx, uf.Path); err == nil {
			_ = s.unmatched.Delete(ctx, uf.ID)
		}
	}
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
		var key string
		switch {
		case c.Item != nil:
			key = "item:" + c.Item.ID
		case c.ExternalItem != nil:
			key = "ext:" + string(c.ExternalItem.Source) + ":" + c.ExternalItem.ExternalID
		default:
			out = append(out, c)
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	return out
}

// confidenceFromSource maps a match strategy name to a MatchConfidence level.
func confidenceFromSource(source string) domain.MatchConfidence {
	switch source {
	case "oshash", "acoustid", "musicbrainz_tag":
		return domain.MatchVerified
	default:
		return domain.MatchNameMatched
	}
}

// embeddedTag safely reads a key from a possibly-nil Fingerprint.EmbeddedTags.
func embeddedTag(fp *domain.Fingerprint, key string) string {
	if fp == nil || fp.EmbeddedTags == nil {
		return ""
	}
	return fp.EmbeddedTags[key]
}

// qualityFromResolution parses a "WxH" resolution string into a Quality tier.
// Returns "" when the input is empty or cannot be parsed.
func qualityFromResolution(resolution string) domain.Quality {
	if resolution == "" {
		return ""
	}
	parts := strings.SplitN(resolution, "x", 2)
	if len(parts) != 2 {
		return ""
	}
	h, err := strconv.Atoi(parts[1])
	if err != nil {
		return ""
	}
	return domain.QualityFromHeight(h)
}

// qualityRank returns a numeric ordering for Quality values (higher = better).
func qualityRank(q domain.Quality) int {
	switch q {
	case domain.Quality4K:
		return 4
	case domain.Quality1080:
		return 3
	case domain.Quality720:
		return 2
	case domain.Quality480:
		return 1
	case domain.QualitySD:
		return 0
	default:
		return -1
	}
}

// ── Queue-entry scan commands ─────────────────────────────────────────────────

// SubmitScanLibraryJob enqueues a background scan for a single library entry's configured path.
func (s *Service) SubmitScanLibraryJob(ctx context.Context, entryID string) (*domain.Job, error) {
	if entryID == "" {
		return nil, errs.Validation("entryId is required")
	}
	entry, err := s.entries.Get(ctx, entryID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get entry: %w", err)
	}
	if entry.Path == "" {
		return nil, errs.Validation("library entry has no path configured")
	}
	roots := []string{entry.Path}
	filter := ports.ScanFilter{EntryID: entryID, ContentTypes: []domain.ContentType{entry.ContentType}}
	return s.jobs.Submit(ctx, "ScanLibrary", map[string]any{"entry_id": entryID},
		func(jobCtx context.Context, _ ports.ProgressReporter) error {
			return s.ScanRoots(jobCtx, roots, filter)
		})
}

// SubmitScanAllRootsJob enqueues a background scan across all provided modules.
// Each module is scanned with its specific content type so the scanner can
// apply the correct extension filter and the pipeline assigns the right type.
func (s *Service) SubmitScanAllRootsJob(ctx context.Context, modules []Module) (*domain.Job, error) {
	return s.jobs.Submit(ctx, "ScanAllRoots", nil,
		func(jobCtx context.Context, _ ports.ProgressReporter) error {
			for _, m := range modules {
				if len(m.Roots) == 0 {
					continue
				}
				filter := ports.ScanFilter{ContentTypes: []domain.ContentType{m.ContentType}}
				if err := s.ScanRoots(jobCtx, m.Roots, filter); err != nil {
					slog.WarnContext(jobCtx, "module scan failed", "content_type", m.ContentType, "err", err)
				}
			}
			return nil
		})
}

// ── Unmatched file queue ──────────────────────────────────────────────────────

// ListUnmatched returns unmatched files matching the given filter.
func (s *Service) ListUnmatched(ctx context.Context, f ports.UnmatchedFilter) ([]*domain.UnmatchedFile, error) {
	return s.unmatched.List(ctx, f)
}

// GetUnmatched returns a single unmatched file by ID.
func (s *Service) GetUnmatched(ctx context.Context, id string) (*domain.UnmatchedFile, error) {
	uf, err := s.unmatched.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	return uf, err
}

// Dismiss marks an unmatched file as dismissed, removing it from the pending queue.
func (s *Service) Dismiss(ctx context.Context, id string) error {
	uf, err := s.unmatched.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return errs.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get unmatched file: %w", err)
	}
	uf.Status = domain.UnmatchedDismissed
	if err := s.unmatched.Save(ctx, uf); err != nil {
		return fmt.Errorf("save dismissed file: %w", err)
	}
	return nil
}

// Rescrape re-runs the identifier chain against an unmatched file's stored fingerprint.
// If query is non-empty it overrides the path used by filename-parser strategies.
// The UnmatchedFile record is not modified.
func (s *Service) Rescrape(ctx context.Context, id, query string) ([]domain.MatchCandidate, error) {
	uf, err := s.unmatched.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get unmatched file: %w", err)
	}

	path := uf.Path
	if query != "" {
		path = query
	}

	f := domain.ScannedFile{
		Path:         path,
		Size:         uf.Size,
		ContentType:  uf.ContentType,
		Fingerprint:  uf.Fingerprint,
		DiscoveredAt: uf.DiscoveredAt,
	}

	var candidates []domain.MatchCandidate
	for _, ider := range s.identifiers {
		if !slices.Contains(ider.ContentTypes(), f.ContentType) {
			continue
		}
		got, err := ider.Identify(ctx, f)
		if err != nil {
			slog.WarnContext(ctx, "identifier failed during rescrape", "path", f.Path, "err", err)
			continue
		}
		candidates = append(candidates, got...)
	}
	return deduplicateCandidates(candidates), nil
}

// ListUnmatchedGrouped returns pending unmatched files grouped by the best candidate's album.
// Files with no candidates are collected into a group with an empty GroupID.
func (s *Service) ListUnmatchedGrouped(ctx context.Context, f ports.UnmatchedFilter) ([]*domain.UnmatchedFileGroup, error) {
	files, err := s.unmatched.List(ctx, f)
	if err != nil {
		return nil, err
	}

	type key = string
	byGroup := make(map[key]*domain.UnmatchedFileGroup)
	var order []key

	addToGroup := func(groupID, groupTitle string, uf *domain.UnmatchedFile, conf float64) {
		if _, ok := byGroup[groupID]; !ok {
			byGroup[groupID] = &domain.UnmatchedFileGroup{
				GroupID:    groupID,
				GroupTitle: groupTitle,
			}
			order = append(order, groupID)
		}
		g := byGroup[groupID]
		g.Files = append(g.Files, uf)
		if conf > g.BestCandidateConfidence {
			g.BestCandidateConfidence = conf
		}
	}

	for _, uf := range files {
		if len(uf.Candidates) == 0 {
			addToGroup("", "", uf, 0)
			continue
		}
		best := uf.Candidates[0]

		// Prefer external album ID (pre-import files). Fall back to the local group
		// when the candidate already links to an imported library item.
		if ext := best.ExternalItem; ext != nil && ext.GroupExternalID != "" {
			addToGroup(ext.GroupExternalID, ext.GroupTitle, uf, best.Confidence)
			continue
		}
		if best.Item != nil {
			item, err := s.items.Get(ctx, best.Item.ID)
			if err != nil {
				addToGroup("", "", uf, best.Confidence)
				continue
			}
			groupTitle := ""
			if item.GroupID != "" && s.groups != nil {
				if grp, err := s.groups.Get(ctx, item.GroupID); err == nil {
					groupTitle = grp.Title
				}
			}
			addToGroup(item.GroupID, groupTitle, uf, best.Confidence)
			continue
		}
		addToGroup("", "", uf, best.Confidence)
	}

	result := make([]*domain.UnmatchedFileGroup, 0, len(byGroup))
	for _, k := range order {
		result = append(result, byGroup[k])
	}
	return result, nil
}
