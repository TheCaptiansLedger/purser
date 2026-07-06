package identifier

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"time"

	"github.com/google/uuid"
)

// MusicGroupQueueWriter is a GroupIdentifier that persists every incoming
// ScannedFileGroup to the music scan queue as a pending entry with no candidates.
// It does no identification — its purpose is to make grouper output immediately
// visible via GET /api/music/queue so the pipeline can be verified at every
// scan step without waiting for the full album identifier.
//
// Task 14 (album identifier) reads from and updates these entries with
// identification results, or triggers auto-import when confidence is high enough.
type MusicGroupQueueWriter struct {
	queue ports.MusicScanGroupRepository
}

var _ ports.GroupIdentifier = (*MusicGroupQueueWriter)(nil)

// NewMusicGroupQueueWriter returns a GroupIdentifier that writes scan groups
// to the pending queue. Wire it before the album identifier so every scanned
// folder is API-visible even before identification completes.
func NewMusicGroupQueueWriter(queue ports.MusicScanGroupRepository) ports.GroupIdentifier {
	return &MusicGroupQueueWriter{queue: queue}
}

// ContentTypes declares that this writer handles music files only.
func (w *MusicGroupQueueWriter) ContentTypes() []domain.ContentType {
	return musicContentTypes
}

// Identify persists the group to the queue if no pending entry for the same
// folder already exists. Idempotent: re-scanning an already-queued folder
// updates the track and disc counts rather than creating a duplicate.
func (w *MusicGroupQueueWriter) Identify(ctx context.Context, group ports.ScannedFileGroup) error {
	rootPath := filepath.Clean(group.RootPath)

	pending, err := w.queue.List(ctx, domain.UnmatchedPending)
	if err != nil {
		return fmt.Errorf("list pending music scan groups: %w", err)
	}

	totalDiscs := countDiscs(group)

	for _, existing := range pending {
		if filepath.Clean(existing.FolderPath) == rootPath {
			existing.TotalTracks = len(group.Files)
			existing.TotalDiscs = totalDiscs
			existing.Files = group.Files
			if err := w.queue.Save(ctx, existing); err != nil {
				return fmt.Errorf("update existing music scan group: %w", err)
			}
			slog.DebugContext(ctx, "music group queue updated",
				"root", rootPath,
				"tracks", existing.TotalTracks,
				"discs", totalDiscs,
				"id", existing.ID,
			)
			return nil
		}
	}

	g := &domain.MusicScanGroup{
		ID:           uuid.New().String(),
		FolderPath:   rootPath,
		Files:        group.Files,
		TotalTracks:  len(group.Files),
		TotalDiscs:   totalDiscs,
		Status:       domain.UnmatchedPending,
		DiscoveredAt: time.Now().UTC(),
	}
	if err := w.queue.Save(ctx, g); err != nil {
		return fmt.Errorf("save music scan group: %w", err)
	}
	slog.InfoContext(ctx, "music group queued",
		"root", rootPath,
		"tracks", g.TotalTracks,
		"discs", g.TotalDiscs,
		"id", g.ID,
	)
	return nil
}

// countDiscs returns the number of distinct disc sub-folders in a group.
// For single-disc albums all files sit directly under RootPath → returns 1.
// For multi-disc albums files sit in CD1/CD2/… sub-directories → returns N.
func countDiscs(group ports.ScannedFileGroup) int {
	root := filepath.Clean(group.RootPath)
	dirs := make(map[string]struct{})
	for _, f := range group.Files {
		dir := filepath.Dir(f.Path)
		if filepath.Clean(dir) != root {
			dirs[dir] = struct{}{}
		}
	}
	if len(dirs) == 0 {
		return 1
	}
	return len(dirs)
}
