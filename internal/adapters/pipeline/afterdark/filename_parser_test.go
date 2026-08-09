package afterdark_test

import (
	"context"
	"purser/internal/adapters/pipeline/afterdark"
	"purser/internal/domain"
	"testing"
)

func TestFilenameParser_ContentTypes(t *testing.T) {
	p := afterdark.FilenameParser{}
	got := p.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeAdult {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeAdult)
	}
}

func TestFilenameParser_Parse(t *testing.T) {
	tests := []struct {
		name       string
		groupPath  string
		scanRoot   string
		wantStudio string
		wantTitle  string
		wantOK     bool
	}{
		{
			name:       "delimiter split with hyphen",
			groupPath:  "/scenes/Brazzers - Sneaking Into My Roommate.mp4",
			scanRoot:   "/scenes",
			wantStudio: "Brazzers",
			wantTitle:  "Sneaking Into My Roommate",
			wantOK:     true,
		},
		{
			name:       "delimiter split with en dash",
			groupPath:  "/scenes/X-Art – Sunset Delight.mkv",
			scanRoot:   "/scenes",
			wantStudio: "X-Art",
			wantTitle:  "Sunset Delight",
			wantOK:     true,
		},
		{
			name:       "delimiter split with underscore-hyphen-underscore",
			groupPath:  "/scenes/Vixen_-_First Time.mp4",
			scanRoot:   "/scenes",
			wantStudio: "Vixen",
			wantTitle:  "First Time",
			wantOK:     true,
		},
		{
			name:       "no delimiter falls back to leaf=title, parent=studio",
			groupPath:  "/scenes/Vixen/First Time.mp4",
			scanRoot:   "/scenes",
			wantStudio: "Vixen",
			wantTitle:  "First Time",
			wantOK:     true,
		},
		{
			name:       "parent is scan root itself: title only, no studio",
			groupPath:  "/scenes/First Time.mp4",
			scanRoot:   "/scenes",
			wantStudio: "",
			wantTitle:  "First Time",
			wantOK:     true,
		},
		{
			name:       "parent is junk name: title only, no studio",
			groupPath:  "/mnt/Downloads/First Time.mp4",
			scanRoot:   "/mnt",
			wantStudio: "",
			wantTitle:  "First Time",
			wantOK:     true,
		},
		{
			name:       "leaf is junk blocklisted name: no guess",
			groupPath:  "/scenes/Vixen/New Folder.mp4",
			scanRoot:   "/scenes",
			wantStudio: "",
			wantTitle:  "",
			wantOK:     false,
		},
		{
			name:       "leaf is purely numeric: no guess",
			groupPath:  "/scenes/Vixen/12345.mp4",
			scanRoot:   "/scenes",
			wantStudio: "",
			wantTitle:  "",
			wantOK:     false,
		},
		{
			name:       "leaf is too short: no guess",
			groupPath:  "/scenes/Vixen/A.mp4",
			scanRoot:   "/scenes",
			wantStudio: "",
			wantTitle:  "",
			wantOK:     false,
		},
		{
			name:       "extension stripped before junk/delimiter checks",
			groupPath:  "/scenes/Vixen/12345.part1.mp4",
			scanRoot:   "/scenes",
			wantStudio: "Vixen",
			wantTitle:  "12345.part1",
			wantOK:     true,
		},
		{
			name:       "delimiter split with empty studio side is still a partial ok result",
			groupPath:  "/scenes/ - First Time.mp4",
			scanRoot:   "/scenes",
			wantStudio: "",
			wantTitle:  "First Time",
			wantOK:     true,
		},
		{
			name:       "no extension: still treated as leaf",
			groupPath:  "/scenes/Vixen - First Time",
			scanRoot:   "/scenes",
			wantStudio: "Vixen",
			wantTitle:  "First Time",
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := afterdark.FilenameParser{}
			gotStudio, gotTitle, gotOK := p.Parse(context.Background(), tt.groupPath, tt.scanRoot)
			if gotStudio != tt.wantStudio || gotTitle != tt.wantTitle || gotOK != tt.wantOK {
				t.Fatalf("Parse(%q, %q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.groupPath, tt.scanRoot, gotStudio, gotTitle, gotOK, tt.wantStudio, tt.wantTitle, tt.wantOK)
			}
		})
	}
}
