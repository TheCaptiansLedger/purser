package identifier

import (
	"context"
	"log/slog"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"regexp"
	"slices"
	"sort"
)

// multiDiscPattern matches common multi-disc sub-folder names: CD1, Disc 1, Disk 2, etc.
var multiDiscPattern = regexp.MustCompile(`(?i)^(cd\d+|disc\s*\d+|disk\s*\d+)$`)

// MusicFolderGrouper implements ports.FileGrouper for music content.
// Groups ScannedFiles by folder. When a common parent has two or more sibling
// sub-folders matching the multi-disc pattern (CD1/CD2, Disc N), those
// sub-folders are merged into a single ScannedFileGroup whose RootPath is
// the common parent.
type MusicFolderGrouper struct{}

var _ ports.FileGrouper = (*MusicFolderGrouper)(nil)

// NewMusicFolderGrouper returns a FileGrouper for music content.
func NewMusicFolderGrouper() ports.FileGrouper {
	return &MusicFolderGrouper{}
}

// ContentTypes declares that this grouper handles music files only.
func (g *MusicFolderGrouper) ContentTypes() []domain.ContentType {
	return musicContentTypes
}

// Group groups music files by folder. Multi-disc sub-folders are merged into one group.
func (g *MusicFolderGrouper) Group(ctx context.Context, files []domain.ScannedFile) ([]ports.ScannedFileGroup, error) {
	var musicFiles []domain.ScannedFile
	for _, f := range files {
		if slices.Contains(musicContentTypes, f.ContentType) {
			musicFiles = append(musicFiles, f)
		}
	}

	if len(musicFiles) == 0 {
		slog.InfoContext(ctx, "music folder grouper", "total_files", 0, "groups", 0)
		return nil, nil //nolint:nilnil // port contract: no groups is a valid empty result
	}

	// Group files by their containing directory.
	byDir := make(map[string][]domain.ScannedFile)
	for _, f := range musicFiles {
		dir := filepath.Dir(f.Path)
		byDir[dir] = append(byDir[dir], f)
	}

	type discEntry struct {
		dir   string
		files []domain.ScannedFile
	}

	// Classify each directory: disc sub-folder or standalone album folder.
	byParent := make(map[string][]discEntry)
	var standaloneDirs []string

	for dir := range byDir {
		base := filepath.Base(dir)
		if multiDiscPattern.MatchString(base) {
			parent := filepath.Dir(dir)
			byParent[parent] = append(byParent[parent], discEntry{dir: dir, files: byDir[dir]})
		} else {
			standaloneDirs = append(standaloneDirs, dir)
		}
	}

	var groups []ports.ScannedFileGroup

	// Parents with ≥2 disc sub-folders become one merged group.
	// Parents with only one disc sub-folder fall back to standalone treatment.
	for parent, discs := range byParent {
		if len(discs) < 2 {
			standaloneDirs = append(standaloneDirs, discs[0].dir)
			continue
		}

		sort.Slice(discs, func(i, j int) bool { return discs[i].dir < discs[j].dir })

		subdirNames := make([]string, len(discs))
		var allFiles []domain.ScannedFile
		for i, d := range discs {
			subdirNames[i] = filepath.Base(d.dir)
			allFiles = append(allFiles, d.files...)
		}

		slog.DebugContext(ctx, "multi-disc detected", "root", parent, "subdirs", subdirNames)

		group := ports.ScannedFileGroup{
			Files:    allFiles,
			RootPath: parent,
		}
		slog.DebugContext(ctx, "music group formed",
			"root", group.RootPath,
			"tracks", len(allFiles),
			"discs", len(discs),
			"multi_disc", true,
		)
		groups = append(groups, group)
	}

	sort.Strings(standaloneDirs)
	for _, dir := range standaloneDirs {
		dirFiles := byDir[dir]
		if len(dirFiles) == 0 {
			continue
		}
		group := ports.ScannedFileGroup{
			Files:    dirFiles,
			RootPath: dir,
		}
		slog.DebugContext(ctx, "music group formed",
			"root", group.RootPath,
			"tracks", len(dirFiles),
			"discs", 1,
			"multi_disc", false,
		)
		groups = append(groups, group)
	}

	slog.InfoContext(ctx, "music folder grouper", "total_files", len(musicFiles), "groups", len(groups))
	return groups, nil
}
