package fs

import (
	"context"
	"log/slog"
	"path/filepath"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"time"
)

type scanner struct {
	fs         ports.FileSystem
	mediaFiles ports.MediaFileRepository
	extMap     map[string]domain.ContentType
}

// NewScanner returns a FileScanner that emits candidate files for the scan
// pipeline. It uses ports.FileSystem.Walk so the filesystem is swappable in
// tests and production without changing scanner logic.
func NewScanner(fs ports.FileSystem, mediaFiles ports.MediaFileRepository) ports.FileScanner {
	return &scanner{
		fs:         fs,
		mediaFiles: mediaFiles,
		extMap:     buildExtMap(),
	}
}

var _ ports.FileScanner = (*scanner)(nil)

func (s *scanner) Scan(ctx context.Context, roots []string, f ports.ScanFilter) (<-chan domain.ScannedFile, error) {
	filterTypes := make(map[domain.ContentType]struct{}, len(f.ContentTypes))
	for _, ct := range f.ContentTypes {
		filterTypes[ct] = struct{}{}
	}

	ch := make(chan domain.ScannedFile)
	go func() {
		defer close(ch)
		for _, root := range roots {
			if err := s.walkRoot(ctx, root, filterTypes, ch); err != nil && ctx.Err() == nil {
				slog.WarnContext(ctx, "scan walk error", "root", root, "err", err)
			}
		}
	}()
	return ch, nil
}

func (s *scanner) walkRoot(ctx context.Context, root string, filterTypes map[domain.ContentType]struct{}, ch chan<- domain.ScannedFile) error {
	return s.fs.Walk(ctx, root, func(info ports.FileInfo) error {
		if info.IsDir {
			return nil
		}
		ext := filepath.Ext(info.Path)
		ct, ok := s.extMap[ext]
		if !ok {
			return nil
		}
		if len(filterTypes) > 0 {
			if _, ok := filterTypes[ct]; !ok {
				return nil
			}
		}

		existing, err := s.mediaFiles.GetByPath(ctx, info.Path)
		if err == nil && existing.Size == info.Size {
			return nil
		}
		if err != nil && !errs.IsNotFound(err) {
			slog.WarnContext(ctx, "media file path lookup failed", "path", info.Path, "err", err)
			return nil
		}

		select {
		case ch <- domain.ScannedFile{
			Path:         info.Path,
			Size:         info.Size,
			ContentType:  ct,
			DiscoveredAt: time.Now().UTC(),
		}:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
}

// buildExtMap returns a map from lowercase extension (with dot) to ContentType,
// built once from domain.ContentTypes(). No switch statement; extension lookup
// is O(1) at scan time.
func buildExtMap() map[string]domain.ContentType {
	m := make(map[string]domain.ContentType)
	for _, ct := range domain.ContentTypes() {
		for _, ext := range ct.MediaExtensions() {
			m[ext] = ct
		}
	}
	return m
}
