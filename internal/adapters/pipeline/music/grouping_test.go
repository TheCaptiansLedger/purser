package music_test

import (
	"context"
	"purser/internal/adapters/pipeline/music"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

func TestGrouping_ContentTypes(t *testing.T) {
	g := music.Grouping{}
	got := g.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeMusic {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeMusic)
	}
}

func TestGrouping_GroupKeys(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		want  map[string]ports.GroupingResult
	}{
		{
			name: "plain single-disc album",
			paths: []string{
				"/music/REO Speedwagon/Hi Infidelity (1980)/01 - Don't Let Him Go.flac",
				"/music/REO Speedwagon/Hi Infidelity (1980)/02 - Keep On Loving You.flac",
			},
			want: map[string]ports.GroupingResult{
				"/music/REO Speedwagon/Hi Infidelity (1980)/01 - Don't Let Him Go.flac":   {GroupKey: "/music/REO Speedwagon/Hi Infidelity (1980)", DiscNumber: 0},
				"/music/REO Speedwagon/Hi Infidelity (1980)/02 - Keep On Loving You.flac": {GroupKey: "/music/REO Speedwagon/Hi Infidelity (1980)", DiscNumber: 0},
			},
		},
		{
			name: "CD1/CD2/CD3 roll-up (Stevie Nicks box set)",
			paths: []string{
				"/music/Stevie Nicks/Enhanced [Box Set]/CD1/01.flac",
				"/music/Stevie Nicks/Enhanced [Box Set]/CD1/02.flac",
				"/music/Stevie Nicks/Enhanced [Box Set]/CD2/01.flac",
				"/music/Stevie Nicks/Enhanced [Box Set]/CD3/01.flac",
			},
			want: map[string]ports.GroupingResult{
				"/music/Stevie Nicks/Enhanced [Box Set]/CD1/01.flac": {GroupKey: "/music/Stevie Nicks/Enhanced [Box Set]", DiscNumber: 1},
				"/music/Stevie Nicks/Enhanced [Box Set]/CD1/02.flac": {GroupKey: "/music/Stevie Nicks/Enhanced [Box Set]", DiscNumber: 1},
				"/music/Stevie Nicks/Enhanced [Box Set]/CD2/01.flac": {GroupKey: "/music/Stevie Nicks/Enhanced [Box Set]", DiscNumber: 2},
				"/music/Stevie Nicks/Enhanced [Box Set]/CD3/01.flac": {GroupKey: "/music/Stevie Nicks/Enhanced [Box Set]", DiscNumber: 3},
			},
		},
		{
			name: "LP1/LP2 roll-up",
			paths: []string{
				"/music/Various/Vinyl Box/LP1/A1.flac",
				"/music/Various/Vinyl Box/LP1/A2.flac",
				"/music/Various/Vinyl Box/LP 2/B1.flac",
			},
			want: map[string]ports.GroupingResult{
				"/music/Various/Vinyl Box/LP1/A1.flac":  {GroupKey: "/music/Various/Vinyl Box", DiscNumber: 1},
				"/music/Various/Vinyl Box/LP1/A2.flac":  {GroupKey: "/music/Various/Vinyl Box", DiscNumber: 1},
				"/music/Various/Vinyl Box/LP 2/B1.flac": {GroupKey: "/music/Various/Vinyl Box", DiscNumber: 2},
			},
		},
		{
			name: "ambiguous siblings stay split, not merged",
			paths: []string{
				"/music/Various/Deluxe Album/CD1/01.flac",
				"/music/Various/Deluxe Album/Bonus Disc/01.flac",
			},
			want: map[string]ports.GroupingResult{
				"/music/Various/Deluxe Album/CD1/01.flac":        {GroupKey: "/music/Various/Deluxe Album/CD1", DiscNumber: 0},
				"/music/Various/Deluxe Album/Bonus Disc/01.flac": {GroupKey: "/music/Various/Deluxe Album/Bonus Disc", DiscNumber: 0},
			},
		},
		{
			name: "mixed disc-pattern styles still roll up (case-insensitive, spaced, D-prefix)",
			paths: []string{
				"/music/Various/Big Box/cd1/01.flac",
				"/music/Various/Big Box/Disc 2/01.flac",
				"/music/Various/Big Box/D3/01.flac",
			},
			want: map[string]ports.GroupingResult{
				"/music/Various/Big Box/cd1/01.flac":    {GroupKey: "/music/Various/Big Box", DiscNumber: 1},
				"/music/Various/Big Box/Disc 2/01.flac": {GroupKey: "/music/Various/Big Box", DiscNumber: 2},
				"/music/Various/Big Box/D3/01.flac":     {GroupKey: "/music/Various/Big Box", DiscNumber: 3},
			},
		},
		{
			name: "stray non-audio file next to disc subfolders doesn't break sibling check",
			paths: []string{
				"/music/Stevie Nicks/Enhanced [Box Set]/CD1/01.flac",
				"/music/Stevie Nicks/Enhanced [Box Set]/CD2/01.flac",
				// Simulates a sidecar file that leaked past upstream
				// classification: it sits in the box-set folder itself,
				// not in a disc subfolder, so it must not count as a
				// non-matching sibling of CD1/CD2.
				"/music/Stevie Nicks/Enhanced [Box Set]/cover.jpg",
			},
			want: map[string]ports.GroupingResult{
				"/music/Stevie Nicks/Enhanced [Box Set]/CD1/01.flac": {GroupKey: "/music/Stevie Nicks/Enhanced [Box Set]", DiscNumber: 1},
				"/music/Stevie Nicks/Enhanced [Box Set]/CD2/01.flac": {GroupKey: "/music/Stevie Nicks/Enhanced [Box Set]", DiscNumber: 2},
				"/music/Stevie Nicks/Enhanced [Box Set]/cover.jpg":   {GroupKey: "/music/Stevie Nicks/Enhanced [Box Set]", DiscNumber: 0},
			},
		},
		{
			name: "lone disc-pattern folder with no other siblings still rolls up",
			paths: []string{
				"/music/Solo Artist/Odd Rip/CD1/01.flac",
				"/music/Solo Artist/Odd Rip/CD1/02.flac",
			},
			want: map[string]ports.GroupingResult{
				"/music/Solo Artist/Odd Rip/CD1/01.flac": {GroupKey: "/music/Solo Artist/Odd Rip", DiscNumber: 1},
				"/music/Solo Artist/Odd Rip/CD1/02.flac": {GroupKey: "/music/Solo Artist/Odd Rip", DiscNumber: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := music.Grouping{}
			got, err := g.GroupKeys(context.Background(), tt.paths)
			if err != nil {
				t.Fatalf("GroupKeys returned error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("GroupKeys() returned %d entries, want %d: %+v", len(got), len(tt.want), got)
			}
			for path, want := range tt.want {
				if got[path] != want {
					t.Errorf("GroupKeys()[%q] = %+v, want %+v", path, got[path], want)
				}
			}
		})
	}
}
