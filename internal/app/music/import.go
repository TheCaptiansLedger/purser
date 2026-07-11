// Package music contains application services for the music import pipeline.
package music

import (
	"context"
	"fmt"
	"log/slog"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// ReleaseImporter implements ports.AlbumImporter. Given the winning candidate
// from album-level identification and the scanned files that produced it, it
// flips the matching MusicRelease stub to imported, creates one Item and
// MediaFile per track from the scanned files' own embedded tags, computes each
// file's SHA1, and writes MusicBrainz IDs back into the file tags so a later
// re-scan can recognize the release by tag alone.
//
// A matching MusicRelease stub — and the LibraryEntry/Group chain above it —
// must already exist, created when the artist/album was added to the library.
// ReleaseImporter does not create a library entry from scratch; that remains
// the responsibility of the existing artist-import flow.
type ReleaseImporter struct {
	releases   ports.MusicReleaseRepository
	items      ports.ItemRepository
	mediaFiles ports.MediaFileRepository
	fileSystem ports.FileSystem
	tagWriter  ports.MusicTagWriter
}

// NewReleaseImporter returns a ports.AlbumImporter backed by the given repositories.
func NewReleaseImporter(
	releases ports.MusicReleaseRepository,
	items ports.ItemRepository,
	mediaFiles ports.MediaFileRepository,
	fileSystem ports.FileSystem,
	tagWriter ports.MusicTagWriter,
) ports.AlbumImporter {
	return &ReleaseImporter{
		releases:   releases,
		items:      items,
		mediaFiles: mediaFiles,
		fileSystem: fileSystem,
		tagWriter:  tagWriter,
	}
}

var _ ports.AlbumImporter = (*ReleaseImporter)(nil)

// ImportRelease creates every track for the winning candidate and flips the
// release stub to imported. If any track fails to import, the release is left
// in its prior status so a failed pass does not falsely report completion.
func (r *ReleaseImporter) ImportRelease(ctx context.Context, candidate domain.MusicReleaseCandidate, group ports.ScannedFileGroup) error {
	slog.InfoContext(ctx, "release import start",
		"release_mbid", candidate.ReleaseMBID, "title", candidate.ReleaseTitle, "track_count", len(group.Files))

	release, err := r.resolveRelease(ctx, candidate)
	if err != nil {
		return fmt.Errorf("resolve release: %w", err)
	}

	for _, f := range group.Files {
		if err := r.importTrack(ctx, release, candidate.ReleaseMBID, f); err != nil {
			return fmt.Errorf("import track %s: %w", f.Path, err)
		}
	}

	release.Status = domain.ReleaseStatusImported
	release.TrackCount = len(group.Files)
	release.UpdatedAt = time.Now().UTC()
	if err := r.releases.Save(ctx, release); err != nil {
		return fmt.Errorf("save release: %w", err)
	}

	slog.InfoContext(ctx, "release import complete",
		"release_id", release.ID, "tracks", len(group.Files), "files", len(group.Files), "status", release.Status)
	return nil
}

// resolveRelease finds the MusicRelease stub matching the winning candidate's
// MBID and applies the candidate's metadata onto it. The stub is expected to
// already exist — created when the artist/album was added to the library.
func (r *ReleaseImporter) resolveRelease(ctx context.Context, candidate domain.MusicReleaseCandidate) (*domain.MusicRelease, error) {
	release, err := r.releases.GetByMBID(ctx, candidate.ReleaseMBID)
	if err != nil {
		if errs.IsNotFound(err) {
			return nil, fmt.Errorf("no release stub for mbid %s: import the artist/album before scanning its files", candidate.ReleaseMBID)
		}
		return nil, fmt.Errorf("get release by mbid: %w", err)
	}
	applyCandidateToRelease(release, candidate)
	return release, nil
}

func applyCandidateToRelease(rel *domain.MusicRelease, c domain.MusicReleaseCandidate) {
	if c.ReleaseTitle != "" {
		rel.Title = c.ReleaseTitle
	}
	if c.ReleaseCountry != "" {
		rel.Country = c.ReleaseCountry
	}
	if c.ReleaseLabel != "" {
		rel.Label = c.ReleaseLabel
	}
	if c.ReleaseBarcode != "" {
		rel.Barcode = c.ReleaseBarcode
	}
	if c.ReleaseFormat != "" {
		rel.Format = c.ReleaseFormat
	}
	if c.ReleaseMediumCount > 0 {
		rel.MediumCount = c.ReleaseMediumCount
	}
	if t, err := time.Parse("2006-01-02", c.ReleaseDate); err == nil {
		rel.Date = t.UTC()
	}
}

// importTrack creates the Item and MediaFile for one scanned file, then writes
// the release/recording MBZ IDs back into the file's tags. Tag write-back
// failure is logged but does not abort the import — the file is already
// correctly imported and tagged in Purser's own database at that point.
func (r *ReleaseImporter) importTrack(ctx context.Context, release *domain.MusicRelease, releaseMBID string, f domain.ScannedFile) error {
	tags := embeddedTags(f)
	now := time.Now().UTC()

	item := &domain.Item{
		ID:             uuid.New().String(),
		ContentType:    domain.ContentTypeMusic,
		LibraryEntryID: release.LibraryEntryID,
		GroupID:        release.GroupID,
		Title:          tags["title"],
		Sequence:       tags["track_number"],
		RuntimeSeconds: runtimeSeconds(tags["duration_ms"]),
		Monitored:      true,
		Status:         domain.StatusImported,
		Metadata: map[string]any{
			"release_id":  release.ID,
			"disc_number": discNumber(tags["disc_number"]),
			"isrc":        tags["isrc"],
		},
		AddedAt: now,
	}
	recordingMBID := tags["musicbrainz_track_id"]
	if recordingMBID != "" {
		item.ExternalIDs = []domain.ExternalID{{Source: domain.SourceMBZRecording, Value: recordingMBID}}
	}
	item.ApplyDefaults()

	mf := &domain.MediaFile{
		ID:       uuid.New().String(),
		ItemID:   item.ID,
		Path:     f.Path,
		Size:     f.Size,
		SHA1:     r.computeSHA1(ctx, f.Path),
		Metadata: tags,
		AddedAt:  now,
	}
	if f.Fingerprint != nil {
		mf.OSHash = f.Fingerprint.OSHash
	}
	if err := r.mediaFiles.Save(ctx, mf); err != nil {
		return fmt.Errorf("save media file: %w", err)
	}

	item.MediaFile = mf
	if err := r.items.Save(ctx, item); err != nil {
		return fmt.Errorf("save item: %w", err)
	}

	if err := r.tagWriter.WriteIDs(ctx, f.Path, releaseMBID, recordingMBID); err != nil {
		slog.WarnContext(ctx, "tag write-back failed", "path", f.Path, "error", err)
	} else {
		slog.DebugContext(ctx, "tag write-back", "path", f.Path, "release_mbid", releaseMBID, "recording_mbid", recordingMBID)
	}
	return nil
}

func (r *ReleaseImporter) computeSHA1(ctx context.Context, path string) string {
	sum, err := r.fileSystem.SHA1(ctx, path)
	if err != nil {
		slog.WarnContext(ctx, "sha1 compute failed", "path", path, "error", err)
		return ""
	}
	return sum
}

func embeddedTags(f domain.ScannedFile) map[string]string {
	if f.Fingerprint == nil || f.Fingerprint.EmbeddedTags == nil {
		return map[string]string{}
	}
	return f.Fingerprint.EmbeddedTags
}

func runtimeSeconds(durationMS string) int {
	ms, _ := strconv.Atoi(durationMS)
	if ms <= 0 {
		return 0
	}
	return ms / 1000
}

func discNumber(tag string) string {
	if tag == "" {
		return "1"
	}
	return tag
}
