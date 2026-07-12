package fswatch

import (
	"path/filepath"
	"testing"
)

func TestDepthResolver_UnitFor(t *testing.T) {
	root := filepath.FromSlash("/library")

	tests := []struct {
		name     string
		depth    int
		path     string
		wantUnit string
		wantOK   bool
	}{
		{
			name:     "depth 2, file directly at unit depth",
			depth:    2,
			path:     filepath.Join(root, "Artist", "Album", "track01.flac"),
			wantUnit: filepath.Join(root, "Artist", "Album"),
			wantOK:   true,
		},
		{
			name:     "depth 2, sidecar file at unit level",
			depth:    2,
			path:     filepath.Join(root, "Artist", "Album", "cover.jpg"),
			wantUnit: filepath.Join(root, "Artist", "Album"),
			wantOK:   true,
		},
		{
			name:     "depth 2, multi-disc subfolder rolls up to the album unit",
			depth:    2,
			path:     filepath.Join(root, "Artist", "Album", "Disc 1", "track01.flac"),
			wantUnit: filepath.Join(root, "Artist", "Album"),
			wantOK:   true,
		},
		{
			name:     "depth 2, the unit directory's own path",
			depth:    2,
			path:     filepath.Join(root, "Artist", "Album"),
			wantUnit: filepath.Join(root, "Artist", "Album"),
			wantOK:   true,
		},
		{
			name:   "depth 2, shallower than the configured unit depth is ignored",
			depth:  2,
			path:   filepath.Join(root, "Artist"),
			wantOK: false,
		},
		{
			name:   "root itself is ignored when depth > 0",
			depth:  2,
			path:   root,
			wantOK: false,
		},
		{
			name:     "depth 0 coalesces everything to root",
			depth:    0,
			path:     filepath.Join(root, "Artist", "Album", "track01.flac"),
			wantUnit: root,
			wantOK:   true,
		},
		{
			name:     "depth 0, root itself is the unit",
			depth:    0,
			path:     root,
			wantUnit: root,
			wantOK:   true,
		},
		{
			name:     "depth 1, immediate child directory",
			depth:    1,
			path:     filepath.Join(root, "Title", "video.mp4"),
			wantUnit: filepath.Join(root, "Title"),
			wantOK:   true,
		},
		{
			name:   "path outside root is ignored",
			depth:  1,
			path:   filepath.Join(filepath.Dir(root), "elsewhere", "file.txt"),
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := DepthResolver{Depth: tt.depth}
			unit, ok := r.UnitFor(root, tt.path)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (unit=%q)", ok, tt.wantOK, unit)
			}
			if ok && unit != tt.wantUnit {
				t.Fatalf("unit = %q, want %q", unit, tt.wantUnit)
			}
		})
	}
}
