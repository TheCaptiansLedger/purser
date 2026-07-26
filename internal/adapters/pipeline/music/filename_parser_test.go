package music_test

import (
	"context"
	"purser/internal/adapters/pipeline/music"
	"purser/internal/domain"
	"testing"
)

func TestFilenameParser_ContentTypes(t *testing.T) {
	p := music.FilenameParser{}
	got := p.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeMusic {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeMusic)
	}
}

func TestFilenameParser_Parse(t *testing.T) {
	tests := []struct {
		name       string
		groupPath  string
		scanRoot   string
		wantArtist string
		wantAlbum  string
		wantOK     bool
	}{
		{
			name:       "delimiter split with hyphen",
			groupPath:  "/music/REO Speedwagon - Hi Infidelity",
			scanRoot:   "/music",
			wantArtist: "REO Speedwagon",
			wantAlbum:  "Hi Infidelity",
			wantOK:     true,
		},
		{
			name:       "delimiter split with en dash",
			groupPath:  "/music/Fleetwood Mac – Rumours",
			scanRoot:   "/music",
			wantArtist: "Fleetwood Mac",
			wantAlbum:  "Rumours",
			wantOK:     true,
		},
		{
			name:       "delimiter split with underscore-hyphen-underscore",
			groupPath:  "/music/Stevie Nicks_-_Bella Donna",
			scanRoot:   "/music",
			wantArtist: "Stevie Nicks",
			wantAlbum:  "Bella Donna",
			wantOK:     true,
		},
		{
			name:       "trailing year stripped before delimiter split",
			groupPath:  "/music/REO Speedwagon - Hi Infidelity (1980)",
			scanRoot:   "/music",
			wantArtist: "REO Speedwagon",
			wantAlbum:  "Hi Infidelity",
			wantOK:     true,
		},
		{
			name:       "trailing bracketed year stripped",
			groupPath:  "/music/Artist/Hi Infidelity [1980]",
			scanRoot:   "/music",
			wantArtist: "Artist",
			wantAlbum:  "Hi Infidelity",
			wantOK:     true,
		},
		{
			name:       "no delimiter falls back to leaf=album, parent=artist",
			groupPath:  "/music/Stevie Nicks/Bella Donna (1981)",
			scanRoot:   "/music",
			wantArtist: "Stevie Nicks",
			wantAlbum:  "Bella Donna",
			wantOK:     true,
		},
		{
			name:       "parent is scan root itself: album only, no artist",
			groupPath:  "/music/Bella Donna (1981)",
			scanRoot:   "/music",
			wantArtist: "",
			wantAlbum:  "Bella Donna",
			wantOK:     true,
		},
		{
			name:       "parent is junk name: album only, no artist",
			groupPath:  "/mnt/Downloads/Bella Donna (1981)",
			scanRoot:   "/mnt",
			wantArtist: "",
			wantAlbum:  "Bella Donna",
			wantOK:     true,
		},
		{
			name:       "leaf is junk blocklisted name: no guess",
			groupPath:  "/music/Stevie Nicks/New Folder",
			scanRoot:   "/music",
			wantArtist: "",
			wantAlbum:  "",
			wantOK:     false,
		},
		{
			name:       "leaf is purely numeric: no guess",
			groupPath:  "/music/Stevie Nicks/1981",
			scanRoot:   "/music",
			wantArtist: "",
			wantAlbum:  "",
			wantOK:     false,
		},
		{
			name:       "leaf is too short: no guess",
			groupPath:  "/music/Stevie Nicks/A",
			scanRoot:   "/music",
			wantArtist: "",
			wantAlbum:  "",
			wantOK:     false,
		},
		{
			name:       "leaf is nothing but a year annotation: no guess",
			groupPath:  "/music/Stevie Nicks/(1981)",
			scanRoot:   "/music",
			wantArtist: "",
			wantAlbum:  "",
			wantOK:     false,
		},
		{
			name:       "delimiter split with empty artist side is still a partial ok result",
			groupPath:  "/music/ - Bella Donna",
			scanRoot:   "/music",
			wantArtist: "",
			wantAlbum:  "Bella Donna",
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := music.FilenameParser{}
			gotArtist, gotAlbum, gotOK := p.Parse(context.Background(), tt.groupPath, tt.scanRoot)
			if gotArtist != tt.wantArtist || gotAlbum != tt.wantAlbum || gotOK != tt.wantOK {
				t.Fatalf("Parse(%q, %q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.groupPath, tt.scanRoot, gotArtist, gotAlbum, gotOK, tt.wantArtist, tt.wantAlbum, tt.wantOK)
			}
		})
	}
}
